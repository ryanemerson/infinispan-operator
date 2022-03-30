package provision

import (
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/security"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UserAuthenticationSecret(ctx pipeline.Context) {
	i := ctx.Instance()
	if !i.IsAuthenticationEnabled() || !i.IsGeneratedSecret() {
		return
	}

	identities, err := security.GetUserCredentials()
	if err != nil {
		ctx.RetryProcessing(err)
		return
	}

	secret := newSecret(i.GetSecretName(), i.Namespace)
	secret.Type = corev1.SecretTypeOpaque // TODO is this explicit definition required?
	secret.Data = map[string][]byte{consts.ServerIdentitiesFilename: identities}
	ctx.Resources().Secrets().Define(secret)
}

func AdminSecret(ctx pipeline.Context) {
	i := ctx.Instance()

	secrets := ctx.Resources().Secrets()
	secretName := i.GetSecretName()
	secret := secrets.Get(secretName)
	if secret == nil {
		// AdminSecret doesn't exist, so define one
		secret = newSecret(secretName, i.Namespace)
		secret.Labels = i.Labels("infinispan-secret-admin-identities")

		if err := ctx.SetControllerReference(secret); err != nil {
			ctx.RetryProcessing(err)
			return
		}
		secrets.Define(secret)
	}
	configFiles := ctx.ConfigFiles()
	secret.Data = map[string][]byte{
		consts.AdminUsernameKey:         []byte(configFiles.AdminIdentities.Username),
		consts.AdminPasswordKey:         []byte(configFiles.AdminIdentities.Password),
		consts.CliPropertiesFilename:    []byte(configFiles.AdminIdentities.CliProperties),
		consts.ServerIdentitiesFilename: configFiles.AdminIdentities.IdentitiesFile,
	}
}

func InfinispanSecuritySecret(ctx pipeline.Context) {
	i := ctx.Instance()
	secret := newSecret(i.GetInfinispanSecuritySecretName(), i.Namespace)
	secret.Labels = i.Labels("infinispan-secret-server-security")
	secret.Data = map[string][]byte{
		consts.ServerIdentitiesCliFilename: []byte(ctx.ConfigFiles().IdentitiesBatch),
		// TODO add keystore.pem EncryptPemKeystoreName
	}
	if err := ctx.SetControllerReference(secret); err != nil {
		ctx.RetryProcessing(err)
		return
	}
	ctx.Resources().Secrets().Define(secret)
}

func newSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
