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
	"context"
	"testing"
	"time"

	envoyCluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoyListener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoyHttpManager "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoyTcpProxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoyUdpProxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/udp/udp_proxy/v3"
	envoyUpstreams "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	envoyType "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	managermetrics "k8c.io/kubelb/internal/metricsutil/manager"
	portlookup "k8c.io/kubelb/internal/port-lookup"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
			cluster := makeCluster("test-cluster", corev1.ProtocolTCP, "", tt.proxyProtocol, nil, envoyCluster.Cluster_EDS, nil, ResolveHeaderLimits(nil))

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
			cluster := makeCluster("test-cluster", tt.protocol, "", false, tt.persistence, envoyCluster.Cluster_EDS, nil, ResolveHeaderLimits(nil))
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
		c := makeCluster("test-cluster", corev1.ProtocolTCP, "", false, nil, envoyCluster.Cluster_EDS, cla, ResolveHeaderLimits(nil))
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
		c := makeCluster("test-cluster", corev1.ProtocolTCP, "", false, nil, envoyCluster.Cluster_STRICT_DNS, cla, ResolveHeaderLimits(nil))
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

func hcmFromListener(t *testing.T, listener *envoyListener.Listener) *envoyHttpManager.HttpConnectionManager {
	t.Helper()
	typedConfig := listener.GetFilterChains()[0].GetFilters()[0].GetTypedConfig()
	if typedConfig == nil {
		t.Fatal("expected typed HCM config")
	}
	hcm := &envoyHttpManager.HttpConnectionManager{}
	if err := typedConfig.UnmarshalTo(hcm); err != nil {
		t.Fatalf("failed to unmarshal HCM: %v", err)
	}
	return hcm
}

func TestResolveHeaderLimits(t *testing.T) {
	if got := ResolveHeaderLimits(nil); got != (HeaderLimits{8192, 4096, 8192}) {
		t.Errorf("nil config = %+v, want defaults", got)
	}

	kb := uint32(96)
	config := &kubelbv1alpha1.Config{
		Spec: kubelbv1alpha1.ConfigSpec{
			EnvoyProxy: kubelbv1alpha1.EnvoyProxy{
				HeaderLimits: &kubelbv1alpha1.EnvoyProxyHeaderLimits{MaxRequestHeadersKb: &kb},
			},
		},
	}
	got := ResolveHeaderLimits(config)
	if got.MaxRequestHeadersKb != 96 {
		t.Errorf("MaxRequestHeadersKb = %d, want 96 (configured)", got.MaxRequestHeadersKb)
	}
	if got.MaxRequestHeadersCount != 4096 || got.MaxResponseHeadersKb != 8192 {
		t.Errorf("unset fields = %+v, want defaults", got)
	}
}

func TestMakeHTTPListener_HeaderLimits(t *testing.T) {
	limits := HeaderLimits{MaxRequestHeadersKb: 96, MaxRequestHeadersCount: 200}
	hcm := hcmFromListener(t, makeHTTPListener("l", "c", 10001, limits))
	if got := hcm.GetMaxRequestHeadersKb().GetValue(); got != 96 {
		t.Errorf("MaxRequestHeadersKb = %d, want 96", got)
	}
	if got := hcm.GetCommonHttpProtocolOptions().GetMaxHeadersCount().GetValue(); got != 200 {
		t.Errorf("MaxHeadersCount = %d, want 200", got)
	}
}

// TestMakeCluster_HTTPHeaderLimits covers the HTTPRoute branch; the GRPCRoute
// branch shares the same upstream header limits, so one branch is
// representative of both.
func TestMakeCluster_HTTPHeaderLimits(t *testing.T) {
	limits := HeaderLimits{MaxRequestHeadersCount: 200, MaxResponseHeadersKb: 128}
	c := makeCluster("test-cluster", corev1.ProtocolTCP, "HTTPRoute", false, nil, envoyCluster.Cluster_EDS, nil, limits)
	opts := &envoyUpstreams.HttpProtocolOptions{}
	if err := c.GetTypedExtensionProtocolOptions()[envoyHTTPProtocolOptionsTypeURL].UnmarshalTo(opts); err != nil {
		t.Fatalf("failed to unmarshal upstream http options: %v", err)
	}
	if got := opts.GetCommonHttpProtocolOptions().GetMaxHeadersCount().GetValue(); got != 200 {
		t.Errorf("MaxHeadersCount = %d, want 200", got)
	}
	if got := opts.GetCommonHttpProtocolOptions().GetMaxResponseHeadersKb().GetValue(); got != 128 {
		t.Errorf("MaxResponseHeadersKb = %d, want 128", got)
	}
}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := kubelbv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add kubelb scheme: %v", err)
	}
	return s
}

