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

# Quick reload script for fast dev iteration
# Rebuilds and redeploys kubelb-manager and CCM only when binaries change
#
# Usage: make e2e-reload

set -euo pipefail

ROOT_DIR="$(git rev-parse --show-toplevel)"
source "${ROOT_DIR}/hack/lib.sh"

KUBECONFIGS_DIR="${KUBECONFIGS_DIR:-${ROOT_DIR}/.e2e-kubeconfigs}"
BIN_DIR="${ROOT_DIR}/bin"
IMAGE_TAG="${IMAGE_TAG:-e2e}"
KUBELB_IMAGE="kubelb:${IMAGE_TAG}"
CCM_IMAGE="kubelb-ccm:${IMAGE_TAG}"

# Hash tracking for change detection
HASH_DIR="${KUBECONFIGS_DIR}/.reload-hashes"
mkdir -p "${HASH_DIR}"

# Verify kubeconfigs exist
for cluster in kubelb tenant1 tenant2; do
  if [[ ! -f "${KUBECONFIGS_DIR}/${cluster}.kubeconfig" ]]; then
    echo "Error: ${KUBECONFIGS_DIR}/${cluster}.kubeconfig not found"
    echo "Run 'make e2e-setup-kind' first"
    exit 1
  fi
done

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

# Build binaries (Go cache makes unchanged builds fast)
# Use fixed BUILD_DATE to ensure reproducible binaries for hash comparison
echodate "Building binaries..."
GOOS=linux GOARCH=amd64 make -C "${ROOT_DIR}" build-kubelb build-ccm BUILD_DATE=e2e -j2

# Check what changed
kubelb_hash=$(get_binary_hash "kubelb")
ccm_hash=$(get_binary_hash "ccm")
kubelb_stored=$(get_stored_hash "kubelb")
ccm_stored=$(get_stored_hash "ccm")

kubelb_changed=false
ccm_changed=false

if [[ "${kubelb_hash}" != "${kubelb_stored}" ]]; then
  kubelb_changed=true
fi
if [[ "${ccm_hash}" != "${ccm_stored}" ]]; then
  ccm_changed=true
fi

if [[ "${kubelb_changed}" == "false" && "${ccm_changed}" == "false" ]]; then
  echodate "No binary changes detected, skipping reload"
  exit 0
fi

# Reload kubelb-manager if changed
if [[ "${kubelb_changed}" == "true" ]]; then
  echodate "kubelb binary changed (${kubelb_stored:-none} -> ${kubelb_hash})"

  echodate "Building kubelb image..."
  docker build -q -t "${KUBELB_IMAGE}" -f "${ROOT_DIR}/kubelb.goreleaser.dockerfile" "${BIN_DIR}/"

  echodate "Loading kubelb image into Kind..."
  kind load docker-image --name=kubelb "${KUBELB_IMAGE}"

  echodate "Restarting kubelb deployment..."
  kubectl --kubeconfig="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
    rollout restart deployment/kubelb -n kubelb

  store_hash "kubelb" "${kubelb_hash}"
  echodate "kubelb-manager reloaded"
fi

# Reload CCM if changed
if [[ "${ccm_changed}" == "true" ]]; then
  echodate "ccm binary changed (${ccm_stored:-none} -> ${ccm_hash})"

  echodate "Building ccm image..."
  docker build -q -t "${CCM_IMAGE}" -f "${ROOT_DIR}/ccm.goreleaser.dockerfile" "${BIN_DIR}/"

  echodate "Loading ccm image into tenant clusters..."
  kind load docker-image --name=tenant1 "${CCM_IMAGE}" &
  kind load docker-image --name=tenant2 "${CCM_IMAGE}" &
  wait

  echodate "Restarting CCM deployments..."
  kubectl --kubeconfig="${KUBECONFIGS_DIR}/tenant1.kubeconfig" \
    rollout restart deployment/kubelb-ccm -n kubelb &
  kubectl --kubeconfig="${KUBECONFIGS_DIR}/tenant2.kubeconfig" \
    rollout restart deployment/kubelb-ccm -n kubelb &
  wait

  store_hash "ccm" "${ccm_hash}"
  echodate "ccm reloaded"
fi

echodate "Reload complete"
