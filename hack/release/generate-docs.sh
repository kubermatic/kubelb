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

# Regenerate everything under docs/generated/ from the current CE checkout, and
# (optionally) the EE counterparts.
#
# CE artifacts (always):
#   docs/generated/api-reference.md
#   docs/generated/api-reference-ee.md
#   docs/generated/metrics.md
#   docs/generated/helm-values-kubelb-manager.md
#   docs/generated/helm-values-kubelb-ccm.md
#
# EE artifacts (only when KUBELB_EE_PATH or KUBELB_EE_TOKEN is set):
#   docs/generated/metrics-ee.md
#   docs/generated/helm-values-kubelb-manager-ee.md
#   docs/generated/helm-values-kubelb-ccm-ee.md
#
# Flags:
#   --ee-only         Skip CE generation (useful when invoked from make where CE
#                     targets already ran as dependencies).
#
# Env:
#   KUBELB_EE_PATH    Path to a local kubelb-ee clone. Used as-is (no clone, no bump).
#   KUBELB_EE_TOKEN   Token used to clone kubelb-ee when KUBELB_EE_PATH is unset.
#   KUBELB_EE_REPO    EE repo slug (default: kubermatic/kubelb-ee).
#   KUBELB_EE_BRANCH  Branch to clone (default: main). Ignored when KUBELB_EE_PATH set.
set -euo pipefail

EE_ONLY=false
if [[ "${1:-}" == "--ee-only" ]]; then
  EE_ONLY=true
fi

WORKSPACE="$(git rev-parse --show-toplevel)"
cd "$WORKSPACE"

OUT="${WORKSPACE}/docs/generated"
mkdir -p "$OUT"

if [[ "$EE_ONLY" != "true" ]]; then
  echo "==> Generating CE docs"
  make generate-helm-docs generate-metricsdocs generate-crd-docs generate-crd-docs-ee
  "${WORKSPACE}/hack/release/extract-helm-values.sh" ./charts "$OUT"
fi

EE_PATH="${KUBELB_EE_PATH:-}"
EE_TMP=""
cleanup() {
  if [[ -n "$EE_TMP" && -d "$EE_TMP" ]]; then
    rm -rf "$EE_TMP"
  fi
}
trap cleanup EXIT

if [[ -z "$EE_PATH" && -n "${KUBELB_EE_TOKEN:-}" ]]; then
  EE_REPO="${KUBELB_EE_REPO:-kubermatic/kubelb-ee}"
  EE_BRANCH="${KUBELB_EE_BRANCH:-main}"
  EE_TMP="$(mktemp -d -t kubelb-ee.XXXXXX)"
  echo "==> Cloning ${EE_REPO}@${EE_BRANCH} to ${EE_TMP}"
  git clone --depth 1 --branch "$EE_BRANCH" \
    "https://x-access-token:${KUBELB_EE_TOKEN}@github.com/${EE_REPO}.git" "$EE_TMP"
  EE_PATH="$EE_TMP"
fi

if [[ -z "$EE_PATH" ]]; then
  echo "==> Skipping EE docs (set KUBELB_EE_PATH or KUBELB_EE_TOKEN to enable)"
  exit 0
fi

if [[ ! -d "$EE_PATH" ]]; then
  echo "ERROR: KUBELB_EE_PATH=${EE_PATH} is not a directory" >&2
  exit 1
fi

echo "==> Generating EE docs from ${EE_PATH}"
(
  cd "$EE_PATH"
  go mod download
  make generate-metricsdocs || echo "::warning::EE generate-metricsdocs failed"
  make generate-helm-docs || echo "::warning::EE generate-helm-docs failed"
)

# EE Makefile vintage may emit metrics to either docs/generated/metrics.md or docs/metrics.md.
metrics_src=""
for cand in "${EE_PATH}/docs/generated/metrics.md" "${EE_PATH}/docs/metrics.md"; do
  if [[ -f "$cand" ]]; then
    metrics_src="$cand"
    break
  fi
done
if [[ -n "$metrics_src" ]]; then
  cp "$metrics_src" "${OUT}/metrics-ee.md"
  echo "Wrote ${OUT}/metrics-ee.md"
else
  echo "::warning::EE metrics output file not found (looked in docs/generated/metrics.md, docs/metrics.md)"
fi

"${WORKSPACE}/hack/release/extract-helm-values.sh" "${EE_PATH}/charts" "$OUT" "-ee"
