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
	"strings"
	"testing"

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"
)

func TestConvertToPolicies_Empty(t *testing.T) {
	result := ConvertToPolicies(PolicyConversionInput{
		IngressName:      "test",
		IngressNamespace: "default",
		RouteName:        "test-route",
		GatewayName:      "test-gateway",
		Annotations:      nil,
	})

	if len(result.SecurityPolicies) != 0 {
		t.Errorf("Expected 0 SecurityPolicies, got %d", len(result.SecurityPolicies))
	}
	if len(result.BackendTrafficPolicies) != 0 {
		t.Errorf("Expected 0 BackendTrafficPolicies, got %d", len(result.BackendTrafficPolicies))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("Expected 0 warnings, got %d", len(result.Warnings))
	}
}

func TestConvertToPolicies_CORS(t *testing.T) {
	result := ConvertToPolicies(PolicyConversionInput{
		IngressName:      "test",
		IngressNamespace: "default",
		RouteName:        "test-route",
		GatewayName:      "test-gateway",
		Annotations: map[string]string{
			EnableCORS:           "true",
			CORSAllowOrigin:      "https://example.com,https://other.com",
			CORSAllowMethods:     "GET,POST,PUT",
			CORSAllowHeaders:     "Content-Type,Authorization",
			CORSExposeHeaders:    "X-Custom-Header",
			CORSAllowCredentials: "true",
			CORSMaxAge:           "3600",
		},
	})

	if len(result.SecurityPolicies) != 1 {
		t.Fatalf("Expected 1 SecurityPolicy, got %d", len(result.SecurityPolicies))
	}

	policy := result.SecurityPolicies[0]
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
}

func TestConvertToPolicies_IPAccess(t *testing.T) {
	tests := []struct {
		name          string
		annotations   map[string]string
		expectAllow   bool
		expectDeny    bool
		defaultAction egv1alpha1.AuthorizationAction
	}{
		{
			name: "whitelist only",
			annotations: map[string]string{
				WhitelistSourceRange: "10.0.0.0/8,192.168.0.0/16",
			},
			expectAllow:   true,
			expectDeny:    false,
			defaultAction: egv1alpha1.AuthorizationActionDeny,
		},
		{
			name: "denylist only",
			annotations: map[string]string{
				DenylistSourceRange: "10.0.0.0/8",
			},
			expectAllow:   false,
			expectDeny:    true,
			defaultAction: egv1alpha1.AuthorizationActionAllow,
		},
		{
			name: "both whitelist and denylist",
			annotations: map[string]string{
				WhitelistSourceRange: "10.0.0.0/8",
				DenylistSourceRange:  "192.168.0.0/16",
			},
			expectAllow: true,
			expectDeny:  true,
			// Denylist is processed second, so its default action wins
			defaultAction: egv1alpha1.AuthorizationActionAllow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToPolicies(PolicyConversionInput{
				IngressName:      "test",
				IngressNamespace: "default",
				RouteName:        "test-route",
				GatewayName:      "test-gateway",
				Annotations:      tt.annotations,
			})

			if len(result.SecurityPolicies) != 1 {
				t.Fatalf("Expected 1 SecurityPolicy, got %d", len(result.SecurityPolicies))
			}

			policy := result.SecurityPolicies[0]
			if policy.Spec.Authorization == nil {
				t.Fatal("Authorization spec is nil")
			}

			hasAllow := false
			hasDeny := false
			for _, rule := range policy.Spec.Authorization.Rules {
				if rule.Action == egv1alpha1.AuthorizationActionAllow {
					hasAllow = true
				}
				if rule.Action == egv1alpha1.AuthorizationActionDeny {
					hasDeny = true
				}
			}

			if hasAllow != tt.expectAllow {
				t.Errorf("Allow rule: got %v, want %v", hasAllow, tt.expectAllow)
			}
			if hasDeny != tt.expectDeny {
				t.Errorf("Deny rule: got %v, want %v", hasDeny, tt.expectDeny)
			}
			if *policy.Spec.Authorization.DefaultAction != tt.defaultAction {
				t.Errorf("DefaultAction: got %v, want %v", *policy.Spec.Authorization.DefaultAction, tt.defaultAction)
			}
		})
	}
}

