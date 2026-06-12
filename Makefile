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

GO_VERSION ?= 1.25.11
# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

# Directories.
ROOT_DIR := $(shell pwd)
TOOLS_DIR := hack/tools
APIS_DIR := api
TEST_DIR := test
WEBHOOKS_DIR := internal/webhooks
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
BIN_DIR := bin

# Binaries.
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/controller-gen
GOLANGCI_LINT := $(ROOT_DIR)/$(TOOLS_BIN_DIR)/golangci-lint
MOCKGEN := $(TOOLS_BIN_DIR)/mockgen
CONVERSION_GEN := $(TOOLS_BIN_DIR)/conversion-gen
KUBEBUILDER := $(TOOLS_BIN_DIR)/kubebuilder
KUSTOMIZE := $(TOOLS_BIN_DIR)/kustomize
SETUP_ENVTEST_BIN := setup-envtest
SETUP_ENVTEST := $(abspath $(TOOLS_BIN_DIR)/$(SETUP_ENVTEST_BIN))

# Define Docker related variables. Releases should modify and double check these vars.
# REGISTRY ?= gcr.io/$(shell gcloud config get-value project)
REGISTRY ?= quay.io/metal3-io
IMAGE_NAME ?= ip-address-manager
CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
TAG ?= v1alpha1
ARCH ?= $(shell go env GOARCH)
ALL_ARCH = amd64 arm arm64 ppc64le s390x

# Allow overriding manifest generation destination directory
MANIFEST_ROOT ?= config
CRD_ROOT ?= $(MANIFEST_ROOT)/crd/bases
METAL3_CRD_ROOT ?= $(MANIFEST_ROOT)/crd/metal3
WEBHOOK_ROOT ?= $(MANIFEST_ROOT)/webhook
RBAC_ROOT ?= $(MANIFEST_ROOT)/rbac

# Allow overriding the imagePullPolicy
PULL_POLICY ?= IfNotPresent

# Testing
COVER_PROFILE = cover.out

## --------------------------------------
## Help
## --------------------------------------

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Testing
## --------------------------------------

export KUBEBUILDER_ENVTEST_KUBERNETES_VERSION ?= 1.36.0
KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env -p path $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))

.PHONY: setup-envtest
setup-envtest: $(SETUP_ENVTEST) ## Set up envtest (download kubebuilder assets)
	@echo KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS)

.PHONY: unit
unit: $(SETUP_ENVTEST) ## Run tests
	$(shell $(SETUP_ENVTEST) use -p env $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION)) && \
	go test -v ./controllers/... ./ipam/... -coverprofile ./cover.out && \
	cd $(APIS_DIR) && go test -v ./... -coverprofile ./cover.out && cd .. && \
	cd $(WEBHOOKS_DIR) && go test -v ./... -coverprofile ./cover.out

.PHONY: test  ## Run linter and tests
test: generate lint unit

.PHONY: unit-cover
unit-cover: $(SETUP_ENVTEST) ## Run unit tests with code coverage
	$(shell $(SETUP_ENVTEST) use -p env $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION)) && \
	go test ./controllers/... ./ipam/... -coverprofile=$(COVER_PROFILE) && \
	go tool cover -func=$(COVER_PROFILE) && \
	cd $(APIS_DIR) && go test -coverprofile=$(COVER_PROFILE) ./... && \
	go tool cover -func=$(COVER_PROFILE) && cd .. && \
	cd $(WEBHOOKS_DIR) && go test -coverprofile=$(COVER_PROFILE) ./... && \
	go tool cover -func=$(COVER_PROFILE)

FUZZ_TIME ?= 30s

.PHONY: fuzz
fuzz: ## Run fuzz tests with seed corpus (no fuzzing, regression test only)
	cd test/fuzz && go test -race -v ./...

.PHONY: fuzz-run
fuzz-run: ## Run all fuzz tests sequentially with fuzzing enabled (use FUZZ_TIME=duration)
	@echo "Discovering fuzz tests..."
	@cd test/fuzz && go test -list='Fuzz.*' | grep '^Fuzz' | while read -r fuzz_test; do \
		echo "Running $$fuzz_test for $(FUZZ_TIME)..."; \
		go test  -fuzz=$$fuzz_test -fuzztime='$(FUZZ_TIME)' || exit 1; \
	done
	@echo "All fuzz tests completed successfully!"

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
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/mockgen go.uber.org/mock/mockgen

$(CONVERSION_GEN): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/conversion-gen k8s.io/code-generator/cmd/conversion-gen

$(KUBEBUILDER): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); ./install_kubebuilder.sh

