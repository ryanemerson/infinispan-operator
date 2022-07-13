package infinispan

import (
	"fmt"
	"testing"

	"github.com/iancoleman/strcase"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	"github.com/infinispan/infinispan-operator/pkg/mime"
	tutils "github.com/infinispan/infinispan-operator/test/e2e/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestLdapWithCustomConfiguration(t *testing.T) {
	defer testKube.CleanNamespaceAndLogOnPanic(t, tutils.Namespace)
	defer teardownLdapService()
	createLdapService()

	ldapServerConfig :=
		`
<infinispan
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
    xsi:schemaLocation="urn:infinispan:config:13.0 https://infinispan.org/schemas/infinispan-config-13.0.xsd
                        urn:infinispan:server:13.0 https://infinispan.org/schemas/infinispan-server-13.0.xsd"
    xmlns="urn:infinispan:config:13.0"
    xmlns:server="urn:infinispan:server:13.0">
    <server
        xmlns="urn:infinispan:server:13.0">
        <security>
            <security-realms>
                <security-realm name="default" >
                    <ldap-realm name="ldap"
                     url="ldap://${org.infinispan.test.host.address}:389"
                     principal="uid=admin,ou=People,dc=infinispan,dc=org"
                     credential="strongPassword"
                     connection-timeout="3000"
                     read-timeout="30000"
                     connection-pooling="true"
                     referral-mode="ignore"
                     page-size="30"
                     direct-verification="true">
                        <identity-mapping rdn-identifier="uid" search-dn="ou=People,dc=infinispan,dc=org" search-recursive="true">
                            <attribute-mapping>
                                <attribute from="cn" to="Roles" filter="(&amp;(objectClass=groupOfNames)(member={1}))"
                             filter-dn="ou=Roles,dc=infinispan,dc=org"/>
                            </attribute-mapping>
                        </identity-mapping>
                    </ldap-realm>
                </security-realm>
            </security-realms>
        </security>
    </server>
</infinispan>
`

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strcase.ToKebab(tutils.TestName(t)),
			Namespace: tutils.Namespace,
		},
		Data: map[string]string{
			"infinispan-config.xml": ldapServerConfig,
		},
	}
	testKube.Create(configMap)

	ispn := tutils.DefaultSpec(t, testKube, func(i *ispnv1.Infinispan) {
		i.Spec.ConfigMapName = configMap.Name
		i.Spec.Container.ExtraJvmOpts = fmt.Sprintf("-Dorg.infinispan.test.host.address=%s.%s.svc.cluster.local", ldapService.Name, ldapService.Namespace)
		i.Spec.Logging = &ispnv1.InfinispanLoggingSpec{
			Categories: map[string]ispnv1.LoggingLevelType{
				"org.infinispan.SERVER": ispnv1.LoggingLevelTrace,
				"org.wildfly.security":  ispnv1.LoggingLevelTrace,
			},
		}
	})
	testKube.CreateInfinispan(ispn, tutils.Namespace)
	testKube.WaitForInfinispanPods(1, tutils.SinglePodTimeout, ispn.Name, tutils.Namespace)
	testKube.WaitForInfinispanCondition(ispn.Name, ispn.Namespace, ispnv1.ConditionWellFormed)

	protocol := testKube.GetSchemaForRest(ispn)
	client_ := tutils.NewHTTPClient("admin", "strongPassword", protocol)
	testKube.WaitForExternalService(ispn, tutils.RouteTimeout, client_)

	// Assert that correct credentials work as expected
	cacheHelper := tutils.NewCacheHelper(tutils.TestName(t), client_)
	cacheHelper.Create(`{"distributed-cache":{}}`, mime.ApplicationJson)
	cacheHelper.TestBasicUsage("testkey", "test-operator")

	// Assert that invalid credentials fail
	badCredClient := tutils.NewHTTPClient("badUser", "badPass", protocol)
	createCacheBadCreds("failCache", badCredClient)
}

var ldapDeploymentLabels = map[string]string{
	"app": "ldap",
}

