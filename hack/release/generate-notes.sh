#!/usr/bin/env bash
# Copyright 2026 The KubeLB Authors.
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

#!/usr/bin/env bash
# Generate combined CE+EE release notes using the k8s release-notes tool.
# Usage: generate-notes.sh <VERSION> <PREV_TAG> [--ee-repo <owner/repo> --ee-path <path>] [CE_REPO]
#
# Requires: k8s.io/release/cmd/release-notes (go install k8s.io/release/cmd/release-notes@latest)
# Requires: GITHUB_TOKEN env var set
#
# Behavior:
# - Generates CE notes with PR links (public repo)
# - Generates EE notes without PR links (private repo)
# - Deduplicates: if all EE notes appear in CE, shows unified output (no EE section)
# - If EE has unique notes, shows separate CE and EE sections
# - Appends release artifacts template (always split CE/EE)
# - Writes to docs/changelogs/CHANGELOG-v{MINOR}.md
set -euo pipefail

VERSION="${1:?Usage: generate-notes.sh <VERSION> <PREV_TAG> [--ee-repo <owner/repo> --ee-path <path>] [CE_REPO]}"
shift
PREV_TAG="${1:?Usage: generate-notes.sh <VERSION> <PREV_TAG> [--ee-repo <owner/repo> --ee-path <path>] [CE_REPO]}"
shift

EE_REPO=""
EE_PATH=""
CE_REPO="kubermatic/kubelb"
CE_BRANCH="main"
EE_BRANCH="main"

while [[ $# -gt 0 ]]; do
  case "$1" in
  --ee-repo)
    EE_REPO="${2:?--ee-repo requires a value}"
    shift 2
    ;;
  --ee-path)
    EE_PATH="${2:?--ee-path requires a value}"
    shift 2
    ;;
  --branch)
    CE_BRANCH="${2:?--branch requires a value}"
    shift 2
    ;;
  --ee-branch)
    EE_BRANCH="${2:?--ee-branch requires a value}"
    shift 2
    ;;
  *)
    CE_REPO="$1"
    shift
    ;;
  esac
done

if [[ -n "$EE_REPO" && -z "$EE_PATH" ]] || [[ -z "$EE_REPO" && -n "$EE_PATH" ]]; then
  echo "Error: --ee-repo and --ee-path must be provided together" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MINOR=$(echo "$VERSION" | grep -oE 'v[0-9]+\.[0-9]+')

CE_ORG="${CE_REPO%%/*}"
CE_NAME="${CE_REPO##*/}"

strip_author() {
  sed -E 's/, \[@[a-zA-Z0-9_-]+\]\(https:\/\/github\.com\/[^)]+\)//g; s/, @[a-zA-Z0-9_-]+\)/)/'
}

strip_pr_links() {
  sed -E 's/\[#([0-9]+)\]\([^)]+\)/#\1/g'
}

strip_pr_refs() {
  sed -E 's/ \(#[0-9]+\)//g; s/ \(\[#[0-9]+\]\([^)]+\)\)//g'
}

strip_wrapper() {
  sed '/^- \[Changes by Kind\]/,/^$/d; /^## Changes by Kind$/d'
}

prefix_ee_headings() {
  sed -E 's/^(#{3,}) /\1 EE /'
}

find_upstream_head() {
  local org="$1" repo="$2" branch="$3"
  # Get the upstream repo's branch HEAD SHA via API. Using a branch-specific
  # endpoint is required for release branches — walking main would include
  # commits never backported to the release branch.
  local upstream_sha
  upstream_sha=$(gh api "repos/${org}/${repo}/commits/${branch}" --jq '.sha' 2> /dev/null || echo "")
  if [[ -n "$upstream_sha" ]]; then
    echo "$upstream_sha"
    return
  fi
  echo "HEAD"
}

