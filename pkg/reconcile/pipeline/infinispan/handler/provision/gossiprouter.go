package provision

import (
	"fmt"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GossipRouter(i *ispnv1.Infinispan, ctx pipeline.Context) {
	r := ctx.Resources()

	if !i.HasSites() {
		_ = ctx.Resources().Delete(i.GetGossipRouterDeploymentName(), &appsv1.Deployment{}, pipeline.RetryOnErr)
		return
	}

	// Remove old deployment to change the deployment name, required for upgrades
	oldRouterDeployment := fmt.Sprintf("%s-tunnel", i.Name)
	if err := r.Delete(oldRouterDeployment, &appsv1.Deployment{}, pipeline.RetryOnErr); err != nil {
		return
	}
	routerDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.GetGossipRouterDeploymentName(),
			Namespace: i.Namespace,
		},
	}

	mutateFn := func() error {
		// TODO add logic from GetGossipRouterDeployment
		return nil
	}

	if err := r.CreateOrUpdate(routerDeployment, true, mutateFn, pipeline.RetryOnErr); err != nil {
		return
	}
	// TODO add operation result to CreateOrUpdate|Patch methods
	//if result != controllerutil.OperationResultNone {
	//	reqLogger.Info(fmt.Sprintf("Cross-site deployment '%s' %s", routerDeployment.Name, string(result)))
	//}

	pods := &corev1.PodList{}
	if err := r.List(i.GossipRouterPodSelectorLabels(), pods, pipeline.RetryOnErr); err != nil {
		ctx.Log().Error(err, "Failed to fetch Gossip Router pod")
		return
	}

	if len(pods.Items) == 0 || !kube.AreAllPodsReady(pods) {
		msg := "Gossip Router pod is not ready"
		ctx.Log().Info(msg)
		i.SetCondition(ispnv1.ConditionGossipRouterReady, metav1.ConditionFalse, msg)
		ctx.Requeue(nil)
		return
	}

	i.SetCondition(ispnv1.ConditionGossipRouterReady, metav1.ConditionTrue, "")
	_ = ctx.UpdateInfinispan()
}

//func GetGossipRouterDeployment(i *ispnv1.Infinispan, ctx pipeline.Context)
