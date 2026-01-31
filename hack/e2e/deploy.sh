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
GO_VERSION="${GO_VERSION:-1.25.6}"
GIT_VERSION="${GIT_VERSION:-e2e}"
GIT_COMMIT="${GIT_COMMIT:-$(git rev-parse --short HEAD)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

# Cluster type detection
USE_KIND="${USE_KIND:-auto}"                # auto, true, false
SKIP_BUILD="${SKIP_BUILD:-false}"           # Skip image building (use pre-built images)
SKIP_IMAGE_LOAD="${SKIP_IMAGE_LOAD:-false}" # Skip image loading
METALLB_IP_RANGE="${METALLB_IP_RANGE:-}"    # Override MetalLB IP range for cloud
CONVERSION_MODE="${CONVERSION_MODE:-false}" # Deploy CCM in standalone conversion mode

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
  for cluster in kubelb tenant1 tenant2 standalone; do
    if [[ ! -f "${KUBECONFIGS_DIR}/${cluster}.kubeconfig" ]]; then
      echo "Error: ${KUBECONFIGS_DIR}/${cluster}.kubeconfig not found"
      echo "Run 'make e2e-setup-kind' first"
      exit 1
    fi
  done
}

#######################################
# Build images using centralized make targets
#######################################
build_images() {
  if [[ "${SKIP_BUILD}" == "true" ]]; then
    echodate "Skipping image build (SKIP_BUILD=true)"
    return 0
  fi

  echodate "Building images..."
  local build_start=$(nowms)
  local build_log="${LOGS_DIR}/build.log"

  # Use centralized make targets with cross-compile for linux/amd64
  # BUILD_DATE=e2e ensures reproducible builds (matches reload.sh)
  if ! GOOS=linux GOARCH=amd64 make e2e-image-kubelb e2e-image-ccm \
    GIT_VERSION="${GIT_VERSION}" \
    GIT_COMMIT="${GIT_COMMIT}" \
    BUILD_DATE=e2e \
    &> "${build_log}"; then
    echodate "FAILED: image build (see ${build_log})"
    cat "${build_log}"
    exit 1
  fi

  # Store binary hashes for reload.sh compatibility
  local hash_dir="${KUBECONFIGS_DIR}/.reload-hashes"
  mkdir -p "${hash_dir}"
  sha256sum "${ROOT_DIR}/bin/kubelb" | cut -c1-12 > "${hash_dir}/kubelb.hash"
  sha256sum "${ROOT_DIR}/bin/ccm" | cut -c1-12 > "${hash_dir}/ccm.hash"

  echodate "Built ${KUBELB_IMAGE} and ${CCM_IMAGE}"

  # Push to registry if configured (for cloud clusters)
  if [[ -n "${IMAGE_REGISTRY}" ]]; then
    echodate "Pushing images to registry..."
    docker tag kubelb:e2e "${KUBELB_IMAGE}"
    docker tag kubelb-ccm:e2e "${CCM_IMAGE}"
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
  # standalone cluster: needs ccm image (standalone mode)
  kind load docker-image --name=kubelb "${KUBELB_IMAGE}" "${CCM_IMAGE}" &
  kind load docker-image --name=tenant1 "${CCM_IMAGE}" &
  kind load docker-image --name=tenant2 "${CCM_IMAGE}" &
  kind load docker-image --name=standalone "${CCM_IMAGE}" &
  wait

  printElapsed "image_loads" ${load_start}
}

