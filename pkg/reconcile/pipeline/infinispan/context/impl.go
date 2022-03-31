package context

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	"github.com/infinispan/infinispan-operator/pkg/http/curl"
	ispnClient "github.com/infinispan/infinispan-operator/pkg/infinispan/client"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	// TODO lookup podList and create new curl
	// TODO cache created client to prevent pod list lookup everytime?
	panic("implement me")
}

func (i impl) InfinispanClientForPod(podName string) api.Infinispan {
	curlClient := i.curlClient(podName)
	return ispnClient.New(curlClient)
}

func (i impl) curlClient(podName string) *curl.Client {
	return curl.New(curl.Config{
		Credentials: &curl.Credentials{
			Username: i.ispnConfig.AdminIdentities.Username,
			Password: i.ispnConfig.AdminIdentities.Password,
		},
		// TODO use constant
		Container: "infinispan",
		Podname:   podName,
		Namespace: i.instance.Namespace,
		Protocol:  "http",
		Port:      consts.InfinispanAdminPort,
	}, i.kubernetes)
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
	if err := i.persistSecrets(); err != nil {
		// TODO add condition to describe persist resource errors?
		// Update status only
		return err
	}

	if err := i.persistConfigMaps(); err != nil {
		return err
	}

	if err := i.persistStatefulSets(); err != nil {
		// TODO Only persist Infinispan Status on error?
	}

	// TODO compare initial spec with new one to see if update required?
	// Update any changes to the Infinispan CR
	if err := i.Update(i.ctx, i.instance); err != nil {
		return err
	}
	// Only update the status if a CR update succeeds
	return i.updateStatus()
}

// TODO just execute inline?
func (i impl) updateStatus() error {
	return i.Status().Update(i.ctx, i.Instance())
}

func (i impl) LoadResource(name string, obj client.Object) error {
	key := types.NamespacedName{Namespace: i.instance.Namespace, Name: name}
	return i.Client.Get(i.ctx, key, obj)
}

func (i impl) ListResources(set map[string]string, list client.ObjectList) error {
	labelSelector := labels.SelectorFromSet(set)
	listOps := &client.ListOptions{Namespace: i.instance.Namespace, LabelSelector: labelSelector}
	return i.Client.List(i.ctx, list, listOps)
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
