package controllers

import (
	"context"
	"fmt"
	"github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	infinispanv1 "github.com/infinispan/infinispan-operator/api/v1"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	pipelineBuilder "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan/builder"
	pipelineContext "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan/context"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO rename once InfinispanReconciler removed
// IspnReconciler reconciles a Infinispan object
type IspnReconciler struct {
	client.Client
	log             logr.Logger
	contextProvider infinispan.ContextProvider
}

func (r *IspnReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.log = ctrl.Log.WithName("controllers").WithName("Infinispan")
	// TODO how to pass Per request logger to provider? Probably only required for trace/debug logging
	r.contextProvider = pipelineContext.Provider(
		r.Client,
		mgr.GetScheme(),
		kube.NewKubernetesFromController(mgr),
		mgr.GetEventRecorderFor("controller-infinispan"),
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&infinispanv1.Infinispan{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=infinispan.org,namespace=infinispan-operator-system,resources=infinispans;infinispans/status;infinispans/finalizers,verbs=get;list;watch;create;update;patch

// +kubebuilder:rbac:groups=core,namespace=infinispan-operator-system,resources=persistentvolumeclaims;services;services/finalizers;endpoints;configmaps;pods;secrets,verbs=get;list;watch;create;update;delete;patch;deletecollection
// +kubebuilder:rbac:groups=core,namespace=infinispan-operator-system,resources=pods/logs,verbs=get
// +kubebuilder:rbac:groups=core,namespace=infinispan-operator-system,resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups=core;events.k8s.io,namespace=infinispan-operator-system,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=create;delete;update

// +kubebuilder:rbac:groups=apps,namespace=infinispan-operator-system,resources=deployments,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=apps,namespace=infinispan-operator-system,resources=replicasets,verbs=get
// +kubebuilder:rbac:groups=apps,namespace=infinispan-operator-system,resources=deployments/finalizers;statefulsets,verbs=get;list;watch;create;update;delete

// +kubebuilder:rbac:groups=networking.k8s.io,namespace=infinispan-operator-system,resources=ingresses,verbs=get;list;watch;create;delete;deletecollection;update
// +kubebuilder:rbac:groups=networking.k8s.io,namespace=infinispan-operator-system,resources=customresourcedefinitions;customresourcedefinitions/status,verbs=get;list

// +kubebuilder:rbac:groups=route.openshift.io,namespace=infinispan-operator-system,resources=routes;routes/custom-host,verbs=get;list;watch;create;delete;deletecollection;update

// +kubebuilder:rbac:groups=monitoring.coreos.com,namespace=infinispan-operator-system,resources=servicemonitors,verbs=get;list;watch;create;delete;update

// +kubebuilder:rbac:groups=core,resources=nodes;serviceaccounts,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions;customresourcedefinitions/status,verbs=get;list;watch
func (reconciler *IspnReconciler) Reconcile(ctx context.Context, ctrlRequest ctrl.Request) (ctrl.Result, error) {
	reqLogger := reconciler.log.WithValues("infinispan", ctrlRequest.NamespacedName)
	// Fetch the Infinispan instance
	instance := &infinispanv1.Infinispan{}
	if err := reconciler.Get(ctx, ctrlRequest.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			reconciler.log.Info("Infinispan CR not found")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, fmt.Errorf("unable to fetch Infinispan CR %w", err)
	}

	// Don't reconcile Infinispan CRs marked for deletion
	if instance.GetDeletionTimestamp() != nil {
		reqLogger.Info(fmt.Sprintf("Ignoring Infinispan CR '%s:%s' marked for deletion", instance.Namespace, instance.Name))
		return reconcile.Result{}, nil
	}

	// TODO construct pipeline with target and source operand version
	pipeline := pipelineBuilder.Builder().
		WithLogger(reqLogger).
		WithContextProvider(reconciler.contextProvider).
		Build()

	retry, err := pipeline.Process(ctx, instance)
	result := ctrl.Result{Requeue: retry}
	reqLogger.Info("Done", "retry", retry, "error", err)
	if retry {
		return result, err
	}
	return result, nil
}
