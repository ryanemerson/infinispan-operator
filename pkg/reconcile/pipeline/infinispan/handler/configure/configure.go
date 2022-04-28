package configure

import (
	"fmt"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/configuration/logging"
	config "github.com/infinispan/infinispan-operator/pkg/infinispan/configuration/server"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/security"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"net/url"
	"strings"
)

const (
	EncryptPkcs12KeystoreName = "keystore.p12"
	EncryptPemKeystoreName    = "keystore.pem"
)

func UserAuthenticationSecret(i *ispnv1.Infinispan, ctx pipeline.Context) {
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

func UserConfigMap(i *ispnv1.Infinispan, ctx pipeline.Context) {
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

func AdminSecret(i *ispnv1.Infinispan, ctx pipeline.Context) {
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

// TODO how to reuse server generation during HotRod rolling upgrade?
// Collect xsite resources before configuration called, make it so that all that is required to generate
// server config is statefulset name, xsite backups
func InfinispanServer(i *ispnv1.Infinispan, ctx pipeline.Context) {
	var roleMapper string
	if i.IsClientCertEnabled() && i.Spec.Security.EndpointEncryption.ClientCert == ispnv1.ClientCertAuthenticate {
		roleMapper = "commonName"
	} else {
		roleMapper = "cluster"
	}

	configSpec := &config.Spec{
		ClusterName:     i.Name,
		Namespace:       i.Namespace,
		StatefulSetName: i.GetStatefulSetName(),
		Infinispan: config.Infinispan{
			Authorization: &config.Authorization{
				Enabled:    i.IsAuthorizationEnabled(),
				RoleMapper: roleMapper,
			},
		},
		JGroups: config.JGroups{
			Diagnostics: consts.JGroupsDiagnosticsFlag == "TRUE",
			FastMerge:   consts.JGroupsFastMerge,
		},
		Endpoints: config.Endpoints{
			Authenticate: i.IsAuthenticationEnabled(),
			ClientCert:   string(ispnv1.ClientCertNone),
		},
		//XSite: xsite,
	}

	// Apply settings for authentication and roles
	specRoles := i.GetAuthorizationRoles()
	if len(specRoles) > 0 {
		confRoles := make([]config.AuthorizationRole, len(specRoles))
		for i, role := range specRoles {
			confRoles[i] = config.AuthorizationRole{
				Name:        role.Name,
				Permissions: strings.Join(role.Permissions, ","),
			}
		}
		configSpec.Infinispan.Authorization.Roles = confRoles
	}

	if i.Spec.CloudEvents != nil {
		configSpec.CloudEvents = &config.CloudEvents{
			Acks:              i.Spec.CloudEvents.Acks,
			BootstrapServers:  i.Spec.CloudEvents.BootstrapServers,
			CacheEntriesTopic: i.Spec.CloudEvents.CacheEntriesTopic,
		}
	}
	if i.IsEncryptionEnabled() {
		ks := ctx.ConfigFiles().Keystore
		configSpec.Keystore = config.Keystore{
			Alias: ks.Alias,
			// Actual value is not used by template, but required to show that a credential ref is required
			Password: ks.Password,
			Path:     ks.Path,
		}

		if i.IsClientCertEnabled() {
			configSpec.Endpoints.ClientCert = string(i.Spec.Security.EndpointEncryption.ClientCert)
			configSpec.Truststore.Path = fmt.Sprintf("%s/%s", consts.ServerEncryptTruststoreRoot, consts.EncryptTruststoreKey)
		}
	}

	// TODO utilise a version specific configurator once server/operator versions decoupled
	serverConf, err := config.Generate(nil, configSpec)
	if err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to generate infinispan.xml: %w", err))
		return
	}
	ctx.ConfigFiles().ServerConfig = serverConf
}

func Logging(i *ispnv1.Infinispan, ctx pipeline.Context) {
	loggingSpec := &logging.Spec{
		Categories: i.GetLogCategoriesForConfig(),
	}
	// TODO utilise a version specific logging once server/operator versions decoupled
	log4jXml, err := logging.Generate(nil, loggingSpec)
	if err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to generate log4j.xml: %w", err))
		return
	}
	ctx.ConfigFiles().Log4j = log4jXml
}

func AdminIdentities(i *ispnv1.Infinispan, ctx pipeline.Context) {
	configFiles := ctx.ConfigFiles()

	user := consts.DefaultOperatorUser
	if configFiles.AdminIdentities == nil {
		// An existing secret was not found in the collect stage, so generate new credentials and define in the context
		identities, err := security.GetAdminCredentials()
		if err != nil {
			ctx.RetryProcessing(err)
			return
		}

		password, err := security.FindPassword(user, identities)
		if err != nil {
			ctx.RetryProcessing(err)
			return
		}

		configFiles.AdminIdentities = &pipeline.AdminIdentities{
			Username:       user,
			Password:       password,
			IdentitiesFile: identities,
		}
	} else {
		password := configFiles.AdminIdentities.Password
		if password == "" {
			var usrErr error
			if password, usrErr = security.FindPassword(user, configFiles.AdminIdentities.IdentitiesFile); usrErr != nil {
				ctx.RetryProcessing(usrErr)
				return
			}
		}
		identities, err := security.CreateIdentitiesFor(user, password)
		if err != nil {
			ctx.RetryProcessing(err)
			return
		}
		configFiles.AdminIdentities.IdentitiesFile = identities
	}

	autoconnectUrl := fmt.Sprintf("http://%s:%s@%s:%d",
		user,
		url.QueryEscape(configFiles.AdminIdentities.Password),
		i.GetAdminServiceName(),
		consts.InfinispanAdminPort,
	)
	configFiles.AdminIdentities.CliProperties = fmt.Sprintf("autoconnect-url=%s", autoconnectUrl)
}

func UserIdentities(_ *ispnv1.Infinispan, ctx pipeline.Context) {
	configFiles := ctx.ConfigFiles()
	if configFiles.UserIdentities == nil {
		identities, err := security.GetUserCredentials()
		if err != nil {
			ctx.RetryProcessing(err)
			return
		}
		configFiles.UserIdentities = identities
	}
}

func IdentitiesBatch(i *ispnv1.Infinispan, ctx pipeline.Context) {
	configFiles := ctx.ConfigFiles()

	// Define admin identities on the server
	batch, err := security.IdentitiesCliFileFromSecret(configFiles.AdminIdentities.IdentitiesFile, "admin", "cli-admin-users.properties", "cli-admin-groups.properties")
	if err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to read admin credentials: %w", err))
		return
	}

	// Add user identities only if authentication enabled
	if i.IsAuthenticationEnabled() {
		usersCliBatch, err := security.IdentitiesCliFileFromSecret(configFiles.UserIdentities, "default", "cli-users.properties", "cli-groups.properties")
		if err != nil {
			ctx.RetryProcessing(fmt.Errorf("unable to read user credentials: %w", err))
			return
		}
		batch += usersCliBatch
	}

	if i.IsEncryptionEnabled() {
		configFiles := ctx.ConfigFiles()

		// Add the keystore credential if the user has provided their own keystore
		if configFiles.Keystore.Password != "" {
			batch += fmt.Sprintf("credentials add keystore -c \"%s\" -p secret\n", configFiles.Keystore.Password)
		}

		if i.IsClientCertEnabled() {
			batch += fmt.Sprintf("credentials add truststore -c \"%s\" -p secret\n", configFiles.Truststore.Password)
		}
	}

	configFiles.IdentitiesBatch = batch
}

