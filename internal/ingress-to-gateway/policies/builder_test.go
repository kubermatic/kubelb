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
	"testing"

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"

	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestSecurityPolicyBuilder_CORS(t *testing.T) {
	builder := NewSecurityPolicyBuilder(
		NewPolicyBuilder("test-ingress", "default", "test-route"),
	)

	maxAge := int64(3600)
	allowCreds := true
	builder.SetCORS(
		[]string{"https://example.com", "https://other.com"},
		[]string{"GET", "POST", "PUT"},
		[]string{"Content-Type", "Authorization"},
		[]string{"X-Custom-Header"},
		&maxAge,
		&allowCreds,
	)

	if builder.IsEmpty() {
		t.Error("Builder should not be empty after SetCORS")
	}

	policy := builder.Build()
	if policy == nil {
		t.Fatal("Build() returned nil")
	}

	if policy.Name != "test-ingress-security" {
		t.Errorf("Expected name 'test-ingress-security', got %q", policy.Name)
	}
	if policy.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got %q", policy.Namespace)
	}

	if policy.Spec.CORS == nil {
		t.Fatal("CORS spec is nil")
	}
	if len(policy.Spec.CORS.AllowOrigins) != 2 {
		t.Errorf("Expected 2 allow origins, got %d", len(policy.Spec.CORS.AllowOrigins))
	}
	if len(policy.Spec.CORS.AllowMethods) != 3 {
		t.Errorf("Expected 3 allow methods, got %d", len(policy.Spec.CORS.AllowMethods))
	}
	if policy.Spec.CORS.AllowCredentials == nil || !*policy.Spec.CORS.AllowCredentials {
		t.Error("AllowCredentials should be true")
	}

	// Verify target ref (using deprecated TargetRef, but it's what we set in builder)
	//nolint:staticcheck // SA1019: testing deprecated TargetRef for backward compatibility
	if policy.Spec.TargetRef == nil {
		t.Fatal("TargetRef is nil")
	}
	//nolint:staticcheck // SA1019: testing deprecated TargetRef for backward compatibility
	if string(policy.Spec.TargetRef.Kind) != "HTTPRoute" {
		//nolint:staticcheck // SA1019: testing deprecated TargetRef for backward compatibility
		t.Errorf("Expected target kind HTTPRoute, got %q", policy.Spec.TargetRef.Kind)
	}
	//nolint:staticcheck // SA1019: testing deprecated TargetRef for backward compatibility
	if string(policy.Spec.TargetRef.Name) != "test-route" {
		//nolint:staticcheck // SA1019: testing deprecated TargetRef for backward compatibility
		t.Errorf("Expected target name 'test-route', got %q", policy.Spec.TargetRef.Name)
	}
}

func TestSecurityPolicyBuilder_IPAllowlist(t *testing.T) {
	builder := NewSecurityPolicyBuilder(
		NewPolicyBuilder("test-ingress", "default", "test-route"),
	)

	builder.SetIPAllowlist([]string{"10.0.0.0/8", "192.168.1.0/24"})

	if builder.IsEmpty() {
		t.Error("Builder should not be empty after SetIPAllowlist")
	}

	policy := builder.Build()
	if policy == nil {
		t.Fatal("Build() returned nil")
	}

	if policy.Spec.Authorization == nil {
		t.Fatal("Authorization spec is nil")
	}
	if len(policy.Spec.Authorization.Rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(policy.Spec.Authorization.Rules))
	}

	rule := policy.Spec.Authorization.Rules[0]
	if rule.Action != egv1alpha1.AuthorizationActionAllow {
		t.Errorf("Expected Allow action, got %v", rule.Action)
	}
	if len(rule.Principal.ClientCIDRs) != 2 {
		t.Errorf("Expected 2 CIDRs, got %d", len(rule.Principal.ClientCIDRs))
	}

	// Default action should be Deny for allowlist
	if policy.Spec.Authorization.DefaultAction == nil {
		t.Fatal("DefaultAction is nil")
	}
	if *policy.Spec.Authorization.DefaultAction != egv1alpha1.AuthorizationActionDeny {
		t.Errorf("Expected default action Deny, got %v", *policy.Spec.Authorization.DefaultAction)
	}
}

