# Guidelines for Writing E2E Tests

## Resource creation

Use `apply` to create resources, whereever possible and duplicate resources should be moved to step templates. Only use script to create resources if it's really neccessary and can't be done easily with `apply`.

## Code duplication

Avoid code duplication. If a resource is used in multiple tests, it should be moved to a step template.

## Cleanup

Always use `finally` to cleanup resources.

## Reserved Variable Names & Namespace Handling

**CRITICAL**: Chainsaw has a built-in `$namespace` variable that it sets to the auto-generated test namespace (e.g., `chainsaw-diverse-emu`). This OVERRIDES any user-defined binding with namespace-related names, including `namespace` and sometimes even `target_namespace` due to variable scoping issues.

**Solution**: Follow layer4/layer7 pattern - **hardcode namespace directly in scripts**:
```yaml
# WRONG - chainsaw may override this
env:
  - name: NAMESPACE
    value: ($target_namespace)  # unreliable

# CORRECT - hardcode in script content
content: |
  set -e
  NAMESPACE=default
  kubectl get ingress -n "$NAMESPACE" ...
```

All standalone step templates hardcode `NAMESPACE=default` in their script content for this reason.

## StepTemplate rules

**IMPORTANT**: StepTemplates MUST use `try`, NOT `finally`. The `finally` keyword is invalid in StepTemplate spec.

```yaml
# CORRECT - StepTemplate with try
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: StepTemplate
spec:
  try:
    - script: ...

# WRONG - StepTemplate with finally (will fail validation)
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: StepTemplate
spec:
  finally:  # ERROR: spec.try: Required value
    - script: ...
```

If you need cleanup behavior, the TEST that uses the template should wrap it in `finally`, not the template itself.

## StepTemplate Bindings

**CRITICAL**: In StepTemplate `spec.bindings`, you can only use hardcoded default values. You CANNOT reference other bindings because they're not in scope when `spec.bindings` is evaluated.

```yaml
# WRONG - $ingress_name not in scope, causes "variable not defined" error
spec:
  bindings:
    - name: service_name
      value: ($ingress_name)  # ERROR!

# CORRECT - hardcoded defaults only
spec:
  bindings:
    - name: gateway_name
      value: kubelb
    - name: min_listeners
      value: "1"
```

For optional parameters that should default to another binding's value, handle it in bash:
```yaml
spec:
  try:
    - script:
        env:
          - name: INGRESS_NAME
            value: ($ingress_name)
        content: |
          # Default SERVICE_NAME to INGRESS_NAME if not set
          SERVICE_NAME="${SERVICE_NAME:-$INGRESS_NAME}"
```

## use.with.bindings — Pass Hardcoded Values

**CRITICAL**: When calling a StepTemplate via `use.with.bindings`, always pass **hardcoded values**, NOT JMESPath references to test-level bindings. The binding expression may silently resolve to empty.

```yaml
# WRONG - ($gateway_name) may resolve to empty string
- name: wait-gateway-ready
  use:
    template: ../../../../step-templates/gateway/verify-gateway-status.yaml
    with:
      bindings:
        - name: gateway_name
          value: ($gateway_name)  # SILENTLY EMPTY!

# CORRECT - hardcoded value
- name: wait-gateway-ready
  use:
    template: ../../../../step-templates/gateway/verify-gateway-status.yaml
    with:
      bindings:
        - name: gateway_name
          value: headers-gw

# ALSO CORRECT - inline the script instead of using a template
- name: wait-gateway-ready
  cluster: tenant1
  try:
    - script:
        content: |
          GATEWAY_NAME="xff-gw"
          # ...
```

## JMESPath Limitations

Chainsaw's JMESPath does NOT support:
- Null coalescing operator `??` (causes "Unknown char: '?'" error)
- Many common operators from other JMESPath implementations

Use bash defaults instead of JMESPath for optional values.

## Cluster Configuration

The e2e tests use 4 Kind clusters:

- **kubelb**: Manager cluster (single-node)
- **tenant1**: Normal CCM hub-and-spoke (multi-node, 3 workers) - default for most tests
- **tenant2**: Normal CCM hub-and-spoke (single-node) - for single-node edge cases
- **standalone**: Standalone conversion CCM (single-node) - for Ingress-to-Gateway conversion tests

## Conversion Tests

