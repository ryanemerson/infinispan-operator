package utils

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
	ispnv1 "github.com/infinispan/infinispan-operator/pkg/apis/infinispan/v1"
	v1 "github.com/infinispan/infinispan-operator/pkg/apis/infinispan/v1"
	"github.com/infinispan/infinispan-operator/pkg/controller/constants"
	users "github.com/infinispan/infinispan-operator/pkg/infinispan/security"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/pointer"
)

const EncryptionSecretNamePostfix = "secret-certs"

func LoadFile(path string) []byte {
	b, err := ioutil.ReadFile(path)
	ExpectNoError(err)
	return b
}

func EndpointEncryption(name string) *ispnv1.EndpointEncryption {
	return &ispnv1.EndpointEncryption{
		Type:           v1.CertificateSourceTypeSecret,
		CertSecretName: fmt.Sprintf("%s-%s", name, EncryptionSecretNamePostfix),
	}
}

func EndpointEncryptionClientCert(name string, clientCert v1.ClientCertType) *v1.EndpointEncryption {
	return &v1.EndpointEncryption{
		Type:           v1.CertificateSourceTypeSecret,
		CertSecretName: fmt.Sprintf("%s-%s", name, EncryptionSecretNamePostfix),
		ClientCert:     clientCert,
	}
}

func EncryptionSecret(name, namespace string) *corev1.Secret {
	s := encryptionSecret(name, namespace)
	s.StringData = map[string]string{
		"tls.key": string(LoadFile("../utils/tls/tls.key")),
		"tls.crt": string(LoadFile("../utils/tls/tls.crt")),
	}
	return s
}

func EncryptionSecretKeystore(name, namespace string, keystore []byte) *corev1.Secret {
	s := encryptionSecret(name, namespace)
	s.StringData = map[string]string{
		"alias":    "server",
		"password": KeystorePassword,
	}
	s.Data = map[string][]byte{
		"keystore.p12": keystore,
	}
	return s
}

func EncryptionSecretClientTrustoreValidate(name, namespace string, keystore, truststore []byte) *corev1.Secret {
	s := EncryptionSecretKeystore(name, namespace, keystore)
	s.StringData["truststore-password"] = TruststorePassword
	s.Data["truststore.p12"] = truststore
	return s
}

func encryptionSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", name, EncryptionSecretNamePostfix),
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
	}
}

var MinimalSpec = ispnv1.Infinispan{
	TypeMeta: InfinispanTypeMeta,
	ObjectMeta: metav1.ObjectMeta{
		Name: DefaultClusterName,
	},
	Spec: ispnv1.InfinispanSpec{
		Replicas: 2,
	},
}

func DefaultSpec(testKube *TestKubernetes) *ispnv1.Infinispan {
	return &ispnv1.Infinispan{
		TypeMeta: InfinispanTypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultClusterName,
			Namespace: Namespace,
		},
		Spec: ispnv1.InfinispanSpec{
			Image: pointer.StringPtr("infinispan/server:client-cert"),
			Service: ispnv1.InfinispanServiceSpec{
				Type: ispnv1.ServiceTypeDataGrid,
			},
			Container: ispnv1.InfinispanContainerSpec{
				CPU:    CPU,
				Memory: Memory,
			},
			Replicas: 1,
			Expose:   ExposeServiceSpec(testKube),
		},
	}
}

func CrossSiteSpec(name string, replicas int32, primarySite, backupSite string) *ispnv1.Infinispan {
	return &ispnv1.Infinispan{
		TypeMeta: InfinispanTypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name: strcase.ToKebab(name + primarySite),
		},
		Spec: ispnv1.InfinispanSpec{
			Replicas: replicas,
			Service: ispnv1.InfinispanServiceSpec{
				Type: ispnv1.ServiceTypeDataGrid,
				Sites: &ispnv1.InfinispanSitesSpec{
					Local: ispnv1.InfinispanSitesLocalSpec{
						Name: "Site" + primarySite,
						Expose: ispnv1.CrossSiteExposeSpec{
							Type: ispnv1.CrossSiteExposeTypeClusterIP,
						},
					},
					Locations: []ispnv1.InfinispanSiteLocationSpec{
						{
							Name:       "Site" + backupSite,
							SecretName: secretSiteName(backupSite),
						},
					},
				},
			},
		},
	}
}

