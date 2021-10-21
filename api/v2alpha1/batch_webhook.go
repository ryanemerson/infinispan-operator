package v2alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var batchlog = logf.Log.WithName("batch-resource")

func (r *Batch) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-infinispan-infinispan-org-v2alpha1-batch,mutating=true,failurePolicy=fail,sideEffects=None,groups=infinispan.infinispan.org,resources=batches,verbs=create;update,versions=v2alpha1,name=mbatch.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &Batch{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Batch) Default() {
	batchlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-infinispan-infinispan-org-v2alpha1-batch,mutating=false,failurePolicy=fail,sideEffects=None,groups=infinispan.infinispan.org,resources=batches,verbs=create;update,versions=v2alpha1,name=vbatch.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &Batch{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Batch) ValidateCreate() error {
	batchlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Batch) ValidateUpdate(old runtime.Object) error {
	batchlog.Info("validate update", "name", r.Name)

	oldBatch := old.(*Batch)
	if r.Spec.Cluster != oldBatch.Spec.Cluster {
		return fmt.Errorf("unable to update immutable field spec.Cluster")
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Batch) ValidateDelete() error {
	batchlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
