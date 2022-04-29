package provision

import (
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func InfinispanConfigMap(i *ispnv1.Infinispan, ctx pipeline.Context) {
	config := ctx.ConfigFiles()

	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.GetConfigName(),
			Namespace: i.Namespace,
		},
	}

	err := ctx.Resources().CreateOrUpdate(configmap, true, func() {
		configmap.Data = map[string]string{
			"infinispan.xml":      config.ServerConfig,
			"infinispan-zero.xml": config.ZeroConfig,
			"log4j.xml":           config.Log4j,
		}
		configmap.Labels = i.Labels("infinispan-configmap-configuration")
	})

	if err != nil {
		ctx.RetryProcessing(err)
	}
}
