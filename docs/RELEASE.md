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
  -f branch=release/v1.4 \
  -f create_branch=true
```

Use `create_branch=true` for the first release on a new minor — it creates `release/vX.Y` on both CE and EE.

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
```
