# Client Certifaction

## Server
The Infinispan server supports two types of client certification:

1. VALIDATE - Only requires the certificates used to sign the client certificates to be in the Truststore. Typically this is the Certificate Authority (CA).
2. AUTHENTICATE - Requires all of the client certificates to be in the Truststore.

### Configuration
To use either 1 or 2, you need to configure `<endpoints socket-binding="..." security-realm="..." require-ssl-client-auth="true">`

and you need to add `<truststore path="{truststore.path}" password="{truststore.password}"/>` to `<server-identities><ssl>..`.

To enable 2, you also need to add a `<truststore-realm/>` element to the `security-realm`.

For example:

```xml
<server>
    <security xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
            xsi:schemaLocation="urn:infinispan:server:12.1 https://infinispan.org/schemas/infinispan-server-12.1.xsd"
            xmlns="urn:infinispan:server:12.1">
    <security-realms>
        <security-realm name="default">
            <server-identities>
                <ssl>
                <keystore path="server.pfx" relative-to="infinispan.server.config.path" keystore-password="secret"
                            alias="server"/>
                <truststore path="ca.pfx"  relative-to="infinispan.server.config.path" password="secret"/>
                </ssl>
            </server-identities>
            <!-- Optional element that should be added if client cert AUTHENTICATION is required -->
            <truststore-realm/>
        </security-realm>
    </security-realms>
    </security>
    <endpoints xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
            xsi:schemaLocation="urn:infinispan:server:12.1 https://infinispan.org/schemas/infinispan-server-12.1.xsd"
            xmlns="urn:infinispan:server:12.1"
            socket-binding="default" security-realm="default" require-ssl-client-auth="true"/>

<server>
```

## Clients

### Hot Rod
A keystore containing the client certificate must be configured on HotRod clients using the [EXTERNAL](https://infinispan.org/docs/stable/titles/hotrod_java/hotrod_java.html#hotrod_endpoint_auth-client) mechanism.

### Rest
The client cert must be associated with any REST call made by the client.

## Operator
The default is for no client certifcation to be enabled, which is equivalent to `require-ssl-client-auth="false"` on the server.

The operator supports both client cert authentication and validation, which can be configured as below:

```yaml
spec:
  security:
    endpointEncryption:
        type: Secret | Service
        certSecretName: tls-secret 
        clientCert: None | Validate | Authenticate
```

The user can either provide an existing Truststore, or relying on the operator to create one. The `endpointEncrpytion.type`
determines how truststore are handled, so these are explained independently below:

> Only pkcs12 format Truststores are supported.

### Secret Encryption

#### User Provided Truststore
The user can provide and existing truststore via the tls secret in a similar manner to how `keystore.p12` can be configured:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: tls-secret
type: Opaque
stringData:
    alias: server 
    password: password 
    truststore-password: password 
data:
    keystore.p12:    "MIIKDgIBAzCCCdQGCSqGSIb3DQEHA..." 
    truststore.p12:  "ALsadasdaFASfASFaSfasfASfasfa..." 
```

#### Generated Truststore
Alternatively the user can provide PEM encoded certificates via the tls secret and the truststore is generated on their behalf.

The `trust.ca` data should be the cert used to sign the client certificates and is mandatory for both cert Authentication and Validation.

The `trust.cert.*` files corresponds to individual client certificates. This is only necessary for cert Authentication.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: tls-secret
type: Opaque
stringData:
    alias: server 
    password: password
    truststore-password: password 
data:
    keystore.p12:  "MIIKDgIBAzCCCdQGCSqGSIb3DQEHA..."
    trust.ca: "Some ca byte string"
    trust.cert.client1: "Some cert byte string"
    trust.cert.client2: "Some cert byte string"
```

> The `trust.cert.<name>` value is used as the alias associated with the certificate in the generated truststore.

> The `truststore-password` field is optional. If a truststore value is provided by the user, then it is applied to the generated Truststore,
otherwise a default value of "password" is used and added to the secret.

### Service Encryption
Currently only supported on Openshift.

Truststore's can only be generated with `type: Service`, it's not possible for a user to provide their own via `truststore.p12`.

#### Validate
If `clientCert: Validate` is configured, then no further action is required by the user. The a truststore is generated
using the CA cert at "/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt".

#### Authenticate
As it's necessary for all client certificates to exist in the server's truststore, the user must provide individual client
certs via the tls secret using the `trust.cert.*` keys.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: tls-secret
type: Opaque
stringData:
    truststore-password: password 
data:
    trust.ca: "Some ca byte string"
    trust.cert.client1: "Some cert byte string"
    trust.cert.client2: "Some cert byte string"
```

#### Client Configuration
Clients can access the Openshift CA cert by one of two means.

1. Annotating a configmap with `service.beta.openshift.io/inject-cabundle:true` and retrieving the ca bundle:

```quote
Other services can request that the CA bundle for the service CA be injected into API service or config map resources by annotating with service.beta.openshift.io/inject-cabundle: true to support validating certificates generated from the service CA. In response, the Operator writes its current CA bundle to the CABundle field of an API service or as service-ca.crt to a config map.
```

2. Accessing the bundle in a pod via `/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt`
