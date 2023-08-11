
# Image URL to use all building/pushing image targets
VERSION ?= v0.8.1
IMG ?= docker.io/substratusai/controller-manager:${VERSION}
IMG_GCPMANAGER ?= docker.io/substratusai/gcp-manager:${VERSION}

# Set to false if you don't want GPU nodepools created
ATTACH_GPU_NODEPOOLS=true


# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.26.1

PLATFORM=$(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(shell uname -m | sed 's/x86_64/amd64/')

PROJECT_ID := $(shell gcloud config get-value project)
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Linux)
	PROTOC_OS := linux
	SKAFFOLD_OS := linux
else
	ifeq ($(UNAME_S),Darwin)
		PROTOC_OS := osx
		SKAFFOLD_OS := darwin
	else
		PROTOC_OS := $(UNAME_S)
		SKAFFOLD_OS := $(UNAME_S)
	endif
endif

ifeq ($(UNAME_M),arm64)
	PROTOC_ARCH := aarch_64
	SKAFFOLD_ARCH := arm64
else
	PROTOC_ARCH := $(UNAME_M)
	SKAFFOLD_ARCH := $(UNAME_M)
endif

PROTOC_PLATFORM := $(PROTOC_OS)-$(PROTOC_ARCH)
SKAFFOLD_PLATFORM := $(SKAFFOLD_OS)-$(SKAFFOLD_ARCH)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

export PATH := $(PATH):$(GOBIN):$(PWD)/bin:
# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

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
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf " \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate protogen fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -v -coverprofile cover.out

.PHONY: test-kubectl
test-kubectl: manifests fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./kubectl/internal/commands -v

.PHONY: render-skaffold-manifests
render-skaffold-manifests: envsubst ## run envsubs against skaffold manifest tesmplates
	@ if [ -n ${PROJECT_ID} ]; then export PROJECT_ID=$(shell gcloud config get-value project); fi && \
	envsubst < config/skaffold-dependencies.sh.tpl > config/skaffold-dependencies.sh && \
	chmod +x config/skaffold-dependencies.sh && \
	envsubst < config/gcpmanager/gcpmanager-dependencies.yaml.tpl > config/gcpmanager/gcpmanager-dependencies.yaml && \
	envsubst < config/gcpmanager/gcpmanager-skaffold.yaml.tpl > config/gcpmanager/gcpmanager-skaffold.yaml

.PHONY: skaffold-dev-gcpmanager
skaffold-dev-gcpmanager: protoc skaffold protogen render-skaffold-manifests ## Run skaffold dev against gcpmanager
	config/skaffold-dependencies.sh && \
	skaffold dev -f config/gcpmanager/gcpmanager-skaffold.yaml

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/controllermanager/main.go

.PHONY: dev-up-gcp
dev-up-gcp: build-installer
	docker run -it \
		-v ${HOME}/.kube:/root/.kube \
		-e PROJECT=$(shell gcloud config get project) \
		-e TOKEN=$(shell gcloud auth print-access-token) \
		-e TF_VAR_attach_gpu_nodepools=${ATTACH_GPU_NODEPOOLS} \
		-e INSTALL_OPERATOR=false \
		substratus-installer gcp-up.sh
	mkdir -p secrets
	gcloud iam service-accounts keys create --iam-account=substratus-gcp-manager@$(shell gcloud config get project).iam.gserviceaccount.com ./secrets/gcp-manager-key.json

.PHONY: dev-down-gcp
dev-down-gcp: build-installer
	docker run -it \
		-v ${HOME}/.kube:/root/.kube \
		-e PROJECT=$(shell gcloud config get project) \
		-e TOKEN=$(shell gcloud auth print-access-token) \
		-e TF_VAR_attach_gpu_nodepools=${ATTACH_GPU_NODEPOOLS} \
		substratus-installer gcp-down.sh
	rm ./secrets/gcp-manager-key.json

