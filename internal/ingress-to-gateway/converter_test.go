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

package ingressconversion

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestConvertIngress_SingleHost(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ingress",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "foo.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/api",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "api-svc",
											Port: networkingv1.ServiceBackendPort{Number: 8080},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := ConvertIngress(ingress, "my-gateway", "gateway-ns")

	if len(result.HTTPRoutes) != 1 {
		t.Fatalf("expected 1 HTTPRoute, got %d", len(result.HTTPRoutes))
	}

	route := result.HTTPRoutes[0]
	if route.Name != "test-ingress-foo-example-com" {
		t.Errorf("unexpected route name: %s", route.Name)
	}
	if len(route.Spec.Hostnames) != 1 || route.Spec.Hostnames[0] != "foo.example.com" {
		t.Errorf("unexpected hostnames: %v", route.Spec.Hostnames)
	}
	if len(route.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(route.Spec.Rules))
	}
	if len(route.Spec.ParentRefs) != 1 {
		t.Fatalf("expected 1 parent ref, got %d", len(route.Spec.ParentRefs))
	}
	if route.Spec.ParentRefs[0].Name != "my-gateway" {
		t.Errorf("unexpected gateway name: %s", route.Spec.ParentRefs[0].Name)
	}
}

func TestConvertIngress_MultipleHosts(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi-host",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "foo.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/foo",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "foo-svc",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
				{
					Host: "bar.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/bar",
									PathType: ptr.To(networkingv1.PathTypeExact),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "bar-svc",
											Port: networkingv1.ServiceBackendPort{Number: 8080},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := ConvertIngress(ingress, "gw", "")

	if len(result.HTTPRoutes) != 2 {
		t.Fatalf("expected 2 HTTPRoutes (one per host), got %d", len(result.HTTPRoutes))
	}

	// Verify each route has exactly one host
	hostsSeen := make(map[gwapiv1.Hostname]bool)
	for _, route := range result.HTTPRoutes {
		if len(route.Spec.Hostnames) != 1 {
			t.Errorf("route %s has %d hostnames, expected 1", route.Name, len(route.Spec.Hostnames))
		}
		hostsSeen[route.Spec.Hostnames[0]] = true
	}

	if !hostsSeen["foo.com"] || !hostsSeen["bar.com"] {
		t.Errorf("missing expected hosts, got: %v", hostsSeen)
	}
}

func TestConvertIngress_NoHost(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "catch-all",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					// No host = catch-all
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "default-svc",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := ConvertIngress(ingress, "gw", "")

	if len(result.HTTPRoutes) != 1 {
		t.Fatalf("expected 1 HTTPRoute, got %d", len(result.HTTPRoutes))
	}

	route := result.HTTPRoutes[0]
	if route.Name != "catch-all" {
		t.Errorf("expected name 'catch-all', got %s", route.Name)
	}
	if len(route.Spec.Hostnames) != 0 {
		t.Errorf("expected no hostnames for catch-all, got %v", route.Spec.Hostnames)
	}
}

func TestConvertIngress_DefaultBackend(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "with-default",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "default-svc",
					Port: networkingv1.ServiceBackendPort{Number: 80},
				},
			},
		},
	}

	result := ConvertIngress(ingress, "gw", "")

	if len(result.HTTPRoutes) != 1 {
		t.Fatalf("expected 1 HTTPRoute, got %d", len(result.HTTPRoutes))
	}

	route := result.HTTPRoutes[0]
	if len(route.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule for default backend, got %d", len(route.Spec.Rules))
	}
	if len(route.Spec.Rules[0].BackendRefs) != 1 {
		t.Fatalf("expected 1 backend ref, got %d", len(route.Spec.Rules[0].BackendRefs))
	}
	if route.Spec.Rules[0].BackendRefs[0].Name != "default-svc" {
		t.Errorf("unexpected backend name: %s", route.Spec.Rules[0].BackendRefs[0].Name)
	}
}

