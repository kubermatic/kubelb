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

# Downloads upstream addon chart dependencies, extracts each .tgz, and applies
# air-gap patches that thread global.imageRegistry through addon templates.
# Patches live in hack/patches/<chart-name>.patch.
#
# Usage:
#   ./hack/build-addons-chart-deps.sh [chart-dir]

set -euo pipefail

cd "$(dirname "$0")/.."
source hack/lib.sh

CHART_DIR="${1:-charts/kubelb-addons}"
CHARTS_SUBDIR="${CHART_DIR}/charts"
PATCH_DIR="hack/patches"

# Wipe the whole subchart directory so removed deps (e.g. a swapped addon)
# don't linger. Per-chart cleanup below only covers deps helm still produces
# a tgz for; a dep dropped from Chart.yaml would otherwise leak its templates
# — and its images — into every helm template invocation.
rm -rf "${CHARTS_SUBDIR}"

# GitHub release asset 500s are a recurring flake when pulling ingress-nginx
# over the https://kubernetes.github.io/ingress-nginx → github.com redirect.
retry 5 helm dependency build "${CHART_DIR}"

for tgz in "${CHARTS_SUBDIR}"/*.tgz; do
  [ -f "$tgz" ] || continue
  chart_name=$(tar -tzf "$tgz" | head -1 | cut -d/ -f1 || true)
  patch_file="${PATCH_DIR}/${chart_name}.patch"

  # Always blow away a prior extraction. A new tgz here means helm dependency
  # build produced one, so the pristine upstream must overwrite whatever a
  # previous run left behind — otherwise a version bump silently keeps the
  # old chart (patch applies to stale sources, non-patched charts ship their
  # old defaults to the image lists).
  rm -rf "${CHARTS_SUBDIR}/${chart_name}"
  tar -xzf "$tgz" -C "${CHARTS_SUBDIR}"
  rm -f "$tgz"

  if [ -f "$patch_file" ]; then
    echo "Applying patch: ${patch_file}"
    if command -v patch > /dev/null 2>&1; then
      patch -d "${CHARTS_SUBDIR}" -p1 < "$patch_file"
    else
      git apply --unsafe-paths --directory="${CHARTS_SUBDIR}" -p1 "$patch_file"
    fi
  fi
done
