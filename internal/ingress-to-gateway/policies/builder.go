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

package policies

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1a2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const (
	// LabelSourceIngress marks policies created from Ingress conversion
	LabelSourceIngress = "kubelb.k8c.io/source-ingress"

	// Policy name suffixes
	SecurityPolicySuffix       = "-security"
	BackendTrafficPolicySuffix = "-backend"
	ClientTrafficPolicySuffix  = "-client"
)

// PolicyBuilder creates Envoy Gateway policies from Ingress annotations.
type PolicyBuilder struct {
	IngressName      string
	IngressNamespace string
	RouteName        string
}

// NewPolicyBuilder creates a new policy builder.
func NewPolicyBuilder(ingressName, ingressNamespace, routeName string) *PolicyBuilder {
	return &PolicyBuilder{
		IngressName:      ingressName,
		IngressNamespace: ingressNamespace,
		RouteName:        routeName,
	}
}

// policyTargetRef creates a target reference to an HTTPRoute.
func (b *PolicyBuilder) policyTargetRef() gwapiv1a2.LocalPolicyTargetReferenceWithSectionName {
	return gwapiv1a2.LocalPolicyTargetReferenceWithSectionName(gwapiv1.LocalPolicyTargetReferenceWithSectionName{
		LocalPolicyTargetReference: gwapiv1.LocalPolicyTargetReference{
			Group: gwapiv1.Group(gwapiv1.GroupName),
			Kind:  gwapiv1.Kind("HTTPRoute"),
			Name:  gwapiv1.ObjectName(b.RouteName),
		},
	})
}

// policyLabels creates standard labels for policies.
func (b *PolicyBuilder) policyLabels() map[string]string {
	return map[string]string{
		LabelSourceIngress: fmt.Sprintf("%s.%s", b.IngressName, b.IngressNamespace),
	}
}

// SecurityPolicyBuilder builds a SecurityPolicy from CORS, IP access, and auth annotations.
type SecurityPolicyBuilder struct {
	*PolicyBuilder
	cors          *egv1alpha1.CORS
	authorization *egv1alpha1.Authorization
	basicAuth     *egv1alpha1.BasicAuth
}

// NewSecurityPolicyBuilder creates a new SecurityPolicyBuilder.
func NewSecurityPolicyBuilder(base *PolicyBuilder) *SecurityPolicyBuilder {
	return &SecurityPolicyBuilder{PolicyBuilder: base}
}

// SetCORS configures CORS settings.
func (b *SecurityPolicyBuilder) SetCORS(
	allowOrigins []string,
	allowMethods []string,
	allowHeaders []string,
	exposeHeaders []string,
	maxAgeSeconds *int64,
	allowCredentials *bool,
) *SecurityPolicyBuilder {
	b.cors = &egv1alpha1.CORS{}

	if len(allowOrigins) > 0 {
		origins := make([]egv1alpha1.Origin, len(allowOrigins))
		for i, o := range allowOrigins {
			origins[i] = egv1alpha1.Origin(o)
		}
		b.cors.AllowOrigins = origins
	}
	if len(allowMethods) > 0 {
		b.cors.AllowMethods = allowMethods
	}
	if len(allowHeaders) > 0 {
		b.cors.AllowHeaders = allowHeaders
	}
	if len(exposeHeaders) > 0 {
		b.cors.ExposeHeaders = exposeHeaders
	}
	if maxAgeSeconds != nil {
		d := metav1.Duration{Duration: time.Duration(*maxAgeSeconds) * time.Second}
		b.cors.MaxAge = &d
	}
	if allowCredentials != nil {
		b.cors.AllowCredentials = allowCredentials
	}

	return b
}

// SetIPAllowlist adds IP allowlist authorization rules.
func (b *SecurityPolicyBuilder) SetIPAllowlist(cidrs []string) *SecurityPolicyBuilder {
	if len(cidrs) == 0 {
		return b
	}

	if b.authorization == nil {
		b.authorization = &egv1alpha1.Authorization{}
	}

	cidrList := make([]egv1alpha1.CIDR, len(cidrs))
	for i, c := range cidrs {
		cidrList[i] = egv1alpha1.CIDR(strings.TrimSpace(c))
	}

	// Add allow rule
	ruleName := "ip-allowlist"
	b.authorization.Rules = append(b.authorization.Rules, egv1alpha1.AuthorizationRule{
		Name:   &ruleName,
		Action: egv1alpha1.AuthorizationActionAllow,
		Principal: egv1alpha1.Principal{
			ClientCIDRs: cidrList,
		},
	})

	// Default deny if using allowlist
	defaultDeny := egv1alpha1.AuthorizationActionDeny
	b.authorization.DefaultAction = &defaultDeny

	return b
}

