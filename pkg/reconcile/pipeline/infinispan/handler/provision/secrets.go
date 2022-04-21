package provision

import (
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UserAuthenticationSecret(ctx pipeline.Context) {
	i := ctx.Instance()
	if !i.IsAuthenticationEnabled() || !i.IsGeneratedSecret() {
		return
	}

	secret := newSecret(i.GetSecretName(), i.Namespace)
	secret.Type = corev1.SecretTypeOpaque // TODO is this explicit definition required?
	secret.Data = map[string][]byte{consts.ServerIdentitiesFilename: ctx.ConfigFiles().UserIdentities}
	ctx.Resources().Define(secret, true)
}

func AdminSecret(ctx pipeline.Context) {
	i := ctx.Instance()

	resources := ctx.Resources()
	secretName := i.GetAdminSecretName()
	secret := &corev1.Secret{}
	if err := resources.Load(secretName, secret); err != nil {
		if !errors.IsNotFound(err) {
			ctx.RetryProcessing(err)
			return
		}
		// AdminSecret doesn't exist, so define one
		secret = newSecret(secretName, i.Namespace)
		secret.Labels = i.Labels("infinispan-secret-admin-identities")
		resources.Define(secret, true)
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
	configFiles := ctx.ConfigFiles()

	secret := newSecret(i.GetInfinispanSecuritySecretName(), i.Namespace)
	secret.Labels = i.Labels("infinispan-secret-server-security")
	secret.Data = map[string][]byte{
		consts.ServerIdentitiesCliFilename: []byte(configFiles.IdentitiesBatch),
	}

	if i.IsEncryptionEnabled() && len(configFiles.Keystore.PemFile) > 0 {
		secret.Data["keystore.pem"] = configFiles.Keystore.PemFile
	}
	ctx.Resources().Define(secret, true)
}

func TruststoreSecret(ctx pipeline.Context) {
	i := ctx.Instance()

	if !i.IsClientCertEnabled() {
		return
	}

	truststore := ctx.ConfigFiles().Truststore
	secret := newSecret(i.GetTruststoreSecretName(), i.Namespace)
	secret.Data = map[string][]byte{
		consts.EncryptTruststoreKey:         truststore.File,
		consts.EncryptTruststorePasswordKey: []byte(truststore.Password),
	}
	ctx.Resources().Define(secret, false)
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
