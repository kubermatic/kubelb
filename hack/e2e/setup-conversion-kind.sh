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

# Setup single Kind cluster for Ingress-to-Gateway conversion e2e tests.
# This is a lightweight alternative to setup-kind.sh that creates only one cluster
# for standalone conversion mode testing.

set -euo pipefail

export TEST_NAME="${TEST_NAME:-e2e-setup-conversion-kind}"
export ROOT_DIR="$(git rev-parse --show-toplevel)"
source "${ROOT_DIR}/hack/lib.sh"

TEST_FAILED=false

cleanup() {
  set +e
  if [[ "${CLEANUP_ON_EXIT:-false}" == "true" ]] && [[ "${TEST_FAILED}" == "true" ]]; then
    echodate "Test failed, cleaning up cluster..."
    kind delete clusters conversion &> /dev/null || true
  fi
}

trap cleanup EXIT

FORCE=false
CLEANUP_ON_EXIT=false

while [[ $# -gt 0 ]]; do
  case $1 in
  --force | -f)
    FORCE=true
    shift
    ;;
  --cleanup-on-failure)
    CLEANUP_ON_EXIT=true
    shift
    ;;
  -h | --help)
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --force, -f           Delete and recreate existing cluster"
    echo "  --cleanup-on-failure  Delete cluster if setup fails"
    echo "  -h, --help            Show this help"
    exit 0
    ;;
  *)
    echo "Unknown option: $1"
    echo "Run '$0 --help' for usage"
    exit 1
    ;;
  esac
done

export KUBECONFIGS_DIR="${KUBECONFIGS_DIR:-${ROOT_DIR}/.e2e-kubeconfigs}"
export LOGS_DIR="${KUBECONFIGS_DIR}/logs"
mkdir -p "${KUBECONFIGS_DIR}" "${LOGS_DIR}"

# Start go mod download and chainsaw install in background
GO_MOD_PID=""
(cd "${ROOT_DIR}" && go mod download 2> "${LOGS_DIR}/go-mod-download.log") &
GO_MOD_PID=$!

CHAINSAW_PID=""
(cd "${ROOT_DIR}" && make chainsaw &> "${LOGS_DIR}/chainsaw-install.log") &
CHAINSAW_PID=$!

KIND_IMAGE="${KIND_IMAGE:-kindest/node:v1.35.0}"

increase_inotify_limits() {
  local max_watches=524288
  local max_instances=512

  if [[ "${OS}" == "darwin" ]]; then
    echodate "Increasing inotify limits in Docker VM..."
    if command -v colima &> /dev/null && colima status &> /dev/null; then
      colima ssh -- sudo sysctl -w fs.inotify.max_user_watches=${max_watches} > /dev/null 2>&1 || true
      colima ssh -- sudo sysctl -w fs.inotify.max_user_instances=${max_instances} > /dev/null 2>&1 || true
    else
      docker run --privileged --rm alpine:latest sh -c \
        "sysctl -w fs.inotify.max_user_watches=${max_watches} && sysctl -w fs.inotify.max_user_instances=${max_instances}" \
        > /dev/null 2>&1 || true
    fi
  else
    echodate "Increasing inotify limits..."
    if [[ -w /proc/sys/fs/inotify/max_user_watches ]]; then
      echo ${max_watches} > /proc/sys/fs/inotify/max_user_watches
      echo ${max_instances} > /proc/sys/fs/inotify/max_user_instances
    else
      sudo sysctl -w fs.inotify.max_user_watches=${max_watches} > /dev/null 2>&1 || true
      sudo sysctl -w fs.inotify.max_user_instances=${max_instances} > /dev/null 2>&1 || true
    fi
  fi
}

setup_mac_docker_networking() {
  if [[ "${OS}" != "darwin" ]]; then
    return 0
  fi

  echodate "Setting up Docker network connectivity for Mac..."

  if ! docker context ls 2> /dev/null | grep -q "desktop-linux.*\*"; then
    echodate "WARNING: Docker Desktop not active. Container IPs may not be reachable."
    echodate "  Run: docker context use desktop-linux"
    return 0
  fi

  if ! brew list chipmk/tap/docker-mac-net-connect &> /dev/null; then
    echodate "Installing docker-mac-net-connect..."
    brew install chipmk/tap/docker-mac-net-connect
  fi

  local dmn_bin="/opt/homebrew/opt/docker-mac-net-connect/bin/docker-mac-net-connect"
  if [[ ! -x "${dmn_bin}" ]]; then
    echodate "WARNING: docker-mac-net-connect binary not found"
    return 1
  fi

  if netstat -rnf inet 2> /dev/null | grep -q "172.*utun"; then
    echodate "Docker network routes already configured"
    return 0
  fi

  local docker_socket
  docker_socket=$(docker context inspect --format '{{.Endpoints.docker.Host}}' 2> /dev/null || echo "")
  if [[ -z "${docker_socket}" ]]; then
    docker_socket="unix://${HOME}/.docker/run/docker.sock"
  fi

  sudo pkill -9 docker-mac-net-connect 2> /dev/null || true
  sleep 0.1

  local port_pid
  port_pid=$(sudo lsof -ti :3333 2> /dev/null || echo "")
  if [[ -n "${port_pid}" ]]; then
    echodate "Killing process holding port 3333 (PID: ${port_pid})..."
    sudo kill -9 ${port_pid} 2> /dev/null || true
    sleep 0.1
  fi

  echodate "Starting docker-mac-net-connect..."
  sudo DOCKER_HOST="${docker_socket}" DOCKER_API_VERSION=1.44 "${dmn_bin}" &> /dev/null &

  local retries=10
  local wait_time=0.1
  while [[ $retries -gt 0 ]]; do
    if netstat -rnf inet 2> /dev/null | grep -q "172.*utun"; then
      echodate "Docker network connectivity configured"
      return 0
    fi
    sleep ${wait_time}
    wait_time=$(echo "${wait_time} * 2" | bc)
    ((retries--))
  done

  echodate "WARNING: Docker network routes not detected. LoadBalancer IPs may not be reachable."
  return 1
}

