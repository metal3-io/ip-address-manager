# Copyright 2018 The Kubernetes Authors.
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

# If you update this file, please follow
# https://suva.sh/posts/well-documented-makefiles

# Ensure Make is run with bash shell as some syntax below is bash-specific
SHELL:=/usr/bin/env bash

.DEFAULT_GOAL:=help

GO_VERSION ?= 1.20.14
# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

# Directories.
TOOLS_DIR := hack/tools
APIS_DIR := api
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
BIN_DIR := bin

# Binaries.
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/controller-gen
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/golangci-lint
MOCKGEN := $(TOOLS_BIN_DIR)/mockgen
CONVERSION_GEN := $(TOOLS_BIN_DIR)/conversion-gen
KUBEBUILDER := $(TOOLS_BIN_DIR)/kubebuilder
KUSTOMIZE := $(TOOLS_BIN_DIR)/kustomize

# Define Docker related variables. Releases should modify and double check these vars.
# REGISTRY ?= gcr.io/$(shell gcloud config get-value project)
REGISTRY ?= quay.io/metal3-io
STAGING_REGISTRY := quay.io/metal3-io
PROD_REGISTRY := quay.io/metal3-io
IMAGE_NAME ?= ip-address-manager
CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
TAG ?= v1alpha1
ARCH ?= amd64
ALL_ARCH = amd64 arm arm64 ppc64le s390x

# Allow overriding manifest generation destination directory
MANIFEST_ROOT ?= config
CRD_ROOT ?= $(MANIFEST_ROOT)/crd/bases
METAL3_CRD_ROOT ?= $(MANIFEST_ROOT)/crd/metal3
WEBHOOK_ROOT ?= $(MANIFEST_ROOT)/webhook
RBAC_ROOT ?= $(MANIFEST_ROOT)/rbac

# Allow overriding the imagePullPolicy
PULL_POLICY ?= IfNotPresent

## --------------------------------------
## Help
## --------------------------------------

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Testing
## --------------------------------------

.PHONY: unit
unit: ## Run tests
	source ./hack/fetch_ext_bins.sh; fetch_tools; setup_envs; go test -v ./controllers/... ./ipam/... -coverprofile ./cover.out; cd $(APIS_DIR); go test -v ./... -coverprofile ./cover.out

.PHONY: test  ## Run formatter, linter and tests
test: generate fmt lint unit

## --------------------------------------
## Build
## --------------------------------------

.PHONY: binaries
binaries: manager ## Builds and installs all binaries

.PHONY: build
build: binaries build-api ## Builds all IPAM modules

.PHONY: manager
manager: ## Build manager binary.
	go build -o $(BIN_DIR)/manager .

.PHONY: build-api
build-api: ## Builds api directory.
	cd $(APIS_DIR) && go build ./...

## --------------------------------------
## Tooling Binaries
## --------------------------------------

$(CONTROLLER_GEN): $(TOOLS_DIR)/go.mod # Build controller-gen from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

$(GOLANGCI_LINT): 
		hack/ensure-golangci-lint.sh $(TOOLS_DIR)/$(BIN_DIR)

$(MOCKGEN): $(TOOLS_DIR)/go.mod # Build mockgen from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/mockgen github.com/golang/mock/mockgen

$(CONVERSION_GEN): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/conversion-gen k8s.io/code-generator/cmd/conversion-gen

$(KUBEBUILDER): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); ./install_kubebuilder.sh

$(KUSTOMIZE): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); ./install_kustomize.sh

## --------------------------------------
## Linting
## --------------------------------------

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Lint codebase
	$(GOLANGCI_LINT) run -v --timeout=5m
	cd $(APIS_DIR); ../$(GOLANGCI_LINT) run -v --timeout=10m

lint-full: $(GOLANGCI_LINT) ## Run slower linters to detect possible issues
	$(GOLANGCI_LINT) run -v --fast=false
	cd $(APIS_DIR); ../$(GOLANGCI_LINT) run -v --fast=false --timeout=30m

# Run go fmt against code
fmt:
	go fmt ./controllers/... ./ipam/... .
	cd $(APIS_DIR); go fmt ./...

# Run go vet against code
vet:
	go vet ./controllers/... ./ipam/... .
	cd $(APIS_DIR); go vet ./...


## --------------------------------------
## Generate
## --------------------------------------

.PHONY: modules
modules: ## Runs go mod to ensure proper vendoring.
	go mod tidy
	go mod verify
	cd $(TOOLS_DIR); go mod tidy
	cd $(TOOLS_DIR); go mod verify
	cd $(APIS_DIR); go mod tidy
	cd $(APIS_DIR); go mod verify

.PHONY: generate
generate: ## Generate code
	$(MAKE) generate-go
	$(MAKE) generate-manifests

.PHONY: generate-go
generate-go: $(CONTROLLER_GEN) $(MOCKGEN) $(CONVERSION_GEN) $(KUBEBUILDER) $(KUSTOMIZE) ## Runs Go related generate targets
	go generate ./...
	cd $(APIS_DIR); go generate ./...
	cd ./api; ../$(CONTROLLER_GEN) \
		paths=./... \
		object:headerFile=../hack/boilerplate/boilerplate.generatego.txt

	$(MOCKGEN) \
	  -destination=./ipam/mocks/zz_generated.ippool_manager.go \
	  -source=./ipam/ippool_manager.go \
		-package=ipam_mocks \
		-copyright_file=./hack/boilerplate/boilerplate.generatego.txt \
		PoolManagerInterface

	$(MOCKGEN) \
	  -destination=./ipam/mocks/zz_generated.manager_factory.go \
	  -source=./ipam/manager_factory.go \
		-package=ipam_mocks \
		-copyright_file=./hack/boilerplate/boilerplate.generatego.txt \
		ManagerFactoryInterface

