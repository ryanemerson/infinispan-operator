package provision

import (
	"fmt"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	ingressv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
				Name:       consts.InfinispanPingPortName,
				Port:       consts.InfinispanPingPort,
				TargetPort: intstr.FromInt(consts.InfinispanPingPort),
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
				Name:       consts.InfinispanUserPortName,
				Port:       consts.InfinispanUserPort,
				TargetPort: intstr.FromInt(consts.InfinispanPingPort),
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
				Name:       consts.InfinispanAdminPortName,
				Port:       consts.InfinispanAdminPort,
				TargetPort: intstr.FromInt(consts.InfinispanPingPort),
			},
		},
	}
	ctx.Resources().Define(service)
}

func ExternalService(ctx pipeline.Context) {
	i := ctx.Instance()

	if !i.IsExposed() {
		return
	}

	// If expose type has changed, ensure that we remove all existing expose definitions
	exposeType := i.GetExposeType()
	for _, gvk := range pipeline.ServiceTypes {
		if ctx.IsTypeSupported(gvk) && gvk.Kind != string(exposeType) {
			labels := i.ExternalServiceSelector()
			switch gvk {
			case pipeline.ServiceGVK:
				serviceList := &corev1.ServiceList{}
				if err := ctx.Resources().List(labels, serviceList); err != nil {
					ctx.Log().Error(err, "unable to list Services for deletion")
				}
				for _, service := range serviceList.Items {
					ctx.Resources().MarkForDeletion(&service)
				}
			case pipeline.RouteGVK:
				routeList := &routev1.RouteList{}
				if err := ctx.Resources().List(labels, routeList); err != nil {
					ctx.Log().Error(err, "unable to list Routes for deletion")
				}
				for _, route := range routeList.Items {
					ctx.Resources().MarkForDeletion(&route)
				}
			case pipeline.IngressGVK:
				ingressList := &ingressv1.IngressList{}
				if err := ctx.Resources().List(labels, ingressList); err != nil {
					ctx.Log().Error(err, "unable to list Ingress' for deletion")
				}
				for _, route := range ingressList.Items {
					ctx.Resources().MarkForDeletion(&route)
				}
			}
		}
	}

	switch exposeType {
	case ispnv1.ExposeTypeLoadBalancer, ispnv1.ExposeTypeNodePort:
		defineExternalService(ctx, i)
	case ispnv1.ExposeTypeRoute:
		if ctx.IsTypeSupported(pipeline.RouteGVK) {
			defineExternalRoute(ctx, i)
		} else if ctx.IsTypeSupported(pipeline.IngressGVK) {
			defineExternalIngress(ctx, i)
		} else {
			ctx.Error(fmt.Errorf("unable to expose cluster with type Route, as no implementations are supported"))
		}
	}
}

func defineExternalService(ctx pipeline.Context, i *ispnv1.Infinispan) {
	exposeConf := i.Spec.Expose
	externalServiceType := corev1.ServiceType(i.Spec.Expose.Type)

	service := newService(
		i.GetServiceExternalName(),
		i.Namespace,
		i.ExternalServiceLabels(),
		i.ServiceAnnotations(),
	)
	for k, v := range i.Spec.Expose.Annotations {
		service.Annotations[k] = v
	}

	exposeSpec := corev1.ServiceSpec{
		Type:     externalServiceType,
		Selector: i.ServiceSelectorLabels(),
		Ports: []corev1.ServicePort{
			{
				Port:       int32(consts.InfinispanUserPort),
				TargetPort: intstr.FromInt(consts.InfinispanUserPort),
			},
		},
	}
	if exposeConf.NodePort > 0 && exposeConf.Type == ispnv1.ExposeTypeNodePort {
		exposeSpec.Ports[0].NodePort = exposeConf.NodePort
	}
	if exposeConf.Port > 0 && exposeConf.Type == ispnv1.ExposeTypeLoadBalancer {
		exposeSpec.Ports[0].Port = exposeConf.Port
	}
	service.Spec = exposeSpec
	ctx.Resources().Define(service)
}

func defineExternalRoute(ctx pipeline.Context, i *ispnv1.Infinispan) {
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "route.openshift.io/v1",
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        i.GetServiceExternalName(),
			Namespace:   i.Namespace,
			Annotations: i.ServiceAnnotations(),
			Labels:      i.ExternalServiceLabels(),
		},
		Spec: routev1.RouteSpec{
			Host: i.Spec.Expose.Host,
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromInt(consts.InfinispanUserPort),
			},
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: i.Name,
			},
		},
	}
	if i.IsEncryptionEnabled() {
		route.Spec.TLS = &routev1.TLSConfig{Termination: routev1.TLSTerminationPassthrough}
	}
	ctx.Resources().Define(route)
}

func defineExternalIngress(ctx pipeline.Context, i *ispnv1.Infinispan) {
	pathTypePrefix := ingressv1.PathTypePrefix
	ingress := &ingressv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        i.GetServiceExternalName(),
			Namespace:   i.Namespace,
			Annotations: i.ServiceAnnotations(),
			Labels:      i.ExternalServiceLabels(),
		},
		Spec: ingressv1.IngressSpec{
			TLS: []ingressv1.IngressTLS{},
			Rules: []ingressv1.IngressRule{
				{
					Host: i.Spec.Expose.Host,
					IngressRuleValue: ingressv1.IngressRuleValue{
						HTTP: &ingressv1.HTTPIngressRuleValue{
							Paths: []ingressv1.HTTPIngressPath{
								{
									PathType: &pathTypePrefix,
									Path:     "/",
									Backend: ingressv1.IngressBackend{
										Service: &ingressv1.IngressServiceBackend{
											Name: i.Name,
											Port: ingressv1.ServiceBackendPort{Number: consts.InfinispanUserPort},
										},
									}}},
						}}}}}}
	if i.IsEncryptionEnabled() {
		ingress.Spec.TLS = []ingressv1.IngressTLS{
			{
				Hosts: []string{i.Spec.Expose.Host},
			},
		}
	}
	ctx.Resources().Define(ingress)
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
