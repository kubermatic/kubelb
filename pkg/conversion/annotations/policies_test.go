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

func TestHandleTimeouts(t *testing.T) {
	tests := []struct {
		name     string
		handler  AnnotationHandler
		key      string
		value    string
		contains string
	}{
		{
			name:     "connect timeout",
			handler:  handleProxyConnectTimeout,
			key:      ProxyConnectTimeout,
			value:    "60",
			contains: "BackendTrafficPolicy",
		},
		{
			name:     "read timeout with unit",
			handler:  handleProxyReadTimeout,
			key:      ProxyReadTimeout,
			value:    "30s",
			contains: "requestTimeout=30s",
		},
		{
			name:     "send timeout",
			handler:  handleProxySendTimeout,
			key:      ProxySendTimeout,
			value:    "120",
			contains: "120s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, warnings := tt.handler(tt.key, tt.value, nil)

			if len(filters) != 0 {
				t.Error("timeout handlers should not return filters")
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

func TestHandleEnableCORS(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		contains    []string
	}{
		{
			name: "cors enabled only",
			annotations: map[string]string{
				EnableCORS: "true",
			},
			contains: []string{"SecurityPolicy", "cors"},
		},
		{
			name: "cors with all options",
			annotations: map[string]string{
				EnableCORS:           "true",
				CORSAllowOrigin:      "https://example.com",
				CORSAllowMethods:     "GET,POST",
				CORSAllowHeaders:     "Content-Type",
				CORSExposeHeaders:    "X-Custom",
				CORSMaxAge:           "3600",
				CORSAllowCredentials: "true",
			},
			contains: []string{"allowOrigins", "allowMethods", "allowHeaders", "maxAge", "allowCredentials"},
		},
		{
			name: "cors disabled",
			annotations: map[string]string{
				EnableCORS: "false",
			},
			contains: nil, // no warnings
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.annotations[EnableCORS]
			filters, warnings := handleEnableCORS(EnableCORS, value, tt.annotations)

			if len(filters) != 0 {
				t.Error("CORS handler should not return filters")
			}

			if tt.contains == nil {
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

func TestHandleRateLimiting(t *testing.T) {
	tests := []struct {
		name     string
		handler  AnnotationHandler
		key      string
		value    string
		contains string
	}{
		{
			name:     "limit-rps",
			handler:  handleLimitRPS,
			key:      LimitRPS,
			value:    "100",
			contains: "rateLimit",
		},
		{
			name:     "limit-connections",
			handler:  handleLimitConnections,
			key:      LimitConnections,
			value:    "50",
			contains: "maxConnections",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, warnings := tt.handler(tt.key, tt.value, nil)

			if len(filters) != 0 {
				t.Error("rate limit handlers should not return filters")
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

func TestHandleIPAccess(t *testing.T) {
	tests := []struct {
		name    string
		handler AnnotationHandler
		key     string
		value   string
		action  string
	}{
		{
			name:    "whitelist",
			handler: handleWhitelistSourceRange,
			key:     WhitelistSourceRange,
			value:   "10.0.0.0/8,192.168.0.0/16",
			action:  "Allow",
		},
		{
			name:    "denylist",
			handler: handleDenylistSourceRange,
			key:     DenylistSourceRange,
			value:   "0.0.0.0/0",
			action:  "Deny",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, warnings := tt.handler(tt.key, tt.value, nil)

			if len(filters) != 0 {
				t.Error("IP access handlers should not return filters")
			}
			if len(warnings) != 1 {
				t.Fatalf("expected 1 warning, got %d", len(warnings))
			}
			if !strings.Contains(warnings[0], tt.action) {
				t.Errorf("warning should contain action %q, got: %s", tt.action, warnings[0])
			}
			if !strings.Contains(warnings[0], "SecurityPolicy") {
				t.Errorf("warning should mention SecurityPolicy, got: %s", warnings[0])
			}
		})
	}
}

func TestHandleProxyBodySize(t *testing.T) {
	filters, warnings := handleProxyBodySize(ProxyBodySize, "10m", nil)

	if len(filters) != 0 {
		t.Error("body size handler should not return filters")
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "ClientTrafficPolicy") {
		t.Errorf("warning should mention ClientTrafficPolicy, got: %s", warnings[0])
	}
}

func TestNormalizeTimeout(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"60", "60s"},
		{"60s", "60s"},
		{"5m", "5m"},
		{"1h", "1h"},
		{"invalid", "invalid"},
	}

	for _, tt := range tests {
		result := normalizeTimeout(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeTimeout(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatStringList(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"a,b,c", `"a", "b", "c"`},
		{"single", `"single"`},
		{" a , b ", `"a", "b"`},
		{"", ""},
	}

	for _, tt := range tests {
		result := formatStringList(tt.input)
		if result != tt.expected {
			t.Errorf("formatStringList(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIntegration_Phase2Annotations(t *testing.T) {
	c := NewConverter()

	t.Run("timeout annotations", func(t *testing.T) {
		result := c.Convert(map[string]string{
			ProxyConnectTimeout: "30",
			ProxyReadTimeout:    "60s",
		})

		// Should have 2 warnings (policy suggestions)
		if len(result.Warnings) != 2 {
			t.Errorf("expected 2 warnings, got %d: %v", len(result.Warnings), result.Warnings)
		}
		// Should have 2 processed
		if len(result.Processed) != 2 {
			t.Errorf("expected 2 processed, got %d", len(result.Processed))
		}
	})

	t.Run("cors annotations", func(t *testing.T) {
		result := c.Convert(map[string]string{
			EnableCORS:       "true",
			CORSAllowOrigin:  "*",
			CORSAllowMethods: "GET,POST",
		})

		// Should have 1 warning (CORS config aggregated)
		if len(result.Warnings) != 1 {
			t.Errorf("expected 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
		}
		// Should have processed all 3
		if len(result.Processed) != 3 {
			t.Errorf("expected 3 processed, got %d", len(result.Processed))
		}
	})

	t.Run("mixed phase 1 and 2", func(t *testing.T) {
		result := c.Convert(map[string]string{
			SSLRedirect:          "true",
			ProxyConnectTimeout:  "30",
			WhitelistSourceRange: "10.0.0.0/8",
		})

		// Should have 1 filter (ssl-redirect) and 2 warnings
		if len(result.Filters) != 1 {
			t.Errorf("expected 1 filter, got %d", len(result.Filters))
		}
		if len(result.Warnings) != 2 {
			t.Errorf("expected 2 warnings, got %d: %v", len(result.Warnings), result.Warnings)
		}
	})
}
