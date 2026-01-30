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
