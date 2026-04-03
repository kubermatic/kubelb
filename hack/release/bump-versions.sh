#!/usr/bin/env bash
# Bump Chart.yaml version/appVersion and values.yaml image tag for kubelb-manager and kubelb-ccm.
# Usage: bump-versions.sh [--dry-run] <VERSION>
# VERSION must be semver with v prefix: v1.4.0, v1.4.0-rc.1
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

DRY_RUN=false
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=true
  shift
fi

VERSION="${1:?Usage: bump-versions.sh [--dry-run] <VERSION>}"

if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
  echo "ERROR: Invalid version format: $VERSION (expected vX.Y.Z or vX.Y.Z-rc.N)" >&2
  exit 1
fi

if git tag -l "$VERSION" | grep -q .; then
  echo "ERROR: Tag $VERSION already exists" >&2
  exit 1
fi

CHARTS=("kubelb-manager" "kubelb-ccm")
SED_CMD="sed"
if [[ "$(uname)" == "Darwin" ]]; then
  SED_CMD="gsed"
  if ! command -v gsed &>/dev/null; then
    echo "ERROR: GNU sed (gsed) required on macOS. Install via: brew install gnu-sed" >&2
    exit 1
  fi
fi

for chart in "${CHARTS[@]}"; do
  CHART_YAML="${REPO_ROOT}/charts/${chart}/Chart.yaml"
  VALUES_YAML="${REPO_ROOT}/charts/${chart}/values.yaml"

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

echo "Bumped charts to ${VERSION}"