cluster_exists() {
  kind get clusters 2> /dev/null | grep -q "^$1$"
}

delete_cluster() {
  local name=$1
  echodate "Deleting existing cluster: ${name}"
  kind delete cluster --name "${name}" &> /dev/null || true
}

create_cluster() {
  local name=$1
  local kubeconfig="${KUBECONFIGS_DIR}/${name}.kubeconfig"
  local logfile="${LOGS_DIR}/${name}.log"

  if cluster_exists "${name}"; then
    if [[ "${FORCE}" == "true" ]]; then
      delete_cluster "${name}"
    else
      echodate "Cluster ${name} exists, skipping (use --force to recreate)"
      kind get kubeconfig --name "${name}" > "${kubeconfig}" 2> /dev/null || true
      return 0
    fi
  fi

  echodate "Creating ${name} cluster (log: ${logfile})"

  local config="kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
kubeadmConfigPatches:
  - |
    kind: KubeletConfiguration
    serializeImagePulls: false
    maxParallelImagePulls: 10"

  KUBECONFIG="${kubeconfig}" kind create cluster \
    --retain \
    --name "${name}" \
    --image "${KIND_IMAGE}" \
    --config <(echo "${config}") &> "${logfile}"
}

prepull_images() {
  local images_file="${ROOT_DIR}/hack/e2e/images-conversion.yaml"
  if [[ ! -f "${images_file}" ]]; then
    echodate "No images-conversion.yaml found, skipping pre-pull"
    return 0
  fi

  # In CI, skip prepull - on-demand pulls are faster for cold cache
  if [[ "${CI:-}" == "true" ]] || [[ -n "${PROW_JOB_ID:-}" ]] || [[ -n "${JOB_NAME:-}" ]] || [[ -n "${GITHUB_ACTIONS:-}" ]]; then
    echodate "CI detected, skipping pre-pull (on-demand faster)"
    return 0
  fi

  # Parse images from YAML (flat list under "images:")
  local images=()
  while IFS= read -r line; do
    if [[ "${line}" =~ ^[[:space:]]*-[[:space:]]+(.+)$ ]]; then
      images+=("${BASH_REMATCH[1]}")
    fi
  done < "${images_file}"

  if [[ ${#images[@]} -eq 0 ]]; then
    echodate "No images found in images-conversion.yaml"
    return 0
  fi

  # Local dev: pull missing images to docker cache, then kind load
  # This makes subsequent runs fast (kind load from cache vs network pull)
  local missing_images=()
  for image in "${images[@]}"; do
    if ! docker image inspect "${image}" &> /dev/null; then
      missing_images+=("${image}")
    fi
  done

  if [[ ${#missing_images[@]} -gt 0 ]]; then
    echodate "Caching ${#missing_images[@]} images to docker (one-time for faster future runs)..."
    local pull_pids=()
    for image in "${missing_images[@]}"; do
      echodate "  Pulling ${image}..."
      docker pull "${image}" &> /dev/null &
      pull_pids+=($!)
    done
    for pid in "${pull_pids[@]}"; do
      wait ${pid} || true
    done
  fi

  echodate "Loading ${#images[@]} images into conversion cluster..."
  kind load docker-image --name "conversion" "${images[@]}" &> /dev/null

  echodate "Pre-pull complete"
}

SCRIPT_START=$(nowms)

echodate "Starting ${TEST_NAME} (Kind: ${KIND_IMAGE})"

increase_inotify_limits
setup_mac_docker_networking

echodate "Checking kind node image..."
PULL_START=$(nowms)
if ! docker image inspect "${KIND_IMAGE}" &> /dev/null; then
  echodate "Pulling ${KIND_IMAGE}..."
  docker pull "${KIND_IMAGE}"
  printElapsed "image_pull" $PULL_START
else
  echodate "Image already present"
fi

echodate ""
echodate "Creating Kind cluster for conversion tests..."
CLUSTER_CREATE_START=$(nowms)

if ! create_cluster "conversion"; then
  TEST_FAILED=true
  echodate "ERROR: Failed to create conversion cluster"
  echodate "Check log: ${LOGS_DIR}/conversion.log"
  exit 1
fi

printElapsed "cluster_creation" $CLUSTER_CREATE_START

# Pre-pull test images (skipped in CI, caches locally for faster subsequent runs)
echodate ""
PREPULL_START=$(nowms)
prepull_images
printElapsed "image_prepull" $PREPULL_START

# Wait for background tasks
if [[ -n "${GO_MOD_PID}" ]]; then
  if wait ${GO_MOD_PID} 2> /dev/null; then
    echodate "Go modules downloaded"
  else
    echodate "WARNING: go mod download failed (see ${LOGS_DIR}/go-mod-download.log)"
  fi
fi

if [[ -n "${CHAINSAW_PID}" ]]; then
  if wait ${CHAINSAW_PID} 2> /dev/null; then
    echodate "Chainsaw installed"
  else
    echodate "WARNING: chainsaw install failed (see ${LOGS_DIR}/chainsaw-install.log)"
  fi
fi

echodate "Setup complete: export KUBECONFIG=${KUBECONFIGS_DIR}/conversion.kubeconfig"
printElapsed "total_setup" $SCRIPT_START
