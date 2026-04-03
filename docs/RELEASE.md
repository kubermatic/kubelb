# Release Process

## Overview

Prep-PR-driven pipeline: the release-prep workflow creates a PR with bumped versions, generated docs, and release notes. Merging the PR auto-tags CE + EE and triggers the release pipeline.

```
release-prep workflow -> prep PR (human review) -> merge -> auto-tag CE+EE -> release.yml (GoReleaser + Trivy + Helm) -> docs dispatch
```

Addons have a separate flow triggered by `addons-v*` tags.

## Minor Release (RC -> GA)

### 1. Create release branch + RC

```bash
# Via workflow (creates branch on CE + EE if needed)
# In GitHub Actions UI: run "Release Prep" with create_branch=true
# Or:
gh workflow run release-prep.yml \
  -f version=v1.4.0-rc.1 \
  -f branch=release/v1.4 \
  -f create_branch=true
```

### 2. Review and merge prep PR

The prep PR contains:
- Bumped chart versions
- Regenerated docs (CRD refs, metrics, helm values)
- Release notes in `docs/changelogs/`

Vulnerability scan runs as a separate check on the PR.

### 3. Auto-tag

Merging the prep PR triggers `auto-tag.yml`:
- Tags CE repo with the version
- Tags EE repo's `release/vX.Y` branch HEAD with the same version
- Dispatches docs update to `kubermatic/docs`

### 4. Release pipeline

The CE tag push triggers `release.yml`:
- GoReleaser builds binaries + Docker images
- Trivy vulnerability scan
- SBOM generation + attestation
- Helm chart publish

The EE tag push triggers EE's own `release.yml`.

### 5. Soak period (RC only)

Test the RC. Cherry-pick fixes to `release/v1.4` as needed.

### 6. GA release

```bash
gh workflow run release-prep.yml \
  -f version=v1.4.0 \
  -f branch=release/v1.4
```

Merge the PR. Same auto-tag + release pipeline flow.

## Patch Release

1. Cherry-pick fixes to the release branch (`release/v1.X`)
2. Run prep: `gh workflow run release-prep.yml -f version=v1.X.Y -f branch=release/v1.X`
3. Merge PR — auto-tag and release pipeline trigger automatically

## Cross-repo Dispatches

Triggered by `auto-tag.yml` on prep PR merge:

| Dispatch | Target Repo | Event Type | Payload |
|---|---|---|---|
| EE tag | `kubermatic/kubelb-ee` | git tag push | version tag |
| Docs update | `kubermatic/docs` | `workflow_call` via `docs-update.yml` | version, minor, is_patch |

## Required Secrets

| Secret | Used By | Purpose |
|---|---|---|
| `REGISTRY_USER` | release.yml | Container registry auth |
| `REGISTRY_PASSWORD` | release.yml | Container registry auth |
| `KUBELB_EE_TOKEN` | release-prep.yml, auto-tag.yml | EE repo access (branch validation, clone, tag) |
| `KUBERMATIC_DOCS_TOKEN` | docs-update.yml (via auto-tag) | Clone docs repo, push branch, create PR |
| `GITHUB_TOKEN` | release-prep.yml | PR creation (automatic) |

## Failure Recovery

| Stage | Recovery |
|---|---|
| **Prep workflow** | Fix the issue, re-run the workflow |
| **Build** | Fix the issue, delete the tag (`git push --delete origin <tag>`), re-tag and push |
| **Trivy scan** | Fix vulnerabilities or re-run with `skip_vulnerability_scans: true` (testing only) |
| **Helm push** | Re-run the `helm` job; it's idempotent |
| **EE tag** | Manually tag EE repo: `git tag <version> && git push origin <version>` |
| **Docs dispatch** | Manually trigger `kubelb-docs-update.yml` in `kubermatic/docs` |

For any stage: `release.yml` supports `workflow_dispatch` with `dry_run: true` for testing.

## Scripts (`hack/release/`)

| Script | Purpose |
|---|---|
| `bump-versions.sh` | Bumps Chart.yaml version/appVersion and values.yaml image tag for manager and CCM charts. Supports `--dry-run`. |
| `generate-notes.sh` | Generates combined CE+EE changelog between two tags. Categorizes by `kind/*` labels. Writes to `docs/changelogs/`. |
| `extract-helm-values.sh` | Extracts values tables from helm-docs READMEs for docs site. |
