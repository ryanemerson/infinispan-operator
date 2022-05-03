package manage

import (
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// RemoveFailedInitContainers Recover Pods with updated init containers in case of fails
func RemoveFailedInitContainers(i *ispnv1.Infinispan, ctx pipeline.Context) {
	podList, err := ctx.InfinispanPods()
	if err != nil {
		return
	}

	statefulSet := &appsv1.StatefulSet{}
	if err := ctx.Resources().Load(i.GetStatefulSetName(), statefulSet, pipeline.RetryOnErr); err != nil {
		return
	}

	for _, pod := range podList.Items {
		if !kube.IsInitContainersEqual(statefulSet.Spec.Template.Spec.InitContainers, pod.Spec.InitContainers) {
			if kube.InitContainerFailed(pod.Status.InitContainerStatuses) {
				if err := ctx.Resources().Delete(pod.Name, &pod); err != nil {
					ctx.RetryProcessing(err)
					return
				}
			}
		}
	}
}

// UpdatePodLabels Ensure all pods have upto date labels
func UpdatePodLabels(i *ispnv1.Infinispan, ctx pipeline.Context) {
	podList, err := ctx.InfinispanPods()
	if err != nil {
		return
	}

	if len(podList.Items) == 0 {
		return
	}

	labelsForPod := i.PodLabels()
	for _, pod := range podList.Items {
		podLabels := make(map[string]string)
		for index, value := range pod.Labels {
			if _, ok := labelsForPod[index]; ok || consts.SystemPodLabels[index] {
				podLabels[index] = value
			}
		}
		for index, value := range labelsForPod {
			podLabels[index] = value
		}

		mutateFn := func() error {
			if pod.CreationTimestamp.IsZero() {
				return errors.NewNotFound(corev1.Resource(""), pod.Name)
			}
			pod.Labels = podLabels
			return nil
		}
		err := ctx.Resources().CreateOrUpdate(&pod, false, mutateFn, pipeline.IgnoreNotFound, pipeline.RetryOnErr)
		if err != nil {
			return
		}
	}
}
