package provision

import (
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UserAuthenticationSecret(ctx pipeline.Context) {
	i := ctx.Instance()
	if !i.IsAuthenticationEnabled() || !i.IsGeneratedSecret() {
		return
	}

	secret := newSecret(i, i.GetSecretName())
	err := ctx.Resources().CreateOrUpdate(secret, true, func() {
		secret.Type = corev1.SecretTypeOpaque // TODO is this explicit definition required?
		secret.Data = map[string][]byte{consts.ServerIdentitiesFilename: ctx.ConfigFiles().UserIdentities}
	})
	if err != nil {
		ctx.RetryProcessing(err)
	}
}

func AdminSecret(ctx pipeline.Context) {
	i := ctx.Instance()
	configFiles := ctx.ConfigFiles()

	secret := newSecret(i, i.GetAdminSecretName())
	err := ctx.Resources().CreateOrPatch(secret, true, func() {
		secret.Labels = i.Labels("infinispan-secret-admin-identities")
		secret.Data = map[string][]byte{
			consts.AdminUsernameKey:         []byte(configFiles.AdminIdentities.Username),
			consts.AdminPasswordKey:         []byte(configFiles.AdminIdentities.Password),
			consts.CliPropertiesFilename:    []byte(configFiles.AdminIdentities.CliProperties),
			consts.ServerIdentitiesFilename: configFiles.AdminIdentities.IdentitiesFile,
		}
	})

	if err != nil {
		ctx.RetryProcessing(err)
	}
}

func InfinispanSecuritySecret(ctx pipeline.Context) {
	i := ctx.Instance()
	configFiles := ctx.ConfigFiles()

	secret := newSecret(i, i.GetInfinispanSecuritySecretName())
	err := ctx.Resources().CreateOrUpdate(secret, true, func() {
		secret.Labels = i.Labels("infinispan-secret-server-security")
		secret.Data = map[string][]byte{
			consts.ServerIdentitiesCliFilename: []byte(configFiles.IdentitiesBatch),
		}
		if i.IsEncryptionEnabled() && len(configFiles.Keystore.PemFile) > 0 {
			secret.Data["keystore.pem"] = configFiles.Keystore.PemFile
		}
	})
	if err != nil {
		ctx.RetryProcessing(err)
	}
}

func TruststoreSecret(ctx pipeline.Context) {
	i := ctx.Instance()

	if !i.IsClientCertEnabled() {
		return
	}

	truststore := ctx.ConfigFiles().Truststore
	secret := newSecret(i, i.GetTruststoreSecretName())
	if err := ctx.Resources().CreateOrPatch(secret, false, func() {
		_, truststoreExists := secret.Data[consts.EncryptTruststoreKey]
		if !truststoreExists {
			secret.Data = map[string][]byte{
				consts.EncryptTruststoreKey:         truststore.File,
				consts.EncryptTruststorePasswordKey: []byte(truststore.Password),
			}
		}
	}); err != nil {
		ctx.RetryProcessing(err)
	}
}

func newSecret(i *ispnv1.Infinispan, name string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: i.Namespace,
		},
	}
}
