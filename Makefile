# PROJECT_VERSION defines the project version for the bundle.
# Read from VERSION file or override with environment variable
PROJECT_VERSION ?= $(shell cat VERSION 2>/dev/null || echo "0.0.1")

GIT_COMMIT ?= $(shell git rev-parse --short HEAD)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
IMAGE_TAG_BASE ?= public.ecr.aws/os-partners/neuron-openshift/operator

# TEST_IMAGE_TAG_BASE defines the private ECR registry for test images
TEST_IMAGE_TAG_BASE ?= 582767206473.dkr.ecr.us-east-1.amazonaws.com/neuron-openshift

# This is the default tag of all images made by this Makefile.
IMAGE_TAG ?= v$(PROJECT_VERSION)

# Determine if we're in test mode based on IMAGE_TAG_BASE
ifeq ($(IMAGE_TAG_BASE),$(TEST_IMAGE_TAG_BASE))
TEST_MODE := true
REGISTRY_BASE := $(TEST_IMAGE_TAG_BASE)
else
TEST_MODE := false
REGISTRY_BASE := public.ecr.aws/os-partners/neuron-openshift
endif

# Image URL to use all building/pushing image targets
IMG ?= $(REGISTRY_BASE)/operator:$(IMAGE_TAG)

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(REGISTRY_BASE)/operator-bundle:$(IMAGE_TAG)

INDEX_IMG := $(REGISTRY_BASE)/operator-index:$(IMAGE_TAG)


# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(PROJECT_VERSION) $(BUNDLE_METADATA_OPTS)

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.23

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

OS := $(shell GOTOOLCHAIN=local go env GOOS)
ARCH := $(shell GOTOOLCHAIN=local go env GOARCH)

ifeq ($(OS),darwin)
export GOPROXY=direct
export GOSUMDB=sum.golang.org
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: generate manager manifests

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./internal/controllers" output:rbac:artifacts:config=config/rbac

.PHONY: generate
generate: controller-gen mockgen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	go generate ./...

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

TEST ?= ./...

.PHONY: unit-test
unit-test: vet ## Run tests.
	go test $(TEST) -coverprofile cover.out

GOFILES_NO_VENDOR = $(shell find . -type f -name '*.go' ! -path "./vendor/*")
.PHONY: lint
lint: golangci-lint ## Run golangci-lint against code.
	@if [ `gofmt -l $(GOFILES_NO_VENDOR) | wc -l` -ne 0 ]; then \
		echo There are some malformed files, please make sure to run \'make fmt\'; \
		gofmt -l $(GOFILES_NO_VENDOR); \
		exit 1; \
	fi
	$(GOLANGCI_LINT) run -v --timeout 5m0s

##@ Build

manager: $(shell find . -name "*.go") go.mod go.sum  ## Build manager binary.
	GOARCH=amd64 GOOS=linux go build -ldflags="-X main.Version=$(PROJECT_VERSION) -X main.GitCommit=$(GIT_COMMIT)" -o $@ ./cmd

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker buildx build --platform linux/amd64 -t $(IMG) --build-arg TARGET=manager --build-arg VERSION=$(PROJECT_VERSION) --push .

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

KUSTOMIZE_CONFIG_CRD ?= config/crd

.PHONY: install
install: manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl apply -k $(KUSTOMIZE_CONFIG_CRD)

.PHONY: uninstall
uninstall: manifests ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	kubectl delete -k $(KUSTOMIZE_CONFIG_CRD) --ignore-not-found=$(ignore-not-found)

KUSTOMIZE_CONFIG_DEFAULT ?= config/default
KUSTOMIZE_CONFIG_HUB_DEFAULT ?= config/default-hub

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	kubectl apply -k $(KUSTOMIZE_CONFIG_DEFAULT)

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	kubectl delete -k $(KUSTOMIZE_CONFIG_DEFAULT) --ignore-not-found=$(ignore-not-found)

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.1)

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
.PHONY: golangci-lint
golangci-lint: ## Download golangci-lint locally if necessary.
	$(call go-get-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2)

.PHONY: mockgen
mockgen: ## Install mockgen locally.
	go install go.uber.org/mock/mockgen@v0.3.0

KUSTOMIZE = $(shell pwd)/bin/kustomize
.PHONY: kustomize
kustomize: ## Download kustomize locally if necessary.
	@if [ ! -f ${KUSTOMIZE} ]; then \
		BINDIR=$(shell pwd)/bin ./hack/download-kustomize; \
	fi


# go-get-tool will 'go install' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
}
endef

OPERATOR_SDK = $(shell pwd)/bin/operator-sdk
.PHONY: operator-sdk
operator-sdk:
	@if [ ! -f ${OPERATOR_SDK} ]; then \
		set -e ;\
		echo "Downloading ${OPERATOR_SDK}"; \
		mkdir -p $(dir ${OPERATOR_SDK}) ; \
		curl -Lo ${OPERATOR_SDK} 'https://github.com/operator-framework/operator-sdk/releases/download/v1.32.0/operator-sdk_${OS}_${ARCH}'; \
		chmod +x ${OPERATOR_SDK}; \
	fi

