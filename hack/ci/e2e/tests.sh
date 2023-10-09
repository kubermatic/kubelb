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

echodate "Expose sample service"
export KUBECONFIG="${TMPDIR}"/tenant1.kubeconfig
kubectl create deployment --image nginx:stable test-server
kubectl expose deployment test-server --name=lb-test-server --type=LoadBalancer --port=80
SVC_UID=$(kubectl get svc lb-test-server -o json | jq --raw-output '.metadata.uid')

echodate "Wait for KubeLB to propagate IP address"
until kubectl get service/lb-test-server --output=jsonpath='{.status.loadBalancer}' | grep "ingress"; do sleep 1 ; done
# TODO with newer kubectl can change to:
# kubectl wait --for=jsonpath='{.status.loadBalancer.ingress}' svc lb-test-server 

ip=$(kubectl get svc lb-test-server -o json | jq --raw-output '.status.loadBalancer.ingress[0].ip')

echodate "Trying to access http://$ip"
# TODO: for some reason, it can take a pretty long time now for the first service to be operational
# and the curl --connect-timeout --max-time seem to not cover the entire case with retries
for i in {1..200}; do
    sleep 10
    if curl --connect-timeout 60 --max-time 60 --fail http://"$ip"; then
        echodate "SUCCESS: got 200 from http://$ip"
        exit 0
    fi
done
echodate "FAIL: accessing http://$ip timed out"
exit 1
