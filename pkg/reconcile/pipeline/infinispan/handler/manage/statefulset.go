package manage

import (
	"fmt"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	"github.com/r3labs/diff/v2"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"strings"
	"time"
)

func StatefulSetRollingUpgrade(ctx pipeline.Context) {
	i := ctx.Instance()

	existingSS := &appsv1.StatefulSet{}
	if err := ctx.Resources().LoadWithNoCaching(i.GetStatefulSetName(), existingSS); err != nil {
		if errors.IsNotFound(err) {
			// No existing StatefulSet so nothing todo
			return
		}
		ctx.RetryProcessing(fmt.Errorf("unable to retrieve StatefulSet in ExecuteGracefulShutdownUpgrade: %w", err))
		return
	}

	newSS := &appsv1.StatefulSet{}
	if err := ctx.Resources().Load(i.GetStatefulSetName(), newSS); err != nil {
		// Should never happen as this step should always be after provision.ClusterStatefulSet
		ctx.RetryProcessing(fmt.Errorf("unable to retrieve latest StatefulSet definition from context: %w", err))
		return
	}

	mergedSS := &appsv1.StatefulSet{}
	if err := kube.Merge(mergedSS, existingSS, newSS); err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to merge the existing and new StatefulSet: %w", err))
		return
	}

	// Compare the existing StatefulSet to the one defined on this reconciliation
	changeLog, err := diff.Diff(existingSS, mergedSS)
	if err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to determine if StatefulSet upgrade is required: %w", err))
		return
	}

	// Ignore irrelevant fields
	changeLog = changeLog.FilterOut(strings.Fields("Status"))
	changeLog = changeLog.FilterOut(strings.Fields("ObjectMeta CreationTimestamp Time"))

	numChanges := len(changeLog)
	if numChanges > 0 {
		// Upgrade required
		withoutReplicas := changeLog.FilterOut([]string{"Spec Replicas"})
		if len(withoutReplicas) < numChanges {
			ctx.Log().Info("replicas changed, update infinispan", "replicas", mergedSS.Spec.Replicas, "previous replicas", existingSS.Spec.Replicas)

			// If there are no more changes, then don't set the updateDate annotation in order to avoid an unnecessary Rolling Upgrade
			if len(withoutReplicas) == 0 {
				// Requeue the request so that the StatefulSet changes are persisted
				ctx.RetryProcessing(nil)
				return
			}
		}
		// Update the updateDate annotation in order to trigger a StatefulSet Rolling Upgrade so that all pods have the latest spec
		newSS.Spec.Template.Annotations["updateDate"] = time.Now().String()
		// Requeue the request so that the StatefulSet changes are persisted
		ctx.RetryProcessing(nil)
		return
	}
}
