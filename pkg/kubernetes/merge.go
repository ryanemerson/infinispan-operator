package kubernetes

import (
	"fmt"
	"github.com/infinispan/infinispan-operator/pkg/mergo"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Merge(merged, left, right client.Object) error {
	leftType := reflect.ValueOf(left).Type()
	rightType := reflect.ValueOf(right).Type()
	if leftType != rightType {
		return fmt.Errorf("merge objects type mismatch. Left=%s, Right=%s", leftType, rightType)
	}

	leftUnstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(left.DeepCopyObject())
	if err != nil {
		return fmt.Errorf("unable to convert left object to unstructured: %w", err)
	}

	rightUnstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(right.DeepCopyObject())
	if err != nil {
		return fmt.Errorf("unable to convert right object to unstructured: %w", err)
	}

	// Merge the right changes into the left object so that the left fields always win
	if err = mergo.Merge(&leftUnstr, rightUnstr, mergo.WithSliceDeepCopy); err != nil {
		return fmt.Errorf("unable to merge unstructured objects: %s", err)
	}
	// Convert the unstructured content to the merged object
	return runtime.DefaultUnstructuredConverter.FromUnstructured(leftUnstr, merged)
}
