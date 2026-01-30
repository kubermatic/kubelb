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
