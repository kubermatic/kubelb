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

	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const boolTrue = "true"

// AnnotationConversionResult holds the converted filters and any warnings
type AnnotationConversionResult struct {
	// Filters contains HTTPRoute filters generated from annotations
	Filters []gwapiv1.HTTPRouteFilter
	// Warnings contains messages about annotations that couldn't be converted
	Warnings []string
	// Processed contains annotation keys that were processed (converted or warned)
	Processed []string
}

// Converter converts NGINX Ingress annotations to Gateway API filters
type Converter struct {
	// handlers maps annotation keys to their conversion functions
	handlers map[string]AnnotationHandler
}

// AnnotationHandler processes a single annotation and returns filters/warnings
type AnnotationHandler func(key, value string, annotations map[string]string) ([]gwapiv1.HTTPRouteFilter, []string)

// NewConverter creates a new annotation converter with all registered handlers
func NewConverter() *Converter {
	c := &Converter{
		handlers: make(map[string]AnnotationHandler),
	}
	c.registerHandlers()
	return c
}

// registerHandlers registers all annotation handlers
func (c *Converter) registerHandlers() {
	// Redirects
	c.handlers[SSLRedirect] = handleSSLRedirect
	c.handlers[ForceSSLRedirect] = handleForceSSLRedirect
	c.handlers[PermanentRedirect] = handlePermanentRedirect
	c.handlers[PermanentRedirectCode] = handlePermanentRedirectCode
	c.handlers[TemporalRedirect] = handleTemporalRedirect

	// URL rewriting
	c.handlers[RewriteTarget] = handleRewriteTarget
	c.handlers[AppRoot] = handleAppRoot

	// Headers
	c.handlers[CustomHeaders] = handleCustomHeaders
	c.handlers[ProxySetHeaders] = handleProxySetHeaders

	// Timeouts
	c.handlers[ProxyConnectTimeout] = handleProxyConnectTimeout
	c.handlers[ProxyReadTimeout] = handleProxyReadTimeout
	c.handlers[ProxySendTimeout] = handleProxySendTimeout

	// CORS
	c.handlers[EnableCORS] = handleEnableCORS
	c.handlers[CORSAllowOrigin] = handleCORSAnnotation
	c.handlers[CORSAllowMethods] = handleCORSAnnotation
	c.handlers[CORSAllowHeaders] = handleCORSAnnotation
	c.handlers[CORSExposeHeaders] = handleCORSAnnotation
	c.handlers[CORSAllowCredentials] = handleCORSAnnotation
	c.handlers[CORSMaxAge] = handleCORSAnnotation

	// Rate limiting
	c.handlers[LimitRPS] = handleLimitRPS
	c.handlers[LimitConnections] = handleLimitConnections

	// IP access control
	c.handlers[WhitelistSourceRange] = handleWhitelistSourceRange
	c.handlers[DenylistSourceRange] = handleDenylistSourceRange

	// Request size
	c.handlers[ProxyBodySize] = handleProxyBodySize

	// Session affinity
	c.handlers[Affinity] = handleAffinity
	c.handlers[SessionCookieName] = handleSessionCookieAnnotation
	c.handlers[SessionCookiePath] = handleSessionCookieAnnotation
	c.handlers[SessionCookieExpires] = handleSessionCookieAnnotation
	c.handlers[SessionCookieSameSite] = handleSessionCookieAnnotation

	// Authentication
	c.handlers[AuthType] = handleAuthType
	c.handlers[AuthURL] = handleAuthURL
	c.handlers[AuthSecret] = handleAuthAnnotation
	c.handlers[AuthRealm] = handleAuthAnnotation

	// Backend protocol
	c.handlers[BackendProtocol] = handleBackendProtocol

	// Canary
	c.handlers[Canary] = handleCanaryEnabled
	c.handlers[CanaryWeight] = handleCanaryAnnotation
	c.handlers[CanaryWeightTotal] = handleCanaryAnnotation
	c.handlers[CanaryByHeader] = handleCanaryAnnotation
	c.handlers[CanaryByHeaderValue] = handleCanaryAnnotation
	c.handlers[CanaryByCookie] = handleCanaryAnnotation

	// Not supported
	c.handlers[ServerSnippet] = handleNotSupported
	c.handlers[ConfigurationSnippet] = handleNotSupported
	c.handlers[StreamSnippet] = handleNotSupported
	c.handlers[EnableModSecurity] = handleNotSupported
	c.handlers[EnableOWASPCoreRules] = handleNotSupported
	c.handlers[UpstreamHashBy] = handleNotSupported
}

// Convert processes all annotations and returns filters and warnings
func (c *Converter) Convert(annotations map[string]string) AnnotationConversionResult {
	result := AnnotationConversionResult{
		Filters:   []gwapiv1.HTTPRouteFilter{},
		Warnings:  []string{},
		Processed: []string{},
	}

	if annotations == nil {
		return result
	}

	// Process each annotation
	for key, value := range annotations {
		handler, ok := c.handlers[key]
		if !ok {
			// Unknown annotation - skip silently (may be for other controllers)
			continue
		}

		filters, warnings := handler(key, value, annotations)
		result.Filters = append(result.Filters, filters...)
		result.Warnings = append(result.Warnings, warnings...)
		result.Processed = append(result.Processed, key)
	}

	// Deduplicate filters (some annotations may generate same filter type)
	result.Filters = deduplicateFilters(result.Filters)

	return result
}

// deduplicateFilters removes duplicate filter types, keeping the last one
func deduplicateFilters(filters []gwapiv1.HTTPRouteFilter) []gwapiv1.HTTPRouteFilter {
	seen := make(map[gwapiv1.HTTPRouteFilterType]int)
	for i, f := range filters {
		seen[f.Type] = i
	}

	// If no duplicates, return as-is
	if len(seen) == len(filters) {
		return filters
	}

	// Build deduplicated list preserving order of last occurrence
	result := make([]gwapiv1.HTTPRouteFilter, 0, len(seen))
	for i, f := range filters {
		if seen[f.Type] == i {
			result = append(result, f)
		}
	}
	return result
}

// handleNotSupported generates a warning for unsupported annotations
func handleNotSupported(key, _ string, _ map[string]string) ([]gwapiv1.HTTPRouteFilter, []string) {
	return nil, []string{fmt.Sprintf("annotation %q is not supported in Gateway API conversion", key)}
}
