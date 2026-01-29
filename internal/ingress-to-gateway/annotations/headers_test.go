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