$(KUSTOMIZE): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); ./install_kustomize.sh

$(SETUP_ENVTEST):
	cd $(TOOLS_DIR); ./install_setup_envtest.sh

## --------------------------------------
## Linting
## --------------------------------------

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Lint codebase
	$(GOLANGCI_LINT) run -v $(GOLANGCI_LINT_EXTRA_ARGS) --timeout=10m
	cd $(APIS_DIR); $(GOLANGCI_LINT) run -v $(GOLANGCI_LINT_EXTRA_ARGS) --timeout=10m
	cd $(TOOLS_DIR)/release; $(GOLANGCI_LINT) run -v --build-tags=tools --modules-download-mode=readonly $(GOLANGCI_LINT_EXTRA_ARGS) --timeout=10m
	cd $(TEST_DIR); $(GOLANGCI_LINT) run -v --build-tags=e2e $(GOLANGCI_LINT_EXTRA_ARGS) --timeout=10m

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT) ## Lint the codebase and run auto-fixers if supported by the linter
	GOLANGCI_LINT_EXTRA_ARGS=--fix $(MAKE) lint

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
	cd $(TEST_DIR); go mod tidy
	cd $(TEST_DIR); go mod verify

.PHONY: generate
generate: ## Generate code
	$(MAKE) generate-go
	$(MAKE) generate-manifests

.PHONY: generate-go
generate-go: $(CONTROLLER_GEN) $(MOCKGEN) $(CONVERSION_GEN) $(KUBEBUILDER) $(KUSTOMIZE) ## Runs Go related generate targets
	go generate ./...
	cd $(APIS_DIR); go generate ./...
	cd $(WEBHOOKS_DIR); go generate ./...
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
		paths=./internal/webhooks/... \
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

.PHONY: docker-build-e2e
docker-build-e2e: ## Build the docker image for e2e tests (matches image name in e2e_conf.yaml)
	docker build --network=host --pull --build-arg ARCH=$(ARCH) . -t $(CONTROLLER_IMG):e2e-test

.PHONY: docker-build-debug
docker-build-debug: ## Build the docker image for controller-manager with debug info
	docker build --network=host --pull --build-arg LDFLAGS="-extldflags=-static" . \
	-t $(CONTROLLER_IMG):$(TAG)
	MANIFEST_IMG=$(CONTROLLER_IMG) \
	MANIFEST_TAG=$(TAG) \
	$(MAKE) set-manifest-image
	$(MAKE) set-manifest-pull-policy

## --------------------------------------
## Docker — All ARCH
## --------------------------------------

.PHONY: docker-build-all ## Build all the architecture docker images
docker-build-all: $(addprefix docker-build-,$(ALL_ARCH))

docker-build-%:
	$(MAKE) ARCH=$* docker-build

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for manager resource)
	sed -i'' -e 's@image: .*@image: \"'"${MANIFEST_IMG}:$(MANIFEST_TAG)"'\"@' ./config/default/manager_image_patch.yaml

.PHONY: set-manifest-image-digest
set-manifest-image-digest:
	$(info Updating kustomize image patch file for manager resource)
	@if [ -z "$(MANIFEST_DIGEST)" ]; then echo "MANIFEST_DIGEST is not set"; exit 1; fi
	sed -i'' -e 's@image: .*@image: \"'"${MANIFEST_IMG}@$(MANIFEST_DIGEST)"'\"@' ./config/default/manager_image_patch.yaml


.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for manager resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/default/manager_pull_policy_patch.yaml

## --------------------------------------
## Deploying
## --------------------------------------

deploy-examples:
	kubectl apply -f ./examples/_out/ippool.yaml

delete-examples:
	kubectl delete -f ./examples/_out/ippool.yaml || true

## --------------------------------------
## Release
## --------------------------------------

RELEASE_DIR := out
RELEASE_NOTES_DIR := releasenotes
$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

$(RELEASE_NOTES_DIR):
	mkdir -p $(RELEASE_NOTES_DIR)/

.PHONY: release-manifests
release-manifests: $(KUSTOMIZE) $(RELEASE_DIR) ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/ipam-components.yaml
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml

.PHONY: release-notes-tool
release-notes-tool:
	go build -C hack/tools -o $(BIN_DIR)/release -tags tools github.com/metal3-io/ip-address-manager/hack/tools/release

.PHONY: release-notes
release-notes: $(RELEASE_NOTES_DIR) $(RELEASE_NOTES) release-notes-tool
	$(TOOLS_BIN_DIR)/release --releaseTag=$(RELEASE_TAG) > $(RELEASE_NOTES_DIR)/$(RELEASE_TAG).md

