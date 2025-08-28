#!/usr/bin/env bash

# Copyright 2025 The KubeLB Authors.
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

set -euo pipefail

# Function to extract and add repositories from Chart.yaml
add_repos_from_chart() {
  local chart_path="$1"

  if [[ ! -f "$chart_path/Chart.yaml" ]]; then
    echo "Warning: $chart_path/Chart.yaml not found, skipping..."
    return
  fi

  echo "Processing $chart_path/Chart.yaml..."

  # Extract dependencies with HTTP(S) repositories
  # Get both name and repository for each dependency
  local dep_count=$(yq eval '.dependencies | length' "$chart_path/Chart.yaml" 2> /dev/null || echo "0")

  if [[ "$dep_count" == "0" ]]; then
    echo "No dependencies found in $chart_path/Chart.yaml"
    return
  fi

  local found_repos=false

  for ((i = 0; i < dep_count; i++)); do
    local repo_url=$(yq eval ".dependencies[$i].repository" "$chart_path/Chart.yaml" 2> /dev/null || true)
    local chart_name=$(yq eval ".dependencies[$i].name" "$chart_path/Chart.yaml" 2> /dev/null || true)

    # Skip if repository is empty or is an OCI registry
    if [[ -z "$repo_url" ]] || [[ "$repo_url" =~ ^oci:// ]]; then
      continue
    fi

    # Only process HTTP(S) repositories
    if [[ ! "$repo_url" =~ ^https?:// ]]; then
      continue
    fi

    found_repos=true

    # Use the chart name as the repository name
    repo_name="$chart_name"

    # Check if the repository is already added
    if helm repo list 2> /dev/null | grep -q "^${repo_name}[[:space:]]"; then
      echo "Repository '$repo_name' already exists, skipping..."
    else
      echo "Adding repository '$repo_name' from $repo_url..."
      helm repo add "$repo_name" "$repo_url"
    fi
  done

  if [[ "$found_repos" == "false" ]]; then
    echo "No HTTP(S) repositories found in $chart_path/Chart.yaml"
  fi
}

# Process all chart directories
charts=(
  "charts/kubelb-manager"
  "charts/kubelb-ccm"
  "charts/kubelb-addons"
)

for chart in "${charts[@]}"; do
  add_repos_from_chart "$chart"
done

# Update all repositories
echo "Updating Helm repositories..."
helm repo update

echo "Helm repositories setup complete!"