// inlineAddressLB builds a LoadBalancer with a single endpoint carrying one
// inline IP address and one TCP port. Inline Addresses mean MapSnapshot never
// resolves an AddressesReference, so the fake client is never hit.
func inlineAddressLB() kubelbv1alpha1.LoadBalancer {
	return kubelbv1alpha1.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Namespace: "tenant-a", Name: "lb1"},
		Spec: kubelbv1alpha1.LoadBalancerSpec{
			Endpoints: []kubelbv1alpha1.LoadBalancerEndpoints{
				{
					Addresses: []kubelbv1alpha1.EndpointAddress{{IP: "10.0.0.1"}},
					Ports:     []kubelbv1alpha1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}},
				},
			},
		},
	}
}

// ingressSourceRoute builds a Route sourced from a Kubernetes Ingress with a
// single upstream Service (one TCP port) and one inline IP endpoint address.
func ingressSourceRoute() kubelbv1alpha1.Route {
	routeObj := unstructured.Unstructured{}
	routeObj.SetKind("Ingress")
	routeObj.SetName("ing1")

	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "app", Name: "svc1", UID: "uid-1"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 8080, Protocol: corev1.ProtocolTCP, NodePort: 30080}},
		},
	}

	return kubelbv1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{Namespace: "tenant-a", Name: "route1"},
		Spec: kubelbv1alpha1.RouteSpec{
			Endpoints: []kubelbv1alpha1.LoadBalancerEndpoints{
				{Addresses: []kubelbv1alpha1.EndpointAddress{{IP: "10.0.0.2"}}},
			},
			Source: kubelbv1alpha1.RouteSource{
				Kubernetes: &kubelbv1alpha1.KubernetesSource{
					Route:    routeObj,
					Services: []kubelbv1alpha1.UpstreamService{{Service: svc}},
				},
			},
		},
	}
}

// TestMapSnapshot_VersionIsDeterministic pins the core property Plan 005 relies
// on: identical inputs (same client, allocator instance, and slices) produce an
// identical snapshot version for every resource type. Port allocation uses
// math/rand, so the two builds must reuse the same populated allocator — the
// version is not reproducible across process runs, only within one.
func TestMapSnapshot_VersionIsDeterministic(t *testing.T) {
	ctx := context.Background()
	cl := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()
	lbs := []kubelbv1alpha1.LoadBalancer{inlineAddressLB()}
	routes := []kubelbv1alpha1.Route{ingressSourceRoute()}

	pa := portlookup.NewPortAllocator()
	if err := pa.AllocatePortsForLoadBalancers(kubelbv1alpha1.LoadBalancerList{Items: lbs}); err != nil {
		t.Fatalf("allocate LB ports: %v", err)
	}
	if err := pa.AllocatePortsForRoutes(routes); err != nil {
		t.Fatalf("allocate route ports: %v", err)
	}

	snap1, err := MapSnapshot(ctx, cl, lbs, routes, pa, "test-node", ResolveHeaderLimits(nil))
	if err != nil {
		t.Fatalf("MapSnapshot #1: %v", err)
	}
	snap2, err := MapSnapshot(ctx, cl, lbs, routes, pa, "test-node", ResolveHeaderLimits(nil))
	if err != nil {
		t.Fatalf("MapSnapshot #2: %v", err)
	}

	for _, rt := range []resource.Type{resource.ClusterType, resource.ListenerType, resource.EndpointType} {
		if v1, v2 := snap1.GetVersion(rt), snap2.GetVersion(rt); v1 != v2 {
			t.Errorf("version drift for %s: %q != %q", rt, v1, v2)
		}
	}
	if snap1.GetVersion(resource.ClusterType) == "" {
		t.Error("cluster version is empty; expected a non-empty hash")
	}
}

