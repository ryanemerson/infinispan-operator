package secret

import (
	"context"
	"fmt"
	"net/url"

	"github.com/go-logr/logr"
	ispnv1 "github.com/infinispan/infinispan-operator/pkg/apis/infinispan/v1"
	consts "github.com/infinispan/infinispan-operator/pkg/controller/constants"
	"github.com/infinispan/infinispan-operator/pkg/controller/infinispan"
	"github.com/infinispan/infinispan-operator/pkg/controller/infinispan/resources"
	users "github.com/infinispan/infinispan-operator/pkg/infinispan/security"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ControllerName   = "secret-controller"
	AdminUserKey     = "username"
	AdminPasswordKey = "password"
)

var ctx = context.Background()

// reconcileSecret reconciles a Secret object
type reconcileSecret struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
}

type secretResource struct {
	infinispan *ispnv1.Infinispan
	client     client.Client
	scheme     *runtime.Scheme
	kube       *kube.Kubernetes
	log        logr.Logger
}

func (r reconcileSecret) ResourceInstance(infinispan *ispnv1.Infinispan, ctrl *resources.Controller, kube *kube.Kubernetes, log logr.Logger) resources.Resource {
	return &secretResource{
		infinispan: infinispan,
		client:     r.Client,
		scheme:     ctrl.Scheme,
		kube:       kube,
		log:        log,
	}
}

func (r reconcileSecret) Types() []*resources.ReconcileType {
	return []*resources.ReconcileType{{ObjectType: &corev1.Secret{}, GroupVersion: corev1.SchemeGroupVersion, GroupVersionSupported: true}}
}

func (r reconcileSecret) EventsPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
	}
}

func Add(mgr manager.Manager) error {
	return resources.CreateController(ControllerName, &reconcileSecret{mgr.GetClient()}, mgr)
}

func (s *secretResource) Process() (reconcile.Result, error) {
	secret, err := s.getSecret(s.infinispan.GetAdminSecretName())
	if err != nil {
		return reconcile.Result{}, err
	}

	// If the operator secret does not already exist create it with generated identities
	if secret == nil {
		if err = s.createAdminIdentitiesSecret(); err != nil {
			return reconcile.Result{}, err
		}
	} else {
		// Patch secret to include any generated fields
		updated, err := s.update(secret, func() error {
			if secret.Data[consts.ServerIdentitiesFilename] != nil && secret.Data[AdminPasswordKey] == nil {
				// Handle migration of old secret where only identities.yaml was present
				secret.Data[AdminUserKey] = []byte(consts.DefaultOperatorUser)
				pwd, err := users.FindPassword(consts.DefaultOperatorUser, secret.Data[consts.ServerIdentitiesFilename])
				if err != nil {
					return err
				}
				secret.Data[AdminPasswordKey] = []byte(pwd)
			}

			if err := adminIdentitiesYaml(secret); err != nil {
				return err
			}

			return s.addCliProperties(secret)
		})

		if updated || err != nil {
			return reconcile.Result{}, err
		}
	}

	// If the user has provided their own secret or authentication is disabled, do nothing
	if !s.infinispan.IsGeneratedSecret() {
		return reconcile.Result{}, nil
	}

	// Create the user identities secret if it doesn't already exist
	secret, err = s.getSecret(s.infinispan.GetSecretName())
	if secret != nil || err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, s.createUserIdentitiesSecret()
}

func (s secretResource) createUserIdentitiesSecret() error {
	identities, err := users.GetUserCredentials()
	if err != nil {
		return err
	}
	secret := s.secretSpec(s.infinispan.GetSecretName(), "infinispan-secret-identities")
	secret.Data[consts.ServerIdentitiesFilename] = identities
	return s.createSecret(secret)
}

func (s secretResource) createAdminIdentitiesSecret() error {
	secret := s.secretSpec(s.infinispan.GetAdminSecretName(), "infinispan-secret-admin-identities")
	secret.Data[AdminUserKey] = []byte(consts.DefaultOperatorUser)
	secret.Data[AdminPasswordKey] = []byte(users.GetRandomStringForAuth(16))

	if err := adminIdentitiesYaml(secret); err != nil {
		return err
	}

	if err := s.addCliProperties(secret); err != nil {
		return err
	}
	return s.createSecret(secret)
}

func (s secretResource) secretSpec(name, label string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.infinispan.Namespace,
			Labels:    infinispan.LabelsResource(s.infinispan.Name, label),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{},
	}
}

func (s secretResource) createSecret(secret *corev1.Secret) error {
	s.log.Info(fmt.Sprintf("Creating Identities Secret %s", secret.Name))
	_, err := controllerutil.CreateOrUpdate(ctx, s.client, secret, func() error {
		return controllerutil.SetControllerReference(s.infinispan, secret, s.scheme)
	})

	if err != nil {
		return fmt.Errorf("Unable to create identities secret: %w", err)
	}
	return nil
}

func adminIdentitiesYaml(secret *corev1.Secret) error {
	usr := string(secret.Data[AdminUserKey])
	pwd := string(secret.Data[AdminPasswordKey])
	identities, err := users.CreateIdentitiesFor(usr, pwd)
	if err != nil {
		return err
	}
	secret.Data[consts.ServerIdentitiesFilename] = identities
	return nil
}

func (s secretResource) addCliProperties(secret *corev1.Secret) error {
	descriptor := secret.Data[consts.ServerIdentitiesFilename]
	pass, err := users.FindPassword(consts.DefaultOperatorUser, []byte(descriptor))
	if err != nil {
		return err
	}
	if secret.StringData == nil {
		secret.StringData = map[string]string{}
	}

	service := s.infinispan.GetServiceName()
	url := fmt.Sprintf("http://%s:%s@%s:%d", consts.DefaultOperatorUser, url.QueryEscape(pass), service, consts.InfinispanAdminPort)
	properties := fmt.Sprintf("autoconnect-url=%s", url)
	secret.Data[consts.CliPropertiesFilename] = []byte(properties)
	return nil
}

func (s secretResource) getSecret(name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := s.client.Get(ctx, types.NamespacedName{Namespace: s.infinispan.Namespace, Name: name}, secret)
	if err == nil {
		return secret, nil
	}
	if errors.IsNotFound(err) {
		return nil, nil
	}
	return nil, err
}

func (s secretResource) update(secret *corev1.Secret, mutate func() error) (bool, error) {
	res, err := kube.CreateOrPatch(ctx, s.client, secret, func() error {
		if secret.CreationTimestamp.IsZero() {
			return errors.NewNotFound(corev1.Resource("secret"), secret.Name)
		}
		return mutate()
	})
	return res != controllerutil.OperationResultNone, err
}