func TestConvertIngress_TLSListeners(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "with-tls",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{"secure.example.com"},
					SecretName: "tls-secret",
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: "secure.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "secure-svc",
											Port: networkingv1.ServiceBackendPort{Number: 443},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := ConvertIngress(ingress, "gw", "")

	// Should generate TLS listener config
	if len(result.TLSListeners) != 1 {
		t.Fatalf("expected 1 TLS listener, got %d", len(result.TLSListeners))
	}

	listener := result.TLSListeners[0]
	if listener.Hostname != "secure.example.com" {
		t.Errorf("unexpected hostname: %s", listener.Hostname)
	}
	if listener.SecretName != "tls-secret" {
		t.Errorf("unexpected secret name: %s", listener.SecretName)
	}
	if listener.SecretNamespace != "default" {
		t.Errorf("unexpected secret namespace: %s", listener.SecretNamespace)
	}
}

func TestConvertIngress_TLSMultipleHosts(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi-tls",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{"foo.example.com", "bar.example.com"},
					SecretName: "shared-tls",
				},
				{
					Hosts:      []string{"other.example.com"},
					SecretName: "other-tls",
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: "foo.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/",
								PathType: ptr.To(networkingv1.PathTypePrefix),
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "svc",
										Port: networkingv1.ServiceBackendPort{Number: 80},
									},
								},
							}},
						},
					},
				},
			},
		},
	}

	result := ConvertIngress(ingress, "gw", "")

	// Should generate 3 TLS listener configs (2 from first TLS, 1 from second)
	if len(result.TLSListeners) != 3 {
		t.Fatalf("expected 3 TLS listeners, got %d", len(result.TLSListeners))
	}

	// Verify hostnames
	hostnames := make(map[gwapiv1.Hostname]string)
	for _, l := range result.TLSListeners {
		hostnames[l.Hostname] = l.SecretName
	}

	if hostnames["foo.example.com"] != "shared-tls" {
		t.Errorf("foo.example.com should use shared-tls")
	}
	if hostnames["bar.example.com"] != "shared-tls" {
		t.Errorf("bar.example.com should use shared-tls")
	}
	if hostnames["other.example.com"] != "other-tls" {
		t.Errorf("other.example.com should use other-tls")
	}
}

func TestConvertIngress_NamedPortWarning(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "named-port",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "test.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "my-svc",
											Port: networkingv1.ServiceBackendPort{Name: "http"}, // Named port
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := ConvertIngress(ingress, "gw", "")

	if len(result.Warnings) == 0 {
		t.Fatal("expected named port warning, got none")
	}

	hasNamedPortWarning := false
	for _, w := range result.Warnings {
		if w == `service "my-svc" uses named port "http" which cannot be resolved; specify port number instead` {
			hasNamedPortWarning = true
			break
		}
	}
	if !hasNamedPortWarning {
		t.Errorf("expected named port warning, got: %v", result.Warnings)
	}
}

func TestConvertIngress_ResourceBackendWarning(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "resource-backend",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "test.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/storage",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Resource: &corev1.TypedLocalObjectReference{
											APIGroup: ptr.To("storage.k8s.io"),
											Kind:     "Bucket",
											Name:     "my-bucket",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := ConvertIngress(ingress, "gw", "")

	if len(result.Warnings) == 0 {
		t.Fatal("expected resource backend warning, got none")
	}

	hasResourceWarning := false
	for _, w := range result.Warnings {
		if w == `resource backend "my-bucket" not supported; only Service backends are converted` {
			hasResourceWarning = true
			break
		}
	}
	if !hasResourceWarning {
		t.Errorf("expected resource backend warning, got: %v", result.Warnings)
	}
}

func TestConvertIngress_PathTypes(t *testing.T) {
	tests := []struct {
		name         string
		pathType     *networkingv1.PathType
		expectedType gwapiv1.PathMatchType
		expectWarn   bool
	}{
		{
			name:         "Exact",
			pathType:     ptr.To(networkingv1.PathTypeExact),
			expectedType: gwapiv1.PathMatchExact,
		},
		{
			name:         "Prefix",
			pathType:     ptr.To(networkingv1.PathTypePrefix),
			expectedType: gwapiv1.PathMatchPathPrefix,
		},
		{
			name:         "ImplementationSpecific",
			pathType:     ptr.To(networkingv1.PathTypeImplementationSpecific),
			expectedType: gwapiv1.PathMatchPathPrefix,
			expectWarn:   true,
		},
		{
			name:         "nil (default)",
			pathType:     nil,
			expectedType: gwapiv1.PathMatchPathPrefix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{
						Host: "test.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/test",
									PathType: tt.pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "svc",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			}

			result := ConvertIngress(ingress, "gw", "")

			if len(result.HTTPRoutes) != 1 || len(result.HTTPRoutes[0].Spec.Rules) != 1 {
				t.Fatal("unexpected route/rule count")
			}

			rule := result.HTTPRoutes[0].Spec.Rules[0]
			if rule.Matches[0].Path == nil || rule.Matches[0].Path.Type == nil {
				t.Fatal("path match type not set")
			}
			if *rule.Matches[0].Path.Type != tt.expectedType {
				t.Errorf("expected path type %v, got %v", tt.expectedType, *rule.Matches[0].Path.Type)
			}

			if tt.expectWarn && len(result.Warnings) == 0 {
				t.Error("expected warning for ImplementationSpecific, got none")
			}
		})
	}
}

