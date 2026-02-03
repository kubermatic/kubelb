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

	"k8c.io/kubelb/pkg/conversion"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const testIssuerLetsencrypt = "letsencrypt"

func TestBuildGateway(t *testing.T) {
	config := conversion.GatewayConfig{
		Name:      "test-gateway",
		Namespace: "default",
		ClassName: "kubelb",
		TLSListeners: []conversion.TLSListener{
			{Hostname: "app.example.com", SecretName: "app-tls"},
		},
		Annotations: map[string]string{
			"cert-manager.io/cluster-issuer": testIssuerLetsencrypt,
		},
	}

	gateway := conversion.BuildGateway(config)

	if gateway.Name != "test-gateway" {
		t.Errorf("expected name test-gateway, got %s", gateway.Name)
	}
	if gateway.Namespace != "default" {
		t.Errorf("expected namespace default, got %s", gateway.Namespace)
	}
	if string(gateway.Spec.GatewayClassName) != "kubelb" {
		t.Errorf("expected class kubelb, got %s", gateway.Spec.GatewayClassName)
	}
	if gateway.Labels[conversion.LabelManagedBy] != conversion.ControllerName {
		t.Errorf("expected managed-by label, got %v", gateway.Labels)
	}
	if gateway.Annotations["cert-manager.io/cluster-issuer"] != testIssuerLetsencrypt {
		t.Errorf("expected annotation, got %v", gateway.Annotations)
	}

	// Should have HTTP + HTTPS listeners
	if len(gateway.Spec.Listeners) != 2 {
		t.Fatalf("expected 2 listeners, got %d", len(gateway.Spec.Listeners))
	}
}

func TestBuildListeners(t *testing.T) {
	tests := []struct {
		name         string
		tlsListeners []conversion.TLSListener
		wantCount    int
		wantHTTPS    bool
	}{
		{
			name:         "no TLS",
			tlsListeners: nil,
			wantCount:    1, // HTTP only
			wantHTTPS:    false,
		},
		{
			name: "single TLS",
			tlsListeners: []conversion.TLSListener{
				{Hostname: "app.example.com", SecretName: "app-tls"},
			},
			wantCount: 2, // HTTP + HTTPS
			wantHTTPS: true,
		},
		{
			name: "multiple TLS",
			tlsListeners: []conversion.TLSListener{
				{Hostname: "app.example.com", SecretName: "app-tls"},
				{Hostname: "api.example.com", SecretName: "api-tls"},
			},
			wantCount: 3, // HTTP + 2 HTTPS
			wantHTTPS: true,
		},
		{
			name: "duplicate hostnames",
			tlsListeners: []conversion.TLSListener{
				{Hostname: "app.example.com", SecretName: "app-tls"},
				{Hostname: "app.example.com", SecretName: "app-tls-2"}, // Duplicate
			},
			wantCount: 2, // HTTP + 1 HTTPS (deduplicated)
			wantHTTPS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listeners := conversion.BuildListeners(tt.tlsListeners)

			if len(listeners) != tt.wantCount {
				t.Errorf("expected %d listeners, got %d", tt.wantCount, len(listeners))
			}

			// Check HTTP listener exists
			hasHTTP := false
			hasHTTPS := false
			for _, l := range listeners {
				if l.Protocol == gwapiv1.HTTPProtocolType {
					hasHTTP = true
				}
				if l.Protocol == gwapiv1.HTTPSProtocolType {
					hasHTTPS = true
				}
			}

			if !hasHTTP {
				t.Error("expected HTTP listener")
			}
			if tt.wantHTTPS && !hasHTTPS {
				t.Error("expected HTTPS listener")
			}
		})
	}
}

func TestMergeListeners(t *testing.T) {
	existing := []gwapiv1.Listener{
		{Name: "http", Port: 80, Protocol: gwapiv1.HTTPProtocolType},
		{Name: "https-existing", Port: 443, Protocol: gwapiv1.HTTPSProtocolType},
	}

	desired := []gwapiv1.Listener{
		{Name: "http", Port: 80, Protocol: gwapiv1.HTTPProtocolType}, // Duplicate
		{Name: "https-new", Port: 443, Protocol: gwapiv1.HTTPSProtocolType},
	}

	result := conversion.MergeListeners(existing, desired)

	if len(result) != 3 {
		t.Errorf("expected 3 listeners, got %d", len(result))
	}

	names := make(map[gwapiv1.SectionName]bool)
	for _, l := range result {
		names[l.Name] = true
	}

	if !names["http"] || !names["https-existing"] || !names["https-new"] {
		t.Errorf("unexpected listeners: %v", names)
	}
}