func CrossSiteSecret(siteName, namespace string, clientConfig *api.Config) *corev1.Secret {
	currentContext := clientConfig.CurrentContext
	clusterKey := clientConfig.Contexts[currentContext].Cluster
	authInfoKey := clientConfig.Contexts[currentContext].AuthInfo
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretSiteName(siteName),
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"certificate-authority": clientConfig.Clusters[clusterKey].CertificateAuthorityData,
			"client-certificate":    clientConfig.AuthInfos[authInfoKey].ClientCertificateData,
			"client-key":            clientConfig.AuthInfos[authInfoKey].ClientKeyData,
		},
	}
}

func secretSiteName(siteName string) string {
	return "secret-site-" + strings.ToLower(siteName)
}

func ExposeServiceSpec(testKube *TestKubernetes) *ispnv1.ExposeSpec {
	return &ispnv1.ExposeSpec{
		Type: exposeServiceType(testKube),
	}
}

func exposeServiceType(testKube *TestKubernetes) ispnv1.ExposeType {
	exposeServiceType := constants.GetEnvWithDefault("EXPOSE_SERVICE_TYPE", string(ispnv1.ExposeTypeNodePort))
	switch exposeServiceType {
	case string(ispnv1.ExposeTypeNodePort):
		return ispnv1.ExposeTypeNodePort
	case string(ispnv1.ExposeTypeLoadBalancer):
		return ispnv1.ExposeTypeLoadBalancer
	case string(ispnv1.ExposeTypeRoute):
		okRoute, err := testKube.Kubernetes.IsGroupVersionSupported(routev1.GroupVersion.String(), "Route")
		if err == nil && okRoute {
			return ispnv1.ExposeTypeRoute
		}
		panic(fmt.Errorf("expose type Route is not supported on the platform: %w", err))
	default:
		panic(fmt.Errorf("unknown service type %s", exposeServiceType))
	}
}

func GetYamlReaderFromFile(filename string) (*yaml.YAMLReader, error) {
	absFileName := getAbsolutePath(filename)
	f, err := os.Open(absFileName)
	if err != nil {
		return nil, err
	}
	return yaml.NewYAMLReader(bufio.NewReader(f)), nil
}

// Obtain the file absolute path given a relative path
func getAbsolutePath(relativeFilePath string) string {
	if !strings.HasPrefix(relativeFilePath, ".") {
		return relativeFilePath
	}
	dir, _ := os.Getwd()
	absPath, _ := filepath.Abs(dir + "/" + relativeFilePath)
	return absPath
}

func clientForCluster(i *ispnv1.Infinispan, kube *TestKubernetes) HTTPClient {
	protocol := kube.GetSchemaForRest(i)

	if !i.IsAuthenticationEnabled() {
		return NewHTTPClientNoAuth(protocol)
	}

	user := constants.DefaultDeveloperUser
	pass, err := users.UserPassword(user, i.GetSecretName(), i.Namespace, kube.Kubernetes)
	ExpectNoError(err)
	return NewHTTPClient(user, pass, protocol)
}

func HTTPClientAndHost(i *ispnv1.Infinispan, kube *TestKubernetes) (string, HTTPClient) {
	client := clientForCluster(i, kube)
	hostAddr := kube.WaitForExternalService(i.GetServiceExternalName(), i.Namespace, i.GetExposeType(), RouteTimeout, client)
	return hostAddr, client
}

func HTTPSClientAndHost(i *v1.Infinispan, tlsConfig *tls.Config, kube *TestKubernetes) (string, HTTPClient) {
	var client HTTPClient
	if i.Spec.Security.EndpointEncryption.ClientCert == ispnv1.ClientCertAuthenticate {
		client = NewHTTPSClientCertAuth(tlsConfig)
	} else {
		if i.IsAuthenticationEnabled() {
			user := constants.DefaultDeveloperUser
			pass, err := users.UserPassword(user, i.GetSecretName(), i.Namespace, kube.Kubernetes)
			ExpectNoError(err)
			client = NewHTTPSClient(user, pass, tlsConfig)
		} else {
			client = NewHTTPSClientNoAuth(tlsConfig)
		}
	}

	hostAddr := kube.WaitForExternalService(i.GetServiceExternalName(), i.Namespace, i.GetExposeType(), RouteTimeout, client)
	return hostAddr, client
}
