
# Image URL to use all building/pushing image targets
IMG ?= kubelb-manager:latest
# Produce CRDs that work back to Kubernetes 1.16+
CRD_OPTIONS ?= "crd:crdVersions=v1"

# manager arguments
ARGS ?= --metrics-addr=:8081

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

# Build operator binary
manager: generate fmt vet
	go build -o bin/manager cmd/manager/main.go

# Build agent binary
agent: generate fmt vet
	go build -o bin/agent cmd/agent/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run-agent: generate fmt vet manifests
	go run ./cmd/manager/main.go $(ARGS)

# Run against the configured Kubernetes cluster in ~/.kube/config
run-manager: generate fmt vet manifests
	go run ./cmd/agent/main.go $(ARGS)

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy-agent: manifests
	cd config/agent && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy-manager: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./..." output:crd:artifacts:config=config/crd/bases
	#$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

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
