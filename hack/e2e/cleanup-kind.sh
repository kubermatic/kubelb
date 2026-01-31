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

export ROOT_DIR="$(git rev-parse --show-toplevel)"
source "${ROOT_DIR}/hack/lib.sh"

KUBECONFIGS_DIR="${KUBECONFIGS_DIR:-${ROOT_DIR}/.e2e-kubeconfigs}"

echodate "Deleting Kind clusters"
kind delete clusters kubelb tenant1 tenant2 standalone || true

echodate "Cleaning up kubeconfigs"
rm -rf "${KUBECONFIGS_DIR}"

echodate "Cleanup complete"
