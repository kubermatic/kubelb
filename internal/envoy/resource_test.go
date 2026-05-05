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

	envoyCluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoyListener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoyTcpProxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoyUdpProxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/udp/udp_proxy/v3"
	envoyType "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"

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

func tcpProxyFromListener(t *testing.T, listener *envoyListener.Listener) *envoyTcpProxy.TcpProxy {
	t.Helper()
	if len(listener.GetFilterChains()) != 1 {
		t.Fatalf("expected 1 filter chain, got %d", len(listener.GetFilterChains()))
	}
	filters := listener.GetFilterChains()[0].GetFilters()
	if len(filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(filters))
	}
	typedConfig := filters[0].GetTypedConfig()
	if typedConfig == nil {
		t.Fatal("expected typed TCP proxy config")
	}
	tcpProxy := &envoyTcpProxy.TcpProxy{}
	if err := typedConfig.UnmarshalTo(tcpProxy); err != nil {
		t.Fatalf("failed to unmarshal TCP proxy: %v", err)
	}
	return tcpProxy
}

func udpProxyFromListener(t *testing.T, listener *envoyListener.Listener) *envoyUdpProxy.UdpProxyConfig {
	t.Helper()
	if len(listener.GetListenerFilters()) != 1 {
		t.Fatalf("expected 1 UDP listener filter")
	}
	typedConfig := listener.GetListenerFilters()[0].GetTypedConfig()
	if typedConfig == nil {
		t.Fatal("expected typed UDP proxy config")
	}
	udpProxy := &envoyUdpProxy.UdpProxyConfig{}
	if err := typedConfig.UnmarshalTo(udpProxy); err != nil {
		t.Fatalf("failed to unmarshal UDP proxy: %v", err)
	}
	return udpProxy
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
			cluster := makeCluster("test-cluster", corev1.ProtocolTCP, "", tt.proxyProtocol, nil)

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

func TestMakeCluster_Persistence(t *testing.T) {
	tests := []struct {
		name        string
		protocol    corev1.Protocol
		persistence *kubelbv1alpha1.LoadBalancerPersistence
		wantPolicy  envoyCluster.Cluster_LbPolicy
		wantMaglev  bool
	}{
		{name: "default remains round robin", protocol: corev1.ProtocolTCP, wantPolicy: envoyCluster.Cluster_ROUND_ROBIN},
		{
			name:        "source ip uses maglev",
			protocol:    corev1.ProtocolTCP,
			persistence: &kubelbv1alpha1.LoadBalancerPersistence{Type: kubelbv1alpha1.LoadBalancerPersistenceTypeSourceIP},
			wantPolicy:  envoyCluster.Cluster_MAGLEV,
			wantMaglev:  true,
		},
		{
			name:        "udp source ip uses maglev",
			protocol:    corev1.ProtocolUDP,
			persistence: &kubelbv1alpha1.LoadBalancerPersistence{Type: kubelbv1alpha1.LoadBalancerPersistenceTypeSourceIP},
			wantPolicy:  envoyCluster.Cluster_MAGLEV,
			wantMaglev:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := makeCluster("test-cluster", tt.protocol, "", false, tt.persistence)
			if cluster.GetLbPolicy() != tt.wantPolicy {
				t.Fatalf("LbPolicy = %v, want %v", cluster.GetLbPolicy(), tt.wantPolicy)
			}
			if gotMaglev := cluster.GetMaglevLbConfig() != nil; gotMaglev != tt.wantMaglev {
				t.Fatalf("Maglev config present = %v, want %v", gotMaglev, tt.wantMaglev)
			}
			if cluster.GetCommonLbConfig().GetHealthyPanicThreshold().GetValue() != 0 {
				t.Fatalf("HealthyPanicThreshold = %v, want 0", cluster.GetCommonLbConfig().GetHealthyPanicThreshold().GetValue())
			}
		})
	}
}