func TestSecurityPolicyBuilder_IPDenylist(t *testing.T) {
	builder := NewSecurityPolicyBuilder(
		NewPolicyBuilder("test-ingress", "default", "test-route"),
	)

	builder.SetIPDenylist([]string{"10.0.0.0/8"})

	policy := builder.Build()
	if policy == nil {
		t.Fatal("Build() returned nil")
	}

	if policy.Spec.Authorization == nil {
		t.Fatal("Authorization spec is nil")
	}

	rule := policy.Spec.Authorization.Rules[0]
	if rule.Action != egv1alpha1.AuthorizationActionDeny {
		t.Errorf("Expected Deny action, got %v", rule.Action)
	}

	// Default action should be Allow for denylist
	if *policy.Spec.Authorization.DefaultAction != egv1alpha1.AuthorizationActionAllow {
		t.Errorf("Expected default action Allow, got %v", *policy.Spec.Authorization.DefaultAction)
	}
}

func TestSecurityPolicyBuilder_BasicAuth(t *testing.T) {
	builder := NewSecurityPolicyBuilder(
		NewPolicyBuilder("test-ingress", "default", "test-route"),
	)

	builder.SetBasicAuth("my-auth-secret", "auth-namespace")

	if builder.IsEmpty() {
		t.Error("Builder should not be empty after SetBasicAuth")
	}

	policy := builder.Build()
	if policy == nil {
		t.Fatal("Build() returned nil")
	}

	if policy.Spec.BasicAuth == nil {
		t.Fatal("BasicAuth spec is nil")
	}
	if string(policy.Spec.BasicAuth.Users.Name) != "my-auth-secret" {
		t.Errorf("Expected secret name 'my-auth-secret', got %q", policy.Spec.BasicAuth.Users.Name)
	}
	if policy.Spec.BasicAuth.Users.Namespace == nil {
		t.Fatal("Namespace is nil")
	}
	if string(*policy.Spec.BasicAuth.Users.Namespace) != "auth-namespace" {
		t.Errorf("Expected namespace 'auth-namespace', got %q", *policy.Spec.BasicAuth.Users.Namespace)
	}
}

func TestSecurityPolicyBuilder_Combined(t *testing.T) {
	builder := NewSecurityPolicyBuilder(
		NewPolicyBuilder("test-ingress", "default", "test-route"),
	)

	// Set multiple features
	builder.SetCORS([]string{"*"}, nil, nil, nil, nil, nil)
	builder.SetIPAllowlist([]string{"10.0.0.0/8"})
	builder.SetBasicAuth("secret", "default")

	policy := builder.Build()
	if policy == nil {
		t.Fatal("Build() returned nil")
	}

	if policy.Spec.CORS == nil {
		t.Error("CORS should not be nil")
	}
	if policy.Spec.Authorization == nil {
		t.Error("Authorization should not be nil")
	}
	if policy.Spec.BasicAuth == nil {
		t.Error("BasicAuth should not be nil")
	}
}

func TestSecurityPolicyBuilder_Empty(t *testing.T) {
	builder := NewSecurityPolicyBuilder(
		NewPolicyBuilder("test-ingress", "default", "test-route"),
	)

	if !builder.IsEmpty() {
		t.Error("New builder should be empty")
	}

	policy := builder.Build()
	if policy != nil {
		t.Error("Build() should return nil for empty builder")
	}
}

func TestBackendTrafficPolicyBuilder_Timeouts(t *testing.T) {
	builder := NewBackendTrafficPolicyBuilder(
		NewPolicyBuilder("test-ingress", "default", "test-route"),
	)

	builder.SetConnectTimeout("30s")
	builder.SetRequestTimeout("60s")

	if builder.IsEmpty() {
		t.Error("Builder should not be empty after setting timeouts")
	}

	policy := builder.Build()
	if policy == nil {
		t.Fatal("Build() returned nil")
	}

	if policy.Name != "test-ingress-backend" {
		t.Errorf("Expected name 'test-ingress-backend', got %q", policy.Name)
	}

	if policy.Spec.Timeout == nil {
		t.Fatal("Timeout spec is nil")
	}
	if policy.Spec.Timeout.TCP == nil || policy.Spec.Timeout.TCP.ConnectTimeout == nil {
		t.Fatal("TCP ConnectTimeout is nil")
	}
	if string(*policy.Spec.Timeout.TCP.ConnectTimeout) != "30s" {
		t.Errorf("Expected connect timeout '30s', got %q", *policy.Spec.Timeout.TCP.ConnectTimeout)
	}
	if policy.Spec.Timeout.HTTP == nil || policy.Spec.Timeout.HTTP.RequestTimeout == nil {
		t.Fatal("HTTP RequestTimeout is nil")
	}
	if string(*policy.Spec.Timeout.HTTP.RequestTimeout) != "60s" {
		t.Errorf("Expected request timeout '60s', got %q", *policy.Spec.Timeout.HTTP.RequestTimeout)
	}
}