run_release_notes() {
  local org="$1" repo="$2" repo_path="$3" outfile="$4" branch="$5"
  local end_sha start_sha log_file rp_flag start_flag
  end_sha=$(find_upstream_head "$org" "$repo" "$branch")
  # Resolve PREV_TAG against the upstream repo via the GitHub API rather than
  # the local clone. release-notes' internal MergeBase walk is otherwise
  # vulnerable to local history rewrites (e.g. an `origin` pointing at a stale
  # fork, followed by `git pull --rebase`): first-parent ancestry no longer
  # reaches the tag commit and the walk silently falls back to a much deeper
  # ancestor, polluting the notes with pre-PREV_TAG PRs.
  start_sha=$(gh api "repos/${org}/${repo}/commits/${PREV_TAG}" --jq '.sha' 2> /dev/null || echo "")
  if [[ -n "$start_sha" ]]; then
    start_flag="--start-sha $start_sha"
    echo "  Using branch: ${branch}, start SHA: ${start_sha:0:12} (${PREV_TAG}), end SHA: ${end_sha:0:12}" >&2
  else
    start_flag="--start-rev $PREV_TAG"
    echo "  WARNING: could not resolve ${PREV_TAG} via gh api; falling back to --start-rev" >&2
    echo "  Using branch: ${branch}, end SHA: ${end_sha:0:12}" >&2
  fi

  log_file=$(mktemp)
  rp_flag=""
  # Only use --repo-path for external clones (EE), never for the workspace.
  # Using --repo-path on the workspace causes the tool to run `git pull --rebase`
  # which can fail (no tracking) and `git stash` which destroys uncommitted prep changes.
  # Additionally, if the external clone's `origin` does not point at the
  # upstream repo (e.g. local dev using a fork), the tool's internal
  # `git pull --rebase` would rebase onto the fork and corrupt the history the
  # MergeBase walk depends on — silently polluting notes with commits from
  # far before PREV_TAG. In that case, fall back to a fresh clone of the
  # upstream into a temp dir and use that as --repo-path. This mirrors what
  # the release-prep GitHub Actions workflow does.
  if [[ "$repo_path" != "$REPO_ROOT" && -d "${repo_path}/.git" ]]; then
    local origin_url
    origin_url=$(git -C "$repo_path" remote get-url origin 2> /dev/null || echo "")
    if [[ "$origin_url" == *"${org}/${repo}"* ]]; then
      git -C "$repo_path" stash --quiet 2> /dev/null || true
      local current_branch
      current_branch=$(git -C "$repo_path" rev-parse --abbrev-ref HEAD 2> /dev/null || echo "")
      if [[ -n "$current_branch" && "$current_branch" != "HEAD" ]]; then
        git -C "$repo_path" branch --set-upstream-to="origin/${current_branch}" "$current_branch" 2> /dev/null || true
      fi
      rp_flag="--repo-path ${repo_path}"
    else
      local fresh_clone clone_url
      fresh_clone=$(mktemp -d -t "release-notes-${repo}.XXXXXX")
      if [[ -n "${GH_TOKEN:-${GITHUB_TOKEN:-}}" ]]; then
        clone_url="https://x-access-token:${GH_TOKEN:-$GITHUB_TOKEN}@github.com/${org}/${repo}.git"
      else
        clone_url="https://github.com/${org}/${repo}.git"
      fi
      echo "  NOTICE: ${repo_path} origin='${origin_url}' does not match ${org}/${repo}; cloning upstream fresh to ${fresh_clone}" >&2
      if git clone --quiet --branch "$branch" "$clone_url" "$fresh_clone" 2> /dev/null; then
        git -C "$fresh_clone" fetch --quiet --tags 2> /dev/null || true
        git -C "$fresh_clone" branch --set-upstream-to="origin/${branch}" "$branch" 2> /dev/null || true
        rp_flag="--repo-path ${fresh_clone}"
      else
        echo "  WARNING: fresh clone of ${org}/${repo}#${branch} failed; letting release-notes attempt its own clone" >&2
      fi
    fi
  fi

  set +e
  # shellcheck disable=SC2086
  release-notes \
    --org "$org" \
    --repo "$repo" \
    --branch "$branch" \
    $start_flag \
    --end-sha "$end_sha" \
    --dependencies=false \
    --output "$outfile" \
    --format markdown \
    --markdown-links \
    $rp_flag 2>&1 | tee "$log_file" | grep -v "^level=info"
  local rc=${PIPESTATUS[0]}
  set -e

  if [ $rc -ne 0 ]; then
    echo "  WARNING: release-notes tool exited with code $rc" >&2
    grep "^level=fatal" "$log_file" >&2 || true
    echo "" > "$outfile"
  fi
  rm -f "$log_file"
}

# Extract just the note text lines (starting with "- ") for comparison
extract_note_texts() {
  { grep '^- ' || true; } | strip_pr_refs | sort
}

echo "Generating CE release notes (${CE_REPO} @ ${CE_BRANCH})..." >&2
CE_FILE=$(mktemp)
run_release_notes "$CE_ORG" "$CE_NAME" "$REPO_ROOT" "$CE_FILE" "$CE_BRANCH"
CE_NOTES=$(cat "$CE_FILE" | strip_author | strip_wrapper)
rm -f "$CE_FILE"

