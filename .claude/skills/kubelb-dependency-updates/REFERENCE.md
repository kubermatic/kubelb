# Reference

## Discovering a repo's security posture

Do not hardcode scanners. Read them out of CI so this works unchanged in `kubelb-ee` or any sibling repo, and so local runs match what will gate the PR.

```bash
# which scanners run, and where
grep -rn "govulncheck\|trivy\|osv-scanner\|grype\|snyk\|dependency-review\|codeql\|scorecard" \
  .github/ .prow/ hack/ Makefile 2>/dev/null | grep -v Binary

# pinned scanner versions and thresholds
grep -rnE "govulncheck@|trivy .*-b /usr/local/bin|severity|fail-on-severity|ignore-unfixed|exit-code" \
  .github/workflows/ .prow/

# suppression configs
git ls-files | grep -iE "osv|trivy|grype|vuln|\.snyk"
```

Then mirror what you find: same tool, same pinned version, same severity threshold. Three things to extract:

- **What runs where.** Blocking on PRs vs release-only changes how urgent a finding is.
- **Thresholds.** `--severity HIGH,CRITICAL` plus `--ignore-unfixed` means MEDIUM findings and unfixable ones are deliberately out of scope. Do not "fix" what the repo has chosen not to gate on.
- **Suppression file and its convention** (see below).

### kubelb's posture as of writing

| Scanner | Version | Where | Gate |
|---|---|---|---|
| govulncheck | v1.1.4 | `pr.yml`, both modules | blocking on every PR |
| trivy (image + rootfs) | v0.69.2 | `pr.yml`, `release.yml`, `release-vuln-scan.yml` | blocking, `HIGH,CRITICAL`, `--ignore-unfixed` |
| dependency-review-action | v5.0.0 | `pr.yml` | `fail-on-severity: high`, scopes runtime + development |
| CodeQL | — | `codeql-analysis.yml` | SARIF to code scanning |
| OpenSSF Scorecard | — | `scorecard.yml` | SARIF to code scanning |

Note `osv-scanner.toml` exists in the repo root but **nothing in-repo invokes osv-scanner** — it is consumed by an external scanner. Edits to it are therefore untestable locally; do not assume a local run validates them.

## Suppressing a finding

Suppress only when a bump genuinely cannot fix it: no fixed version exists, or the vulnerable package is not reachable from the binaries. Prove the second claim before claiming it:

```bash
go mod why golang.org/x/crypto/openpgp        # run in both modules
```

Record the id and the reasoning in the repo's suppression file. The existing entry is the format to copy:

```toml
[[IgnoredVulns]]
id = "GO-2026-5932"
reason = "golang.org/x/crypto/openpgp is deprecated with no fixed version; kubelb does not import any openpgp package (verified with `go mod why golang.org/x/crypto/openpgp` in both modules), x/crypto is only an indirect dependency"
```

A reason that does not say *why it is unreachable or unfixable* is not a reason.

# Pitfalls

Each of these cost real time. Ordered by how badly they bite.

## Stacked PRs lose commits

`kubermatic-bot` deletes the head branch immediately on merge. Two consequences:

1. **Auto-close.** GitHub closes any PR whose *base* branch is deleted. A stacked PR dies the moment its parent merges, and it cannot be recovered in place: closed PRs cannot be retargeted, and reopening fails because the base is gone. You must rebase the branch onto the squashed `main` and open a fresh PR.
2. **Silent loss.** If a parent and its child merge in the same second, the parent's squash snapshots the branch *before* the child's commit lands. The child reports **Merged** while its content reached nothing. This happened to the agentgateway bump — GitHub said merged, `Chart.yaml` still said `v1.3.1`.

Prefer one flat PR against `main`. If you must stack, verify afterwards by reading the tree:

```bash
git show upstream/<base-branch>:charts/kubelb-addons/Chart.yaml | grep -A2 "name: agentgateway"
```

Never conclude "it landed" from PR status. Audit a whole batch with:

```bash
git diff --name-only <pre-work-sha> upstream/main > /tmp/inmain.txt
git diff --name-only <pre-work-sha> upstream/<open-branch> > /tmp/inpr.txt
# every file touched by the PRs must appear in one of those two
```

## `test -s` tool guards never reinstall

Makefile install targets guard on file existence:

```make
test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install ...@$(CONTROLLER_TOOLS_VERSION)
```

Bumping the version variable does nothing when a binary is already there. `make manifests` runs the **old** tool and produces no diff, so the bump looks complete and is not. Guard on version output instead:

```make
@$(LOCALBIN)/controller-gen --version 2>/dev/null | grep -q "$(CONTROLLER_TOOLS_VERSION)$$" || \
    GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)
```

