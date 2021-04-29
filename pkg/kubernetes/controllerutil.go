package kubernetes

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	consts "github.com/infinispan/infinispan-operator/pkg/controller/constants"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// OperationResultUpdatedStatus means that an existing resource and its status is updated
	OperationResultUpdatedStatus controllerutil.OperationResult = "updatedStatus"
	// OperationResultUpdatedStatusOnly means that only an existing status is updated
	OperationResultUpdatedStatusOnly controllerutil.OperationResult = "updatedStatusOnly"
)

// CreateOrPatch creates or patches the given object in the Kubernetes
// cluster. The object's desired state must be reconciled with the before
// state inside the passed in callback MutateFn.
//
// The MutateFn is called regardless of creating or updating an object.
//
// It returns the executed operation and an error.
//
// Ported from https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/controller/controllerutil/controllerutil.go
func CreateOrPatch(ctx context.Context, c client.Client, obj runtime.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}
	if err := c.Get(ctx, key, obj); err != nil {
		if !errors.IsNotFound(err) {
			return controllerutil.OperationResultNone, err
		}
		if f != nil {
			if err := mutate(f, key, obj); err != nil {
				return controllerutil.OperationResultNone, err
			}
		}
		if err := c.Create(ctx, obj); err != nil {
			return controllerutil.OperationResultNone, err
		}
		return controllerutil.OperationResultCreated, nil
	}

	// Create patches for the object and its possible status.
	objPatch := client.MergeFrom(obj.DeepCopyObject())
	statusPatch := client.MergeFrom(obj.DeepCopyObject())

	// Create a copy of the original object as well as converting that copy to
	// unstructured data.
	before, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj.DeepCopyObject())
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	// Attempt to extract the status from the resource for easier comparison later
	beforeStatus, hasBeforeStatus, err := unstructured.NestedFieldCopy(before, "status")
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	// If the resource contains a status then remove it from the unstructured
	// copy to avoid unnecessary patching later.
	if hasBeforeStatus {
		unstructured.RemoveNestedField(before, "status")
	}

	// Mutate the original object.
	if f != nil {
		if err := mutate(f, key, obj); err != nil {
			return controllerutil.OperationResultNone, err
		}
	}

	// Convert the resource to unstructured to compare against our before copy.
	after, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	// Attempt to extract the status from the resource for easier comparison later
	afterStatus, hasAfterStatus, err := unstructured.NestedFieldCopy(after, "status")
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	// If the resource contains a status then remove it from the unstructured
	// copy to avoid unnecessary patching later.
	if hasAfterStatus {
		unstructured.RemoveNestedField(after, "status")
	}

	result := controllerutil.OperationResultNone

	if !reflect.DeepEqual(before, after) {
		// Only issue a Patch if the before and after resources (minus status) differ
		if err := c.Patch(ctx, obj, objPatch); err != nil {
			return result, err
		}
		result = controllerutil.OperationResultUpdated
	}

	if (hasBeforeStatus || hasAfterStatus) && !reflect.DeepEqual(beforeStatus, afterStatus) {
		// Only issue a Status Patch if the resource has a status and the beforeStatus
		// and afterStatus copies differ
		if result == controllerutil.OperationResultUpdated {
			// If Status was replaced by Patch before, set it to afterStatus
			objectAfterPatch, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
			if err != nil {
				return result, err
			}
			if err = unstructured.SetNestedField(objectAfterPatch, afterStatus, "status"); err != nil {
				return result, err
			}
			// If Status was replaced by Patch before, restore patched structure to the obj
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(objectAfterPatch, obj); err != nil {
				return result, err
			}
		}
		if err := c.Status().Patch(ctx, obj, statusPatch); err != nil {
			return result, err
		}
		if result == controllerutil.OperationResultUpdated {
			result = OperationResultUpdatedStatus
		} else {
			result = OperationResultUpdatedStatusOnly
		}
	}

	return result, nil
}

// mutate wraps a MutateFn and applies validation to its result
func mutate(f controllerutil.MutateFn, key client.ObjectKey, obj runtime.Object) error {
	if err := f(); err != nil {
		return err
	}
	if newKey, err := client.ObjectKeyFromObject(obj); err != nil || key != newKey {
		return fmt.Errorf("MutateFn cannot mutate object name and/or object namespace")
	}
	return nil
}

// LookupResource lookup for resource to be created by separate resource controller
func LookupResource(name, namespace string, resource runtime.Object, client client.Client, logger logr.Logger) (*reconcile.Result, error) {
	err := client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, resource)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("%s resource '%s' not ready", reflect.TypeOf(resource).Elem().Name(), name))
			return &reconcile.Result{RequeueAfter: consts.DefaultWaitOnCreateResource}, nil
		}
		return &reconcile.Result{}, err
	}
	return nil, nil
}

// getOperatorNamespace returns the operator namespace or the WATCH_NAMESPACE
// value if the operator is running outside the cluster.
// "OSDK_FORCE_RUN_MODE"="local" must be set if running locally (not in the cluster)
func GetOperatorNamespace() (string, error) {
	operatorNs, err := k8sutil.GetOperatorNamespace()
	// This makes everything works even running outside the cluster
	if err == k8sutil.ErrRunLocal {
		var operatorWatchNs string
		operatorWatchNs, err = k8sutil.GetWatchNamespace()
		if operatorWatchNs != "" {
			operatorNs = strings.Split(operatorWatchNs, ",")[0]
		}
	}
	return operatorNs, err
}

func LookupOperatorTokenSecret(client client.Client, logger logr.Logger) (*corev1.Secret, error) {
	operatorName, err := k8sutil.GetOperatorName()
	if err != nil {
		return nil, err
	}

	operatorNamespace, err := GetOperatorNamespace()
	if err != nil {
		return nil, err
	}

	ctx := context.TODO()
	serviceAccount := &corev1.ServiceAccount{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: operatorNamespace, Name: operatorName}, serviceAccount); err != nil {
		return nil, fmt.Errorf("Unable to retrieve operator ServiceAccount: %w", err)
	}

	var secretRef *corev1.ObjectReference
	for _, s := range serviceAccount.Secrets {
		if strings.Contains(s.Name, "-token-") {
			secretRef = &s
		}
	}

	if secretRef == nil {
		return nil, fmt.Errorf("Unable to find operator token secret")
	}

	secret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: operatorNamespace, Name: secretRef.Name}, secret); err != nil {
		return nil, fmt.Errorf("Unable to retrieve operator token Secret: %w", err)
	}
	return secret, err
}
