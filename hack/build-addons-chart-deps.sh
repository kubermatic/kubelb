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

# GitHub release asset 500s are a recurring flake when pulling ingress-nginx
# over the https://kubernetes.github.io/ingress-nginx → github.com redirect.
retry 5 helm dependency build "${CHART_DIR}"

for tgz in "${CHARTS_SUBDIR}"/*.tgz; do
  [ -f "$tgz" ] || continue
  chart_name=$(tar -tzf "$tgz" | head -1 | cut -d/ -f1 || true)
  patch_file="${PATCH_DIR}/${chart_name}.patch"

  # If a patch exists, always re-extract so it applies fresh against the
  # pristine upstream chart. Without this, a stale dir from a prior run could
  # silently remain unpatched.
  if [ -d "${CHARTS_SUBDIR}/${chart_name}" ]; then
    if [ -f "$patch_file" ]; then
      rm -rf "${CHARTS_SUBDIR}/${chart_name}"
    else
      rm -f "$tgz"
      continue
    fi
  fi

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
