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
	"time"

	envoyCluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	dto "github.com/prometheus/client_model/go"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	managermetrics "k8c.io/kubelb/internal/metricsutil/manager"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func counterValue(t *testing.T, c interface {
	Write(*dto.Metric) error
}) float64 {
	t.Helper()
	m := &dto.Metric{}
	if err := c.Write(m); err != nil {
		t.Fatalf("counter Write failed: %v", err)
	}
	return m.GetCounter().GetValue()
}

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
			cluster := makeCluster("test-cluster", corev1.ProtocolTCP, "", tt.proxyProtocol, envoyCluster.Cluster_EDS, nil)

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
			name:  "ingress ssl-passthrough true",
			route: routeOfKind("Ingress", map[string]string{"nginx.ingress.kubernetes.io/ssl-passthrough": "true"}),
			want:  true,
		},
		{
			name:  "ingress ssl-passthrough True mixed case",
			route: routeOfKind("Ingress", map[string]string{"nginx.ingress.kubernetes.io/ssl-passthrough": "True"}),
			want:  true,
		},
		{
			name:  "ingress ssl-passthrough false",
			route: routeOfKind("Ingress", map[string]string{"nginx.ingress.kubernetes.io/ssl-passthrough": "false"}),
			want:  false,
		},
		{
			name:  "ingress ssl-passthrough true overrides missing backend-protocol",
			route: routeOfKind("Ingress", map[string]string{"nginx.ingress.kubernetes.io/ssl-passthrough": "true", "nginx.ingress.kubernetes.io/backend-protocol": "HTTP"}),
			want:  true,
		},
		{
			name:  "httproute with backend-protocol HTTPS annotation is ignored",
			route: routeOfKind("HTTPRoute", map[string]string{"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS"}),
			want:  false,
		},
		{
			name:  "httproute with ssl-passthrough annotation is ignored",
			route: routeOfKind("HTTPRoute", map[string]string{"nginx.ingress.kubernetes.io/ssl-passthrough": "true"}),
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

func dedupListenersByAddressTestHelper(listeners []types.Resource) []types.Resource {
	return dedupListenersByAddress(listeners, "test-ns")
}

func TestDedupListenersByAddress(t *testing.T) {
	l1 := makeTCPListener("cluster-a", "listener-a", 10001)
	l2 := makeTCPListener("cluster-b", "listener-b", 10001)
	l3 := makeTCPListener("cluster-c", "listener-c", 10002)

	out := dedupListenersByAddressTestHelper([]types.Resource{l1, l2, l3})

	if got, want := len(out), 2; got != want {
		t.Fatalf("expected %d listeners after dedup, got %d", want, got)
	}
	if out[0] != l1 {
		t.Errorf("expected first listener to be kept")
	}
	if out[1] != l3 {
		t.Errorf("expected non-colliding listener to be preserved")
	}
}

func TestDedupListenersByAddress_DistinctProtocols(t *testing.T) {
	tcp := makeTCPListener("cluster-tcp", "listener-tcp", 10001)
	udp := makeUDPListener("cluster-udp", "listener-udp", 10001)

	out := dedupListenersByAddressTestHelper([]types.Resource{tcp, udp})
	if len(out) != 2 {
		t.Fatalf("TCP and UDP on same port must not be deduped; got %d", len(out))
	}
}

func TestDedupListenersByAddress_ShortCircuits(t *testing.T) {
	if out := dedupListenersByAddressTestHelper(nil); out != nil {
		t.Fatalf("nil input should pass through, got %v", out)
	}
	single := []types.Resource{makeTCPListener("c", "l", 10001)}
	if out := dedupListenersByAddressTestHelper(single); len(out) != 1 {
		t.Fatalf("single listener should pass through, got %d", len(out))
	}
}

// TestDedupListenersByAddress_NoMetricIncrementWithoutDuplicates asserts the
// dedup counter stays flat when the input has no collisions.
func TestDedupListenersByAddress_NoMetricIncrementWithoutDuplicates(t *testing.T) {
	m := managermetrics.EnvoyDuplicateListenersDropped.WithLabelValues("ns-clean")
	before := counterValue(t, m)

	listeners := []types.Resource{
		makeTCPListener("c1", "l1", 10001),
		makeTCPListener("c2", "l2", 10002),
		makeUDPListener("c3", "l3", 10003),
	}
	dedupListenersByAddress(listeners, "ns-clean")

	if after := counterValue(t, m); after != before {
		t.Fatalf("dedup counter incremented without duplicates: before=%v after=%v", before, after)
	}
}

func TestDedupListenersByAddress_LabelCardinalityMatches(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("WithLabelValues panicked: %v (label cardinality drift)", r)
		}
	}()
	l1 := makeTCPListener("cluster-a", "listener-a", 10010)
	l2 := makeTCPListener("cluster-b", "listener-b", 10010)
	out := dedupListenersByAddress([]types.Resource{l1, l2}, "tenant-bb8")
	if len(out) != 1 {
		t.Fatalf("expected 1 listener kept, got %d", len(out))
	}
}

