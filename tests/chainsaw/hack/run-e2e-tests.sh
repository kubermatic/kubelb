#!/bin/bash

# KubeLB E2E Test Runner
# This script sets up the test infrastructure and runs Chainsaw e2e tests

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
INFRA_DIR="$PROJECT_ROOT/tests/chainsaw/infra"
E2E_DIR="$PROJECT_ROOT/tests/chainsaw/e2e"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
SKIP_SETUP=false
SKIP_CLEANUP=false
PARALLEL_TESTS=1
SPECIFIC_TEST=""
VERBOSE=false

show_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

KubeLB E2E Test Runner

This script sets up the test infrastructure using Kind clusters and MetalLB,
then runs the complete Chainsaw e2e test suite for KubeLB.

OPTIONS:
    -h, --help           Show this help message
    -s, --skip-setup     Skip infrastructure setup (clusters must already exist)
    -c, --skip-cleanup   Skip infrastructure cleanup after tests
    -p, --parallel N     Run N tests in parallel (default: 1)
    -t, --test NAME      Run specific test only (e.g., simple-service)
    -v, --verbose        Enable verbose output
    --setup-only         Only setup infrastructure, don't run tests
    --cleanup-only       Only cleanup infrastructure, don't run tests

EXAMPLES:
    $0                              # Full e2e test run (setup + tests + cleanup)
    $0 --skip-setup                 # Run tests on existing infrastructure
    $0 --test simple-service        # Run only the simple-service test
    $0 --parallel 3                 # Run up to 3 tests in parallel
    $0 --setup-only                 # Only setup test infrastructure
    $0 --cleanup-only               # Only cleanup test infrastructure

PREREQUISITES:
    - Docker running
    - kubectl installed
    - Helm installed
    - Go toolchain (for building images)
    - Internet connectivity (for pulling images)

EOF
}

log() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

fatal() {
    error "$*"
    exit 1
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
            ;;
        -s|--skip-setup)
            SKIP_SETUP=true
            shift
            ;;
        -c|--skip-cleanup)
            SKIP_CLEANUP=true
            shift
            ;;
        -p|--parallel)
            PARALLEL_TESTS="$2"
            shift 2
            ;;
        -t|--test)
            SPECIFIC_TEST="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        --setup-only)
            SKIP_CLEANUP=true
            SPECIFIC_TEST="__setup_only__"
            shift
            ;;
        --cleanup-only)
            SKIP_SETUP=true
            SPECIFIC_TEST="__cleanup_only__"
            shift
            ;;
        *)
            fatal "Unknown option: $1"
            ;;
    esac
done

# Validation
if ! command -v docker &> /dev/null; then
    fatal "Docker is required but not installed"
fi

if ! command -v kubectl &> /dev/null; then
    fatal "kubectl is required but not installed"
fi

if ! command -v helm &> /dev/null; then
    fatal "helm is required but not installed"
fi

if ! docker info &> /dev/null; then
    fatal "Docker daemon is not running"
fi

# Validate parallel tests value
if ! [[ "$PARALLEL_TESTS" =~ ^[0-9]+$ ]] || [ "$PARALLEL_TESTS" -lt 1 ] || [ "$PARALLEL_TESTS" -gt 10 ]; then
    fatal "Parallel tests must be a number between 1 and 10"
fi

setup_infrastructure() {
    log "Setting up test infrastructure..."
    
    if [ ! -f "$INFRA_DIR/setup-e2e.sh" ]; then
        fatal "Infrastructure setup script not found at $INFRA_DIR/setup-e2e.sh"
    fi
    
    cd "$INFRA_DIR"
    if [ "$VERBOSE" = true ]; then
        bash setup-e2e.sh
    else
        bash setup-e2e.sh > /dev/null 2>&1
    fi
    
    if [ $? -eq 0 ]; then
        log "Infrastructure setup completed successfully"
    else
        fatal "Infrastructure setup failed"
    fi
}

cleanup_infrastructure() {
    log "Cleaning up test infrastructure..."
    
    if [ ! -f "$INFRA_DIR/cleanup-e2e.sh" ]; then
        warn "Infrastructure cleanup script not found at $INFRA_DIR/cleanup-e2e.sh"
        return
    fi
    
    cd "$INFRA_DIR"
    if [ "$VERBOSE" = true ]; then
        bash cleanup-e2e.sh
    else
        bash cleanup-e2e.sh > /dev/null 2>&1
    fi
    
    if [ $? -eq 0 ]; then
        log "Infrastructure cleanup completed successfully"
    else
        warn "Infrastructure cleanup completed with warnings"
    fi
}

