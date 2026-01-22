#!/bin/sh

# Based on volcano/hack/gen-admission-secret.sh

set -e

usage() {
    cat <<EOF
Generate certificate suitable for use with an admission webhook service.

This script uses k8s' CertificateSigningRequest API to a generate a
certificate signed by k8s CA suitable for use with admission webhook
services. This requires permissions to create and approve CSRs. See
https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster for
detailed explanation and additional instructions.

The server key/cert k8s CA cert are stored in a k8s secret.

usage: ${0} [OPTIONS]

The following flags are required.

       --service          Service name of webhook.
       --namespace        Namespace where webhook service and secret reside.
       --secret           Secret name for CA cert and server cert/key pair.
EOF
    exit 1
}

while [ $# -gt 0 ]; do
    case ${1} in
        --service)
            SERVICE="$2"
            shift
            ;;
        --namespace)
            NAMESPACE="$2"
            shift
            ;;
        --secret)
            SECRET="$2"
            shift
            ;;
        *)
            usage
            ;;
    esac
    shift
done

[ -z "${SERVICE}" ] && usage
[ -z "${NAMESPACE}" ] && usage
[ -z "${SECRET}" ] && usage

if [ ! -x "$(command -v openssl)" ]; then
    echo "openssl not found"
    exit 1
fi

CSR_NAME=${SERVICE}.${NAMESPACE}.svc
echo "creating certs in tmpdir ${TMP_DIR} "

# Create a self-signed CA
openssl req -nodes -new -x509 -keyout ca.key -out ca.crt -subj "/CN=${SERVICE}.${NAMESPACE}.svc"

# Create a server cert
cat > server.conf <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${SERVICE}
DNS.2 = ${SERVICE}.${NAMESPACE}
DNS.3 = ${SERVICE}.${NAMESPACE}.svc
EOF

openssl req -nodes -new -keyout tls.key -out tls.csr -subj "/CN=${SERVICE}.${NAMESPACE}.svc" -config server.conf

# Sign the server cert
openssl x509 -req -in tls.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out tls.crt -extensions v3_req -extfile server.conf

# Create the secret with CA cert and server cert/key
kubectl create secret generic ${SECRET} \
    --from-file=tls.key=tls.key \
    --from-file=tls.crt=tls.crt \
    --from-file=ca.crt=ca.crt \
    -n ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# Patch the webhook configuration with the CA bundle
CA_BUNDLE=$(cat ca.crt | base64 | tr -d '\n')
kubectl patch validatingwebhookconfiguration work-flow-admission \
    --type='json' -p="[{'op': 'replace', 'path': '/webhooks/0/clientConfig/caBundle', 'value':'${CA_BUNDLE}'}]"

echo "Secret ${SECRET} created and webhook patched."
