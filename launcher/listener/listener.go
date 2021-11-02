package listener

import (
	"context"
	"fmt"
	"os"
	"time"

	v1 "github.com/infinispan/infinispan-operator/api/v1"
	"github.com/infinispan/infinispan-operator/api/v2alpha1"
	sse "github.com/r3labs/sse/v2"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(v1.AddToScheme(scheme))
	utilruntime.Must(v2alpha1.AddToScheme(scheme))
}

type Parameters struct {
	// The namespace in which to CRUD resources
	Namespace string
	// The Name of the Infinispan cluster to listen to in the configured namespace
	Cluster string
}

type ResourceListener struct {
	Parameters
	ctx    context.Context
	client client.Client
}

func New(ctx context.Context, p Parameters) {
	ctx, cancel := context.WithCancel(ctx)

	k8sClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		fmt.Println("failed to create client")
		os.Exit(1)
	}

	infinispan := &v1.Infinispan{}
	err = k8sClient.Get(ctx, types.NamespacedName{Namespace: p.Namespace, Name: p.Cluster}, infinispan)
	if err != nil {
		fmt.Println(fmt.Errorf("unable to load Infinispan cluster %s in Namespace %s: %v", p.Cluster, p.Namespace, err))
		os.Exit(1)
	}

	// TODO handle authentication support
	// TODO make sure to includeCurrentState so any existing caches are created
	serviceUrl := fmt.Sprintf("%s.%s.svc.cluster.local:11223", infinispan.GetServiceName(), p.Namespace)
	containerSse := sse.NewClient(serviceUrl + "/rest/v2/container/config?action=listen")
	containerSse.Headers = map[string]string{
		"Accept": "application/yaml",
	}

	rl := ResourceListener{
		Parameters: p,
		ctx:        ctx,
		client:     k8sClient,
	}

	go containerSse.SubscribeRawWithContext(ctx, func(msg *sse.Event) {
		event := string(msg.Event)
		switch event {
		case "create-cache":
			rl.createCache(msg.Data)
		case "remove-cache":
			rl.removeCache(msg.Data)
		default:
			// TODO handle gracefully. Log?
			panic("Unknown msg.Event: " + event)
		}
	})

	go func() {
		time.Sleep(10 * time.Minute)
		cancel()
	}()

	<-ctx.Done()
	fmt.Println("finish")
}

func (rl *ResourceListener) createCache(data []byte) {
	type Config struct {
		Infinispan struct {
			CacheContainer struct {
				Caches map[string]interface{}
			} `yaml:"cacheContainer"`
		}
	}

	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		panic("FUCK!")
	}

	if len(config.Infinispan.CacheContainer.Caches) > 1 {
		panic("THIS SHOULDN'T HAPPEN!")
	}
	var cacheName string
	var cacheConfig interface{}
	for cacheName, cacheConfig = range config.Infinispan.CacheContainer.Caches {
		break
	}

	configYaml, err := yaml.Marshal(cacheConfig)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Create cache %s\n%s\n", cacheName, configYaml)
	// TODO create cache CR
	// TODO add labels
	// TODO use annotations to show that the resource was created by the listener
	cache := &v2alpha1.Cache{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cacheName,
			Namespace: rl.Namespace,
		},
		Spec: v2alpha1.CacheSpec{
			ClusterName: "TODO",
			Template:    string(configYaml),
		},
	}
	// TODO set controllerReference to the Infinispan cluster so that the Cache CR is deleted with the cluster
	// if err = controllerutil.SetControllerReference(r.instance, pvc, r.scheme); err != nil {
	// 	return err
	// }
	if err = rl.client.Create(rl.ctx, cache); err != nil {
		panic(fmt.Errorf("unable to create Cache CR: %w", err))
	}
}

func (rl *ResourceListener) updateCache(data string) {
	cache := &v2alpha1.Cache{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "TODO",
			Namespace: rl.Namespace,
		},
	}
	res, err := controllerutil.CreateOrUpdate(rl.ctx, rl.client, cache, func() error {
		if cache.CreationTimestamp.IsZero() {
			// TODO
		}
		return nil
	})
	if err != nil {
		panic(fmt.Errorf("unable to update Cache CR: %w", err))
	}
	fmt.Print(res)
}

func (rl *ResourceListener) removeCache(data []byte) {
	cacheName := string(data)
	fmt.Printf("Remove cache %s\n", cacheName)

	// Lookup Cache CR and delete if it exists
}
