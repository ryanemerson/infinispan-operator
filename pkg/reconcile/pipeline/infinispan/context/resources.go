package context

import (
	"fmt"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	k8sctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type resources struct {
	*impl
}

type resource struct {
	client.Object
	delete bool
}

func (r resources) resourceKey(name string, obj client.Object) string {
	gvk, err := apiutil.GVKForObject(obj, r.scheme)
	if err != nil {
		// Panic so that we don't have to handle errors for all Resources methods
		// Panic is caught by the pipeline handler and logged, so the Operator won't terminate
		panic(err)
	}
	return fmt.Sprintf("%s.%s", name, gvk)
}

func (i *impl) Resources() pipeline.Resources {
	return &resources{i}
}

func (r resources) Define(obj client.Object, setControllerRef bool) {
	if setControllerRef {
		if err := r.SetControllerReference(obj); err != nil {
			// Panic so that we don't have to handle errors for all Resources methods
			// Panic is caught by the pipeline handler and logged, so the Operator won't terminate
			panic(err)
		}
	}

	key := r.resourceKey(obj.GetName(), obj)
	r.resources[key] = resource{
		Object: obj,
	}
}

func (r resources) Load(name string, obj client.Object) error {
	return r.loadAndCache(name, obj, r.LoadWithNoCaching)
}

func (r resources) LoadGlobal(name string, obj client.Object) error {
	return r.loadAndCache(name, obj, r.LoadGlobalWithNoCaching)
}

func (r resources) loadAndCache(name string, obj client.Object, load func(string, client.Object) error) error {
	key := r.resourceKey(name, obj)
	if storedObj, ok := r.resources[key]; ok {
		// Reflection trickery so that the passed obj reference is updated to the stored pointer
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(storedObj.Object).Elem())
		return nil
	}
	if err := load(name, obj); err != nil {
		return err
	}
	r.resources[key] = resource{
		Object: obj,
	}
	return nil
}

func (r resources) LoadWithNoCaching(name string, obj client.Object) error {
	// TODO are these necessary?
	obj.SetName(name)
	obj.SetNamespace(r.instance.Namespace)
	return r.Client.Get(r.ctx, types.NamespacedName{Namespace: r.instance.Namespace, Name: name}, obj)
}

func (r resources) LoadGlobalWithNoCaching(name string, obj client.Object) error {
	return r.Client.Get(r.ctx, types.NamespacedName{Name: name}, obj)
}

func (r resources) List(set map[string]string, list client.ObjectList) error {
	labelSelector := labels.SelectorFromSet(set)
	listOps := &client.ListOptions{Namespace: r.instance.Namespace, LabelSelector: labelSelector}
	return r.Client.List(r.ctx, list, listOps)
}

func (r resources) MarkForDeletion(obj client.Object) {
	key := r.resourceKey(obj.GetName(), obj)
	if storedObj, ok := r.resources[key]; ok {
		storedObj.delete = true
		return
	}
	r.resources[key] = resource{
		Object: obj,
		delete: true,
	}
}

func (r resources) SetControllerReference(controlled metav1.Object) error {
	return k8sctrlutil.SetControllerReference(r.instance, controlled, r.scheme)
}