.PHONY: generate-manifests
generate-manifests: $(CONTROLLER_GEN) ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) \
		paths=./ \
		paths=./api/... \
		paths=./controllers/... \
		crd:crdVersions=v1 \
		rbac:roleName=manager-role \
		output:crd:dir=$(CRD_ROOT) \
		output:rbac:dir=$(RBAC_ROOT) \
		output:webhook:dir=$(WEBHOOK_ROOT) \
		webhook

.PHONY: generate-examples
generate-examples: clean-examples ## Generate examples configurations to run a cluster.
	./examples/generate.sh

## --------------------------------------
## Docker
## --------------------------------------

.PHONY: docker-build
docker-build: ## Build the docker image for controller-manager
	docker build --network=host --pull --build-arg ARCH=$(ARCH) . -t $(CONTROLLER_IMG)-$(ARCH):$(TAG)
	MANIFEST_IMG=$(CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) $(MAKE) set-manifest-image
	$(MAKE) set-manifest-pull-policy

.PHONY: docker-push
docker-push: ## Push the docker image
	docker push $(CONTROLLER_IMG)-$(ARCH):$(TAG)

## --------------------------------------
## Docker — All ARCH
## --------------------------------------

.PHONY: docker-build-all ## Build all the architecture docker images
docker-build-all: $(addprefix docker-build-,$(ALL_ARCH))

docker-build-%:
	$(MAKE) ARCH=$* docker-build

.PHONY: docker-push-all ## Push all the architecture docker images
docker-push-all: $(addprefix docker-push-,$(ALL_ARCH))
	$(MAKE) docker-push-manifest

docker-push-%:
	$(MAKE) ARCH=$* docker-push

.PHONY: docker-push-manifest
docker-push-manifest: ## Push the fat manifest docker image.
	## Minimum docker version 18.06.0 is required for creating and pushing manifest images.
	docker manifest create --amend $(CONTROLLER_IMG):$(TAG) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(CONTROLLER_IMG)\-&:$(TAG)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${CONTROLLER_IMG}:${TAG} ${CONTROLLER_IMG}-$${arch}:${TAG}; done
	docker manifest push --purge ${CONTROLLER_IMG}:${TAG}
	MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(TAG) $(MAKE) set-manifest-image
	$(MAKE) set-manifest-pull-policy

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for manager resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' ./config/default/manager_image_patch.yaml


.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for manager resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/default/manager_pull_policy_patch.yaml

## --------------------------------------
## Deploying
## --------------------------------------

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: generate-examples
	kubectl apply -f examples/_out/cert-manager.yaml
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment cert-manager
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment cert-manager-cainjector
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment cert-manager-webhook
	kubectl apply -f examples/_out/provider-components.yaml

.PHONY: kind-create
kind-create: ## create ipam kind cluster if needed
	./hack/kind_with_registry.sh

deploy-examples:
	kubectl apply -f ./examples/_out/ippool.yaml

delete-examples:
	kubectl delete -f ./examples/_out/ippool.yaml || true

.PHONY: kind-reset
kind-reset: ## Destroys the "ipam" kind cluster.
	kind delete cluster --name=ipam || true

## --------------------------------------
## Release
## --------------------------------------

RELEASE_TAG ?= $(shell git describe --abbrev=0 2>/dev/null)
RELEASE_DIR := out
RELEASE_NOTES_DIR := releasenotes
PREVIOUS_TAG ?= $(shell git tag -l | grep -B 1 $(RELEASE_TAG) | head -n 1)

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

$(RELEASE_NOTES_DIR):
	mkdir -p $(RELEASE_NOTES_DIR)/

.PHONY: release-manifests
release-manifests: $(KUSTOMIZE) $(RELEASE_DIR) ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/ipam-components.yaml

.PHONY: release-notes
release-notes: $(RELEASE_NOTES_DIR) $(RELEASE_NOTES)
	go run ./hack/tools/release/notes.go --from=$(PREVIOUS_TAG) > $(RELEASE_NOTES_DIR)/releasenotes.md

.PHONY: release
release:
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "You have uncommitted changes"; exit 1; fi
	git checkout "${RELEASE_TAG}"
	MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(RELEASE_TAG) $(MAKE) set-manifest-image
	$(MAKE) release-manifests
	$(MAKE) release-notes

## --------------------------------------
## Cleanup / Verification
## --------------------------------------

.PHONY: clean
clean: ## Remove all generated files
	$(MAKE) clean-bin
	$(MAKE) clean-temporary

.PHONY: clean-bin
clean-bin: ## Remove all generated binaries
	rm -rf bin
	rm -rf hack/tools/bin

.PHONY: clean-temporary
clean-temporary: ## Remove all temporary files and folders
	rm -f minikube.kubeconfig
	rm -f kubeconfig

.PHONY: clean-release
clean-release: ## Remove the release folder
	rm -rf $(RELEASE_DIR)

.PHONY: clean-examples
clean-examples: ## Remove all the temporary files generated in the examples folder
	rm -rf examples/_out/
	rm -f examples/provider-components/provider-components-*.yaml

.PHONY: verify
verify: verify-boilerplate verify-modules

.PHONY: verify-boilerplate
verify-boilerplate:
	./hack/verify-boilerplate.sh

.PHONY: verify-modules
verify-modules: modules
	@if !(git diff --quiet HEAD -- go.sum go.mod hack/tools/go.mod hack/tools/go.sum); then \
		echo "go module files are out of date"; exit 1; \
	fi

go-version: ## Print the go version we use to compile our binaries and images
	@echo $(GO_VERSION)