func TestResolveEndpointLiteral(t *testing.T) {
	const (
		testIP   = "10.0.0.1"
		testFQDN = "node.example.com"
	)
	tests := []struct {
		name        string
		addr        kubelbv1alpha1.EndpointAddress
		wantLiteral string
		wantIsIP    bool
	}{
		{name: "ipv4 in IP field", addr: kubelbv1alpha1.EndpointAddress{IP: testIP}, wantLiteral: testIP, wantIsIP: true},
		{name: "ipv6 in IP field", addr: kubelbv1alpha1.EndpointAddress{IP: "::1"}, wantLiteral: "::1", wantIsIP: true},
		{name: "fqdn in IP field", addr: kubelbv1alpha1.EndpointAddress{IP: testFQDN}, wantLiteral: testFQDN, wantIsIP: false},
		{name: "hostname only", addr: kubelbv1alpha1.EndpointAddress{Hostname: testFQDN}, wantLiteral: testFQDN, wantIsIP: false},
		{name: "ip in hostname field", addr: kubelbv1alpha1.EndpointAddress{Hostname: testIP}, wantLiteral: testIP, wantIsIP: true},
		{name: "both set, IP wins", addr: kubelbv1alpha1.EndpointAddress{IP: testIP, Hostname: testFQDN}, wantLiteral: testIP, wantIsIP: true},
		{name: "empty", addr: kubelbv1alpha1.EndpointAddress{}, wantLiteral: "", wantIsIP: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLiteral, gotIsIP := resolveEndpointLiteral(tt.addr)
			if gotLiteral != tt.wantLiteral || gotIsIP != tt.wantIsIP {
				t.Errorf("resolveEndpointLiteral(%+v) = (%q, %v), want (%q, %v)", tt.addr, gotLiteral, gotIsIP, tt.wantLiteral, tt.wantIsIP)
			}
		})
	}
}

func TestPickClusterDiscoveryType(t *testing.T) {
	const (
		testIP   = "10.0.0.1"
		testFQDN = "node.example.com"
	)
	tests := []struct {
		name  string
		addrs []kubelbv1alpha1.EndpointAddress
		want  envoyCluster.Cluster_DiscoveryType
	}{
		{
			name:  "all IPs in IP field",
			addrs: []kubelbv1alpha1.EndpointAddress{{IP: testIP}, {IP: "10.0.0.2"}},
			want:  envoyCluster.Cluster_EDS,
		},
		{
			name:  "any FQDN in IP field forces STRICT_DNS",
			addrs: []kubelbv1alpha1.EndpointAddress{{IP: testIP}, {IP: testFQDN}},
			want:  envoyCluster.Cluster_STRICT_DNS,
		},
		{
			name:  "hostname only",
			addrs: []kubelbv1alpha1.EndpointAddress{{Hostname: testFQDN}},
			want:  envoyCluster.Cluster_STRICT_DNS,
		},
		{
			name:  "mixed IP and hostname",
			addrs: []kubelbv1alpha1.EndpointAddress{{IP: testIP}, {Hostname: testFQDN}},
			want:  envoyCluster.Cluster_STRICT_DNS,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickClusterDiscoveryType(tt.addrs); got != tt.want {
				t.Errorf("pickClusterDiscoveryType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMakeCluster_DiscoveryType(t *testing.T) {
	cla := makeClusterLoadAssignment("test-cluster", nil)

	t.Run("EDS keeps EdsClusterConfig and no inline LoadAssignment", func(t *testing.T) {
		c := makeCluster("test-cluster", corev1.ProtocolTCP, "", false, envoyCluster.Cluster_EDS, cla)
		if c.GetType() != envoyCluster.Cluster_EDS {
			t.Errorf("type = %v, want EDS", c.GetType())
		}
		if c.EdsClusterConfig == nil {
			t.Error("EdsClusterConfig should be set for EDS")
		}
		if c.LoadAssignment != nil {
			t.Error("LoadAssignment should be nil for EDS (delivered via separate xDS resource)")
		}
	})

	t.Run("STRICT_DNS sets inline LoadAssignment and drops EdsClusterConfig", func(t *testing.T) {
		c := makeCluster("test-cluster", corev1.ProtocolTCP, "", false, envoyCluster.Cluster_STRICT_DNS, cla)
		if c.GetType() != envoyCluster.Cluster_STRICT_DNS {
			t.Errorf("type = %v, want STRICT_DNS", c.GetType())
		}
		if c.EdsClusterConfig != nil {
			t.Error("EdsClusterConfig must be nil for STRICT_DNS")
		}
		if c.LoadAssignment == nil {
			t.Error("LoadAssignment must be set inline for STRICT_DNS")
		}
		if c.DnsRefreshRate == nil || c.DnsRefreshRate.AsDuration() != 30*time.Second { //nolint:staticcheck // SA1019
			t.Errorf("DnsRefreshRate = %v, want 30s", c.DnsRefreshRate) //nolint:staticcheck // SA1019
		}
		if !c.RespectDnsTtl { //nolint:staticcheck // SA1019
			t.Error("RespectDnsTtl must be true for STRICT_DNS")
		}
	})
}
