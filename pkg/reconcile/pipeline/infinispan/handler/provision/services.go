package provision

import (
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

func PingService(ctx pipeline.Context) {
	i := ctx.Instance()

	service := newService(
		i.GetPingServiceName(),
		i.Namespace,
		i.ServiceLabels("infinispan-service-ping"),
		i.ServiceAnnotations(),
	)

	service.Spec = corev1.ServiceSpec{
		Type:      corev1.ServiceTypeClusterIP,
		ClusterIP: corev1.ClusterIPNone,
		Selector:  i.ServiceSelectorLabels(),
		Ports: []corev1.ServicePort{
			{
				Name: consts.InfinispanPingPortName,
				Port: consts.InfinispanPingPort,
			},
		},
	}
	ctx.Resources().Define(service)
}

func ClusterService(ctx pipeline.Context) {
	i := ctx.Instance()

	service := newService(
		i.GetServiceName(),
		i.Namespace,
		i.ServiceLabels("infinispan-service"),
		i.ServiceAnnotations(),
	)

	service.Spec = corev1.ServiceSpec{
		Type:     corev1.ServiceTypeClusterIP,
		Selector: i.ServiceSelectorLabels(),
		Ports: []corev1.ServicePort{
			{
				Name: consts.InfinispanUserPortName,
				Port: consts.InfinispanUserPort,
			},
		},
	}

	if i.IsEncryptionCertFromService() {
		if strings.Contains(i.Spec.Security.EndpointEncryption.CertServiceName, "openshift.io") {
			// Using platform service. Only OpenShift is integrated atm
			secretName := i.GetKeystoreSecretName()
			service.Annotations[i.Spec.Security.EndpointEncryption.CertServiceName+"/serving-cert-secret-name"] = secretName
		}
	}
	ctx.Resources().Define(service)
}

func AdminService(ctx pipeline.Context) {
	i := ctx.Instance()

	service := newService(
		i.GetAdminServiceName(),
		i.Namespace,
		i.ServiceLabels("infinispan-service-admin"),
		i.ServiceAnnotations(),
	)

	service.Spec = corev1.ServiceSpec{
		Type:      corev1.ServiceTypeClusterIP,
		ClusterIP: corev1.ClusterIPNone,
		Selector:  i.ServiceSelectorLabels(),
		Ports: []corev1.ServicePort{
			{
				Name: consts.InfinispanAdminPortName,
				Port: consts.InfinispanAdminPort,
			},
		},
	}
	ctx.Resources().Define(service)
}

func ExternalService(ctx pipeline.Context) {
	// TODO
}

func newService(name, namespace string, labels, annotations map[string]string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
	}
}
