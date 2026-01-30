# Guidelines for Writing E2E Tests

## Resource creation

Use `apply` to create resources, whereever possible and duplicate resources should be moved to step templates. Only use script to create resources if it's really neccessary and can't be done easily with `apply`.

## Code duplication

Avoid code duplication. If a resource is used in multiple tests, it should be moved to a step template.

## Cleanup

Always use `finally` to cleanup resources.

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