Conversion tests are located in `tests/layer7/conversion/` and run against the `standalone` cluster. They use the `suite: conversion` label for selective execution:

```bash
make e2e-select select=suite=conversion  # Run only conversion tests
```

The step templates use `cluster: standalone` explicitly to target the standalone cluster.

## Available Step Templates

### Common

| Template | Purpose | Required Bindings |
|----------|---------|-------------------|
| `common/deploy-echo-backend.yaml` | Deploy echo server + ClusterIP service + readiness check | `backend_name`, `backend_message` |
| `common/verify-route-crd.yaml` | Assert Route CRD exists with labels | `resource_name`, `expected_kind`, `kubelb_namespace` |
| `common/verify-route-cleanup.yaml` | Assert Route CRD removed | `resource_name`, `kubelb_namespace` |
| `common/verify-http-response.yaml` | Verify HTTP response via Host header | `host`, `expected_response` |
| `common/verify-headers.yaml` | Verify response headers | `gateway_name`, `route_name`, `backend_name` |
| `common/cleanup-service.yaml` | Delete service + verify LB CRD removed | `service_name`, `tenant`, `kubelb_namespace` |

### Gateway API

| Template | Purpose | Required Bindings |
|----------|---------|-------------------|
| `gateway/create-http-gateway.yaml` | Create Gateway + wait for IP | `gateway_name` |
| `gateway/verify-route-crd.yaml` | Assert Route CRD for Gateway resource | `resource_name`, `kubelb_namespace` |

### Layer4

| Template | Purpose | Required Bindings |
|----------|---------|-------------------|
| `layer4/assert-lb-crd.yaml` | Assert LB CRD exists with labels+finalizer | `service_name`, `kubelb_namespace` |
| `layer4/assert-no-lb-crd.yaml` | Assert NO LB CRD exists | `service_name`, `kubelb_namespace` |
| `layer4/verify-lb-cleanup.yaml` | Assert LB CRD removed | `service_name`, `kubelb_namespace` |
| `layer4/verify-status-propagation.yaml` | Verify LB IP matches tenant service IP | `service_name`, `tenant`, `kubelb_namespace` |
| `layer4/verify-http-response.yaml` | Verify HTTP from LB service | `service_name`, `expected`, `port`, `tenant` |

### Layer7 Cleanup

| Template | Purpose | Required Bindings |
|----------|---------|-------------------|
| `layer7/cleanup-gateway.yaml` | Delete Gateway + verify Route CRD removed | `gateway_name`, `tenant`, `kubelb_namespace` |
| `layer7/cleanup-route.yaml` | Delete any Route kind + verify CRD removed | `route_name`, `route_kind`, `tenant`, `kubelb_namespace` |

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

## Manual Test Debugging

For debugging test failures manually (instead of using chainsaw), create resources from the test directly and inspect logs.

### Kubeconfig Files

E2E kubeconfigs are stored in `.e2e-kubeconfigs/`:

```bash
export KUBELB_KUBECONFIG=.e2e-kubeconfigs/kubelb.kubeconfig
export TENANT1_KUBECONFIG=.e2e-kubeconfigs/tenant1.kubeconfig
export TENANT2_KUBECONFIG=.e2e-kubeconfigs/tenant2.kubeconfig
export STANDALONE_KUBECONFIG=.e2e-kubeconfigs/standalone.kubeconfig
```

### Manual Debugging Steps

1. Apply resources from the test file manually:
   ```bash
   kubectl --kubeconfig=$TENANT2_KUBECONFIG apply -f <resource.yaml>
   ```

2. Check CCM logs:
   ```bash
   kubectl --kubeconfig=$TENANT1_KUBECONFIG logs -n kubelb -l app.kubernetes.io/name=kubelb-ccm -f
   ```

3. Check KubeLB manager logs:
   ```bash
   kubectl --kubeconfig=$KUBELB_KUBECONFIG logs -n kubelb -l app.kubernetes.io/name=kubelb-manager -f
   ```

4. Check Route CRDs in kubelb cluster:
   ```bash
   kubectl --kubeconfig=$KUBELB_KUBECONFIG get routes.kubelb.k8c.io -A
   ```

5. Check envoy proxy status:
   ```bash
   kubectl --kubeconfig=$KUBELB_KUBECONFIG get pods -n <tenant-namespace>
   ```
