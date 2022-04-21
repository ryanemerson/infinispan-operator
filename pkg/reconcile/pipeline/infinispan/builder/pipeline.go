package builder

// TODO rename package to pipeline
import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/version"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	"github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan/handler/collect"
	"github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan/handler/configure"
	"github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan/handler/manage"
	"github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan/handler/provision"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
)

var _ pipeline.Pipeline = &impl{}

type impl struct {
	*pipeline.ContextProviderConfig
	ctxProvider     pipeline.ContextProvider
	handlers        []pipeline.Handler
	deployedVersion *version.Version
	targetVersion   *version.Version
}

func (i *impl) Process(ctx context.Context, infinispan *ispnv1.Infinispan) (retry bool, err error) {
	defer func() {
		if perr := recover(); perr != nil {
			retry = true
			err = fmt.Errorf("panic occurred: %v", perr)
		}
	}()
	i.ContextProviderConfig.Instance = infinispan
	ispnContext, err := i.ctxProvider.Get(ctx, i.ContextProviderConfig)
	if err != nil {
		return false, err
	}

	var status pipeline.FlowStatus
	for _, h := range i.handlers {
		invokeHandler(h, ispnContext)
		status = ispnContext.FlowStatus()
		if status.Stop {
			break
		}
	}
	err = ispnContext.Close()
	if err != nil {
		return true, err
	}
	return status.Retry, status.Err
}

func invokeHandler(h pipeline.Handler, ctx pipeline.Context) {
	defer func() {
		if err := recover(); err != nil {
			ctx.RetryProcessing(fmt.Errorf("panic occurred: %v", err))
		}
	}()
	h.Handle(ctx)
}

type builder impl

func (b *builder) WithAnnotations(annotations map[string]string) *builder {
	if len(annotations) > 0 {
		b.DefaultAnnotations = annotations
	}
	return b
}

func (b *builder) WithContextProvider(ctxProvider pipeline.ContextProvider) *builder {
	b.ctxProvider = ctxProvider
	return b
}

func (b *builder) WithDeployedVersion(v *version.Version) *builder {
	b.deployedVersion = v
	return b
}

func (b *builder) WithLabels(labels map[string]string) *builder {
	if len(labels) > 0 {
		b.DefaultLabels = labels
	}
	return b
}

func (b *builder) WithLogger(logger logr.Logger) *builder {
	b.Logger = logger
	return b
}

func (b *builder) WithSupportedTypes(types map[schema.GroupVersionKind]struct{}) *builder {
	b.SupportedTypes = types
	return b
}

func (b *builder) WithTargetVersion(v *version.Version) *builder {
	b.targetVersion = v
	return b
}

func (b *builder) Build() pipeline.Pipeline {
	// TODO init handlers based upon Version

	// No upgrade required
	if b.deployedVersion == nil {
		// TODO
	}
	handlers := handlerBuilder{
		handlers: make([]pipeline.HandlerFunc, 0),
	}

	// Apply default meta before doing anything else
	handlers.Add(manage.PrelimChecksCondition)

	// Collect Handlers
	handlers.Add(
		collect.UserAuthenticationSecret,
		collect.UserEncryptionSecrets,
		collect.UserConfigMap,
		collect.UserDefinedStorageClass,
		collect.AdminSecret,
	)

	// Upgrade handlers
	// TODO disable if Rolling Upgrades configured
	handlers.Add(
		manage.ScheduleGracefulShutdownUpgrade,
		manage.ExecuteGracefulShutdownUpgrade,
	)

	// Configuration Handlers
	handlers.Add(
		configure.Keystore,
		configure.Truststore,
		configure.InfinispanServer,
		configure.Logging,
		configure.AdminIdentities,
		configure.UserIdentities,
		configure.IdentitiesBatch,
	)

	// Provision Handlers
	handlers.Add(
		provision.UserAuthenticationSecret,
		provision.AdminSecret,
		provision.InfinispanSecuritySecret,
		provision.TruststoreSecret,
		provision.InfinispanConfigMap,
		provision.PingService,
		provision.AdminService,
		provision.ClusterService,
		provision.ExternalService,
		provision.ClusterStatefulSet,
	)

	handlers.Add(
		manage.StatefulSetRollingUpgrade,
		manage.WellFormedCondition,
	)

	handlers.AddEnvSpecific("MAKE_DATADIR_WRITABLE", "true", provision.AddChmodInitContainer)

	// Runtime Handlers

	b.handlers = handlers.Build()

	impl := impl(*b)
	return &impl
}

func Builder() *builder {
	return &builder{
		ContextProviderConfig: &pipeline.ContextProviderConfig{},
	}
}

type handlerBuilder struct {
	handlers []pipeline.HandlerFunc
}

func (h *handlerBuilder) Add(handlerFunc ...pipeline.HandlerFunc) *handlerBuilder {
	h.handlers = append(h.handlers, handlerFunc...)
	return h
}

func (h *handlerBuilder) AddFeatureSpecific(predicate bool, handlerFunc ...pipeline.HandlerFunc) *handlerBuilder {
	if predicate {
		return h.Add(handlerFunc...)
	}
	return h
}

func (h *handlerBuilder) AddEnvSpecific(envName, envValue string, handlerFunc ...pipeline.HandlerFunc) *handlerBuilder {
	if val, ok := os.LookupEnv(envName); ok && val == envValue {
		h.handlers = append(h.handlers, handlerFunc...)
	}
	return h
}

func (h *handlerBuilder) Build() []pipeline.Handler {
	handlers := make([]pipeline.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler
	}
	return handlers
}