func TestConvertIngress_LabelsPreserved(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "labeled",
			Namespace: "default",
			Labels: map[string]string{
				"app":     "myapp",
				"version": "v1",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: "test.com",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: ptr.To(networkingv1.PathTypePrefix),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "svc",
									Port: networkingv1.ServiceBackendPort{Number: 80},
								},
							},
						}},
					},
				},
			}},
		},
	}

	result := ConvertIngress(ingress, "gw", "")

	route := result.HTTPRoutes[0]
	if route.Labels["app"] != "myapp" || route.Labels["version"] != "v1" {
		t.Errorf("labels not preserved: %v", route.Labels)
	}
}

func TestConvertIngress_ParentRefNamespace(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "app-ns"},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: "test.com",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: ptr.To(networkingv1.PathTypePrefix),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "svc",
									Port: networkingv1.ServiceBackendPort{Number: 80},
								},
							},
						}},
					},
				},
			}},
		},
	}

	// Gateway in different namespace
	result := ConvertIngress(ingress, "gw", "gateway-ns")
	route := result.HTTPRoutes[0]

	if route.Spec.ParentRefs[0].Namespace == nil {
		t.Fatal("expected namespace to be set")
	}
	if *route.Spec.ParentRefs[0].Namespace != "gateway-ns" {
		t.Errorf("unexpected namespace: %s", *route.Spec.ParentRefs[0].Namespace)
	}

	// Gateway in same namespace
	result2 := ConvertIngress(ingress, "gw", "app-ns")
	route2 := result2.HTTPRoutes[0]

	if route2.Spec.ParentRefs[0].Namespace != nil {
		t.Errorf("expected namespace to be nil when same as route, got: %s", *route2.Spec.ParentRefs[0].Namespace)
	}
}

func TestHTTPRouteName(t *testing.T) {
	tests := []struct {
		ingressName string
		host        string
		expected    string
	}{
		{"my-ingress", "foo.com", "my-ingress-foo-com"},
		{"ing", "a.b.c.d", "ing-a-b-c-d"},
		{"test", "", "test"},
	}

	for _, tt := range tests {
		got := httpRouteName(tt.ingressName, tt.host)
		if got != tt.expected {
			t.Errorf("httpRouteName(%q, %q) = %q, want %q", tt.ingressName, tt.host, got, tt.expected)
		}
	}
}

