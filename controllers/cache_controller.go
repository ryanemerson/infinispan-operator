package controllers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/iancoleman/strcase"
	v1 "github.com/infinispan/infinispan-operator/api/v1"
	v2alpha1 "github.com/infinispan/infinispan-operator/api/v2alpha1"
	"github.com/infinispan/infinispan-operator/controllers/constants"
	"github.com/infinispan/infinispan-operator/pkg/infinispan"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/caches"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// CacheReconciler reconciles a Cache object
type CacheReconciler struct {
	client.Client
	log        logr.Logger
	scheme     *runtime.Scheme
	kubernetes *kube.Kubernetes
	eventRec   record.EventRecorder
}

type CacheListener struct {
	// The Infinispan cluster to listen to in the configured namespace
	Cluster *v1.Infinispan
	Ctx     context.Context
	Client  client.Client
}

type cacheRequest struct {
	*CacheReconciler
	ctx        context.Context
	cache      *v2alpha1.Cache
	infinispan *v1.Infinispan
	reqLogger  logr.Logger
}

// SetupWithManager sets up the controller with the Manager.
func (r *CacheReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.log = ctrl.Log.WithName("controllers").WithName("Cache")
	r.scheme = mgr.GetScheme()
	r.kubernetes = kube.NewKubernetesFromController(mgr)
	r.eventRec = mgr.GetEventRecorderFor("cache-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v2alpha1.Cache{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=infinispan.org,namespace=infinispan-operator-system,resources=caches;caches/status;caches/finalizers,verbs=get;list;watch;create;update;patch

func (r *CacheReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("+++++ Reconciling Cache.")
	defer reqLogger.Info("----- End Reconciling Cache.")

	// Fetch the Cache instance
	instance := &v2alpha1.Cache{}
	if err := r.Client.Get(ctx, request.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// TODO implement Finalizer https://sdk.operatorframework.io/docs/building-operators/golang/advanced-topics/
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Cache resource not found. Ignoring it since cache deletion is not supported")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if instance.Spec.AdminAuth != nil {
		reqLogger.Info("Ignoring and removing 'spec.AdminAuth' field. The operator's admin credentials are now used to perform cache operations")
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, instance, func() error {
			instance.Spec.AdminAuth = nil
			return nil
		})
		return reconcile.Result{}, err
	}

	// Fetch the Infinispan cluster
	infinispan := &v1.Infinispan{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: instance.Namespace, Name: instance.Spec.ClusterName}, infinispan); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, fmt.Sprintf("Infinispan cluster %s not found", infinispan.Name))
			return reconcile.Result{RequeueAfter: constants.DefaultWaitOnCluster}, err
		}
		return reconcile.Result{}, err
	}

	// Cluster must be well formed
	if !infinispan.IsWellFormed() {
		reqLogger.Info(fmt.Sprintf("Infinispan cluster %s not well formed", infinispan.Name))
		return reconcile.Result{RequeueAfter: constants.DefaultWaitOnCluster}, nil
	}

	cache := &cacheRequest{
		CacheReconciler: r,
		ctx:             ctx,
		cache:           instance,
		infinispan:      infinispan,
	}

	// Don't contact the Infinispan server for resources created by the ConfigListener
	if !cache.listenerResource() {
		if result, err := cache.reconcileInfinispan(); result != nil {
			return *result, err
		}
	}

	_, err := kube.CreateOrPatch(ctx, r.Client, instance, func() error {
		if instance.CreationTimestamp.IsZero() {
			return errors.NewNotFound(schema.ParseGroupResource("cache.infinispan.org"), instance.Name)
		}
		instance.SetCondition("Ready", metav1.ConditionTrue, "")
		return nil
	})

	if err != nil {
		err = fmt.Errorf("unable to update cache %s status: %w", instance.Name, err)
		reqLogger.Error(err, "")
		return reconcile.Result{}, err
	}
	return ctrl.Result{}, nil
}

// Determine if reconciliation was triggered by the ConfigListener
func (r *cacheRequest) listenerResource() bool {
	annotations := r.cache.ObjectMeta.Annotations
	if val, exists := annotations[constants.ListenerAnnotationGeneration]; exists {
		listenerGeneration, _ := strconv.ParseInt(val, 10, 64)
		return r.cache.Generation == listenerGeneration
	}
	return false
}

func (r *cacheRequest) newClusterClient() (string, *infinispan.Cluster, error) {
	podList, err := PodList(r.infinispan, r.kubernetes, r.ctx)
	if err != nil {
		return "", nil, err
	} else if len(podList.Items) < 1 {
		return "", nil, fmt.Errorf("no Infinispan pods available")
	}

	cluster, err := NewCluster(r.infinispan, r.kubernetes, r.ctx)
	if err != nil {
		return "", nil, err
	}
	return podList.Items[0].Name, cluster, nil
}

func (r *cacheRequest) reconcileInfinispan() (*reconcile.Result, error) {
	podName, cluster, err := r.newClusterClient()
	if err != nil {
		return &reconcile.Result{}, fmt.Errorf("unable to create Cluster client: %w", err)
	}

	cacheExists, err := cluster.ExistsCache(r.cache.GetCacheName(), podName)
	if err != nil {
		err := fmt.Errorf("unable to determine if cache exists: %w", err)
		r.reqLogger.Error(err, "")
		return &reconcile.Result{}, err
	}

	if r.infinispan.IsDataGrid() {
		err = r.reconcileDataGrid(cacheExists, podName, cluster)
	} else {
		err = r.reconcileCacheService(cacheExists, podName, cluster)
	}
	return &reconcile.Result{}, err
}

