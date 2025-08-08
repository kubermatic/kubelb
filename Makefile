SHELL = /bin/bash -eu -o pipefail

# Image URL to use all building/pushing image targets
KUBELB_IMG ?= quay.io/kubermatic/kubelb-manager
KUBELB_CCM_IMG ?= quay.io/kubermatic/kubelb-ccm

## Tool Versions
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.32.0
KUSTOMIZE_VERSION ?= v5.7.1
CONTROLLER_TOOLS_VERSION ?= v0.18.0
GO_VERSION = 1.24.6
HELM_DOCS_VERSION ?= v1.14.2
CRD_REF_DOCS_VERSION ?= v0.2.0

CRD_CODE_GEN_PATH = "./api/ce/..."
RECONCILE_HELPER_PATH = "internal/resources/reconciling/zz_generated_reconcile.go"

GATEWAY_RELEASE_CHANNEL ?= standard
GATEWAY_API_VERSION ?= v1.3.0

export GOPATH?=$(shell go env GOPATH)
export CGO_ENABLED=0
export GOPROXY?=https://proxy.golang.org
export GO111MODULE=on
export GOFLAGS?=-mod=readonly -trimpath
export GIT_TAG ?= $(shell git tag --points-at HEAD)

GIT_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X 'k8c.io/kubelb/internal/version.GitVersion=$(GIT_VERSION)' \
	-X 'k8c.io/kubelb/internal/version.GitCommit=$(GIT_COMMIT)' \
	-X 'k8c.io/kubelb/internal/version.BuildDate=$(BUILD_DATE)'

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
	$(CONTROLLER_GEN) --version
	$(CONTROLLER_GEN) crd webhook paths=$(CRD_CODE_GEN_PATH) output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) crd webhook paths=$(CRD_CODE_GEN_PATH) output:crd:artifacts:config=charts/kubelb-manager/crds
	$(CONTROLLER_GEN) rbac:roleName=kubelb-ccm paths="./internal/controllers/ccm/..." output:artifacts:config=config/ccm/rbac
	$(CONTROLLER_GEN) rbac:roleName=kubelb paths="./internal/controllers/kubelb/..." output:artifacts:config=config/kubelb/rbac
	cp charts/kubelb-manager/crds/kubelb.k8c.io_syncsecrets.yaml charts/kubelb-ccm/crds/kubelb.k8c.io_syncsecrets.yaml

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate/boilerplate.go.txt" paths="./..."

update-codegen: generate controller-gen manifests reconciler-gen generate-helm-docs fmt vet go-mod-tidy

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

go-mod-tidy:
	go mod tidy

verify-boilerplate:  ## Run verify-boilerplate code.
	./hack/verify-boilerplate.sh

verify-imports:  ## Run verify-imports code.
	./hack/verify-import-order.sh

clean:  ## Clean binaries
	rm -rf bin/*
	@cd cli && $(MAKE) clean

.PHONY: test
test: envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test -v ./internal/... -coverprofile cover.out

##@ Build

.PHONY: build
build: build-ccm build-kubelb

build-%: fmt vet ## Build manager binary.
	CGO_ENABLED=0 go build -v \
		-ldflags "$(LDFLAGS)" \
		-o bin/$* cmd/$*/main.go

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
	docker build --build-arg GO_VERSION=$(GO_VERSION) \
		--build-arg GIT_VERSION="$(GIT_VERSION)" \
		--build-arg GIT_COMMIT="$(GIT_COMMIT)" \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		-t ${KUBELB_IMAGE_NAME} -f kubelb.dockerfile .
	docker build --build-arg GO_VERSION=$(GO_VERSION) \
		--build-arg GIT_VERSION="$(GIT_VERSION)" \
		--build-arg GIT_COMMIT="$(GIT_COMMIT)" \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		-t ${CCM_IMAGE_NAME} -f ccm.dockerfile .

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

.PHONY: shfmt
shfmt:
	shfmt -w -sr -i 2 hack

HELM_DOCS ?= $(LOCALBIN)/helm-docs
CRD_REF_DOCS ?= $(LOCALBIN)/crd-ref-docs

.PHONY: helm-docs
helm-docs: $(HELM_DOCS) ## Download helm-docs locally if necessary.
$(HELM_DOCS): $(LOCALBIN)
	test -s $(LOCALBIN)/helm-docs || GOBIN=$(LOCALBIN) go install github.com/norwoodj/helm-docs/cmd/helm-docs@$(HELM_DOCS_VERSION)

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
release-charts: bump-chart helm-lint generate-helm-docs
	CHART_VERSION=$(IMAGE_TAG) ./hack/release-helm-charts.sh

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs locally if necessary.
$(CRD_REF_DOCS): $(LOCALBIN)
	test -s $(LOCALBIN)/crd-ref-docs || GOBIN=$(LOCALBIN) go install github.com/elastic/crd-ref-docs@$(CRD_REF_DOCS_VERSION)

generate-crd-docs: crd-ref-docs ## Generate API reference documentation.
	$(LOCALBIN)/crd-ref-docs --renderer=markdown \
		--source-path ./api/kubelb.k8c.io \
		--config=./hack/crd-ref-docs.yaml \
		--output-path ./docs/api-reference.md

.PHONY: update-gateway-api-crds
update-gateway-api-crds:
	GATEWAY_RELEASE_CHANNEL=standard make download-gateway-api-crds
	GATEWAY_RELEASE_CHANNEL=experimental make download-gateway-api-crds

.PHONY: download-gateway-api-crds
download-gateway-api-crds: ## Download Gateway API CRDs
	@echo "Downloading Gateway API CRDs..."
	@curl -s -k "https://api.github.com/repos/kubernetes-sigs/gateway-api/contents/config/crd/$(GATEWAY_RELEASE_CHANNEL)?ref=$(GATEWAY_API_VERSION)" | \
		jq -r '.[] | select(.name != "kustomization.yaml") | .name' | \
		while read filename; do \
			echo "Downloading $$filename..."; \
			curl -k -sLo "internal/resources/crds/gatewayapi/$(GATEWAY_RELEASE_CHANNEL)/$$filename" \
				"https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/$(GATEWAY_API_VERSION)/config/crd/$(GATEWAY_RELEASE_CHANNEL)/$$filename"; \
		done
	@echo "Gateway API CRDs downloaded successfully to internal/resources/crds/gatewayapi/$(GATEWAY_RELEASE_CHANNEL)/"

.PHONY: reconciler-gen
reconciler-gen: ## Generate reconciler helpers
	go run k8c.io/reconciler/cmd/reconciler-gen --config hack/reconciling.yaml > $(RECONCILE_HELPER_PATH)
	$(eval currentYear := $(shell date +%Y))
	$(SED) -i "s/Copyright YEAR/Copyright $(currentYear)/g" $(RECONCILE_HELPER_PATH)