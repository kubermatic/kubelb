/*
Copyright 2026 The KubeLB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package annotations

import (
	"fmt"
	"strings"

	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// handleRewriteTarget converts rewrite-target annotation to URLRewrite filter
// nginx.ingress.kubernetes.io/rewrite-target: /
// nginx.ingress.kubernetes.io/rewrite-target: /$2
func handleRewriteTarget(key, value string, annotations map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, []string{fmt.Sprintf("annotation %q has empty value", key)}
	}

	var warnings []string

	// Check for capture groups (e.g., /$1, /$2) which aren't directly supported
	if containsCaptureGroup(value) {
		warnings = append(warnings, fmt.Sprintf("annotation %q uses capture groups (%s) which require regex path matching; converted to simple prefix replacement", key, value))
		// Strip capture groups for basic conversion
		value = stripCaptureGroups(value)
	}

	// Check if use-regex is enabled
	if useRegex, ok := annotations[UseRegex]; ok && useRegex == boolTrue {
		warnings = append(warnings, "annotation use-regex is not fully supported; regex patterns may not work as expected")
	}

	filter := gwapiv1.HTTPRouteFilter{
		Type: gwapiv1.HTTPRouteFilterURLRewrite,
		URLRewrite: &gwapiv1.HTTPURLRewriteFilter{
			Path: &gwapiv1.HTTPPathModifier{
				Type:               gwapiv1.PrefixMatchHTTPPathModifier,
				ReplacePrefixMatch: &value,
			},
		},
	}

	return []gwapiv1.HTTPRouteFilter{filter}, warnings
}

// handleAppRoot generates warning for app-root annotation
// nginx.ingress.kubernetes.io/app-root: /app
// NGINX only redirects "/" path to the specified path, but Gateway API filters
// apply to ALL matched routes. Creating a separate HTTPRoute rule for "/" is required.
func handleAppRoot(key, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, []string{fmt.Sprintf("annotation %q has empty value", key)}
	}

	// Ensure path starts with /
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}

	// app-root in NGINX only redirects "/" to the specified path.
	// Gateway API filters apply to ALL routes, not just "/".
	// Creating a redirect filter would incorrectly redirect all paths.
	// User must manually create a separate HTTPRoute rule for path "/" with redirect.
	warning := fmt.Sprintf(
		"app-root=%q requires manual HTTPRoute: create separate rule with path=\"/\" (Exact) "+
			"and RequestRedirect filter to %q; cannot auto-convert as filter would apply to all paths",
		value, value,
	)

	return nil, []string{warning}
}

// handleUseRegex handles use-regex annotation.
// nginx.ingress.kubernetes.io/use-regex: "true"
// When true, paths are converted to RegularExpression path type.
func handleUseRegex(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value != boolTrue {
		return nil, nil
	}

	// use-regex=true is now converted to pathType: RegularExpression.
	// Note: RegularExpression support is implementation-specific in Gateway API.
	return nil, []string{"use-regex=true converted to pathType: RegularExpression (support is implementation-specific)"}
}

// containsCaptureGroup checks if the value contains regex capture group references
func containsCaptureGroup(value string) bool {
	for i := 0; i < len(value)-1; i++ {
		if value[i] == '$' && value[i+1] >= '0' && value[i+1] <= '9' {
			return true
		}
	}
	return false
}

// stripCaptureGroups removes capture group references from the value
func stripCaptureGroups(value string) string {
	result := strings.Builder{}
	i := 0
	for i < len(value) {
		if i < len(value)-1 && value[i] == '$' && value[i+1] >= '0' && value[i+1] <= '9' {
			// Skip the capture group reference
			i += 2
			// Skip any additional digits
			for i < len(value) && value[i] >= '0' && value[i] <= '9' {
				i++
			}
			continue
		}
		result.WriteByte(value[i])
		i++
	}

	stripped := result.String()
	if stripped == "" {
		return "/"
	}
	return stripped
}
