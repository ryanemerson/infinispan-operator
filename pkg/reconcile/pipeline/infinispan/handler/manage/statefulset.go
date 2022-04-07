package manage

import (
	"fmt"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	diff "github.com/r3labs/diff/v2"
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

	// Compare the existing StatefulSet spec to the one defined on this reconciliation
	changeLog, err := diff.Diff(existingSS.Spec, newSS.Spec)
	if err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to determine if StatefulSet upgrade is required: %w", err))
		return
	}
	changeLog = changeLog.FilterOut(strings.Fields("Template ObjectMeta Annotations updateDate"))
	changeLog = changeLog.FilterOut(strings.Fields("Template Spec Volumes \\d VolumeSource ConfigMap DefaultMode"))
	changeLog = changeLog.FilterOut(strings.Fields("Template Spec Volumes \\d VolumeSource Secret DefaultMode"))
	changeLog = changeLog.FilterOut(strings.Fields("Template Spec Volumes \\d VolumeSource Secret DefaultMode"))
	changeLog = changeLog.FilterOut(strings.Fields("Template Spec Containers \\d TerminationMessage*"))
	changeLog = changeLog.FilterOut(strings.Fields("Template Spec Containers \\d TerminationMessage*"))
	changeLog = changeLog.FilterOut(strings.Fields("Template Spec Containers \\d ImagePullPolicy"))
	changeLog = changeLog.FilterOut(strings.Fields("Template Spec RestartPolicy"))
	changeLog = changeLog.FilterOut(strings.Fields("Template Spec RestartPolicy"))
	changeLog = changeLog.FilterOut(strings.Fields("Template Spec TerminationGracePeriodSeconds"))
	changeLog = changeLog.FilterOut(strings.Fields("Template Spec DNSPolicy"))
	changeLog = changeLog.FilterOut(strings.Fields("Template Spec SecurityContext"))
	changeLog = changeLog.FilterOut(strings.Fields("Template Spec SchedulerName"))
	changeLog = changeLog.FilterOut(strings.Fields("PodManagementPolicy"))
	changeLog = changeLog.FilterOut(strings.Fields("RevisionHistoryLimit"))

	if len(changeLog) > 0 {
		// Upgrade required
		// TODO check if replicas is only change. If so, don't set "updateDate" annotation as no need for rolling upgrade
		// 	r.reqLogger.Info("replicas changed, update infinispan", "replicas", replicas, "previous replicas", previousReplicas)
		newSS.Spec.Template.Annotations["updateDate"] = time.Now().String()
		// TODO updateDate persisted, pods rolling out it seems ... what's going wrong?
	}
}