// SetIPDenylist adds IP denylist authorization rules.
func (b *SecurityPolicyBuilder) SetIPDenylist(cidrs []string) *SecurityPolicyBuilder {
	if len(cidrs) == 0 {
		return b
	}

	if b.authorization == nil {
		b.authorization = &egv1alpha1.Authorization{}
	}

	cidrList := make([]egv1alpha1.CIDR, len(cidrs))
	for i, c := range cidrs {
		cidrList[i] = egv1alpha1.CIDR(strings.TrimSpace(c))
	}

	// Add deny rule
	ruleName := "ip-denylist"
	b.authorization.Rules = append(b.authorization.Rules, egv1alpha1.AuthorizationRule{
		Name:   &ruleName,
		Action: egv1alpha1.AuthorizationActionDeny,
		Principal: egv1alpha1.Principal{
			ClientCIDRs: cidrList,
		},
	})

	// Default allow if using denylist
	defaultAllow := egv1alpha1.AuthorizationActionAllow
	b.authorization.DefaultAction = &defaultAllow

	return b
}

// SetBasicAuth configures basic authentication.
func (b *SecurityPolicyBuilder) SetBasicAuth(secretRef, namespace string) *SecurityPolicyBuilder {
	if secretRef == "" {
		return b
	}

	ns := gwapiv1.Namespace(namespace)
	b.basicAuth = &egv1alpha1.BasicAuth{
		Users: gwapiv1.SecretObjectReference{
			Name:      gwapiv1.ObjectName(secretRef),
			Namespace: &ns,
		},
	}
	return b
}

// IsEmpty returns true if no security settings are configured.
func (b *SecurityPolicyBuilder) IsEmpty() bool {
	return b.cors == nil && b.authorization == nil && b.basicAuth == nil
}

// Build creates the SecurityPolicy.
func (b *SecurityPolicyBuilder) Build() *egv1alpha1.SecurityPolicy {
	if b.IsEmpty() {
		return nil
	}

	return &egv1alpha1.SecurityPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "gateway.envoyproxy.io/v1alpha1",
			Kind:       egv1alpha1.KindSecurityPolicy,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.IngressName + SecurityPolicySuffix,
			Namespace: b.IngressNamespace,
			Labels:    b.policyLabels(),
		},
		Spec: egv1alpha1.SecurityPolicySpec{
			PolicyTargetReferences: egv1alpha1.PolicyTargetReferences{
				TargetRef: ptrTo(b.policyTargetRef()),
			},
			CORS:          b.cors,
			Authorization: b.authorization,
			BasicAuth:     b.basicAuth,
		},
	}
}

// BackendTrafficPolicyBuilder builds a BackendTrafficPolicy from timeout, rate limit, and session annotations.
type BackendTrafficPolicyBuilder struct {
	*PolicyBuilder
	timeout        *egv1alpha1.Timeout
	rateLimit      *egv1alpha1.RateLimitSpec
	circuitBreaker *egv1alpha1.CircuitBreaker
}

// NewBackendTrafficPolicyBuilder creates a new BackendTrafficPolicyBuilder.
func NewBackendTrafficPolicyBuilder(base *PolicyBuilder) *BackendTrafficPolicyBuilder {
	return &BackendTrafficPolicyBuilder{PolicyBuilder: base}
}

// SetConnectTimeout sets TCP connect timeout.
func (b *BackendTrafficPolicyBuilder) SetConnectTimeout(duration string) *BackendTrafficPolicyBuilder {
	d := parseDuration(duration)
	if d == "" {
		return b
	}

	if b.timeout == nil {
		b.timeout = &egv1alpha1.Timeout{}
	}
	if b.timeout.TCP == nil {
		b.timeout.TCP = &egv1alpha1.TCPTimeout{}
	}
	gwDuration := gwapiv1.Duration(d)
	b.timeout.TCP.ConnectTimeout = &gwDuration
	return b
}

// SetRequestTimeout sets HTTP request timeout.
func (b *BackendTrafficPolicyBuilder) SetRequestTimeout(duration string) *BackendTrafficPolicyBuilder {
	d := parseDuration(duration)
	if d == "" {
		return b
	}

	if b.timeout == nil {
		b.timeout = &egv1alpha1.Timeout{}
	}
	if b.timeout.HTTP == nil {
		b.timeout.HTTP = &egv1alpha1.HTTPTimeout{}
	}
	gwDuration := gwapiv1.Duration(d)
	b.timeout.HTTP.RequestTimeout = &gwDuration
	return b
}

