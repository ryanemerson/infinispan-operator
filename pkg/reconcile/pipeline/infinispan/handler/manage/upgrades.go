package manage

import (
	"fmt"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ingressv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// Current update path:
// 1. r.scheduleUpgradeIfNeeded
// If !ConditionUpgrade and image upgrade is required, then set ConditionUpgrade True and spec.Replicas == 0, update CR
// Proceed if no errors
// [ConditionUpgrade]

// 2. r.reconcileGracefulShutdown
// If spec.Replicas == 0
// 		If StatefulSet spec hasn't been updated to replicas == 0
// 		If !ConditionStopping, Then initiate GracefulShutdown on the server. Set ConditionStopping = true, ConditionWellFormed = false. Requeue
// [ConditionUpgrade, ConditionStopping]

// 3. r.IsUpgradeNeeded
// ConditionStopping so return false
// "replicas to restart with not yet set, wait for graceful shutdown to complete"

// 4. r.reconcileGracefulShutdown
// 		Set status.replicasWantedAtRestart = statefulset.Spec.Replicas, Update CR
// 		Set statefulset.Spec.Replicas = 0, Update statefulset
// 		Set ConditionGracefulShutdown = true, ConditionStopping = false. Requeue
// [ConditionUpgrade, ConditionGracefulShutdown]

// 5. r.IsUpgradeNeeded, returns true
// "graceful shutdown after upgrade completed, continue upgrade process"
// Destroy resources
// Set ConditionUpgrading = false, spec.replicas = status.replicasWantedAtRestart. Requeue
// [ConditionGracefulShutdown]

// 6. r.reconcileGracefulShutdown
// If spec.Replicas != 0 && ConditionGracefulShutdown
// "Resuming from graceful shutdown"
// Set ConditionGracefulShutdown = false, Status.ReplicasWanatedAtRestart = 0. Requeue
// []

// 7. Create new StatefulSet

// 8. r.getInfinispanConditions
//  Requeue until required #pods exists
//  [ConditionWellFormed]

func ScheduleGracefulShutdownUpgrade(ctx pipeline.Context) {
	i := ctx.Instance()

	if i.IsUpgradeCondition() || i.Spec.Upgrades.Type != ispnv1.UpgradeTypeShutdown {
		return
	}

	podList := &corev1.PodList{}
	if err := ctx.Resources().List(i.PodLabels(), podList); err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to list pods in ScheduleGracefulShutdownUpgrade: %w", err))
		return
	}

	// We can't upgrade pods that don't exist
	if len(podList.Items) == 0 {
		return
	}

	// Get default Infinispan image for a running Infinispan pod
	// TODO use constant for container name
	podDefaultImage := kube.GetPodDefaultImage(*kube.GetContainer("infinispan", &podList.Items[0].Spec))

	// If the operator's default image differs from the pod's default image,
	// schedule an upgrade by gracefully shutting down the current cluster.
	if podDefaultImage != consts.DefaultImageName {
		ctx.Log().Info("schedule an Infinispan cluster upgrade", "pod default image", podDefaultImage, "desired image", consts.DefaultImageName)
		i.SetCondition(ispnv1.ConditionUpgrade, metav1.ConditionTrue, "")
		i.Spec.Replicas = 0
		// Retry in order to persist the Status updates
		ctx.RetryProcessing(nil)
		return
	}
}

