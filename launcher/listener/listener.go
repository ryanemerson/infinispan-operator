package listener

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	v1 "github.com/infinispan/infinispan-operator/api/v1"
	"github.com/infinispan/infinispan-operator/api/v2alpha1"
	"github.com/infinispan/infinispan-operator/controllers"
	"github.com/infinispan/infinispan-operator/controllers/constants"
	"github.com/infinispan/infinispan-operator/pkg/kubernetes"
	sse "github.com/r3labs/sse/v2"
	"gopkg.in/cenkalti/backoff.v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(v1.AddToScheme(scheme))
	utilruntime.Must(v2alpha1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
}

type Parameters struct {
	// The namespace in which to CRUD resources
	Namespace string
	// The Name of the Infinispan cluster to listen to in the configured namespace
	Cluster string
}

func New(ctx context.Context, p Parameters) {
	ctx, cancel := context.WithCancel(ctx)

	kubernetes, err := kubernetes.NewKubernetesFromConfig(ctrl.GetConfigOrDie(), scheme)
	if err != nil {
		fmt.Println("failed to create client")
		os.Exit(1)
	}

	infinispan := &v1.Infinispan{}
	if err = kubernetes.Client.Get(ctx, types.NamespacedName{Namespace: p.Namespace, Name: p.Cluster}, infinispan); err != nil {
		fmt.Println(fmt.Errorf("unable to load Infinispan cluster %s in Namespace %s: %v", p.Cluster, p.Namespace, err))
		os.Exit(1)
	}

	secret := &corev1.Secret{}
	if err = kubernetes.Client.Get(ctx, types.NamespacedName{Namespace: p.Namespace, Name: infinispan.GetAdminSecretName()}, secret); err != nil {
		fmt.Println(fmt.Errorf("unable to load Infinispan Admin identities secret %s in Namespace %s: %v", p.Cluster, p.Namespace, err))
		os.Exit(1)
	}

	service := fmt.Sprintf("%s.%s.svc.cluster.local:11223", infinispan.GetAdminServiceName(), p.Namespace)
	fmt.Printf("Attempting to consume streams from service '%s'\n", service)
	user := secret.Data[constants.AdminUsernameKey]
	password := secret.Data[constants.AdminPasswordKey]
	service = fmt.Sprintf("http://%s:%s@%s", user, password, service)

	// TODO make sure to includeCurrentState so any existing caches are created
	containerSse := sse.NewClient(service + "/rest/v2/container/config?action=listen")
	containerSse.Connection.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	containerSse.Headers = map[string]string{
		"Accept": "application/yaml",
	}
	// TODO how to make this more rebust?
	// Fix number of attempts to reconnect?
	// How should Listener behave if Cluster becomes unavailable at runtime?
	containerSse.ReconnectStrategy = &backoff.StopBackOff{}
	containerSse.ReconnectNotify = func(e error, t time.Duration) {
		// TODO log
		fmt.Println(e)
	}

	cacheListener := &controllers.CacheListener{
		Infinispan: infinispan,
		Ctx:        ctx,
		Kubernetes: kubernetes,
	}

	go func() {
		err = containerSse.SubscribeRawWithContext(ctx, func(msg *sse.Event) {
			var err error
			event := string(msg.Event)
			fmt.Printf("ConfigListener received event '%s':\n---\n%s\n---\n", event, msg.Data)
			switch event {
			case "create-cache", "update-cache":
				err = cacheListener.CreateOrUpdate(msg.Data)
			case "remove-cache":
				err = cacheListener.Delete(msg.Data)
			default:
				err = fmt.Errorf("unknown msg.Event: %s", event)
			}
			if err != nil {
				err = fmt.Errorf("eError encountered for event '%s': %v", event, err)
				// TODO handle gracefully. Log?
				fmt.Println(err)
			}
		})
		if err != nil {
			err = fmt.Errorf("error encountered on SSE subscribe")
			fmt.Println(err)
			cancel()
		}
	}()
	<-ctx.Done()
}
