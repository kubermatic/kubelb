#!/usr/bin/env bash

# Copyright 2024 The KubeLB Authors.
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

TEMPDIR=_tmp
DIFFROOTS=("api" "cmd" "config" "internal")
TMP_DIFFROOT="$TEMPDIR"

cleanup() {
  rm -rf "$TEMPDIR"
}
trap "cleanup" EXIT SIGINT

cleanup

mkdir -p "${TMP_DIFFROOT}"
for dir in "${DIFFROOTS[@]}"; do
  cp -a "${dir}" "${TMP_DIFFROOT}/"
done

# Update generated code
make update-codegen

echodate "Diffing directories against freshly generated codegen"
ret=0
for dir in "${DIFFROOTS[@]}"; do
  echodate "Checking ${dir}..."
  if ! diff -Naupr "${dir}" "${TMP_DIFFROOT}/${dir}"; then
    echodate "${dir} is out of date. Please run hack/update-codegen.sh"
    ret=1
  else
    echodate "${dir} is up to date."
  fi
  cp -a "${TMP_DIFFROOT}/${dir}"/* "${dir}"
done

if [[ $ret -eq 0 ]]; then
  echodate "All directories are up to date."
else
  echodate "One or more directories are out of date. Please run hack/update-codegen.sh"
  exit 1
fi

reconcileHelpers=internal/resources/reconciling/zz_generated_reconcile.go
go run k8c.io/reconciler/cmd/reconciler-gen --config hack/reconciling.yaml > $reconcileHelpers

currentYear=$(date +%Y)
$sed -i "s/Copyright YEAR/Copyright $currentYear/g" $reconcileHelpers
