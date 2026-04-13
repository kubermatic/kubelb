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

cd $(dirname $0)/../..
source hack/lib.sh

export PATH=$PATH:$(go env GOPATH)/bin

EXIT_CODE=0
SUMMARY=

echodate "Installing tools..."

apt-get update -qq && apt-get install -y -qq yamllint > /dev/null 2>&1 &
go install mvdan.cc/sh/v3/cmd/shfmt@v3.13.1 &
go install go.xrstf.de/gimps@v0.6.2 &
go install github.com/kubermatic-labs/boilerplate@v0.3.0 &
go install github.com/frapposelli/wwhrd@v0.4.0 &
wait

echodate "Tools installed."

try() {
  local title="$1"
  shift

  echodate "=== $title ==="
  echo

  start_time=$(date +%s)

  set +e
  "$@"
  exitCode=$?
  set -e

  elapsed_time=$(($(date +%s) - $start_time))
  TEST_NAME="$title" write_junit $exitCode "$elapsed_time"

  local status
  if [[ $exitCode -eq 0 ]]; then
    echo -e "\n[${elapsed_time}s] PASS"
    status=PASS
  else
    echo -e "\n[${elapsed_time}s] FAIL"
    status=FAIL
    EXIT_CODE=1
  fi

  SUMMARY="$SUMMARY\n$(printf "%-40s %s" "$title" "$status")"

  git reset --hard --quiet
  git clean --force

  echo
}

try "Verify go.mod" make check-dependencies
try "Verify YAML" yamllint -c .yamllint.conf .
try "Verify boilerplate" make verify-boilerplate
try "Verify imports" make verify-imports
try "Verify shfmt" shfmt -l -sr -i 2 -d hack
try "Verify licenses" ./hack/verify-licenses.sh

echo
echo "SUMMARY"
echo "======="
echo
echo "Check                                    Result"
echo "-----------------------------------------------"
echo -e "$SUMMARY"

exit $EXIT_CODE