.PHONY: release
release:
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "You have uncommitted changes"; exit 1; fi
	git checkout "${RELEASE_TAG}"
	@if [ -n "$(MANIFEST_DIGEST)" ]; then \
		MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_DIGEST=$(MANIFEST_DIGEST) $(MAKE) set-manifest-image-digest; \
	else \
		echo "MANIFEST_DIGEST is not set, falling back to tag"; \
		MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(RELEASE_TAG) $(MAKE) set-manifest-image; \
	fi
	$(MAKE) release-manifests

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

.PHONY: verify
verify: verify-boilerplate verify-modules

.PHONY: verify-boilerplate
verify-boilerplate:
	./hack/verify-boilerplate.sh

.PHONY: verify-modules
verify-modules: modules
	@if !(git diff --quiet HEAD -- go.sum go.mod hack/tools/go.mod hack/tools/go.sum test/go.mod test/go.sum); then \
		echo "go module files are out of date"; exit 1; \
	fi

go-version: ## Print the go version we use to compile our binaries and images
	@echo $(GO_VERSION)

## --------------------------------------
## E2e stuff
## --------------------------------------
GINKGO_FOCUS ?=
GINKGO_FOCUS_LABELS ?=
GINKGO_SKIP ?=
GINKGO_SKIP_LABELS ?=
GINKGO_NODES ?= 2
GINKGO_TIMEOUT ?= 30m
GINKGO_POLL_PROGRESS_AFTER ?= 10m
GINKGO_POLL_PROGRESS_INTERVAL ?= 2m
E2E_CONF_FILE ?= $(ROOT_DIR)/test/e2e/config/e2e_conf.yaml
USE_EXISTING_CLUSTER ?= false
SKIP_RESOURCE_CLEANUP ?= false
GINKGO_NOCOLOR ?= false
GINKGO := $(TOOLS_BIN_DIR)/ginkgo

# to set multiple ginkgo skip flags, if any
ifneq ($(strip $(GINKGO_SKIP)),)
_SKIP_ARGS := $(foreach arg,$(strip $(GINKGO_SKIP)),-skip="$(arg)")
endif

# to set multiple ginkgo skip labels, if any
ifneq ($(strip $(GINKGO_SKIP_LABELS)),)
_SKIP_LABELS_ARGS := --label-filter="!$(GINKGO_SKIP_LABELS)"
endif

# to focus on specific labels
ifneq ($(strip $(GINKGO_FOCUS_LABELS)),)
_FOCUS_LABELS_ARGS := --label-filter="$(GINKGO_FOCUS_LABELS)"
endif

ARTIFACTS ?= ${ROOT_DIR}/test/e2e/_artifacts

.PHONY: test-e2e
# Only runs tests make sure to build the e2e test image before running.
# You can use the docker-build-e2e target to build the image.
# features only: make test-e2e GINKGO_SKIP_LABELS=basic make test-e2e GINKGO_FOCUS_LABELS=features
# basic only: make test-e2e GINKGO_SKIP_LABELS=features or make test-e2e GINKGO_FOCUS_LABELS=basic
# all tests: make test-e2e
test-e2e: $(GINKGO) $(KUSTOMIZE) ## Run the end-to-end tests
	PATH=$(abspath $(TOOLS_BIN_DIR)):$(PATH) \
	$(GINKGO) -v --trace -poll-progress-after=$(GINKGO_POLL_PROGRESS_AFTER) \
		-poll-progress-interval=$(GINKGO_POLL_PROGRESS_INTERVAL) --tags=e2e \
		--focus="$(GINKGO_FOCUS)" \
		$(_SKIP_ARGS)  $(_SKIP_LABELS_ARGS) $(_FOCUS_LABELS_ARGS) \
		--nodes=$(GINKGO_NODES) \
		--timeout=$(GINKGO_TIMEOUT) \
		--no-color=$(GINKGO_NOCOLOR) \
		--output-dir="$(ARTIFACTS)" \
		--junit-report="junit.e2e_suite.1.xml" \
		$(GINKGO_ARGS) test/e2e -- \
		-e2e.config="$(E2E_CONF_FILE)" \
		-e2e.use-existing-cluster=$(USE_EXISTING_CLUSTER) \
		-e2e.skip-resource-cleanup=$(SKIP_RESOURCE_CLEANUP) \
		-e2e.artifacts-folder="$(ARTIFACTS)"

$(GINKGO): $(TOOLS_DIR)/go.mod ## Build ginkgo from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/ginkgo github.com/onsi/ginkgo/v2/ginkgo
