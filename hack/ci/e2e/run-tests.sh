#!/usr/bin/env bash

# Copyright 2023 The KubeLB Authors.
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

export ROOT_DIR="$(git rev-parse --show-toplevel)"
export DIR="$(dirname "$(realpath "$0")")"
cd "$DIR"

source "${ROOT_DIR}/hack/lib.sh"

function cleanup() {
  echodate "Executing cleanup"
  kind delete clusters --all || :
  if [[ ! -z "${TMPDIR+x}" ]]; then
    rm -rf "$TMPDIR"
  fi
}

function kubectl_get_debug_resources() {
  kubectl -v9 get po -A -o wide > "$ARTIFACTS/kubectl/$1.po.a.wide.out" 2>&1 || :
  kubectl -v9 get svc -A -o wide > "$ARTIFACTS/kubectl/$1.svc.a.wide.out" 2>&1 || :
  kubectl -v9 get node -o wide > "$ARTIFACTS/kubectl/$1.node.wide.out" 2>&1 || :

  kubectl -v9 get po -A -o yaml > "$ARTIFACTS/kubectl/$1.po.yaml" 2>&1 || :
  kubectl -v9 get svc -A -o yaml > "$ARTIFACTS/kubectl/$1.svc.yaml" 2>&1 || :
  kubectl -v9 get node -o yaml > "$ARTIFACTS/kubectl/$1.node.yaml" 2>&1 || :
}

function dump_logs() {
  if [[ ! -z "${ARTIFACTS+x}" ]]; then
    echodate "Dumping logs"
    mkdir -p "$ARTIFACTS/kubectl"
    export KUBECONFIG="${TMPDIR}"/kubelb.kubeconfig
    kubectl_get_debug_resources kubelb
    export KUBECONFIG="${TMPDIR}"/tenant1.kubeconfig
    kubectl_get_debug_resources tenant1
    export KUBECONFIG="${TMPDIR}"/tenant2.kubeconfig
    kubectl_get_debug_resources tenant2

    mkdir -p "$ARTIFACTS/kind_logs/{kubelb,tenant1}"
    kind export logs "$ARTIFACTS/kind_logs/kubelb" --name kubelb || :
    kind export logs "$ARTIFACTS/kind_logs/tenant1" --name tenant1 || :
    kind export logs "$ARTIFACTS/kind_logs/tenant2" --name tenant2 || :

    mkdir -p "$ARTIFACTS/pod_logs"
    protokol --kubeconfig "${TMPDIR}"/kubelb.kubeconfig --flat --oneshot --output "$ARTIFACTS/pod_logs" --namespace kubelb 'kubelb-*' || :
    protokol --kubeconfig "${TMPDIR}"/kubelb.kubeconfig --flat --oneshot --output "$ARTIFACTS/pod_logs" --namespace tenant1 'kubelb-ccm-*' || :
    protokol --kubeconfig "${TMPDIR}"/kubelb.kubeconfig --flat --oneshot --output "$ARTIFACTS/pod_logs" --namespace tenant2 'kubelb-ccm-*' || :
  fi
}

trap dump_logs ERR
trap cleanup EXIT SIGINT SIGTERM

export TMPDIR=$(mktemp -d)
if [[ -z "${local_ip+x}" ]]; then
  local_ip=$(ip -j route show | jq -r '.[] | select(.dst == "default") | .prefsrc' | head -n1)
fi

echodate "Pre-pulling base image in the background"
(
  cd ${ROOT_DIR}
  dockerfile="ccm.dockerfile"
  line=$(grep "as builder" $dockerfile)
  dockerImage=$(echo $line | awk -F'FROM | as builder' '{print $2}')
  echodate "Pulling image: $dockerImage"
  docker pull $dockerImage &> /dev/null &
)

echodate "Creating kubelb kind cluster"
KUBECONFIG="${TMPDIR}"/kubelb.kubeconfig kind create cluster --retain --name kubelb --config <(
  cat << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "$local_ip"
EOF
) &

echodate "Creating tenant1 kind cluster"
KUBECONFIG="${TMPDIR}"/tenant1.kubeconfig kind create cluster --retain --name tenant1 --config <(
  cat << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "$local_ip"
EOF
) &

echodate "Creating tenant2 kind cluster"
KUBECONFIG="${TMPDIR}"/tenant2.kubeconfig kind create cluster --retain --name tenant2 --config <(
  cat << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "$local_ip"
nodes:
  - role: control-plane
  - role: worker
  - role: worker
  - role: worker
EOF
) &

wait

./deploy-kubelb.sh
./tests.sh
