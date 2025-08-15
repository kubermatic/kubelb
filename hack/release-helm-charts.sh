#!/usr/bin/env bash

# Copyright 2023 The KubeLB Authors.
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
source hack/lib.sh

JOB_NAME=${JOB_NAME:-}
PROW_JOB_ID=${PROW_JOB_ID:-}
CHART_VERSION=${CHART_VERSION:-}
RELEASE_ADDONS_ONLY=${RELEASE_ADDONS_ONLY:-false}

## When running out of CI, it's expected that the user has already configured vault
if [ -n "$JOB_NAME" ] || [ -n "$PROW_JOB_ID" ]; then
  echodate "Getting secrets from Vault"
  retry 5 vault_ci_login
fi

if [ -z "$CHART_VERSION" ]; then
  echo "CHART_VERSION is empty, cannot proceed"
  exit 1
fi

REGISTRY_HOST="${REGISTRY_HOST:-quay.io}"
REPOSITORY_PREFIX="${REPOSITORY_PREFIX:-kubermatic/helm-charts}"

if [ -z "${VAULT_ADDR:-}" ]; then
  export VAULT_ADDR=https://vault.kubermatic.com/
fi

REGISTRY_USER="${REGISTRY_USER:-$(vault kv get -field=username dev/kubermatic-quay.io)}"
REGISTRY_PASSWORD="${REGISTRY_PASSWORD:-$(vault kv get -field=password dev/kubermatic-quay.io)}"

echo ${REGISTRY_PASSWORD} | helm registry login ${REGISTRY_HOST} --username ${REGISTRY_USER} --password-stdin

# Package and publish charts
if [ "$RELEASE_ADDONS_ONLY" = "true" ]; then
  # Release only addons chart
  ADDONS="kubelb-addons"
  CHART_PACKAGE_ADDONS="${ADDONS}-${CHART_VERSION}.tgz"

  echodate "Packaging helm chart ${CHART_PACKAGE_ADDONS}"

  helm dependency build charts/${ADDONS}
  helm package charts/${ADDONS} --version ${CHART_VERSION} --destination ./

  echodate "Publishing helm chart to OCI registry ${REGISTRY_HOST}/${REPOSITORY_PREFIX}"
  helm push ${CHART_PACKAGE_ADDONS} oci://${REGISTRY_HOST}/${REPOSITORY_PREFIX}

  rm ${CHART_PACKAGE_ADDONS}
else
  # Release manager and ccm charts (default behavior)
  MANAGER="kubelb-manager"
  CCM="kubelb-ccm"
  CHART_PACKAGE_MANAGER="${MANAGER}-${CHART_VERSION}.tgz"
  CHART_PACKAGE_CCM="${CCM}-${CHART_VERSION}.tgz"

  echodate "Packaging helm charts ${CHART_PACKAGE_MANAGER} and ${CHART_PACKAGE_CCM}"

  helm dependency build charts/kubelb-manager
  helm dependency build charts/kubelb-ccm

  helm package charts/${MANAGER} --version ${CHART_VERSION} --destination ./
  helm package charts/${CCM} --version ${CHART_VERSION} --destination ./

  echodate "Publishing helm charts to OCI registry ${REGISTRY_HOST}/${REPOSITORY_PREFIX}"
  helm push ${CHART_PACKAGE_MANAGER} oci://${REGISTRY_HOST}/${REPOSITORY_PREFIX}
  helm push ${CHART_PACKAGE_CCM} oci://${REGISTRY_HOST}/${REPOSITORY_PREFIX}

  rm ${CHART_PACKAGE_MANAGER}
  rm ${CHART_PACKAGE_CCM}
fi
helm registry logout ${REGISTRY_HOST}
