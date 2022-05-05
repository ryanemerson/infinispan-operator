package pipeline

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	"github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan/handler/configure"
	"github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan/handler/manage"
	"github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan/handler/provision"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"runtime/debug"
	"time"
)

var _ pipeline.Pipeline = &impl{}

type impl struct {
	*pipeline.ContextProviderConfig
	ctxProvider pipeline.ContextProvider
	handlers    []pipeline.Handler
}

func (i *impl) Process(ctx context.Context) (retry bool, delay time.Duration, err error) {
	defer func() {
		if perr := recover(); perr != nil {
			retry = true
			err = fmt.Errorf("panic occurred: %v", perr)
		}
	}()
	ispnContext, err := i.ctxProvider.Get(ctx, i.ContextProviderConfig)
	if err != nil {
		return false, 0, err
	}

	var status pipeline.FlowStatus
	for _, h := range i.handlers {
		invokeHandler(h, i.Infinispan, ispnContext)
		status = ispnContext.FlowStatus()
		if status.Stop {
			break
		}
	}
	err = ispnContext.Close()
	if err != nil {
		return true, status.Delay, err
	}
	return status.Retry, status.Delay, status.Err
}

func invokeHandler(h pipeline.Handler, i *ispnv1.Infinispan, ctx pipeline.Context) {
	defer func() {
		if err := recover(); err != nil {
			e := fmt.Errorf("panic occurred: %v", err)
			ctx.Log().Error(e, string(debug.Stack()))
			ctx.Requeue(e)
		}
	}()
	h.Handle(i, ctx)
}

type builder impl

func (b *builder) For(i *ispnv1.Infinispan) *builder {
	b.Infinispan = i
	return b
}

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

func (b *builder) Build() pipeline.Pipeline {
	i := b.Infinispan
	// TODO init handlers based upon Version defined in Spec and Status
	handlers := handlerBuilder{
		handlers: make([]pipeline.HandlerFunc, 0),
	}

	// Apply default meta before doing anything else
	handlers.Add(manage.PrelimChecksCondition)

	// Configuration Handlers
	handlers.AddFeatureSpecific(i.IsAuthenticationEnabled(), configure.UserAuthenticationSecret)
	handlers.AddFeatureSpecific(i.UserConfigDefined(), configure.UserConfigMap)
	handlers.AddFeatureSpecific(i.IsEncryptionEnabled(), configure.Keystore)
	handlers.AddFeatureSpecific(i.IsClientCertEnabled(), configure.Truststore)
	handlers.AddFeatureSpecific(i.IsAuthenticationEnabled() && i.IsGeneratedSecret(), configure.UserIdentities)
	handlers.Add(
		configure.AdminSecret,
		configure.InfinispanServer,
		configure.Logging,
		configure.AdminIdentities,
		configure.IdentitiesBatch,
	)

	// Provision Handlers
	handlers.AddFeatureSpecific(i.IsAuthenticationEnabled() && i.IsGeneratedSecret(), provision.UserAuthenticationSecret)
	handlers.AddFeatureSpecific(i.IsClientCertEnabled(), provision.TruststoreSecret)
	handlers.Add(
		provision.AdminSecret,
		provision.InfinispanSecuritySecret,
		provision.InfinispanConfigMap,
		provision.PingService,
		provision.AdminService,
		provision.ClusterService,
		provision.ClusterStatefulSet,
	)
	handlers.AddFeatureSpecific(i.IsExposed(), provision.ExternalService)

	// Manage the created Cluster
	// TODO set Status.StatefulSetName
	handlers.Add(manage.PodStatus)
	handlers.AddFeatureSpecific(i.GracefulShutdownUpgrades(), manage.GracefulShutdownUpgrade)
	handlers.Add(
		manage.RemoveFailedInitContainers,
		manage.UpdatePodLabels,
	)
	handlers.AddFeatureSpecific(i.GracefulShutdownUpgrades(), manage.ScheduleGracefulShutdownUpgrade)

	handlers.Add(
		manage.GracefulShutdown,
		manage.AwaitUpgrade,
		manage.StatefulSetRollingUpgrade,
		manage.AwaitPodIps,
		// TODO add autoscaling equipment
		manage.AwaitWellFormedCondition,
		manage.ConfigureLoggers,
		provision.ConfigListener,
	)
	handlers.AddFeatureSpecific(i.IsCache(), manage.CacheService)
	handlers.Add(
		manage.ConsoleUrl,
	)
	// TODO add xsite view conditions

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
