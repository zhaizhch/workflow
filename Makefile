# Copyright 2026 The Volcano Authors.
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

BIN_DIR=output/bin
LOCALBIN=$(shell pwd)/_output/bin
export CGO_ENABLED=0
IMAGE_PREFIX?=hub.bjuci.io/dev
IMAGE_TAG?=latest

# Tool versions
CONTROLLER_GEN_VERSION ?= v0.17.3

all: build

init:
	mkdir -p ${BIN_DIR}

generate:
	./hack/update-gencode.sh

# Generate CRD manifests using controller-gen
manifests: controller-gen
	$(CONTROLLER_GEN) crd paths="./pkg/apis/..." output:crd:artifacts:config=config/crd/bases

# Install controller-gen locally
controller-gen: init
	@if [ ! -f $(LOCALBIN)/controller-gen ]; then \
		echo "Installing controller-gen $(CONTROLLER_GEN_VERSION)..."; \
		GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION); \
	fi
CONTROLLER_GEN = $(LOCALBIN)/controller-gen

# Install CRDs into the cluster
install-crds: manifests
	kubectl apply --server-side -f config/crd/bases/

# Uninstall CRDs from the cluster
uninstall-crds:
	kubectl delete -f config/crd/bases/

test:
	go test ./pkg/...

test-coverage:
	go test -coverprofile=coverage.out ./pkg/...
	go tool cover -html=coverage.out -o coverage.html

build: init
	go build -o ${BIN_DIR}/flow-controller ./cmd/flow-controller

images-build-controller:
	docker build --platform linux/amd64 -t ${IMAGE_PREFIX}/work-flow:${IMAGE_TAG} .

images-build-admission:
	docker build --platform linux/amd64 -t ${IMAGE_PREFIX}/work-flow-admission:${IMAGE_TAG} -f Dockerfile.webhook .

images-push: 
	docker push ${IMAGE_PREFIX}/work-flow:${IMAGE_TAG}
	docker push ${IMAGE_PREFIX}/work-flow-admission:${IMAGE_TAG}

images: images-build-controller images-build-admission images-push

clean:
	rm -rf _output/
	rm -rf coverage.out
	rm -rf coverage.html

deploy-example:
	kubectl apply -f examples/worktemplates.yaml
	kubectl apply -f examples/workflows.yaml

undeploy-example:
	kubectl delete -f examples/workflows.yaml
	kubectl delete -f examples/worktemplates.yaml

.PHONY: all build images clean generate test test-coverage manifests controller-gen install-crds uninstall-crds images-push images-build-controller images-build-admission