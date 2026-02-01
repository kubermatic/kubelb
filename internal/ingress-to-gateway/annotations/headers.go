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

// handleProxySetHeaders converts proxy-set-headers to RequestHeaderModifier
// Format: "Header1:Value1,Header2:Value2" or ConfigMap reference
func handleProxySetHeaders(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	var warnings []string

	// Check if it's a ConfigMap reference (namespace/name format)
	if strings.Contains(value, "/") && !strings.Contains(value, ":") {
		warnings = append(warnings, fmt.Sprintf("proxy-set-headers references ConfigMap %q; manual conversion required", value))
		return nil, warnings
	}

	headers, parseWarnings := parseHeaderList(value)
	warnings = append(warnings, parseWarnings...)

	if len(headers) == 0 {
		return nil, warnings
	}

	filter := gwapiv1.HTTPRouteFilter{
		Type: gwapiv1.HTTPRouteFilterRequestHeaderModifier,
		RequestHeaderModifier: &gwapiv1.HTTPHeaderFilter{
			Set: headers,
		},
	}

	return []gwapiv1.HTTPRouteFilter{filter}, warnings
}

// handleCustomHeaders converts custom-headers to ResponseHeaderModifier
// Format: "Header1:Value1,Header2:Value2" or ConfigMap reference
func handleCustomHeaders(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	var warnings []string

	// Check if it's a ConfigMap reference
	if strings.Contains(value, "/") && !strings.Contains(value, ":") {
		warnings = append(warnings, fmt.Sprintf("custom-headers references ConfigMap %q; manual conversion required", value))
		return nil, warnings
	}

	headers, parseWarnings := parseHeaderList(value)
	warnings = append(warnings, parseWarnings...)

	if len(headers) == 0 {
		return nil, warnings
	}

	filter := gwapiv1.HTTPRouteFilter{
		Type: gwapiv1.HTTPRouteFilterResponseHeaderModifier,
		ResponseHeaderModifier: &gwapiv1.HTTPHeaderFilter{
			Add: headers,
		},
	}

	return []gwapiv1.HTTPRouteFilter{filter}, warnings
}

// handleUpstreamVhost converts upstream-vhost annotation to RequestHeaderModifier
// nginx.ingress.kubernetes.io/upstream-vhost: "internal.example.com"
// Sets the Host header sent to the upstream server
func handleUpstreamVhost(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	filter := gwapiv1.HTTPRouteFilter{
		Type: gwapiv1.HTTPRouteFilterRequestHeaderModifier,
		RequestHeaderModifier: &gwapiv1.HTTPHeaderFilter{
			Set: []gwapiv1.HTTPHeader{
				{
					Name:  "Host",
					Value: value,
				},
			},
		},
	}

	return []gwapiv1.HTTPRouteFilter{filter}, nil
}

// handleXForwardedPrefix converts x-forwarded-prefix annotation to RequestHeaderModifier
// nginx.ingress.kubernetes.io/x-forwarded-prefix: "/api"
// Sets the X-Forwarded-Prefix header
func handleXForwardedPrefix(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	filter := gwapiv1.HTTPRouteFilter{
		Type: gwapiv1.HTTPRouteFilterRequestHeaderModifier,
		RequestHeaderModifier: &gwapiv1.HTTPHeaderFilter{
			Set: []gwapiv1.HTTPHeader{
				{
					Name:  "X-Forwarded-Prefix",
					Value: value,
				},
			},
		},
	}

	return []gwapiv1.HTTPRouteFilter{filter}, nil
}

// handlePreserveHost handles the preserve-host annotation
// nginx.ingress.kubernetes.io/preserve-host: "true" (default) or "false"
// When true (default), the original Host header is preserved
// When false, the Host header should be rewritten to the service name
func handlePreserveHost(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value != "false" {
		return nil, nil // default is true, no action needed
	}
	warning := "preserve-host=false requires URLRewrite filter with Hostname at backend level; " +
		"add filter to BackendRef: filters: [{type: URLRewrite, urlRewrite: {hostname: <service-name>}}]"
	return nil, []string{warning}
}

// parseHeaderList parses "Header1:Value1,Header2:Value2" format
func parseHeaderList(value string) ([]gwapiv1.HTTPHeader, []string) {
	var headers []gwapiv1.HTTPHeader
	var warnings []string

	pairs := strings.Split(value, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			warnings = append(warnings, fmt.Sprintf("invalid header format %q, expected 'Name:Value'", pair))
			continue
		}

		name := strings.TrimSpace(parts[0])
		headerValue := strings.TrimSpace(parts[1])

		if name == "" {
			warnings = append(warnings, fmt.Sprintf("empty header name in %q", pair))
			continue
		}

		headers = append(headers, gwapiv1.HTTPHeader{
			Name:  gwapiv1.HTTPHeaderName(name),
			Value: headerValue,
		})
	}

	return headers, warnings
}

// handleHSTS converts HSTS annotations to a Strict-Transport-Security response header.
// nginx.ingress.kubernetes.io/hsts: "true"
// nginx.ingress.kubernetes.io/hsts-max-age: "31536000" (default)
// nginx.ingress.kubernetes.io/hsts-include-subdomains: "true"
// nginx.ingress.kubernetes.io/hsts-preload: "true"
func handleHSTS(_, value string, annotations map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value != boolTrue {
		return nil, nil
	}

	// Build Strict-Transport-Security header value
	headerValue := buildHSTSHeaderValue(annotations)

	filter := gwapiv1.HTTPRouteFilter{
		Type: gwapiv1.HTTPRouteFilterResponseHeaderModifier,
		ResponseHeaderModifier: &gwapiv1.HTTPHeaderFilter{
			Set: []gwapiv1.HTTPHeader{
				{
					Name:  "Strict-Transport-Security",
					Value: headerValue,
				},
			},
		},
	}

	return []gwapiv1.HTTPRouteFilter{filter}, nil
}

// handleHSTSAnnotation handles auxiliary HSTS annotations (max-age, include-subdomains, preload).
// These are processed as part of handleHSTS and don't generate separate filters.
func handleHSTSAnnotation(_, _ string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	// Processed as part of handleHSTS; no standalone action
	return nil, nil
}

// buildHSTSHeaderValue constructs the Strict-Transport-Security header value from annotations.
func buildHSTSHeaderValue(annotations map[string]string) string {
	// Default max-age is 31536000 (1 year) as per nginx-ingress defaults
	maxAge := "31536000"
	if v, ok := annotations[HSTSMaxAge]; ok && v != "" {
		maxAge = v
	}

	value := "max-age=" + maxAge

	// Include subdomains if enabled
	if v, ok := annotations[HSTSIncludeSubdomains]; ok && v == "true" {
		value += "; includeSubDomains"
	}

	// Add preload if enabled
	if v, ok := annotations[HSTSPreload]; ok && v == "true" {
		value += "; preload"
	}

	return value
}
