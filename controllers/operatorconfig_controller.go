package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/infinispan/infinispan-operator/pkg/k8sutil"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const configMapName = "infinispan-operator-config"

var currentConfig map[string]string = make(map[string]string)

// ReconcileInfinispan reconciles a Infinispan object
type ReconcileOperatorConfig struct {
	Client     client.Client
	scheme     *runtime.Scheme
	log        logr.Logger
	kubernetes *kube.Kubernetes
}

func (r *ReconcileOperatorConfig) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.log = ctrl.Log.WithName("controllers").WithName("OperatorConfig")
	r.scheme = mgr.GetScheme()
	r.kubernetes = kube.NewKubernetesFromController(mgr)

	// Create a new controller
	operatorNS, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(predicate.Funcs{
			DeleteFunc: func(e event.DeleteEvent) bool {
				return e.Object.GetName() == configMapName && e.Object.GetNamespace() == operatorNS
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return e.Object.GetName() == configMapName && e.Object.GetNamespace() == operatorNS
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return e.ObjectNew.GetName() == configMapName && e.ObjectNew.GetNamespace() == operatorNS
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return e.Object.GetName() == configMapName && e.Object.GetNamespace() == operatorNS
			},
		}).
		Complete(r)
}

func (r *ReconcileOperatorConfig) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	operatorNs, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		r.log.Error(err, "Error getting operator runtime namespace")
		return reconcile.Result{Requeue: true}, nil
	}

	configMap := &corev1.ConfigMap{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: operatorNs, Name: configMapName}, configMap)
	if err != nil && !errors.IsNotFound(err) {
		r.log.Error(err, "Error getting operator configuration resource")
		return reconcile.Result{Requeue: true}, nil
	}

	config := map[string]string{
		grafanaDashboardMonitoringKey: "middleware",
		grafanaDashboardNameKey:       "infinispan",
	}
	// Merge config value with defaults
	for k, v := range configMap.Data {
		config[k] = v
	}
	res, err := r.reconcileGrafana(config, currentConfig, operatorNs)
	return *res, err
}
