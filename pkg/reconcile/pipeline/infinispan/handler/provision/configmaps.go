package provision

import (
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func InfinispanConfigMap(ctx pipeline.Context) {
	i := ctx.Instance()
	config := ctx.ConfigFiles()

	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.GetConfigName(),
			Namespace: i.Namespace,
		},
	}

	err := ctx.Resources().CreateOrUpdate(configmap, true, func() {
		configmap.Data = map[string]string{
			"infinispan.xml": config.ServerConfig,
			"log4j.xml":      config.Log4j,
		}
		configmap.Labels = i.Labels("infinispan-configmap-configuration")
	})

	if err != nil {
		ctx.RetryProcessing(err)
	}
}
