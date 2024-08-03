SHELL = /bin/bash -eu -o pipefail

# Image URL to use all building/pushing image targets
KUBELB_IMG ?= quay.io/kubermatic/kubelb-manager
KUBELB_CCM_IMG ?= quay.io/kubermatic/kubelb-ccm

## Tool Versions
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.30.1
KUSTOMIZE_VERSION ?= v5.4.3
CONTROLLER_TOOLS_VERSION ?= v0.15.0
GO_VERSION = 1.22.5

export GOPATH?=$(shell go env GOPATH)
export CGO_ENABLED=0
export GOPROXY?=https://proxy.golang.org
export GO111MODULE=on
export GOFLAGS?=-mod=readonly -trimpath
export GIT_TAG ?= $(shell git tag --points-at HEAD)

IMAGE_TAG = \
		$(shell echo $$(git rev-parse HEAD && if [[ -n $$(git status --porcelain) ]]; then echo '-dirty'; fi)|tr -d ' ')

VERSION = $(shell cat VERSION)

CCM_IMAGE_NAME ?= $(KUBELB_CCM_IMG):$(IMAGE_TAG)
KUBELB_IMAGE_NAME ?= $(KUBELB_IMG):$(IMAGE_TAG)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# SED is used to allow the Makefile to be used on both Linux and macOS.
SED ?= sed
ifeq ($(shell uname), Darwin)
	SED = gsed
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
manifests: generate controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) rbac:roleName=kubelb-ccm paths="./internal/controllers/ccm/..." output:artifacts:config=config/ccm/rbac
	$(CONTROLLER_GEN) rbac:roleName=kubelb paths="./internal/controllers/kubelb/..." output:artifacts:config=config/kubelb/rbac
	$(CONTROLLER_GEN) crd webhook paths="./..." output:crd:artifacts:config=charts/kubelb-manager/crds

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
test: envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test -v ./internal/... -coverprofile cover.out

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
	if [[ -n "$(GIT_TAG)" ]]; then \
		docker tag $(KUBELB_IMAGE_NAME) $(KUBELB_IMG):$(GIT_TAG) && \
		docker tag $(KUBELB_IMAGE_NAME) $(KUBELB_IMG):latest && \
		docker tag $(CCM_IMAGE_NAME) $(KUBELB_CCM_IMG):$(GIT_TAG) && \
		docker tag $(CCM_IMAGE_NAME) $(KUBELB_CCM_IMG):latest ;\
	fi

	docker push $(KUBELB_IMG) --all-tags
	docker push $(KUBELB_CCM_IMG) --all-tags

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

.PHONY: e2e-deploy-ccm
e2e-deploy-ccm-%: manifests kustomize
	$(KUSTOMIZE) build ./hack/ci/e2e/config/ccm/$* | kubectl apply -f -

.PHONY: e2e-deploy-kubelb
e2e-deploy-kubelb: manifests kustomize
	$(KUSTOMIZE) build ./hack/ci/e2e/config/kubelb | kubectl apply -f -

.PHONY: undeploy
undeploy-%: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/deploy/$* | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: bump
bump: kustomize
	cd config/deploy/kubelb && $(KUSTOMIZE) edit set image controller=quay.io/kubermatic/kubelb-manager:$(IMAGE_TAG)
	cd config/deploy/ccm && $(KUSTOMIZE) edit set image controller=quay.io/kubermatic/kubelb-ccm:$(IMAGE_TAG)

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

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

HELM_DOCS ?= $(LOCALBIN)/helm-docs

.PHONY: helm-docs
helm-docs: $(HELM_DOCS) ## Download helm-docs locally if necessary.
$(HELM_DOCS): $(LOCALBIN)
	test -s $(LOCALBIN)/helm-docs || GOBIN=$(LOCALBIN) go install github.com/norwoodj/helm-docs/cmd/helm-docs@v1.14.2

helm-lint:
	helm lint charts/*

generate-helm-docs: helm-docs
	$(LOCALBIN)/helm-docs charts/

.PHONY: bump-chart
bump-chart:
	$(SED) -i "s/^version:.*/version: $(IMAGE_TAG)/" charts/*/Chart.yaml
	$(SED) -i "s/^appVersion:.*/appVersion: $(IMAGE_TAG)/" charts/*/Chart.yaml
	$(SED) -i "s/tag:.*/tag: $(IMAGE_TAG)/" charts/*/values.yaml

.PHONY: release-charts helm-docs generate-helm-docs
release-charts: helm-lint generate-helm-docs bump-chart
	CHART_VERSION=$(IMAGE_TAG) ./hack/release-helm-charts.sh