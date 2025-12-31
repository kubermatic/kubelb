#!/usr/bin/env bash

# Copyright 2025 The KubeLB Authors.
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

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/../../../
INFRA_DIR="$SCRIPT_ROOT/tests/chainsaw/infra"
E2E_DIR="$SCRIPT_ROOT/tests/chainsaw/e2e"

source "$SCRIPT_ROOT/hack/lib.sh"

setup_infrastructure() {
    echodate "Setting up test infrastructure..."
    cd "$INFRA_DIR"
    bash setup-e2e.sh
}

cleanup_infrastructure() {
    echodate "Cleaning up test infrastructure..."
    if [ -f "$INFRA_DIR/cleanup-e2e.sh" ]; then
        cd "$INFRA_DIR"
        bash cleanup-e2e.sh
    fi
}

run_chainsaw_tests() {
    echodate "Running Chainsaw e2e tests..."
    cd "$E2E_DIR"
    export KUBECONFIG_DIR="${KUBECONFIG_DIR:-/tmp/kubelb-e2e}"
    chainsaw test --config .chainsaw.yaml
}

main() {
    echodate "Starting KubeLB E2E Test Runner"
    setup_infrastructure
    run_chainsaw_tests
    cleanup_infrastructure
    echodate "E2E test run completed successfully!"
}

# Handle signals for cleanup
trap cleanup_infrastructure INT TERM

# Run main function
main "$@"