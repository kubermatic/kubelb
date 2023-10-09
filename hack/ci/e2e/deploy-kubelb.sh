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

set -xeuo pipefail

source "${ROOT_DIR}/hack/lib.sh"

METALLB_VERSION=v0.13.11

export KUBECONFIG="${TMPDIR}"/kubelb.kubeconfig

echodate "Deploying metallb"
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/"$METALLB_VERSION"/config/manifests/metallb-native.yaml
kubectl wait --namespace metallb-system --for=condition=ready pod --selector=app=metallb --timeout=360s
gw=$(docker network inspect -f json kind | jq --raw-output '.[].IPAM.Config[0].Gateway' | sed -e 's/\(.*\..*\).*\..*\..*/\1/')

cat <<EOF | kubectl apply -f -
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: kubelb
  namespace: metallb-system
spec:
  addresses:
  - $gw.255.200-$gw.255.250
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: kubelb
  namespace: metallb-system
EOF

echodate "Set up kubelb pre-requisites"
kubectl create ns cluster-tenant1
kubectl config set-context $(kubectl config current-context) --namespace="cluster-tenant1"
kubectl create secret generic kubelb-cluster --from-file=kubelb="${TMPDIR}"/kubelb.kubeconfig --from-file=tenant="${TMPDIR}"/tenant1.kubeconfig
kubectl config set-context $(kubectl config current-context) --namespace=kubelb
(
    cd "${ROOT_DIR}"
    echodate "Build kubelb binaries"
    make build-kubelb
    make build-ccm
    
    echodate "Build kubelb images"
    KUBELB_IMAGE_NAME="kubermatic.io/kubelb:e2e" CCM_IMAGE_NAME="kubermatic.io/ccm:e2e" make docker-image
    kind load docker-image --name=kubelb kubermatic.io/kubelb:e2e
    kind load docker-image --name=kubelb kubermatic.io/ccm:e2e
    
    echodate "Build kubelb images"
    make install
    make e2e-deploy-kubelb
    make e2e-deploy-ccm
)