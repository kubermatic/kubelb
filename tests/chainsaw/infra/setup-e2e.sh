#!/usr/bin/env bash

# Copyright 2025 The KubeLB Authors.
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

set -euo pipefail

# Get the root directory of the repository
export ROOT_DIR="$(git rev-parse --show-toplevel)"
export SCRIPT_DIR="$(dirname "$(realpath "$0")")"
export CHAINSAW_VERSION="${CHAINSAW_VERSION:-v0.2.13}"

# Source the common library functions
source "${ROOT_DIR}/hack/lib.sh"

# Configuration variables
export E2E_IMAGE_TAG="${E2E_IMAGE_TAG:-v0.0.0-e2e}"
export KUBELB_IMAGE_NAME="quay.io/kubermatic/kubelb-manager:${E2E_IMAGE_TAG}"
export CCM_IMAGE_NAME="quay.io/kubermatic/kubelb-ccm:${E2E_IMAGE_TAG}"
export TMPDIR="${TMPDIR:-$(mktemp -d)}"

# Cleanup function
function cleanup() {
  echodate "Executing cleanup"
  if [[ "${CLEANUP_ON_EXIT:-true}" == "true" ]]; then
    kind delete clusters --all || :
    if [[ ! -z "${TMPDIR+x}" ]]; then
      rm -rf "$TMPDIR"
    fi
  else
    echodate "Skipping cleanup (CLEANUP_ON_EXIT=false)"
    echodate "Kubeconfigs are available at:"
    echodate "  - ${TMPDIR}/kubelb.kubeconfig"
    echodate "  - ${TMPDIR}/vyse.kubeconfig"
    echodate "  - ${TMPDIR}/omen.kubeconfig"
  fi
}

# Function to wait for deployment to be ready
function wait_for_deployment() {
  local namespace=$1
  local deployment=$2
  local timeout=${3:-300}

  echodate "Waiting for deployment ${deployment} in namespace ${namespace} to be ready..."
  kubectl wait --namespace "${namespace}" \
    --for=condition=available \
    --timeout="${timeout}s" \
    deployment/"${deployment}"
}

# Set cleanup trap
trap cleanup EXIT SIGINT SIGTERM

echodate "=== Starting KubeLB E2E Infrastructure Setup ==="
echodate "Image tag: ${E2E_IMAGE_TAG}"
echodate "Temp directory: ${TMPDIR}"

# Get local IP for Kind API server access
if [[ -z "${local_ip+x}" ]]; then
  local_ip=$(ip -j route show 2>/dev/null | jq -r '.[] | select(.dst == "default") | .prefsrc' | head -n1 || \
             ifconfig | grep "inet " | grep -v 127.0.0.1 | awk '{print $2}' | head -n1)
fi
echodate "Using local IP: ${local_ip}"

# Check and install Chainsaw if needed
if ! command -v chainsaw &> /dev/null; then
  echodate "Chainsaw not found, installing..."
  # Detect OS and architecture
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"
  case "${ARCH}" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
  esac

  # Download and install Chainsaw
  CHAINSAW_URL="https://github.com/kyverno/chainsaw/releases/download/${CHAINSAW_VERSION}/chainsaw_${OS}_${ARCH}.tar.gz"
  echodate "Downloading Chainsaw ${CHAINSAW_VERSION} for ${OS}_${ARCH}..."

  curl -sL "${CHAINSAW_URL}" | tar -xz -C /tmp

  # Move to appropriate location based on permissions
  if [[ -w /usr/local/bin ]]; then
    sudo mv /tmp/chainsaw /usr/local/bin/chainsaw
  elif [[ -d "${HOME}/bin" ]]; then
    mv /tmp/chainsaw "${HOME}/bin/chainsaw"
  else
    mkdir -p "${HOME}/.local/bin"
    mv /tmp/chainsaw "${HOME}/.local/bin/chainsaw"
    export PATH="${HOME}/.local/bin:${PATH}"
  fi

  echodate "Chainsaw installed successfully"
else
  echodate "Chainsaw is already installed: $(chainsaw version 2>/dev/null || chainsaw --version)"
fi

# Phase 1: Build Docker images
echodate "=== Phase 1: Building Docker images ==="
cd "${ROOT_DIR}"

