#!/usr/bin/env bash
# Copyright 2020 The KubeLB Authors.
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

if [ $# -ne 1 ]; then
  echo 'No cluster ID provided'
  echo './create-kubelb-sa.sh <cluster-id>'
  exit 1
fi

clusterId=$1
namespace="cluster-$clusterId"

kubectl create namespace "$namespace"
kubectl label namespace "$namespace" kubelb.k8c.io/managed-by=kubelb
cat << EOF | kubectl apply -n "$namespace" -f -
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kubelb-agent
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kubelb-agent-role
rules:
  - apiGroups:
      - kubelb.k8c.io
    resources:
      - loadbalancers
    verbs:
      - create
      - delete
      - get
      - list
      - patch
      - update
      - watch
  - apiGroups:
      - kubelb.k8c.io
    resources:
      - loadbalancers/status
    verbs:
      - get
      - patch
      - update
  - apiGroups:
    - kubelb.k8c.io
    resources:
    - configs
    verbs:
    - get
    - list
    - watch
  - apiGroups:
    - kubelb.k8c.io
    resources:
    - configs/status
    verbs:
    - get
    - patch
    - update
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kubelb-agent-rolebinding
subjects:
  - kind: ServiceAccount
    name: kubelb-agent
roleRef:
  kind: Role
  name: kubelb-agent-role
  apiGroup: rbac.authorization.k8s.io
EOF

# your server name goes here
server=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
token_name=$(kubectl -n $namespace get sa kubelb-agent -o jsonpath='{.secrets[0].name}')
ca=$(kubectl -n $namespace get secret/$token_name -o jsonpath='{.data.ca\.crt}')
token=$(kubectl -n $namespace get secret/$token_name -o jsonpath='{.data.token}' | base64 --decode)

echo "
apiVersion: v1
kind: Config
clusters:
- name: kubelb-cluster
  cluster:
    certificate-authority-data: ${ca}
    server: ${server}
contexts:
- name: default-context
  context:
    cluster: kubelb-cluster
    namespace: $namespace
    user: default-user
current-context: default-context
users:
- name: default-user
  user:
    token: ${token}"
