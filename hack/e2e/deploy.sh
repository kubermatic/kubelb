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

set -euo pipefail

export TEST_NAME="${TEST_NAME:-e2e-deploy}"
export ROOT_DIR="$(git rev-parse --show-toplevel)"
source "${ROOT_DIR}/hack/lib.sh"

KUBECONFIGS_DIR="${KUBECONFIGS_DIR:-${ROOT_DIR}/.e2e-kubeconfigs}"
E2E_MANIFESTS_DIR="${ROOT_DIR}/test/e2e/manifests"
CHARTS_DIR="${ROOT_DIR}/charts"
LOGS_DIR="${KUBECONFIGS_DIR}/logs"

# Image configuration
IMAGE_REGISTRY="${IMAGE_REGISTRY:-}" # Empty for local, set for cloud (e.g., gcr.io/myproject)
IMAGE_TAG="${IMAGE_TAG:-e2e}"
KUBELB_IMAGE="${IMAGE_REGISTRY:+${IMAGE_REGISTRY}/}kubelb:${IMAGE_TAG}"
CCM_IMAGE="${IMAGE_REGISTRY:+${IMAGE_REGISTRY}/}kubelb-ccm:${IMAGE_TAG}"
GO_VERSION="${GO_VERSION:-1.24}"
GIT_VERSION="${GIT_VERSION:-e2e}"
GIT_COMMIT="${GIT_COMMIT:-$(git rev-parse --short HEAD)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

# Cluster type detection
USE_KIND="${USE_KIND:-auto}"                # auto, true, false
SKIP_BUILD="${SKIP_BUILD:-false}"           # Skip image building (use pre-built images)
SKIP_IMAGE_LOAD="${SKIP_IMAGE_LOAD:-false}" # Skip image loading
METALLB_IP_RANGE="${METALLB_IP_RANGE:-}"    # Override MetalLB IP range for cloud

mkdir -p "${LOGS_DIR}"