func TestListenerNameFromHostname(t *testing.T) {
	tests := []struct {
		hostname string
		expected string
	}{
		{"app.example.com", "https-app-example-com"},
		{"api.test.io", "https-api-test-io"},
		{"*", "https-wildcard"},
		{"*.example.com", "https-wildcard-example-com"},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			got := conversion.ListenerNameFromHostname(tt.hostname)
			if got != tt.expected {
				t.Errorf("ListenerNameFromHostname(%q) = %q, want %q", tt.hostname, got, tt.expected)
			}
		})
	}
}

func TestExtractGatewayAnnotations(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/target":    "lb.example.com",
				"external-dns.alpha.kubernetes.io/ttl":       "300",
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
	}

	// Test with external-dns enabled and user-specified gateway annotations
	r := &Reconciler{
		PropagateExternalDNS: true,
		GatewayAnnotations: map[string]string{
			"cert-manager.io/cluster-issuer": testIssuerLetsencrypt,
			"custom.io/my-annotation":        "my-value",
		},
	}

	annotations := r.extractGatewayAnnotations(ingress)

	// Should have user-specified gateway annotations
	if annotations["cert-manager.io/cluster-issuer"] != testIssuerLetsencrypt {
		t.Error("expected cert-manager.io/cluster-issuer from GatewayAnnotations")
	}
	if annotations["custom.io/my-annotation"] != "my-value" {
		t.Error("expected custom.io/my-annotation from GatewayAnnotations")
	}

	// Should have external-dns target annotation from Ingress
	if annotations["external-dns.alpha.kubernetes.io/target"] != "lb.example.com" {
		t.Error("expected external-dns target")
	}

	// Should NOT have external-dns ttl (goes to HTTPRoute)
	if _, ok := annotations["external-dns.alpha.kubernetes.io/ttl"]; ok {
		t.Error("ttl should not be on Gateway")
	}

	// Should NOT have nginx annotations
	if _, ok := annotations["nginx.ingress.kubernetes.io/rewrite-target"]; ok {
		t.Error("nginx annotations should not be propagated")
	}
}

func TestExtractHTTPRouteAnnotations(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/target":   "lb.example.com",
				"external-dns.alpha.kubernetes.io/ttl":      "300",
				"external-dns.alpha.kubernetes.io/hostname": "app.example.com",
			},
		},
	}

	r := &Reconciler{
		PropagateExternalDNS: true,
	}

	annotations := r.extractHTTPRouteAnnotations(ingress)

	// Should have ttl and hostname (HTTPRoute annotations)
	if annotations["external-dns.alpha.kubernetes.io/ttl"] != "300" {
		t.Error("expected ttl")
	}
	if annotations["external-dns.alpha.kubernetes.io/hostname"] != "app.example.com" {
		t.Error("expected hostname")
	}

	// Should NOT have target (goes to Gateway)
	if _, ok := annotations["external-dns.alpha.kubernetes.io/target"]; ok {
		t.Error("target should not be on HTTPRoute")
	}
}

func TestExtractAnnotations_FlagsDisabled(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/target": "lb.example.com",
				"external-dns.alpha.kubernetes.io/ttl":    "300",
			},
		},
	}

	r := &Reconciler{
		PropagateExternalDNS: false,
		GatewayAnnotations:   nil,
	}

	gatewayAnnotations := r.extractGatewayAnnotations(ingress)
	httpRouteAnnotations := r.extractHTTPRouteAnnotations(ingress)

	if len(gatewayAnnotations) != 0 {
		t.Errorf("expected no gateway annotations, got %v", gatewayAnnotations)
	}
	if len(httpRouteAnnotations) != 0 {
		t.Errorf("expected no httproute annotations, got %v", httpRouteAnnotations)
	}
}

func TestExtractGatewayAnnotations_WithGatewayAnnotationsOnly(t *testing.T) {
	// Test that gateway annotations work without any Ingress annotations
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: nil,
		},
	}

	r := &Reconciler{
		PropagateExternalDNS: true,
		GatewayAnnotations: map[string]string{
			"cert-manager.io/cluster-issuer": testIssuerLetsencrypt,
		},
	}

	annotations := r.extractGatewayAnnotations(ingress)

	if annotations["cert-manager.io/cluster-issuer"] != testIssuerLetsencrypt {
		t.Error("expected gateway annotations to be set even without Ingress annotations")
	}
}

func TestListenersEqual(t *testing.T) {
	a := []gwapiv1.Listener{
		{Name: "http", Port: 80},
		{Name: "https", Port: 443},
	}
	b := []gwapiv1.Listener{
		{Name: "https", Port: 443},
		{Name: "http", Port: 80},
	}
	c := []gwapiv1.Listener{
		{Name: "http", Port: 80},
	}

	if !listenersEqual(a, b) {
		t.Error("expected a and b to be equal")
	}
	if listenersEqual(a, c) {
		t.Error("expected a and c to be not equal")
	}
}