func Keystore(i *ispnv1.Infinispan, ctx pipeline.Context) {
	keystore := &pipeline.Keystore{}
	if i.IsEncryptionCertFromService() {
		if strings.Contains(i.Spec.Security.EndpointEncryption.CertServiceName, "openshift.io") {
			keystore.Path = consts.ServerOperatorSecurity + "/" + EncryptPemKeystoreName
		}
	} else {
		keystoreSecret := &corev1.Secret{}
		if err := ctx.Resources().Load(i.GetKeystoreSecretName(), keystoreSecret); err != nil {
			ctx.RetryProcessing(fmt.Errorf("unable to load user keystore secret: %w", err))
			return
		}

		isUserProvidedPrivateKey := func() bool {
			for _, k := range []string{corev1.TLSPrivateKeyKey, corev1.TLSCertKey} {
				if _, ok := keystoreSecret.Data[k]; !ok {
					return false
				}
			}
			return true
		}

		if userKeystore, exists := keystoreSecret.Data[EncryptPkcs12KeystoreName]; exists {
			// If the user provides a keystore in secret then use it ...
			keystore.Path = fmt.Sprintf("%s/%s", consts.ServerEncryptKeystoreRoot, EncryptPkcs12KeystoreName)
			keystore.Alias = string(keystoreSecret.Data["alias"])
			keystore.Password = string(keystoreSecret.Data["password"])
			keystore.File = userKeystore
		} else if isUserProvidedPrivateKey() {
			keystore.Path = consts.ServerOperatorSecurity + "/" + EncryptPemKeystoreName
			keystore.PemFile = append(keystoreSecret.Data["tls.key"], keystoreSecret.Data["tls.crt"]...)
		}
	}
	ctx.ConfigFiles().Keystore = keystore
}

func Truststore(i *ispnv1.Infinispan, ctx pipeline.Context) {
	trustSecret := &corev1.Secret{}
	if err := ctx.Resources().Load(i.GetTruststoreSecretName(), trustSecret); err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to load user truststore secret: %w", err))
		return
	}

	passwordBytes, passwordProvided := trustSecret.Data[consts.EncryptTruststorePasswordKey]
	password := string(passwordBytes)

	// If Truststore and password already exist, nothing to do
	if truststore, exists := trustSecret.Data[consts.EncryptTruststoreKey]; exists {
		if !passwordProvided {
			ctx.RetryProcessing(fmt.Errorf("the '%s' key must be provided when configuring an existing Truststore", consts.EncryptTruststorePasswordKey))
			return
		}
		ctx.ConfigFiles().Truststore = &pipeline.Truststore{
			File:     truststore,
			Password: password,
		}
		return
	}

	if !passwordProvided {
		password = "password"
	}

	// Generate Truststore from provided ca and cert files
	caPem := trustSecret.Data["trust.ca"]
	certs := [][]byte{caPem}

	for certKey := range trustSecret.Data {
		if strings.HasPrefix(certKey, "trust.cert.") {
			certs = append(certs, trustSecret.Data[certKey])
		}
	}
	truststore, err := security.GenerateTruststore(certs, password)
	if err != nil {
		ctx.RetryProcessing(err)
		return
	}
	ctx.ConfigFiles().Truststore = &pipeline.Truststore{
		File:     truststore,
		Password: password,
	}
}
