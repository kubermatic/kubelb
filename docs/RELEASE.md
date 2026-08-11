# Releasing KubeLB

## Pre-release (RC / Alpha / Beta)

Tag and push directly — no prep workflow, no docs update.

```bash
git checkout release/v1.4
git tag v1.4.0-rc.1
git push origin v1.4.0-rc.1
```

This triggers `release.yml` which builds, scans, and publishes images + charts.

## Minor Release

### 1. Run release-prep

Go to **Actions → Release Prep → Run workflow**, or:

```bash
gh workflow run release-prep.yml \
  -f version=v1.4.0 \
  -f branch=release/v1.4
```

If `release/v1.4` doesn't exist on CE or EE, it's created from main automatically.

### 2. Review the prep PR

The workflow creates a PR against the release branch with:

- Bumped chart versions and image tags
- Regenerated CRD docs, helm docs, metrics, and helm values
- Combined CE+EE release notes in `docs/changelogs/`

A vulnerability scan (`release-vuln-scan.yml`) runs as a separate check on the PR.

### 3. Merge the prep PR

On merge, `release-auto-tag.yml` runs automatically:

1. Tags this repo (CE) with the version
2. Tags `kubermatic/kubelb-ee` with the same version
3. Creates a docs update PR on `kubermatic/docs`
4. Comments on the prep PR with links to the CE/EE tags and docs PR

Both CE and EE tag pushes trigger their respective `release.yml` workflows (GoReleaser, Trivy scan, SBOM, Helm charts).

### 4. Review the docs PR

A comment on the prep PR links to the docs PR. Review and merge it.

## Patch Release

Same as minor, without `create_branch` (release branch already exists):

```bash
gh workflow run release-prep.yml \
  -f version=v1.4.1 \
  -f branch=release/v1.4
```

Cherry-pick fixes to `release/v1.4` before running prep.

## Addons Release

The `kubelb-addons` chart versions independently of the manager/ccm and releases
off `main` with an `addons-v*` tag. No prep workflow, no release branch, no docs PR.

Addons are always released from CE. There is no CE/EE version parity for this
chart — a single chart with a single version, published once from CE and
consumed as-is by EE. Never cut an addons release from the EE repo.

### 1. PR to `main`

Bump `KUBELB_ADDONS_CHART_VERSION` in the Makefile, then:

```bash
make bump-addons-chart helm-dependency-update generate-helm-docs
```

`bump-addons-chart` only rewrites `Chart.yaml`; the Makefile variable must be
edited by hand. It is what GoReleaser uses to pull the chart into the airgap
bundle on a full release — a stale value breaks the next `v*` release.

Verify before pushing:

```bash
make verify-addons-patches verify-helm-lock helm-lint
```

### 2. Tag and push

```bash
git tag addons-v0.5.0
git push origin addons-v0.5.0
```

Only the `helm-addons` job in `release.yml` runs: package, push to
`oci://quay.io/kubermatic/helm-charts`, cosign sign. The `release` and `helm`
jobs are skipped.

To release without a tag (or to retry a failed push — the job is idempotent):

```bash
gh workflow run release.yml -f release_type=addons -f addons_version=v0.5.0
```

Dispatch replaces step 2 only — it does not replace step 1. The job re-runs
`bump-addons-chart`, `helm-dependency-update` and `generate-helm-docs` inside the
runner and commits nothing back, so the published chart is versioned correctly
but the repo (Makefile pin, lock file, helm docs) is untouched. It also builds
from whatever is already on the checked-out branch, so chart changes must be
merged first.

## What happens when

| Action | Result |
| --- | --- |
| Push `v*` tag | `release.yml` → GoReleaser + Trivy + SBOM + Helm charts |
| Push `addons-v*` tag | `release.yml` → addons Helm chart only |
| Merge prep PR to `release/v*` | `release-auto-tag.yml` → tag CE + EE, docs PR, comment on prep PR |
| PR from `chore/prepare-v*` to `release/v*` | `release-vuln-scan.yml` → Trivy scan on built images |

## Failure Recovery

| Stage | Fix |
| --- | --- |
| Prep workflow | Fix issue, re-run workflow |
| Build/publish | Delete tag (`git push --delete origin <tag>`), fix, re-tag |
| Trivy scan | Fix vulns or re-run with `skip_vulnerability_scans: true` |
| Helm push | Re-run `helm` job (idempotent) |
| EE tag | Manually: `cd kubelb-ee && git tag <v> && git push origin <v>` |
| Docs PR | Manually run `release-docs-update.yml` via workflow_dispatch |

`release.yml` supports `workflow_dispatch` with `dry_run: true` for testing.

## Required Secrets

| Secret | Workflows | Purpose |
| --- | --- | --- |
| `KUBELB_EE_TOKEN` | release-prep, release-auto-tag | EE repo access (branch create, clone, tag) |
| `KUBERMATIC_DOCS_TOKEN` | release-docs-update | Docs repo access (push branch, create PR) |
| `REGISTRY_USER` / `REGISTRY_PASSWORD` | release | quay.io container registry |
| `GITHUB_TOKEN` | release-prep, release-auto-tag | PR creation, comments (automatic) |

## Scripts (`hack/release/`)

| Script | Purpose |
| --- | --- |
| `bump-versions.sh` | Bumps Chart.yaml version/appVersion and values.yaml image tag. Supports `--dry-run`. |
| `generate-notes.sh` | Generates combined CE+EE changelog between two tags. Writes to `docs/changelogs/`. |
| `extract-helm-values.sh` | Extracts values tables from helm-docs READMEs for docs site. |

## Makefile Targets

```bash
make release-prep VERSION=v1.4.0 BRANCH=release/v1.4  # Trigger release-prep workflow
make release-notes-preview                              # Preview changelog using latest stable tag
make generate-crd-docs-ee                               # Generate EE API reference docs
make bump-addons-chart                                  # Set addons Chart.yaml version/appVersion
make release-addons-chart                               # Package + push addons chart (CI uses this)
```
