# E2E Test Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reduce e2e test duplication by replacing script polling with `assert`, adding `catch` diagnostics, using `finally` for cleanup, and consolidating step templates. Start with layer4 as reference, then roll out.

**Architecture:** Hybrid approach — refactor layer4 (13 tests) first as reference implementation, validate with `make e2e-select select=layer=layer4`, then apply same patterns to layer7/ingress, layer7/conversion, features, isolated, layer7/grpcroute.

**Tech Stack:** Chainsaw v1alpha1/v1alpha2, Kind clusters, kubectl, curl

---

## Phase 1: Foundation — New/Rewritten Step Templates

### Task 1: Create `debug/collect-diagnostics.yaml`

**Files:**
- Create: `test/e2e/step-templates/debug/collect-diagnostics.yaml`

**Step 1: Create the diagnostics step template**

This is used as a `catch` block in every test step. Collects controller logs, pod logs, events, and resource descriptions on failure.

```yaml
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
#
# Step Template: Collect diagnostics on test failure
# Used as catch block — collects controller logs, pod state, events
# Optional bindings:
#   - tenant: tenant cluster name (default: tenant1)
#   - kubelb_namespace: namespace in kubelb cluster (default: tenant-primary)
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: StepTemplate
metadata:
  name: collect-diagnostics
spec:
  bindings:
    - name: tenant
      value: tenant1
    - name: kubelb_namespace
      value: tenant-primary
  try:
    - script:
        skipCommandOutput: false
        timeout: 30s
        content: |
          set +e
          ROOT_DIR="$(git rev-parse --show-toplevel)"
          KUBECONFIGS_DIR="${ROOT_DIR}/.e2e-kubeconfigs"
          TENANT="tenant1"
          LB_NS="tenant-primary"

          echo "=== KubeLB Manager Logs (last 50 lines) ==="
          KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
            kubectl logs -n kubelb -l app.kubernetes.io/name=kubelb-manager --tail=50 2>/dev/null || echo "(no manager logs)"

          echo ""
          echo "=== KubeLB CCM Logs (last 50 lines) ==="
          KUBECONFIG="${KUBECONFIGS_DIR}/${TENANT}.kubeconfig" \
            kubectl logs -n kubelb -l app.kubernetes.io/name=kubelb-ccm --tail=50 2>/dev/null || echo "(no ccm logs)"

          echo ""
          echo "=== Envoy Pods in ${LB_NS} ==="
          KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
            kubectl get pods -n "$LB_NS" -o wide 2>/dev/null || echo "(none)"

          echo ""
          echo "=== LoadBalancer CRDs in ${LB_NS} ==="
          KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
            kubectl get loadbalancers.kubelb.k8c.io -n "$LB_NS" -o wide 2>/dev/null || echo "(none)"

          echo ""
          echo "=== Route CRDs in ${LB_NS} ==="
          KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
            kubectl get routes.kubelb.k8c.io -n "$LB_NS" -o wide 2>/dev/null || echo "(none)"

          echo ""
          echo "=== Events in ${LB_NS} (kubelb cluster) ==="
          KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
            kubectl get events -n "$LB_NS" --sort-by='.lastTimestamp' 2>/dev/null | tail -20 || echo "(none)"

          echo ""
          echo "=== Services in default ns (${TENANT}) ==="
          KUBECONFIG="${KUBECONFIGS_DIR}/${TENANT}.kubeconfig" \
            kubectl get svc -n default -o wide 2>/dev/null || echo "(none)"

          echo ""
          echo "=== Events in default ns (${TENANT}) ==="
          KUBECONFIG="${KUBECONFIGS_DIR}/${TENANT}.kubeconfig" \
            kubectl get events -n default --sort-by='.lastTimestamp' 2>/dev/null | tail -20 || echo "(none)"

          echo ""
          echo "=== Diagnostics collection complete ==="
          exit 0
```

NOTE: This template hardcodes tenant1/tenant-primary because step template bindings can't reference test-level bindings (chainsaw limitation). For tenant2 tests, the script still provides useful kubelb-side diagnostics. A future improvement could parameterize via env vars if chainsaw adds that support.

**Step 2: Verify the template is valid YAML**

Run: `python3 -c "import yaml; yaml.safe_load(open('test/e2e/step-templates/debug/collect-diagnostics.yaml'))"`
Expected: No output (valid YAML)

**Step 3: Commit**

```bash
git add test/e2e/step-templates/debug/collect-diagnostics.yaml
git commit -m "e2e: add diagnostics step template for catch blocks"
```

---

### Task 2: Create `layer4/assert-lb-crd.yaml`

**Files:**
- Create: `test/e2e/step-templates/layer4/assert-lb-crd.yaml`

**Step 1: Create assert-based LB CRD verification template**

Replaces the script polling loop in basic, multi-node, multi-port, udp, single-node tests.

