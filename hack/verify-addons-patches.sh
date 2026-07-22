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

# Verifies that every hack/patches/*.patch applies cleanly against the pinned
# addon chart dependencies. Runs the real build script in strict mode so a
# chart bump that drifts a patch (or renames a chart, orphaning its patch)
# fails CI instead of surfacing at release time.

set -euo pipefail

cd $(dirname $0)/..

CHARTS_SUBDIR="charts/kubelb-addons/charts"

# Register the HTTP(S) addon repos; helm dependency build needs them for
# non-OCI deps (ingress-nginx, external-dns). Requires yq.
./hack/ensure-helm-repos.sh

STRICT_PATCH=true ./hack/build-addons-chart-deps.sh

rejects="$(find "${CHARTS_SUBDIR}" -name '*.rej' 2> /dev/null || true)"
if [ -n "$rejects" ]; then
  echo "ERROR: patch rejects found:"
  echo "$rejects"
  exit 1
fi

for patch_file in hack/patches/*.patch; do
  chart_name="$(basename "$patch_file" .patch)"
  if [ ! -d "${CHARTS_SUBDIR}/${chart_name}" ]; then
    echo "ERROR: ${patch_file} matches no addon chart dependency, so it is never applied."
    echo "Rename it to match the chart directory under ${CHARTS_SUBDIR}, or delete it."
    exit 1
  fi
done

echo "All addon chart patches applied cleanly."
