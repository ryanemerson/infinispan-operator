package context

import (
	"fmt"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type resources struct {
	*impl
}

type resource struct {
	resource          client.Object
	persistedResource client.Object
}

func resourceKey(name string, object client.Object) string {
	return fmt.Sprintf("%s.%s", name, object.GetObjectKind().GroupVersionKind().GroupKind())
}

func (r resource) Object() client.Object {
	return r.resource
}

func (r resource) IsUpdated() bool {
	return !reflect.DeepEqual(r.persistedResource, r.resource)
}

func (i *impl) Resources() pipeline.Resources {
	return &resources{i}
}

func (r resources) Get(name string, object client.Object) bool {
	key := resourceKey(name, object)
	resource, ok := r.resources[key]
	if ok {
		// TODO check this works as expected
		object = resource.Object()
	}
	return ok
}

func (r resources) Define(object client.Object) {
	key := resourceKey(object.GetName(), object)
	r.resources[key] = &resource{
		resource: object,
	}
}

func (r resources) Load(name string, obj client.Object) error {
	obj.SetName(name)
	obj.SetNamespace(r.instance.Namespace)
	if err := r.LoadResource(name, obj); err != nil {
		return err
	}

	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}

	jsonCopy := runtime.DeepCopyJSON(unstructuredObj)

	// Get the type of the resource requested and create an empty instance so that we can store the representation as client.Object
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = reflect.Indirect(val)
	}
	objCopy := reflect.New(val.Type()).Interface().(client.Object)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(jsonCopy, objCopy); err != nil {
		return err
	}

	key := resourceKey(name, obj)
	r.resources[key] = &resource{
		persistedResource: obj,
		resource:          objCopy,
	}
	return nil
}

func (r resources) List(set map[string]string, list client.ObjectList) error {
	labelSelector := labels.SelectorFromSet(set)
	listOps := &client.ListOptions{Namespace: r.instance.Namespace, LabelSelector: labelSelector}
	return r.Client.List(r.ctx, list, listOps)
}