func TestMakeTCPListener_Persistence(t *testing.T) {
	tests := []struct {
		name        string
		persistence *kubelbv1alpha1.LoadBalancerPersistence
		wantHash    bool
	}{
		{name: "default has no hash policy"},
		{
			name:        "source ip adds tcp hash policy",
			persistence: &kubelbv1alpha1.LoadBalancerPersistence{Type: kubelbv1alpha1.LoadBalancerPersistenceTypeSourceIP},
			wantHash:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener := makeTCPListener("test-cluster", "test-listener", 10001, tt.persistence)
			tcpProxy := tcpProxyFromListener(t, listener)
			if gotHash := len(tcpProxy.GetHashPolicy()) == 1; gotHash != tt.wantHash {
				t.Fatalf("hash policy present = %v, want %v", gotHash, tt.wantHash)
			}
			if tt.wantHash {
				hashPolicy := tcpProxy.GetHashPolicy()[0]
				if hashPolicy.GetSourceIp() == nil {
					t.Fatalf("hash policy = %T, want source_ip", hashPolicy.GetPolicySpecifier())
				}
				if !proto.Equal(hashPolicy, &envoyType.HashPolicy{PolicySpecifier: &envoyType.HashPolicy_SourceIp_{SourceIp: &envoyType.HashPolicy_SourceIp{}}}) {
					t.Fatalf("unexpected hash policy: %v", hashPolicy)
				}
			}
		})
	}
}

func TestMakeUDPListener_Persistence(t *testing.T) {
	tests := []struct {
		name        string
		persistence *kubelbv1alpha1.LoadBalancerPersistence
		wantHash    bool
	}{
		{name: "default has no hash policy"},
		{
			name:        "source ip adds udp hash policy",
			persistence: &kubelbv1alpha1.LoadBalancerPersistence{Type: kubelbv1alpha1.LoadBalancerPersistenceTypeSourceIP},
			wantHash:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener := makeUDPListener("test-cluster", "test-listener", 10001, tt.persistence)
			udpProxy := udpProxyFromListener(t, listener)
			if gotHash := len(udpProxy.GetHashPolicies()) == 1; gotHash != tt.wantHash {
				t.Fatalf("hash policy present = %v, want %v", gotHash, tt.wantHash)
			}
			if tt.wantHash {
				hashPolicy := udpProxy.GetHashPolicies()[0]
				if !hashPolicy.GetSourceIp() {
					t.Fatalf("hash policy = %T, want source_ip", hashPolicy.GetPolicySpecifier())
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
	l1 := makeTCPListener("cluster-a", "listener-a", 10001, nil)
	l2 := makeTCPListener("cluster-b", "listener-b", 10001, nil)
	l3 := makeTCPListener("cluster-c", "listener-c", 10002, nil)

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
	tcp := makeTCPListener("cluster-tcp", "listener-tcp", 10001, nil)
	udp := makeUDPListener("cluster-udp", "listener-udp", 10001, nil)

	out := dedupListenersByAddressTestHelper([]types.Resource{tcp, udp})
	if len(out) != 2 {
		t.Fatalf("TCP and UDP on same port must not be deduped; got %d", len(out))
	}
}

func TestDedupListenersByAddress_ShortCircuits(t *testing.T) {
	if out := dedupListenersByAddressTestHelper(nil); out != nil {
		t.Fatalf("nil input should pass through, got %v", out)
	}
	single := []types.Resource{makeTCPListener("c", "l", 10001, nil)}
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
		makeTCPListener("c1", "l1", 10001, nil),
		makeTCPListener("c2", "l2", 10002, nil),
		makeUDPListener("c3", "l3", 10003, nil),
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
	l1 := makeTCPListener("cluster-a", "listener-a", 10010, nil)
	l2 := makeTCPListener("cluster-b", "listener-b", 10010, nil)
	out := dedupListenersByAddress([]types.Resource{l1, l2}, "tenant-bb8")
	if len(out) != 1 {
		t.Fatalf("expected 1 listener kept, got %d", len(out))
	}
}
