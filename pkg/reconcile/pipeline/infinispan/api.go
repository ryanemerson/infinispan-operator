package infinispan

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	ispnApi "github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
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
	// Process given Infinispan CR
	// Returns true if processing should be repeated and optional error if occurred
	// important: even if error occurred it might not be needed to retry processing
	Process(ctx context.Context, infinispan *ispnv1.Infinispan) (bool, error)
}

// A pipeline stage
type Handler interface {
	Handle(ctx Context)
}

type HandlerFunc func(ctx Context)

func (f HandlerFunc) Handle(ctx Context) {
	f(ctx)
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
	InfinispanClient() ispnApi.Infinispan

	InfinispanClientForPod(podName string) ispnApi.Infinispan

	ConfigFiles() *ConfigFiles

	Resources() Resources

	// TODO remove?
	// Return true if StatefulSet doesn't exist yet
	NewCluster() bool

	Log() logr.Logger

	EventRecorder() record.EventRecorder

	DefaultAnnotations() map[string]string

	DefaultLabels() map[string]string

	IsTypeSupported(gvk schema.GroupVersionKind) bool

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
	Define(obj client.Object, setControllerRef bool)
	// TODO add Get. Like Load, except that no error is returned and a panic is thrown if the resource does not exist
	// Use for actions in a pipeline that have a hard requirement on a resource that should already have been loaded by a prior step?
	// OR just call LOAD as required and remove collect stage entirely?
	// The collection and configure packages can be merged into one?
	// Loads a resource from the Infinispan namespace
	Load(name string, obj client.Object) error
	// Loads a globally scoped resource
	LoadGlobal(name string, obj client.Object) error
	LoadGlobalWithNoCaching(name string, obj client.Object) error
	LoadWithNoCaching(name string, obj client.Object) error
	List(set map[string]string, list client.ObjectList) error
	MarkForDeletion(obj client.Object)
	// Set the controller reference of the passed object to the Infinispan CR being reconciled
	SetControllerReference(controlled metav1.Object) error
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
