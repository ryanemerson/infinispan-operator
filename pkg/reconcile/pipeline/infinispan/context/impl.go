package context

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ pipeline.Context = &impl{}

func Provider(client client.Client, scheme *runtime.Scheme, kubernetes *kube.Kubernetes, eventRec record.EventRecorder) pipeline.ContextProvider {
	return &provider{
		Client:     client,
		scheme:     scheme,
		kubernetes: kubernetes,
		eventRec:   eventRec,
	}
}

type provider struct {
	client.Client
	scheme     *runtime.Scheme
	kubernetes *kube.Kubernetes
	eventRec   record.EventRecorder
}

func (p *provider) Get(ctx context.Context, logger logr.Logger, infinispan *ispnv1.Infinispan) (pipeline.Context, error) {
	return &impl{
		provider:    p,
		flowCtrl:    &flowCtrl{},
		ctx:         ctx,
		logger:      logger,
		instance:    infinispan,
		ispnConfig:  &pipeline.ConfigFiles{},
		secrets:     make(map[string]*secretResource),
		configmaps:  make(map[string]*configmapResource),
		statefulSet: make(map[string]*statefulsetResource),
	}, nil
}

// TODO rename contextImpl
type impl struct {
	*flowCtrl
	*provider
	ctx         context.Context
	logger      logr.Logger
	instance    *ispnv1.Infinispan
	ispnConfig  *pipeline.ConfigFiles
	secrets     map[string]*secretResource
	configmaps  map[string]*configmapResource
	statefulSet map[string]*statefulsetResource
}

func (i impl) Instance() *ispnv1.Infinispan {
	return i.instance
}

func (i impl) InfinispanClient() api.Infinispan {
	//TODO implement me
	panic("implement me")
}

func (i impl) ConfigFiles() *pipeline.ConfigFiles {
	return i.ispnConfig
}

func (i impl) NewCluster() bool {
	//TODO implement me
	panic("implement me")
}

func (i impl) Log() logr.Logger {
	return i.logger
}

func (i impl) EventRecorder() record.EventRecorder {
	return i.eventRec
}

func (i impl) SetControllerReference(controlled metav1.Object) error {
	return k8sctrlutil.SetControllerReference(i.instance, controlled, i.scheme)
}

func (i impl) SetCondition(condition *metav1.Condition) {
	//TODO implement me
	panic("implement me")
}

func (i impl) Close() error {
	if i.err != nil {
		// TODO handle error
		// Introduce new condition?
		// Only persist Infinispan to update CR
		return nil
	}
	//TODO implement me
	if err := i.persistSecrets(); err != nil {
		// TODO Only persist Infinispan Status on error?

	}

	if err := i.persistConfigMaps(); err != nil {
		// TODO Only persist Infinispan Status on error?
	}

	if err := i.persistStatefulSets(); err != nil {
		// TODO Only persist Infinispan Status on error?
	}
	return nil
}

func (i impl) LoadResource(name string, obj client.Object) error {
	key := types.NamespacedName{Namespace: i.instance.Namespace, Name: name}
	return i.Client.Get(i.ctx, key, obj)
}

func (i impl) createOrUpdate(obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)

	// Create an empty instance of the provided client.Object for retrieval so the passed object's definition is not overwritten
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = reflect.Indirect(val)
	}
	empty := reflect.New(val.Type()).Interface().(client.Object)

	if err := i.Client.Get(i.ctx, key, empty); err != nil {
		if errors.IsNotFound(err) {
			// The resource does not exist, so we create it
			return i.Create(i.ctx, obj)
		}
		return err
	}
	// Resource already exists, so update
	return i.Update(i.ctx, obj)
}

func (i impl) persistResource(pr pipeline.PersistableResource) error {
	if !pr.IsUpdated() {
		return nil
	}

	obj := pr.Object()
	if pr.IsUserCreated() {
		// If a secret was provided by the user then we can only update the resource
		if err := i.Update(i.ctx, obj); err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("unable to persist changes to '%s' %s: %w", obj.GetName(), obj.GetObjectKind(), err)
			}
		}
	} else {
		if err := i.createOrUpdate(obj); err != nil {
			return fmt.Errorf("unable to persist changes to '%s' %s: %w", obj.GetName(), obj.GetObjectKind(), err)
		}
	}
	return nil
}

func (i impl) persistSecrets() error {
	for _, secret := range i.secrets {
		if err := i.persistResource(secret); err != nil {
			return err
		}
	}
	return nil
}

func (i impl) persistConfigMaps() error {
	for _, configmap := range i.configmaps {
		if err := i.persistResource(configmap); err != nil {
			return err
		}
	}
	return nil
}

func (i impl) persistStatefulSets() error {
	for _, statefulset := range i.statefulSet {
		if err := i.persistResource(statefulset); err != nil {
			return err
		}
	}
	return nil
}

func (i impl) persistInfinispan() error {
	// Only persist Infinispan Status?
	// CR spec updates should be handled by user and webhooks
	return nil
}
