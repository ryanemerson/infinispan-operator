package infinispan

import (
	"context"
	"github.com/go-logr/logr"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	ispnApi "github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

// Pipeline context passed to each handler
type Context interface {
	Instance() *ispnv1.Infinispan

	// InfinispanClient operand specific client for Infinispan servers
	InfinispanClient() ispnApi.Infinispan

	ConfigFiles() *ConfigFiles

	Resources() Resources

	// Return true if StatefulSet doesn't exist yet
	NewCluster() bool

	Log() logr.Logger

	EventRecorder() record.EventRecorder

	// Load a generic resource from the cluster namespace
	LoadResource(name string, obj client.Object) error

	// Set the controller reference of the passed object to the Infinispan CR being reconciled
	SetControllerReference(controlled metav1.Object) error

	// Sets context condition
	// TODO add condition to show reconcile errors
	SetCondition(condition *metav1.Condition)

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
	Truststore      *Keystore
}

type UserConfig struct {
	ServerConfig         string
	ServerConfigEncoding string
	Log4j                string
}

type AdminIdentities struct {
	Username       string
	Password       string
	IdentitiesFile []byte
	CliProperties  string
}

type Keystore struct {
	File     []byte
	Alias    string
	Password string
}

type Resources interface {
	Secrets() Secrets
	ConfigMaps() ConfigMaps
	StatefulSets() StatefulSets
}

type Secrets interface {
	Get(name string) *corev1.Secret
	Define(secret *corev1.Secret)
	Load(name string) (*corev1.Secret, error)
}

type ConfigMaps interface {
	Get(name string) *corev1.ConfigMap
	Define(configmap *corev1.ConfigMap)
	Load(name string) (*corev1.ConfigMap, error)
}

type StatefulSets interface {
	Get(name string) *appsv1.StatefulSet
	Define(statefulset *appsv1.StatefulSet)
	Load(name string) (*appsv1.StatefulSet, error)
}

type PersistableResource interface {
	Object() client.Object
	IsUpdated() bool
	IsUserCreated() bool
}

type Secret interface {
	PersistableResource
	Definition() *corev1.Secret
}

type ConfigMap interface {
	PersistableResource
	Definition() *corev1.ConfigMap
}

type StatefulSet interface {
	PersistableResource
	Definition() *appsv1.StatefulSet
}

// TODO add StatefulSet interface?
// Could add additional methods to determine if Rolling update is possible?
// Is there a need for the Hashes anymore? Can we just use Secret/ConfigMap isUpdated?
// IsUpdated won't work if partial reconciliation where Secret/ConfigMap updates, but StatefulSet hasn't yet

// TODO is this necessary or do
// Provides context for a given Infinispan
type ContextProvider interface {
	Get(ctx context.Context, logger logr.Logger, infinispan *ispnv1.Infinispan) (Context, error)
}

// Pipeline
// 1. User resources
