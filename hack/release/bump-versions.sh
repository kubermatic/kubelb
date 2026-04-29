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
# Bump Chart.yaml version/appVersion and values.yaml image tag for kubelb-manager and kubelb-ccm.
# Usage: bump-versions.sh [--dry-run] [--target-dir <dir>] <VERSION>
# VERSION must be semver with v prefix: v1.4.0, v1.4.0-rc.1
#
# --target-dir <dir>: bump charts under <dir>/charts/ instead of the CE workspace.
#   Used by release workflows to bump a freshly-cloned kubelb-ee in /tmp without
#   committing the bump back. Defaults to the CE workspace root.
#
# When --target-dir is given:
#   - The local-tag-exists check is skipped (the target may be a transient clone).
#   - The git-tag check still runs against the CE workspace if invoked from there.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

DRY_RUN=false
TARGET_DIR=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --target-dir)
      TARGET_DIR="${2:?--target-dir requires a value}"
      shift 2
      ;;
    -*)
      echo "ERROR: unknown flag: $1" >&2
      echo "Usage: bump-versions.sh [--dry-run] [--target-dir <dir>] <VERSION>" >&2
      exit 1
      ;;
    *)
      break
      ;;
  esac
done

VERSION="${1:?Usage: bump-versions.sh [--dry-run] [--target-dir <dir>] <VERSION>}"
TARGET_DIR="${TARGET_DIR:-$REPO_ROOT}"

if [[ ! -d "$TARGET_DIR/charts" ]]; then
  echo "ERROR: charts directory not found: $TARGET_DIR/charts" >&2
  exit 1
fi

if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
  echo "ERROR: Invalid version format: $VERSION (expected vX.Y.Z or vX.Y.Z-rc.N)" >&2
  exit 1
fi

# The local tag check only makes sense for the CE workspace. External clones
# (e.g. /tmp/kubelb-ee in CI) are throw-away — re-bumping them is fine.
if [[ "$TARGET_DIR" == "$REPO_ROOT" ]] && git tag -l "$VERSION" | grep -q .; then
  echo "ERROR: Tag $VERSION already exists" >&2
  exit 1
fi

CHARTS=("kubelb-manager" "kubelb-ccm")
SED_CMD="sed"
if [[ "$(uname)" == "Darwin" ]]; then
  SED_CMD="gsed"
  if ! command -v gsed &> /dev/null; then
    echo "ERROR: GNU sed (gsed) required on macOS. Install via: brew install gnu-sed" >&2
    exit 1
  fi
fi

for chart in "${CHARTS[@]}"; do
  CHART_YAML="${TARGET_DIR}/charts/${chart}/Chart.yaml"
  VALUES_YAML="${TARGET_DIR}/charts/${chart}/values.yaml"

  if [[ ! -f "$CHART_YAML" ]]; then
    echo "  skip ${chart}: ${CHART_YAML} not found"
    continue
  fi

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "=== ${chart}/Chart.yaml ==="
    diff <($SED_CMD -e "s/^version:.*/version: ${VERSION}/" -e "s/^appVersion:.*/appVersion: ${VERSION}/" "$CHART_YAML") "$CHART_YAML" || true
    echo "=== ${chart}/values.yaml ==="
    diff <($SED_CMD "0,/^  tag:/{s/^  tag:.*/  tag: ${VERSION}/}" "$VALUES_YAML") "$VALUES_YAML" || true
  else
    $SED_CMD -i "s/^version:.*/version: ${VERSION}/" "$CHART_YAML"
    $SED_CMD -i "s/^appVersion:.*/appVersion: ${VERSION}/" "$CHART_YAML"
    $SED_CMD -i "0,/^  tag:/{s/^  tag:.*/  tag: ${VERSION}/}" "$VALUES_YAML"
  fi
done

echo "Bumped charts in ${TARGET_DIR} to ${VERSION}"
