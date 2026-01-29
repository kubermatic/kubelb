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
	"strconv"
	"strings"

	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// PolicySuggestion contains suggested policy configuration for an annotation
type PolicySuggestion struct {
	PolicyType string // e.g., "BackendTrafficPolicy", "SecurityPolicy"
	Field      string // e.g., "spec.timeout.http.requestTimeout"
	Value      string // The value to set
}

// handleProxyConnectTimeout suggests BackendTrafficPolicy for connect timeout
func handleProxyConnectTimeout(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	duration := normalizeTimeout(value)
	warning := fmt.Sprintf(
		"proxy-connect-timeout=%q requires BackendTrafficPolicy: spec.timeout.tcp.connectTimeout=%s",
		value, duration,
	)
	return nil, []string{warning}
}

// handleProxyReadTimeout suggests BackendTrafficPolicy for read timeout
func handleProxyReadTimeout(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	duration := normalizeTimeout(value)
	warning := fmt.Sprintf(
		"proxy-read-timeout=%q requires BackendTrafficPolicy: spec.timeout.http.requestTimeout=%s",
		value, duration,
	)
	return nil, []string{warning}
}

// handleProxySendTimeout suggests BackendTrafficPolicy for send timeout
func handleProxySendTimeout(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	duration := normalizeTimeout(value)
	warning := fmt.Sprintf(
		"proxy-send-timeout=%q requires BackendTrafficPolicy: spec.timeout.http.requestTimeout=%s",
		value, duration,
	)
	return nil, []string{warning}
}

// handleEnableCORS suggests SecurityPolicy for CORS configuration
func handleEnableCORS(_, value string, annotations map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value != boolTrue {
		return nil, nil
	}

	var corsConfig []string

	// Collect all CORS-related annotations
	if origin := annotations[CORSAllowOrigin]; origin != "" {
		corsConfig = append(corsConfig, fmt.Sprintf("allowOrigins: [%s]", formatStringList(origin)))
	}
	if methods := annotations[CORSAllowMethods]; methods != "" {
		corsConfig = append(corsConfig, fmt.Sprintf("allowMethods: [%s]", formatStringList(methods)))
	}
	if headers := annotations[CORSAllowHeaders]; headers != "" {
		corsConfig = append(corsConfig, fmt.Sprintf("allowHeaders: [%s]", formatStringList(headers)))
	}
	if expose := annotations[CORSExposeHeaders]; expose != "" {
		corsConfig = append(corsConfig, fmt.Sprintf("exposeHeaders: [%s]", formatStringList(expose)))
	}
	if maxAge := annotations[CORSMaxAge]; maxAge != "" {
		corsConfig = append(corsConfig, fmt.Sprintf("maxAge: %s", maxAge))
	}
	if creds := annotations[CORSAllowCredentials]; creds == boolTrue {
		corsConfig = append(corsConfig, "allowCredentials: true")
	}

	config := "enabled"
	if len(corsConfig) > 0 {
		config = strings.Join(corsConfig, ", ")
	}

	warning := fmt.Sprintf("enable-cors=true requires SecurityPolicy: spec.cors={%s}", config)
	return nil, []string{warning}
}

// handleCORSAnnotation is a no-op for individual CORS annotations (handled by enable-cors)
func handleCORSAnnotation(_, _ string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	// Individual CORS annotations are processed by handleEnableCORS
	return nil, nil
}

// handleLimitRPS suggests BackendTrafficPolicy for rate limiting
func handleLimitRPS(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	rps, err := strconv.Atoi(value)
	if err != nil {
		return nil, []string{fmt.Sprintf("limit-rps=%q is not a valid number", value)}
	}

	warning := fmt.Sprintf(
		"limit-rps=%d requires BackendTrafficPolicy: spec.rateLimit.local.requests=%d, unit=Second",
		rps, rps,
	)
	return nil, []string{warning}
}

// handleLimitConnections suggests BackendTrafficPolicy for connection limiting
func handleLimitConnections(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	conns, err := strconv.Atoi(value)
	if err != nil {
		return nil, []string{fmt.Sprintf("limit-connections=%q is not a valid number", value)}
	}

	warning := fmt.Sprintf(
		"limit-connections=%d requires BackendTrafficPolicy: spec.circuitBreaker.maxConnections=%d",
		conns, conns,
	)
	return nil, []string{warning}
}

// handleWhitelistSourceRange suggests SecurityPolicy for IP allowlist
func handleWhitelistSourceRange(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	cidrs := formatStringList(value)
	warning := fmt.Sprintf(
		"whitelist-source-range=%q requires SecurityPolicy: spec.authorization.rules[].action=Allow, from.source.cidr=[%s]",
		value, cidrs,
	)
	return nil, []string{warning}
}

// handleDenylistSourceRange suggests SecurityPolicy for IP denylist
func handleDenylistSourceRange(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	cidrs := formatStringList(value)
	warning := fmt.Sprintf(
		"denylist-source-range=%q requires SecurityPolicy: spec.authorization.rules[].action=Deny, from.source.cidr=[%s]",
		value, cidrs,
	)
	return nil, []string{warning}
}

// handleProxyBodySize suggests ClientTrafficPolicy for body size limit
func handleProxyBodySize(_, value string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	if value == "" {
		return nil, nil
	}

	warning := fmt.Sprintf(
		"proxy-body-size=%q requires ClientTrafficPolicy: spec.clientConfig.requestBodySize=%s",
		value, value,
	)
	return nil, []string{warning}
}

// normalizeTimeout converts NGINX timeout format (e.g., "60", "60s") to duration string
func normalizeTimeout(value string) string {
	value = strings.TrimSpace(value)

	// Already has a unit suffix
	if strings.HasSuffix(value, "s") || strings.HasSuffix(value, "m") || strings.HasSuffix(value, "h") {
		return value
	}

	// Plain number = seconds
	if _, err := strconv.Atoi(value); err == nil {
		return value + "s"
	}

	return value
}

// formatStringList converts comma-separated string to quoted list
func formatStringList(value string) string {
	parts := strings.Split(value, ",")
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			quoted = append(quoted, fmt.Sprintf("%q", p))
		}
	}
	return strings.Join(quoted, ", ")
}
