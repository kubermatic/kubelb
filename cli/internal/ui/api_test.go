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

package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8c.io/kubelb/cli/internal/ingress"
	"k8c.io/kubelb/pkg/conversion"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMatchesIngressClass(t *testing.T) {
	tests := []struct {
		name     string
		ingress  *networkingv1.Ingress
		class    string
		expected bool
	}{
		{
			name: "matches spec",
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					IngressClassName: strPtr("nginx"),
				},
			},
			class:    "nginx",
			expected: true,
		},
		{
			name: "matches annotation",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						conversion.AnnotationIngressClass: "nginx",
					},
				},
			},
			class:    "nginx",
			expected: true,
		},
		{
			name:     "no class",
			ingress:  &networkingv1.Ingress{},
			class:    "nginx",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesIngressClass(tt.ingress, tt.class)
			if result != tt.expected {
				t.Errorf("matchesIngressClass() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractInfo(t *testing.T) {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
			Annotations: map[string]string{
				conversion.AnnotationConversionStatus:   "converted",
				conversion.AnnotationConvertedHTTPRoute: "default/test",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{Host: "test.example.com"},
			},
		},
	}

	info := extractInfo(ing)

	if info.Name != "test" {
		t.Errorf("Name = %v, want test", info.Name)
	}
	if info.Namespace != "default" {
		t.Errorf("Namespace = %v, want default", info.Namespace)
	}
	if info.Status != ingress.StatusConverted {
		t.Errorf("Status = %v, want converted", info.Status)
	}
	if len(info.Hosts) != 1 || info.Hosts[0] != "test.example.com" {
		t.Errorf("Hosts = %v, want [test.example.com]", info.Hosts)
	}
	if len(info.HTTPRoutes) != 1 || info.HTTPRoutes[0] != "default/test" {
		t.Errorf("HTTPRoutes = %v, want [default/test]", info.HTTPRoutes)
	}
}

func TestCalculateSummary(t *testing.T) {
	infos := []ingress.Info{
		{Status: ingress.StatusConverted},
		{Status: ingress.StatusNew},
		{Status: ingress.StatusFailed},
	}

	summary := calculateSummary(infos)

	if summary.Total != 3 {
		t.Errorf("Total = %v, want 3", summary.Total)
	}
	if summary.Converted != 1 {
		t.Errorf("Converted = %v, want 1", summary.Converted)
	}
	if summary.New != 1 {
		t.Errorf("New = %v, want 1", summary.New)
	}
	if summary.Failed != 1 {
		t.Errorf("Failed = %v, want 1", summary.Failed)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}

	writeJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %v, want application/json", contentType)
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("Response = %v, want {key: value}", result)
	}
}

func TestHandleAnnotations(t *testing.T) {
	server := &Server{
		opts: &conversion.Options{},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/annotations", nil)
	w := httptest.NewRecorder()

	server.handleAnnotations(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp AnnotationsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Converted) == 0 {
		t.Error("Expected some converted annotations")
	}
	if len(resp.Policy) == 0 {
		t.Error("Expected some policy annotations")
	}
	if len(resp.Unsupported) == 0 {
		t.Error("Expected some unsupported annotations")
	}
}

func strPtr(s string) *string {
	return &s
}
