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

# Quick reload script for conversion tests
# Rebuilds and redeploys CCM only when binary changes
#
# Usage: make e2e-conversion-reload

set -euo pipefail

ROOT_DIR="$(git rev-parse --show-toplevel)"
source "${ROOT_DIR}/hack/lib.sh"

KUBECONFIGS_DIR="${KUBECONFIGS_DIR:-${ROOT_DIR}/.e2e-kubeconfigs}"
BIN_DIR="${ROOT_DIR}/bin"
IMAGE_TAG="${IMAGE_TAG:-e2e}"
CCM_IMAGE="kubelb-ccm:${IMAGE_TAG}"

# Hash tracking for change detection
HASH_DIR="${KUBECONFIGS_DIR}/.reload-hashes"
mkdir -p "${HASH_DIR}"

# Verify kubeconfig exists
if [[ ! -f "${KUBECONFIGS_DIR}/conversion.kubeconfig" ]]; then
  echodate "Conversion cluster not found, skipping reload"
  exit 0
fi

# Get current binary hash (empty if file doesn't exist)
get_binary_hash() {
  local binary="$1"
  if [[ -f "${BIN_DIR}/${binary}" ]]; then
    sha256sum "${BIN_DIR}/${binary}" | cut -c1-12
  else
    echo ""
  fi
}

# Get stored hash from last reload
get_stored_hash() {
  local key="$1"
  if [[ -f "${HASH_DIR}/${key}.hash" ]]; then
    cat "${HASH_DIR}/${key}.hash"
  else
    echo ""
  fi
}

# Store hash after successful reload
store_hash() {
  local key="$1"
  local hash="$2"
  echo "${hash}" > "${HASH_DIR}/${key}.hash"
}

# Check if existing binary has wrong architecture (e.g., darwin/arm64 instead of linux/amd64)
# If wrong arch, clear stored hash to force rebuild and reload
if [[ -f "${BIN_DIR}/ccm" ]] && ! is_linux_amd64 "${BIN_DIR}/ccm"; then
  echodate "WARNING: ccm binary has wrong architecture, forcing rebuild"
  rm -f "${HASH_DIR}/ccm-conversion.hash"
fi

# Build CCM binary with e2e tags (Go cache makes unchanged builds fast)
# Use fixed BUILD_DATE to ensure reproducible binaries for hash comparison
echodate "Building CCM binary..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -tags e2e \
  -ldflags "-X 'k8c.io/kubelb/internal/version.BuildDate=e2e'" \
  -o "${BIN_DIR}/ccm" "${ROOT_DIR}/cmd/ccm/main.go"

# Check if binary changed
ccm_hash=$(get_binary_hash "ccm")
ccm_stored=$(get_stored_hash "ccm-conversion")

if [[ "${ccm_hash}" == "${ccm_stored}" ]]; then
  echodate "No CCM binary changes detected, skipping reload"
  exit 0
fi

echodate "CCM binary changed (${ccm_stored:-none} -> ${ccm_hash})"

echodate "Building CCM image..."
docker build -q -t "${CCM_IMAGE}" -f "${ROOT_DIR}/ccm.goreleaser.dockerfile" "${BIN_DIR}/"

echodate "Loading CCM image into conversion cluster..."
kind load docker-image --name=conversion "${CCM_IMAGE}"

echodate "Restarting CCM deployment..."
kubectl --kubeconfig="${KUBECONFIGS_DIR}/conversion.kubeconfig" \
  rollout restart deployment/kubelb-ccm -n kubelb
kubectl --kubeconfig="${KUBECONFIGS_DIR}/conversion.kubeconfig" \
  -n kubelb wait --for=condition=available deployment/kubelb-ccm --timeout=2m

store_hash "ccm-conversion" "${ccm_hash}"
echodate "CCM reloaded for conversion cluster"
