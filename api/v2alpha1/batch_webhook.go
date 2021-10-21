package v2alpha1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var batchlog = logf.Log.WithName("webhook").WithName("Batch")

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

// Change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-infinispan-infinispan-org-v2alpha1-batch,mutating=false,failurePolicy=fail,sideEffects=None,groups=infinispan.infinispan.org,resources=batches,verbs=create;update,versions=v2alpha1,name=vbatch.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &Batch{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Batch) ValidateCreate() error {
	batchlog.Info("validate create", "name", r.Name)

	var allErrs field.ErrorList
	if r.Spec.ConfigMap == nil && r.Spec.Config == nil {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("configMap"), "'Spec.config' OR 'spec.ConfigMap' must be configured"))
	} else if r.Spec.ConfigMap != nil && r.Spec.Config != nil {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("configMap"), "At most one of ['Spec.config', 'spec.ConfigMap'] must be configured"))
	}
	return r.StatusError(allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Batch) ValidateUpdate(old runtime.Object) error {
	batchlog.Info("validate update", "name", r.Name)

	var allErrs field.ErrorList
	oldBatch := old.(*Batch)
	if r.Spec.Cluster != oldBatch.Spec.Cluster {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("config"), "'spec.config' cannot be updated after initial Batch creation"))
	}
	return r.StatusError(allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Batch) ValidateDelete() error {
	batchlog.Info("validate delete", "name", r.Name)
	// Not enabled
	return nil
}

func (r *Batch) StatusError(allErrs field.ErrorList) error {
	if len(allErrs) != 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: GroupVersion.Group, Kind: "Batch"},
			r.Name, allErrs)
	}
	return nil
}