echodate "Building KubeLB manager image: ${KUBELB_IMAGE_NAME}"
echodate "Building CCM image: ${CCM_IMAGE_NAME}"

KUBELB_IMAGE_NAME="${KUBELB_IMAGE_NAME}" \
CCM_IMAGE_NAME="${CCM_IMAGE_NAME}" \
make docker-image

# Phase 2: Create Kind clusters
echodate "=== Phase 2: Creating Kind clusters ==="

echodate "Creating kubelb management cluster..."
KUBECONFIG="${TMPDIR}/kubelb.kubeconfig" kind create cluster --retain --name kubelb --config <(
  cat << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "${local_ip}"
EOF
)

echodate "Creating vyse cluster..."
KUBECONFIG="${TMPDIR}/vyse.kubeconfig" kind create cluster --retain --name vyse --config <(
  cat << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "${local_ip}"
EOF
)

echodate "Creating omen cluster (multi-node)..."
KUBECONFIG="${TMPDIR}/omen.kubeconfig" kind create cluster --retain --name omen --config <(
  cat << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "${local_ip}"
nodes:
  - role: control-plane
  - role: worker
  - role: worker
  - role: worker
EOF
)

# Phase 3: Load Docker images into Kind clusters
echodate "=== Phase 3: Loading Docker images into Kind clusters ==="

for cluster in kubelb vyse omen; do
  echodate "Loading images into ${cluster} cluster..."
  kind load docker-image --name="${cluster}" "${KUBELB_IMAGE_NAME}"
  kind load docker-image --name="${cluster}" "${CCM_IMAGE_NAME}"
done

# Phase 4: Deploy KubeLB Manager
echodate "=== Phase 4: Deploying KubeLB Manager ==="
export KUBECONFIG="${TMPDIR}/kubelb.kubeconfig"

# Install CRDs
echodate "Installing CRDs..."
kubectl apply -f "${ROOT_DIR}/charts/kubelb-manager/crds/"

# Create namespace for KubeLB manager
echodate "Creating kubelb namespace..."
kubectl create namespace kubelb || true

# Deploy KubeLB manager using Helm with custom values
echodate "Deploying KubeLB manager via Helm..."
helm upgrade --install kubelb-manager \
  "${ROOT_DIR}/charts/kubelb-manager" \
  --namespace kubelb \
  --values "${SCRIPT_DIR}/kubelb-manager-values.yaml" \
  --set image.tag="${E2E_IMAGE_TAG}" \
  --wait \
  --timeout 5m

# Wait for manager to be ready
wait_for_deployment kubelb kubelb-manager

# Get Kind network gateway IP
echodate "Configuring MetalLB IP pool..."
gw=$(docker network inspect -f json kind | jq --raw-output '.[].IPAM.Config[1].Gateway' | sed -e 's/\(.*\..*\).*\..*\..*/\1/')
echodate "Using gateway IP range: ${gw}.255.200-${gw}.255.250"

# Deploy config and tenants with gateway IP substitution
echodate "Deploying config and tenants..."
# Create a temporary file with substituted gateway IP
temp_management_yaml="${TMPDIR}/management.yaml"
sed "s/\${GW}/${gw}/g" "${ROOT_DIR}/tests/chainsaw/infra/manifests/management.yaml" > "${temp_management_yaml}"
kubectl apply -f "${temp_management_yaml}"
# Clean up the temporary file immediately
rm -f "${temp_management_yaml}"

# Phase 5: Deploy CCM for each tenant
echodate "=== Phase 5: Deploying CCM for tenants ==="

tenants=("vyse" "omen")
tenant_clusters=("vyse" "omen")

