#!/usr/bin/env bash

# Copyright 2026 The KubeLB Authors.
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

# Deploy CCM in standalone conversion mode to single Kind cluster.
# This deploys:
# - kubelb-addons (envoy-gateway, ingress-nginx, cert-manager, metallb)
# - kubelb-ccm in standalone conversion mode
# - test apps

set -euo pipefail

export TEST_NAME="${TEST_NAME:-e2e-deploy-conversion}"
export ROOT_DIR="$(git rev-parse --show-toplevel)"
source "${ROOT_DIR}/hack/lib.sh"

KUBECONFIGS_DIR="${KUBECONFIGS_DIR:-${ROOT_DIR}/.e2e-kubeconfigs}"
E2E_MANIFESTS_DIR="${ROOT_DIR}/test/e2e/manifests"
CHARTS_DIR="${ROOT_DIR}/charts"
LOGS_DIR="${KUBECONFIGS_DIR}/logs"

# Image configuration
IMAGE_TAG="${IMAGE_TAG:-e2e}"
CCM_IMAGE="kubelb-ccm:${IMAGE_TAG}"
GO_VERSION="${GO_VERSION:-1.25.6}"
GIT_VERSION="${GIT_VERSION:-e2e}"
GIT_COMMIT="${GIT_COMMIT:-$(git rev-parse --short HEAD)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

SKIP_BUILD="${SKIP_BUILD:-false}"
SKIP_IMAGE_LOAD="${SKIP_IMAGE_LOAD:-false}"
METALLB_IP_RANGE="${METALLB_IP_RANGE:-}"

mkdir -p "${LOGS_DIR}"

#######################################
# Verify prerequisites
#######################################
verify_kubeconfig() {
  if [[ ! -f "${KUBECONFIGS_DIR}/conversion.kubeconfig" ]]; then
    echo "Error: ${KUBECONFIGS_DIR}/conversion.kubeconfig not found"
    echo "Run 'make e2e-conversion-setup-kind' first"
    exit 1
  fi
}

get_helm_command() {
  local release="$1"
  local namespace="$2"
  if helm status "$release" -n "$namespace" &> /dev/null; then
    echo "upgrade"
  else
    echo "install"
  fi
}

#######################################
# Build CCM image
#######################################
build_ccm_image() {
  if [[ "${SKIP_BUILD}" == "true" ]]; then
    echodate "Skipping image build (SKIP_BUILD=true)"
    return 0
  fi

  echodate "Building CCM image..."
  local build_start=$(nowms)
  local build_log="${LOGS_DIR}/build-ccm.log"

  if ! GOOS=linux GOARCH=amd64 make e2e-image-ccm \
    GIT_VERSION="${GIT_VERSION}" \
    GIT_COMMIT="${GIT_COMMIT}" \
    BUILD_DATE=e2e \
    &> "${build_log}"; then
    echodate "FAILED: CCM image build (see ${build_log})"
    cat "${build_log}"
    exit 1
  fi

  echodate "Built ${CCM_IMAGE}"
  printElapsed "ccm_build" ${build_start}
}

#######################################
# Load CCM image into Kind cluster
#######################################
load_ccm_image() {
  if [[ "${SKIP_IMAGE_LOAD}" == "true" ]]; then
    echodate "Skipping image load (SKIP_IMAGE_LOAD=true)"
    return 0
  fi

  echodate "Loading CCM image into Kind cluster..."
  local load_start=$(nowms)

  kind load docker-image --name=conversion "${CCM_IMAGE}"

  printElapsed "image_load" ${load_start}
}

#######################################
# Deploy kubelb-addons
#######################################
deploy_addons() {
  echodate "Deploying kubelb-addons..."
  local deploy_start=$(nowms)

  export KUBECONFIG="${KUBECONFIGS_DIR}/conversion.kubeconfig"

  # Create namespace
  kubectl create ns kubelb 2> /dev/null || true

  # Ensure helm repos are configured
  echodate "Ensuring helm repos are configured..."
  "${ROOT_DIR}/hack/ensure-helm-repos.sh"

  # Build addons dependencies
  echodate "Building kubelb-addons dependencies..."
  helm dependency build "${CHARTS_DIR}/kubelb-addons"

  # Install/upgrade addons
  local helm_cmd
  helm_cmd=$(get_helm_command "kubelb-addons" "kubelb")
  echodate "Running helm ${helm_cmd} for kubelb-addons..."

  helm ${helm_cmd} kubelb-addons "${CHARTS_DIR}/kubelb-addons" \
    --namespace kubelb \
    --values "${E2E_MANIFESTS_DIR}/conversion/addons-values.yaml" \
    --wait \
    --timeout 10m

  printElapsed "addons_deploy" ${deploy_start}
}

#######################################
# Configure MetalLB IP pool
#######################################
configure_metallb_pool() {
  echodate "Configuring MetalLB IP pool..."
  export KUBECONFIG="${KUBECONFIGS_DIR}/conversion.kubeconfig"

  local metallb_ip_range="${METALLB_IP_RANGE}"

  if [[ -z "${metallb_ip_range}" ]]; then
    # Auto-detect from Kind's Docker network
    local metallb_gw
    metallb_gw=$(docker network inspect -f json kind | jq --raw-output '.[].IPAM.Config[].Gateway | select(. != null) | select(contains(":") | not)' | sed -e 's/\(.*\..*\).*\..*\..*/\1/')
    metallb_ip_range="${metallb_gw}.255.200-${metallb_gw}.255.250"
  fi

  echodate "MetalLB IP pool: ${metallb_ip_range}"
  sed "s|\${METALLB_IP_RANGE}|${metallb_ip_range}|g" \
    "${E2E_MANIFESTS_DIR}/metallb/ipaddresspool.yaml.tpl" | kubectl apply -f -
}