.PHONY: dev-up-kind
dev-up-kind:
	cd install/scripts && ./kind-up.sh

.PHONY: dev-run-kind
# Controller manager configuration #
dev-run-kind: export CLOUD=kind
dev-run-kind: export CLUSTER_NAME=substratus
dev-run-kind: export ARTIFACT_BUCKET_URL=kind://bucket
dev-run-kind: export REGISTRY_URL=http://docker.svc.cluster.local:5000
# Run the controller manager and the sci.
dev-run-kind: manifests kustomize install-crds
	go run ./cmd/sci-kind & \
	go run ./cmd/controllermanager/main.go \
		--sci-address=localhost:10080 \
		--config-dump-path=/tmp/substratus-config.yaml

.PHONY: dev-down-kind
dev-down-kind:
	cd install/scripts && ./kind-down.sh


.PHONY: dev-up-aws
dev-up-aws: build-installer
	docker run -it \
		-v ${HOME}/.kube:/root/.kube \
		-e AWS_ACCOUNT_ID="$(shell aws sts get-caller-identity --query Account --output text)" \
		-e AWS_ACCESS_KEY_ID=$(shell aws configure get aws_access_key_id) \
		-e AWS_SECRET_ACCESS_KEY=$(shell aws configure get aws_secret_access_key) \
		-e AWS_SESSION_TOKEN=$(shell aws configure get aws_session_token) \
		-e INSTALL_OPERATOR=false \
		substratus-installer aws-up.sh

.PHONY: dev-down-aws
dev-down-aws: build-installer
	docker run -it \
		-v ${HOME}/.kube:/root/.kube \
		-e AWS_ACCOUNT_ID="$(shell aws sts get-caller-identity --query Account --output text)" \
		-e AWS_ACCESS_KEY_ID=$(shell aws configure get aws_access_key_id) \
		-e AWS_SECRET_ACCESS_KEY=$(shell aws configure get aws_secret_access_key) \
		-e AWS_SESSION_TOKEN=$(shell aws configure get aws_session_token) \
		substratus-installer aws-down.sh

.PHONY: dev-run-gcp
# Controller manager configuration #
dev-run-gcp: export CLOUD=gcp
dev-run-gcp: export GPU_TYPE=nvidia-l4
dev-run-gcp: export PROJECT_ID=$(shell gcloud config get project)
dev-run-gcp: export CLUSTER_NAME=substratus
dev-run-gcp: export CLUSTER_LOCATION=us-central1
# Cloud manager configuration #
dev-run-gcp: export GOOGLE_APPLICATION_CREDENTIALS=./secrets/gcp-manager-key.json
# Run the controller manager and the cloud manager.
dev-run-gcp: manifests kustomize install-crds
	go run ./cmd/gcpmanager & \
	go run ./cmd/controllermanager/main.go \
		--sci-address=localhost:10080 \
		--config-dump-path=/tmp/substratus-config.yaml

.PHONY: run
run: ## Run a controller from your host.
	go run ./cmd/controllermanager/main.go

# If you wish built the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64 ). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

.PHONY: docs
docs: crd-ref-docs embedmd
	$(CRD_REF_DOCS) \
		--config=./docs/api/config.yaml \
		--log-level=INFO \
		--output-path=./docs/api/generated.md \
		--source-path=./api \
		--templates-dir=./docs/api/templates/markdown \
		--renderer=markdown
	# TODO: Embed YAML examples into the generate API documentation.
	# $(EMBEDMD) -w ./docs/api/generated.md

# PLATFORMS defines the target platforms for the manager image be build to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - able to use docker buildx . More info: https://docs.docker.com/build/buildx/
# - have enable BuildKit, More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image for your registry (i.e. if you do not inform a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To properly provided solutions that supports more than one platform you should use this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: test ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- docker buildx create --name project-v3-builder
	docker buildx use project-v3-builder
	- docker buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- docker buildx rm project-v3-builder
	rm Dockerfile.cross

