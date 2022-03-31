package infinispan

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	ispnApi "github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
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

	// TODO move to Resources?
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
	Define(obj client.Object)
	Load(name string, obj client.Object) error
	List(set map[string]string, list client.ObjectList) error
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
