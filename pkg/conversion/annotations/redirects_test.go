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
	"testing"

	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestHandleSSLRedirect(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		annotations    map[string]string
		expectFilters  int
		expectWarnings int
		expectScheme   string
		expectCode     int
	}{
		{
			name:          "ssl-redirect true",
			value:         "true",
			annotations:   nil,
			expectFilters: 1,
			expectScheme:  "https",
			expectCode:    301,
		},
		{
			name:          "ssl-redirect false",
			value:         "false",
			annotations:   nil,
			expectFilters: 0,
		},
		{
			name:  "ssl-redirect with force-ssl-redirect",
			value: "true",
			annotations: map[string]string{
				ForceSSLRedirect: "true",
			},
			expectFilters: 0, // force-ssl-redirect takes precedence
		},
		{
			name:          "ssl-redirect empty value",
			value:         "",
			annotations:   nil,
			expectFilters: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, warnings := handleSSLRedirect(SSLRedirect, tt.value, tt.annotations)

			if len(filters) != tt.expectFilters {
				t.Errorf("expected %d filters, got %d", tt.expectFilters, len(filters))
			}
			if len(warnings) != tt.expectWarnings {
				t.Errorf("expected %d warnings, got %d", tt.expectWarnings, len(warnings))
			}

			if tt.expectFilters > 0 {
				f := filters[0]
				if f.Type != gwapiv1.HTTPRouteFilterRequestRedirect {
					t.Errorf("expected RequestRedirect filter, got %s", f.Type)
				}
				if f.RequestRedirect == nil {
					t.Fatal("RequestRedirect is nil")
				}
				if f.RequestRedirect.Scheme == nil || *f.RequestRedirect.Scheme != tt.expectScheme {
					t.Errorf("expected scheme %q, got %v", tt.expectScheme, f.RequestRedirect.Scheme)
				}
				if f.RequestRedirect.StatusCode == nil || *f.RequestRedirect.StatusCode != tt.expectCode {
					t.Errorf("expected status code %d, got %v", tt.expectCode, f.RequestRedirect.StatusCode)
				}
			}
		})
	}
}

func TestHandleForceSSLRedirect(t *testing.T) {
	tests := []struct {
		name          string
		value         string
		expectFilters int
	}{
		{"force-ssl-redirect true", "true", 1},
		{"force-ssl-redirect false", "false", 0},
		{"force-ssl-redirect empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, warnings := handleForceSSLRedirect(ForceSSLRedirect, tt.value, nil)

			if len(filters) != tt.expectFilters {
				t.Errorf("expected %d filters, got %d", tt.expectFilters, len(filters))
			}
			if len(warnings) != 0 {
				t.Errorf("expected no warnings, got %d", len(warnings))
			}

			if tt.expectFilters > 0 {
				f := filters[0]
				if f.RequestRedirect.Scheme == nil || *f.RequestRedirect.Scheme != "https" {
					t.Error("expected https scheme")
				}
			}
		})
	}
}