SUGGESTED_NAMESPACE := $(shell yq -r '.namespace' config/default/kustomization.yaml)
.PHONY: bundle
bundle: operator-sdk manifests kustomize
	rm -fr ./bundle
	${OPERATOR_SDK} generate kustomize manifests --apis-dir api
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	OPERATOR_SDK="${OPERATOR_SDK}" \
		     BUNDLE_GEN_FLAGS="${BUNDLE_GEN_FLAGS} --extra-service-accounts awslabs-gpu-operator-kmm-device-plugin,awslabs-gpu-operator-kmm-module-loader,awslabs-gpu-operator-neuron-scheduler,awslabs-gpu-operator-neuron-scheduler-extension" \
		     PKG=aws-neuron-operator \
		     SOURCE_DIR=$(dir $(realpath $(lastword $(MAKEFILE_LIST)))) \
		     ./hack/generate-bundle
	yq -i '.metadata.annotations."operatorframework.io/suggested-namespace" = "$(SUGGESTED_NAMESPACE)"' \
		bundle/manifests/*.clusterserviceversion.yaml
	${OPERATOR_SDK} bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker buildx build --platform linux/amd64,linux/arm64 -f bundle.Dockerfile -t $(BUNDLE_IMG) --push .

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.31.0/${OS}-${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

.PHONY: index
index: opm
	${OPM} index add --bundles ${BUNDLE_IMG} --tag ${INDEX_IMG} --generate -d Dockerfile.index --container-tool docker
	docker buildx build --platform linux/amd64 -t $(INDEX_IMG) -f Dockerfile.index --push .
	rm -rf Dockerfile.index database

##@ Test Pipeline

.PHONY: test-docker-build
test-docker-build: ## Build docker image for test pipeline with private registry.
	$(eval IMAGE_TAG_BASE := $(TEST_IMAGE_TAG_BASE))
	$(eval IMG := $(TEST_IMAGE_TAG_BASE)/operator:$(IMAGE_TAG))
	docker buildx build --platform linux/amd64 -t $(IMG) --build-arg TARGET=manager --build-arg VERSION=$(PROJECT_VERSION) --push .

.PHONY: test-bundle
test-bundle: operator-sdk manifests kustomize ## Generate test bundle for private registry.
	$(eval IMAGE_TAG_BASE := $(TEST_IMAGE_TAG_BASE))
	$(eval IMG := $(TEST_IMAGE_TAG_BASE)/operator:$(IMAGE_TAG))
	$(eval BUNDLE_IMG := $(TEST_IMAGE_TAG_BASE)/operator-bundle:$(IMAGE_TAG))
	rm -fr ./bundle
	${OPERATOR_SDK} generate kustomize manifests --apis-dir api
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	OPERATOR_SDK="${OPERATOR_SDK}" \
		     BUNDLE_GEN_FLAGS="${BUNDLE_GEN_FLAGS} --extra-service-accounts awslabs-gpu-operator-kmm-device-plugin,awslabs-gpu-operator-kmm-module-loader,awslabs-gpu-operator-neuron-scheduler,awslabs-gpu-operator-neuron-scheduler-extension" \
		     PKG=aws-neuron-operator \
		     SOURCE_DIR=$(dir $(realpath $(lastword $(MAKEFILE_LIST)))) \
		     ./hack/generate-bundle
	${OPERATOR_SDK} bundle validate ./bundle

.PHONY: test-bundle-build
test-bundle-build: ## Build test bundle image for private registry.
	$(eval BUNDLE_IMG := $(TEST_IMAGE_TAG_BASE)/operator-bundle:$(IMAGE_TAG))
	docker buildx build --platform linux/amd64,linux/arm64 -f bundle.Dockerfile -t $(BUNDLE_IMG) --push .

.PHONY: test-index
test-index: opm ## Generate test index for private registry.
	$(eval BUNDLE_IMG := $(TEST_IMAGE_TAG_BASE)/operator-bundle:$(IMAGE_TAG))
	$(eval INDEX_IMG := $(TEST_IMAGE_TAG_BASE)/operator-index:$(IMAGE_TAG))
	${OPM} index add --bundles ${BUNDLE_IMG} --tag ${INDEX_IMG} --generate -d Dockerfile.index --container-tool docker
	docker buildx build --platform linux/amd64 -t $(INDEX_IMG) -f Dockerfile.index --push .
	rm -rf Dockerfile.index database

.PHONY: test-manifests
test-manifests: manifests kustomize ## Generate test manifests for private registry.
	$(eval IMG := $(TEST_IMAGE_TAG_BASE)/operator:$(IMAGE_TAG))
	./hack/generate-release-manifests.sh $(PROJECT_VERSION) $(IMG) --test-mode

##@ Release

.PHONY: version
version: ## Show current version
	@echo $(PROJECT_VERSION)



.PHONY: release-manifests
release-manifests: manifests kustomize ## Generate customer-ready manifests for release.
	./hack/generate-release-manifests.sh $(PROJECT_VERSION) $(IMG)