func (r *cacheRequest) reconcileCacheService(cacheExists bool, podName string, cluster *infinispan.Cluster) error {
	spec := r.cache.Spec
	if cacheExists {
		err := fmt.Errorf("cannot update an existing cache in a CacheService cluster")
		r.reqLogger.Error(err, "Error updating cache")
		return err
	}

	if spec.TemplateName != "" || spec.Template != "" {
		err := fmt.Errorf("cannot create a cache with a template in a CacheService cluster")
		r.reqLogger.Error(err, "Error creating cache")
		return err
	}

	template, err := caches.DefaultCacheTemplateXML(podName, r.infinispan, cluster, r.reqLogger)
	if err != nil {
		err = fmt.Errorf("unable to obtain default cache template: %w", err)
		r.reqLogger.Error(err, "Error getting default XML")
		return err
	}

	err = cluster.CreateCacheWithTemplate(r.cache.Spec.Name, template, podName)
	if err != nil {
		err = fmt.Errorf("unable to create cache using default template: %w", err)
		r.reqLogger.Error(err, "Error in creating cache")
		return err
	}
	return nil
}

func (r *cacheRequest) reconcileDataGrid(cacheExists bool, podName string, cluster *infinispan.Cluster) error {
	spec := r.cache.Spec
	cacheName := r.cache.GetCacheName()
	if cacheExists {
		// TODO handle update
		return nil
	}

	var err error
	if spec.TemplateName != "" {
		if err = cluster.CreateCacheWithTemplateName(cacheName, spec.TemplateName, podName); err != nil {
			err = fmt.Errorf("unable to create cache with template name '%s': %w", spec.TemplateName, err)
		}
	} else {
		if err = cluster.CreateCacheWithTemplate(cacheName, spec.Template, podName); err != nil {
			err = fmt.Errorf("unable to create cache with template: %w", err)
		}
	}

	if err != nil {
		r.reqLogger.Error(err, "Unable to create Cache")
	}
	return err
}

func (cl *CacheListener) Create(data []byte) error {
	cacheName, configYaml, err := unmarshallEventConfig(data)
	if err != nil {
		return err
	}

	kebabCacheName := strcase.ToKebab(cacheName)
	if cacheName != kebabCacheName {
		fmt.Printf("Creating Cache CR with name '%s' for cache '%s'. Transformation required by k8s conventions\n", cacheName, kebabCacheName)
	}

	fmt.Printf("Create cache %s\n%s\n", cacheName, configYaml)
	// TODO add labels?
	cache := &v2alpha1.Cache{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kebabCacheName,
			Namespace: cl.Cluster.Namespace,
			Annotations: map[string]string{
				constants.ListenerAnnotationGeneration: "1",
			},
		},
		Spec: v2alpha1.CacheSpec{
			Name:        cacheName,
			ClusterName: cl.Cluster.Name,
			Template:    configYaml,
		},
	}
	if err = controllerutil.SetControllerReference(cl.Cluster, cache, cl.Client.Scheme()); err != nil {
		return err
	}
	return cl.Client.Create(cl.Ctx, cache)
}

// TODO what happens if a Template is updated on the server? How to handle with consuming CRs?
func (cl *CacheListener) Update(data []byte) error {
	cacheName, configYaml, err := unmarshallEventConfig(data)
	if err != nil {
		return err
	}

	cache := &v2alpha1.Cache{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strcase.ToKebab(cacheName),
			Namespace: cl.Cluster.Namespace,
		},
	}
	res, err := controllerutil.CreateOrUpdate(cl.Ctx, cl.Client, cache, func() error {
		// TODO handle conversion of Yaml -> User format
		cache.ObjectMeta.Annotations[constants.ListenerAnnotationGeneration] = strconv.FormatInt(cache.Generation+1, 10)
		cache.Spec = v2alpha1.CacheSpec{
			Name:        cacheName,
			ClusterName: cl.Cluster.Name,
			Template:    configYaml,
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to update Cache CR: %w", err)
	}
	// TODO handle res
	fmt.Print(res)
	return nil
}

func (cl *CacheListener) Delete(data []byte) error {
	cacheName := string(data)
	fmt.Printf("Remove cache %s\n", cacheName)
	crName := strcase.ToKebab(cacheName)

	cache := &v2alpha1.Cache{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: cl.Cluster.Namespace,
		},
	}
	// TODO how do we prevent controller from issuing REST DELETE call to server in Finalizer?
	err := cl.Client.Delete(cl.Ctx, cache)
	// If the CR can't be found, do nothing
	if !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func unmarshallEventConfig(data []byte) (string, string, error) {
	type Config struct {
		Infinispan struct {
			CacheContainer struct {
				Caches map[string]interface{}
			} `yaml:"cacheContainer"`
		}
	}

	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return "", "", fmt.Errorf("unable to unmarshal event data: %w", err)
	}

	if len(config.Infinispan.CacheContainer.Caches) != 1 {
		return "", "", fmt.Errorf("unexpected yaml format: %s", data)
	}
	var cacheName string
	var cacheConfig interface{}
	// Retrieve the first (and only) entry in the map
	for cacheName, cacheConfig = range config.Infinispan.CacheContainer.Caches {
		break
	}

	configYaml, err := yaml.Marshal(cacheConfig)
	if err != nil {
		return "", "", fmt.Errorf("unable to marshall cache configuration: %w", err)
	}
	return cacheName, string(configYaml), nil
}
