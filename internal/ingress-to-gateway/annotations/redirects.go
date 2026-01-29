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
	"net/url"
	"strconv"

	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// handleSSLRedirect converts ssl-redirect annotation to RequestRedirect filter
// nginx.ingress.kubernetes.io/ssl-redirect: "true"
// Redirects HTTP requests to HTTPS
func handleSSLRedirect(_, value string, annotations map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	// Only process if value is "true"
	if value != boolTrue {
		return nil, nil
	}

	// Check if force-ssl-redirect is also set - it takes precedence
	if forceSSL, ok := annotations[ForceSSLRedirect]; ok && forceSSL == boolTrue {
		// Skip, force-ssl-redirect will handle it
		return nil, nil
	}

	return createHTTPSRedirectFilter(301), nil
}

// handleForceSSLRedirect converts force-ssl-redirect annotation
// nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
// Forces redirect to HTTPS even without TLS termination
func handleForceSSLRedirect(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value != boolTrue {
		return nil, nil
	}

	return createHTTPSRedirectFilter(301), nil
}

// handlePermanentRedirect converts permanent-redirect annotation
// nginx.ingress.kubernetes.io/permanent-redirect: "https://example.com/new-path"
// Returns 301 redirect to the specified URL
func handlePermanentRedirect(key, value string, annotations map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, []string{fmt.Sprintf("annotation %q has empty value", key)}
	}

	// Parse the redirect URL
	parsedURL, err := url.Parse(value)
	if err != nil {
		return nil, []string{fmt.Sprintf("annotation %q has invalid URL %q: %v", key, value, err)}
	}

	// Determine status code (default 301, can be overridden)
	statusCode := 301
	if codeStr, ok := annotations[PermanentRedirectCode]; ok {
		if code, err := strconv.Atoi(codeStr); err == nil && isValidRedirectCode(code) {
			statusCode = code
		}
	}

	return createURLRedirectFilter(parsedURL, statusCode), nil
}

// handlePermanentRedirectCode is a no-op, handled by handlePermanentRedirect
func handlePermanentRedirectCode(_, _ string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	// This annotation modifies permanent-redirect behavior
	// The actual handling is done in handlePermanentRedirect
	return nil, nil
}

// handleTemporalRedirect converts temporal-redirect annotation
// nginx.ingress.kubernetes.io/temporal-redirect: "https://example.com/temp"
// Returns 302 redirect to the specified URL
func handleTemporalRedirect(key, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, []string{fmt.Sprintf("annotation %q has empty value", key)}
	}

	parsedURL, err := url.Parse(value)
	if err != nil {
		return nil, []string{fmt.Sprintf("annotation %q has invalid URL %q: %v", key, value, err)}
	}

	return createURLRedirectFilter(parsedURL, 302), nil
}

// createHTTPSRedirectFilter creates a RequestRedirect filter for HTTP->HTTPS
func createHTTPSRedirectFilter(statusCode int) []gwapiv1.HTTPRouteFilter {
	scheme := "https"
	code := statusCodeToGatewayAPI(statusCode)

	return []gwapiv1.HTTPRouteFilter{
		{
			Type: gwapiv1.HTTPRouteFilterRequestRedirect,
			RequestRedirect: &gwapiv1.HTTPRequestRedirectFilter{
				Scheme:     &scheme,
				StatusCode: &code,
			},
		},
	}
}

// createURLRedirectFilter creates a RequestRedirect filter for a full URL redirect
func createURLRedirectFilter(targetURL *url.URL, statusCode int) []gwapiv1.HTTPRouteFilter {
	code := statusCodeToGatewayAPI(statusCode)

	filter := gwapiv1.HTTPRouteFilter{
		Type: gwapiv1.HTTPRouteFilterRequestRedirect,
		RequestRedirect: &gwapiv1.HTTPRequestRedirectFilter{
			StatusCode: &code,
		},
	}

	// Set scheme if specified
	if targetURL.Scheme != "" {
		filter.RequestRedirect.Scheme = &targetURL.Scheme
	}

	// Set hostname if specified
	if targetURL.Host != "" {
		// Extract hostname without port
		hostname := targetURL.Hostname()
		gwHostname := gwapiv1.PreciseHostname(hostname)
		filter.RequestRedirect.Hostname = &gwHostname

		// Set port if specified
		if portStr := targetURL.Port(); portStr != "" {
			if port, err := strconv.Atoi(portStr); err == nil {
				gwPort := gwapiv1.PortNumber(port)
				filter.RequestRedirect.Port = &gwPort
			}
		}
	}

	// Set path if specified
	if targetURL.Path != "" && targetURL.Path != "/" {
		pathType := gwapiv1.FullPathHTTPPathModifier
		filter.RequestRedirect.Path = &gwapiv1.HTTPPathModifier{
			Type:            pathType,
			ReplaceFullPath: &targetURL.Path,
		}
	}

	return []gwapiv1.HTTPRouteFilter{filter}
}

// statusCodeToGatewayAPI converts an HTTP status code to Gateway API type
func statusCodeToGatewayAPI(code int) int {
	// Gateway API only supports 301 and 302
	switch code {
	case 301, 308:
		return 301
	case 302, 303, 307:
		return 302
	default:
		return 302
	}
}

// isValidRedirectCode checks if the status code is a valid redirect code
func isValidRedirectCode(code int) bool {
	return code >= 300 && code < 400
}
