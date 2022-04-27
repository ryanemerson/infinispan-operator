package context

import (
	"fmt"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	k8sctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type resources struct {
	*impl
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

func (r resources) Create(obj client.Object, setControllerRef bool) error {
	if setControllerRef {
		if err := r.SetControllerReference(obj); err != nil {
			return err
		}
	}
	return exec(
		r.Client.Create(r.ctx, obj),
	)
}

func (r resources) CreateOrUpdate(obj client.Object, setControllerRef bool, mutate func()) error {
	_, err := k8sctrlutil.CreateOrUpdate(r.ctx, r.Client, obj, func() error {
		if mutate != nil {
			mutate()
		}
		if setControllerRef {
			return r.SetControllerReference(obj)
		}
		return nil
	})
	return exec(err)
}

func (r resources) CreateOrPatch(obj client.Object, setControllerRef bool, mutate func()) error {
	_, err := kube.CreateOrPatch(r.ctx, r.Client, obj, func() error {
		if mutate != nil {
			mutate()
		}
		if setControllerRef {
			return r.SetControllerReference(obj)
		}
		return nil
	})
	return exec(err)
}

func (r resources) Delete(name string, obj client.Object) error {
	obj.SetName(name)
	obj.SetNamespace(r.instance.Namespace)
	return exec(
		client.IgnoreNotFound(
			r.Client.Delete(r.ctx, obj),
		),
	)
}

func (r resources) List(set map[string]string, list client.ObjectList) error {
	labelSelector := labels.SelectorFromSet(set)
	listOps := &client.ListOptions{Namespace: r.instance.Namespace, LabelSelector: labelSelector}
	return exec(
		r.Client.List(r.ctx, list, listOps),
	)
}

func (r resources) Load(name string, obj client.Object) error {
	// TODO are these necessary?
	obj.SetName(name)
	obj.SetNamespace(r.instance.Namespace)
	return exec(
		r.Client.Get(r.ctx, types.NamespacedName{Namespace: r.instance.Namespace, Name: name}, obj),
	)
}

func (r resources) LoadGlobal(name string, obj client.Object) error {
	return exec(
		r.Client.Get(r.ctx, types.NamespacedName{Name: name}, obj),
	)
}

func (r resources) SetControllerReference(controlled metav1.Object) error {
	return exec(
		k8sctrlutil.SetControllerReference(r.instance, controlled, r.scheme),
	)
}

func (r resources) Update(obj client.Object) error {
	return exec(
		r.Client.Update(r.ctx, obj),
	)
}

// Wrapper for all executions that allows error debugging in a single place
func exec(err error) error {
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	return err
}
