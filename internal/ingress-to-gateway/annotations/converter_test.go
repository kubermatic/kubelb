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

func TestConverter_NilAnnotations(t *testing.T) {
	c := NewConverter()
	result := c.Convert(nil)

	if len(result.Filters) != 0 {
		t.Errorf("expected no filters for nil annotations, got %d", len(result.Filters))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings for nil annotations, got %d", len(result.Warnings))
	}
}

func TestConverter_EmptyAnnotations(t *testing.T) {
	c := NewConverter()
	result := c.Convert(map[string]string{})

	if len(result.Filters) != 0 {
		t.Errorf("expected no filters for empty annotations, got %d", len(result.Filters))
	}
}

func TestConverter_UnknownAnnotations(t *testing.T) {
	c := NewConverter()
	result := c.Convert(map[string]string{
		"some-other-annotation":       "value",
		"kubernetes.io/ingress.class": "nginx",
	})

	// Unknown annotations should be silently ignored
	if len(result.Filters) != 0 {
		t.Errorf("expected no filters for unknown annotations, got %d", len(result.Filters))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings for unknown annotations, got %d", len(result.Warnings))
	}
}

func TestConverter_NotSupportedAnnotations(t *testing.T) {
	c := NewConverter()

	tests := []struct {
		name       string
		annotation string
		value      string
	}{
		{"server-snippet", ServerSnippet, "custom config"},
		{"configuration-snippet", ConfigurationSnippet, "custom config"},
		{"stream-snippet", StreamSnippet, "stream config"},
		{"enable-modsecurity", EnableModSecurity, "true"},
		{"enable-owasp-core-rules", EnableOWASPCoreRules, "true"},
		{"upstream-hash-by", UpstreamHashBy, "$request_uri"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Convert(map[string]string{tt.annotation: tt.value})

			if len(result.Filters) != 0 {
				t.Errorf("expected no filters for unsupported annotation, got %d", len(result.Filters))
			}
			if len(result.Warnings) != 1 {
				t.Fatalf("expected 1 warning for unsupported annotation, got %d", len(result.Warnings))
			}
		})
	}
}

func TestConverter_CanaryWarning(t *testing.T) {
	c := NewConverter()

	t.Run("canary enabled", func(t *testing.T) {
		result := c.Convert(map[string]string{
			Canary: "true",
		})

		if len(result.Warnings) != 1 {
			t.Fatalf("expected 1 warning for canary, got %d", len(result.Warnings))
		}
		if result.Warnings[0] != "canary ingresses should be converted to weighted HTTPRoute backends" {
			t.Errorf("unexpected warning: %s", result.Warnings[0])
		}
	})

	t.Run("canary disabled", func(t *testing.T) {
		result := c.Convert(map[string]string{
			Canary: "false",
		})

		if len(result.Warnings) != 0 {
			t.Errorf("expected no warnings for canary=false, got %d", len(result.Warnings))
		}
	})
}

func TestConverter_DeduplicateFilters(t *testing.T) {
	// Test that duplicate filter types are deduplicated
	filters := []gwapiv1.HTTPRouteFilter{
		{Type: gwapiv1.HTTPRouteFilterRequestRedirect},
		{Type: gwapiv1.HTTPRouteFilterURLRewrite},
		{Type: gwapiv1.HTTPRouteFilterRequestRedirect}, // duplicate
	}

	result := deduplicateFilters(filters)

	if len(result) != 2 {
		t.Errorf("expected 2 filters after dedup, got %d", len(result))
	}

	// Verify types
	types := make(map[gwapiv1.HTTPRouteFilterType]bool)
	for _, f := range result {
		types[f.Type] = true
	}
	if !types[gwapiv1.HTTPRouteFilterRequestRedirect] || !types[gwapiv1.HTTPRouteFilterURLRewrite] {
		t.Error("missing expected filter types after dedup")
	}
}

func TestConverter_ProcessedTracking(t *testing.T) {
	c := NewConverter()
	result := c.Convert(map[string]string{
		SSLRedirect:            "true",
		RewriteTarget:          "/api",
		"unrelated-annotation": "value",
	})

	// Should have processed 2 annotations
	if len(result.Processed) != 2 {
		t.Errorf("expected 2 processed annotations, got %d: %v", len(result.Processed), result.Processed)
	}
}

func TestConverterWithOptions_SkipPolicyWarnings(t *testing.T) {
	// Annotations that would normally generate policy warnings
	policyAnnotations := map[string]string{
		ProxyConnectTimeout:  "30",
		ProxyReadTimeout:     "60",
		ProxySendTimeout:     "60",
		EnableCORS:           "true",
		CORSAllowOrigin:      "*",
		LimitRPS:             "100",
		LimitConnections:     "50",
		WhitelistSourceRange: "10.0.0.0/8",
		DenylistSourceRange:  "192.168.0.0/16",
		AuthType:             "basic",
		AuthSecret:           "my-secret",
	}

	t.Run("default converter generates warnings", func(t *testing.T) {
		c := NewConverter()
		result := c.Convert(policyAnnotations)

		if len(result.Warnings) == 0 {
			t.Error("expected warnings from default converter, got none")
		}
	})

	t.Run("skip policy warnings suppresses warnings", func(t *testing.T) {
		c := NewConverterWithOptions(true)
		result := c.Convert(policyAnnotations)

		// Should have no warnings for auto-converted policy annotations
		for _, warning := range result.Warnings {
			t.Errorf("unexpected warning: %s", warning)
		}
	})

	t.Run("non-policy warnings still generated", func(t *testing.T) {
		c := NewConverterWithOptions(true)
		result := c.Convert(map[string]string{
			ServerSnippet: "some snippet", // not supported
			AuthType:      "digest",       // unsupported auth type
		})

		// Should still have warnings for unsupported annotations
		if len(result.Warnings) < 2 {
			t.Errorf("expected at least 2 warnings for unsupported annotations, got %d: %v",
				len(result.Warnings), result.Warnings)
		}
	})

	t.Run("auth-url still warns even with skip policy warnings", func(t *testing.T) {
		c := NewConverterWithOptions(true)
		result := c.Convert(map[string]string{
			AuthURL: "https://auth.example.com/verify",
		})

		// auth-url requires manual configuration, should still warn
		if len(result.Warnings) == 0 {
			t.Error("expected warning for auth-url, got none")
		}
	})
}