#######################################
# Ensure helm repos are configured (call once before deploys)
#######################################
ensure_helm_repos_once() {
  local marker="${KUBECONFIGS_DIR}/.helm-repos-configured"
  if [[ ! -f "${marker}" ]]; then
    echodate "Ensuring helm repos are configured..."
    "${ROOT_DIR}/hack/ensure-helm-repos.sh"
    touch "${marker}"
  fi
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
  # Use manager-specific subdir to avoid race with parallel standalone deploy
  local temp_chart_dir="${KUBECONFIGS_DIR}/charts/manager"
  rm -rf "${temp_chart_dir}"
  mkdir -p "${temp_chart_dir}"
  cp -r "${CHARTS_DIR}/kubelb-manager" "${temp_chart_dir}/"
  cp -r "${CHARTS_DIR}/kubelb-addons" "${temp_chart_dir}/"

  # Patch Chart.yaml to use local kubelb-addons
  sed -i.bak 's|repository: oci://quay.io/kubermatic/helm-charts|repository: file://../kubelb-addons|g' \
    "${temp_chart_dir}/kubelb-manager/Chart.yaml"
  rm -f "${temp_chart_dir}/kubelb-manager/Chart.yaml.bak"

  # Patch dependency version to match local addons chart version
  local addons_version
  addons_version=$(grep '^version:' "${temp_chart_dir}/kubelb-addons/Chart.yaml" | awk '{print $2}')
  sed -i.bak "/name: kubelb-addons/,/version:/ s|version: .*|version: ${addons_version}|" \
    "${temp_chart_dir}/kubelb-manager/Chart.yaml"
  rm -f "${temp_chart_dir}/kubelb-manager/Chart.yaml.bak"

  # Update helm dependencies (skip if unchanged)
  local chart_hash
  chart_hash=$(sha256sum "${CHARTS_DIR}/kubelb-manager/Chart.yaml" "${CHARTS_DIR}/kubelb-addons/Chart.yaml" 2> /dev/null | sha256sum | cut -c1-8)
  if [[ ! -f "${temp_chart_dir}/.helm-deps-${chart_hash}" ]]; then
    ensure_helm_repos_once
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

  # Restart deployment to pick up new image (tag is fixed :e2e)
  if [[ "${helm_cmd}" == "upgrade" ]]; then
    echodate "Restarting kubelb deployment to pick up new image..."
    kubectl -n kubelb rollout restart deployment/kubelb
    kubectl -n kubelb wait --for=condition=available deployment/kubelb --timeout=5m
  fi

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

  # In standalone mode, only deploy to tenant1 with standalone settings
  if [[ "${CONVERSION_MODE}" == "true" ]]; then
    unset TENANT_MAP
    declare -A TENANT_MAP=(
      ["primary"]="tenant1"
    )
  fi

  local ccm_pids=()
  for tenant in "${!TENANT_MAP[@]}"; do
    local cluster="${TENANT_MAP[$tenant]}"

    (
      export KUBECONFIG="${KUBECONFIGS_DIR}/${cluster}.kubeconfig"
      echodate "Deploying CCM to ${cluster} for tenant: ${tenant}"

      # Create kubelb namespace in tenant cluster
      kubectl create ns kubelb 2> /dev/null || true

      # Skip kubelb-cluster secret in standalone conversion mode
      if [[ "${CONVERSION_MODE}" != "true" ]]; then
        # Create secret with kubeconfig to kubelb management cluster
        kubectl -n kubelb create secret generic kubelb-cluster \
          --from-file=kubelb="${kubelb_kubeconfig}" \
          --dry-run=client -o yaml | kubectl apply -f -
      fi

      # Deploy CCM via helm in tenant cluster
      local helm_cmd
      helm_cmd=$(get_helm_command "kubelb-ccm" "kubelb")
      echodate "Running helm ${helm_cmd} for kubelb-ccm on ${cluster}..."

      local values_file="${E2E_MANIFESTS_DIR}/kubelb-ccm/values.yaml"
      local extra_args=""

      if [[ "${CONVERSION_MODE}" == "true" ]]; then
        values_file="${E2E_MANIFESTS_DIR}/kubelb-ccm/values-standalone.yaml"
      else
        extra_args="--set kubelb.tenantName=${tenant}"
        # tenant2 uses integrated conversion mode (kubelb + local HTTPRoute conversion)
        if [[ "$cluster" == "tenant2" ]]; then
          values_file="${E2E_MANIFESTS_DIR}/kubelb-ccm/values-integrated.yaml"
        fi
      fi

      helm ${helm_cmd} kubelb-ccm "${CHARTS_DIR}/kubelb-ccm" \
        --namespace kubelb \
        --values "${values_file}" \
        --set image.repository="${CCM_IMAGE%:*}" \
        --set image.tag="${CCM_IMAGE#*:}" \
        --set image.pullPolicy="${pull_policy}" \
        --set imagePullSecrets= \
        ${extra_args}

      # Restart deployment to pick up new image (tag is fixed :e2e)
      if [[ "${helm_cmd}" == "upgrade" ]]; then
        echodate "Restarting kubelb-ccm deployment on ${cluster}..."
        kubectl -n kubelb rollout restart deployment/kubelb-ccm
      fi
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

  # In standalone mode, only wait for tenant1
  if [[ "${CONVERSION_MODE}" == "true" ]]; then
    unset TENANT_MAP
    declare -A TENANT_MAP=(
      ["primary"]="tenant1"
    )
  fi

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
    kubectl -n kubelb wait --for=condition=available deployment/kubelb-ingress-nginx-controller --timeout=7m &
  local ingress_pid=$!

  # MetalLB controller (required before IPAddressPool can be created)
  KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
    kubectl -n kubelb wait --for=condition=available deployment/kubelb-metallb-controller --timeout=7m &
  local metallb_pid=$!

  # Envoy Gateway controller (required for Gateway API data plane)
  KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
    kubectl -n kubelb wait --for=condition=available deployment/envoy-gateway --timeout=7m &
  local envoy_gw_pid=$!

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
  wait ${envoy_gw_pid} || failed+=("envoy-gateway")

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
# Deploy kubelb-addons to standalone cluster (standalone mode)
#######################################
deploy_standalone_addons() {
  echodate "Deploying kubelb-addons to standalone cluster..."
  local deploy_start=$(nowms)

  export KUBECONFIG="${KUBECONFIGS_DIR}/standalone.kubeconfig"

  # Create namespace
  kubectl create ns kubelb 2> /dev/null || true

  # Build addons dependencies (skip if unchanged - use hash-based caching)
  local addons_hash
  addons_hash=$(sha256sum "${CHARTS_DIR}/kubelb-addons/Chart.yaml" "${CHARTS_DIR}/kubelb-addons/Chart.lock" 2> /dev/null | sha256sum | cut -c1-8)
  local addons_cache_marker="${KUBECONFIGS_DIR}/.addons-deps-${addons_hash}"
  if [[ ! -f "${addons_cache_marker}" ]]; then
    ensure_helm_repos_once
    echodate "Building kubelb-addons dependencies..."
    helm dependency build "${CHARTS_DIR}/kubelb-addons" &> /dev/null
    touch "${addons_cache_marker}"
  else
    echodate "Addons dependencies unchanged, skipping build"
  fi

  # Install/upgrade addons
  local helm_cmd
  helm_cmd=$(get_helm_command "kubelb-addons" "kubelb")

  helm ${helm_cmd} kubelb-addons "${CHARTS_DIR}/kubelb-addons" \
    --namespace kubelb \
    --values "${E2E_MANIFESTS_DIR}/standalone/addons-values.yaml"

  printElapsed "standalone_addons_deploy" ${deploy_start}
}

#######################################
# Deploy CCM in standalone conversion mode to standalone cluster
#######################################
deploy_standalone_ccm() {
  echodate "Deploying CCM in standalone conversion mode..."
  local deploy_start=$(nowms)

  export KUBECONFIG="${KUBECONFIGS_DIR}/standalone.kubeconfig"

  # Set pull policy based on cluster type
  local pull_policy="IfNotPresent"
  if is_kind_cluster && [[ -z "${IMAGE_REGISTRY}" ]]; then
    pull_policy="Never"
  fi

  # Install/upgrade CCM
  local helm_cmd
  helm_cmd=$(get_helm_command "kubelb-ccm" "kubelb")

  helm ${helm_cmd} kubelb-ccm "${CHARTS_DIR}/kubelb-ccm" \
    --namespace kubelb \
    --values "${E2E_MANIFESTS_DIR}/kubelb-ccm/values-standalone.yaml" \
    --set image.repository="${CCM_IMAGE%:*}" \
    --set image.tag="${CCM_IMAGE#*:}" \
    --set image.pullPolicy="${pull_policy}" \
    --set imagePullSecrets=

  # Restart deployment to pick up new image (tag is fixed :e2e)
  if [[ "${helm_cmd}" == "upgrade" ]]; then
    echodate "Restarting kubelb-ccm deployment on standalone cluster..."
    kubectl -n kubelb rollout restart deployment/kubelb-ccm
  fi

  printElapsed "standalone_ccm_deploy" ${deploy_start}
}

#######################################
# Wait for standalone cluster addons to be ready
#######################################
wait_for_standalone_ready() {
  echodate "Waiting for standalone cluster to be ready..."
  local wait_start=$(nowms)

  export KUBECONFIG="${KUBECONFIGS_DIR}/standalone.kubeconfig"

  # Wait for Gateway API CRDs
  kubectl wait --for=condition=established crd gatewayclasses.gateway.networking.k8s.io --timeout=5m &
  local gateway_crd_pid=$!

  # Wait for cert-manager CRDs
  kubectl wait --for=condition=established crd certificates.cert-manager.io --timeout=5m &
  local cert_crd_pid=$!

  # Wait for ingress-nginx
  kubectl -n kubelb wait --for=condition=available deployment/kubelb-addons-ingress-nginx-controller --timeout=7m &
  local ingress_pid=$!

  # Wait for envoy-gateway
  kubectl -n kubelb wait --for=condition=available deployment/envoy-gateway --timeout=7m &
  local envoy_pid=$!

  # Wait for MetalLB
  kubectl -n kubelb wait --for=condition=available deployment/kubelb-addons-metallb-controller --timeout=7m &
  local metallb_pid=$!

  # Wait for cert-manager
  kubectl -n kubelb wait --for=condition=available deployment/kubelb-addons-cert-manager --timeout=5m &
  local certmanager_pid=$!

  # Wait for CCM
  kubectl -n kubelb wait --for=condition=available deployment/kubelb-ccm --timeout=5m &
  local ccm_pid=$!

  # Collect results
  local failed=()
  wait ${gateway_crd_pid} || failed+=("gateway-crds")
  wait ${cert_crd_pid} || failed+=("cert-manager-crds")
  wait ${ingress_pid} || failed+=("ingress-nginx")
  wait ${envoy_pid} || failed+=("envoy-gateway")
  wait ${metallb_pid} || failed+=("metallb")
  wait ${certmanager_pid} || failed+=("cert-manager")
  wait ${ccm_pid} || failed+=("ccm")

  if [[ ${#failed[@]} -gt 0 ]]; then
    echodate "ERROR: Failed to verify standalone cluster: ${failed[*]}"
    exit 1
  fi

  printElapsed "standalone_ready" ${wait_start}

  # Configure MetalLB IP pool for standalone cluster
  configure_standalone_metallb_pool

  # Apply GatewayClass and ClusterIssuer
  echodate "Applying manifests to standalone cluster..."
  kubectl apply -f "${E2E_MANIFESTS_DIR}/kubelb-manager/manifests.yaml"
}

#######################################
# Configure MetalLB IP pool for standalone cluster
#######################################
configure_standalone_metallb_pool() {
  echodate "Configuring MetalLB IP pool for standalone cluster..."
  export KUBECONFIG="${KUBECONFIGS_DIR}/standalone.kubeconfig"

  local metallb_ip_range="${METALLB_IP_RANGE}"

  if [[ -z "${metallb_ip_range}" ]]; then
    # Auto-detect from Kind's Docker network (use different range than kubelb cluster)
    local metallb_gw
    metallb_gw=$(docker network inspect -f json kind | jq --raw-output '.[].IPAM.Config[].Gateway | select(. != null) | select(contains(":") | not)' | sed -e 's/\(.*\..*\).*\..*\..*/\1/')
    # Use .255.150-.255.199 range (different from kubelb's .255.200-.255.250)
    metallb_ip_range="${metallb_gw}.255.150-${metallb_gw}.255.199"
  fi

  echodate "Conversion MetalLB IP pool: ${metallb_ip_range}"
  sed "s|\${METALLB_IP_RANGE}|${metallb_ip_range}|g" \
    "${E2E_MANIFESTS_DIR}/metallb/ipaddresspool.yaml.tpl" | kubectl apply -f -
}

#######################################
# Deploy shared test apps to tenant clusters
#######################################
deploy_test_apps() {
  echodate "Deploying shared test apps to all clusters..."

  declare -A TENANT_MAP=(
    ["primary"]="tenant1"
    ["secondary"]="tenant2"
  )

  for tenant in "${!TENANT_MAP[@]}"; do
    local cluster="${TENANT_MAP[$tenant]}"
    KUBECONFIG="${KUBECONFIGS_DIR}/${cluster}.kubeconfig" \
      kubectl apply -f "${E2E_MANIFESTS_DIR}/test-apps/echo-server.yaml" &
  done

  # Also deploy to standalone cluster
  KUBECONFIG="${KUBECONFIGS_DIR}/standalone.kubeconfig" \
    kubectl apply -f "${E2E_MANIFESTS_DIR}/test-apps/echo-server.yaml" &

  wait
  echodate "Test apps deployed (pods starting in background)"
}

#######################################
# Wait for deployment to exist then wait for readiness
# Handles race condition where deployment is created dynamically
#######################################
wait_for_deployment_exist_and_ready() {
  local namespace="$1"
  local deployment="$2"
  local timeout="${3:-300}"

  local deadline=$(($(date +%s) + timeout))
  while ! kubectl -n "${namespace}" get deployment "${deployment}" &> /dev/null; do
    if [[ $(date +%s) -ge $deadline ]]; then
      echodate "Timeout waiting for ${deployment} to exist"
      return 1
    fi
    sleep 2
  done

  kubectl -n "${namespace}" wait --for=condition=available "deployment/${deployment}" --timeout="${timeout}s"
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

# Ensure helm repos configured once before parallel deploys
ensure_helm_repos_once

# Deploy kubelb manager and standalone addons in parallel (both do similar helm work)
deploy_kubelb_manager &
manager_pid=$!
deploy_standalone_addons &
standalone_addons_pid=$!

# Wait for manager before tenant setup (tenants need manager CRDs)
wait ${manager_pid}

setup_tenants
deploy_ccms &
ccm_pid=$!

# Wait for standalone addons before CCM deploy
wait ${standalone_addons_pid}
deploy_standalone_ccm &
standalone_ccm_pid=$!

# Wait for CCMs to finish
wait ${ccm_pid}
wait ${standalone_ccm_pid}

# Wait for all clusters to be ready in parallel
wait_for_ready &
wait_for_standalone_ready &
wait

deploy_test_apps

# Wait for KubeLB tenant envoy proxy deployments to be ready
# These are created dynamically after CCM creates LoadBalancer CRDs, so poll for existence first
echodate "Waiting for tenant envoy proxies..."
KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
  wait_for_deployment_exist_and_ready tenant-primary envoy-tenant-primary 300 &
KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
  wait_for_deployment_exist_and_ready tenant-secondary envoy-tenant-secondary 300 &
wait
echodate "Tenant envoy proxies ready"

echodate ""
echodate "============================================"
echodate "  Deployment Complete"
echodate "============================================"
echodate ""
echodate "Clusters ready:"
echodate "  kubelb:     export KUBECONFIG=${KUBECONFIGS_DIR}/kubelb.kubeconfig"
echodate "  tenant1:    export KUBECONFIG=${KUBECONFIGS_DIR}/tenant1.kubeconfig"
echodate "  tenant2:    export KUBECONFIG=${KUBECONFIGS_DIR}/tenant2.kubeconfig"
echodate "  standalone: export KUBECONFIG=${KUBECONFIGS_DIR}/standalone.kubeconfig"
echodate ""
printElapsed "total_deploy" ${SCRIPT_START}
