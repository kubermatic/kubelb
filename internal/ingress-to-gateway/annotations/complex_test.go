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
)

func TestHandleAffinity(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		contains    []string
		noWarning   bool
	}{
		{
			name: "cookie affinity only",
			annotations: map[string]string{
				Affinity: "cookie",
			},
			contains: []string{"BackendTrafficPolicy", "sessionPersistence"},
		},
		{
			name: "cookie affinity with config",
			annotations: map[string]string{
				Affinity:              "cookie",
				SessionCookieName:     "SERVERID",
				SessionCookiePath:     "/",
				SessionCookieExpires:  "3600",
				SessionCookieSameSite: "Strict",
			},
			contains: []string{"name=", "SERVERID", "path=", "ttl=", "sameSite="},
		},
		{
			name: "unsupported affinity type",
			annotations: map[string]string{
				Affinity: "ip",
			},
			contains: []string{"not supported"},
		},
		{
			name: "empty affinity",
			annotations: map[string]string{
				Affinity: "",
			},
			noWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.annotations[Affinity]
			filters, warnings := handleAffinity(Affinity, value, tt.annotations)

			if len(filters) != 0 {
				t.Error("affinity handler should not return filters")
			}

			if tt.noWarning {
				if len(warnings) != 0 {
					t.Errorf("expected no warnings, got %d", len(warnings))
				}
				return
			}

			if len(warnings) != 1 {
				t.Fatalf("expected 1 warning, got %d", len(warnings))
			}

			for _, s := range tt.contains {
				if !strings.Contains(warnings[0], s) {
					t.Errorf("warning should contain %q, got: %s", s, warnings[0])
				}
			}
		})
	}
}

func TestHandleAuthType(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		contains    []string
	}{
		{
			name: "basic auth",
			annotations: map[string]string{
				AuthType: "basic",
			},
			contains: []string{"SecurityPolicy", "basicAuth"},
		},
		{
			name: "basic auth with secret",
			annotations: map[string]string{
				AuthType:   "basic",
				AuthSecret: "my-auth-secret",
				AuthRealm:  "Protected Area",
			},
			contains: []string{"secretRef=", "my-auth-secret", "realm="},
		},
		{
			name: "digest auth (unsupported)",
			annotations: map[string]string{
				AuthType: "digest",
			},
			contains: []string{"not supported"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.annotations[AuthType]
			filters, warnings := handleAuthType(AuthType, value, tt.annotations)

			if len(filters) != 0 {
				t.Error("auth handler should not return filters")
			}
			if len(warnings) != 1 {
				t.Fatalf("expected 1 warning, got %d", len(warnings))
			}

			for _, s := range tt.contains {
				if !strings.Contains(warnings[0], s) {
					t.Errorf("warning should contain %q, got: %s", s, warnings[0])
				}
			}
		})
	}
}

func TestHandleAuthURL(t *testing.T) {
	filters, warnings := handleAuthURL(AuthURL, "https://auth.example.com/verify", nil)

	if len(filters) != 0 {
		t.Error("auth-url handler should not return filters")
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "extAuth") {
		t.Errorf("warning should mention extAuth, got: %s", warnings[0])
	}
}

func TestHandleBackendProtocol(t *testing.T) {
	tests := []struct {
		value    string
		contains string
	}{
		{"GRPC", "GRPCRoute"},
		{"grpc", "GRPCRoute"},
		{"GRPCS", "GRPCRoute"},
		{"HTTPS", "BackendTLSPolicy"},
		{"HTTP", ""}, // no warning for HTTP
		{"FCGI", "not supported"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			filters, warnings := handleBackendProtocol(BackendProtocol, tt.value, nil)

			if len(filters) != 0 {
				t.Error("backend-protocol handler should not return filters")
			}

			if tt.contains == "" {
				if len(warnings) != 0 {
					t.Errorf("expected no warnings for HTTP, got %d", len(warnings))
				}
				return
			}

			if len(warnings) != 1 {
				t.Fatalf("expected 1 warning, got %d", len(warnings))
			}
			if !strings.Contains(warnings[0], tt.contains) {
				t.Errorf("warning should contain %q, got: %s", tt.contains, warnings[0])
			}
		})
	}
}