// SetRateLimitRPS sets rate limiting in requests per second.
func (b *BackendTrafficPolicyBuilder) SetRateLimitRPS(rps int) *BackendTrafficPolicyBuilder {
	if rps <= 0 {
		return b
	}

	b.rateLimit = &egv1alpha1.RateLimitSpec{
		Type: egv1alpha1.LocalRateLimitType,
		Local: &egv1alpha1.LocalRateLimit{
			Rules: []egv1alpha1.RateLimitRule{
				{
					Limit: egv1alpha1.RateLimitValue{
						Requests: uint(rps),
						Unit:     egv1alpha1.RateLimitUnitSecond,
					},
				},
			},
		},
	}
	return b
}

// SetRateLimitRPM sets rate limiting in requests per minute.
func (b *BackendTrafficPolicyBuilder) SetRateLimitRPM(rpm int) *BackendTrafficPolicyBuilder {
	if rpm <= 0 {
		return b
	}

	b.rateLimit = &egv1alpha1.RateLimitSpec{
		Type: egv1alpha1.LocalRateLimitType,
		Local: &egv1alpha1.LocalRateLimit{
			Rules: []egv1alpha1.RateLimitRule{
				{
					Limit: egv1alpha1.RateLimitValue{
						Requests: uint(rpm),
						Unit:     egv1alpha1.RateLimitUnitMinute,
					},
				},
			},
		},
	}
	return b
}

// SetMaxConnections sets max connections via circuit breaker.
func (b *BackendTrafficPolicyBuilder) SetMaxConnections(maxConns int) *BackendTrafficPolicyBuilder {
	if maxConns <= 0 {
		return b
	}

	if b.circuitBreaker == nil {
		b.circuitBreaker = &egv1alpha1.CircuitBreaker{}
	}
	conns := int64(maxConns)
	b.circuitBreaker.MaxConnections = &conns
	return b
}

// IsEmpty returns true if no backend traffic settings are configured.
func (b *BackendTrafficPolicyBuilder) IsEmpty() bool {
	return b.timeout == nil && b.rateLimit == nil && b.circuitBreaker == nil
}

// Build creates the BackendTrafficPolicy.
func (b *BackendTrafficPolicyBuilder) Build() *egv1alpha1.BackendTrafficPolicy {
	if b.IsEmpty() {
		return nil
	}

	return &egv1alpha1.BackendTrafficPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "gateway.envoyproxy.io/v1alpha1",
			Kind:       egv1alpha1.KindBackendTrafficPolicy,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.IngressName + BackendTrafficPolicySuffix,
			Namespace: b.IngressNamespace,
			Labels:    b.policyLabels(),
		},
		Spec: egv1alpha1.BackendTrafficPolicySpec{
			PolicyTargetReferences: egv1alpha1.PolicyTargetReferences{
				TargetRef: ptrTo(b.policyTargetRef()),
			},
			ClusterSettings: egv1alpha1.ClusterSettings{
				Timeout:        b.timeout,
				CircuitBreaker: b.circuitBreaker,
			},
			RateLimit: b.rateLimit,
		},
	}
}

// Helper functions

// parseDuration converts NGINX-style timeout values to Gateway API duration format.
// NGINX accepts: "60", "60s", "60m", "60h"
// Gateway API expects: "60s", "1m", "1h"
func parseDuration(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	// If already has a unit suffix, return as-is
	if strings.HasSuffix(value, "s") || strings.HasSuffix(value, "m") || strings.HasSuffix(value, "h") {
		return value
	}

	// Plain number means seconds
	if _, err := strconv.Atoi(value); err == nil {
		return value + "s"
	}

	return value
}

// ptrTo returns a pointer to the value.
func ptrTo[T any](v T) *T {
	return &v
}

// ParseCIDRList splits a comma-separated CIDR list.
func ParseCIDRList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ParseStringList splits a comma-separated string list.
func ParseStringList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ParseInt parses an integer from string, returning 0 if invalid.
func ParseInt(value string) int {
	if value == "" {
		return 0
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return i
}

// ParseInt64 parses an int64 from string, returning nil if invalid.
func ParseInt64(value string) *int64 {
	if value == "" {
		return nil
	}
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil
	}
	return &i
}

// ParseBool parses a boolean from string.
func ParseBool(value string) *bool {
	if value == "" {
		return nil
	}
	b := value == "true"
	return &b
}