func TestConvertIngress_DeterministicOrdering(t *testing.T) {
	// Create Ingress with multiple hosts to test ordering stability
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ordering-test",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "zebra.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/z",
								PathType: ptr.To(networkingv1.PathTypePrefix),
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "z-svc",
										Port: networkingv1.ServiceBackendPort{Number: 80},
									},
								},
							}},
						},
					},
				},
				{
					Host: "alpha.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/a",
								PathType: ptr.To(networkingv1.PathTypePrefix),
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "a-svc",
										Port: networkingv1.ServiceBackendPort{Number: 80},
									},
								},
							}},
						},
					},
				},
				{
					Host: "beta.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/b",
								PathType: ptr.To(networkingv1.PathTypePrefix),
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "b-svc",
										Port: networkingv1.ServiceBackendPort{Number: 80},
									},
								},
							}},
						},
					},
				},
			},
		},
	}

	// Run conversion multiple times and verify consistent ordering
	var firstResult ConversionResult
	for i := 0; i < 10; i++ {
		result := ConvertIngress(ingress, "gw", "")

		if len(result.HTTPRoutes) != 3 {
			t.Fatalf("iteration %d: expected 3 HTTPRoutes, got %d", i, len(result.HTTPRoutes))
		}

		if i == 0 {
			firstResult = result
		} else {
			// Verify ordering is consistent
			for j, route := range result.HTTPRoutes {
				if route.Name != firstResult.HTTPRoutes[j].Name {
					t.Errorf("iteration %d: route %d name mismatch: got %s, expected %s",
						i, j, route.Name, firstResult.HTTPRoutes[j].Name)
				}
			}
		}
	}

	// Verify routes are alphabetically sorted by hostname
	expectedOrder := []gwapiv1.Hostname{"alpha.com", "beta.com", "zebra.com"}
	for i, route := range firstResult.HTTPRoutes {
		if route.Spec.Hostnames[0] != expectedOrder[i] {
			t.Errorf("route %d: expected hostname %s, got %s", i, expectedOrder[i], route.Spec.Hostnames[0])
		}
	}
}

func TestConvertIngress_NamedPortResolution(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "named-port",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "test.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "my-svc",
											Port: networkingv1.ServiceBackendPort{Name: "http"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	services := map[types.NamespacedName]*corev1.Service{
		{Name: "my-svc", Namespace: "default"}: {
			ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{Name: "http", Port: 8080},
					{Name: "https", Port: 8443},
				},
			},
		},
	}

	result := ConvertIngressWithServices(ConversionInput{
		Ingress:          ingress,
		GatewayName:      "gw",
		GatewayNamespace: "",
		Services:         services,
	})

	// Should have no warnings about named port
	for _, w := range result.Warnings {
		if w == `service "my-svc" uses named port "http" which cannot be resolved; specify port number instead` {
			t.Error("should not have named port warning when Service is provided")
		}
	}

	// Verify port was resolved
	if len(result.HTTPRoutes) != 1 {
		t.Fatalf("expected 1 HTTPRoute, got %d", len(result.HTTPRoutes))
	}
	backendRef := result.HTTPRoutes[0].Spec.Rules[0].BackendRefs[0]
	if backendRef.Port == nil || *backendRef.Port != 8080 {
		t.Errorf("expected port 8080, got %v", backendRef.Port)
	}
}

func TestConvertIngress_SinglePortAutoDetect(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-port",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "test.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "single-port-svc",
											// No port specified
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	services := map[types.NamespacedName]*corev1.Service{
		{Name: "single-port-svc", Namespace: "default"}: {
			ObjectMeta: metav1.ObjectMeta{Name: "single-port-svc", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{Name: "http", Port: 80},
				},
			},
		},
	}

	result := ConvertIngressWithServices(ConversionInput{
		Ingress:          ingress,
		GatewayName:      "gw",
		GatewayNamespace: "",
		Services:         services,
	})

	// Should have no warnings about missing port
	for _, w := range result.Warnings {
		if w == `service "single-port-svc" has no port specified; Gateway API requires explicit port` {
			t.Error("should not have missing port warning when Service has single port")
		}
	}

	// Verify port was auto-detected
	if len(result.HTTPRoutes) != 1 {
		t.Fatalf("expected 1 HTTPRoute, got %d", len(result.HTTPRoutes))
	}
	backendRef := result.HTTPRoutes[0].Spec.Rules[0].BackendRefs[0]
	if backendRef.Port == nil || *backendRef.Port != 80 {
		t.Errorf("expected port 80, got %v", backendRef.Port)
	}
}

