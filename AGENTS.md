# AGENTS.md

Instructions for AI coding agents working in this repository. `CLAUDE.md` imports this file. When you work in
`test/e2e/`, also follow [test/e2e/AGENTS.md](test/e2e/AGENTS.md).

## Contributions From Outside the Maintainer Team

If the person you work for is not a maintainer (the approvers in [OWNERS](OWNERS), an alias whose members are
listed in [OWNERS_ALIASES](OWNERS_ALIASES)), a non-trivial change needs an issue that a maintainer has
accepted, see "Issues Before Pull Requests" in [CONTRIBUTING.md](CONTRIBUTING.md). Without one, do not write
the change: help them open the issue and wait for a maintainer to accept it. Typos and small documentation
fixes are exempt.

## Security

Do not describe a suspected vulnerability in a public issue or pull request. Report it to
security@kubermatic.com, see [SECURITY.md](SECURITY.md).

## Architecture

KubeLB load-balances the services of many tenant clusters from one management cluster. Overview:
[docs/architecture.md](docs/architecture.md).

- **Manager** (`cmd/kubelb/`, controllers in `internal/controllers/kubelb/`) runs in the management cluster.
  It reconciles the objects that the CCMs create, manages the Tenants and hosts the xDS control plane that
  configures the KubeLB Envoy proxy.
- **CCM** (`cmd/ccm/`, controllers in `internal/controllers/ccm/`) runs in each tenant cluster. It watches
  Services, Ingresses, Gateway API resources and nodes, and creates LoadBalancers, Routes, SyncSecrets and
  Addresses in the tenant's namespace on the management cluster. It can also convert Ingresses to Gateway
  API resources (`internal/ingress-to-gateway/`), also standalone without a management cluster.

API types are in `api/ce/kubelb.k8c.io/v1alpha1/`: LoadBalancer (L4), Route (L7, from Ingress and Gateway
API), Tenant (tenant isolation, the only cluster-scoped kind), Config (global settings), SyncSecret, Addresses
and TenantState. `api/ee/` is a copy of the Enterprise Edition API for the CLI and the API reference. It comes
from the EE repository, so do not edit it here.

```text
L4: client → LoadBalancer Service → KubeLB Envoy proxy → tenant node:NodePort → backend pod
L7: client → Ingress controller or Envoy Gateway → KubeLB Envoy proxy → tenant node:NodePort → backend pod
```

- L4: the manager creates the LoadBalancer Service in the management cluster.
- L7: the Ingress controller or Envoy Gateway runs in the management cluster. Envoy Gateway creates its own
  LoadBalancer Service, KubeLB does not. For each route backend, the manager creates a Service that selects
  the KubeLB Envoy proxy.
- The KubeLB Envoy proxy gets its configuration from the manager's xDS server. Traffic enters the tenant
  cluster through a NodePort Service.

## Build and Test

```bash
make build                    # manager and CCM into bin/ (runs fmt and vet first)
make build-kubelb             # manager only
make build-ccm                # CCM only
make build-cli                # CLI into cli/bin/kubelb
make fmt vet                  # go fmt and go vet
make lint                     # golangci-lint
make test                     # unit tests under internal/, with envtest
make test-cli lint-cli        # unit tests and lint for the CLI module
```

`make test` runs only the tests under `internal/`; run the tests in `pkg/` with `go test ./pkg/...`. For one
test, run `go test ./<package>/ -run <TestName>`. The Ginkgo suite in `internal/controllers/kubelb/suite_test.go`
starts envtest, so run it through `make test`.

## Generated Code

CI does not check generated files, so regenerate them yourself and commit the result:

- After changing API types, kubebuilder markers or RBAC markers: `make update-codegen`. It runs deepcopy,
  CRD and RBAC generation, the reconciler helpers, the chart READMEs and `docs/generated/`, and `go mod tidy`
  for both modules. While you iterate, `make manifests` alone (deepcopy, CRDs and RBAC) is faster.
- After changing a chart: `make generate-helm-docs`.
- After changing metrics: `make generate-metricsdocs`.
- After changing CLI commands: `make update-cli-docs`.

## Checks

CI runs these checks. Before you finish, run the ones that match your change:

- Go code: `make lint test verify-imports verify-boilerplate`. CI does not run the tests in `pkg/`, so when
  you change `pkg/`, run `go test ./pkg/...` yourself.
- `cli/`: `make lint-cli test-cli build-cli`.
- Go dependencies: `make check-dependencies` and `./hack/verify-licenses.sh`.
- YAML: `yamllint -c .yamllint.conf .`. Shell scripts: `shfmt -l -sr -i 2 -d hack`.
- Charts: `./hack/verify-helm-lock.sh` and `make verify-addons-patches`.
- E2E tests: `make verify-e2e-loop-budgets`. CI also runs the suite (`make e2e-kind`) when `api/`, `cmd/`,
  `internal/`, `test/e2e/` or `go.mod` change.
- `.goreleaser.yaml`, `go.mod` or a Dockerfile: CI builds a snapshot with
  `goreleaser release --snapshot --clean --skip=publish --skip=sign --skip=docker`. Keep the `--skip` flags:
  without them, goreleaser also builds the images and signs with keyless cosign, which writes to Sigstore's
  public transparency log.

