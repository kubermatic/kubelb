#!/usr/bin/env bash

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