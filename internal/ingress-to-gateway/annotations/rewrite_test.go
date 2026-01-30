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

func TestHandleRewriteTarget(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		annotations    map[string]string
		expectFilters  int
		expectWarnings int
		expectPath     string
	}{
		{
			name:          "simple rewrite to root",
			value:         "/",
			annotations:   nil,
			expectFilters: 1,
			expectPath:    "/",
		},
		{
			name:          "rewrite to path",
			value:         "/api/v1",
			annotations:   nil,
			expectFilters: 1,
			expectPath:    "/api/v1",
		},
		{
			name:           "rewrite with capture group",
			value:          "/$1",
			annotations:    nil,
			expectFilters:  1,
			expectWarnings: 1,
			expectPath:     "/", // capture group stripped
		},
		{
			name:           "rewrite with multiple capture groups",
			value:          "/api/$1/$2",
			annotations:    nil,
			expectFilters:  1,
			expectWarnings: 1,
			expectPath:     "/api//", // capture groups stripped
		},
		{
			name:  "rewrite with use-regex",
			value: "/newpath",
			annotations: map[string]string{
				UseRegex: "true",
			},
			expectFilters:  1,
			expectWarnings: 1, // warning about use-regex
			expectPath:     "/newpath",
		},
		{
			name:           "empty value",
			value:          "",
			annotations:    nil,
			expectFilters:  0,
			expectWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, warnings := handleRewriteTarget(RewriteTarget, tt.value, tt.annotations)

			if len(filters) != tt.expectFilters {
				t.Errorf("expected %d filters, got %d", tt.expectFilters, len(filters))
			}
			if len(warnings) != tt.expectWarnings {
				t.Errorf("expected %d warnings, got %d: %v", tt.expectWarnings, len(warnings), warnings)
			}

			if tt.expectFilters > 0 {
				f := filters[0]
				if f.Type != gwapiv1.HTTPRouteFilterURLRewrite {
					t.Errorf("expected URLRewrite filter, got %s", f.Type)
				}
				if f.URLRewrite == nil {
					t.Fatal("URLRewrite is nil")
				}
				if f.URLRewrite.Path == nil {
					t.Fatal("URLRewrite.Path is nil")
				}
				if f.URLRewrite.Path.Type != gwapiv1.PrefixMatchHTTPPathModifier {
					t.Errorf("expected PrefixMatch path modifier, got %s", f.URLRewrite.Path.Type)
				}
				if f.URLRewrite.Path.ReplacePrefixMatch == nil {
					t.Fatal("ReplacePrefixMatch is nil")
				}
				if *f.URLRewrite.Path.ReplacePrefixMatch != tt.expectPath {
					t.Errorf("expected path %q, got %q", tt.expectPath, *f.URLRewrite.Path.ReplacePrefixMatch)
				}
			}
		})
	}
}

func TestHandleAppRoot(t *testing.T) {
	// app-root now generates warning only, not filters
	// because NGINX app-root only redirects "/" but Gateway API filters
	// would apply to ALL routes
	tests := []struct {
		name           string
		value          string
		expectFilters  int
		expectWarnings int
	}{
		{
			name:           "app-root with leading slash",
			value:          "/app",
			expectFilters:  0, // warning only, no filter
			expectWarnings: 1,
		},
		{
			name:           "app-root without leading slash",
			value:          "dashboard",
			expectFilters:  0,
			expectWarnings: 1,
		},
		{
			name:           "app-root with nested path",
			value:          "/admin/dashboard",
			expectFilters:  0,
			expectWarnings: 1,
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
			filters, warnings := handleAppRoot(AppRoot, tt.value, nil)

			if len(filters) != tt.expectFilters {
				t.Errorf("expected %d filters, got %d", tt.expectFilters, len(filters))
			}
			if len(warnings) != tt.expectWarnings {
				t.Errorf("expected %d warnings, got %d: %v", tt.expectWarnings, len(warnings), warnings)
			}

			// Verify warning contains manual HTTPRoute instructions
			if tt.value != "" && len(warnings) > 0 {
				if !contains(warnings[0], "manual HTTPRoute") {
					t.Errorf("expected warning to contain 'manual HTTPRoute', got: %s", warnings[0])
				}
			}
		})
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestContainsCaptureGroup(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"/$1", true},
		{"/$2", true},
		{"/api/$1/users", true},
		{"/$1$2", true},
		{"/", false},
		{"/api", false},
		{"/api/v1", false},
		{"$", false},      // $ without digit
		{"$abc", false},   // $ with letters
		{"/path$", false}, // $ at end without digit
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			result := containsCaptureGroup(tt.value)
			if result != tt.expected {
				t.Errorf("containsCaptureGroup(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestStripCaptureGroups(t *testing.T) {
	tests := []struct {
		value    string
		expected string
	}{
		{"/$1", "/"},
		{"/$2", "/"},
		{"/api/$1", "/api/"},
		{"/$1/$2", "//"},
		{"/api/$1/users/$2", "/api//users/"},
		{"/", "/"},
		{"/api", "/api"},
		{"$1", "/"},   // only capture group results in /
		{"/$12", "/"}, // multi-digit capture group
		{"/prefix$1suffix", "/prefixsuffix"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			result := stripCaptureGroups(tt.value)
			if result != tt.expected {
				t.Errorf("stripCaptureGroups(%q) = %q, want %q", tt.value, result, tt.expected)
			}
		})
	}
}

func TestIntegration_RewriteWithConverter(t *testing.T) {
	c := NewConverter()

	t.Run("simple rewrite", func(t *testing.T) {
		result := c.Convert(map[string]string{
			RewriteTarget: "/api",
		})

		if len(result.Filters) != 1 {
			t.Fatalf("expected 1 filter, got %d", len(result.Filters))
		}

		f := result.Filters[0]
		if f.Type != gwapiv1.HTTPRouteFilterURLRewrite {
			t.Errorf("expected URLRewrite filter, got %s", f.Type)
		}
	})

	t.Run("app-root", func(t *testing.T) {
		result := c.Convert(map[string]string{
			AppRoot: "/dashboard",
		})

		// app-root now generates warning only, not filters
		if len(result.Filters) != 0 {
			t.Errorf("expected 0 filters (warning only), got %d", len(result.Filters))
		}

		if len(result.Warnings) != 1 {
			t.Errorf("expected 1 warning, got %d", len(result.Warnings))
		}
	})
}

func TestIntegration_ComplexAnnotationSet(t *testing.T) {
	c := NewConverter()
	result := c.Convert(map[string]string{
		SSLRedirect:            "true",
		RewriteTarget:          "/api/$1",
		ServerSnippet:          "custom config",
		ConfigurationSnippet:   "location config",
		"unrelated-annotation": "ignored",
	})

	// Should have 2 filters (ssl-redirect + rewrite)
	if len(result.Filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(result.Filters))
	}

	// Should have warnings for:
	// - capture group in rewrite-target
	// - server-snippet not supported
	// - configuration-snippet not supported
	if len(result.Warnings) < 3 {
		t.Errorf("expected at least 3 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}

	// Should have processed 4 annotations (not the unrelated one)
	if len(result.Processed) != 4 {
		t.Errorf("expected 4 processed annotations, got %d: %v", len(result.Processed), result.Processed)
	}
}