Version output formats differ: `controller-gen --version` → `Version: v0.21.0` (with `v`); `chainsaw version` → `Version: 0.2.15` (no `v`, use `$(subst v,,...)`). `setup-envtest` has no version flag at all. After changing a guard, `rm -f bin/<tool>` and confirm the reinstall actually happens.

## Version skew between surfaces

The same upstream version is pinned in several places that drift apart. Check these pairs every time:

| Must match | Found via |
|---|---|
| `GATEWAY_API_VERSION` (Makefile) | `sigs.k8s.io/gateway-api` in `go.mod` |
| `gateway-class.yaml` envoy image | `DefaultEnvoyProxyImage` for the pinned envoy-gateway chart |
| `hack/e2e/images*.yaml` envoy | `envoyImage` const in `loadbalancer_controller.go` |
| `hack/e2e/images*.yaml` charts | versions in `charts/kubelb-addons/Chart.yaml` |

Read the envoy-gateway default straight from the module cache rather than guessing:

```bash
grep -n "DefaultEnvoyProxyImage" \
  "$(go env GOMODCACHE)/github.com/envoyproxy/gateway@v1.8.3/api/v1alpha1/shared_types.go"
```

The `gateway-class.yaml` pin only applies when `global.imageRegistry` is set, so a skew here means air-gapped installs silently run a different dataplane version than everyone else. Bumping the envoy-gateway chart *always* means updating this pin too.

## Chart minors delete what patches target

`hack/patches/*.patch` are context diffs against upstream addon charts. A minor bump can make hunks **obsolete** rather than merely drifted — metallb 0.16 removed the kube-rbac-proxy sidecar entirely, invalidating 6 hunks. Blindly re-fuzzing would have reintroduced dead config.

Regenerate:

```bash
helm pull oci://quay.io/metallb/chart/metallb --version <new> -d "$W"
tar -xzf "$W"/metallb-<new>.tgz -C "$W/a" && cp -R "$W/a/metallb" "$W/b/"
cd "$W/b" && patch -p1 --fuzz=3 < hack/patches/metallb.patch   # collect .rej
# decide per reject: obsolete (drop) or drifted (re-apply by hand)
find "$W/b" -name '*.rej' -delete
cd "$W" && diff -ruN --exclude=README.md --exclude=README.md.gotmpl a b > metallb.patch
```

Watch for helpers whose anchor moved — the `metallb.imageRepository` define failed to apply while its *callers* applied fine, which would have shipped a chart calling an undefined template. Then confirm the patch's actual purpose still works:

```bash
helm template t charts/kubelb-addons --set global.imageRegistry=mirror.example.com ... | grep "image:"
```

## Dependabot blind spots

- **docker** ecosystem only scans Dockerfiles at the repo root — `.prow/` images are invisible.
- Action **version inputs** (`with: version: v2.11.4`) are not `uses:` refs and are never proposed.
- **agentgateway** is explicitly ignored: the dead pre-v1.0 lineage semver-sorts above the current 1.x line, so dependabot "upgrades" into an obsolete lineage. Bump manually and check the CRD API versions — 1.4.0 stayed on `v1alpha1` and only added a CRD, so it was safe despite the ignore rule's breaking-minor warning.
- `github/codeql-action` looks perpetually behind against `releases/latest`; it tags releases `codeql-bundle-*`. False positive, leave it.

## Environment gotchas

- A stale gitignored `vendor/` breaks `go list -m -u` with "inconsistent vendoring". Prefix with `GOFLAGS=-mod=mod`.
- A stale `charts/kubelb-addons/charts/` makes `helm-lint` fail with "missing these dependencies" for charts that are not in `Chart.yaml`. Run `make helm-dependency-update` first; do not chase it as a real failure.
- `Chart.lock` digests are **identical** between helm 3 and helm 4, and helm does not rewrite the file when deps are unchanged. So a lock generated locally on helm 4 is safe for CI on helm 3, and `verify-helm-lock` will not thrash on the `generated:` timestamp.
- `azure/setup-helm` has no `version:` input in this repo, so it floats to `latest`. Workflows are therefore on helm 4 regardless of what `hack/ci/verify.sh` pins.

## Known-deferred items

Do not treat these as oversights; they need an explicit decision and an e2e run:

- Managed envoy dataplane (`envoyImage` in `loadbalancer_controller.go`) lags latest envoy by several minors.
- `kindest/node` trails the `k8s.io/*` minor that envtest derives from `k8s.io/api`.
- `grpcbin:latest` floats in two chainsaw tests while the preload lists pin it by digest.
- `test/e2e/values.yaml` is passed via `--values` but has zero `$values.` references — dead config, including a very stale envoy pin.
- Bumping the prow `golangci-lint` image is blocked on cleaning up the goconst/noctx findings it surfaces in the cli module.
