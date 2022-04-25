package collect

import (
	"fmt"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

func UserAuthenticationSecret(ctx pipeline.Context) {
	i := ctx.Instance()
	if !i.IsAuthenticationEnabled() {
		return
	}
	secret := &corev1.Secret{}
	if err := ctx.Resources().Load(i.GetSecretName(), secret); err != nil {
		if !i.IsGeneratedSecret() {
			ctx.RetryProcessing(fmt.Errorf("unable to load user credential secret: %w", err))
		}
		return
	}

	userIdentities, ok := secret.Data[consts.ServerIdentitiesFilename]
	if !ok {
		ctx.RetryProcessing(fmt.Errorf("authentiation secret '%s' missing required file '%s'", secret.Name, consts.ServerIdentitiesCliFilename))
		return
	}
	ctx.ConfigFiles().UserIdentities = userIdentities
}

func UserEncryptionSecrets(ctx pipeline.Context) {
	i := ctx.Instance()
	if !i.IsEncryptionEnabled() {
		return
	}

	resources := ctx.Resources()
	if err := resources.Load(i.GetKeystoreSecretName(), &corev1.Secret{}); err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to load user keystore secret: %w", err))
		return
	}

	if i.IsClientCertEnabled() {
		if err := resources.Load(i.GetTruststoreSecretName(), &corev1.Secret{}); err != nil {
			ctx.RetryProcessing(fmt.Errorf("unable to load user truststore secret: %w", err))
			return
		}
	}
}

func UserConfigMap(ctx pipeline.Context) {
	i := ctx.Instance()
	if !i.UserConfigDefined() {
		return
	}

	overlayConfigMap := &corev1.ConfigMap{}
	if err := ctx.Resources().Load(i.Spec.ConfigMapName, overlayConfigMap); err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to load user configmap: %w", err))
		return
	}

	var overlayConfigMapKey string
	var overlayLog4jConfig bool
	// Loop through the data looking for something like xml, json or yaml
	for configMapKey := range overlayConfigMap.Data {
		if configMapKey == "infinispan-config.xml" || configMapKey == "infinispan-config.json" || configMapKey == "infinispan-config.yaml" {
			overlayConfigMapKey = configMapKey
			break
		}
	}

	// Check if the user added a custom log4j.xml config
	userLog4j, overlayLog4jConfig := overlayConfigMap.Data["log4j.xml"]

	if overlayConfigMapKey == "" && !overlayLog4jConfig {
		err := fmt.Errorf("one of infinispan-config.[xml|yaml|json] or log4j.xml must be present in the provided ConfigMap: %s", overlayConfigMap.Name)
		ctx.RetryProcessing(err)
	}

	configFiles := ctx.ConfigFiles()
	configFiles.UserConfig = pipeline.UserConfig{
		Log4j:                userLog4j,
		ServerConfig:         overlayConfigMap.Data[overlayConfigMapKey],
		ServerConfigFileName: overlayConfigMapKey,
	}
}

func AdminSecret(ctx pipeline.Context) {
	i := ctx.Instance()
	secret := &corev1.Secret{}
	if err := ctx.Resources().Load(i.GetAdminSecretName(), secret); err != nil {
		if !errors.IsNotFound(err) {
			ctx.RetryProcessing(err)
		}
		return
	}
	ctx.ConfigFiles().AdminIdentities = &pipeline.AdminIdentities{
		Username:       string(secret.Data[consts.AdminUsernameKey]),
		Password:       string(secret.Data[consts.AdminPasswordKey]),
		IdentitiesFile: secret.Data[consts.ServerIdentitiesFilename],
	}
}
