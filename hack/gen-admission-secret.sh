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

# Create certs using Go script to handle backdating (clock skew fix)
go run hack/gen_cert.go --service "${SERVICE}" --namespace "${NAMESPACE}" --out-dir "${TMP_DIR}"

# (openssl commands removed)

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
