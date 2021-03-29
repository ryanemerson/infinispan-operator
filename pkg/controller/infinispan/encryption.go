package infinispan

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"strings"

	v1 "github.com/infinispan/infinispan-operator/pkg/apis/infinispan/v1"
	config "github.com/infinispan/infinispan-operator/pkg/infinispan/configuration"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	p12 "software.sslmate.com/src/go-pkcs12"
)

const (
	EncryptClientCertPrefix   = "trust.cert."
	EncryptClientCAName       = "trust.ca"
	EncryptKeystoreName       = "keystore.p12"
	EncryptKeystorePath       = ServerRoot + "/conf/keystore"
	EncryptMountPath          = "/etc/encrypt"
	EncryptTruststoreName     = "truststore.p12"
	EncryptTruststorePassword = "password"
)

var ctx = context.TODO()

func ConfigureServerEncryption(i *v1.Infinispan, c *config.InfinispanConfiguration, client client.Client) error {
	if i.IsEncryptionDisabled() {
		return nil
	}

	configureNewKeystore := func(c *config.InfinispanConfiguration) {
		c.Keystore.CrtPath = EncryptMountPath
		c.Keystore.Path = EncryptKeystorePath
		c.Keystore.Password = "password"
		c.Keystore.Alias = "server"
	}

	tlsSecretName := i.GetEncryptionSecretName()
	if tlsSecretName == "" {
		if i.IsClientCertEnabled() {
			// It's not possible for client cert to be configured if no tls secret is provided by the user or via a
			// encryption service, as no certificates are available to be added to the server's truststore, resulting
			// in no client's being able to authenticate with the server.
			return fmt.Errorf("Field 'CertSecretName' must be provided for '%s' or '%s' to be configured", v1.ClientCertAuthenticate, v1.ClientCertNone)
		}
		return nil
	}

	tlsSecret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{Namespace: i.Namespace, Name: tlsSecretName}, tlsSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("Secret %s for endpoint encryption not found.", tlsSecretName)
		}
		return fmt.Errorf("Error in getting secret %s for endpoint encryption: %w", tlsSecretName, err)
	}

	if i.IsEncryptionCertFromService() {
		if strings.Contains(i.Spec.Security.EndpointEncryption.CertServiceName, "openshift.io") {
			configureNewKeystore(c)
			if i.IsClientCertEnabled() {
				caPem, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt")
				if err != nil {
					return err
				}
				return configureClientCert(caPem, i, c, tlsSecret, client)
			}
			return nil
		}
	}

	secretContains := func(keys ...string) bool {
		for _, k := range keys {
			if _, ok := tlsSecret.Data[k]; !ok {
				return false
			}
		}
		return true
	}

	if secretContains(EncryptKeystoreName) {
		// If user provide a keystore in secret then use it ...
		c.Keystore.Path = fmt.Sprintf("%s/%s", EncryptMountPath, EncryptKeystoreName)
		c.Keystore.Password = string(tlsSecret.Data["password"])
		c.Keystore.Alias = string(tlsSecret.Data["alias"])
	} else if secretContains("tls.key", "tls.crt") {
		configureNewKeystore(c)
	}

	if i.IsClientCertEnabled() {
		caPem := tlsSecret.Data[EncryptClientCAName]
		return configureClientCert(caPem, i, c, tlsSecret, client)
	}
	return nil
}

func configureClientCert(caPem []byte, m *v1.Infinispan, c *config.InfinispanConfiguration, tlsSecret *corev1.Secret, client client.Client) error {
	// If the user explicitly provides a truststore use that, otherwise look for individual client certificates and create one
	c.Truststore.Path = fmt.Sprintf("%s/%s", EncryptMountPath, EncryptTruststoreName)
	c.Endpoints.ClientCert = string(m.Spec.Security.EndpointEncryption.ClientCert)

	// If secret already contains a truststore only configure the password
	if _, ok := tlsSecret.Data[EncryptTruststoreName]; ok {
		c.Truststore.Password = string(tlsSecret.Data["truststore-password"])
		return nil
	}

	certs := [][]byte{caPem}
	for certKey := range tlsSecret.Data {
		if strings.HasPrefix(certKey, EncryptClientCertPrefix) {
			certs = append(certs, tlsSecret.Data[certKey])
		}
	}

	truststore, err := generateTruststore(certs, EncryptTruststorePassword)
	if err != nil {
		return err
	}

	_, err = kube.CreateOrPatch(ctx, client, tlsSecret, func() error {
		if tlsSecret.CreationTimestamp.IsZero() {
			return errors.NewNotFound(corev1.Resource("secret"), m.GetEncryptionSecretName())
		}
		tlsSecret.Data[EncryptTruststoreName] = truststore
		tlsSecret.Data["truststore-password"] = []byte("password")
		return nil
	})
	return err
}

func generateTruststore(pemFiles [][]byte, password string) ([]byte, error) {
	certs := []*x509.Certificate{}
	for _, pemFile := range pemFiles {
		pemRaw := pemFile
		for {
			block, rest := pem.Decode(pemRaw)
			if block == nil {
				break
			}

			if block.Type == "CERTIFICATE" {
				cert, err := x509.ParseCertificate(block.Bytes)
				if err != nil {
					return nil, fmt.Errorf("Unable to parse certificate: %w", err)
				}
				certs = append(certs, cert)
			} else {
				log.Info(fmt.Sprintf("Ingoring pem entry type %s when generating truststore. Only CERTIFICATE is supported.", block.Type))
			}
			pemRaw = rest
		}
	}
	truststore, err := p12.EncodeTrustStore(rand.Reader, certs, password)
	if err != nil {
		return nil, fmt.Errorf("Unable to create truststore with user provided cert files: %w", err)
	}
	return truststore, nil

}
