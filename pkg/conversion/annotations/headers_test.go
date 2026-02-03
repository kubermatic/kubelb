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

	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestHandleProxySetHeaders(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		expectFilters  int
		expectWarnings int
		expectHeaders  int
	}{
		{
			name:          "single header",
			value:         "X-Custom:value",
			expectFilters: 1,
			expectHeaders: 1,
		},
		{
			name:          "multiple headers",
			value:         "X-Header1:value1,X-Header2:value2",
			expectFilters: 1,
			expectHeaders: 2,
		},
		{
			name:           "configmap reference",
			value:          "namespace/configmap-name",
			expectFilters:  0,
			expectWarnings: 1,
		},
		{
			name:          "empty value",
			value:         "",
			expectFilters: 0,
		},
		{
			name:           "invalid format",
			value:          "invalid-no-colon",
			expectFilters:  0,
			expectWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, warnings := handleProxySetHeaders(ProxySetHeaders, tt.value, nil)

			if len(filters) != tt.expectFilters {
				t.Errorf("expected %d filters, got %d", tt.expectFilters, len(filters))
			}
			if len(warnings) != tt.expectWarnings {
				t.Errorf("expected %d warnings, got %d: %v", tt.expectWarnings, len(warnings), warnings)
			}

			if tt.expectFilters > 0 && tt.expectHeaders > 0 {
				f := filters[0]
				if f.Type != gwapiv1.HTTPRouteFilterRequestHeaderModifier {
					t.Errorf("expected RequestHeaderModifier filter, got %s", f.Type)
				}
				if f.RequestHeaderModifier == nil {
					t.Fatal("RequestHeaderModifier is nil")
				}
				if len(f.RequestHeaderModifier.Set) != tt.expectHeaders {
					t.Errorf("expected %d headers, got %d", tt.expectHeaders, len(f.RequestHeaderModifier.Set))
				}
			}
		})
	}
}

func TestHandleCustomHeaders(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		expectFilters  int
		expectWarnings int
		expectHeaders  int
	}{
		{
			name:          "single response header",
			value:         "X-Frame-Options:DENY",
			expectFilters: 1,
			expectHeaders: 1,
		},
		{
			name:          "multiple response headers",
			value:         "X-Content-Type-Options:nosniff,X-XSS-Protection:1",
			expectFilters: 1,
			expectHeaders: 2,
		},
		{
			name:           "configmap reference",
			value:          "default/custom-headers",
			expectFilters:  0,
			expectWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, warnings := handleCustomHeaders(CustomHeaders, tt.value, nil)

			if len(filters) != tt.expectFilters {
				t.Errorf("expected %d filters, got %d", tt.expectFilters, len(filters))
			}
			if len(warnings) != tt.expectWarnings {
				t.Errorf("expected %d warnings, got %d: %v", tt.expectWarnings, len(warnings), warnings)
			}

			if tt.expectFilters > 0 {
				f := filters[0]
				if f.Type != gwapiv1.HTTPRouteFilterResponseHeaderModifier {
					t.Errorf("expected ResponseHeaderModifier filter, got %s", f.Type)
				}
				if f.ResponseHeaderModifier == nil {
					t.Fatal("ResponseHeaderModifier is nil")
				}
				if len(f.ResponseHeaderModifier.Add) != tt.expectHeaders {
					t.Errorf("expected %d headers, got %d", tt.expectHeaders, len(f.ResponseHeaderModifier.Add))
				}
			}
		})
	}
}

func TestParseHeaderList(t *testing.T) {
	tests := []struct {
		input          string
		expectHeaders  int
		expectWarnings int
	}{
		{"X-Header:value", 1, 0},
		{"H1:v1,H2:v2,H3:v3", 3, 0},
		{"  H1 : v1 , H2 : v2  ", 2, 0},
		{"invalid", 0, 1},
		{"", 0, 0},
		{":value", 0, 1}, // empty name
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			headers, warnings := parseHeaderList(tt.input)
			if len(headers) != tt.expectHeaders {
				t.Errorf("expected %d headers, got %d", tt.expectHeaders, len(headers))
			}
			if len(warnings) != tt.expectWarnings {
				t.Errorf("expected %d warnings, got %d", tt.expectWarnings, len(warnings))
			}
		})
	}
}

func TestHandlePreserveHost(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		expectWarnings int
	}{
		{
			name:           "true (default behavior)",
			value:          "true",
			expectWarnings: 0,
		},
		{
			name:           "empty (default behavior)",
			value:          "",
			expectWarnings: 0,
		},
		{
			name:           "false generates warning",
			value:          "false",
			expectWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, warnings := handlePreserveHost(PreserveHost, tt.value, nil)

			if len(filters) != 0 {
				t.Errorf("expected 0 filters, got %d", len(filters))
			}
			if len(warnings) != tt.expectWarnings {
				t.Errorf("expected %d warnings, got %d: %v", tt.expectWarnings, len(warnings), warnings)
			}
			if tt.expectWarnings > 0 && len(warnings) > 0 {
				if !strings.Contains(warnings[0], "URLRewrite") {
					t.Errorf("warning should mention URLRewrite, got: %s", warnings[0])
				}
			}
		})
	}
}