wait_for_infrastructure() {
    log "Waiting for infrastructure to be ready..."
    
    # Wait for kubeconfigs to be available
    local kubeconfig_dir="${KUBECONFIG_DIR:-/tmp/kubelb-e2e}"
    local max_wait=300 # 5 minutes
    local wait_time=0
    
    while [ $wait_time -lt $max_wait ]; do
        if [ -f "$kubeconfig_dir/kubelb.kubeconfig" ] && \
           [ -f "$kubeconfig_dir/vyse.kubeconfig" ] && \
           [ -f "$kubeconfig_dir/omen.kubeconfig" ]; then
            break
        fi
        sleep 5
        wait_time=$((wait_time + 5))
    done
    
    if [ $wait_time -ge $max_wait ]; then
        fatal "Timeout waiting for kubeconfigs to be ready"
    fi
    
    # Test connectivity to clusters
    export KUBECONFIG="$kubeconfig_dir/kubelb.kubeconfig"
    if ! kubectl get nodes &> /dev/null; then
        fatal "Cannot connect to management cluster"
    fi
    
    export KUBECONFIG="$kubeconfig_dir/vyse.kubeconfig"
    if ! kubectl get nodes &> /dev/null; then
        fatal "Cannot connect to vyse cluster"
    fi
    
    export KUBECONFIG="$kubeconfig_dir/omen.kubeconfig"
    if ! kubectl get nodes &> /dev/null; then
        fatal "Cannot connect to omen cluster"
    fi
    
    # Wait for KubeLB components to be ready
    export KUBECONFIG="$kubeconfig_dir/kubelb.kubeconfig"
    log "Waiting for KubeLB manager to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment/kubelb-manager -n kubelb
    
    log "Infrastructure is ready for testing"
}

run_chainsaw_tests() {
    log "Running Chainsaw e2e tests..."
    
    cd "$E2E_DIR"
    
    # Set environment variables for Chainsaw
    export KUBECONFIG_DIR="${KUBECONFIG_DIR:-/tmp/kubelb-e2e}"
    
    local chainsaw_args=(
        "--config" ".chainsaw.yaml"
        "--report-format" "JSON"
        "--report-name" "kubelb-e2e-report"
    )
    
    if [ "$PARALLEL_TESTS" -gt 1 ]; then
        chainsaw_args+=("--parallel" "$PARALLEL_TESTS")
    fi
    
    if [ "$VERBOSE" = true ]; then
        chainsaw_args+=("--verbose")
    fi
    
    # Run specific test or all tests
    if [ -n "$SPECIFIC_TEST" ] && [ "$SPECIFIC_TEST" != "__setup_only__" ] && [ "$SPECIFIC_TEST" != "__cleanup_only__" ]; then
        if [ -d "$SPECIFIC_TEST" ]; then
            log "Running specific test: $SPECIFIC_TEST"
            chainsaw_args+=("$SPECIFIC_TEST")
        else
            fatal "Test directory '$SPECIFIC_TEST' not found in $E2E_DIR"
        fi
    else
        log "Running all e2e tests (parallel: $PARALLEL_TESTS)"
    fi
    
    # Check if chainsaw is installed
    if ! command -v chainsaw &> /dev/null; then
        fatal "Chainsaw is not installed. Please install it from https://kyverno.github.io/chainsaw/"
    fi
    
    # Run the tests
    if chainsaw test "${chainsaw_args[@]}"; then
        log "All tests passed successfully!"
        return 0
    else
        error "Some tests failed"
        return 1
    fi
}

show_test_results() {
    local report_file="$E2E_DIR/kubelb-e2e-report.json"
    if [ -f "$report_file" ]; then
        log "Test results available in: $report_file"
        
        # Basic summary from JSON report (if jq is available)
        if command -v jq &> /dev/null; then
            local total_tests=$(jq '.tests | length' "$report_file" 2>/dev/null || echo "N/A")
            local passed_tests=$(jq '[.tests[] | select(.outcome == "passed")] | length' "$report_file" 2>/dev/null || echo "N/A")
            local failed_tests=$(jq '[.tests[] | select(.outcome == "failed")] | length' "$report_file" 2>/dev/null || echo "N/A")
            
            echo
            log "Test Summary:"
            log "  Total Tests: $total_tests"
            log "  Passed: $passed_tests"
            log "  Failed: $failed_tests"
        fi
    fi
}

main() {
    log "Starting KubeLB E2E Test Runner"
    log "Project root: $PROJECT_ROOT"
    log "Test configuration: parallel=$PARALLEL_TESTS, specific_test='$SPECIFIC_TEST'"
    
    # Handle special cases
    if [ "$SPECIFIC_TEST" = "__cleanup_only__" ]; then
        cleanup_infrastructure
        exit 0
    fi
    
    local test_exit_code=0
    
    # Setup infrastructure
    if [ "$SKIP_SETUP" = false ]; then
        setup_infrastructure
        wait_for_infrastructure
    else
        log "Skipping infrastructure setup"
        wait_for_infrastructure
    fi
    
    # Run tests (unless setup-only)
    if [ "$SPECIFIC_TEST" != "__setup_only__" ]; then
        if run_chainsaw_tests; then
            log "All tests completed successfully"
        else
            error "Test execution failed"
            test_exit_code=1
        fi
        
        show_test_results
    else
        log "Setup-only mode: infrastructure ready for manual testing"
        log "Use 'chainsaw test --config tests/chainsaw/e2e/.chainsaw.yaml' to run tests"
        log "Use '$0 --cleanup-only' to cleanup when done"
    fi
    
    # Cleanup infrastructure
    if [ "$SKIP_CLEANUP" = false ]; then
        cleanup_infrastructure
    else
        log "Skipping infrastructure cleanup"
        if [ "$SPECIFIC_TEST" != "__setup_only__" ]; then
            log "Use '$0 --cleanup-only' to cleanup infrastructure when ready"
        fi
    fi
    
    if [ $test_exit_code -eq 0 ]; then
        log "E2E test run completed successfully!"
    else
        error "E2E test run completed with failures"
    fi
    
    exit $test_exit_code
}

# Handle signals for cleanup
trap cleanup_infrastructure INT TERM

# Run main function
main "$@"