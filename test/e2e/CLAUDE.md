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

All conversion step templates hardcode `NAMESPACE=default` in their script content for this reason.

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

## JMESPath Limitations

Chainsaw's JMESPath does NOT support:
- Null coalescing operator `??` (causes "Unknown char: '?'" error)
- Many common operators from other JMESPath implementations

Use bash defaults instead of JMESPath for optional values.

## Conversion Tests: Single Cluster Setup

Conversion tests run against a single cluster. The Makefile sets `--kube-config` to make the conversion cluster the default, so tests don't need to specify `cluster: conversion` on every step.

The step templates still use `cluster: conversion` explicitly for clarity, but inline test steps can omit it since conversion is the default kubeconfig.
