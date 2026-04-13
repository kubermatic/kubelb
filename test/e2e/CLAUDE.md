# E2E Test Guide

## Quick Reference

```bash
make e2e                                    # Run all tests
make e2e-select select=resource=httproute   # By resource
make e2e-select select=layer=layer7         # By layer
make e2e-select select=suite=conversion     # Conversion only
make e2e-select select=test=basic           # Single test name
chainsaw test --test-file test/e2e/tests/layer7/httproute/basic/chainsaw-test.yaml
```

## Clusters

| Cluster | Role | Nodes | kubelb_namespace |
|---------|------|-------|-----------------|
| `kubelb` | Manager cluster | 1 | N/A |
| `tenant1` | Default CCM tenant | 3 workers | `tenant-primary` |
| `tenant2` | Single-node CCM tenant | 1 | `tenant-secondary` |
| `standalone` | Conversion CCM | 1 | N/A |

## Test Structure

Tests live in `test/e2e/tests/{layer4,layer7}/{resource}/{test-name}/chainsaw-test.yaml`.
Templates live in `test/e2e/step-templates/{common,gateway,layer4,layer7,ingress,conversion,isolated}/`.

Standard L7 test pattern:

1. Deploy backend(s) via `deploy-echo-backend` template
2. Create Gateway via `create-http-gateway` template
3. Create route (HTTPRoute/GRPCRoute/Ingress) — usually inline script (needs Gateway IP for nip.io hostnames)
4. Verify Route CRD via `verify-route-crd` template
5. Verify HTTP connectivity — inline script
6. Cleanup in `finally` block — inline script (always inline, `catch` blocks don't support `use`)
7. Verify cleanup via `verify-route-cleanup` template

## Chainsaw Pitfalls

### `$namespace` is reserved

Chainsaw sets `$namespace` to auto-generated test namespace. Never use `namespace` as a binding name. Hardcode `default` directly.

### `use.with.bindings` must be hardcoded

JMESPath refs (`($var)`) silently resolve to empty. Always pass literal strings:

```yaml
# WRONG — silently empty
- name: gateway_name
  value: ($gateway_name)

# CORRECT
- name: gateway_name
  value: basic-gw
```

### StepTemplates use `try`, never `finally`

`finally` is invalid in StepTemplate spec. Tests that call templates handle cleanup themselves.

### StepTemplate `spec.bindings` — hardcoded defaults only

Cannot reference other bindings. Use bash defaults for optional params:

```yaml
content: |
  SERVICE_NAME="${SERVICE_NAME:-$INGRESS_NAME}"
```

### `catch` blocks don't support `use`

Only inline operations work in `catch`. Cleanup templates must be called as regular steps.

### `assert` is strict

- Ports must be int, not string
- Don't assert partial arrays (must match full length)

### JMESPath limitations

No `??` operator. Use bash defaults instead.

## Writing a New L7 Test

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: my-test
  labels:
    all:
    test: my-test
    resource: httproute     # or: gateway, grpcroute, ingress
    layer: layer7
spec:
  steps:
    # 1. Deploy backend
    - name: deploy-backend
      use:
        template: ../../../../step-templates/common/deploy-echo-backend.yaml
        with:
          bindings:
            - name: backend_name
              value: my-echo
            - name: backend_message
              value: hello-from-my-test

    # 2. Create Gateway + wait for IP
    - name: create-gateway
      use:
        template: ../../../../step-templates/gateway/create-http-gateway.yaml
        with:
          bindings:
            - name: gateway_name
              value: my-gw

    # 3. Create HTTPRoute (needs Gateway IP for nip.io)
    - name: create-httproute
      cluster: tenant1
      try:
        - script:
            timeout: 60s
            content: |
              set -e
              IP=$(kubectl get gateway my-gw -n default \
                -o jsonpath='{.status.addresses[0].value}')
              HOST="my-test.${IP}.nip.io"
              cat <<EOF | kubectl apply -f -
              apiVersion: gateway.networking.k8s.io/v1
              kind: HTTPRoute
              metadata:
                name: my-route
                namespace: default
              spec:
                parentRefs:
                  - name: my-gw
                hostnames:
                  - "${HOST}"
                rules:
                  - backendRefs:
                      - name: my-echo
                        port: 80
              EOF
            check:
              ($error == null): true

    # 4. Verify Route CRD
    - name: verify-route-crd
      use:
        template: ../../../../step-templates/common/verify-route-crd.yaml
        with:
          bindings:
            - name: resource_name
              value: my-route
            - name: expected_kind
              value: HTTPRoute.gateway.networking.k8s.io
            - name: kubelb_namespace
              value: tenant-primary

    # 5. Verify HTTP
    - name: verify-http
      cluster: tenant1
      try:
        - script:
            timeout: 60s
            content: |
              set -e
              IP=$(kubectl get gateway my-gw -n default \
                -o jsonpath='{.status.addresses[0].value}')
              HOST="my-test.${IP}.nip.io"
              for i in $(seq 1 10); do
                RESPONSE=$(curl -s --max-time 5 -H "Host: ${HOST}" "http://${IP}/" 2>/dev/null || true)
                if echo "$RESPONSE" | grep -q "hello-from-my-test"; then
                  echo "OK"
                  exit 0
                fi
                sleep 2
              done
              exit 1
            check:
              ($error == null): true

    # 6. Cleanup + verify
    - name: cleanup
      cluster: tenant1
      try:
        - script:
            timeout: 60s
            content: |
              kubectl delete httproute my-route -n default --wait=false
            check:
              ($error == null): true
      finally:
        - script:
            cluster: tenant1
            timeout: 60s
            content: |
              kubectl delete httproute/my-route gateway/my-gw deployment/my-echo service/my-echo -n default --ignore-not-found

    - name: verify-cleanup
      use:
        template: ../../../../step-templates/common/verify-route-cleanup.yaml
        with:
          bindings:
            - name: resource_name
              value: my-route
            - name: kubelb_namespace
              value: tenant-primary
```

## Available Step Templates

### Common

| Template | Purpose | Required Bindings |
|----------|---------|-------------------|
| `common/deploy-echo-backend.yaml` | Deploy echo server + ClusterIP + readiness | `backend_name`, `backend_message` |
| `common/verify-route-crd.yaml` | Assert Route CRD exists with labels | `resource_name`, `expected_kind`, `kubelb_namespace` |
| `common/verify-route-cleanup.yaml` | Assert Route CRD removed | `resource_name`, `kubelb_namespace` |
| `common/verify-http-response.yaml` | Verify HTTP response via Host header | `host`, `expected_response` |
| `common/verify-headers.yaml` | Verify response headers | `gateway_name`, `route_name`, `backend_name` |
| `common/cleanup-service.yaml` | Delete service + verify LB CRD removed | `service_name`, `tenant`, `kubelb_namespace` |

### Gateway API

| Template | Purpose | Required Bindings |
|----------|---------|-------------------|
| `gateway/create-http-gateway.yaml` | Create Gateway + wait for IP | `gateway_name` |
| `gateway/verify-route-crd.yaml` | Assert Route CRD for Gateway | `resource_name`, `kubelb_namespace` |

### Layer4

| Template | Purpose | Required Bindings |
|----------|---------|-------------------|
| `layer4/assert-lb-crd.yaml` | Assert LB CRD exists | `service_name`, `kubelb_namespace` |
| `layer4/assert-no-lb-crd.yaml` | Assert NO LB CRD exists | `service_name`, `kubelb_namespace` |
| `layer4/verify-lb-cleanup.yaml` | Assert LB CRD removed | `service_name`, `kubelb_namespace` |
| `layer4/verify-status-propagation.yaml` | Verify LB IP matches tenant service IP | `service_name`, `tenant`, `kubelb_namespace` |
| `layer4/verify-http-response.yaml` | Verify HTTP from LB service | `service_name`, `expected`, `port`, `tenant` |

### Layer7 Cleanup

| Template | Purpose | Required Bindings |
|----------|---------|-------------------|
| `layer7/cleanup-gateway.yaml` | Delete Gateway + verify Route CRD removed | `gateway_name`, `tenant`, `kubelb_namespace` |
| `layer7/cleanup-route.yaml` | Delete Route + verify CRD removed | `route_name`, `route_kind`, `tenant`, `kubelb_namespace` |

### Ingress

| Template | Purpose | Required Bindings |
|----------|---------|-------------------|
| `ingress/cleanup-ingress.yaml` | Delete Ingress + verify Route CRD removed | `ingress_name` |
| `ingress/verify-ingress-status.yaml` | Assert Ingress has LB IP | `ingress_name` |
| `ingress/verify-route-crd.yaml` | Assert Route CRD for Ingress | `ingress_name`, `kubelb_namespace` |

### Conversion

| Template | Purpose | Required Bindings |
|----------|---------|-------------------|
| `conversion/cleanup-conversion.yaml` | Delete Ingress + service on standalone | `ingress_name` |
| `conversion/verify-gateway.yaml` | Assert Gateway exists | `gateway_name` |
| `conversion/verify-httproute.yaml` | Assert HTTPRoute with host | `ingress_name`, `expected_host` |
| `conversion/verify-http-traffic.yaml` | Verify HTTP via Gateway | `host`, `expected_response` |
| `conversion/verify-status-annotation.yaml` | Check conversion-status annotation | `ingress_name`, `expected_status` |
| `conversion/verify-policy.yaml` | Check policy resource created | `policy_name`, `policy_kind` |
| `conversion/verify-redirect.yaml` | Verify HTTP redirect behavior | `host`, `expected_code` |
| `conversion/verify-warnings.yaml` | Check warning events | `ingress_name`, `expected_warning` |
| `conversion/verify-skip-annotations.yaml` | Check skip-conversion annotation | `ingress_name` |

### Isolated

| Template | Purpose | Required Bindings |
|----------|---------|-------------------|
| `isolated/reset-configs.yaml` | Reset Config CRD to defaults | `manifestsPath` |
| `isolated/reset-tenants.yaml` | Reset Tenant CRDs to defaults | `manifestsPath` |
| `isolated/patch-config.yaml` | Patch Config CRD | `patch_json` |
| `isolated/patch-tenant.yaml` | Patch Tenant CRD | `tenant_name`, `patch_json` |

## Debugging Failures

Run a failing test in isolation:

```bash
make e2e-select select=test=basic
```

Kubeconfigs in `.e2e-kubeconfigs/`:

```bash
export KUBELB_KUBECONFIG=.e2e-kubeconfigs/kubelb.kubeconfig
export TENANT1_KUBECONFIG=.e2e-kubeconfigs/tenant1.kubeconfig
```

Check logs:

```bash
kubectl --kubeconfig=$TENANT1_KUBECONFIG logs -n kubelb -l app.kubernetes.io/name=kubelb-ccm -f
kubectl --kubeconfig=$KUBELB_KUBECONFIG logs -n kubelb -l app.kubernetes.io/name=kubelb-manager -f
```

Check resources:

```bash
kubectl --kubeconfig=$KUBELB_KUBECONFIG get routes.kubelb.k8c.io -A
kubectl --kubeconfig=$KUBELB_KUBECONFIG get gateway,httproute -A
kubectl --kubeconfig=$TENANT1_KUBECONFIG get gateway,httproute -n default
```