func TestConvertToPolicies_BasicAuth(t *testing.T) {
	result := ConvertToPolicies(PolicyConversionInput{
		IngressName:      "test",
		IngressNamespace: "my-namespace",
		RouteName:        "test-route",
		GatewayName:      "test-gateway",
		Annotations: map[string]string{
			AuthType:   "basic",
			AuthSecret: "my-auth-secret",
		},
	})

	if len(result.SecurityPolicies) != 1 {
		t.Fatalf("Expected 1 SecurityPolicy, got %d", len(result.SecurityPolicies))
	}

	policy := result.SecurityPolicies[0]
	if policy.Spec.BasicAuth == nil {
		t.Fatal("BasicAuth spec is nil")
	}
	if string(policy.Spec.BasicAuth.Users.Name) != "my-auth-secret" {
		t.Errorf("Expected secret name 'my-auth-secret', got %q", policy.Spec.BasicAuth.Users.Name)
	}
	// Verify namespace is set
	if policy.Spec.BasicAuth.Users.Namespace == nil {
		t.Fatal("BasicAuth namespace is nil")
	}
	if string(*policy.Spec.BasicAuth.Users.Namespace) != "my-namespace" {
		t.Errorf("Expected namespace 'my-namespace', got %q", *policy.Spec.BasicAuth.Users.Namespace)
	}
}

func TestConvertToPolicies_Timeouts(t *testing.T) {
	result := ConvertToPolicies(PolicyConversionInput{
		IngressName:      "test",
		IngressNamespace: "default",
		RouteName:        "test-route",
		GatewayName:      "test-gateway",
		Annotations: map[string]string{
			ProxyConnectTimeout: "30",
			ProxyReadTimeout:    "60",
		},
	})

	if len(result.BackendTrafficPolicies) != 1 {
		t.Fatalf("Expected 1 BackendTrafficPolicy, got %d", len(result.BackendTrafficPolicies))
	}

	policy := result.BackendTrafficPolicies[0]
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

func TestConvertToPolicies_RateLimits(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expectRPS   bool
		expectRPM   bool
		expectConns bool
	}{
		{
			name: "RPS only",
			annotations: map[string]string{
				LimitRPS: "100",
			},
			expectRPS: true,
		},
		{
			name: "RPM only",
			annotations: map[string]string{
				LimitRPM: "6000",
			},
			expectRPM: true,
		},
		{
			name: "connections only",
			annotations: map[string]string{
				LimitConnections: "1000",
			},
			expectConns: true,
		},
		{
			name: "RPS takes precedence over RPM",
			annotations: map[string]string{
				LimitRPS: "100",
				LimitRPM: "6000",
			},
			expectRPS: true,
			expectRPM: false, // RPS wins
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToPolicies(PolicyConversionInput{
				IngressName:      "test",
				IngressNamespace: "default",
				RouteName:        "test-route",
				GatewayName:      "test-gateway",
				Annotations:      tt.annotations,
			})

			if len(result.BackendTrafficPolicies) != 1 {
				t.Fatalf("Expected 1 BackendTrafficPolicy, got %d", len(result.BackendTrafficPolicies))
			}

			policy := result.BackendTrafficPolicies[0]

			if tt.expectRPS || tt.expectRPM {
				if policy.Spec.RateLimit == nil {
					t.Fatal("RateLimit spec is nil")
				}
				rule := policy.Spec.RateLimit.Local.Rules[0]
				if tt.expectRPS && rule.Limit.Unit != egv1alpha1.RateLimitUnitSecond {
					t.Errorf("Expected Second unit, got %v", rule.Limit.Unit)
				}
				if tt.expectRPM && rule.Limit.Unit != egv1alpha1.RateLimitUnitMinute {
					t.Errorf("Expected Minute unit, got %v", rule.Limit.Unit)
				}
			}

			if tt.expectConns {
				if policy.Spec.CircuitBreaker == nil {
					t.Fatal("CircuitBreaker spec is nil")
				}
			}
		})
	}
}