EE_NOTES=""
EE_HAS_UNIQUE=false
if [[ -n "$EE_REPO" && -n "$EE_PATH" ]]; then
  EE_ORG="${EE_REPO%%/*}"
  EE_NAME="${EE_REPO##*/}"

  echo "Generating EE release notes (${EE_REPO} @ ${EE_BRANCH})..." >&2
  EE_FILE=$(mktemp)
  run_release_notes "$EE_ORG" "$EE_NAME" "$EE_PATH" "$EE_FILE" "$EE_BRANCH"
  EE_NOTES_RAW=$(cat "$EE_FILE" | strip_author | strip_pr_links | strip_wrapper)
  rm -f "$EE_FILE"

  # Check if EE has unique notes not in CE
  # Compare note text without PR refs (since CE has links, EE has plain #NNN)
  CE_TEXTS=$(echo "$CE_NOTES" | extract_note_texts)
  EE_TEXTS=$(echo "$EE_NOTES_RAW" | extract_note_texts)

  if [[ -n "$EE_TEXTS" ]]; then
    # Find EE notes not present in CE (by note text, ignoring PR references)
    EE_UNIQUE=$(comm -23 <(echo "$EE_TEXTS") <(echo "$CE_TEXTS") 2> /dev/null || echo "$EE_TEXTS")
    if [[ -n "$EE_UNIQUE" ]]; then
      EE_HAS_UNIQUE=true
      EE_NOTES=$(echo "$EE_NOTES_RAW" | prefix_ee_headings)
    fi
  fi
fi

# Build output
OUTPUT="## ${VERSION}\n\n"
OUTPUT+="**GitHub release: [${VERSION}](https://github.com/${CE_REPO}/releases/tag/${VERSION})**\n\n"

if [[ "$EE_HAS_UNIQUE" == "true" ]]; then
  # Split: CE section + EE section (EE has unique content)
  OUTPUT+="### Community Edition\n\n"
  if [[ -n "$CE_NOTES" && "$CE_NOTES" != $'\n' ]]; then
    OUTPUT+="${CE_NOTES}\n"
  else
    OUTPUT+="No notable changes.\n"
  fi
  OUTPUT+="\n### Enterprise Edition\n\n"
  OUTPUT+="**Enterprise Edition includes everything from Community Edition and more. The release notes below are for changes specific to just the Enterprise Edition.**\n\n"
  OUTPUT+="${EE_NOTES}\n"
else
  # Unified: no separate EE section (all EE notes are cherry-picks from CE)
  if [[ -n "$CE_NOTES" && "$CE_NOTES" != $'\n' ]]; then
    OUTPUT+="${CE_NOTES}\n"
  else
    OUTPUT+="No notable changes.\n"
  fi
fi

ADDONS_VERSION=$(grep -E '^KUBELB_ADDONS_CHART_VERSION \?=' "${REPO_ROOT}/Makefile" 2> /dev/null | cut -d'=' -f2 | tr -d ' ' || echo "")

# Release artifacts (always split CE/EE)
OUTPUT+="\n### Release Artifacts\n\n"
OUTPUT+="#### Community Edition\n\n"
OUTPUT+="For Community Edition, the release artifacts are available on [GitHub Releases](https://github.com/${CE_REPO}/releases/tag/${VERSION}).\n\n"
OUTPUT+="#### Enterprise Edition\n\n"

OUTPUT+="<details>\n<summary><b>Docker Images</b></summary>\n\n"
OUTPUT+="\`\`\`bash\n"
OUTPUT+="# Login to registry\n"
OUTPUT+="docker login quay.io -u <username> -p <password>\n\n"
OUTPUT+="# kubelb manager\n"
OUTPUT+="docker pull quay.io/kubermatic/kubelb-manager-ee:${VERSION}\n\n"
OUTPUT+="# ccm\n"
OUTPUT+="docker pull quay.io/kubermatic/kubelb-ccm-ee:${VERSION}\n\n"
OUTPUT+="# connection-manager\n"
OUTPUT+="docker pull quay.io/kubermatic/kubelb-connection-manager-ee:${VERSION}\n"
OUTPUT+="\`\`\`\n\n</details>\n\n"

OUTPUT+="<details>\n<summary><b>Helm Charts</b></summary>\n\n"
OUTPUT+="\`\`\`bash\n"
OUTPUT+="# kubelb-manager\n"
OUTPUT+="helm pull oci://quay.io/kubermatic/helm-charts/kubelb-manager-ee --version ${VERSION}\n\n"
OUTPUT+="# kubelb-ccm\n"
OUTPUT+="helm pull oci://quay.io/kubermatic/helm-charts/kubelb-ccm-ee --version ${VERSION}\n\n"
OUTPUT+="# kubelb-addons\n"
OUTPUT+="helm pull oci://quay.io/kubermatic/helm-charts/kubelb-addons --version ${ADDONS_VERSION}\n"
OUTPUT+="\`\`\`\n\n</details>\n\n"

