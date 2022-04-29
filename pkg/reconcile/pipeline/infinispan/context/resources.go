package context

import (
	"fmt"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

func (r resources) Load(name string, obj client.Object, opts ...func(config *pipeline.ResourcesConfig)) error {
	// TODO are these necessary?
	obj.SetName(name)
	obj.SetNamespace(r.instance.Namespace)

	loadFn := func() error {
		return r.Client.Get(r.ctx, types.NamespacedName{Namespace: r.instance.Namespace, Name: name}, obj)
	}

	return exec(
		r.load(name, obj, loadFn, opts...),
	)
}

func (r resources) LoadGlobal(name string, obj client.Object, opts ...func(config *pipeline.ResourcesConfig)) error {
	loadFn := func() error {
		return r.Client.Get(r.ctx, types.NamespacedName{Name: name}, obj)
	}

	return exec(
		r.load(name, obj, loadFn, opts...),
	)
}

func (r resources) load(name string, obj client.Object, load func() error, opts ...func(config *pipeline.ResourcesConfig)) error {
	config := &pipeline.ResourcesConfig{}
	for _, opt := range opts {
		opt(config)
	}

	handleErr := func(err error) error {
		if err == nil {
			return nil
		}

		isNotFound := errors.IsNotFound(err)
		if isNotFound && config.IgnoreNotFound {
			return nil
		}

		if config.RetryOnErr {
			r.RetryProcessing(err)
		}

		if isNotFound && !config.SkipEventRec {
			msg := fmt.Sprintf("%s resource '%s' not ready", reflect.TypeOf(obj).Elem().Name(), name)
			r.Log().Info(msg)
			r.EventRecorder().Event(r.instance, corev1.EventTypeWarning, "ResourceNotReady", msg)
		}
		return err
	}

	key := r.resourceKey(name, obj)
	if !config.InvalidateCache {
		if storedObj, ok := r.resources[key]; ok {
			// Reflection trickery so that the passed obj reference is updated to the stored pointer
			reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(storedObj).Elem())
			return nil
		}
	}
	if err := load(); err != nil {
		return handleErr(err)
	}
	r.resources[key] = obj
	return nil
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
