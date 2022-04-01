package configure

import (
	"fmt"
	v1 "github.com/infinispan/infinispan-operator/api/v1"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/configuration/logging"
	config "github.com/infinispan/infinispan-operator/pkg/infinispan/configuration/server"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/security"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	"net/url"
	"strings"
)

// TODO how to reuse server generation during HotRod rolling upgrade?
// Collect xsite resources before configuration called, make it so that all that is required to generate
// server config is statefulset name, xsite backups
func InfinispanServer(ctx pipeline.Context) {
	i := ctx.Instance()
	serverConf, err := generateServer(i.GetStatefulSetName(), i, nil)
	if err != nil {
		ctx.RetryProcessing(fmt.Errorf("unable to generate infinispan.xml: %w", err))
		return
	}
	ctx.ConfigFiles().ServerConfig = serverConf
}

func generateServer(statefulSet string, i *v1.Infinispan, xsite interface{}) (string, error) {
	var roleMapper string
	if i.IsClientCertEnabled() && i.Spec.Security.EndpointEncryption.ClientCert == v1.ClientCertAuthenticate {
		roleMapper = "commonName"
	} else {
		roleMapper = "cluster"
	}

	configSpec := &config.Spec{
		ClusterName:     i.Name,
		Namespace:       i.Namespace,
		StatefulSetName: statefulSet,
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
			ClientCert:   string(v1.ClientCertNone),
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
	// TODO Add TLS

	// TODO utilise a version specific configurator once server/operator versions decoupled
	return config.Generate(nil, configSpec)
}

func Logging(ctx pipeline.Context) {
	i := ctx.Instance()

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

func AdminIdentities(ctx pipeline.Context) {
	i := ctx.Instance()
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

func UserIdentities(ctx pipeline.Context) {
	i := ctx.Instance()
	if !i.IsAuthenticationEnabled() || !i.IsGeneratedSecret() {
		return
	}

	identities, err := security.GetUserCredentials()
	if err != nil {
		ctx.RetryProcessing(err)
		return
	}
	ctx.ConfigFiles().UserIdentities = identities
}

func IdentitiesBatch(ctx pipeline.Context) {
	i := ctx.Instance()
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
	// TODO add TLS batch commands
	configFiles.IdentitiesBatch = batch
}