// TestMapSnapshot_ProducesExpectedResources is a characterization test: the
// asserted counts are read from the current implementation's actual output for
// a single LB (1 endpoint, 1 TCP port, 1 inline IP address). An IP address
// yields an EDS discovery type, so one endpoint (ClusterLoadAssignment) is
// emitted alongside the cluster and listener.
func TestMapSnapshot_ProducesExpectedResources(t *testing.T) {
	ctx := context.Background()
	cl := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()
	lbs := []kubelbv1alpha1.LoadBalancer{inlineAddressLB()}

	pa := portlookup.NewPortAllocator()
	if err := pa.AllocatePortsForLoadBalancers(kubelbv1alpha1.LoadBalancerList{Items: lbs}); err != nil {
		t.Fatalf("allocate LB ports: %v", err)
	}

	snap, err := MapSnapshot(ctx, cl, lbs, nil, pa, "test-node", ResolveHeaderLimits(nil))
	if err != nil {
		t.Fatalf("MapSnapshot: %v", err)
	}

	const (
		wantClusters  = 1
		wantListeners = 1
		wantEndpoints = 1
	)
	if got := len(snap.GetResources(resource.ClusterType)); got != wantClusters {
		t.Errorf("clusters = %d, want %d", got, wantClusters)
	}
	if got := len(snap.GetResources(resource.ListenerType)); got != wantListeners {
		t.Errorf("listeners = %d, want %d", got, wantListeners)
	}
	if got := len(snap.GetResources(resource.EndpointType)); got != wantEndpoints {
		t.Errorf("endpoints = %d, want %d (EDS discovery type emits a ClusterLoadAssignment)", got, wantEndpoints)
	}
}

// TestMapSnapshot_HostnameEndpointEmitsNoEDSEndpoint characterizes the
// STRICT_DNS branch: a hostname address forces a DNS-aware cluster, whose
// load assignment is inlined on the cluster rather than delivered as a separate
// EDS endpoint resource, so the endpoints map is empty.
func TestMapSnapshot_HostnameEndpointEmitsNoEDSEndpoint(t *testing.T) {
	ctx := context.Background()
	cl := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()

	lb := inlineAddressLB()
	lb.Spec.Endpoints[0].Addresses = []kubelbv1alpha1.EndpointAddress{{Hostname: "backend.example.com"}}
	lbs := []kubelbv1alpha1.LoadBalancer{lb}

	pa := portlookup.NewPortAllocator()
	if err := pa.AllocatePortsForLoadBalancers(kubelbv1alpha1.LoadBalancerList{Items: lbs}); err != nil {
		t.Fatalf("allocate LB ports: %v", err)
	}

	snap, err := MapSnapshot(ctx, cl, lbs, nil, pa, "test-node", ResolveHeaderLimits(nil))
	if err != nil {
		t.Fatalf("MapSnapshot: %v", err)
	}

	if got := len(snap.GetResources(resource.ClusterType)); got != 1 {
		t.Errorf("clusters = %d, want 1", got)
	}
	if got := len(snap.GetResources(resource.EndpointType)); got != 0 {
		t.Errorf("endpoints = %d, want 0 (STRICT_DNS inlines its load assignment)", got)
	}
}

// TestMapSnapshot_VersionStableAcrossReorder asserts the property Plan 005
// relies on from a different angle: mutating a metadata field that MapSnapshot
// never reads (an annotation unrelated to proxy-protocol) leaves every resource
// version unchanged. The allocator is reused so port assignment is identical.
func TestMapSnapshot_VersionStableAcrossReorder(t *testing.T) {
	ctx := context.Background()
	cl := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()

	lbs := []kubelbv1alpha1.LoadBalancer{inlineAddressLB()}
	pa := portlookup.NewPortAllocator()
	if err := pa.AllocatePortsForLoadBalancers(kubelbv1alpha1.LoadBalancerList{Items: lbs}); err != nil {
		t.Fatalf("allocate LB ports: %v", err)
	}

	snap1, err := MapSnapshot(ctx, cl, lbs, nil, pa, "test-node", ResolveHeaderLimits(nil))
	if err != nil {
		t.Fatalf("MapSnapshot #1: %v", err)
	}

	mutated := inlineAddressLB()
	mutated.Annotations = map[string]string{"kubelb.k8c.io/irrelevant": "value"}
	mutated.ResourceVersion = "99999"
	lbs2 := []kubelbv1alpha1.LoadBalancer{mutated}

	snap2, err := MapSnapshot(ctx, cl, lbs2, nil, pa, "test-node", ResolveHeaderLimits(nil))
	if err != nil {
		t.Fatalf("MapSnapshot #2: %v", err)
	}

	for _, rt := range []resource.Type{resource.ClusterType, resource.ListenerType, resource.EndpointType} {
		if v1, v2 := snap1.GetVersion(rt), snap2.GetVersion(rt); v1 != v2 {
			t.Errorf("version changed for %s despite ignored-field mutation: %q != %q", rt, v1, v2)
		}
	}
}