for i in "${!tenants[@]}"; do
  tenant_name="${tenants[$i]}"
  tenant_cluster="${tenant_clusters[$i]}"
  # Tenant controller creates namespace with pattern tenant-{name}
  tenant_namespace="tenant-${tenant_name}"

  echodate "Setting up CCM for ${tenant_name} in namespace ${tenant_namespace}..."

  # Wait for tenant namespace to be created by the Tenant controller
  echodate "Waiting for tenant namespace ${tenant_namespace} to be created..."
  timeout=120
  while ! kubectl get namespace "${tenant_namespace}" &>/dev/null && [[ $timeout -gt 0 ]]; do
    sleep 2
    timeout=$((timeout - 2))
  done

  if ! kubectl get namespace "${tenant_namespace}" &>/dev/null; then
    echodate "ERROR: Namespace ${tenant_namespace} was not created after 2 minutes"
    return 1
  fi

  # Wait for kubeconfig secret to be created by the Tenant controller
  echodate "Waiting for kubeconfig secret kubelb-ccm-kubeconfig in namespace ${tenant_namespace}..."
  timeout=120
  while ! kubectl get secret kubelb-ccm-kubeconfig -n "${tenant_namespace}" &>/dev/null && [[ $timeout -gt 0 ]]; do
    sleep 2
    timeout=$((timeout - 2))
  done

  if ! kubectl get secret kubelb-ccm-kubeconfig -n "${tenant_namespace}" &>/dev/null; then
    echodate "ERROR: Secret kubelb-ccm-kubeconfig was not created in namespace ${tenant_namespace} after 2 minutes"
    return 1
  fi

  # Extract the management cluster kubeconfig from the Tenant controller-generated secret
  echodate "Extracting management cluster kubeconfig for ${tenant_name}..."
  kubectl get secret kubelb-ccm-kubeconfig -n "${tenant_namespace}" -o jsonpath='{.data.kubelb}' | base64 -d > "${TMPDIR}/kubelb-${tenant_name}.kubeconfig"

  # Create kubeconfig secret that CCM will use
  echodate "Creating kubeconfig secret for CCM ${tenant_name}..."
  kubectl create secret generic kubelb-cluster \
    --from-file=kubelb="${TMPDIR}/kubelb-${tenant_name}.kubeconfig" \
    --from-file=tenant="${TMPDIR}/${tenant_cluster}.kubeconfig" \
    --namespace="${tenant_namespace}" \
    --dry-run=client -o yaml | kubectl apply -f -

  # Deploy CCM using Helm - it will use kubelb-ccm-kubeconfig secret for management cluster access
  echodate "Deploying CCM for ${tenant_name} via Helm..."
  helm upgrade --install "kubelb-ccm-${tenant_name}" \
    "${ROOT_DIR}/charts/kubelb-ccm" \
    --namespace "${tenant_namespace}" \
    --set kubelb.tenantName="${tenant_name}" \
    --set kubelb.clusterSecretName=kubelb-cluster \
    --set image.tag="${E2E_IMAGE_TAG}" \
    --set image.pullPolicy=Never \
    --wait \
    --timeout 5m

  # Wait for CCM to be ready
  wait_for_deployment "${tenant_namespace}" "kubelb-ccm-${tenant_name}"
done

# Verify setup
echodate "=== Verifying infrastructure setup ==="

# Check management cluster
export KUBECONFIG="${TMPDIR}/kubelb.kubeconfig"
echodate "KubeLB Manager status:"
kubectl get pods -n kubelb

echodate "CCM deployments status:"
for tenant_name in "${tenants[@]}"; do
  tenant_namespace="tenant-${tenant_name}"
  echodate "CCM for ${tenant_name} in namespace ${tenant_namespace}:"
  kubectl get pods -n "${tenant_namespace}"
done

# Check tenant clusters connectivity
for cluster in vyse omen; do
  export KUBECONFIG="${TMPDIR}/${cluster}.kubeconfig"
  echodate "Checking ${cluster} cluster nodes:"
  kubectl get nodes
done

echodate "=== Infrastructure setup complete! ==="
echodate ""
echodate "Ready for Chainsaw testing!"
echodate ""
echodate "Kubeconfig files available at:"
echodate "  - Management: ${TMPDIR}/kubelb.kubeconfig"
echodate "  - Vyse: ${TMPDIR}/vyse.kubeconfig"
echodate "  - Omen: ${TMPDIR}/omen.kubeconfig"
echodate ""
echodate "To run tests:"
echodate "  export KUBECONFIG_DIR=${TMPDIR}"
echodate "  cd tests/chainsaw/e2e"
echodate "  chainsaw test"
echodate ""
echodate "To prevent cleanup on exit, set CLEANUP_ON_EXIT=false"