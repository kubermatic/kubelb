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

# Source the common library functions
source "${ROOT_DIR}/hack/lib.sh"

# Configuration
export TMPDIR="${TMPDIR:-/tmp}"

echodate "=== Starting E2E Infrastructure Cleanup ==="

# Delete Kind clusters
for cluster in kubelb vyse omen; do
  if kind get clusters 2>/dev/null | grep -q "^${cluster}$"; then
    echodate "Deleting ${cluster} cluster..."
    kind delete cluster --name "${cluster}"
  else
    echodate "${cluster} cluster not found, skipping..."
  fi
done

# Clean up kubeconfig files
echodate "Cleaning up kubeconfig files..."
for config in kubelb vyse omen; do
  kubeconfig_file="${TMPDIR}/${config}.kubeconfig"
  if [[ -f "${kubeconfig_file}" ]]; then
    echodate "Removing ${kubeconfig_file}..."
    rm -f "${kubeconfig_file}"
  else
    echodate "${kubeconfig_file} not found, skipping..."
  fi
done

# Clean up temporary files
temp_files=("${TMPDIR}/kubelb-vyse.kubeconfig" "${TMPDIR}/kubelb-omen.kubeconfig")
for temp_file in "${temp_files[@]}"; do
  if [[ -f "${temp_file}" ]]; then
    echodate "Removing temporary file ${temp_file}..."
    rm -f "${temp_file}"
  fi
done

# Clean up any temporary directories created by mktemp
if [[ -d "${TMPDIR}" && "${TMPDIR}" != "/tmp" && "${TMPDIR}" =~ ^/tmp/tmp\. ]]; then
  echodate "Removing temporary directory ${TMPDIR}..."
  rm -rf "${TMPDIR}"
fi

echodate "=== Cleanup complete ==="