package manage

import (
	"fmt"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sort"
	"strings"
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
	statefulSet := &appsv1.StatefulSet{}
	if err := ctx.Resources().Load(i.GetStatefulSetName(), statefulSet); err != nil {
		if errors.IsNotFound(err) {
			// StatefulSet hasn't been created yet, so it's not possible for cluster to be well-formed
			err = nil
		}
		ctx.RetryProcessing(err)
		return
	}
	podList := &corev1.PodList{}
	if err := ctx.Resources().List(i.PodLabels(), podList); err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to list pods when checking if cluster WellFormed: %w", err))
		return
	}
	kube.FilterByStatefulSetUUID(podList, statefulSet)

	if !kube.ArePodIPsReady(podList) {
		ctx.Log().Info("Pods IPs are not ready yet")

		i.SetCondition(ispnv1.ConditionWellFormed, metav1.ConditionUnknown, "Pods are not ready")
		i.RemoveCondition(ispnv1.ConditionCrossSiteViewFormed)
		i.Status.StatefulSetName = statefulSet.Name
		ctx.RetryProcessing(nil)
		return
	}

	clusterViews := make(map[string]bool)
	numPods := int32(len(podList.Items))
	var conditions []ispnv1.InfinispanCondition
	var errors []string
	// Avoid contacting the server(s) if we're still waiting for pods
	if numPods < i.Spec.Replicas {
		errors = append(errors, fmt.Sprintf("Running %d pods. Needed %d", numPods, i.Spec.Replicas))
	} else {
		for _, pod := range podList.Items {
			if kube.IsPodReady(pod) {
				if members, err := ctx.InfinispanClientForPod(pod.Name).Container().Members(); err == nil {
					sort.Strings(members)
					clusterView := strings.Join(members, ",")
					clusterViews[clusterView] = true
				} else {
					errors = append(errors, pod.Name+": "+err.Error())
				}
			} else {
				// Pod not ready, no need to query
				errors = append(errors, pod.Name+": pod not ready")
			}
		}
	}

	// Evaluating WellFormed condition
	wellformed := ispnv1.InfinispanCondition{Type: ispnv1.ConditionWellFormed}
	views := make([]string, len(clusterViews))
	index := 0
	for k := range clusterViews {
		views[index] = k
		index++
	}
	sort.Strings(views)
	if len(errors) == 0 {
		if len(views) == 1 {
			wellformed.Status = metav1.ConditionTrue
			wellformed.Message = "View: " + views[0]
		} else {
			wellformed.Status = metav1.ConditionFalse
			wellformed.Message = "Views: " + strings.Join(views, ",")
		}
	} else {
		wellformed.Status = metav1.ConditionUnknown
		wellformed.Message = "Errors: " + strings.Join(errors, ",") + " Views: " + strings.Join(views, ",")
	}
	conditions = append(conditions, wellformed)
	i.SetConditions(conditions)

	if i.NotClusterFormed(len(podList.Items), int(i.Spec.Replicas)) {
		ctx.Log().Info("Cluster not well-formed, retrying ...")
		ctx.RetryProcessing(nil)
	}
}
