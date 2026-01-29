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

const defaultConfigEnabled = "enabled"

// handleAffinity handles session affinity annotations
// nginx.ingress.kubernetes.io/affinity: "cookie"
func handleAffinity(_, value string, annotations map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	if value != "cookie" {
		return nil, []string{fmt.Sprintf("affinity=%q is not supported; only 'cookie' affinity can be converted", value)}
	}

	// Collect session cookie config
	var config []string
	if name := annotations[SessionCookieName]; name != "" {
		config = append(config, fmt.Sprintf("name=%q", name))
	}
	if path := annotations[SessionCookiePath]; path != "" {
		config = append(config, fmt.Sprintf("path=%q", path))
	}
	if expires := annotations[SessionCookieExpires]; expires != "" {
		config = append(config, fmt.Sprintf("ttl=%s", expires))
	}
	if sameSite := annotations[SessionCookieSameSite]; sameSite != "" {
		config = append(config, fmt.Sprintf("sameSite=%s", sameSite))
	}

	configStr := defaultConfigEnabled
	if len(config) > 0 {
		configStr = strings.Join(config, ", ")
	}

	warning := fmt.Sprintf(
		"affinity=cookie requires BackendTrafficPolicy: spec.sessionPersistence.type=Cookie, cookie={%s}",
		configStr,
	)
	return nil, []string{warning}
}

// handleSessionCookieAnnotation is a no-op for individual session cookie annotations
func handleSessionCookieAnnotation(_, _ string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	// Individual session cookie annotations are processed by handleAffinity
	return nil, nil
}

// handleAuthType handles auth-type annotation
// nginx.ingress.kubernetes.io/auth-type: "basic"
func handleAuthType(_, value string, annotations map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	switch value {
	case "basic":
		secret := annotations[AuthSecret]
		realm := annotations[AuthRealm]

		var config []string
		if secret != "" {
			config = append(config, fmt.Sprintf("secretRef=%q", secret))
		}
		if realm != "" {
			config = append(config, fmt.Sprintf("realm=%q", realm))
		}

		configStr := defaultConfigEnabled
		if len(config) > 0 {
			configStr = strings.Join(config, ", ")
		}

		warning := fmt.Sprintf(
			"auth-type=basic requires SecurityPolicy: spec.basicAuth={%s}",
			configStr,
		)
		return nil, []string{warning}

	case "digest":
		return nil, []string{"auth-type=digest is not supported in Gateway API; consider using basic auth or external auth"}

	default:
		return nil, []string{fmt.Sprintf("auth-type=%q is not supported", value)}
	}
}

// handleAuthURL handles external auth annotation
// nginx.ingress.kubernetes.io/auth-url: "https://auth.example.com/verify"
func handleAuthURL(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	warning := fmt.Sprintf(
		"auth-url=%q requires SecurityPolicy: spec.extAuth.http.url=%q",
		value, value,
	)
	return nil, []string{warning}
}

// handleAuthAnnotation is a no-op for supporting auth annotations
func handleAuthAnnotation(_, _ string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	// Supporting auth annotations (auth-secret, auth-realm) are processed by handleAuthType
	return nil, nil
}

// handleBackendProtocol handles backend-protocol annotation
// nginx.ingress.kubernetes.io/backend-protocol: "GRPC"
func handleBackendProtocol(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	value = strings.ToUpper(value)

	switch value {
	case "GRPC", "GRPCS":
		return nil, []string{fmt.Sprintf(
			"backend-protocol=%s indicates this Ingress should be converted to GRPCRoute instead of HTTPRoute",
			value,
		)}

	case "HTTPS":
		return nil, []string{
			"backend-protocol=HTTPS requires BackendTLSPolicy to configure TLS to the backend",
		}

	case "HTTP":
		// Default, no action needed
		return nil, nil

	case "FCGI":
		return nil, []string{"backend-protocol=FCGI is not supported in Gateway API"}

	default:
		return nil, []string{fmt.Sprintf("backend-protocol=%q is not recognized", value)}
	}
}

// handleCanaryEnabled handles canary annotation (already exists but enhance it)
func handleCanaryEnabled(_, value string, annotations map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value != boolTrue {
		return nil, nil
	}

	var config []string

	// Check for weight-based canary
	if weight := annotations[CanaryWeight]; weight != "" {
		config = append(config, fmt.Sprintf("weight=%s", weight))
		if total := annotations[CanaryWeightTotal]; total != "" {
			config = append(config, fmt.Sprintf("total=%s", total))
		}
	}

	// Check for header-based canary
	if header := annotations[CanaryByHeader]; header != "" {
		headerVal := annotations[CanaryByHeaderValue]
		if headerVal != "" {
			config = append(config, fmt.Sprintf("header=%s:%s", header, headerVal))
		} else {
			config = append(config, fmt.Sprintf("header=%s", header))
		}
	}

	// Check for cookie-based canary
	if cookie := annotations[CanaryByCookie]; cookie != "" {
		config = append(config, fmt.Sprintf("cookie=%s", cookie))
	}

	var warnings []string

	if len(config) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"canary ingress with {%s} should be converted to HTTPRoute with weighted backends or header matches",
			strings.Join(config, ", "),
		))
	} else {
		warnings = append(warnings, "canary ingresses should be converted to weighted HTTPRoute backends")
	}

	return nil, warnings
}

// handleCanaryAnnotation is a no-op for supporting canary annotations
func handleCanaryAnnotation(_, _ string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	// Supporting canary annotations are processed by handleCanaryEnabled
	return nil, nil
}