func TestBackendTrafficPolicyBuilder_RateLimitRPS(t *testing.T) {
	builder := NewBackendTrafficPolicyBuilder(
		NewPolicyBuilder("test-ingress", "default", "test-route"),
	)

	builder.SetRateLimitRPS(100)

	policy := builder.Build()
	if policy == nil {
		t.Fatal("Build() returned nil")
	}

	if policy.Spec.RateLimit == nil {
		t.Fatal("RateLimit spec is nil")
	}
	if policy.Spec.RateLimit.Local == nil || len(policy.Spec.RateLimit.Local.Rules) != 1 {
		t.Fatal("Expected 1 local rate limit rule")
	}

	rule := policy.Spec.RateLimit.Local.Rules[0]
	if rule.Limit.Requests != 100 {
		t.Errorf("Expected 100 requests, got %d", rule.Limit.Requests)
	}
	if rule.Limit.Unit != egv1alpha1.RateLimitUnitSecond {
		t.Errorf("Expected Second unit, got %v", rule.Limit.Unit)
	}
}

func TestBackendTrafficPolicyBuilder_RateLimitRPM(t *testing.T) {
	builder := NewBackendTrafficPolicyBuilder(
		NewPolicyBuilder("test-ingress", "default", "test-route"),
	)

	builder.SetRateLimitRPM(6000)

	policy := builder.Build()
	if policy == nil {
		t.Fatal("Build() returned nil")
	}

	rule := policy.Spec.RateLimit.Local.Rules[0]
	if rule.Limit.Requests != 6000 {
		t.Errorf("Expected 6000 requests, got %d", rule.Limit.Requests)
	}
	if rule.Limit.Unit != egv1alpha1.RateLimitUnitMinute {
		t.Errorf("Expected Minute unit, got %v", rule.Limit.Unit)
	}
}

func TestBackendTrafficPolicyBuilder_MaxConnections(t *testing.T) {
	builder := NewBackendTrafficPolicyBuilder(
		NewPolicyBuilder("test-ingress", "default", "test-route"),
	)

	builder.SetMaxConnections(1000)

	policy := builder.Build()
	if policy == nil {
		t.Fatal("Build() returned nil")
	}

	if policy.Spec.CircuitBreaker == nil {
		t.Fatal("CircuitBreaker spec is nil")
	}
	if policy.Spec.CircuitBreaker.MaxConnections == nil {
		t.Fatal("MaxConnections is nil")
	}
	if *policy.Spec.CircuitBreaker.MaxConnections != 1000 {
		t.Errorf("Expected 1000 max connections, got %d", *policy.Spec.CircuitBreaker.MaxConnections)
	}
}

func TestBackendTrafficPolicyBuilder_Combined(t *testing.T) {
	builder := NewBackendTrafficPolicyBuilder(
		NewPolicyBuilder("test-ingress", "default", "test-route"),
	)

	builder.SetConnectTimeout("10s")
	builder.SetRequestTimeout("30s")
	builder.SetRateLimitRPS(50)
	builder.SetMaxConnections(500)

	policy := builder.Build()
	if policy == nil {
		t.Fatal("Build() returned nil")
	}

	if policy.Spec.Timeout == nil {
		t.Error("Timeout should not be nil")
	}
	if policy.Spec.RateLimit == nil {
		t.Error("RateLimit should not be nil")
	}
	if policy.Spec.CircuitBreaker == nil {
		t.Error("CircuitBreaker should not be nil")
	}
}

func TestBackendTrafficPolicyBuilder_Empty(t *testing.T) {
	builder := NewBackendTrafficPolicyBuilder(
		NewPolicyBuilder("test-ingress", "default", "test-route"),
	)

	if !builder.IsEmpty() {
		t.Error("New builder should be empty")
	}

	policy := builder.Build()
	if policy != nil {
		t.Error("Build() should return nil for empty builder")
	}
}

