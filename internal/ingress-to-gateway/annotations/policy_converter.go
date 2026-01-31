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
	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"

	"k8c.io/kubelb/internal/ingress-to-gateway/policies"
)

// PolicyConversionInput holds the input for policy conversion.
type PolicyConversionInput struct {
	IngressName      string
	IngressNamespace string
	RouteName        string
	GatewayName      string
	Annotations      map[string]string
}

// PolicyConversionResult holds the result of policy conversion.
type PolicyConversionResult struct {
	SecurityPolicies       []*egv1alpha1.SecurityPolicy
	BackendTrafficPolicies []*egv1alpha1.BackendTrafficPolicy
	ClientTrafficPolicies  []*egv1alpha1.ClientTrafficPolicy
	// Warnings for annotations that couldn't be fully converted to policies
	Warnings []string
}

// ConvertToPolicies converts NGINX Ingress annotations to Envoy Gateway policies.
func ConvertToPolicies(input PolicyConversionInput) PolicyConversionResult {
	result := PolicyConversionResult{}

	if input.Annotations == nil {
		return result
	}

	// Build SecurityPolicy (CORS, IP access, auth)
	securityPolicy := buildSecurityPolicy(input)
	if securityPolicy != nil {
		result.SecurityPolicies = append(result.SecurityPolicies, securityPolicy)
	}

	// Build BackendTrafficPolicy (timeouts, rate limits, circuit breaker)
	backendPolicy, warnings := buildBackendTrafficPolicy(input)
	if backendPolicy != nil {
		result.BackendTrafficPolicies = append(result.BackendTrafficPolicies, backendPolicy)
	}
	result.Warnings = append(result.Warnings, warnings...)

	// External auth warning - auth-url requires backend reference configuration
	if input.Annotations[AuthURL] != "" {
		result.Warnings = append(result.Warnings,
			"auth-url cannot be automatically converted to SecurityPolicy ExtAuth; requires manual backend reference configuration")
	}

	// Note: ClientTrafficPolicy targets Gateway, not HTTPRoute
	// Body size and other client-side settings require more complex handling
	// We'll add warnings for those annotations that can't be converted
	if input.Annotations[ProxyBodySize] != "" {
		result.Warnings = append(result.Warnings,
			"proxy-body-size cannot be directly converted to ClientTrafficPolicy; requires manual Envoy configuration")
	}

	return result
}

// buildSecurityPolicy creates a SecurityPolicy from CORS, IP access, and auth annotations.
func buildSecurityPolicy(input PolicyConversionInput) *egv1alpha1.SecurityPolicy {
	builder := policies.NewSecurityPolicyBuilder(
		policies.NewPolicyBuilder(input.IngressName, input.IngressNamespace, input.RouteName),
	)

	// CORS configuration
	if input.Annotations[EnableCORS] == boolTrue {
		allowOrigins := policies.ParseStringList(input.Annotations[CORSAllowOrigin])
		allowMethods := policies.ParseStringList(input.Annotations[CORSAllowMethods])
		allowHeaders := policies.ParseStringList(input.Annotations[CORSAllowHeaders])
		exposeHeaders := policies.ParseStringList(input.Annotations[CORSExposeHeaders])
		maxAge := policies.ParseInt64(input.Annotations[CORSMaxAge])
		allowCredentials := policies.ParseBool(input.Annotations[CORSAllowCredentials])

		builder.SetCORS(allowOrigins, allowMethods, allowHeaders, exposeHeaders, maxAge, allowCredentials)
	}

	// IP allowlist
	if whitelist := input.Annotations[WhitelistSourceRange]; whitelist != "" {
		cidrs := policies.ParseCIDRList(whitelist)
		builder.SetIPAllowlist(cidrs)
	}

	// IP denylist
	if denylist := input.Annotations[DenylistSourceRange]; denylist != "" {
		cidrs := policies.ParseCIDRList(denylist)
		builder.SetIPDenylist(cidrs)
	}

	// Basic auth
	if input.Annotations[AuthType] == "basic" {
		builder.SetBasicAuth(input.Annotations[AuthSecret], input.IngressNamespace)
	}

	if builder.IsEmpty() {
		return nil
	}

	return builder.Build()
}

// buildBackendTrafficPolicy creates a BackendTrafficPolicy from timeout, rate limit, and circuit breaker annotations.
func buildBackendTrafficPolicy(input PolicyConversionInput) (*egv1alpha1.BackendTrafficPolicy, []string) {
	var warnings []string

	builder := policies.NewBackendTrafficPolicyBuilder(
		policies.NewPolicyBuilder(input.IngressName, input.IngressNamespace, input.RouteName),
	)

	// Timeouts
	if connectTimeout := input.Annotations[ProxyConnectTimeout]; connectTimeout != "" {
		builder.SetConnectTimeout(connectTimeout)
	}
	if readTimeout := input.Annotations[ProxyReadTimeout]; readTimeout != "" {
		builder.SetRequestTimeout(readTimeout)
	}
	// proxy-send-timeout maps to the same request timeout in Envoy Gateway
	// The distinction between read and send timeouts doesn't exist the same way
	if sendTimeout := input.Annotations[ProxySendTimeout]; sendTimeout != "" {
		// If read timeout wasn't set, use send timeout
		if input.Annotations[ProxyReadTimeout] == "" {
			builder.SetRequestTimeout(sendTimeout)
		}
	}

	// Rate limiting
	if rps := policies.ParseInt(input.Annotations[LimitRPS]); rps > 0 {
		builder.SetRateLimitRPS(rps)
	} else if rpm := policies.ParseInt(input.Annotations[LimitRPM]); rpm > 0 {
		builder.SetRateLimitRPM(rpm)
	}

	// Max connections (circuit breaker)
	if maxConns := policies.ParseInt(input.Annotations[LimitConnections]); maxConns > 0 {
		builder.SetMaxConnections(maxConns)
	}

	// Session affinity - not directly supported in BackendTrafficPolicy the same way
	// Envoy Gateway uses different mechanisms for session persistence
	if input.Annotations[Affinity] == "cookie" {
		warnings = append(warnings,
			"affinity=cookie requires manual BackendTrafficPolicy configuration for session persistence")
	}

	if builder.IsEmpty() {
		return nil, warnings
	}

	return builder.Build(), warnings
}
