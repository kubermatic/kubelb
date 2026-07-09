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

package ingress

import (
	"testing"

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
			name: "matches spec.ingressClassName",
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					IngressClassName: strPtr("nginx"),
				},
			},
			class:    "nginx",
			expected: true,
		},
		{
			name: "does not match spec.ingressClassName",
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					IngressClassName: strPtr("nginx"),
				},
			},
			class:    "traefik",
			expected: false,
		},
		{
			name: "matches legacy annotation",
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
			name: "does not match legacy annotation",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						conversion.AnnotationIngressClass: "nginx",
					},
				},
			},
			class:    "traefik",
			expected: false,
		},
		{
			name: "spec takes precedence over annotation",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						conversion.AnnotationIngressClass: "traefik",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: strPtr("nginx"),
				},
			},
			class:    "nginx",
			expected: true,
		},
		{
			name:     "no class specified",
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

func TestExtractIngressInfo(t *testing.T) {
	tests := []struct {
		name     string
		ingress  *networkingv1.Ingress
		expected Info
	}{
		{
			name: "new ingress with hosts",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{Host: "app.example.com"},
						{Host: "www.example.com"},
					},
				},
			},
			expected: Info{
				Name:      "my-app",
				Namespace: "default",
				Status:    StatusNew,
				Hosts:     []string{"app.example.com", "www.example.com"},
			},
		},
		{
			name: "converted ingress with routes",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						conversion.AnnotationConversionStatus:   "converted",
						conversion.AnnotationConvertedHTTPRoute: "default/my-app",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{Host: "app.example.com"},
					},
				},
			},
			expected: Info{
				Name:       "my-app",
				Namespace:  "default",
				Status:     StatusConverted,
				Hosts:      []string{"app.example.com"},
				HTTPRoutes: []string{"default/my-app"},
			},
		},
		{
			name: "skipped ingress with reason",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "canary-app",
					Namespace: "default",
					Annotations: map[string]string{
						conversion.AnnotationConversionStatus:     "skipped",
						conversion.AnnotationConversionSkipReason: "canary annotations are not supported",
					},
				},
			},
			expected: Info{
				Name:       "canary-app",
				Namespace:  "default",
				Status:     StatusSkipped,
				SkipReason: "canary annotations are not supported",
			},
		},
		{
			name: "ingress with warnings",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					Annotations: map[string]string{
						conversion.AnnotationConversionStatus:   "partial",
						conversion.AnnotationConversionWarnings: "warning1;warning2",
					},
				},
			},
			expected: Info{
				Name:      "my-app",
				Namespace: "default",
				Status:    StatusPartial,
				Warnings:  []string{"warning1", "warning2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractInfo(tt.ingress)
			if result.Name != tt.expected.Name {
				t.Errorf("Name = %v, want %v", result.Name, tt.expected.Name)
			}
			if result.Namespace != tt.expected.Namespace {
				t.Errorf("Namespace = %v, want %v", result.Namespace, tt.expected.Namespace)
			}
			if result.Status != tt.expected.Status {
				t.Errorf("Status = %v, want %v", result.Status, tt.expected.Status)
			}
			if len(result.Hosts) != len(tt.expected.Hosts) {
				t.Errorf("Hosts count = %v, want %v", len(result.Hosts), len(tt.expected.Hosts))
			}
			if len(result.HTTPRoutes) != len(tt.expected.HTTPRoutes) {
				t.Errorf("HTTPRoutes count = %v, want %v", len(result.HTTPRoutes), len(tt.expected.HTTPRoutes))
			}
			if len(result.Warnings) != len(tt.expected.Warnings) {
				t.Errorf("Warnings count = %v, want %v", len(result.Warnings), len(tt.expected.Warnings))
			}
			if result.SkipReason != tt.expected.SkipReason {
				t.Errorf("SkipReason = %v, want %v", result.SkipReason, tt.expected.SkipReason)
			}
		})
	}
}

func TestCalculateSummary(t *testing.T) {
	infos := []Info{
		{Status: StatusConverted},
		{Status: StatusConverted},
		{Status: StatusPartial},
		{Status: StatusPending},
		{Status: StatusFailed},
		{Status: StatusSkipped},
		{Status: StatusNew},
		{Status: StatusNew},
	}

	summary := calculateSummary(infos)

	if summary.Total != 8 {
		t.Errorf("Total = %v, want 8", summary.Total)
	}
	if summary.Converted != 2 {
		t.Errorf("Converted = %v, want 2", summary.Converted)
	}
	if summary.Partial != 1 {
		t.Errorf("Partial = %v, want 1", summary.Partial)
	}
	if summary.Pending != 1 {
		t.Errorf("Pending = %v, want 1", summary.Pending)
	}
	if summary.Failed != 1 {
		t.Errorf("Failed = %v, want 1", summary.Failed)
	}
	if summary.Skipped != 1 {
		t.Errorf("Skipped = %v, want 1", summary.Skipped)
	}
	if summary.New != 2 {
		t.Errorf("New = %v, want 2", summary.New)
	}
}

func strPtr(s string) *string {
	return &s
}
