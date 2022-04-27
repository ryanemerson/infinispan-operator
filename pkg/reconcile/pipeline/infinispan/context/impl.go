package context

import (
	"context"
	"github.com/go-logr/logr"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	"github.com/infinispan/infinispan-operator/pkg/http/curl"
	ispnClient "github.com/infinispan/infinispan-operator/pkg/infinispan/client"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	return i.updateStatus()
}

func (i impl) updateStatus() error {
	return i.update(func(ispn *ispnv1.Infinispan) {
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
