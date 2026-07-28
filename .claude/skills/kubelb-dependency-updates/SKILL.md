---
name: kubelb-dependency-updates
description: Sweep and update every dependency surface in the kubelb repo (go.mod, Makefile tool versions, prow images, GitHub Actions, hack/ci pins, addon Helm charts, pinned container images) and split the work into PRs. Use when asked to update or bump dependencies, tooling, chart versions, addon versions, or image pins in kubelb/kubelb-ce/kubelb-ee.
---

# KubeLB Dependency Updates

## The two things that go wrong

1. **A surface gets missed.** Dependabot covers less than it looks like. Work the inventory below, do not assume.
2. **A bump silently no-ops or silently gets lost.** Verify by reading the resulting tree, never by trusting a "merged" badge or a clean `make` run.

## Scan for CVEs first

Vulnerabilities decide what to bump and in what order. Run the scanners **before** touching any version, and land security-driven bumps as their own PR ahead of routine ones — a CVE fix should be reviewable and backportable without a pile of cosmetic bumps around it.

Discover what this repo already runs rather than assuming; see [REFERENCE.md](REFERENCE.md#discovering-a-repos-security-posture) for the discovery commands. Reuse its pinned scanner versions and thresholds so local results match CI. For kubelb today that is:

```bash
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
govulncheck ./... && (cd cli && govulncheck ./...)

make build build-cli
docker build -t kubelb-manager:scan -f kubelb.goreleaser.dockerfile .
trivy image --severity HIGH,CRITICAL --ignore-unfixed kubelb-manager:scan
trivy rootfs --severity HIGH,CRITICAL --ignore-unfixed cli/bin/kubelb
```

Then triage: a finding that a bump fixes drives that bump. A finding with no fixed version, or in a package the binaries never import, gets suppressed **with a reason** rather than chased — see [REFERENCE.md](REFERENCE.md#suppressing-a-finding).

## Inventory

Run every row. `auto` = dependabot proposes it weekly; still confirm it is current.

| Surface | Where | Auto? |
|---|---|---|
| Go modules (root + cli) | `go.mod`, `cli/go.mod` | direct only |
| Makefile tools | `CONTROLLER_TOOLS_VERSION`, `CHAINSAW_VERSION`, `KUSTOMIZE_VERSION`, `HELM_DOCS_VERSION`, `CRD_REF_DOCS_VERSION`, `SETUP_ENVTEST_VERSION`, `GO_VERSION`, `GATEWAY_API_VERSION` | no |
| CI tool pins | `hack/ci/verify.sh` (helm, yq, shfmt, gimps, boilerplate, wwhrd) | no |
| Prow images | `.prow/verify.yaml`, `.prow/postsubmits.yaml` | **no** |
| Action SHAs | `.github/workflows/*.yml` `uses:` | yes |
| Action version *inputs* | same files, `with: version:` | **no** |
| Addon charts | `charts/kubelb-addons/Chart.yaml` + `Chart.lock` | yes, except agentgateway |
| agentgateway / -crds | same | **no** (ignored, see `dependabot.yml`) |
| Envoy dataplane pins | `internal/controllers/kubelb/loadbalancer_controller.go`, `charts/kubelb-addons/templates/gateway-class.yaml`, `hack/e2e/images*.yaml` | no |
| Base images | `*.dockerfile` | yes |
| Kind node | `hack/e2e/setup-kind.sh`, `.prow/verify.yaml` | no |

Go deps: direct deps are usually already current. Use `go get -u ./...` in **both** modules to catch indirect ones. Drop the `k8c.io/kubelb` bump it makes in `cli/go.mod` — inert (`replace` points at `../`) and pure noise.

## PR grouping

Split on **user-facing vs internal**, not one-PR-per-bump.

- **Security fixes → first, on their own.** Anything closing a CVE goes ahead of routine bumps so it can be reviewed and backported cleanly.
- **User-facing → its own PR, one per concern.** Addon charts (they deploy into tenant clusters), the managed envoy dataplane image, anything with a real `release-note`, CRD/API changes. Each needs its own e2e signal and its own revert story.
- **Internal → bundle freely into one PR.** Makefile tools, prow images, action SHAs, `hack/ci` pins, go.mod. Nobody downstream sees these.

Keep addon-chart bumps that carry CRD changes (envoy-gateway, agentgateway, metallb minors) separate from each other so a red e2e run stays bisectable.

**Avoid stacked PRs.** See [REFERENCE.md](REFERENCE.md#stacked-prs-lose-commits) — `kubermatic-bot` deletes branches on merge, which auto-closes dependent PRs and can silently drop a merged commit. Prefer one flat PR against `main`.

## Verification gate

```bash
make lint && make test && make build
make verify-helm-lock verify-addons-patches helm-lint   # any chart change
make lint-cli test-cli                                  # any go.mod change
```

Chart changes: commit `Chart.lock` before running `verify-helm-lock` — it compares via `git diff` against HEAD, so an uncommitted lock always reports out-of-sync.

Before bumping the prow `golangci-lint` image, run `make lint` **and** `make lint-cli` with that exact version locally. A newer linter surfaces new findings and turns the `pull-kubelb-lint-cli` job red.

## Pitfalls

Read [REFERENCE.md](REFERENCE.md) before starting. The ones that have actually bitten: silent no-op tool installs, version skew between go.mod and the Makefile/chart pins, chart minors that delete what a `hack/patches/*.patch` targets, and stacked PRs losing commits.
