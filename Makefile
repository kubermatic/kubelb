SHELL = /bin/bash -eu -o pipefail

# Image URL to use all building/pushing image targets
KUBELB_IMG ?= quay.io/kubermatic/kubelb-manager
KUBELB_CCM_IMG ?= quay.io/kubermatic/kubelb-ccm

## Tool Versions
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $$3}')
KUSTOMIZE_VERSION ?= v5.8.0
CONTROLLER_TOOLS_VERSION ?= v0.20.0
GO_VERSION = 1.25.7
HELM_DOCS_VERSION ?= v1.14.2
CRD_REF_DOCS_VERSION ?= v0.2.0
CHAINSAW_VERSION ?= v0.2.13

CRD_CODE_GEN_PATH = "./api/ce/..."
RECONCILE_HELPER_PATH = "internal/resources/reconciling/zz_generated_reconcile.go"

GATEWAY_RELEASE_CHANNEL ?= standard
GATEWAY_API_VERSION ?= v1.4.1
KUBELB_ADDONS_CHART_VERSION ?= v0.3.1

export GOPATH?=$(shell go env GOPATH)
export CGO_ENABLED=0
export GOPROXY?=https://proxy.golang.org
export GO111MODULE=on
export GOFLAGS?=-mod=readonly -trimpath
export GIT_TAG ?= $(shell git tag --points-at HEAD 2>/dev/null | head -n1)

GIT_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X 'k8c.io/kubelb/internal/versioninfo.GitVersion=$(GIT_VERSION)' \
	-X 'k8c.io/kubelb/internal/versioninfo.GitCommit=$(GIT_COMMIT)' \
	-X 'k8c.io/kubelb/internal/versioninfo.BuildDate=$(BUILD_DATE)'

IMAGE_TAG = \
		$(shell echo $$(git rev-parse HEAD 2>/dev/null && if [[ -n $$(git status --porcelain 2>/dev/null) ]]; then echo '-dirty'; fi)|tr -d ' ')

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

update-codegen: generate controller-gen manifests reconciler-gen generate-helm-docs generate-metricsdocs fmt vet go-mod-tidy

helm-dependency-update:
	./hack/ensure-helm-repos.sh && \
	helm dependency update charts/kubelb-manager && \
	helm dependency build charts/kubelb-manager && \
	helm dependency update charts/kubelb-ccm && \
	helm dependency build charts/kubelb-ccm && \
	helm dependency update charts/kubelb-addons && \
	helm dependency build charts/kubelb-addons

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

lint: ## Run golangci-lint against code.
	golangci-lint run -v

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

TAGS ?=

build-%: fmt vet ## Build manager binary.
	CGO_ENABLED=0 go build -v \
		$(if $(TAGS),-tags $(TAGS),) \
		-ldflags "$(LDFLAGS)" \
		-o bin/$* cmd/$*/main.go

.PHONY: e2e-image-kubelb
e2e-image-kubelb: ## Build kubelb e2e image (linux/amd64)
	CGO_ENABLED=0 go build -v -tags e2e \
		-ldflags "$(LDFLAGS)" \
		-o bin/kubelb cmd/kubelb/main.go
	docker build -q -t kubelb:e2e -f kubelb.goreleaser.dockerfile bin/

.PHONY: e2e-image-ccm
e2e-image-ccm: ## Build ccm e2e image (linux/amd64)
	CGO_ENABLED=0 go build -v -tags e2e \
		-ldflags "$(LDFLAGS)" \
		-o bin/ccm cmd/ccm/main.go
	docker build -q -t kubelb-ccm:e2e -f ccm.goreleaser.dockerfile bin/

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
CHAINSAW ?= $(LOCALBIN)/chainsaw

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

.PHONY: chainsaw
chainsaw: $(CHAINSAW) ## Download chainsaw locally if necessary.
$(CHAINSAW): $(LOCALBIN)
	test -s $(LOCALBIN)/chainsaw || GOPROXY=https://proxy.golang.org,direct GOBIN=$(LOCALBIN) go install github.com/kyverno/chainsaw@$(CHAINSAW_VERSION)

##@ E2E Testing

E2E_DIR ?= ./test/e2e
KUBECONFIGS_DIR ?= $(shell pwd)/.e2e-kubeconfigs
CHAINSAW_CONFIG ?= $(E2E_DIR)/config.yaml
CHAINSAW_VALUES ?= $(E2E_DIR)/values.yaml
# All 4 clusters: kubelb (manager), tenant1 (multi-node), tenant2 (single-node), standalone (conversion)
CHAINSAW_CLUSTERS ?= --cluster kubelb=$(KUBECONFIGS_DIR)/kubelb.kubeconfig \
	--cluster tenant1=$(KUBECONFIGS_DIR)/tenant1.kubeconfig \
	--cluster tenant2=$(KUBECONFIGS_DIR)/tenant2.kubeconfig \
	--cluster standalone=$(KUBECONFIGS_DIR)/standalone.kubeconfig
