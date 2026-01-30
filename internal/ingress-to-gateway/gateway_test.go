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

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestBuildGateway(t *testing.T) {
	config := GatewayConfig{
		Name:      "test-gateway",
		Namespace: "default",
		ClassName: "kubelb",
		TLSListeners: []TLSListener{
			{Hostname: "app.example.com", SecretName: "app-tls"},
		},
		Annotations: map[string]string{
			"cert-manager.io/cluster-issuer": "letsencrypt",
		},
	}

	gateway := buildGateway(config)

	if gateway.Name != "test-gateway" {
		t.Errorf("expected name test-gateway, got %s", gateway.Name)
	}
	if gateway.Namespace != "default" {
		t.Errorf("expected namespace default, got %s", gateway.Namespace)
	}
	if string(gateway.Spec.GatewayClassName) != "kubelb" {
		t.Errorf("expected class kubelb, got %s", gateway.Spec.GatewayClassName)
	}
	if gateway.Labels[LabelManagedBy] != ControllerName {
		t.Errorf("expected managed-by label, got %v", gateway.Labels)
	}
	if gateway.Annotations["cert-manager.io/cluster-issuer"] != "letsencrypt" {
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
		tlsListeners []TLSListener
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
			tlsListeners: []TLSListener{
				{Hostname: "app.example.com", SecretName: "app-tls"},
			},
			wantCount: 2, // HTTP + HTTPS
			wantHTTPS: true,
		},
		{
			name: "multiple TLS",
			tlsListeners: []TLSListener{
				{Hostname: "app.example.com", SecretName: "app-tls"},
				{Hostname: "api.example.com", SecretName: "api-tls"},
			},
			wantCount: 3, // HTTP + 2 HTTPS
			wantHTTPS: true,
		},
		{
			name: "duplicate hostnames",
			tlsListeners: []TLSListener{
				{Hostname: "app.example.com", SecretName: "app-tls"},
				{Hostname: "app.example.com", SecretName: "app-tls-2"}, // Duplicate
			},
			wantCount: 2, // HTTP + 1 HTTPS (deduplicated)
			wantHTTPS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listeners := buildListeners(tt.tlsListeners)

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

	result := mergeListeners(existing, desired)

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
			got := listenerNameFromHostname(tt.hostname)
			if got != tt.expected {
				t.Errorf("listenerNameFromHostname(%q) = %q, want %q", tt.hostname, got, tt.expected)
			}
		})
	}
}

func TestExtractGatewayAnnotations(t *testing.T) {
	const tlsAcmeValue = "enabled" // Use distinct value to avoid goconst warning

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"cert-manager.io/cluster-issuer":             "letsencrypt",
				"cert-manager.io/duration":                   "8760h",
				"kubernetes.io/tls-acme":                     tlsAcmeValue,
				"external-dns.alpha.kubernetes.io/target":    "lb.example.com",
				"external-dns.alpha.kubernetes.io/ttl":       "300",
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
	}

	// Test with both flags enabled
	r := &Reconciler{
		PropagateCertManager: true,
		PropagateExternalDNS: true,
	}

	annotations := r.extractGatewayAnnotations(ingress)

	// Should have cert-manager annotations
	if annotations["cert-manager.io/cluster-issuer"] != "letsencrypt" {
		t.Error("expected cert-manager.io/cluster-issuer")
	}
	if annotations["cert-manager.io/duration"] != "8760h" {
		t.Error("expected cert-manager.io/duration")
	}
	if annotations["kubernetes.io/tls-acme"] != tlsAcmeValue {
		t.Error("expected kubernetes.io/tls-acme")
	}

	// Should have external-dns target annotation
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
				"cert-manager.io/cluster-issuer":            "letsencrypt",
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

	// Should NOT have cert-manager annotations
	if _, ok := annotations["cert-manager.io/cluster-issuer"]; ok {
		t.Error("cert-manager annotations should not be on HTTPRoute")
	}
}

func TestExtractAnnotations_FlagsDisabled(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"cert-manager.io/cluster-issuer":          "letsencrypt",
				"external-dns.alpha.kubernetes.io/target": "lb.example.com",
				"external-dns.alpha.kubernetes.io/ttl":    "300",
			},
		},
	}

	r := &Reconciler{
		PropagateCertManager: false,
		PropagateExternalDNS: false,
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
