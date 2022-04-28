package manage

import (
	"fmt"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	"github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan/handler/provision"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ingressv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func ScheduleGracefulShutdownUpgrade(i *ispnv1.Infinispan, ctx pipeline.Context) {
	if i.IsUpgradeCondition() {
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

func ExecuteGracefulShutdownUpgrade(i *ispnv1.Infinispan, ctx pipeline.Context) {
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
	if err := ctx.Resources().Load(i.GetStatefulSetName(), statefulSet); err != nil {
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
		destroyResources(i, ctx)

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

func destroyResources(i *ispnv1.Infinispan, ctx pipeline.Context) {

	type resource struct {
		name string
		obj  client.Object
	}

	resources := []resource{
		{i.GetStatefulSetName(), &appsv1.StatefulSet{}},
		{i.GetGossipRouterDeploymentName(), &appsv1.Deployment{}},
		{i.GetConfigName(), &corev1.ConfigMap{}},
		{i.Name, &corev1.Service{}},
		{i.GetPingServiceName(), &corev1.Service{}},
		{i.GetAdminServiceName(), &corev1.Service{}},
		{i.GetServiceExternalName(), &corev1.Service{}},
		{i.GetSiteServiceName(), &corev1.Service{}},
	}

	del := func(name string, obj client.Object) error {
		if err := ctx.Resources().Delete(name, obj); err != nil {
			ctx.RetryProcessing(err)
			return err
		}
		return nil
	}

	for _, r := range resources {
		if err := del(r.name, r.obj); err != nil {
			ctx.RetryProcessing(err)
			return
		}
	}

	if ctx.IsTypeSupported(pipeline.RouteGVK) {
		if err := del(i.GetServiceExternalName(), &routev1.Route{}); err != nil {
			return
		}
	} else if ctx.IsTypeSupported(pipeline.IngressGVK) {
		if err := del(i.GetServiceExternalName(), &ingressv1.Ingress{}); err != nil {
			return
		}
	}

	provision.RemoveConfigListener(i, ctx)
}