CHAINSAW_FLAGS ?= --config $(CHAINSAW_CONFIG) --values $(CHAINSAW_VALUES) $(CHAINSAW_CLUSTERS)

.PHONY: e2e-setup-kind
e2e-setup-kind: ## Setup Kind clusters for e2e tests (kubelb, tenant1, tenant2, standalone)
	./hack/e2e/setup-kind.sh

.PHONY: e2e-cleanup-kind
e2e-cleanup-kind: ## Cleanup Kind clusters
	./hack/e2e/cleanup-kind.sh

.PHONY: e2e-deploy
e2e-deploy: ## Deploy KubeLB to all Kind clusters
	KUBECONFIGS_DIR=$(KUBECONFIGS_DIR) ./hack/e2e/deploy.sh

.PHONY: e2e-reload
e2e-reload: ## Quick reload of kubelb/ccm after code changes (faster than e2e-deploy)
	KUBECONFIGS_DIR=$(KUBECONFIGS_DIR) ./hack/e2e/reload.sh

.PHONY: e2e-kind
e2e-kind: e2e-setup-kind e2e-deploy e2e ## Full e2e with Kind setup

.PHONY: e2e-local
e2e-local: e2e-setup-local e2e

.PHONY: e2e-setup-local
e2e-setup-local: e2e-cleanup-kind e2e-setup-kind e2e-deploy ## Reset Kind clusters and redeploy

.PHONY: e2e
e2e: chainsaw ## Run all e2e tests (requires KUBECONFIGS_DIR or existing clusters)
	KUBECONFIG=$(KUBECONFIGS_DIR)/tenant1.kubeconfig $(CHAINSAW) test $(E2E_DIR)/tests $(CHAINSAW_FLAGS) --quiet

.PHONY: e2e-select
e2e-select: chainsaw ## Run e2e tests matching label selector (e.g., make e2e-select select=layer=layer4)
	KUBECONFIG=$(KUBECONFIGS_DIR)/tenant1.kubeconfig $(CHAINSAW) test $(E2E_DIR)/tests $(CHAINSAW_FLAGS) --selector $(select)

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
	$(SED) -i "s/^version:.*/version: $(IMAGE_TAG)/" charts/kubelb-ccm/Chart.yaml charts/kubelb-manager/Chart.yaml
	$(SED) -i "s/^appVersion:.*/appVersion: $(IMAGE_TAG)/" charts/kubelb-ccm/Chart.yaml charts/kubelb-manager/Chart.yaml
	$(SED) -i "s/^  tag:.*/  tag: $(IMAGE_TAG)/" charts/kubelb-ccm/values.yaml charts/kubelb-manager/values.yaml

.PHONY: release-charts helm-docs generate-helm-docs
release-charts: bump-chart helm-lint helm-dependency-update generate-helm-docs
	CHART_VERSION=$(IMAGE_TAG) ./hack/release-helm-charts.sh

bump-addons-chart:
	$(SED) -i "s/^version:.*/version: $(KUBELB_ADDONS_CHART_VERSION)/" charts/kubelb-addons/Chart.yaml
	$(SED) -i "s/^appVersion:.*/appVersion: $(KUBELB_ADDONS_CHART_VERSION)/" charts/kubelb-addons/Chart.yaml

release-addons-chart: bump-addons-chart helm-lint helm-dependency-update generate-helm-docs
	CHART_VERSION=$(KUBELB_ADDONS_CHART_VERSION) RELEASE_ADDONS_ONLY=true ./hack/release-helm-charts.sh

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs locally if necessary.
$(CRD_REF_DOCS): $(LOCALBIN)
	test -s $(LOCALBIN)/crd-ref-docs || GOBIN=$(LOCALBIN) go install github.com/elastic/crd-ref-docs@$(CRD_REF_DOCS_VERSION)

generate-crd-docs: crd-ref-docs ## Generate API reference documentation.
	$(LOCALBIN)/crd-ref-docs --renderer=markdown \
		--source-path ./api/ce/kubelb.k8c.io \
		--config=./hack/crd-ref-docs.yaml \
		--output-path ./docs/api-reference.md

.PHONY: generate-metricsdocs
generate-metricsdocs: ## Generate metrics reference documentation.
	mkdir -p $(shell pwd)/docs
	go run ./internal/metricsutil/metricsdocs > docs/metrics.md

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