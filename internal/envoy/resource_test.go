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

package envoy

import (
	"testing"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestMakeCluster_ProxyProtocol(t *testing.T) {
	tests := []struct {
		name          string
		proxyProtocol bool
		wantTransport bool
	}{
		{
			name:          "proxy protocol disabled",
			proxyProtocol: false,
			wantTransport: false,
		},
		{
			name:          "proxy protocol v2 enabled",
			proxyProtocol: true,
			wantTransport: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := makeCluster("test-cluster", corev1.ProtocolTCP, "", tt.proxyProtocol)

			if tt.wantTransport && cluster.TransportSocket == nil {
				t.Error("expected TransportSocket to be set for proxy protocol v2")
			}
			if !tt.wantTransport && cluster.TransportSocket != nil {
				t.Error("expected TransportSocket to be nil when proxy protocol is disabled")
			}

			if tt.wantTransport {
				if cluster.TransportSocket.Name != "envoy.transport_sockets.upstream_proxy_protocol" {
					t.Errorf("expected transport socket name 'envoy.transport_sockets.upstream_proxy_protocol', got %q", cluster.TransportSocket.Name)
				}
			}
		})
	}
}

func TestIsTLSBackend(t *testing.T) {
	routeOfKind := func(kind string, annotations map[string]string) *kubelbv1alpha1.Route {
		obj := unstructured.Unstructured{}
		obj.SetKind(kind)
		obj.SetAnnotations(annotations)
		return &kubelbv1alpha1.Route{
			Spec: kubelbv1alpha1.RouteSpec{
				Source: kubelbv1alpha1.RouteSource{
					Kubernetes: &kubelbv1alpha1.KubernetesSource{Route: obj},
				},
			},
		}
	}

	tests := []struct {
		name  string
		route *kubelbv1alpha1.Route
		want  bool
	}{
		{
			name:  "nil route",
			route: nil,
			want:  false,
		},
		{
			name:  "ingress without annotations",
			route: routeOfKind("Ingress", nil),
			want:  false,
		},
		{
			name:  "ingress backend-protocol HTTP",
			route: routeOfKind("Ingress", map[string]string{"nginx.ingress.kubernetes.io/backend-protocol": "HTTP"}),
			want:  false,
		},
		{
			name:  "ingress backend-protocol HTTPS",
			route: routeOfKind("Ingress", map[string]string{"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS"}),
			want:  true,
		},
		{
			name:  "ingress backend-protocol https lowercase",
			route: routeOfKind("Ingress", map[string]string{"nginx.ingress.kubernetes.io/backend-protocol": "https"}),
			want:  true,
		},
		{
			name:  "ingress backend-protocol GRPCS",
			route: routeOfKind("Ingress", map[string]string{"nginx.ingress.kubernetes.io/backend-protocol": "GRPCS"}),
			want:  true,
		},
		{
			name:  "httproute with backend-protocol HTTPS annotation is ignored",
			route: routeOfKind("HTTPRoute", map[string]string{"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS"}),
			want:  false,
		},
		{
			name:  "grpcroute with backend-protocol HTTPS annotation is ignored",
			route: routeOfKind("GRPCRoute", map[string]string{"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS"}),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTLSBackend(tt.route); got != tt.want {
				t.Errorf("isTLSBackend() = %v, want %v", got, tt.want)
			}
		})
	}
}
