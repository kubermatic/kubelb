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

# A chainsaw retry loop whose sleep budget exceeds its step timeout is killed
# mid-wait: the test fails without ever printing the loop's own failure
# message or diagnostics, and slow-but-converging waits die early (this is
# how backend-transport-switch and the Aug 2026 airgap flakes bit us).
#
# Heuristic per script operation (from one `timeout: Ns` line to the next):
# max(seq 1 N) * max(sleep S) must not exceed the timeout. Command overhead
# (kubectl, curl --max-time) is deliberately ignored, so equality passes;
# only loops that cannot finish even with instant commands are flagged.

set -euo pipefail

cd "$(dirname "$0")/.."
source hack/lib.sh

FAILED=0

while IFS= read -r file; do
  violations=$(awk '
    function flush() {
      if (t > 0 && n > 0 && s > 0 && n * s > t) {
        printf "  line %d: timeout=%ds but loop sleeps up to %dx%ds=%ds\n", tline, t, n, s, n * s
      }
      n = 0
      s = 0
    }
    match($0, /timeout:[ ]*[0-9]+s/) {
      flush()
      t = substr($0, RSTART, RLENGTH)
      gsub(/[^0-9]/, "", t)
      t += 0
      tline = NR
      next
    }
    match($0, /seq 1 [0-9]+/) {
      v = substr($0, RSTART + 6, RLENGTH - 6) + 0
      if (v > n) n = v
    }
    match($0, /sleep [0-9]+/) {
      v = substr($0, RSTART + 6, RLENGTH - 6) + 0
      if (v > s) s = v
    }
    END { flush() }
  ' "$file")
  if [[ -n "$violations" ]]; then
    echodate "FAIL: ${file}"
    echo "$violations"
    FAILED=1
  fi
done < <(find test/e2e/tests test/e2e/step-templates -name '*.yaml')

if [[ $FAILED -ne 0 ]]; then
  echodate "Loop budgets above exceed their step timeout. Either raise the"
  echodate "step timeout past the loop's worst case or shrink the loop; see"
  echodate "the pitfalls section in test/e2e/AGENTS.md."
  exit 1
fi

echodate "All e2e retry-loop budgets fit inside their step timeouts."