func TestHandleCanaryEnabled(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		contains    []string
		noWarning   bool
	}{
		{
			name: "canary enabled only",
			annotations: map[string]string{
				Canary: "true",
			},
			contains: []string{"weighted"},
		},
		{
			name: "canary with weight",
			annotations: map[string]string{
				Canary:       "true",
				CanaryWeight: "20",
			},
			contains: []string{"weight=20"},
		},
		{
			name: "canary with header",
			annotations: map[string]string{
				Canary:              "true",
				CanaryByHeader:      "X-Canary",
				CanaryByHeaderValue: "always",
			},
			contains: []string{"header=X-Canary:always"},
		},
		{
			name: "canary with cookie",
			annotations: map[string]string{
				Canary:         "true",
				CanaryByCookie: "canary",
			},
			contains: []string{"cookie=canary"},
		},
		{
			name: "canary disabled",
			annotations: map[string]string{
				Canary: "false",
			},
			noWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.annotations[Canary]
			filters, warnings := handleCanaryEnabled(Canary, value, tt.annotations)

			if len(filters) != 0 {
				t.Error("canary handler should not return filters")
			}

			if tt.noWarning {
				if len(warnings) != 0 {
					t.Errorf("expected no warnings, got %d", len(warnings))
				}
				return
			}

			if len(warnings) != 1 {
				t.Fatalf("expected 1 warning, got %d", len(warnings))
			}

			for _, s := range tt.contains {
				if !strings.Contains(warnings[0], s) {
					t.Errorf("warning should contain %q, got: %s", s, warnings[0])
				}
			}
		})
	}
}

func TestIntegration_Phase3Annotations(t *testing.T) {
	c := NewConverter()

	t.Run("session affinity", func(t *testing.T) {
		result := c.Convert(map[string]string{
			Affinity:          "cookie",
			SessionCookieName: "SERVERID",
		})

		if len(result.Warnings) != 1 {
			t.Errorf("expected 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
		}
		if len(result.Processed) != 2 {
			t.Errorf("expected 2 processed, got %d", len(result.Processed))
		}
	})

	t.Run("authentication", func(t *testing.T) {
		result := c.Convert(map[string]string{
			AuthType:   "basic",
			AuthSecret: "my-secret",
		})

		if len(result.Warnings) != 1 {
			t.Errorf("expected 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
		}
		if len(result.Processed) != 2 {
			t.Errorf("expected 2 processed, got %d", len(result.Processed))
		}
	})

	t.Run("external auth", func(t *testing.T) {
		result := c.Convert(map[string]string{
			AuthURL: "https://auth.example.com/verify",
		})

		if len(result.Warnings) != 1 {
			t.Errorf("expected 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
		}
		if !strings.Contains(result.Warnings[0], "extAuth") {
			t.Errorf("warning should mention extAuth: %s", result.Warnings[0])
		}
	})

	t.Run("canary deployment", func(t *testing.T) {
		result := c.Convert(map[string]string{
			Canary:            "true",
			CanaryWeight:      "10",
			CanaryWeightTotal: "100",
		})

		if len(result.Warnings) != 1 {
			t.Errorf("expected 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
		}
		if len(result.Processed) != 3 {
			t.Errorf("expected 3 processed, got %d", len(result.Processed))
		}
	})

	t.Run("grpc backend", func(t *testing.T) {
		result := c.Convert(map[string]string{
			BackendProtocol: "GRPC",
		})

		if len(result.Warnings) != 1 {
			t.Errorf("expected 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
		}
		if !strings.Contains(result.Warnings[0], "GRPCRoute") {
			t.Errorf("warning should mention GRPCRoute: %s", result.Warnings[0])
		}
	})
}