func TestConvertIngress_MultiPortServiceWarning(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-port-multi",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "test.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "multi-port-svc",
											// No port specified
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	services := map[types.NamespacedName]*corev1.Service{
		{Name: "multi-port-svc", Namespace: "default"}: {
			ObjectMeta: metav1.ObjectMeta{Name: "multi-port-svc", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{Name: "http", Port: 80},
					{Name: "https", Port: 443},
				},
			},
		},
	}

	result := ConvertIngressWithServices(ConversionInput{
		Ingress:          ingress,
		GatewayName:      "gw",
		GatewayNamespace: "",
		Services:         services,
	})

	// Should warn about multiple ports
	hasWarning := false
	for _, w := range result.Warnings {
		if w == `service "multi-port-svc" has multiple ports; specify port explicitly` {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("expected multi-port warning, got: %v", result.Warnings)
	}
}

func TestConvertIngress_ServiceNotFound(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "missing-svc",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "test.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "missing-svc",
											Port: networkingv1.ServiceBackendPort{Name: "http"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Empty services map - Service not found
	services := map[types.NamespacedName]*corev1.Service{}

	result := ConvertIngressWithServices(ConversionInput{
		Ingress:          ingress,
		GatewayName:      "gw",
		GatewayNamespace: "",
		Services:         services,
	})

	// Should warn about service not found
	hasWarning := false
	for _, w := range result.Warnings {
		if w == `service "missing-svc" not found for port resolution` {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("expected service not found warning, got: %v", result.Warnings)
	}
}

func TestConvertIngress_WithAnnotations(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "annotated-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/ssl-redirect":   "true",
				"nginx.ingress.kubernetes.io/rewrite-target": "/api",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/v1",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "backend",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := ConvertIngress(ingress, "gateway", "default")

	if len(result.HTTPRoutes) != 1 {
		t.Fatalf("expected 1 HTTPRoute, got %d", len(result.HTTPRoutes))
	}

	route := result.HTTPRoutes[0]
	if len(route.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(route.Spec.Rules))
	}

	rule := route.Spec.Rules[0]

	// Should have 2 filters: RequestRedirect (ssl-redirect) and URLRewrite (rewrite-target)
	if len(rule.Filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(rule.Filters))
	}

	// Check filter types
	filterTypes := make(map[gwapiv1.HTTPRouteFilterType]bool)
	for _, f := range rule.Filters {
		filterTypes[f.Type] = true
	}

	if !filterTypes[gwapiv1.HTTPRouteFilterRequestRedirect] {
		t.Error("missing RequestRedirect filter from ssl-redirect")
	}
	if !filterTypes[gwapiv1.HTTPRouteFilterURLRewrite] {
		t.Error("missing URLRewrite filter from rewrite-target")
	}

	// Should have processed both annotations
	if len(result.ProcessedAnnotations) != 2 {
		t.Errorf("expected 2 processed annotations, got %d: %v", len(result.ProcessedAnnotations), result.ProcessedAnnotations)
	}
}

func TestConvertIngress_WithPolicyAnnotations(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "policy-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/proxy-connect-timeout": "30",
				"nginx.ingress.kubernetes.io/enable-cors":           "true",
				"nginx.ingress.kubernetes.io/cors-allow-origin":     "*",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "api.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "api",
											Port: networkingv1.ServiceBackendPort{Number: 8080},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := ConvertIngress(ingress, "gateway", "default")

	// Policy annotations don't create filters, they create warnings
	route := result.HTTPRoutes[0]
	if len(route.Spec.Rules[0].Filters) != 0 {
		t.Errorf("expected 0 filters for policy annotations, got %d", len(route.Spec.Rules[0].Filters))
	}

	// Should have warnings for policy suggestions
	if len(result.Warnings) < 2 {
		t.Errorf("expected at least 2 policy warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}

	// Should have processed all 3 annotations
	if len(result.ProcessedAnnotations) != 3 {
		t.Errorf("expected 3 processed annotations, got %d", len(result.ProcessedAnnotations))
	}
}
