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

cd $(dirname $0)/..

./hack/ensure-helm-repos.sh

EXIT_CODE=0

for chart in charts/kubelb-manager charts/kubelb-ccm charts/kubelb-addons; do
  if [[ -f "$chart/Chart.lock" ]]; then
    echo "Checking $chart..."
    helm dependency update "$chart" > /dev/null 2>&1
    if ! git diff --quiet "$chart/Chart.lock"; then
      echo "ERROR: $chart/Chart.lock is out of sync with Chart.yaml"
      echo "Run 'make helm-dependency-update' and commit the updated lock file."
      git diff "$chart/Chart.lock"
      EXIT_CODE=1
    fi
  fi
done

if [[ $EXIT_CODE -eq 0 ]]; then
  echo "All Helm chart lock files are in sync."
fi

exit $EXIT_CODE
