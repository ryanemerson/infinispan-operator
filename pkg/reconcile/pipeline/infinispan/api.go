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

// Pipeline for Infinispan reconciliation
type Pipeline interface {
	// Process the pipeline
	// Returns true if processing should be repeated and optional error if occurred
	// important: even if error occurred it might not be needed to retry processing
	Process(ctx context.Context) (bool, error)
}

// Handler an individual stage in the pipeline
type Handler interface {
	Handle(i *ispnv1.Infinispan, ctx Context)
}

type HandlerFunc func(i *ispnv1.Infinispan, ctx Context)

func (f HandlerFunc) Handle(i *ispnv1.Infinispan, ctx Context) {
	f(i, ctx)
}

// FlowStatus Pipeline flow control
type FlowStatus struct {
	Retry bool
	Stop  bool
	Err   error
}

func (f *FlowStatus) String() string {
	return fmt.Sprintf("Retry=%t, Stop=%t, Err=%s", f.Retry, f.Stop, f.Err.Error())
}

// ContextProvider interface used by Pipeline implementations to obtain a Context
type ContextProvider interface {
	Get(ctx context.Context, config *ContextProviderConfig) (Context, error)
}

type ContextProviderConfig struct {
	DefaultAnnotations map[string]string
	DefaultLabels      map[string]string
	Infinispan         *ispnv1.Infinispan
	Logger             logr.Logger
	SupportedTypes     map[schema.GroupVersionKind]struct{} // We only care about keys, so use struct{} as it requires 0 bytes
}

// Context of the pipeline, which is passed to each Handler
type Context interface {
	// InfinispanClient returns a client for the Operand servers
	// The client is created Lazily and cached per Pipeline execution to prevent repeated calls to retrieve the cluster pods
	// An error is thrown on initial client creation if the cluster pods can't be retrieved or don't exist.
	InfinispanClient() (ispnApi.Infinispan, error)

	// InfinispanClientForPod returns a client for the specific pod
	InfinispanClientForPod(podName string) ispnApi.Infinispan

	// ConfigFiles returns the ConfigFiles struct used to hold all configuration data required by the Operand
	ConfigFiles() *ConfigFiles

	// Resources interface provides convenience functions for interacting with kubernetes resources
	Resources() Resources

	// Ctx the Pipeline's context.Context that should be passed to any functions requiring a context
	Ctx() context.Context

	// Log the Infinispan request logger
	Log() logr.Logger

	// EventRecorder associated with the Infinispan controller
	EventRecorder() record.EventRecorder

	// Kubernetes exposes the underlying kubernetes client for when Resources doesn't provide the required functionality.
	// In general this method should be avoided if the same behaviour can be performed via the Resources interface
	Kubernetes() *kubernetes.Kubernetes

	// DefaultAnnotations defined for the Operator via ENV vars
	DefaultAnnotations() map[string]string

	// DefaultLabels defined for the Operator via ENV vars
	DefaultLabels() map[string]string

	// IsTypeSupported returns true if the GVK is supported on the kubernetes cluster
	IsTypeSupported(gvk schema.GroupVersionKind) bool

	// UpdateInfinispan updates the Infinispan CR resource being reconciled
	UpdateInfinispan() error

	// RetryProcessing indicates that the pipeline should stop once the current Handler has finished execution and
	// reconciliation should be requeued
	RetryProcessing(reason error)

	// Error Indicates that en error has occurred while processing the cluster
	Error(err error)

	// StopProcessing indicates that the pipeline should stop once the current Handler has finished execution
	StopProcessing()

	// Close the context and persist any changes to the Infinispan CR
	Close() error

	// FlowStatus the current status of the Pipeline
	FlowStatus() FlowStatus
}

// Resources interface that provides common functionality for interacting with Kubernetes resources
type Resources interface {
	// Create the passed object in the Infinispan namespace, setting the objects ControllerRef to the Infinispan CR if
	// setControllerRef is true
	Create(obj client.Object, setControllerRef bool) error
	// CreateOrUpdate the passed object in the Infinispan namespace, setting the objects ControllerRef to the Infinispan CR if
	// setControllerRef is true
	CreateOrUpdate(obj client.Object, setControllerRef bool, mutate func()) error
	// CreateOrPatch the passed object in the Infinispan namespace, setting the objects ControllerRef to the Infinispan CR if
	// setControllerRef is true
	CreateOrPatch(obj client.Object, setControllerRef bool, mutate func()) error
	// Delete the obj from the Infinispan namespace
	Delete(name string, obj client.Object) error
	// List resources in the Infinispan namespace using the passed set as a LabelSelector
	List(set map[string]string, list client.ObjectList) error
	// Load a resource from the Infinispan namespace
	Load(name string, obj client.Object, opts ...func(config *ResourcesConfig)) error
	// LoadGlobal loads a cluster scoped kubernetes resource
	LoadGlobal(name string, obj client.Object, opts ...func(config *ResourcesConfig)) error
	// SetControllerReference Set the controller reference of the passed object to the Infinispan CR being reconciled
	SetControllerReference(controlled metav1.Object) error
	// Update a kubernetes resource in the Infinispan namespace
	Update(obj client.Object) error
}

// ResourcesConfig config used by Resources implementations to control implementation behaviour
type ResourcesConfig struct {
	IgnoreNotFound  bool
	InvalidateCache bool
	RetryOnErr      bool
	SkipEventRec    bool
}

// IgnoreNotFound return nil when NotFound errors are present
func IgnoreNotFound(config *ResourcesConfig) {
	config.IgnoreNotFound = true
}

// RetryOnErr set Context#RetryProcessing(err) when an error is encountered
func RetryOnErr(config *ResourcesConfig) {
	config.RetryOnErr = true
}

// InvalidateCache ignore any cached resources and execute a new call to the api-server
func InvalidateCache(config *ResourcesConfig) {
	config.InvalidateCache = true
}

// SkipEventRec do not send an event to the EventRecorder in the event of an error
func SkipEventRec(config *ResourcesConfig) {
	config.SkipEventRec = true
}

// ConfigFiles is used to hold all configuration required by the Operand in provisioned resources
type ConfigFiles struct {
	ServerConfig    string
	ZeroConfig      string
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

var (
	ServiceTypes      = []schema.GroupVersionKind{ServiceGVK, RouteGVK, IngressGVK}
	ServiceGVK        = corev1.SchemeGroupVersion.WithKind("Service")
	RouteGVK          = routev1.SchemeGroupVersion.WithKind("Route")
	IngressGVK        = ingressv1.SchemeGroupVersion.WithKind("Ingress")
	ServiceMonitorGVK = monitoringv1.SchemeGroupVersion.WithKind("ServiceMonitor")
)