OUTPUT+="<details>\n<summary><b>SBOMs</b></summary>\n\n"
OUTPUT+="Container image SBOMs are attached as OCI artifacts and attested with cosign.\n\n"
OUTPUT+="**Pull SBOM:**\n\n"
OUTPUT+="\`\`\`bash\n"
OUTPUT+="# Login to registry\n"
OUTPUT+="oras login quay.io -u <username> -p <password>\n\n"
OUTPUT+="## kubelb-manager\n"
OUTPUT+="SBOM_DIGEST=\$(oras discover --format json --artifact-type application/spdx+json \\\\\n"
OUTPUT+="  quay.io/kubermatic/kubelb-manager-ee:${VERSION} | jq -r '.referrers[0].digest')\n"
OUTPUT+="oras pull quay.io/kubermatic/kubelb-manager-ee@\${SBOM_DIGEST} --output sbom/\n\n"
OUTPUT+="## kubelb-ccm\n"
OUTPUT+="SBOM_DIGEST=\$(oras discover --format json --artifact-type application/spdx+json \\\\\n"
OUTPUT+="  quay.io/kubermatic/kubelb-ccm-ee:${VERSION} | jq -r '.referrers[0].digest')\n"
OUTPUT+="oras pull quay.io/kubermatic/kubelb-ccm-ee@\${SBOM_DIGEST} --output sbom/\n\n"
OUTPUT+="## kubelb-connection-manager\n"
OUTPUT+="SBOM_DIGEST=\$(oras discover --format json --artifact-type application/spdx+json \\\\\n"
OUTPUT+="  quay.io/kubermatic/kubelb-connection-manager-ee:${VERSION} | jq -r '.referrers[0].digest')\n"
OUTPUT+="oras pull quay.io/kubermatic/kubelb-connection-manager-ee@\${SBOM_DIGEST} --output sbom/\n"
OUTPUT+="\`\`\`\n\n"
OUTPUT+="**Verify SBOM attestation:**\n\n"
OUTPUT+="\`\`\`bash\n"
OUTPUT+="cosign verify-attestation quay.io/kubermatic/kubelb-manager-ee:${VERSION} \\\\\n"
OUTPUT+="  --type spdxjson \\\\\n"
OUTPUT+="  --certificate-identity-regexp=\"^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*\" \\\\\n"
OUTPUT+="  --certificate-oidc-issuer=https://token.actions.githubusercontent.com\n\n"
OUTPUT+="cosign verify-attestation quay.io/kubermatic/kubelb-ccm-ee:${VERSION} \\\\\n"
OUTPUT+="  --type spdxjson \\\\\n"
OUTPUT+="  --certificate-identity-regexp=\"^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*\" \\\\\n"
OUTPUT+="  --certificate-oidc-issuer=https://token.actions.githubusercontent.com\n\n"
OUTPUT+="cosign verify-attestation quay.io/kubermatic/kubelb-connection-manager-ee:${VERSION} \\\\\n"
OUTPUT+="  --type spdxjson \\\\\n"
OUTPUT+="  --certificate-identity-regexp=\"^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*\" \\\\\n"
OUTPUT+="  --certificate-oidc-issuer=https://token.actions.githubusercontent.com\n"
OUTPUT+="\`\`\`\n\n</details>\n\n"

