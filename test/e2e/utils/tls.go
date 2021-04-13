package utils

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"time"
)

const (
	KeystorePassword   = "secret"
	TruststorePassword = "secret"
	keyBits            = 2048
	tmpDir             = "/tmp/infinispan/operator/tls"
)

var serialNumber int64 = 1

type certHolder struct {
	privateKey *rsa.PrivateKey
	cert       *x509.Certificate
	certBytes  []byte
}

// Returns a keystore using a self-signed certificate, and the corresponding tls.Config required by clients to connect to the server
func CreateKeystore() (keystore []byte, clientTLSConf *tls.Config) {
	ca := ca()
	server := cert("server", ca)
	keystore = createKeystore(ca, server)

	certpool := x509.NewCertPool()
	certpool.AddCert(ca.cert)
	clientTLSConf = &tls.Config{
		RootCAs: certpool,
	}
	return
}

// Returns a keystore & truststore using a self-signed certificate, and the corresponding tls.Config required by clients to connect to the server
// If authenticate is true, then the returned truststore contains all client certificates, otherwise it simply contains the CA for validation
func CreateKeyAndTruststore(authenticate bool) (keystore []byte, truststore []byte, clientTLSConf *tls.Config) {
	ca := ca()
	server := cert("server", ca)
	keystore = createKeystore(ca, server)

	client := cert("client", ca)
	truststore = createTruststore(ca, client, authenticate)

	clientCertBytes := append(client.getCertPEM(), client.getPrivateKeyPEM()...)
	// TODO remove
	ioutil.WriteFile(tmpFile("client.pem"), clientCertBytes, 0777)

	certpool := x509.NewCertPool()
	certpool.AddCert(ca.cert)

	clientTLSConf = &tls.Config{
		GetClientCertificate: func(t *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			certificate, err := tls.X509KeyPair(client.getCertPEM(), client.getPrivateKeyPEM())
			return &certificate, err
		},
		RootCAs:    certpool,
		ServerName: "server",
	}
	return
}

func ca() *certHolder {
	// create our private and public key
	privateKey, err := rsa.GenerateKey(rand.Reader, keyBits)
	ExpectNoError(err)

	// create the CA
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			CommonName:         "CA",
			Organization:       []string{"JBoss"},
			OrganizationalUnit: []string{"Infinispan"},
			Locality:           []string{"Red Hat"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(10, 0, 0),
		IsCA:      true,
		// ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		// KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		PublicKeyAlgorithm:    x509.RSA,
		SignatureAlgorithm:    x509.SHA256WithRSA,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &privateKey.PublicKey, privateKey)
	ExpectNoError(err)

	cert, err := x509.ParseCertificate(certBytes)
	ExpectNoError(err)

	return &certHolder{
		privateKey: privateKey,
		cert:       cert,
		certBytes:  certBytes,
	}
}

func cert(name string, ca *certHolder) *certHolder {
	// create our private and public key
	privateKey, err := rsa.GenerateKey(rand.Reader, keyBits)
	ExpectNoError(err)

	// set up our server certificate
	server := &x509.Certificate{
		SerialNumber: big.NewInt(serialNumber),
		Subject: pkix.Name{
			CommonName:         name,
			Organization:       []string{"JBoss"},
			OrganizationalUnit: []string{"Infinispan"},
			Locality:           []string{"Red Hat"},
		},
		Issuer: ca.cert.Subject,
		// IPAddresses:        []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(10, 0, 0),
		// SubjectKeyId: []byte{1, 2, 3, 4, 6},
		// TODO required? Used in java testsuite
		// Extensions: ,
		// ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		// KeyUsage:           x509.KeyUsageDigitalSignature,
		PublicKeyAlgorithm: x509.RSA,
		SignatureAlgorithm: x509.SHA256WithRSA,
	}
	serialNumber++

	certBytes, err := x509.CreateCertificate(rand.Reader, server, ca.cert, &privateKey.PublicKey, ca.privateKey)
	ExpectNoError(err)

	cert, err := x509.ParseCertificate(certBytes)
	ExpectNoError(err)

	return &certHolder{
		privateKey: privateKey,
		cert:       cert,
		certBytes:  certBytes,
	}
}

