# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
make build                    # Build all binaries (kubelb-manager, ccm)
make build-kubelb             # Build manager only
make build-ccm                # Build CCM only
make lint                     # Run golangci-lint
make fmt vet                  # Format and vet
make test                     # Unit tests with envtest
make manifests generate       # Generate CRDs, RBAC, DeepCopy
make update-codegen           # Full code generation pipeline
```

## E2E Testing

Uses Chainsaw framework (declarative YAML-based) with Kind clusters.

```bash
make e2e-kind                 # Full setup: create clusters, deploy, run tests
make e2e-local                # Local setup: create clusters, deploy, run tests
make e2e-setup-kind           # Create Kind clusters only (kubelb, tenant1, tenant2)
make e2e-deploy               # Build images and deploy to clusters
make e2e                      # Run all e2e tests
make e2e-select select=layer=layer4        # L4 tests only
make e2e-select select=resource=service    # Service tests only
chainsaw test --test-file test/e2e/tests/layer4/service/basic/chainsaw-test.yaml  # Single test
```

Test structure:

- `test/e2e/tests/` - Test cases organized by layer/resource type
- `test/e2e/step-templates/` - Reusable test step templates
- `test/e2e/manifests/` - Infrastructure manifests (MetalLB, tenants)

### Chainsaw Documentation

If lookup for documentation fails, use the repository <https://github.com/kyverno/chainsaw/tree/main/website/docs> directly.

### Debugging test failures

When debugging test failures, use the `make e2e-select select=<key>=<value>` command to run a single test or a group of tests that were failing. The labels can be found in the `chainsaw-test.yaml` files in the `test/e2e/tests/` directory.

## Architecture

Three-component hub-and-spoke model:

1. **KubeLB Manager** (`cmd/kubelb/`) - Central management cluster, hosts Envoy xDS control plane, receives LB configs from CCMs
2. **KubeLB CCM** (`cmd/ccm/`) - Cloud Controller Manager in tenant clusters, watches Services/Ingresses/Gateway API, propagates as LoadBalancer CRDs

Key API resources (`api/ce/kubelb.k8c.io/v1alpha1/`):

- **LoadBalancer** - Layer 4 service config
- **Route** - Layer 7 routing (Ingress/Gateway API)
- **Tenant** - Multi-tenant isolation (cluster-scoped)
- **Config** - Global settings

Controllers:

- `internal/controllers/kubelb/` - Manager controllers (route, sync_secret)
- `internal/controllers/ccm/` - CCM controllers (service, ingress, gateway routes, nodes)

## Code Generation

Run `make update-codegen` after:

- Modifying CRD types in `api/`
- Adding new kubebuilder markers
- Changing RBAC requirements in controller comments

## Import Order

Configured in `.gimps.yaml`:

1. Standard library
2. External (`github.com`)
3. Kubermatic (`k8c.io/**`, `github.com/kubermatic/**`)
4. Envoy (`github.com/envoyproxy/**`)
5. Kubernetes (`k8s.io/**`, `*.k8s.io/**`)

## Patterns

- Controller-runtime reconciler pattern with finalizers (`kubelb.k8c.io/lb-finalizer`)
- Prometheus metrics: register in `internal/metrics/`, use subsystem prefixes `kubelb_manager_*`, `kubelb_ccm_*`
- Structured logging via logr/zap with `.V(level)` for verbosity
- Error wrapping with `github.com/pkg/errors`

## Helm Charts

Three charts in `charts/`:

- `kubelb-manager` - Manager deployment
- `kubelb-ccm` - CCM deployment
- `kubelb-addons` - Supporting resources

Run `make generate-helm-docs` after chart changes.

## Documentation

Official documentation for KubeLB is available at <https://docs.kubermatic.com/kubelb>. Please refer to this documentation for the latest information about the project. If and when you find gaps in the documentation, please prompt the user to update the documentation.