#######################################
# Detect if using kind clusters
#######################################
is_kind_cluster() {
  if [[ "${USE_KIND}" == "true" ]]; then
    return 0
  elif [[ "${USE_KIND}" == "false" ]]; then
    return 1
  fi
  # Auto-detect: check if kind cluster exists
  kind get clusters 2> /dev/null | grep -q "^kubelb$"
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
# Verify prerequisites
#######################################
verify_kubeconfigs() {
  for cluster in kubelb tenant1 tenant2; do
    if [[ ! -f "${KUBECONFIGS_DIR}/${cluster}.kubeconfig" ]]; then
      echo "Error: ${KUBECONFIGS_DIR}/${cluster}.kubeconfig not found"
      echo "Run 'make e2e-setup-kind' first"
      exit 1
    fi
  done
}

#######################################
# Build images in parallel
# Uses host Go cache for fast builds, then packages with goreleaser dockerfiles
# Skips docker build if image with same binary hash exists
#######################################
build_images() {
  if [[ "${SKIP_BUILD}" == "true" ]]; then
    echodate "Skipping image build (SKIP_BUILD=true)"
    return 0
  fi

  echodate "Building images..."
  local build_start=$(nowms)
  local bin_dir="${ROOT_DIR}/bin"

  # Check if source files have changed since last build
  local src_hash
  src_hash=$(echo "TAGS=e2e $(git ls-files -s cmd/ internal/ api/ pkg/ go.mod go.sum 2> /dev/null)" | sha256sum | cut -c1-12)
  local src_hash_file="${bin_dir}/.src-hash"

  if [[ -f "${src_hash_file}" ]] && [[ "$(cat "${src_hash_file}")" == "${src_hash}" ]] &&
    [[ -f "${bin_dir}/kubelb" ]] && [[ -f "${bin_dir}/ccm" ]]; then
    echodate "Source unchanged (hash: ${src_hash}), skipping binary build"
  else
    # Build binaries using Makefile (uses host Go cache, cross-compile for linux)
    # Use TAGS=e2e to enable relaxed gateway name restrictions for parallel e2e tests
    echodate "Building binaries via make (hash: ${src_hash})..."
    local build_log="${LOGS_DIR}/build.log"
    if ! GOOS=linux GOARCH=amd64 make build-kubelb build-ccm \
      TAGS=e2e \
      GIT_VERSION="${GIT_VERSION}" \
      GIT_COMMIT="${GIT_COMMIT}" \
      BUILD_DATE="${BUILD_DATE}" \
      &> "${build_log}"; then
      echodate "FAILED: binary build (see ${build_log})"
      exit 1
    fi
    echo "${src_hash}" > "${src_hash_file}"
    echodate "Binaries built"
  fi

  # Compute per-binary hashes for granular cache detection
  local kubelb_hash ccm_hash
  kubelb_hash=$(sha256sum "${bin_dir}/kubelb" | cut -c1-12)
  ccm_hash=$(sha256sum "${bin_dir}/ccm" | cut -c1-12)
  local kubelb_hash_image="kubelb:${kubelb_hash}"
  local ccm_hash_image="kubelb-ccm:${ccm_hash}"

  # Package images using goreleaser dockerfiles (just COPY binary)
  # Skip each image independently if hash already exists
  local failed=()
  local build_pids=()

  if docker image inspect "${kubelb_hash_image}" &> /dev/null; then
    echodate "kubelb image with hash ${kubelb_hash} exists, skipping"
  else
    echodate "Building kubelb image (hash: ${kubelb_hash})..."
    docker build -t "${kubelb_hash_image}" -f kubelb.goreleaser.dockerfile "${bin_dir}" &
    build_pids+=("kubelb:$!")
  fi

  if docker image inspect "${ccm_hash_image}" &> /dev/null; then
    echodate "ccm image with hash ${ccm_hash} exists, skipping"
  else
    echodate "Building ccm image (hash: ${ccm_hash})..."
    docker build -t "${ccm_hash_image}" -f ccm.goreleaser.dockerfile "${bin_dir}" &
    build_pids+=("ccm:$!")
  fi

  # Wait for any builds that were started
  for entry in "${build_pids[@]}"; do
    local name="${entry%%:*}"
    local pid="${entry##*:}"
    if ! wait ${pid}; then
      failed+=("${name}")
      echodate "FAILED: ${name} image packaging"
    else
      echodate "Built ${name} image"
    fi
  done

  if [[ ${#failed[@]} -gt 0 ]]; then
    exit 1
  fi

  # Tag hash images with :e2e for helm to use
  docker tag "${kubelb_hash_image}" "${KUBELB_IMAGE}"
  docker tag "${ccm_hash_image}" "${CCM_IMAGE}"
  echodate "Tagged ${KUBELB_IMAGE} and ${CCM_IMAGE}"

  # Push to registry if configured (for cloud clusters)
  if [[ -n "${IMAGE_REGISTRY}" ]]; then
    echodate "Pushing images to registry..."
    docker push "${KUBELB_IMAGE}" &
    docker push "${CCM_IMAGE}" &
    wait
  fi

  printElapsed "image_builds" ${build_start}
}

#######################################
# Load images into kind clusters
#######################################
load_images() {
  if [[ "${SKIP_IMAGE_LOAD}" == "true" ]]; then
    echodate "Skipping image load (SKIP_IMAGE_LOAD=true)"
    return 0
  fi

  if ! is_kind_cluster; then
    echodate "Skipping image load (not using kind clusters)"
    return 0
  fi

  echodate "Loading images into Kind clusters..."
  local load_start=$(nowms)

  # Load images in parallel to all clusters
  # kubelb cluster: needs kubelb + ccm images
  # tenant clusters: need ccm image (CCM runs there)
  kind load docker-image --name=kubelb "${KUBELB_IMAGE}" "${CCM_IMAGE}" &
  kind load docker-image --name=tenant1 "${CCM_IMAGE}" &
  kind load docker-image --name=tenant2 "${CCM_IMAGE}" &
  wait

  printElapsed "image_loads" ${load_start}
}

#######################################
# Deploy KubeLB manager (includes addons as subchart)
#######################################
deploy_kubelb_manager() {
  echodate "Deploying KubeLB manager..."
  local deploy_start=$(nowms)

  export KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig"

  # Create high-priority PriorityClass for kubelb manager
  kubectl apply -f - << EOF
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: kubelb-high-priority
value: 1000000
globalDefault: false
description: "High priority for kubelb manager to ensure it starts before addons"
EOF

  # Set pull policy based on cluster type
  local pull_policy="IfNotPresent"
  if is_kind_cluster && [[ -z "${IMAGE_REGISTRY}" ]]; then
    pull_policy="Never"
  fi

  # Create temp chart with local kubelb-addons dependency
  # This ensures e2e tests use latest local charts
  local temp_chart_dir="${KUBECONFIGS_DIR}/charts"
  rm -rf "${temp_chart_dir}"
  mkdir -p "${temp_chart_dir}"
  cp -r "${CHARTS_DIR}/kubelb-manager" "${temp_chart_dir}/"
  cp -r "${CHARTS_DIR}/kubelb-addons" "${temp_chart_dir}/"

  # Patch Chart.yaml to use local kubelb-addons
  sed -i.bak 's|repository: oci://quay.io/kubermatic/helm-charts|repository: file://../kubelb-addons|g' \
    "${temp_chart_dir}/kubelb-manager/Chart.yaml"
  rm -f "${temp_chart_dir}/kubelb-manager/Chart.yaml.bak"

  # Update helm dependencies (skip if unchanged)
  local chart_hash
  chart_hash=$(sha256sum "${CHARTS_DIR}/kubelb-manager/Chart.yaml" "${CHARTS_DIR}/kubelb-addons/Chart.yaml" 2> /dev/null | sha256sum | cut -c1-8)
  if [[ ! -f "${temp_chart_dir}/.helm-deps-${chart_hash}" ]]; then
    echodate "Ensuring helm repos are configured..."
    "${ROOT_DIR}/hack/ensure-helm-repos.sh"
    echodate "Building kubelb-addons dependencies..."
    helm dependency build "${temp_chart_dir}/kubelb-addons"
    echodate "Updating kubelb-manager dependencies..."
    helm dependency update "${temp_chart_dir}/kubelb-manager"
    touch "${temp_chart_dir}/.helm-deps-${chart_hash}"
  else
    echodate "Helm dependencies unchanged, skipping update"
  fi

  # Install/upgrade manager with full values (no --wait, addons install in background)
  local helm_cmd
  helm_cmd=$(get_helm_command "kubelb" "kubelb")
  echodate "Running helm ${helm_cmd} for kubelb manager..."
  helm ${helm_cmd} kubelb "${temp_chart_dir}/kubelb-manager" \
    --namespace kubelb \
    --create-namespace \
    --values "${E2E_MANIFESTS_DIR}/kubelb-manager/values.yaml" \
    --set image.repository="${KUBELB_IMAGE%:*}" \
    --set image.tag="${KUBELB_IMAGE#*:}" \
    --set image.pullPolicy="${pull_policy}" \
    --set imagePullSecrets=

  # Apply Config CR (required for kubelb-manager when skipConfigGeneration=true)
  echodate "Applying Config CR..."
  kubectl apply -f "${E2E_MANIFESTS_DIR}/kubelb-manager/config.yaml"

  # Wait for kubelb manager only (required before tenant/CCM setup)
  echodate "Waiting for kubelb manager..."
  kubectl -n kubelb wait --for=condition=available deployment/kubelb --timeout=5m

  printElapsed "kubelb_manager_deploy" ${deploy_start}
}

#######################################
# Configure MetalLB IP pool
#######################################
configure_metallb_pool() {
  # Skip if metallb not needed
  if ! is_kind_cluster && [[ -z "${METALLB_IP_RANGE}" ]]; then
    return 0
  fi

  echodate "Configuring MetalLB IP pool..."
  export KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig"

  local metallb_ip_range="${METALLB_IP_RANGE}"

  if [[ -z "${metallb_ip_range}" ]]; then
    # Auto-detect from Kind's Docker network (IPv4 only, filter out nulls and IPv6)
    local metallb_gw
    metallb_gw=$(docker network inspect -f json kind | jq --raw-output '.[].IPAM.Config[].Gateway | select(. != null) | select(contains(":") | not)' | sed -e 's/\(.*\..*\).*\..*\..*/\1/')
    metallb_ip_range="${metallb_gw}.255.200-${metallb_gw}.255.250"
  fi

  echodate "MetalLB IP pool: ${metallb_ip_range}"
  sed "s|\${METALLB_IP_RANGE}|${metallb_ip_range}|g" \
    "${E2E_MANIFESTS_DIR}/metallb/ipaddresspool.yaml.tpl" | kubectl apply -f -
}

#######################################
# Setup tenants (create Tenant CRs in kubelb cluster)
#######################################
setup_tenants() {
  echodate "Setting up tenants in kubelb cluster..."
  local setup_start=$(nowms)

  export KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig"

  declare -A TENANT_MAP=(
    ["primary"]="tenant1"
    ["secondary"]="tenant2"
  )

  for tenant in "${!TENANT_MAP[@]}"; do
    echodate "Creating Tenant CR: ${tenant}"
    kubectl apply -f "${E2E_MANIFESTS_DIR}/tenants/${tenant}.yaml"
  done

  # Wait for tenant CRs to be ready (typically <1s)
  for tenant in "${!TENANT_MAP[@]}"; do
    kubectl wait --for=jsonpath='{.status.ready}'=true tenant/"${tenant}" -n kubelb --timeout=5s 2> /dev/null || true
  done

  printElapsed "tenant_setup" ${setup_start}
}

#######################################
# Deploy CCM to tenant clusters
#######################################
deploy_ccms() {
  echodate "Deploying CCMs to tenant clusters..."
  local deploy_start=$(nowms)

  # Prepare kubeconfig for CCM to connect to kubelb management cluster
  local kubelb_kubeconfig="${KUBECONFIGS_DIR}/kubelb.kubeconfig"
  if is_kind_cluster; then
    kubelb_kubeconfig="${KUBECONFIGS_DIR}/kubelb-internal.kubeconfig"
  fi

  # Set pull policy based on cluster type
  local pull_policy="IfNotPresent"
  if is_kind_cluster && [[ -z "${IMAGE_REGISTRY}" ]]; then
    pull_policy="Never"
  fi

  declare -A TENANT_MAP=(
    ["primary"]="tenant1"
    ["secondary"]="tenant2"
  )

  local ccm_pids=()
  for tenant in "${!TENANT_MAP[@]}"; do
    local cluster="${TENANT_MAP[$tenant]}"

    (
      export KUBECONFIG="${KUBECONFIGS_DIR}/${cluster}.kubeconfig"
      echodate "Deploying CCM to ${cluster} for tenant: ${tenant}"

      # Create kubelb namespace in tenant cluster
      kubectl create ns kubelb 2> /dev/null || true

      # Create secret with kubeconfig to kubelb management cluster
      kubectl -n kubelb create secret generic kubelb-cluster \
        --from-file=kubelb="${kubelb_kubeconfig}" \
        --dry-run=client -o yaml | kubectl apply -f -

      # Deploy CCM via helm in tenant cluster
      local helm_cmd
      helm_cmd=$(get_helm_command "kubelb-ccm" "kubelb")
      echodate "Running helm ${helm_cmd} for kubelb-ccm on ${cluster}..."
      helm ${helm_cmd} kubelb-ccm "${CHARTS_DIR}/kubelb-ccm" \
        --namespace kubelb \
        --values "${E2E_MANIFESTS_DIR}/kubelb-ccm/values.yaml" \
        --set image.repository="${CCM_IMAGE%:*}" \
        --set image.tag="${CCM_IMAGE#*:}" \
        --set image.pullPolicy="${pull_policy}" \
        --set imagePullSecrets= \
        --set kubelb.tenantName="${tenant}"
    ) &
    ccm_pids+=($!)
  done

  # Wait for all CCM deployments
  local failed=()
  for pid in "${ccm_pids[@]}"; do
    wait ${pid} || failed+=("ccm")
  done

  if [[ ${#failed[@]} -gt 0 ]]; then
    echodate "ERROR: Failed to deploy some CCMs"
    exit 1
  fi

  printElapsed "ccm_deploy" ${deploy_start}
}

#######################################
# Wait for all deployments (runs all checks in parallel)
#######################################
wait_for_ready() {
  echodate "Verifying deployments in parallel..."
  local wait_start=$(nowms)

  declare -A TENANT_MAP=(
    ["primary"]="tenant1"
    ["secondary"]="tenant2"
  )

  # Launch all wait operations in parallel (manager already waited in deploy_kubelb_manager)
  # CCMs in tenant clusters
  local ccm_pids=()
  for tenant in "${!TENANT_MAP[@]}"; do
    local cluster="${TENANT_MAP[$tenant]}"
    KUBECONFIG="${KUBECONFIGS_DIR}/${cluster}.kubeconfig" \
      kubectl -n kubelb wait --for=condition=available deployment/kubelb-ccm --timeout=120s &
    ccm_pids+=($!)
  done

  # Gateway API CRDs
  KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
    kubectl wait --for=condition=established crd gatewayclasses.gateway.networking.k8s.io --timeout=5m &
  local gateway_crd_pid=$!

  # cert-manager CRDs (required for ClusterIssuer in manifests.yaml)
  KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
    kubectl wait --for=condition=established crd clusterissuers.cert-manager.io --timeout=5m &
  local certmanager_crd_pid=$!

  # cert-manager webhook
  KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
    kubectl -n kubelb wait --for=condition=available deployment/kubelb-cert-manager-webhook --timeout=5m &
  local webhook_pid=$!

  # ingress-nginx controller
  KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
    kubectl -n kubelb wait --for=condition=available deployment/kubelb-ingress-nginx-controller --timeout=5m &
  local ingress_pid=$!

  # MetalLB controller (required before IPAddressPool can be created)
  KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
    kubectl -n kubelb wait --for=condition=available deployment/kubelb-metallb-controller --timeout=5m &
  local metallb_pid=$!

  # Wait for all and check for failures
  local failed=()
  for pid in "${ccm_pids[@]}"; do
    wait ${pid} || failed+=("ccm")
  done
  wait ${gateway_crd_pid} || failed+=("gateway-crds")
  wait ${certmanager_crd_pid} || failed+=("cert-manager-crds")
  wait ${webhook_pid} || failed+=("cert-manager-webhook")
  wait ${ingress_pid} || failed+=("ingress-nginx")
  wait ${metallb_pid} || failed+=("metallb")

  if [[ ${#failed[@]} -gt 0 ]]; then
    echodate "ERROR: Failed to verify: ${failed[*]}"
    exit 1
  fi

  printElapsed "readiness_checks" ${wait_start}

  # Configure MetalLB IP pool (metallb controller is now ready)
  configure_metallb_pool

  echodate "Applying kubelb-manager manifests..."
  export KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig"
  kubectl apply -f "${E2E_MANIFESTS_DIR}/kubelb-manager/manifests.yaml"

  echodate "All deployments ready"
}

#######################################
# Deploy shared test apps to tenant clusters
#######################################
deploy_test_apps() {
  echodate "Deploying shared test apps to tenant clusters..."

  declare -A TENANT_MAP=(
    ["primary"]="tenant1"
    ["secondary"]="tenant2"
  )

  for tenant in "${!TENANT_MAP[@]}"; do
    local cluster="${TENANT_MAP[$tenant]}"
    KUBECONFIG="${KUBECONFIGS_DIR}/${cluster}.kubeconfig" \
      kubectl apply -f "${E2E_MANIFESTS_DIR}/test-apps/echo-server.yaml" &
  done

  wait
  echodate "Test apps deployed (pods starting in background)"
}

#######################################
# Main
#######################################
SCRIPT_START=$(nowms)

echodate "============================================"
echodate "  ${TEST_NAME}"
echodate "============================================"
echodate "Kubelb image: ${KUBELB_IMAGE}"
echodate "CCM image: ${CCM_IMAGE}"
if is_kind_cluster; then
  echodate "Cluster type: kind"
else
  echodate "Cluster type: external"
fi
echodate ""

verify_kubeconfigs
build_images
load_images
deploy_kubelb_manager
setup_tenants
deploy_ccms
wait_for_ready
deploy_test_apps

echodate ""
echodate "============================================"
echodate "  Deployment Complete"
echodate "============================================"
echodate ""
echodate "Clusters ready:"
echodate "  kubelb:  export KUBECONFIG=${KUBECONFIGS_DIR}/kubelb.kubeconfig"
echodate "  tenant1: export KUBECONFIG=${KUBECONFIGS_DIR}/tenant1.kubeconfig"
echodate "  tenant2: export KUBECONFIG=${KUBECONFIGS_DIR}/tenant2.kubeconfig"
echodate ""
printElapsed "total_deploy" ${SCRIPT_START}
