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
	"os"
)

var _ pipeline.Pipeline = &impl{}

type impl struct {
	logger          logr.Logger
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
	ispnContext, err := i.ctxProvider.Get(ctx, i.logger, infinispan)
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

func (b *builder) WithLogger(logger logr.Logger) *builder {
	b.logger = logger
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

	// Collect Handlers
	handlers.Add(
		collect.UserAuthenticationSecret,
		collect.UserEncryptionSecrets,
		collect.UserConfigMap,
		collect.UserDefinedStorageClass,
		collect.AdminSecret,
	)

	// Configuration Handlers
	handlers.Add(
		configure.Infinispan,
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
		provision.InfinispanConfigMap,
		provision.PingService,
		provision.AdminService,
		provision.ClusterService,
		provision.ExternalService,
		provision.ClusterStatefulSet,
	)

	// Manage Handlers
	handlers.Add(
		manage.PrelimChecksCondition,
		manage.UpgradeConditionTrue,
		manage.WellFormedCondition,
		manage.ScheduleUpgrade,
	)
	handlers.AddEnvSpecific("MAKE_DATADIR_WRITABLE", "true", provision.AddChmodInitContainer)

	// Runtime Handlers

	b.handlers = handlers.Build()

	impl := impl(*b)
	return &impl
}

func Builder() *builder {
	return &builder{}
}

type handlerBuilder struct {
	handlers []pipeline.HandlerFunc
}

func (h *handlerBuilder) Add(handlerFunc ...pipeline.HandlerFunc) *handlerBuilder {
	h.handlers = append(h.handlers, handlerFunc...)
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
