#!/bin/sh

# Copyright 2026 zhaizhicheng.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

usage() {
    cat <<EOF
Generate certificate suitable for use with an admission webhook service.

This script generates a self-signed certificate and stores it in a Kubernetes secret.
It also patches the specified ValidatingWebhookConfiguration with the CA bundle.

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

TMP_DIR=$(mktemp -d)
echo "creating certs in tmpdir ${TMP_DIR}"

# Create a CSR config
cat > ${TMP_DIR}/server.conf <<EOF
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

# Create a CA cert
openssl genrsa -out ${TMP_DIR}/ca.key 2048
openssl req -x509 -new -nodes -key ${TMP_DIR}/ca.key -subj "/CN=Admission Webhook CA" -days 3650 -out ${TMP_DIR}/ca.crt

# Create server key and signing request
openssl genrsa -out ${TMP_DIR}/tls.key 2048
openssl req -new -key ${TMP_DIR}/tls.key -subj "/CN=${SERVICE}.${NAMESPACE}.svc" -out ${TMP_DIR}/server.csr -config ${TMP_DIR}/server.conf

# Sign the server cert with the CA
openssl x509 -req -in ${TMP_DIR}/server.csr \
    -CA ${TMP_DIR}/ca.crt -CAkey ${TMP_DIR}/ca.key \
    -CAcreateserial -out ${TMP_DIR}/tls.crt -days 3650 \
    -extensions v3_req -extfile ${TMP_DIR}/server.conf

# Create the secret with CA cert and server cert/key
# We delete first to ensure it's updated if it exists
kubectl delete secret ${SECRET} -n ${NAMESPACE} 2>/dev/null || true

kubectl create secret generic ${SECRET} \
    --from-file=tls.key=${TMP_DIR}/tls.key \
    --from-file=tls.crt=${TMP_DIR}/tls.crt \
    --from-file=ca.crt=${TMP_DIR}/ca.crt \
    -n ${NAMESPACE}

# Base64 encode the CA cert for the webhook configuration
CA_BUNDLE=$(cat ${TMP_DIR}/ca.crt | base64 | tr -d '\n')

# Patch the webhook configuration with the CA bundle
# Note: We use the hardcoded name 'work-flow-admission' which is defined in the deployment YAML
kubectl patch validatingwebhookconfiguration work-flow-admission \
    --type='json' -p="[{'op': 'replace', 'path': '/webhooks/0/clientConfig/caBundle', 'value':'${CA_BUNDLE}'}]"

echo "Secret ${SECRET} created and webhook patched."

# Copy certs to the current directory so the calling process can use them
cp ${TMP_DIR}/tls.crt .
cp ${TMP_DIR}/tls.key .

rm -rf ${TMP_DIR}