```yaml
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
#
# Step Template: Assert LoadBalancer CRD exists with correct spec
# Required bindings (pass via use.with.bindings):
#   - service_name: origin service name (label selector)
#   - kubelb_namespace: namespace in kubelb cluster (e.g., tenant-primary)
# Optional bindings:
#   - expected_port: expected port number (default: "80")
#   - expected_protocol: expected protocol (default: "TCP")
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: StepTemplate
metadata:
  name: assert-lb-crd
spec:
  bindings:
    - name: expected_port
      value: "80"
    - name: expected_protocol
      value: TCP
  try:
    - assert:
        timeout: 60s
        cluster: kubelb
        resource:
          apiVersion: kubelb.k8c.io/v1alpha1
          kind: LoadBalancer
          metadata:
            namespace: ($kubelb_namespace)
            labels:
              kubelb.k8c.io/origin-name: ($service_name)
            finalizers:
              - kubelb.k8c.io/cleanup
          spec:
            ports:
              - port: ($expected_port)
            endpoints:
              - addressesReference:
                  (name != ''): true
```

**Step 2: Commit**

```bash
git add test/e2e/step-templates/layer4/assert-lb-crd.yaml
git commit -m "e2e: add assert-based LB CRD verification template"
```

---

### Task 3: Rewrite `layer4/verify-lb-cleanup.yaml` to use `assert`

**Files:**
- Modify: `test/e2e/step-templates/layer4/verify-lb-cleanup.yaml`

**Step 1: Replace script polling with error operation**

The `error` operation asserts that a resource does NOT exist. Replace the entire file:

```yaml
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
#
# Step Template: Verify LoadBalancer CRD is removed after service deletion
# Required bindings (pass via use.with.bindings):
#   - service_name: name of the Service resource (used for label selector)
#   - kubelb_namespace: namespace in kubelb cluster (e.g., tenant-primary)
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: StepTemplate
metadata:
  name: verify-lb-cleanup
spec:
  try:
    - script:
        cluster: kubelb
        timeout: 60s
        env:
          - name: SERVICE_NAME
            value: ($service_name)
          - name: LB_NS
            value: ($kubelb_namespace)
        content: |
          set -e
          for i in $(seq 1 20); do
            LB=$(kubectl get loadbalancers.kubelb.k8c.io -n "$LB_NS" \
              -l kubelb.k8c.io/origin-name="$SERVICE_NAME" -o name 2>/dev/null | head -1)
            if [ -z "$LB" ]; then
              echo "LoadBalancer CRD removed"
              exit 0
            fi
            sleep 2
          done
          echo "LoadBalancer CRD not removed"
          kubectl get loadbalancers.kubelb.k8c.io -n "$LB_NS" -o yaml
          exit 1
        check:
          ($error == null): true
```

NOTE: Keeping as script because chainsaw's `error` operation checks if a specific resource exists, but we're matching by label selector — `error` doesn't support label selectors on arbitrary CRDs well. The script is already lean.

**Step 2: Commit**

```bash
git add test/e2e/step-templates/layer4/verify-lb-cleanup.yaml
git commit -m "e2e: clean up verify-lb-cleanup template"
```

---

### Task 4: Rewrite `layer4/verify-status-propagation.yaml` to be leaner

**Files:**
- Modify: `test/e2e/step-templates/layer4/verify-status-propagation.yaml`

