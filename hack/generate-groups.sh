#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

# generate-groups generates everything for a project with external types only, e.g. a project based
# on CustomResourceDefinitions.

if [ "$#" -lt 4 ] || [ "${1}" == "--help" ]; then
  cat <<EOF
Usage: $(basename "$0") <generators> <output-package> <apis-package> <groups-versions> ...

  <generators>        the generators comma separated to run (deepcopy,defaulter,client,lister,informer) or "all".
  <output-package>    the output package name (e.g. github.com/example/project/pkg/generated).
  <apis-package>      the external types dir (e.g. github.com/example/api or github.com/example/project/pkg/apis).
  <groups-versions>   the groups and their versions in the format "groupA:v1,v2 groupB:v1 groupC:v2", relative
                      to <api-package>.
  ...                 arbitrary flags passed to all generator binaries.


Examples:
  $(basename "$0") all             github.com/example/project/pkg/client github.com/example/project/pkg/apis "foo:v1 bar:v1alpha1,v1beta1"
  $(basename "$0") deepcopy,client github.com/example/project/pkg/client github.com/example/project/pkg/apis "foo:v1 bar:v1alpha1,v1beta1"
EOF
  exit 0
fi

GENS="$1"
OUTPUT_PKG="$2"
APIS_PKG="$3"
GROUPS_WITH_VERSIONS="$4"
shift 4

(
  # To support running this script from anywhere, first cd into this directory,
  # and then install with forced module mode on and fully qualified name.
  cd "$(dirname "${0}")"
  GO111MODULE=on GOOS=$(go env GOOS) go get k8s.io/code-generator/cmd/{defaulter-gen,client-gen,lister-gen,informer-gen,deepcopy-gen,applyconfiguration-gen}
  GO111MODULE=on GOOS=$(go env GOOS) go install k8s.io/code-generator/cmd/{defaulter-gen,client-gen,lister-gen,informer-gen,deepcopy-gen,applyconfiguration-gen}
)
# Go installs the above commands to get installed in $GOBIN if defined, and $GOPATH/bin otherwise:
GOBIN="$(go env GOBIN)"
gobin="${GOBIN:-$(go env GOPATH)/bin}"

function codegen::join() { local IFS="$1"; shift; echo "$*"; }

# enumerate group versions
FQ_APIS=() # e.g. k8s.io/api/apps/v1
for GVs in ${GROUPS_WITH_VERSIONS}; do
  IFS=: read -r G Vs <<<"${GVs}"

  # enumerate versions
  for V in ${Vs//,/ }; do
    FQ_APIS+=("${APIS_PKG}/${G}/${V}")
  done
done

if [ "${GENS}" = "all" ] || grep -qw "deepcopy" <<<"${GENS}"; then
  echo "Generating deepcopy funcs"
  "${gobin}/deepcopy-gen" --bounding-dirs "$(codegen::join , "${FQ_APIS[@]}")" --output-file zz_generated.deepcopy "$@"
fi

MODULE_NAME="${APIS_PKG%/pkg/apis}"
CLIENTSET_OUTPUT_PKG="${CLIENTSET_PKG_NAME:-versioned}"

if [ "${GENS}" = "all" ] || grep -qw "client" <<<"${GENS}"; then
  echo "Generating clientset for ${GROUPS_WITH_VERSIONS} at ${OUTPUT_PKG}/clientset/${CLIENTSET_OUTPUT_PKG}"
  "${gobin}/client-gen" --clientset-name "${CLIENTSET_OUTPUT_PKG}" --input-base "" --input "$(codegen::join , "${FQ_APIS[@]}")"  --output-dir "${OUTPUT_PKG}/clientset" --output-pkg  "${MODULE_NAME}/pkg/client/clientset" "$@"
fi

if [ "${GENS}" = "all" ] || grep -qw "lister" <<<"${GENS}"; then
  echo "Generating listers for ${GROUPS_WITH_VERSIONS} at ${OUTPUT_PKG}/listers"
  "${gobin}/lister-gen" "${FQ_APIS[@]}"  --output-dir "${OUTPUT_PKG}/listers" --output-pkg "${MODULE_NAME}/pkg/client/listers" "$@"
fi

if [ "${GENS}" = "all" ] || grep -qw "informer" <<<"${GENS}"; then
  echo "Generating informers for ${GROUPS_WITH_VERSIONS} at ${OUTPUT_PKG}/informers/externalversions"
  "${gobin}/informer-gen" \
           "${FQ_APIS[@]}" \
           --versioned-clientset-package "${MODULE_NAME}/pkg/client/clientset/${CLIENTSET_OUTPUT_PKG}" \
           --listers-package "${MODULE_NAME}/pkg/client/listers" \
           --output-dir "${OUTPUT_PKG}/informers" \
           --output-pkg "${MODULE_NAME}/pkg/client/informers" \
           "$@"
fi

if [ "${GENS}" = "all" ] || grep -qw "applyconfiguration" <<<"${GENS}"; then
  echo "Generating apply configurations for ${GROUPS_WITH_VERSIONS} at ${OUTPUT_PKG}/applyconfiguration"
  "${gobin}/applyconfiguration-gen" \
           "${FQ_APIS[@]}" \
           --output-dir "${OUTPUT_PKG}/applyconfiguration" \
           --output-pkg "${MODULE_NAME}/pkg/client/applyconfiguration" \
           "$@"
fi
