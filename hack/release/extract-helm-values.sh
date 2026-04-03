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

#!/usr/bin/env bash
# Extract the values table from helm-docs generated README.md files.
# Saves standalone markdown files suitable for render_external_markdown.
#
# Usage: extract-helm-values.sh <charts-dir> <output-dir> [suffix]
#   charts-dir: path to charts/ directory (e.g. ./charts or /tmp/kubelb-ee/charts)
#   output-dir: where to write extracted tables (e.g. ./docs)
#   suffix: optional suffix for output files (e.g. "-ee" → helm-values-manager-ee.md)
set -euo pipefail

CHARTS_DIR="${1:?Usage: extract-helm-values.sh <charts-dir> <output-dir> [suffix]}"
OUTPUT_DIR="${2:?Usage: extract-helm-values.sh <charts-dir> <output-dir> [suffix]}"
SUFFIX="${3:-}"

mkdir -p "$OUTPUT_DIR"

for chart in kubelb-manager kubelb-ccm; do
  README="${CHARTS_DIR}/${chart}/README.md"
  if [[ ! -f "$README" ]]; then
    echo "WARNING: ${README} not found, skipping" >&2
    continue
  fi

  OUTPUT="${OUTPUT_DIR}/helm-values-${chart}${SUFFIX}.md"
  sed -n '/^## Values$/,/^## /{/^## Values$/d;/^## /d;p;}' "$README" > "$OUTPUT"

  # Remove trailing blank lines
  sed -i -e :a -e '/^\n*$/{$d;N;ba' -e '}' "$OUTPUT" 2> /dev/null || true

  echo "Extracted: ${OUTPUT}"
done
