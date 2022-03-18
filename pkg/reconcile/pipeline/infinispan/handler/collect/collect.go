package collect

import (
	"fmt"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"strings"
)

func UserAuthenticationSecret(ctx pipeline.Context) {
	i := ctx.Instance()
	if i.IsAuthenticationEnabled() && !i.IsGeneratedSecret() {
		secret, err := ctx.Resources().Secrets().Load(i.GetSecretName())
		if err != nil {
			ctx.RetryProcessing(fmt.Errorf("unable to load user credential secret: %w", err))
			return
		}

		userIdentities, ok := secret.Data[consts.ServerIdentitiesFilename]
		if !ok {
			ctx.RetryProcessing(fmt.Errorf("authentiation secret '%s' missing required file '%s'", secret.Name, consts.ServerIdentitiesCliFilename))
			return
		}
		ctx.ConfigFiles().UserIdentities = userIdentities
	}
}

func UserEncryptionSecrets(ctx pipeline.Context) {
	i := ctx.Instance()
	if !i.IsEncryptionEnabled() {
		return
	}

	if _, err := ctx.Resources().Secrets().Load(i.GetKeystoreSecretName()); err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to load user keystore secret: %w", err))
		return
	}

	if i.IsClientCertEnabled() {
		if _, err := ctx.Resources().Secrets().Load(i.GetTruststoreSecretName()); err != nil {
			ctx.RetryProcessing(fmt.Errorf("unable to load user truststore secret: %w", err))
			return
		}
	}
}

func UserConfigMap(ctx pipeline.Context) {
	i := ctx.Instance()
	if i.UserConfigDefined() {
		return
	}
	overlayConfigMap, err := ctx.Resources().ConfigMaps().Load(i.Spec.ConfigMapName)
	if err != nil {
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
		ServerConfigEncoding: strings.Split(overlayConfigMapKey, ".")[1],
	}
}

func UserDefinedStorageClass(ctx pipeline.Context) {
	i := ctx.Instance()
	storageClassName := i.StorageClassName()
	if i.IsEphemeralStorage() || storageClassName == "" {
		return
	}

	if err := ctx.LoadResource(storageClassName, &storagev1.StorageClass{}); err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to load StorageClass '%s': %w", storageClassName, err))
	}
}

func AdminSecret(ctx pipeline.Context) {
	i := ctx.Instance()
	secret, err := ctx.Resources().Secrets().Load(i.GetAdminSecretName())
	if err != nil {
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
