package manage

import (
	infinispanv1 "github.com/infinispan/infinispan-operator/api/v1"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UpgradeConditionTrue(ctx pipeline.Context) {
	i := ctx.Instance()
	log := ctx.Log()

	if i.IsUpgradeNeeded(log) {
		log.Info("Upgrade needed")
		// TODO how to destroy resources?
		// MarkForDeletion on all resources and call Close() when pipeline stops?
		i.SetCondition(infinispanv1.ConditionUpgrade, metav1.ConditionFalse, "")
		if i.Spec.Replicas != i.Status.ReplicasWantedAtRestart {
			log.Info("removed Infinispan resources, force an upgrade now", "replicasWantedAtRestart", i.Status.ReplicasWantedAtRestart)
			i.Spec.Replicas = i.Status.ReplicasWantedAtRestart
		}
		ctx.RetryProcessing(nil)
		return
	}
}

func ScheduleUpgrade(ctx pipeline.Context) {
	// TODO scheduleUpgradeIfNeeded from old controller
}

func GracefulShutdown(ctx pipeline.Context) {
	// TODO reconcileGracefulShutdown
}