func TestPolicyBuilder_TargetRef(t *testing.T) {
	base := NewPolicyBuilder("my-ingress", "my-namespace", "my-route")
	builder := NewSecurityPolicyBuilder(base)
	builder.SetCORS([]string{"*"}, nil, nil, nil, nil, nil)

	policy := builder.Build()
	if policy == nil {
		t.Fatal("Build() returned nil")
	}

	//nolint:staticcheck // SA1019: testing deprecated TargetRef for backward compatibility
	targetRef := policy.Spec.TargetRef
	if targetRef == nil {
		t.Fatal("TargetRef is nil")
	}
	if string(targetRef.Group) != gwapiv1.GroupName {
		t.Errorf("Expected group %q, got %q", gwapiv1.GroupName, targetRef.Group)
	}
	if string(targetRef.Kind) != "HTTPRoute" {
		t.Errorf("Expected kind HTTPRoute, got %q", targetRef.Kind)
	}
	if string(targetRef.Name) != "my-route" {
		t.Errorf("Expected name 'my-route', got %q", targetRef.Name)
	}
}

func TestPolicyBuilder_Labels(t *testing.T) {
	base := NewPolicyBuilder("my-ingress", "my-namespace", "my-route")
	builder := NewSecurityPolicyBuilder(base)
	builder.SetCORS([]string{"*"}, nil, nil, nil, nil, nil)

	policy := builder.Build()
	if policy == nil {
		t.Fatal("Build() returned nil")
	}

	expectedLabel := "my-ingress.my-namespace"
	if policy.Labels[LabelSourceIngress] != expectedLabel {
		t.Errorf("Expected label %q, got %q", expectedLabel, policy.Labels[LabelSourceIngress])
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"60", "60s"},
		{"60s", "60s"},
		{"5m", "5m"},
		{"1h", "1h"},
		{"", ""},
		{" 30 ", "30s"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseDuration(tt.input)
			if got != tt.expected {
				t.Errorf("parseDuration(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseCIDRList(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"10.0.0.0/8", 1},
		{"10.0.0.0/8,192.168.0.0/16", 2},
		{"10.0.0.0/8, 192.168.0.0/16, 172.16.0.0/12", 3},
		{"", 0},
		{" 10.0.0.0/8 , 192.168.0.0/16 ", 2},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseCIDRList(tt.input)
			if len(got) != tt.expected {
				t.Errorf("ParseCIDRList(%q) returned %d items, want %d", tt.input, len(got), tt.expected)
			}
		})
	}
}

func TestParseStringList(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"GET", 1},
		{"GET,POST,PUT", 3},
		{"GET, POST, PUT", 3},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseStringList(tt.input)
			if len(got) != tt.expected {
				t.Errorf("ParseStringList(%q) returned %d items, want %d", tt.input, len(got), tt.expected)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"100", 100},
		{"0", 0},
		{"-1", -1},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseInt(tt.input)
			if got != tt.expected {
				t.Errorf("ParseInt(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseInt64(t *testing.T) {
	tests := []struct {
		input    string
		expected *int64
	}{
		{"100", ptrTo(int64(100))},
		{"", nil},
		{"invalid", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseInt64(tt.input)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("ParseInt64(%q) = %v, want nil", tt.input, *got)
				}
			} else {
				if got == nil {
					t.Errorf("ParseInt64(%q) = nil, want %d", tt.input, *tt.expected)
				} else if *got != *tt.expected {
					t.Errorf("ParseInt64(%q) = %d, want %d", tt.input, *got, *tt.expected)
				}
			}
		})
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected *bool
	}{
		{"true", ptrTo(true)},
		{"false", ptrTo(false)},
		{"", nil},
		{"yes", ptrTo(false)}, // Only "true" is considered true
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseBool(tt.input)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("ParseBool(%q) = %v, want nil", tt.input, *got)
				}
			} else {
				if got == nil {
					t.Errorf("ParseBool(%q) = nil, want %v", tt.input, *tt.expected)
				} else if *got != *tt.expected {
					t.Errorf("ParseBool(%q) = %v, want %v", tt.input, *got, *tt.expected)
				}
			}
		})
	}
}
