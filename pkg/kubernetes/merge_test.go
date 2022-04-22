package kubernetes_test

import (
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

var _ = Describe("Merge", func() {
	Context("StatefulSet", func() {
		It("should respect the latest StatefulSet changes", func() {
			spec := ispnv1.InfinispanContainerSpec{Memory: "1Gi:1Gi"}

			memRequests, memLimits, _ := spec.GetMemoryResources()

			existing := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "some-res-version",
				},
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Args: strings.Fields("-l /opt/infinispan/server/conf/operator/log4j.xml -c operator/infinispan.xml"),
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceMemory: memRequests,
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: memLimits,
									},
								},
							}},
						},
					},
				},
			}

			spec = ispnv1.InfinispanContainerSpec{Memory: "512Mi:256Mi"}
			memRequests, memLimits, _ = spec.GetMemoryResources()

			latest := &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Args: strings.Fields("-l /opt/infinispan/server/conf/operator/log4j.xml -u user/infinispan-config.xml -c operator/infinispan.xml"),
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceMemory: memRequests,
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: memLimits,
									},
								},
							}},
						},
					},
				},
			}

			mergedSS := &appsv1.StatefulSet{}
			Expect(kube.Merge(mergedSS, existing, latest)).Should(Succeed())

			Expect(mergedSS.ResourceVersion).Should(Equal(existing.ResourceVersion))
			container := mergedSS.Spec.Template.Spec.Containers[0]
			Expect(container.Args).To(Equal(latest.Spec.Template.Spec.Containers[0].Args))
			Expect(container.Resources.Requests.Memory().String()).To(Equal("256Mi"))
			Expect(container.Resources.Limits.Memory().String()).To(Equal("512Mi"))
		})
	})
})