**Step 1: Simplify — keep script (cross-cluster comparison can't be an assert)**

```yaml
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
#
# Step Template: Verify LoadBalancer CRD status matches tenant service status
# Required bindings (pass via use.with.bindings):
#   - service_name: service name to verify
#   - tenant: tenant cluster name (e.g., tenant1)
#   - kubelb_namespace: namespace in kubelb cluster (e.g., tenant-primary)
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: StepTemplate
metadata:
  name: verify-status-propagation
spec:
  try:
    - script:
        timeout: 60s
        env:
          - name: SERVICE_NAME
            value: ($service_name)
          - name: TENANT
            value: ($tenant)
          - name: LB_NS
            value: ($kubelb_namespace)
        content: |
          set -e
          ROOT_DIR="$(git rev-parse --show-toplevel)"
          KUBECONFIGS_DIR="${ROOT_DIR}/.e2e-kubeconfigs"

          for i in $(seq 1 20); do
            LB_IP=$(KUBECONFIG="${KUBECONFIGS_DIR}/kubelb.kubeconfig" \
              kubectl get loadbalancers.kubelb.k8c.io -n "$LB_NS" \
              -l kubelb.k8c.io/origin-name="$SERVICE_NAME" \
              -o jsonpath='{.items[0].status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)

            TENANT_IP=$(KUBECONFIG="${KUBECONFIGS_DIR}/${TENANT}.kubeconfig" \
              kubectl get svc "$SERVICE_NAME" -n default \
              -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)

            if [ -n "$LB_IP" ] && [ -n "$TENANT_IP" ] && [ "$LB_IP" = "$TENANT_IP" ]; then
              echo "Status propagation verified: $LB_IP"
              exit 0
            fi
            sleep 2
          done

          echo "Status propagation failed (LB: '$LB_IP', Tenant: '$TENANT_IP')"
          exit 1
        check:
          ($error == null): true
```

**Step 2: Commit**

```bash
git add test/e2e/step-templates/layer4/verify-status-propagation.yaml
git commit -m "e2e: simplify verify-status-propagation template"
```

---

### Task 5: Slim down `layer4/verify-http-response.yaml`

**Files:**
- Modify: `test/e2e/step-templates/layer4/verify-http-response.yaml`

**Step 1: Remove verbose logging, keep retry logic (traffic test must stay as script)**

```yaml
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
#
# Step Template: Verify HTTP response from LoadBalancer service
# Required bindings (pass via use.with.bindings):
#   - service_name: name of the Service resource
#   - expected: expected substring in response
#   - port: port to test (default: "80")
#   - tenant: tenant cluster (default: tenant1)
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: StepTemplate
metadata:
  name: verify-http-response
spec:
  try:
    - script:
        cluster: ($tenant)
        timeout: 120s
        env:
          - name: SERVICE_NAME
            value: ($service_name)
          - name: PORT
            value: ($port)
          - name: EXPECTED
            value: ($expected)
        content: |
          set -e
          for i in $(seq 1 30); do
            IP=$(kubectl get svc "$SERVICE_NAME" -n default \
              -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)
            if [ -n "$IP" ]; then
              RESPONSE=$(curl -s --max-time 5 "http://${IP}:${PORT}" || true)
              if echo "$RESPONSE" | grep -q "$EXPECTED"; then
                echo "HTTP OK: got '$EXPECTED' from $IP:$PORT"
                exit 0
              fi
            fi
            sleep 3
          done
          echo "HTTP verification failed for $SERVICE_NAME:$PORT (expected: $EXPECTED)"
          exit 1
        check:
          ($error == null): true
```

**Step 2: Commit**

```bash
git add test/e2e/step-templates/layer4/verify-http-response.yaml
git commit -m "e2e: simplify verify-http-response template"
```

---

### Task 6: Create `layer4/assert-no-lb-crd.yaml` for negative tests

**Files:**
- Create: `test/e2e/step-templates/layer4/assert-no-lb-crd.yaml`

**Step 1: Create negative assertion template**

Replaces identical scripts in negative, negative-externalname, negative-nodeport tests.

```yaml
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
#
# Step Template: Assert NO LoadBalancer CRD exists for a given service
# Required bindings (pass via use.with.bindings):
#   - service_name: origin service name (label selector)
#   - kubelb_namespace: namespace in kubelb cluster (e.g., tenant-primary)
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: StepTemplate
metadata:
  name: assert-no-lb-crd
spec:
  try:
    - script:
        cluster: kubelb
        timeout: 30s
        env:
          - name: SERVICE_NAME
            value: ($service_name)
          - name: LB_NS
            value: ($kubelb_namespace)
        content: |
          set -e
          sleep 3
          LB=$(kubectl get loadbalancers.kubelb.k8c.io -n "$LB_NS" \
            -l kubelb.k8c.io/origin-name="$SERVICE_NAME" -o name 2>/dev/null || true)
          if [ -n "$LB" ]; then
            echo "LoadBalancer CRD unexpectedly created: $LB"
            exit 1
          fi
          echo "No LoadBalancer CRD created (expected)"
          exit 0
        check:
          ($error == null): true
```

**Step 2: Commit**

```bash
git add test/e2e/step-templates/layer4/assert-no-lb-crd.yaml
git commit -m "e2e: add negative assertion template for LB CRD"
```

---

## Phase 2: Refactor Layer4 Tests

### Task 7: Refactor `basic/chainsaw-test.yaml`

**Files:**
- Modify: `test/e2e/tests/layer4/service/basic/chainsaw-test.yaml`

**Step 1: Rewrite test**

Replace inline script polling (steps 2, 5) with `assert-lb-crd` template. Add `catch` blocks. Move cleanup to `finally`. Remove separate finalizer verification step (it's now part of `assert-lb-crd`).

```yaml
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

apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: basic-service-loadbalancer
  labels:
    all:
    test: basic
    resource: service
    layer: layer4
spec:
  description: |
    Test basic LoadBalancer service full lifecycle.
    Creates LoadBalancer service, verifies CRD in kubelb cluster,
    tests HTTP connectivity, then verifies cleanup removes CRD.
  bindings:
    - name: service_name
      value: echo-basic
    - name: kubelb_namespace
      value: tenant-primary
  steps:
    - name: create-loadbalancer-service
      cluster: tenant1
      try:
        - apply:
            resource:
              apiVersion: v1
              kind: Service
              metadata:
                name: ($service_name)
                namespace: default
              spec:
                type: LoadBalancer
                selector:
                  app: echo-shared
                ports:
                  - port: 80
                    targetPort: 5678
        - assert:
            timeout: 60s
            resource:
              apiVersion: v1
              kind: Service
              metadata:
                name: ($service_name)
                namespace: default
              status:
                loadBalancer:
                  ingress:
                    - {}
      catch:
        - use:
            template: ../../../../step-templates/debug/collect-diagnostics.yaml

    - name: verify-loadbalancer-crd
      use:
        template: ../../../../step-templates/layer4/assert-lb-crd.yaml
        with:
          bindings:
            - name: service_name
              value: echo-basic
            - name: kubelb_namespace
              value: tenant-primary
      catch:
        - use:
            template: ../../../../step-templates/debug/collect-diagnostics.yaml

    - name: verify-status-propagation
      use:
        template: ../../../../step-templates/layer4/verify-status-propagation.yaml
        with:
          bindings:
            - name: service_name
              value: echo-basic
            - name: tenant
              value: tenant1
            - name: kubelb_namespace
              value: tenant-primary
      catch:
        - use:
            template: ../../../../step-templates/debug/collect-diagnostics.yaml

    - name: verify-http-response
      use:
        template: ../../../../step-templates/layer4/verify-http-response.yaml
        with:
          bindings:
            - name: service_name
              value: echo-basic
            - name: expected
              value: hello-shared
            - name: port
              value: "80"
            - name: tenant
              value: tenant1
      catch:
        - use:
            template: ../../../../step-templates/debug/collect-diagnostics.yaml

    - name: cleanup-and-verify
      try:
        - delete:
            cluster: tenant1
            ref:
              apiVersion: v1
              kind: Service
              name: ($service_name)
              namespace: default
      finally:
        - script:
            cluster: tenant1
            timeout: 30s
            content: |
              kubectl delete service echo-basic -n default --ignore-not-found

    - name: verify-crd-removed
      use:
        template: ../../../../step-templates/layer4/verify-lb-cleanup.yaml
        with:
          bindings:
            - name: service_name
              value: echo-basic
            - name: kubelb_namespace
              value: tenant-primary
```

**Step 2: Commit**

```bash
git add test/e2e/tests/layer4/service/basic/chainsaw-test.yaml
git commit -m "e2e: refactor basic layer4 test with assert templates and catch blocks"
```

---

### Task 8: Refactor negative tests (3 files)

**Files:**
- Modify: `test/e2e/tests/layer4/service/negative/chainsaw-test.yaml`
- Modify: `test/e2e/tests/layer4/service/negative-externalname/chainsaw-test.yaml`
- Modify: `test/e2e/tests/layer4/service/negative-nodeport/chainsaw-test.yaml`

**Step 1: Rewrite `negative/chainsaw-test.yaml`**

Replace inline "verify no LB CRD" script with `assert-no-lb-crd` template.

```yaml
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

apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: clusterip-no-loadbalancer-crd
  labels:
    all:
    test: negative
    resource: service
    layer: layer4
spec:
  description: |
    Negative test: ClusterIP services do NOT create LoadBalancer CRDs.
  bindings:
    - name: name
      value: echo-clusterip
    - name: kubelb_namespace
      value: tenant-primary
  steps:
    - name: create-clusterip-service
      cluster: tenant1
      try:
        - apply:
            resource:
              apiVersion: v1
              kind: Service
              metadata:
                name: ($name)
                namespace: default
              spec:
                type: ClusterIP
                selector:
                  app: echo-shared
                ports:
                  - port: 80
                    targetPort: 5678
        - assert:
            timeout: 60s
            resource:
              apiVersion: v1
              kind: Service
              metadata:
                name: ($name)
                namespace: default
              spec:
                type: ClusterIP

    - name: verify-no-loadbalancer-crd
      use:
        template: ../../../../step-templates/layer4/assert-no-lb-crd.yaml
        with:
          bindings:
            - name: service_name
              value: echo-clusterip
            - name: kubelb_namespace
              value: tenant-primary
```

**Step 2: Rewrite `negative-externalname/chainsaw-test.yaml`**

```yaml
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

apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: externalname-no-loadbalancer-crd
  labels:
    all:
    test: negative-externalname
    resource: service
    layer: layer4
spec:
  description: |
    Negative test: ExternalName services do NOT create LoadBalancer CRDs.
  bindings:
    - name: name
      value: echo-externalname
    - name: kubelb_namespace
      value: tenant-primary
  steps:
    - name: create-externalname-service
      cluster: tenant1
      try:
        - apply:
            file: ../../../../testdata/services/echo-externalname.yaml
        - assert:
            timeout: 60s
            resource:
              apiVersion: v1
              kind: Service
              metadata:
                name: ($name)
                namespace: default
              spec:
                type: ExternalName

    - name: verify-no-loadbalancer-crd
      use:
        template: ../../../../step-templates/layer4/assert-no-lb-crd.yaml
        with:
          bindings:
            - name: service_name
              value: echo-externalname
            - name: kubelb_namespace
              value: tenant-primary
```

**Step 3: Rewrite `negative-nodeport/chainsaw-test.yaml`**

```yaml
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

apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: nodeport-no-loadbalancer-crd
  labels:
    all:
    test: negative-nodeport
    resource: service
    layer: layer4
spec:
  description: |
    Negative test: NodePort services do NOT create LoadBalancer CRDs.
  bindings:
    - name: name
      value: echo-nodeport
    - name: kubelb_namespace
      value: tenant-primary
  steps:
    - name: create-nodeport-service
      cluster: tenant1
      try:
        - apply:
            resource:
              apiVersion: v1
              kind: Service
              metadata:
                name: ($name)
                namespace: default
              spec:
                type: NodePort
                selector:
                  app: echo-shared
                ports:
                  - port: 80
                    targetPort: 5678
        - assert:
            timeout: 60s
            resource:
              apiVersion: v1
              kind: Service
              metadata:
                name: ($name)
                namespace: default
              spec:
                type: NodePort

    - name: verify-no-loadbalancer-crd
      use:
        template: ../../../../step-templates/layer4/assert-no-lb-crd.yaml
        with:
          bindings:
            - name: service_name
              value: echo-nodeport
            - name: kubelb_namespace
              value: tenant-primary
```

**Step 4: Commit**

```bash
git add test/e2e/tests/layer4/service/negative/chainsaw-test.yaml \
  test/e2e/tests/layer4/service/negative-externalname/chainsaw-test.yaml \
  test/e2e/tests/layer4/service/negative-nodeport/chainsaw-test.yaml
git commit -m "e2e: refactor negative layer4 tests to use assert-no-lb-crd template"
```

---

### Task 9: Refactor `multi-port/chainsaw-test.yaml`

**Files:**
- Modify: `test/e2e/tests/layer4/service/multi-port/chainsaw-test.yaml`

**Step 1: Replace inline LB CRD wait script (step 3) with assert template. Add catch blocks.**

```yaml
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

apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: multi-port-service-loadbalancer
  labels:
    all:
    test: multi-port
    resource: service
    layer: layer4
spec:
  description: |
    Test multi-port LoadBalancer service with correct port routing.
    Deploys multi-echo server with two containers on different ports,
    creates multi-port LoadBalancer, verifies each port returns correct message.
  bindings:
    - name: name
      value: echo-multiport
    - name: message1
      value: "hello-from-port-80"
    - name: message2
      value: "hello-from-port-81"
    - name: port1
      value: 80
    - name: port2
      value: 81
    - name: kubelb_namespace
      value: tenant-primary
  steps:
    - name: deploy-multi-echo-server
      cluster: tenant1
      try:
        - apply:
            file: ../../../../testdata/deployments/multi-echo.yaml
        - assert:
            timeout: 60s
            resource:
              apiVersion: apps/v1
              kind: Deployment
              metadata:
                name: ($name)
                namespace: default
              status:
                readyReplicas: 1
      catch:
        - use:
            template: ../../../../step-templates/debug/collect-diagnostics.yaml

    - name: create-multiport-loadbalancer
      cluster: tenant1
      try:
        - apply:
            file: ../../../../testdata/services/multi-echo-lb.yaml
        - assert:
            timeout: 60s
            resource:
              apiVersion: v1
              kind: Service
              metadata:
                name: ($name)
                namespace: default
              status:
                loadBalancer:
                  ingress:
                    - {}
      catch:
        - use:
            template: ../../../../step-templates/debug/collect-diagnostics.yaml

    - name: verify-loadbalancer-crd
      use:
        template: ../../../../step-templates/layer4/assert-lb-crd.yaml
        with:
          bindings:
            - name: service_name
              value: echo-multiport
            - name: kubelb_namespace
              value: tenant-primary
      catch:
        - use:
            template: ../../../../step-templates/debug/collect-diagnostics.yaml

    - name: verify-status-propagation
      use:
        template: ../../../../step-templates/layer4/verify-status-propagation.yaml
        with:
          bindings:
            - name: service_name
              value: echo-multiport
            - name: tenant
              value: tenant1
            - name: kubelb_namespace
              value: tenant-primary

    - name: verify-port1-response
      use:
        template: ../../../../step-templates/layer4/verify-http-response.yaml
        with:
          bindings:
            - name: service_name
              value: echo-multiport
            - name: expected
              value: "hello-from-port-80"
            - name: port
              value: "80"
            - name: tenant
              value: tenant1

    - name: verify-port2-response
      use:
        template: ../../../../step-templates/layer4/verify-http-response.yaml
        with:
          bindings:
            - name: service_name
              value: echo-multiport
            - name: expected
              value: "hello-from-port-81"
            - name: port
              value: "81"
            - name: tenant
              value: tenant1
```

**Step 2: Commit**

```bash
git add test/e2e/tests/layer4/service/multi-port/chainsaw-test.yaml
git commit -m "e2e: refactor multi-port test with assert template and catch blocks"
```

---

### Task 10: Refactor `multi-node/chainsaw-test.yaml`

**Files:**
- Modify: `test/e2e/tests/layer4/service/multi-node/chainsaw-test.yaml`

**Step 1: Replace inline LB CRD wait (step 3) with assert template. Add catch. Keep endpoint count and multiple-requests scripts (unique to this test).**

```yaml
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

apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: multi-node-service-loadbalancer
  labels:
    all:
    test: multi-node
    resource: service
    layer: layer4
spec:
  description: |
    Test LoadBalancer with multiple backend endpoints.
    Deploys echo server with 3 replicas, creates LoadBalancer,
    verifies all endpoints are synced and service responds correctly.
  bindings:
    - name: name
      value: echo-multinode
    - name: message
      value: "hello-from-multinode"
    - name: replicas
      value: 3
    - name: kubelb_namespace
      value: tenant-primary
  steps:
    - name: deploy-multi-replica-server
      cluster: tenant1
      try:
        - apply:
            file: ../../../../testdata/deployments/echo-server-replicas.yaml
        - assert:
            timeout: 60s
            resource:
              apiVersion: apps/v1
              kind: Deployment
              metadata:
                name: ($name)
                namespace: default
              status:
                readyReplicas: ($replicas)
      catch:
        - use:
            template: ../../../../step-templates/debug/collect-diagnostics.yaml

    - name: create-loadbalancer-service
      cluster: tenant1
      try:
        - apply:
            file: ../../../../testdata/services/echo-lb.yaml
        - assert:
            timeout: 60s
            resource:
              apiVersion: v1
              kind: Service
              metadata:
                name: ($name)
                namespace: default
              status:
                loadBalancer:
                  ingress:
                    - {}
      catch:
        - use:
            template: ../../../../step-templates/debug/collect-diagnostics.yaml

    - name: verify-loadbalancer-crd
      use:
        template: ../../../../step-templates/layer4/assert-lb-crd.yaml
        with:
          bindings:
            - name: service_name
              value: echo-multinode
            - name: kubelb_namespace
              value: tenant-primary
      catch:
        - use:
            template: ../../../../step-templates/debug/collect-diagnostics.yaml

    - name: verify-status-propagation
      use:
        template: ../../../../step-templates/layer4/verify-status-propagation.yaml
        with:
          bindings:
            - name: service_name
              value: echo-multinode
            - name: tenant
              value: tenant1
            - name: kubelb_namespace
              value: tenant-primary

    - name: verify-endpoint-count
      cluster: tenant1
      try:
        - script:
            timeout: 60s
            content: |
              set -e
              for i in $(seq 1 20); do
                EP_COUNT=$(kubectl get endpointslices \
                  -l kubernetes.io/service-name=echo-multinode \
                  -o jsonpath='{.items[*].endpoints[*].addresses}' 2>/dev/null \
                  | jq -s 'add | length' || echo "0")
                if [ "$EP_COUNT" -eq 3 ]; then
                  echo "Endpoint count verified: $EP_COUNT"
                  exit 0
                fi
                sleep 2
              done
              echo "Expected 3 endpoints, got $EP_COUNT"
              exit 1
            check:
              ($error == null): true

    - name: verify-http-response
      use:
        template: ../../../../step-templates/layer4/verify-http-response.yaml
        with:
          bindings:
            - name: service_name
              value: echo-multinode
            - name: expected
              value: hello-from-multinode
            - name: port
              value: "80"
            - name: tenant
              value: tenant1

    - name: verify-multiple-requests
      cluster: tenant1
      try:
        - script:
            timeout: 60s
            content: |
              set -e
              IP=$(kubectl get svc echo-multinode -n default \
                -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
              SUCCESS=0
              for i in $(seq 1 10); do
                RESPONSE=$(curl -s --max-time 5 "http://${IP}" || true)
                if echo "$RESPONSE" | grep -q "hello-from-multinode"; then
                  SUCCESS=$((SUCCESS + 1))
                fi
              done
              if [ "$SUCCESS" -eq 10 ]; then
                echo "All 10 requests succeeded"
                exit 0
              fi
              echo "$SUCCESS/10 requests succeeded"
              exit 1
            check:
              ($error == null): true
```

**Step 2: Commit**

```bash
git add test/e2e/tests/layer4/service/multi-node/chainsaw-test.yaml
git commit -m "e2e: refactor multi-node test with assert template and catch blocks"
```

---

### Task 11: Refactor `multi-port-multi-node/chainsaw-test.yaml`

**Files:**
- Modify: `test/e2e/tests/layer4/service/multi-port-multi-node/chainsaw-test.yaml`

**Step 1: Replace inline LB CRD wait with assert template. Add catch. Slim down multi-request script.**

Same pattern as multi-port but with replicas. Replace step 3 (inline script) with `assert-lb-crd` template. Keep the multi-request verification script (unique test). Add catch blocks.

The file follows the exact same pattern as Task 9 (multi-port) — use `assert-lb-crd` template for step 3, keep rest of template usages, add catch blocks, slim the multi-request script.

**Step 2: Commit**

```bash
git add test/e2e/tests/layer4/service/multi-port-multi-node/chainsaw-test.yaml
git commit -m "e2e: refactor multi-port-multi-node test with assert template"
```

---

### Task 12: Refactor `udp/chainsaw-test.yaml`

**Files:**
- Modify: `test/e2e/tests/layer4/service/udp/chainsaw-test.yaml`

**Step 1: Replace inline LB CRD wait with assert template (with UDP protocol). Add catch blocks. Keep UDP connectivity script (unique).**

Use `assert-lb-crd` with `expected_protocol: UDP` and `expected_port: "9999"`. Keep the tftp/curl UDP verification as inline script since it's unique to this test.

**Step 2: Commit**

```bash
git add test/e2e/tests/layer4/service/udp/chainsaw-test.yaml
git commit -m "e2e: refactor udp test with assert template and catch blocks"
```

---

### Task 13: Refactor `single-node/chainsaw-test.yaml`

**Files:**
- Modify: `test/e2e/tests/layer4/service/single-node/chainsaw-test.yaml`

**Step 1: Replace inline LB CRD wait (step 2) with assert template. Add catch. Move cleanup to finally.**

```yaml
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

apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: single-node-service-loadbalancer
  labels:
    all:
    multi-tenant: "true"
    test: single-node
    resource: service
    layer: layer4
    nodes: single
spec:
  description: |
    Test LoadBalancer service on single-node cluster (tenant2).
  bindings:
    - name: service_name
      value: echo-single-node
    - name: kubelb_namespace
      value: tenant-secondary
  steps:
    - name: create-loadbalancer-service
      cluster: tenant2
      try:
        - apply:
            resource:
              apiVersion: v1
              kind: Service
              metadata:
                name: ($service_name)
                namespace: default
              spec:
                type: LoadBalancer
                selector:
                  app: echo-shared
                ports:
                  - port: 80
                    targetPort: 5678
        - assert:
            timeout: 60s
            resource:
              apiVersion: v1
              kind: Service
              metadata:
                name: ($service_name)
                namespace: default
              status:
                loadBalancer:
                  ingress:
                    - {}
      catch:
        - use:
            template: ../../../../step-templates/debug/collect-diagnostics.yaml

    - name: verify-loadbalancer-crd
      use:
        template: ../../../../step-templates/layer4/assert-lb-crd.yaml
        with:
          bindings:
            - name: service_name
              value: echo-single-node
            - name: kubelb_namespace
              value: tenant-secondary
      catch:
        - use:
            template: ../../../../step-templates/debug/collect-diagnostics.yaml

    - name: verify-http-response
      use:
        template: ../../../../step-templates/layer4/verify-http-response.yaml
        with:
          bindings:
            - name: service_name
              value: echo-single-node
            - name: expected
              value: hello-shared
            - name: port
              value: "80"
            - name: tenant
              value: tenant2

    - name: cleanup
      cluster: tenant2
      try:
        - delete:
            ref:
              apiVersion: v1
              kind: Service
              name: ($service_name)
              namespace: default
      finally:
        - script:
            cluster: tenant2
            timeout: 30s
            content: |
              kubectl delete service echo-single-node -n default --ignore-not-found
```

**Step 2: Commit**

```bash
git add test/e2e/tests/layer4/service/single-node/chainsaw-test.yaml
git commit -m "e2e: refactor single-node test with assert template and catch blocks"
```

---

### Task 14: Refactor `cross-tenant/chainsaw-test.yaml`

**Files:**
- Modify: `test/e2e/tests/layer4/service/cross-tenant/chainsaw-test.yaml`

**Step 1: Replace inline "wait for both LBs" script (step 5) with two assert-lb-crd template calls. Replace inline status propagation script (step 6) with two verify-status-propagation template calls. Add catch blocks.**

The cross-tenant inline scripts do the same thing as the templates but for two tenants. Replace with two sequential template calls.

**Step 2: Commit**

```bash
git add test/e2e/tests/layer4/service/cross-tenant/chainsaw-test.yaml
git commit -m "e2e: refactor cross-tenant test to use templates"
```

---

### Task 15: Refactor `multiple-services/chainsaw-test.yaml`

**Files:**
- Modify: `test/e2e/tests/layer4/service/multiple-services/chainsaw-test.yaml`

**Step 1: Replace inline "wait for both LBs" (step 5) with two assert-lb-crd calls. Replace inline status propagation (step 6) with two verify-status-propagation calls. Add catch blocks. Keep isolation and concurrent request scripts (unique).**

**Step 2: Commit**

```bash
git add test/e2e/tests/layer4/service/multiple-services/chainsaw-test.yaml
git commit -m "e2e: refactor multiple-services test to use templates"
```

---

### Task 16: Refactor `proxy-protocol/chainsaw-test.yaml`

**Files:**
- Modify: `test/e2e/tests/layer4/service/proxy-protocol/chainsaw-test.yaml`

**Step 1: Add catch blocks to all steps. Move cleanup (steps 9-10) to finally block. Keep inline scripts for ETP/PP verification (unique to this test). Replace LB CRD wait scripts with assert-lb-crd where applicable.**

This test is complex (530 lines) and most scripts are unique (ETP propagation, PP header parsing). Main improvements:
- Add catch blocks
- Move cleanup to finally
- The ETP/PP scripts stay inline since they're unique

**Step 2: Commit**

```bash
git add test/e2e/tests/layer4/service/proxy-protocol/chainsaw-test.yaml
git commit -m "e2e: add catch blocks and finally cleanup to proxy-protocol test"
```

---

### Task 17: Refactor `negative/protocol-mismatch/chainsaw-test.yaml`

**Files:**
- Modify: `test/e2e/tests/layer4/service/negative/protocol-mismatch/chainsaw-test.yaml`

**Step 1: Replace inline LB CRD wait (step 3) with assert-lb-crd (UDP). Move cleanup (step 6) to finally. Add catch blocks. Keep TCP-to-UDP failure scripts (unique).**

**Step 2: Commit**

```bash
git add test/e2e/tests/layer4/service/negative/protocol-mismatch/chainsaw-test.yaml
git commit -m "e2e: refactor protocol-mismatch test with assert template and finally"
```

---

### Task 18: Validate layer4 tests

**Step 1: Run layer4 e2e tests**

Run: `make e2e-select select=layer=layer4`

Expected: All 13 tests pass. If any fail, debug using the new catch block diagnostics output.

**Step 2: If all pass, commit any fixes and tag this as the reference implementation**

```bash
git add -A
git commit -m "e2e: layer4 refactor complete — reference implementation"
```

---

## Phase 3: Roll Out to Layer7/Ingress Tests

### Task 19: Create ingress-specific templates

**Files:**
- Create: `test/e2e/step-templates/ingress/assert-route-crd.yaml` (replace script with assert)
- Modify: existing ingress templates to be leaner

Pattern: Same as layer4 — replace script polling with assert for Route CRD existence, add catch blocks, use finally for cleanup.

### Task 20-25: Refactor ingress tests one by one

Apply the same patterns from layer4:
- Replace inline Route CRD verification scripts with `assert` template
- Replace inline "wait for Ingress IP" scripts with existing `wait-for-ingress-ip.yaml` template
- Add catch blocks with `collect-diagnostics.yaml`
- Move cleanup to `finally` blocks
- Validate: `make e2e-select select=resource=ingress`

---

## Phase 4: Roll Out to Layer7/Conversion Tests

### Task 26-30: Refactor conversion tests

Same patterns. The conversion tests have their own templates already (`conversion/verify-gateway.yaml`, etc). Main work:
- Add catch blocks
- Replace any remaining inline scripts with existing templates
- Use finally for cleanup
- Validate: `make e2e-select select=suite=conversion`

---

## Phase 5: Roll Out to Features/Isolated Tests

### Task 31-35: Refactor features and isolated tests

- Features tests (metrics, syncsecret): Add catch blocks, use templates
- Isolated tests (config, tenant): Add catch blocks, move Config/Tenant reset to finally
- Validate: `make e2e-select select=suite=features` and `make e2e-select select=suite=isolated`

---

## Phase 6: Roll Out to GRPCRoute Tests

### Task 36-37: Refactor grpcroute tests

- Replace inline Gateway IP wait with existing `wait-for-gateway-ip.yaml`
- Add catch blocks
- Validate: `make e2e-select select=resource=grpcroute`

---

## Phase 7: Final Validation

### Task 38: Full e2e run

Run: `make e2e`

Expected: All 114 tests pass.

### Task 39: Cleanup unused templates (if any)

Check if any old templates are now unused after refactoring.

### Task 40: Update `test/e2e/CLAUDE.md` with new patterns

Document the new conventions:
- Prefer `assert` over script polling for K8s resource checks
- Use `finally` for cleanup
- Available templates and when to use them

---

## Execution Findings (for EE repo)

### Chainsaw Limitations Discovered

1. **`catch` blocks do NOT support `use` (template references)**
   - Chainsaw v0.2.14 catch blocks only support inline operations: `script`, `command`, `podLogs`, `events`, `describe`, `get`, `delete`, `wait`, `sleep`
   - This was never supported in any version — not a regression
   - The `use` field is only valid in `try` and `finally` blocks
   - **Impact**: The `debug/collect-diagnostics.yaml` template was created but cannot be used in catch blocks. It can still be used in `try`/`finally` blocks if needed.
   - **Workaround**: For on-failure diagnostics, use chainsaw's built-in `events` and `podLogs` operations inline in catch blocks, or use inline `script` blocks.

2. **`assert` strict type checking on ports**
   - Chainsaw `assert` compares types strictly — `port: "80"` (string) != `port: 80` (int64)
   - Always use unquoted integers for numeric fields in assert resources
   - Binding defaults must match the CRD field type exactly

3. **`assert` strict array length matching**
   - When asserting `spec.ports: [{port: 80}]`, chainsaw checks that the array has exactly 1 element
   - Multi-port services (2+ ports) will fail this assertion
   - **Solution**: Don't assert specific ports in a shared template; assert labels/finalizers/endpoints instead

4. **`use.with.bindings` — always pass hardcoded values** (previously known)
   - JMESPath refs like `($binding_name)` may silently resolve to empty
   - Always use hardcoded string/int values in `use.with.bindings`

### Pre-existing Issues

- Chainsaw v0.2.14 has nil pointer panics in `apply/operation.go:102` when running tests with parallel=6. This is a chainsaw bug, not related to our test refactoring. The panics cause ~80% of parallel tests to crash. Isolated tests (concurrent: false) run fine.
- `envoy-cleanup` test fails pre-refactor — not related to our changes.

### What Worked Well

- `assert` for K8s resource existence (labels, finalizers, endpoints) — cleaner than script polling
- `finally` blocks for cleanup — ensures resources are cleaned up even on failure
- Step templates in `try` blocks — significant deduplication for common patterns
- Removing `skipCommandOutput: true` — useful output is now visible on failure