OUTPUT+="<details>\n<summary><b>Verify Signatures</b></summary>\n\n"
OUTPUT+="**Docker images:**\n\n"
OUTPUT+="\`\`\`bash\n"
OUTPUT+="cosign verify quay.io/kubermatic/kubelb-manager-ee:${VERSION} \\\\\n"
OUTPUT+="  --certificate-identity-regexp=\"^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*\" \\\\\n"
OUTPUT+="  --certificate-oidc-issuer=https://token.actions.githubusercontent.com\n\n"
OUTPUT+="cosign verify quay.io/kubermatic/kubelb-ccm-ee:${VERSION} \\\\\n"
OUTPUT+="  --certificate-identity-regexp=\"^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*\" \\\\\n"
OUTPUT+="  --certificate-oidc-issuer=https://token.actions.githubusercontent.com\n\n"
OUTPUT+="cosign verify quay.io/kubermatic/kubelb-connection-manager-ee:${VERSION} \\\\\n"
OUTPUT+="  --certificate-identity-regexp=\"^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*\" \\\\\n"
OUTPUT+="  --certificate-oidc-issuer=https://token.actions.githubusercontent.com\n"
OUTPUT+="\`\`\`\n\n"
OUTPUT+="**Helm charts:**\n\n"
OUTPUT+="\`\`\`bash\n"
OUTPUT+="cosign verify quay.io/kubermatic/helm-charts/kubelb-manager-ee:${VERSION} \\\\\n"
OUTPUT+="  --certificate-identity-regexp=\"^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*\" \\\\\n"
OUTPUT+="  --certificate-oidc-issuer=https://token.actions.githubusercontent.com\n\n"
OUTPUT+="cosign verify quay.io/kubermatic/helm-charts/kubelb-ccm-ee:${VERSION} \\\\\n"
OUTPUT+="  --certificate-identity-regexp=\"^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*\" \\\\\n"
OUTPUT+="  --certificate-oidc-issuer=https://token.actions.githubusercontent.com\n\n"
OUTPUT+="cosign verify quay.io/kubermatic/helm-charts/kubelb-addons:${ADDONS_VERSION} \\\\\n"
OUTPUT+="  --certificate-identity-regexp=\"^https://github.com/kubermatic/kubelb/.github/workflows/release.yml@refs/tags/addons-v.*\" \\\\\n"
OUTPUT+="  --certificate-oidc-issuer=https://token.actions.githubusercontent.com\n"
OUTPUT+="\`\`\`\n\n"
OUTPUT+="**Release checksums (requires repository access):**\n\n"
OUTPUT+="\`\`\`bash\n"
OUTPUT+="cosign verify-blob --bundle checksums.txt.sigstore.json checksums.txt \\\\\n"
OUTPUT+="  --certificate-identity-regexp=\"^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*\" \\\\\n"
OUTPUT+="  --certificate-oidc-issuer=https://token.actions.githubusercontent.com\n"
OUTPUT+="\`\`\`\n\n</details>\n\n"

OUTPUT+="<details>\n<summary><b>Tools</b></summary>\n\n"
OUTPUT+="- [Cosign](https://github.com/sigstore/cosign) - Container signing\n"
OUTPUT+="- [ORAS](https://oras.land) - OCI Registry As Storage\n\n"
OUTPUT+="</details>\n"

# Write to changelog file
CHANGELOG="${REPO_ROOT}/docs/changelogs/CHANGELOG-${MINOR}.md"
mkdir -p "$(dirname "$CHANGELOG")"

if [[ ! -f "$CHANGELOG" ]]; then
  printf "# KubeLB %s Changelog\n\n" "$MINOR" > "$CHANGELOG"
fi

TMPFILE=$(mktemp)
head -1 "$CHANGELOG" > "$TMPFILE"
echo "" >> "$TMPFILE"
printf '%b' "$OUTPUT" >> "$TMPFILE"
tail -n +2 "$CHANGELOG" >> "$TMPFILE"
mv "$TMPFILE" "$CHANGELOG"

# Rebuild TOC at the top of the changelog
TITLE_LINE=$(head -1 "$CHANGELOG")
TOC=""
while IFS= read -r heading; do
  ver="${heading#\#\# }"
  anchor=$(echo "$ver" | tr -d '.')
  TOC+="- [${ver}](#${anchor})\n"
  in_block=false
  has_ce=false
  has_ee=false
  while IFS= read -r line; do
    if [[ "$line" == "## ${ver}" ]]; then
      in_block=true
      continue
    fi
    if $in_block && [[ "$line" =~ ^##\  ]]; then
      break
    fi
    if $in_block && [[ "$line" == "### Community Edition" ]]; then
      has_ce=true
    fi
    if $in_block && [[ "$line" == "### Enterprise Edition" ]]; then
      has_ee=true
    fi
  done < "$CHANGELOG"
  if $has_ce; then
    TOC+="  - [Community Edition](#community-edition)\n"
  fi
  if $has_ee; then
    TOC+="  - [Enterprise Edition](#enterprise-edition)\n"
  fi
done < <(grep '^## v' "$CHANGELOG")

TOCFILE=$(mktemp)
echo "$TITLE_LINE" > "$TOCFILE"
echo "" >> "$TOCFILE"
printf '%b' "$TOC" >> "$TOCFILE"
sed -n '/^## v/,$p' "$CHANGELOG" >> "$TOCFILE"
mv "$TOCFILE" "$CHANGELOG"

printf '%b' "$OUTPUT"