func ExecuteGracefulShutdownUpgrade(ctx pipeline.Context) {
	i := ctx.Instance()
	logger := ctx.Log()

	if !i.HasAnyConditionTrue(ispnv1.ConditionUpgrade, ispnv1.ConditionStopping, ispnv1.ConditionGracefulShutdown) {
		return
	}

	podList := &corev1.PodList{}
	if err := ctx.Resources().List(i.PodLabels(), podList); err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to list pods in ExecuteGracefulShutdownUpgrade: %w", err))
		return
	}

	statefulSet := &appsv1.StatefulSet{}
	if err := ctx.Resources().LoadWithNoCaching(i.GetStatefulSetName(), statefulSet); err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to retrieve StatefulSet in ExecuteGracefulShutdownUpgrade: %w", err))
		return
	}

	// Initiate the GracefulShutdown if it's not already in progress
	if i.IsUpgradeCondition() && !i.HasCondition(ispnv1.ConditionGracefulShutdown) && i.Spec.Replicas == 0 {

		logger.Info(".Spec.Replicas==0")
		if *statefulSet.Spec.Replicas != 0 {
			logger.Info("StatefulSet.Spec.Replicas!=0")
			// Only send a GracefulShutdown request to the server if it hasn't succeeded already
			if !i.IsConditionTrue(ispnv1.ConditionStopping) {
				logger.Info("Sending GracefulShutdown request to the Infinispan cluster")
				var shutdownExecuted bool
				for _, pod := range podList.Items {
					if kube.IsPodReady(pod) {
						ispnClient := ctx.InfinispanClientForPod(pod.Name)
						// This will fail on 12.x servers as the method does not exist
						if err := ispnClient.Container().Shutdown(); err != nil {
							logger.Error(err, "Error encountered on container shutdown. Attempting to execute GracefulShutdownTask")

							if err := ispnClient.Container().ShutdownTask(); err != nil {
								logger.Error(err, fmt.Sprintf("Error encountered using GracefulShutdownTask on pod %s", pod.Name))
								continue
							} else {
								shutdownExecuted = true
								break
							}
						} else {
							shutdownExecuted = true
							logger.Info("Executed GracefulShutdown on pod: ", "Pod.Name", pod.Name)
							break
						}
					}
				}

				if shutdownExecuted {
					logger.Info("GracefulShutdown successfully executed on the Infinispan cluster")
					i.SetCondition(ispnv1.ConditionStopping, metav1.ConditionTrue, "")
					i.SetCondition(ispnv1.ConditionWellFormed, metav1.ConditionFalse, "")
				}
				// Retry in order to persist the Status updates, or retry the GracefulShutdown if na error occurred
				ctx.RetryProcessing(nil)
				return
			}

			i.Status.ReplicasWantedAtRestart = *statefulSet.Spec.Replicas
			statefulSet.Spec.Replicas = pointer.Int32Ptr(0)

			// GracefulShutdown in progress, but we must wait until the StatefulSet has scaled down before proceeding
			ctx.RetryProcessing(nil)
			return
		}
		// GracefulShutdown complete, proceed with the upgrade
		i.SetCondition(ispnv1.ConditionGracefulShutdown, metav1.ConditionTrue, "")
		i.SetCondition(ispnv1.ConditionStopping, metav1.ConditionFalse, "")

		// Retry in order to persist the Status updates
		ctx.RetryProcessing(nil)
		return
	}

	if i.IsUpgradeCondition() && i.HasCondition(ispnv1.ConditionGracefulShutdown) {
		logger.Info("GracefulShutdown complete, continuing upgrade process")
		markAllResourcesForDeletion(ctx, i)

		i.Spec.Replicas = i.Status.ReplicasWantedAtRestart
		i.SetCondition(ispnv1.ConditionUpgrade, metav1.ConditionFalse, "")
		// Retry in order to persist the Status updates
		ctx.RetryProcessing(nil)
		return
	}

	if i.Spec.Replicas > 0 && i.HasCondition(ispnv1.ConditionGracefulShutdown) {
		logger.Info("Resuming from graceful shutdown")
		if i.Spec.Replicas != i.Status.ReplicasWantedAtRestart {
			ctx.RetryProcessing(fmt.Errorf("Spec.Replicas(%d) must be 0 or equal to Status.ReplicasWantedAtRestart(%d)", i.Spec.Replicas, i.Status.ReplicasWantedAtRestart))
		}
		i.Status.ReplicasWantedAtRestart = 0
		i.SetCondition(ispnv1.ConditionGracefulShutdown, metav1.ConditionFalse, "")
		// Retry in order to persist the Status updates
		ctx.RetryProcessing(nil)
		return
	}
}

func markAllResourcesForDeletion(ctx pipeline.Context, i *ispnv1.Infinispan) {
	meta := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Name:      name,
			Namespace: i.Namespace,
		}
	}

	r := ctx.Resources()
	r.MarkForDeletion(&appsv1.StatefulSet{ObjectMeta: meta(i.GetStatefulSetName())})
	r.MarkForDeletion(&appsv1.Deployment{ObjectMeta: meta(i.GetGossipRouterDeploymentName())})
	r.MarkForDeletion(&corev1.ConfigMap{ObjectMeta: meta(i.GetConfigName())})
	r.MarkForDeletion(&corev1.Service{ObjectMeta: meta(i.Name)})
	r.MarkForDeletion(&corev1.Service{ObjectMeta: meta(i.GetPingServiceName())})
	r.MarkForDeletion(&corev1.Service{ObjectMeta: meta(i.GetAdminServiceName())})
	r.MarkForDeletion(&corev1.Service{ObjectMeta: meta(i.GetServiceExternalName())})
	r.MarkForDeletion(&corev1.Service{ObjectMeta: meta(i.GetSiteServiceName())})

	// ConfigListener
	configListenerName := i.GetConfigListenerName()
	r.MarkForDeletion(&appsv1.Deployment{ObjectMeta: meta(configListenerName)})
	r.MarkForDeletion(&rbacv1.RoleBinding{ObjectMeta: meta(configListenerName)})
	r.MarkForDeletion(&rbacv1.Role{ObjectMeta: meta(configListenerName)})
	r.MarkForDeletion(&corev1.ServiceAccount{ObjectMeta: meta(configListenerName)})

	if ctx.IsTypeSupported(pipeline.RouteGVK) {
		r.MarkForDeletion(&routev1.Route{ObjectMeta: meta(i.GetServiceExternalName())})
	} else if ctx.IsTypeSupported(pipeline.IngressGVK) {
		r.MarkForDeletion(&ingressv1.Ingress{ObjectMeta: meta(i.GetServiceExternalName())})
	}
}