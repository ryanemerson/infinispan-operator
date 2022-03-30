package manage

import (
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func PrelimChecksCondition(ctx pipeline.Context) {
	i := ctx.Instance()
	if i.GetCondition(ispnv1.ConditionPrelimChecksPassed).Status == metav1.ConditionFalse {
		i.SetCondition(ispnv1.ConditionPrelimChecksPassed, metav1.ConditionTrue, "")
		ctx.RetryProcessing(nil)
	}
}

func WellFormedCondition(ctx pipeline.Context) {
	i := ctx.Instance()
	statefulset := ctx.Resources().StatefulSets().Get(i.GetStatefulSetName())
	if statefulset == nil {
		// Statefulset hasn't been created yet, so it's not possible for cluster to be wellformed
		ctx.RetryProcessing(nil)
		return
	}
	ctx.ListResources() ?
	// Get PodList
	if !kube.ArePodIPsReady(podList) {
		reqLogger.Info("Pods IPs are not ready yet")
		return ctrl.Result{Requeue: true, RequeueAfter: consts.DefaultWaitClusterPodsNotReady}, r.update(func() {
			infinispan.SetCondition(infinispanv1.ConditionWellFormed, metav1.ConditionUnknown, "Pods are not ready")
			infinispan.RemoveCondition(infinispanv1.ConditionCrossSiteViewFormed)
			infinispan.Status.StatefulSetName = statefulSet.Name
		})
	}
}
