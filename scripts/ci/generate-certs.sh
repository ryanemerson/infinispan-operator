#!/bin/bash
set -e

ROOT_DOMAIN=$1
DIR=$2
SSL_FILE=sslconf-${ROOT_DOMAIN}.conf

rm -rf $DIR
mkdir $DIR
cd $DIR

# Generate SSL Config with SANs
cat > ${SSL_FILE} <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
[req_distinguished_name]
localityName_default = Red Hat
organizationalUnitName_default = Infinispan
[ v3_req ]
# Extensions to add to a certificate request
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${ROOT_DOMAIN}
DNS.2 = *.${ROOT_DOMAIN}
DNS.3 = localhost
EOF

# Create CA certificate
openssl req -new -nodes -out ca.csr \
 -keyout ca.key -subj "/CN=CA/OU=Infinispan/O=JBoss/L=Red Hat"
chmod og-rwx ca.key

openssl x509 -req -in ca.csr -days 398 \
 -extfile /etc/ssl/openssl.cnf -extensions v3_ca \
 -signkey ca.key -out ca.crt

# Create Server certificate signed by CA
openssl req -new -nodes -out server.csr \
 -keyout server.key -subj "/CN=${ROOT_DOMAIN}" -extensions v3_req
chmod og-rwx server.key

openssl x509 -req -in server.csr -days 398 \
 -CA ca.crt -CAkey ca.key -CAcreateserial \
 -out server.crt -extensions v3_req -extfile ${SSL_FILE}

rm -f *.csr *.srl ${SSL_FILE}