func TestHandlePermanentRedirect(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		annotations    map[string]string
		expectFilters  int
		expectWarnings int
		expectScheme   string
		expectHost     string
		expectPath     string
		expectCode     int
	}{
		{
			name:          "simple URL redirect",
			value:         "https://example.com/new",
			annotations:   nil,
			expectFilters: 1,
			expectScheme:  "https",
			expectHost:    "example.com",
			expectPath:    "/new",
			expectCode:    301,
		},
		{
			name:  "redirect with custom status code",
			value: "https://example.com/moved",
			annotations: map[string]string{
				PermanentRedirectCode: "308",
			},
			expectFilters: 1,
			expectCode:    301, // 308 maps to 301 in Gateway API
		},
		{
			name:          "redirect to root",
			value:         "https://newdomain.com/",
			annotations:   nil,
			expectFilters: 1,
			expectScheme:  "https",
			expectHost:    "newdomain.com",
			expectCode:    301,
		},
		{
			name:           "empty value",
			value:          "",
			annotations:    nil,
			expectFilters:  0,
			expectWarnings: 1,
		},
		{
			name:           "invalid URL",
			value:          "://invalid",
			annotations:    nil,
			expectFilters:  0,
			expectWarnings: 1,
		},
		{
			name:          "URL with port",
			value:         "https://example.com:8443/api",
			annotations:   nil,
			expectFilters: 1,
			expectScheme:  "https",
			expectHost:    "example.com",
			expectPath:    "/api",
			expectCode:    301,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, warnings := handlePermanentRedirect(PermanentRedirect, tt.value, tt.annotations)

			if len(filters) != tt.expectFilters {
				t.Errorf("expected %d filters, got %d", tt.expectFilters, len(filters))
			}
			if len(warnings) != tt.expectWarnings {
				t.Errorf("expected %d warnings, got %d: %v", tt.expectWarnings, len(warnings), warnings)
			}

			if tt.expectFilters > 0 {
				f := filters[0]
				if f.Type != gwapiv1.HTTPRouteFilterRequestRedirect {
					t.Errorf("expected RequestRedirect filter, got %s", f.Type)
				}
				if f.RequestRedirect == nil {
					t.Fatal("RequestRedirect is nil")
				}

				if tt.expectScheme != "" {
					if f.RequestRedirect.Scheme == nil || *f.RequestRedirect.Scheme != tt.expectScheme {
						t.Errorf("expected scheme %q, got %v", tt.expectScheme, f.RequestRedirect.Scheme)
					}
				}
				if tt.expectHost != "" {
					if f.RequestRedirect.Hostname == nil || string(*f.RequestRedirect.Hostname) != tt.expectHost {
						t.Errorf("expected hostname %q, got %v", tt.expectHost, f.RequestRedirect.Hostname)
					}
				}
				if tt.expectPath != "" && tt.expectPath != "/" {
					if f.RequestRedirect.Path == nil || f.RequestRedirect.Path.ReplaceFullPath == nil {
						t.Errorf("expected path %q, got nil", tt.expectPath)
					} else if *f.RequestRedirect.Path.ReplaceFullPath != tt.expectPath {
						t.Errorf("expected path %q, got %q", tt.expectPath, *f.RequestRedirect.Path.ReplaceFullPath)
					}
				}
				if tt.expectCode != 0 {
					if f.RequestRedirect.StatusCode == nil || *f.RequestRedirect.StatusCode != tt.expectCode {
						t.Errorf("expected status code %d, got %v", tt.expectCode, f.RequestRedirect.StatusCode)
					}
				}
			}
		})
	}
}

func TestHandleTemporalRedirect(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		expectFilters  int
		expectWarnings int
		expectCode     int
	}{
		{
			name:          "simple temporal redirect",
			value:         "https://temp.example.com/maintenance",
			expectFilters: 1,
			expectCode:    302,
		},
		{
			name:           "empty value",
			value:          "",
			expectFilters:  0,
			expectWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, warnings := handleTemporalRedirect(TemporalRedirect, tt.value, nil)

			if len(filters) != tt.expectFilters {
				t.Errorf("expected %d filters, got %d", tt.expectFilters, len(filters))
			}
			if len(warnings) != tt.expectWarnings {
				t.Errorf("expected %d warnings, got %d", tt.expectWarnings, len(warnings))
			}

			if tt.expectFilters > 0 && tt.expectCode != 0 {
				f := filters[0]
				if f.RequestRedirect.StatusCode == nil || *f.RequestRedirect.StatusCode != tt.expectCode {
					t.Errorf("expected status code %d, got %v", tt.expectCode, f.RequestRedirect.StatusCode)
				}
			}
		})
	}
}

func TestStatusCodeToGatewayAPI(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{301, 301},
		{302, 302},
		{303, 302},
		{307, 302},
		{308, 301},
		{200, 302}, // invalid, defaults to 302
	}

	for _, tt := range tests {
		result := statusCodeToGatewayAPI(tt.input)
		if result != tt.expected {
			t.Errorf("statusCodeToGatewayAPI(%d) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestIsValidRedirectCode(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{301, true},
		{302, true},
		{307, true},
		{308, true},
		{200, false},
		{404, false},
		{500, false},
	}

	for _, tt := range tests {
		result := isValidRedirectCode(tt.code)
		if result != tt.expected {
			t.Errorf("isValidRedirectCode(%d) = %v, want %v", tt.code, result, tt.expected)
		}
	}
}

func TestIntegration_SSLAndRewriteTogether(t *testing.T) {
	c := NewConverter()
	result := c.Convert(map[string]string{
		SSLRedirect:   "true",
		RewriteTarget: "/api",
	})

	// Should have 2 filters
	if len(result.Filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(result.Filters))
	}

	// Verify filter types
	types := make(map[gwapiv1.HTTPRouteFilterType]bool)
	for _, f := range result.Filters {
		types[f.Type] = true
	}

	if !types[gwapiv1.HTTPRouteFilterRequestRedirect] {
		t.Error("missing RequestRedirect filter")
	}
	if !types[gwapiv1.HTTPRouteFilterURLRewrite] {
		t.Error("missing URLRewrite filter")
	}
}
