SHELL = /bin/bash -eu -o pipefail

# Image URL to use all building/pushing image targets
KUBELB_IMG ?= quay.io/kubermatic/kubelb-manager-ee
KUBELB_CCM_IMG ?= quay.io/kubermatic/kubelb-ccm-ee
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.28.2

export GOPATH?=$(shell go env GOPATH)
export CGO_ENABLED=0
export GOPROXY?=https://proxy.golang.org
export GO111MODULE=on
export GOFLAGS?=-mod=readonly -trimpath
export GIT_TAG ?= $(shell git tag --points-at HEAD)

GO_VERSION = 1.21.0

IMAGE_TAG = \
		$(shell echo $$(git rev-parse HEAD && if [[ -n $$(git status --porcelain) ]]; then echo '-dirty'; fi)|tr -d ' ')
CCM_IMAGE_NAME ?= $(KUBELB_CCM_IMG):$(IMAGE_TAG)
KUBELB_IMAGE_NAME ?= $(KUBELB_IMG):$(IMAGE_TAG)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build-kubelb build-ccm

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
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) rbac:roleName=kubelb-ccm paths="./pkg/controllers/ccm/..." output:artifacts:config=config/ccm/rbac
	$(CONTROLLER_GEN) rbac:roleName=kubelb paths="./pkg/controllers/kubelb/..." output:artifacts:config=config/kubelb/rbac

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

lint: ## Run golangci-lint against code.
	golangci-lint run -v --timeout=5m

yamllint:  ## Run yamllint against code.
	yamllint -c .yamllint.conf .

check-dependencies: ## Verify go.mod.
	go mod verify

verify-boilerplate:  ## Run verify-boilerplate code.
	./hack/verify-boilerplate.sh

verify-imports:  ## Run verify-imports code.
	./hack/verify-import-order.sh

clean:  ## Clean binaries
	rm -rf bin/*

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: build-ccm build-kubelb

build-%: generate fmt vet ## Build manager binary.
	CGO_ENABLED=0 go build -v -o bin/$* cmd/$*/main.go

.PHONY: run
run-%: manifests generate fmt vet ## Run a controller from your host.
	go run cmd/$*/main.go

.PHONY: download-gocache
download-gocache:
	@./hack/ci/download-gocache.sh

# If you wish built the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64 ). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-image
docker-image:
	docker build --build-arg GO_VERSION=$(GO_VERSION) -t ${KUBELB_IMAGE_NAME} -f kubelb.dockerfile .
	docker build --build-arg GO_VERSION=$(GO_VERSION) -t ${CCM_IMAGE_NAME} -f ccm.dockerfile .

.PHONY: docker-image-publish
docker-image-publish: docker-image
	docker push $(KUBELB_IMAGE_NAME)
	docker push $(CCM_IMAGE_NAME)
	if [[ -n "$(GIT_TAG)" ]]; then \
		$(eval IMAGE_TAG = $(GIT_TAG)) \
		docker build -t $(KUBELB_IMAGE_NAME) -f kubelb.dockerfile . && \
		docker push $(KUBELB_IMAGE_NAME) && \
		docker build -t $(CCM_IMAGE_NAME) . && \
		docker push $(CCM_IMAGE_NAME) && \
		$(eval IMAGE_TAG = latest) \
		docker build -t $(KUBELB_IMAGE_NAME) -f kubelb.dockerfile . ;\
		docker push $(KUBELB_IMAGE_NAME) ;\
		docker build -t $(CCM_IMAGE_NAME) -f ccm.dockerfile . ;\
		docker push $(CCM_IMAGE_NAME) ;\
	fi

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy-%: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
#	cd config/$* && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/deploy/$* | kubectl apply -f -

.PHONY: undeploy
undeploy-%: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/deploy/$* | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: bump
bump: kustomize
	cd config/deploy/kubelb && $(KUSTOMIZE) edit set image controller=quay.io/kubermatic/kubelb-manager-ee:$(IMAGE_TAG)
	cd config/deploy/ccm && $(KUSTOMIZE) edit set image controller=quay.io/kubermatic/kubelb-ccm-ee:$(IMAGE_TAG)

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v4.5.7
CONTROLLER_TOOLS_VERSION ?= v0.11.3

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