.PHONY: protogen
protogen: protoc ## Generate protobuf files.
	cd internal/sci && \
	protoc \
		-I$(LOCALBIN)/include \
		-I. \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		sci.proto

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install-crds
install-crds: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall-crds
uninstall-crds: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: installation-scripts
installation-scripts:
	perl -pi -e "s/version=.*/version=$(VERSION)/g" install/scripts/install-kubectl-plugins.sh

.PHONY: installation-manifests
installation-manifests: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	cd config/gcpmanager && $(KUSTOMIZE) edit set image gcp-manager=${IMG_GCPMANAGER}
	$(KUSTOMIZE) build config/install-gcp > install/kubernetes/system.yaml
	$(KUSTOMIZE) build config/install-kind > install/kubernetes/kind/system.yaml

.PHONY: prepare-release
prepare-release: installation-scripts installation-manifests docs

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
EMBEDMD ?= $(LOCALBIN)/embedmd
CRD_REF_DOCS ?= $(LOCALBIN)/crd-ref-docs
PROTOC ?= $(LOCALBIN)/protoc

## Tool Versions
KUSTOMIZE_VERSION ?= v5.0.0
CONTROLLER_TOOLS_VERSION ?= v0.11.3
CRD_REF_DOCS_VERSION ?= v0.0.9
PROTOC_VERSION ?= 23.4
PROTOC_GEN_GO_GRPC_VERSION ?= v1.1.0
PROTOC_GEN_GO_VERSION ?= v1.31.0

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) --output install_kustomize.sh && bash install_kustomize.sh $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); rm install_kustomize.sh; }

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs.
$(CRD_REF_DOCS): $(LOCALBIN)
	test -s $(LOCALBIN)/crd-ref-docs || \
	GOBIN=$(LOCALBIN) go install github.com/elastic/crd-ref-docs@$(CRD_REF_DOCS_VERSION)

.PHONY: embedmd
embedmd: $(EMBEDMD)
$(EMBEDMD): $(LOCALBIN)
	test -s $(LOCALBIN)/embedmd || GOBIN=$(LOCALBIN) go install github.com/campoy/embedmd@latest

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: protoc
protoc: $(PROTOC) ## download and install protoc.
$(PROTOC): $(LOCALBIN)
	@if ! test -x $(LOCALBIN)/protoc || ! test -d $(LOCALBIN)/include; then \
		curl -L https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-$(PROTOC_PLATFORM).zip -o /tmp/protoc-${PROTOC_VERSION}-$(PROTOC_PLATFORM).zip; \
		unzip /tmp/protoc-${PROTOC_VERSION}-$(PROTOC_PLATFORM).zip -d /tmp/protoc/; \
		cp /tmp/protoc/bin/protoc $(LOCALBIN)/protoc; \
		cp -r /tmp/protoc/include $(LOCALBIN)/; \
		rm -rf /tmp/protoc/; \
		rm /tmp/protoc-${PROTOC_VERSION}-$(PROTOC_PLATFORM).zip; \
		go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@${PROTOC_GEN_GO_GRPC_VERSION}; \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION); \
	fi

.PHONY: skaffold
skaffold:
	@ test -s $(LOCALBIN)/skaffold || \
	( curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-$(SKAFFOLD_PLATFORM) && \
	chmod +x skaffold && \
	mv skaffold $(LOCALBIN)/skaffold )

.PHONY: envsubst
envsubst:
	@ test -s $(LOCALBIN)/envsubst || \
	( curl -L https://github.com/a8m/envsubst/releases/download/v1.2.0/envsubst-`uname -s`-`uname -m` -o envsubst && \
	chmod +x envsubst && \
	mv envsubst $(LOCALBIN)/envsubst )

.PHONY: build-installer
build-installer: ## Build the GCP installer.
	@ docker build ./install -t substratus-installer
