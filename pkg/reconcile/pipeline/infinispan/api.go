package infinispan

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	ispnApi "github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
	"github.com/infinispan/infinispan-operator/pkg/kubernetes"
	routev1 "github.com/openshift/api/route/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	ingressv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO add constants file for common logic

// Pipeline for Infinispan reconciliation
type Pipeline interface {
	// Process the pipeline
	// Returns true if processing should be repeated and optional error if occurred
	// important: even if error occurred it might not be needed to retry processing
	Process(ctx context.Context) (bool, error)
}

// A pipeline stage
type Handler interface {
	Handle(i *ispnv1.Infinispan, ctx Context)
}

type HandlerFunc func(i *ispnv1.Infinispan, ctx Context)

func (f HandlerFunc) Handle(i *ispnv1.Infinispan, ctx Context) {
	f(i, ctx)
}

// Pipeline flow control
type FlowStatus struct {
	Retry bool
	Stop  bool
	Err   error
}

func (f *FlowStatus) String() string {
	return fmt.Sprintf("Retry=%t, Stop=%t, Err=%s", f.Retry, f.Stop, f.Err.Error())
}

// Pipeline context passed to each handler
type Context interface {
	Instance() *ispnv1.Infinispan

	// InfinispanClient operand specific client for Infinispan servers
	InfinispanClient() (ispnApi.Infinispan, error)

	InfinispanClientForPod(podName string) ispnApi.Infinispan

	ConfigFiles() *ConfigFiles

	Resources() Resources

	Ctx() context.Context

	Log() logr.Logger

	EventRecorder() record.EventRecorder

	Kubernetes() *kubernetes.Kubernetes

	DefaultAnnotations() map[string]string

	DefaultLabels() map[string]string

	IsTypeSupported(gvk schema.GroupVersionKind) bool

	UpdateStatus() error

	// Indicates that the cluster should be retried at some later time
	// The current processing stops and context gets closed
	RetryProcessing(reason error)

	// Indicates that en error has occurred while processing the cluster
	Error(err error)

	// Stops processing
	StopProcessing()

	// Closes the context, persisting changed resources
	// Returns error if occurrs
	Close() error

	FlowStatus() FlowStatus
}

type ConfigFiles struct {
	ServerConfig    string
	Log4j           string
	UserIdentities  []byte
	AdminIdentities *AdminIdentities
	IdentitiesBatch string
	UserConfig      UserConfig
	Keystore        *Keystore
	Truststore      *Truststore
}

type UserConfig struct {
	ServerConfig         string
	ServerConfigFileName string
	Log4j                string
}

type AdminIdentities struct {
	Username       string
	Password       string
	IdentitiesFile []byte
	CliProperties  string
}

type Keystore struct {
	Alias    string
	File     []byte
	PemFile  []byte
	Password string
	Path     string
}

type Truststore struct {
	File     []byte
	Password string
}

type Resources interface {
	Create(obj client.Object, setControllerRef bool) error
	CreateOrUpdate(obj client.Object, setControllerRef bool, mutate func()) error
	CreateOrPatch(obj client.Object, setControllerRef bool, mutate func()) error
	Delete(name string, obj client.Object) error
	List(set map[string]string, list client.ObjectList) error
	Load(name string, obj client.Object) error
	LoadGlobal(name string, obj client.Object) error
	// SetControllerReference Set the controller reference of the passed object to the Infinispan CR being reconciled
	SetControllerReference(controlled metav1.Object) error
	Update(obj client.Object) error
}

type ContextProvider interface {
	Get(ctx context.Context, config *ContextProviderConfig) (Context, error)
}

type ContextProviderConfig struct {
	DefaultAnnotations map[string]string
	DefaultLabels      map[string]string
	Instance           *ispnv1.Infinispan
	Logger             logr.Logger
	SupportedTypes     map[schema.GroupVersionKind]struct{} // We only care about keys, so use struct{} as it requires 0 bytes
}

var (
	ServiceTypes      = []schema.GroupVersionKind{ServiceGVK, RouteGVK, IngressGVK}
	ServiceGVK        = corev1.SchemeGroupVersion.WithKind("Service")
	RouteGVK          = routev1.SchemeGroupVersion.WithKind("Route")
	IngressGVK        = ingressv1.SchemeGroupVersion.WithKind("Ingress")
	ServiceMonitorGVK = monitoringv1.SchemeGroupVersion.WithKind("ServiceMonitor")
)