#######################################
# Deploy CCM in standalone conversion mode
#######################################
deploy_ccm_standalone() {
  echodate "Deploying CCM in standalone conversion mode..."
  local deploy_start=$(nowms)

  export KUBECONFIG="${KUBECONFIGS_DIR}/conversion.kubeconfig"

  # Install/upgrade CCM
  local helm_cmd
  helm_cmd=$(get_helm_command "kubelb-ccm" "kubelb")
  echodate "Running helm ${helm_cmd} for kubelb-ccm..."

  helm ${helm_cmd} kubelb-ccm "${CHARTS_DIR}/kubelb-ccm" \
    --namespace kubelb \
    --values "${E2E_MANIFESTS_DIR}/kubelb-ccm/values-conversion.yaml" \
    --set image.repository="${CCM_IMAGE%:*}" \
    --set image.tag="${CCM_IMAGE#*:}" \
    --set image.pullPolicy=Never \
    --set imagePullSecrets=

  # Wait for CCM
  echodate "Waiting for CCM deployment..."
  kubectl -n kubelb wait --for=condition=available deployment/kubelb-ccm --timeout=5m

  # Restart if upgrading to pick up new image
  if [[ "${helm_cmd}" == "upgrade" ]]; then
    echodate "Restarting kubelb-ccm deployment..."
    kubectl -n kubelb rollout restart deployment/kubelb-ccm
    kubectl -n kubelb wait --for=condition=available deployment/kubelb-ccm --timeout=5m
  fi

  printElapsed "ccm_deploy" ${deploy_start}
}

#######################################
# Wait for addons to be ready
#######################################
wait_for_addons() {
  echodate "Waiting for addons to be ready..."
  local wait_start=$(nowms)

  export KUBECONFIG="${KUBECONFIGS_DIR}/conversion.kubeconfig"

  # Wait for Gateway API CRDs
  kubectl wait --for=condition=established crd gatewayclasses.gateway.networking.k8s.io --timeout=5m &
  local gateway_crd_pid=$!

  # Wait for cert-manager CRDs
  kubectl wait --for=condition=established crd certificates.cert-manager.io --timeout=5m &
  local cert_crd_pid=$!

  # Wait for ingress-nginx
  kubectl -n kubelb wait --for=condition=available deployment/kubelb-addons-ingress-nginx-controller --timeout=5m &
  local ingress_pid=$!

  # Wait for envoy-gateway (no prefix in chart)
  kubectl -n kubelb wait --for=condition=available deployment/envoy-gateway --timeout=5m &
  local envoy_pid=$!

  # Wait for MetalLB
  kubectl -n kubelb wait --for=condition=available deployment/kubelb-addons-metallb-controller --timeout=5m &
  local metallb_pid=$!

  # Wait for cert-manager
  kubectl -n kubelb wait --for=condition=available deployment/kubelb-addons-cert-manager --timeout=5m &
  local certmanager_pid=$!

  # Collect results
  local failed=()
  wait ${gateway_crd_pid} || failed+=("gateway-crds")
  wait ${cert_crd_pid} || failed+=("cert-manager-crds")
  wait ${ingress_pid} || failed+=("ingress-nginx")
  wait ${envoy_pid} || failed+=("envoy-gateway")
  wait ${metallb_pid} || failed+=("metallb")
  wait ${certmanager_pid} || failed+=("cert-manager")

  if [[ ${#failed[@]} -gt 0 ]]; then
    echodate "ERROR: Failed to verify: ${failed[*]}"
    exit 1
  fi

  printElapsed "addons_ready" ${wait_start}
}

#######################################
# Deploy test apps
#######################################
deploy_test_apps() {
  echodate "Deploying test apps..."

  export KUBECONFIG="${KUBECONFIGS_DIR}/conversion.kubeconfig"
  kubectl apply -f "${E2E_MANIFESTS_DIR}/test-apps/echo-server.yaml"

  # Wait for echo-server
  kubectl wait --for=condition=available deployment/echo-shared -n default --timeout=2m

  echodate "Test apps deployed"
}

#######################################
# Main
#######################################
SCRIPT_START=$(nowms)

echodate "============================================"
echodate "  ${TEST_NAME}"
echodate "============================================"
echodate "CCM image: ${CCM_IMAGE}"
echodate ""

verify_kubeconfig
build_ccm_image
load_ccm_image
deploy_addons
wait_for_addons
configure_metallb_pool

# Apply GatewayClass and ClusterIssuer
echodate "Applying GatewayClass and ClusterIssuer..."
export KUBECONFIG="${KUBECONFIGS_DIR}/conversion.kubeconfig"
kubectl apply -f "${E2E_MANIFESTS_DIR}/kubelb-manager/manifests.yaml"

deploy_ccm_standalone
deploy_test_apps

echodate ""
echodate "============================================"
echodate "  Deployment Complete"
echodate "============================================"
echodate ""
echodate "Cluster ready:"
echodate "  export KUBECONFIG=${KUBECONFIGS_DIR}/conversion.kubeconfig"
echodate ""
echodate "CCM running in standalone conversion mode"
echodate "  Gateway: kubelb (namespace: default)"
echodate "  GatewayClass: eg"
echodate "  IngressClass: nginx"
echodate "  Domain: test.local -> gateway.local"
echodate ""
printElapsed "total_deploy" ${SCRIPT_START}