func createKeystore(ca, server *certHolder) []byte {
	var fileMode os.FileMode = 0777
	ExpectNoError(os.MkdirAll(tmpDir, fileMode))
	defer os.RemoveAll(tmpDir)

	privKeyFile := tmpFile("server_key.pem")
	certFile := tmpFile("server_cert.pem")
	generatedKeystore := tmpFile("keystore.p12")

	ioutil.WriteFile(privKeyFile, server.getPrivateKeyPEM(), fileMode)
	// TODO do we need to include the CA here or just server?
	ioutil.WriteFile(certFile, append(server.getCertPEM(), ca.getCertPEM()...), fileMode)

	cmd := exec.Command("openssl", "pkcs12", "-export", "-in", certFile, "-inkey", privKeyFile,
		"-name", server.cert.Subject.CommonName, "-out", generatedKeystore, "-password", "pass:"+KeystorePassword, "-noiter", "-nomaciter")
	ExpectNoError(cmd.Run())

	keystore, err := ioutil.ReadFile(generatedKeystore)
	ExpectNoError(err)
	return keystore
}

func createTruststore(ca, client *certHolder, authenticate bool) []byte {
	var fileMode os.FileMode = 0777
	ExpectNoError(os.MkdirAll(tmpDir, fileMode))
	// TODO uncomment
	// defer os.RemoveAll(tmpDir)

	certFile := tmpFile("trust_cert.pem")
	generatedTruststore := tmpFile("truststore.p12")

	var certs []byte
	if authenticate {
		certs = append(ca.getCertPEM(), client.getCertPEM()...)
	} else {
		certs = ca.getCertPEM()
	}
	bagAttributes := "Certificate bag\nBag Attributes\n    friendlyName: ca\n    2.16.840.1.113894.746875.1.1: <Unsupported tag 6>\n"
	certs = append([]byte(bagAttributes), certs...)
	ioutil.WriteFile(certFile, certs, fileMode)

	// openssl cannot create pkcs12 in a way that java likes
	// TOOD use keytool instead :(
	// keytool -keystore tuststore.p12 -alias ca -import -file /tmp/infinispan/operator/tls/trust_cert.pem -noprompt -storepass secret
	// TODO try just using this library instead? https://pkg.go.dev/software.sslmate.com/src/go-pkcs12?utm_source=godoc
	// cmd := exec.Command("openssl", "pkcs12", "-export", "-nokeys", "-in", certFile, "-out", generatedTruststore, "-password", "pass:"+TruststorePassword)
	cmd := exec.Command("keytool", "-keystore", generatedTruststore, "-alias", "ca", "-import", "-file", certFile, "-noprompt", "-storepass", TruststorePassword)
	ExpectNoError(cmd.Run())

	truststore, err := ioutil.ReadFile(generatedTruststore)
	ExpectNoError(err)
	return truststore
}

func tmpFile(name string) string {
	return fmt.Sprintf("%s/%s", tmpDir, name)
}

// Return the private key in PEM format
func (c *certHolder) getPrivateKeyPEM() []byte {
	privKeyPEM := new(bytes.Buffer)
	pem.Encode(privKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(c.privateKey),
	})
	return privKeyPEM.Bytes()
}

// Return the certificate in PEM format
func (c *certHolder) getCertPEM() []byte {
	cert := new(bytes.Buffer)
	pem.Encode(cert, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: c.certBytes,
	})
	return cert.Bytes()
}

// Return the PEM bytes of the private key and the certificate
func (c *certHolder) getPEM() []byte {
	return append(c.getPrivateKeyPEM(), c.getPrivateKeyPEM()...)
}