func TestConvertToPolicies_Warnings(t *testing.T) {
	tests := []struct {
		name           string
		annotations    map[string]string
		expectWarnings []string
	}{
		{
			name: "auth-url warning",
			annotations: map[string]string{
				AuthURL: "https://auth.example.com/validate",
			},
			expectWarnings: []string{"auth-url cannot be automatically converted"},
		},
		{
			name: "proxy-body-size warning",
			annotations: map[string]string{
				ProxyBodySize: "10m",
			},
			expectWarnings: []string{"proxy-body-size cannot be directly converted"},
		},
		{
			name: "session affinity cookie warning",
			annotations: map[string]string{
				Affinity: "cookie",
			},
			expectWarnings: []string{"affinity=cookie requires manual"},
		},
		{
			name: "multiple warnings",
			annotations: map[string]string{
				AuthURL:       "https://auth.example.com/validate",
				ProxyBodySize: "10m",
				Affinity:      "cookie",
			},
			expectWarnings: []string{
				"auth-url cannot be automatically converted",
				"proxy-body-size cannot be directly converted",
				"affinity=cookie requires manual",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToPolicies(PolicyConversionInput{
				IngressName:      "test",
				IngressNamespace: "default",
				RouteName:        "test-route",
				GatewayName:      "test-gateway",
				Annotations:      tt.annotations,
			})

			if len(result.Warnings) != len(tt.expectWarnings) {
				t.Errorf("Expected %d warnings, got %d: %v", len(tt.expectWarnings), len(result.Warnings), result.Warnings)
			}

			for _, expected := range tt.expectWarnings {
				found := false
				for _, warning := range result.Warnings {
					if strings.Contains(warning, expected) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q, got %v", expected, result.Warnings)
				}
			}
		})
	}
}

func TestConvertToPolicies_Combined(t *testing.T) {
	result := ConvertToPolicies(PolicyConversionInput{
		IngressName:      "test",
		IngressNamespace: "default",
		RouteName:        "test-route",
		GatewayName:      "test-gateway",
		Annotations: map[string]string{
			// Security policy annotations
			EnableCORS:           "true",
			CORSAllowOrigin:      "*",
			WhitelistSourceRange: "10.0.0.0/8",
			AuthType:             "basic",
			AuthSecret:           "my-secret",
			// Backend policy annotations
			ProxyConnectTimeout: "30",
			ProxyReadTimeout:    "60",
			LimitRPS:            "100",
			LimitConnections:    "500",
		},
	})

	// Should have both policy types
	if len(result.SecurityPolicies) != 1 {
		t.Errorf("Expected 1 SecurityPolicy, got %d", len(result.SecurityPolicies))
	}
	if len(result.BackendTrafficPolicies) != 1 {
		t.Errorf("Expected 1 BackendTrafficPolicy, got %d", len(result.BackendTrafficPolicies))
	}

	// Verify security policy has all features
	secPolicy := result.SecurityPolicies[0]
	if secPolicy.Spec.CORS == nil {
		t.Error("SecurityPolicy should have CORS")
	}
	if secPolicy.Spec.Authorization == nil {
		t.Error("SecurityPolicy should have Authorization")
	}
	if secPolicy.Spec.BasicAuth == nil {
		t.Error("SecurityPolicy should have BasicAuth")
	}

	// Verify backend policy has all features
	backendPolicy := result.BackendTrafficPolicies[0]
	if backendPolicy.Spec.Timeout == nil {
		t.Error("BackendTrafficPolicy should have Timeout")
	}
	if backendPolicy.Spec.RateLimit == nil {
		t.Error("BackendTrafficPolicy should have RateLimit")
	}
	if backendPolicy.Spec.CircuitBreaker == nil {
		t.Error("BackendTrafficPolicy should have CircuitBreaker")
	}
}

func TestConvertToPolicies_NoSecurityPolicyWhenEmpty(t *testing.T) {
	// Only backend annotations, no security annotations
	result := ConvertToPolicies(PolicyConversionInput{
		IngressName:      "test",
		IngressNamespace: "default",
		RouteName:        "test-route",
		GatewayName:      "test-gateway",
		Annotations: map[string]string{
			ProxyConnectTimeout: "30",
		},
	})

	if len(result.SecurityPolicies) != 0 {
		t.Errorf("Expected 0 SecurityPolicies, got %d", len(result.SecurityPolicies))
	}
	if len(result.BackendTrafficPolicies) != 1 {
		t.Errorf("Expected 1 BackendTrafficPolicy, got %d", len(result.BackendTrafficPolicies))
	}
}

func TestConvertToPolicies_NoBackendPolicyWhenEmpty(t *testing.T) {
	// Only security annotations, no backend annotations
	result := ConvertToPolicies(PolicyConversionInput{
		IngressName:      "test",
		IngressNamespace: "default",
		RouteName:        "test-route",
		GatewayName:      "test-gateway",
		Annotations: map[string]string{
			EnableCORS:      "true",
			CORSAllowOrigin: "*",
		},
	})

	if len(result.SecurityPolicies) != 1 {
		t.Errorf("Expected 1 SecurityPolicy, got %d", len(result.SecurityPolicies))
	}
	if len(result.BackendTrafficPolicies) != 0 {
		t.Errorf("Expected 0 BackendTrafficPolicies, got %d", len(result.BackendTrafficPolicies))
	}
}
