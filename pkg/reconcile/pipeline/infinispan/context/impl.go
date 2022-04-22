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
	"github.com/r3labs/diff/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
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

func (p *provider) Get(ctx context.Context, config *pipeline.ContextProviderConfig) (pipeline.Context, error) {
	return &impl{
		provider:              p,
		flowCtrl:              &flowCtrl{},
		ContextProviderConfig: config,
		instance:              config.Instance, // TODO remove and just use value from config directly
		ctx:                   ctx,
		ispnConfig:            &pipeline.ConfigFiles{},
		resources:             make(map[string]resource),
	}, nil
}

// TODO rename contextImpl
type impl struct {
	*flowCtrl
	*provider
	*pipeline.ContextProviderConfig
	ctx        context.Context
	instance   *ispnv1.Infinispan
	ispnConfig *pipeline.ConfigFiles
	resources  map[string]resource
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
	return i.Logger
}

func (i impl) EventRecorder() record.EventRecorder {
	return i.eventRec
}

func (i impl) DefaultAnnotations() map[string]string {
	return i.ContextProviderConfig.DefaultAnnotations
}

func (i impl) DefaultLabels() map[string]string {
	return i.ContextProviderConfig.DefaultLabels
}

func (i impl) IsTypeSupported(gvk schema.GroupVersionKind) bool {
	_, ok := i.SupportedTypes[gvk]
	return ok
}

func (i impl) Close() error {
	if i.err != nil {
		//	// Only persist Infinispan Status to persist any errors represented in status.Conditions
		return i.updateStatus()
	}

	for _, resource := range i.resources {
		if resource.delete {
			if err := i.Delete(i.ctx, resource.Object); err != nil {
				if !errors.IsNotFound(err) {
					return fmt.Errorf("unable to delete '%s' %s: %w", resource.GetName(), resource.GetObjectKind(), err)
				}
			}
			continue
		}

		if err := i.createOrPatch(resource.Object); err != nil {
			// TODO add condition to describe persist resource error?
			return fmt.Errorf("unable to persist changes to '%s' %s: %w", resource.GetName(), resource.GetObjectKind(), err)
		}
	}
	return i.updateAll()
}

func (i impl) updateStatus() error {
	return i.update(func(ispn *ispnv1.Infinispan) {
		ispn.Status = i.instance.Status
	})
}

func (i impl) updateAll() error {
	return i.update(func(ispn *ispnv1.Infinispan) {
		ispn.ObjectMeta.Annotations = i.instance.ObjectMeta.Annotations
		ispn.ObjectMeta.Labels = i.instance.ObjectMeta.Labels
		ispn.Spec = i.instance.Spec
		ispn.Status = i.instance.Status
	})
}

func (i impl) update(update func(ispn *ispnv1.Infinispan)) error {
	loadedInstance := i.instance.DeepCopy()
	_, err := kube.CreateOrPatch(i.ctx, i.Client, loadedInstance, func() error {
		if loadedInstance.CreationTimestamp.IsZero() || loadedInstance.GetDeletionTimestamp() != nil {
			return errors.NewNotFound(schema.ParseGroupResource("infinispan.infinispan.org"), loadedInstance.Name)
		}
		update(loadedInstance)
		return nil
	})
	return err
}

func (i impl) createOrPatch(obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)

	// Create an empty instance of the provided client.Object for retrieval so the passed object's definition is not overwritten
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = reflect.Indirect(val)
	}
	objType := val.Type()
	existing := reflect.New(objType).Interface().(client.Object)

	if err := i.Client.Get(i.ctx, key, existing); err != nil {
		if errors.IsNotFound(err) {
			// The resource does not exist, so we create it
			return i.Create(i.ctx, obj)
		}
		return err
	}

	latest := reflect.New(objType).Interface().(client.Object)
	if err := kube.Merge(latest, existing, obj); err != nil {
		return fmt.Errorf("unable to merge existing and new resource: %w", err)
	}

	// Get the diff of the existing and latest resource definition
	changeLog, err := diff.Diff(existing, latest)
	if err != nil {
		return fmt.Errorf("unable to merge existing and new resource: %w", err)
	}

	// Only update the k8s resource if there's changes in the diff that we require
	changeLog = changeLog.FilterOut(strings.Fields("Status"))
	changeLog = changeLog.FilterOut(strings.Fields("ObjectMeta CreationTimestamp Time"))
	if len(changeLog) > 0 {
		if err = i.Client.Update(i.ctx, latest); err != nil {
			return err
		}
	}
	return nil
}