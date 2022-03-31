package context

import (
	"fmt"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type resources struct {
	*impl
}

func resourceKey(name string, gvk schema.GroupVersionKind) string {
	return fmt.Sprintf("%s.%s", name, gvk)
}

func (i *impl) Resources() pipeline.Resources {
	return &resources{i}
}

func (r resources) Define(obj client.Object) {
	key := resourceKey(obj.GetName(), obj.GetObjectKind().GroupVersionKind())
	r.resources[key] = obj
}

func (r resources) Load(name string, obj client.Object) error {
	key := resourceKey(name, obj.GetObjectKind().GroupVersionKind())
	if storedObj, ok := r.resources[key]; ok {
		obj = storedObj
	}
	obj.SetName(name)
	obj.SetNamespace(r.instance.Namespace)
	if err := r.Client.Get(r.ctx, types.NamespacedName{Namespace: r.instance.Namespace, Name: name}, obj); err != nil {
		return err
	}
	r.resources[key] = obj
	return nil
}

func (r resources) List(set map[string]string, list client.ObjectList) error {
	labelSelector := labels.SelectorFromSet(set)
	listOps := &client.ListOptions{Namespace: r.instance.Namespace, LabelSelector: labelSelector}
	return r.Client.List(r.ctx, list, listOps)
}