var ldapConfig = `
dn: ou=People,dc=infinispan,dc=org
objectclass: top
objectclass: organizationalUnit
ou: People

dn: ou=Roles,dc=infinispan,dc=org
objectclass: top
objectclass: organizationalUnit
ou: Roles

dn: uid=admin,ou=People,dc=infinispan,dc=org
objectclass: top
objectclass: uidObject
objectclass: person
uid: admin
cn: ISPN Admin
sn: admin
userPassword: strongPassword

dn: uid=deployer,ou=People,dc=infinispan,dc=org
objectClass: top
objectclass: uidObject
objectClass: person
uid: deployer
cn: ISPN Deployer
sn: deployer
userPassword: lessStrongPassword

dn: uid=application,ou=People,dc=infinispan,dc=org
objectclass: top
objectclass: uidObject
objectclass: person
uid: application
cn: ISPN Application
sn: application
userPassword: somePassword

dn: uid=observer,ou=People,dc=infinispan,dc=org
objectclass: top
objectclass: uidObject
objectclass: person
uid: observer
cn: ISPN Reader
sn: observer
userPassword: password

dn: uid=monitor,ou=People,dc=infinispan,dc=org
objectclass: top
objectclass: uidObject
objectclass: person
uid: monitor
cn: ISPN Monitor
sn: monitor
userPassword: weakPassword

dn: uid=unprivileged,ou=People,dc=infinispan,dc=org
objectclass: top
objectclass: uidObject
objectclass: person
uid: unprivileged
cn: ISPN Unprivileged
sn: unprivileged
userPassword: weakPassword

dn: uid=executor,ou=People,dc=infinispan,dc=org
objectClass: top
objectclass: uidObject
objectClass: person
uid: executor
cn: ISPN Executor
sn: executor
userPassword: executorPassword

dn: uid=reader,ou=People,dc=infinispan,dc=org
objectclass: top
objectclass: uidObject
objectclass: person
uid: reader
cn: ISPN Reader
sn: reader
userPassword: readerPassword

dn: uid=writer,ou=People,dc=infinispan,dc=org
objectclass: top
objectclass: uidObject
objectclass: person
uid: writer
cn: ISPN Writer
sn: writer
userPassword: writerPassword

dn: cn=admin,ou=Roles,dc=infinispan,dc=org
objectClass: top
objectClass: groupOfNames
cn: admin
description: the Infinispan admin group
member: uid=admin,ou=People,dc=infinispan,dc=org

dn: cn=deployer,ou=Roles,dc=infinispan,dc=org
objectClass: top
objectClass: groupOfNames
cn: deployer
description: the Infinispan deployer group
member: uid=deployer,ou=People,dc=infinispan,dc=org

dn: cn=observer,ou=Roles,dc=infinispan,dc=org
objectClass: top
objectClass: groupOfNames
cn: observer
description: the Infinispan observer group
member: uid=observer,ou=People,dc=infinispan,dc=org

dn: cn=monitor,ou=Roles,dc=infinispan,dc=org
objectClass: top
objectClass: groupOfNames
cn: monitor
description: the Infinispan monitor group
member: uid=monitor,ou=People,dc=infinispan,dc=org

dn: cn=executor,ou=Roles,dc=infinispan,dc=org
objectClass: top
objectClass: groupOfNames
cn: executor
description: the Infinispan executor group
member: uid=executor,ou=People,dc=infinispan,dc=org

dn: cn=writer,ou=Roles,dc=infinispan,dc=org
objectClass: top
objectClass: groupOfNames
cn: writer
description: a writer that cannot read
member: uid=writer,ou=People,dc=infinispan,dc=org

dn: cn=reader,ou=Roles,dc=infinispan,dc=org
objectClass: top
objectClass: groupOfNames
cn: reader
description: a reader that cannot write
member: uid=reader,ou=People,dc=infinispan,dc=org

dn: cn=UnprivilegedRole,ou=Roles,dc=infinispan,dc=org
objectClass: top
objectClass: groupOfNames
cn: UnprivilegedRole
description: the Infinispan unprivileged group
member: uid=unprivileged,ou=People,dc=infinispan,dc=org

dn: cn=___schema_manager,ou=Roles,dc=infinispan,dc=org
objectClass: top
objectClass: groupOfNames
cn: ___schema_manager
description: the Infinispan schema managers
member: uid=admin,ou=People,dc=infinispan,dc=org

dn: cn=___script_manager,ou=Roles,dc=infinispan,dc=org
objectClass: top
objectClass: groupOfNames
cn: ___script_manager
description: the Infinispan script managers
member: uid=admin,ou=People,dc=infinispan,dc=org
`

var ldapConfigMap = &corev1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "ldap-server",
		Namespace: tutils.Namespace,
	},
	Data: map[string]string{
		"config.ldif": ldapConfig,
	},
}

var ldapService = &corev1.Service{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Service",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "ldap-server",
		Namespace: tutils.Namespace,
	},
	Spec: corev1.ServiceSpec{
		Selector: ldapDeploymentLabels,
		Ports: []corev1.ServicePort{{
			Protocol: corev1.ProtocolTCP,
			Port:     389,
		}},
	},
}

var ldapDeployment = &appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "ldap-server",
		Namespace: tutils.Namespace,
	},
	Spec: appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: ldapDeploymentLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: ldapDeploymentLabels,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "ldap",
					Image: "osixia/openldap",
					Args: []string{
						"--copy-service",
					},
					Env: []corev1.EnvVar{
						{Name: "LDAP_ORGANISATION", Value: "Infinispan"},
						{Name: "LDAP_DOMAIN", Value: "infinispan.org"},
						{Name: "LDAP_ADMIN_PASSWORD", Value: "admin"},
						{Name: "LDAP_LOG_LEVEL", Value: "-1"},
						{Name: "LDAP_SEED_INTERNAL_LDIF_PATH", Value: "/ldif"},
					},
					Ports: []corev1.ContainerPort{{
						ContainerPort: 389,
					}},
					VolumeMounts: []corev1.VolumeMount{{
						Name: "ldap-config",
						//MountPath: "/container/service/slapd/assets/config/bootstrap/ldif/custom",
						MountPath: "/ldif",
						//SubPath:   "config.ldif",
					}},
				}},
				Volumes: []corev1.Volume{{
					Name: "ldap-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: ldapConfigMap.Name},
						},
					}},
				}},
		},
	},
}

func createLdapService() {
	testKube.Create(ldapConfigMap)
	testKube.Create(ldapDeployment)
	testKube.Create(ldapService)
	testKube.WaitForDeployment(ldapDeployment.Name, tutils.Namespace)
}

func teardownLdapService() {
	testKube.Delete(ldapConfigMap)
	testKube.Delete(ldapService)
	selector := labels.SelectorFromSet(ldapDeploymentLabels)
	testKube.DeleteResource(tutils.Namespace, selector, ldapDeployment, tutils.SinglePodTimeout)
}
