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

BIN_DIR=output/bin
LOCALBIN=$(shell pwd)/_output/bin
export CGO_ENABLED=0
IMAGE_PREFIX?=hub.bjuci.io/dev
IMAGE_TAG?=latest

# Tool versions
CONTROLLER_GEN_VERSION ?= v0.17.3

all: build

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

init:
	mkdir -p ${BIN_DIR}

generate:
	GOPROXY=https://proxy.golang.org,direct ./hack/update-gencode.sh

manifests: controller-gen ## Generate CRD manifests using controller-gen
	$(CONTROLLER_GEN) crd paths="./pkg/apis/..." output:crd:artifacts:config=config/crd/bases

# Install controller-gen locally
controller-gen: init
	@if [ ! -f $(LOCALBIN)/controller-gen ]; then \
		echo "Installing controller-gen $(CONTROLLER_GEN_VERSION)..."; \
		GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION); \
	fi
CONTROLLER_GEN = $(LOCALBIN)/controller-gen

install-crds: manifests ## Install CRDs into the cluster
	kubectl apply --server-side -f config/crd/bases/

install: install-crds ## Install the controller and generic admission secret
	kubectl apply -f installer/controller/work-flow-controller.yaml
	./hack/gen-admission-secret.sh --service work-flow-service --namespace zzc-system --secret work-flow-admission-secret

uninstall-crds: ## Uninstall CRDs from the cluster
	kubectl delete -f config/crd/bases/

test: ## Run unit tests
	go test ./pkg/...

test-coverage: ## Run unit tests with coverage profile
	go test -coverprofile=coverage.out ./pkg/...
	go tool cover -html=coverage.out -o coverage.html

build: init ## Build flow-controller binary
	go build -o ${BIN_DIR}/flow-controller ./cmd/flow-controller

images-build-controller: ## Build flow-controller docker image
	docker build --platform linux/amd64 -t ${IMAGE_PREFIX}/work-flow:${IMAGE_TAG} -f build/Dockerfile .

images-build-admission: ## Build flow-admission docker image
	docker build --platform linux/amd64 -t ${IMAGE_PREFIX}/work-flow-admission:${IMAGE_TAG} -f build/Dockerfile.webhook .

images-push: ## Push docker images
	docker push ${IMAGE_PREFIX}/work-flow:${IMAGE_TAG}
	docker push ${IMAGE_PREFIX}/work-flow-admission:${IMAGE_TAG}

images: images-build-controller images-build-admission images-push ## Build and push all docker images

clean: ## Clean up build outputs
	rm -rf *output/
	rm -rf coverage.out
	rm -rf coverage.html

deploy-example: ## Deploy simple workflow example
	kubectl apply -f examples/worktemplates.yaml
	kubectl apply -f examples/workflows.yaml

undeploy-example: ## Undeploy simple workflow example
	kubectl delete -f examples/workflows.yaml
	kubectl delete -f examples/worktemplates.yaml

deploy-advanced-example: ## Deploy advanced workflow example
	kubectl apply -f examples/advanced-templates.yaml
	kubectl apply -f examples/advanced.yaml

undeploy-advanced-example: ## Undeploy advanced workflow example
	kubectl delete -f examples/advanced.yaml
	kubectl delete -f examples/advanced-templates.yaml

.PHONY: all build images clean generate test test-coverage manifests controller-gen install-crds uninstall-crds images-push images-build-controller images-build-admission deploy-example undeploy-example deploy-advanced-example undeploy-advanced-example