func TestIntegration_HeadersWithConverter(t *testing.T) {
	c := NewConverter()

	t.Run("request headers", func(t *testing.T) {
		result := c.Convert(map[string]string{
			ProxySetHeaders: "X-Custom:test",
		})

		if len(result.Filters) != 1 {
			t.Fatalf("expected 1 filter, got %d", len(result.Filters))
		}
		if result.Filters[0].Type != gwapiv1.HTTPRouteFilterRequestHeaderModifier {
			t.Errorf("expected RequestHeaderModifier, got %s", result.Filters[0].Type)
		}
	})

	t.Run("response headers", func(t *testing.T) {
		result := c.Convert(map[string]string{
			CustomHeaders: "X-Frame-Options:DENY",
		})

		if len(result.Filters) != 1 {
			t.Fatalf("expected 1 filter, got %d", len(result.Filters))
		}
		if result.Filters[0].Type != gwapiv1.HTTPRouteFilterResponseHeaderModifier {
			t.Errorf("expected ResponseHeaderModifier, got %s", result.Filters[0].Type)
		}
	})
}

func TestHandleHSTS(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		annotations    map[string]string
		expectFilters  int
		expectWarnings int
		expectedHeader string
	}{
		{
			name:           "HSTS disabled",
			value:          "false",
			annotations:    nil,
			expectFilters:  0,
			expectWarnings: 0,
		},
		{
			name:           "HSTS enabled with defaults",
			value:          "true",
			annotations:    map[string]string{},
			expectFilters:  1,
			expectWarnings: 0,
			expectedHeader: "max-age=31536000",
		},
		{
			name:  "HSTS with custom max-age",
			value: "true",
			annotations: map[string]string{
				HSTSMaxAge: "86400",
			},
			expectFilters:  1,
			expectWarnings: 0,
			expectedHeader: "max-age=86400",
		},
		{
			name:  "HSTS with includeSubDomains",
			value: "true",
			annotations: map[string]string{
				HSTSIncludeSubdomains: "true",
			},
			expectFilters:  1,
			expectWarnings: 0,
			expectedHeader: "max-age=31536000; includeSubDomains",
		},
		{
			name:  "HSTS with preload",
			value: "true",
			annotations: map[string]string{
				HSTSPreload: "true",
			},
			expectFilters:  1,
			expectWarnings: 0,
			expectedHeader: "max-age=31536000; preload",
		},
		{
			name:  "HSTS with all options",
			value: "true",
			annotations: map[string]string{
				HSTSMaxAge:            "63072000",
				HSTSIncludeSubdomains: "true",
				HSTSPreload:           "true",
			},
			expectFilters:  1,
			expectWarnings: 0,
			expectedHeader: "max-age=63072000; includeSubDomains; preload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, warnings := handleHSTS(HSTS, tt.value, tt.annotations)

			if len(filters) != tt.expectFilters {
				t.Errorf("expected %d filters, got %d", tt.expectFilters, len(filters))
			}
			if len(warnings) != tt.expectWarnings {
				t.Errorf("expected %d warnings, got %d: %v", tt.expectWarnings, len(warnings), warnings)
			}

			if tt.expectFilters > 0 {
				f := filters[0]
				if f.Type != gwapiv1.HTTPRouteFilterResponseHeaderModifier {
					t.Errorf("expected ResponseHeaderModifier filter, got %s", f.Type)
				}
				if f.ResponseHeaderModifier == nil {
					t.Fatal("ResponseHeaderModifier is nil")
				}
				if len(f.ResponseHeaderModifier.Set) != 1 {
					t.Fatalf("expected 1 header, got %d", len(f.ResponseHeaderModifier.Set))
				}
				header := f.ResponseHeaderModifier.Set[0]
				if header.Name != "Strict-Transport-Security" {
					t.Errorf("expected Strict-Transport-Security header, got %s", header.Name)
				}
				if header.Value != tt.expectedHeader {
					t.Errorf("expected header value %q, got %q", tt.expectedHeader, header.Value)
				}
			}
		})
	}
}

func TestBuildHSTSHeaderValue(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    string
	}{
		{
			name:        "defaults",
			annotations: map[string]string{},
			expected:    "max-age=31536000",
		},
		{
			name:        "custom max-age",
			annotations: map[string]string{HSTSMaxAge: "86400"},
			expected:    "max-age=86400",
		},
		{
			name:        "with includeSubDomains",
			annotations: map[string]string{HSTSIncludeSubdomains: "true"},
			expected:    "max-age=31536000; includeSubDomains",
		},
		{
			name:        "with preload",
			annotations: map[string]string{HSTSPreload: "true"},
			expected:    "max-age=31536000; preload",
		},
		{
			name: "full config",
			annotations: map[string]string{
				HSTSMaxAge:            "63072000",
				HSTSIncludeSubdomains: "true",
				HSTSPreload:           "true",
			},
			expected: "max-age=63072000; includeSubDomains; preload",
		},
		{
			name:        "includeSubDomains false",
			annotations: map[string]string{HSTSIncludeSubdomains: "false"},
			expected:    "max-age=31536000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildHSTSHeaderValue(tt.annotations)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestIntegration_HSTSWithConverter(t *testing.T) {
	c := NewConverter()

	result := c.Convert(map[string]string{
		HSTS:                  "true",
		HSTSMaxAge:            "86400",
		HSTSIncludeSubdomains: "true",
	})

	if len(result.Filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(result.Filters))
	}
	if result.Filters[0].Type != gwapiv1.HTTPRouteFilterResponseHeaderModifier {
		t.Errorf("expected ResponseHeaderModifier, got %s", result.Filters[0].Type)
	}

	// Should have processed 3 annotations
	if len(result.Processed) != 3 {
		t.Errorf("expected 3 processed annotations, got %d: %v", len(result.Processed), result.Processed)
	}

	// Verify header value
	header := result.Filters[0].ResponseHeaderModifier.Set[0]
	expected := "max-age=86400; includeSubDomains"
	if header.Value != expected {
		t.Errorf("expected %q, got %q", expected, header.Value)
	}
}
