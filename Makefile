# Copyright 2020 The KubeLB Authors.
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

SHELL = /bin/bash -eu -o pipefail

# Produce CRDs that work back to Kubernetes 1.16+
CRD_OPTIONS ?= "crd:crdVersions=v1"

# run arguments
ARGS ?= -v=1

REGISTRY ?= quay.io
REGISTRY_NAMESPACE ?= kubermatic-labs

IMAGE_TAG = "latest"

MANAGER_IMAGE_NAME ?= $(REGISTRY)/$(REGISTRY_NAMESPACE)/kubelb
AGENT_IMAGE_NAME ?= $(REGISTRY)/$(REGISTRY_NAMESPACE)/kubelb-agent

LDFLAGS ?= -ldflags '-s -w'

export GIT_TAG ?= $(shell git tag --points-at HEAD)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager agent

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager cmd/manager/main.go

# Build agent binary
agent: generate fmt vet
	go build -o bin/agent cmd/agent/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run-%: generate fmt vet manifests
	go run ./cmd/$*/main.go $(ARGS)

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy-%: manifests
	kustomize build config/$* | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) webhook paths="./pkg/..." output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=agent-role paths="./pkg/controllers/agent/..." output:artifacts:config=config/agent/rbac
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role paths="./pkg/controllers/manager/..." output:artifacts:config=config/manager/rbac

# Build manager image
docker-build-manager:
	docker build -f manager.dockerfile -t $(MANAGER_IMAGE_NAME):$(IMAGE_TAG) .

# Build agent image
docker-build-agent:
	docker build -f agent.dockerfile -t $(AGENT_IMAGE_NAME):$(IMAGE_TAG) .

# publish docker images
docker-image-publish: docker-build-manager docker-build-agent
	docker push $(MANAGER_IMAGE_NAME):$(IMAGE_TAG)
	docker push $(AGENT_IMAGE_NAME):$(IMAGE_TAG)
	if [[ -n "$(GIT_TAG)" ]]; then \
  		docker tag $(MANAGER_IMAGE_NAME):$(IMAGE_TAG) $(MANAGER_IMAGE_NAME):$(GIT_TAG) ;\
		docker tag $(AGENT_IMAGE_NAME):$(IMAGE_TAG) $(AGENT_IMAGE_NAME):$(GIT_TAG) ;\
		docker push $(AGENT_IMAGE_NAME):$(GIT_TAG) ;\
		docker push $(MANAGER_IMAGE_NAME):$(GIT_TAG) ;\
  	fi

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

lint:
	golangci-lint run -v --timeout=5m

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate/boilerplate.go.txt" paths="./..."

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