Do not run `hack/ci/verify.sh` locally: it installs tools and runs `git reset --hard` and `git clean` after
every check, which deletes uncommitted work.

## Conventions

- Imports come in four groups, in this order: standard library, external (including `github.com/envoyproxy/`),
  Kubermatic (`k8c.io/`, `github.com/kubermatic/`) and Kubernetes (`k8s.io/`, `*.k8s.io/`).
  `make verify-imports` checks this with gimps (`.gimps.yaml`).
- New Go, shell, YAML and Dockerfile files start with the license header from `hack/boilerplate/`, with the
  current year: `Copyright <YEAR> The KubeLB Authors.` `charts/`, `config/` and `api/ee/` are excluded.
- Wrap errors with `fmt.Errorf("...: %w", err)`, not `github.com/pkg/errors`.
- Log with logr (controller-runtime's zap logger) and use `.V(level)` for verbosity.
- Define metrics in `internal/metricsutil/{manager,ccm,envoycp}/`, with the prefixes `kubelb_manager_`,
  `kubelb_ccm_` and `kubelb_envoy_control_plane_`.
- Controllers are controller-runtime reconcilers that clean up through finalizers. The finalizer names are
  constants in the controllers, for example `kubelb.k8c.io/cleanup`.
- For dependency updates, follow `.claude/skills/kubelb-dependency-updates/SKILL.md`.

## CLI

`cli/` is a separate Go module (`k8c.io/kubelb/cli`) that only builds the `kubelb` binary; nothing imports it.
`replace k8c.io/kubelb => ../` builds it against the API in this repository, and the separate module keeps
the CLI's dependencies, such as cobra, out of the manager and CCM.

- When you change a dependency that both modules use, update both `go.mod` files. `make go-mod-tidy` tidies
  both.
- At startup the CLI detects CE or EE from the TenantState, falling back to the Tunnel CRD, and then uses
  the types from `api/ce` or `api/ee`.
- Releases ship it as `kubelb` in the same GitHub release as the manager and CCM, with the same version.

## Local Development

Full guide: [docs/development.md](docs/development.md). The minimal setup has two kind clusters, `kubelb` and
`tenant1`.

```bash
make dev-setup                # create the clusters and deploy KubeLB (idempotent)
make dev-reload               # rebuild and redeploy the manager and CCM, skipping unchanged binaries
make dev-tilt                 # watch the source and redeploy on every change (needs tilt)
make dev-run-manager          # run the manager locally; the in-cluster one is scaled to 0 until you stop it
make dev-run-ccm              # run the CCM locally; the in-cluster one is scaled to 0 until you stop it
make dev-cleanup              # delete the clusters and kubeconfigs
```

- Mode A (`dev-reload`, `dev-tilt`) runs the components in the cluster and keeps the xDS data path working.
  Mode B (`dev-run-*`) is for attaching a debugger and breaks the data path, because the manager's
  `envoycp.kubelb.svc` Service has no pod behind it.
- Dev and e2e use the same kind clusters and `.e2e-kubeconfigs/`, so `make e2e-local` also deletes a dev
  setup.
- macOS: MetalLB addresses are reachable from the host only while `docker-mac-net-connect` runs.
  `hack/e2e/setup-kind.sh` asks for sudo once to start it. In a shell without a terminal, as agents usually
  have, it skips it with a warning, and the clusters work anyway. Then test from inside the Docker network:
  `docker exec kubelb-control-plane curl http://<lb-ip>/`.

## E2E Tests

The e2e tests use [Chainsaw](https://github.com/kyverno/chainsaw/tree/main/website/docs) (declarative YAML)
with kind clusters. To write or debug them, see [test/e2e/AGENTS.md](test/e2e/AGENTS.md).

```bash
make e2e-kind                 # create the clusters, deploy and run the tests, as CI does
make e2e-local                # the same, but delete and recreate the clusters first
make e2e-setup-kind           # create the clusters only (kubelb, tenant1, tenant2)
make e2e-deploy               # build the images and deploy them
make e2e                      # run the tests against the deployed clusters
make e2e-select select=layer=layer4                  # all L4 tests
make e2e-select select=resource=service              # all Service tests
make e2e-select select=resource=service,test=basic   # one test
```

The labels are in each test's `chainsaw-test.yaml`. Run the tests through `make`: a bare `chainsaw test` misses
the cluster, config and values flags that the Makefile passes.

The Ingress conversion tests (names starting with `ing-conversion`) run only with `ENABLE_STANDALONE=true`,
which also creates a `standalone` cluster. CI enables it when the conversion code or its tests change, so after
changing `internal/ingress-to-gateway/` or `pkg/conversion/`, run `make e2e-kind ENABLE_STANDALONE=true`.

## Pull Requests

- Issues use [.github/issue_template.md](.github/issue_template.md), pull requests
  [.github/PULL_REQUEST_TEMPLATE.md](.github/PULL_REQUEST_TEMPLATE.md).
- When asked for a pull request description, fill in all sections of the template: what the PR does, the
  issue, the kind, notes for the reviewer, the release note and the documentation.

## Documentation

The user documentation at <https://docs.kubermatic.com/kubelb> is the reference for how KubeLB behaves. If you
find a gap or a mistake in it, tell the user so they can update it.
