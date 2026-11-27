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

package portlookup

import (
	"context"
	"fmt"
	"sync"
	"testing"

	dto "github.com/prometheus/client_model/go"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	managermetrics "k8c.io/kubelb/internal/metricsutil/manager"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// stubReader serves canned LoadBalancerList and RouteList responses for LoadState tests.
type stubReader struct {
	lbs    []kubelbv1alpha1.LoadBalancer
	routes []kubelbv1alpha1.Route
}

func (s *stubReader) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return nil
}
func (s *stubReader) List(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
	switch l := list.(type) {
	case *kubelbv1alpha1.LoadBalancerList:
		l.Items = s.lbs
	case *kubelbv1alpha1.RouteList:
		l.Items = s.routes
	}
	return nil
}

// TestAllocatePorts_NoIntraBatchCollision guards against the regression where
// allocatePort reads portLookupReverse but doesn't write, allowing two keys in
// the same AllocatePorts call to receive the same random port.
func TestAllocatePorts_NoIntraBatchCollision(t *testing.T) {
	pa := NewPortAllocator()

	const n = 500
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		keys[i] = fmt.Sprintf("port-%d", i)
	}
	pa.AllocatePorts("endpointA", keys)

	seen := make(map[int]string)
	for _, k := range keys {
		port, ok := pa.Lookup("endpointA", k)
		if !ok {
			t.Fatalf("port not allocated for key %s", k)
		}
		if owner, dup := seen[port]; dup {
			t.Fatalf("port %d allocated twice in one batch: owners %s and %s", port, owner, k)
		}
		seen[port] = k
	}
}

// TestLoadState_HealsUpgradeCollision simulates the upgrade state where two
// routes sharing a backend service have the same port recorded in both routes'
// statuses. After LoadState, at most one endpointKey must own that port.
func TestLoadState_HealsUpgradeCollision(t *testing.T) {
	lookupTable := LookupTable{
		"endpointKey_A": map[string]int{"tcp/80": 12345},
		"endpointKey_B": map[string]int{"tcp/80": 12345}, // collision
		"endpointKey_C": map[string]int{"tcp/443": 23456},
	}

	healLoadStateCollisions(lookupTable)

	aHas := len(lookupTable["endpointKey_A"]) > 0
	bHas := len(lookupTable["endpointKey_B"]) > 0
	if aHas == bHas {
		t.Fatalf("expected exactly one of endpointKey_A/B to retain the port; got A=%v B=%v", aHas, bHas)
	}
	if lookupTable["endpointKey_C"]["tcp/443"] != 23456 {
		t.Fatalf("non-colliding entry was disturbed: %+v", lookupTable["endpointKey_C"])
	}
}

// TestLoadState_HealsUpgradeCollision_Deterministic asserts the healing picks
// the same owner across runs (sorted key order).
func TestLoadState_HealsUpgradeCollision_Deterministic(t *testing.T) {
	for i := 0; i < 20; i++ {
		lookupTable := LookupTable{
			"endpointKey_B": map[string]int{"tcp/80": 12345},
			"endpointKey_A": map[string]int{"tcp/80": 12345},
		}
		healLoadStateCollisions(lookupTable)
		if len(lookupTable["endpointKey_A"]) != 1 {
			t.Fatalf("iteration %d: endpointKey_A should have kept its port", i)
		}
		if len(lookupTable["endpointKey_B"]) != 0 {
			t.Fatalf("iteration %d: endpointKey_B should have been cleared", i)
		}
	}
}

// TestAllocateAfterHeal confirms the cleared endpointKey gets a fresh port on
// the next allocation pass.
func TestAllocateAfterHeal(t *testing.T) {
	pa := NewPortAllocator()
	pa.portLookup = LookupTable{
		"endpointKey_A": map[string]int{"tcp/80": 12345},
		"endpointKey_B": map[string]int{},
	}
	pa.recomputeAvailablePorts()

	pa.AllocatePorts("endpointKey_B", []string{"tcp/80"})

	portB, ok := pa.Lookup("endpointKey_B", "tcp/80")
	if !ok {
		t.Fatalf("expected endpointKey_B to receive a port")
	}
	if portB == 12345 {
		t.Fatalf("expected endpointKey_B to get a port different from endpointKey_A's %d, got %d", 12345, portB)
	}
}

// TestHealLoadStateCollisions_NoopOnCleanState asserts healing does not mutate a
// lookup table that has no duplicates.
func TestHealLoadStateCollisions_NoopOnCleanState(t *testing.T) {
	original := LookupTable{
		"endpointKey_A": map[string]int{"tcp/80": 10001, "tcp/443": 10002},
		"endpointKey_B": map[string]int{"tcp/80": 20001, "udp/53": 20002},
	}
	snapshot := LookupTable{}
	for k, v := range original {
		inner := map[string]int{}
		for pk, pv := range v {
			inner[pk] = pv
		}
		snapshot[k] = inner
	}

	healLoadStateCollisions(original)

	for epKey, ports := range snapshot {
		for portKey, port := range ports {
			if original[epKey][portKey] != port {
				t.Fatalf("non-colliding entry mutated: %s/%s had %d, now %d",
					epKey, portKey, port, original[epKey][portKey])
			}
		}
	}
	if len(original["endpointKey_A"]) != 2 || len(original["endpointKey_B"]) != 2 {
		t.Fatalf("entry count changed: %+v", original)
	}
}

// TestLoadState_RecoversFromScrambledStatus drives LoadState end-to-end with a
// LoadBalancer whose status was written by the pre-fix updateLoadBalancerStatus
// (Name and UpstreamTargetPort indexed against differently-sorted parallel
// arrays). The relaxed match predicate must still recover the correct port
// mapping by matching on Name+Protocol alone.
func TestLoadState_RecoversFromScrambledStatus(t *testing.T) {
	lb := kubelbv1alpha1.LoadBalancer{}
	lb.Name = "test-lb"
	lb.Namespace = "tenant-test"
	lb.Spec.Endpoints = []kubelbv1alpha1.LoadBalancerEndpoints{
		{
			Ports: []kubelbv1alpha1.EndpointPort{
				{Name: "c", Port: 30003, Protocol: corev1.ProtocolTCP},
				{Name: "a", Port: 30001, Protocol: corev1.ProtocolTCP},
				{Name: "b", Port: 30002, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	lb.Status.Service.Ports = []kubelbv1alpha1.ServicePort{
		// Pre-fix code paired Name="a" (post-sort first) with Endpoints[0].Port = 30003.
		{ServicePort: corev1.ServicePort{Name: "a", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(10001)}, UpstreamTargetPort: 30003},
		{ServicePort: corev1.ServicePort{Name: "b", Port: 443, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(10002)}, UpstreamTargetPort: 30001},
		{ServicePort: corev1.ServicePort{Name: "c", Port: 8080, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(10003)}, UpstreamTargetPort: 30002},
	}

	pa := NewPortAllocator()
	if err := pa.LoadState(context.Background(), &stubReader{lbs: []kubelbv1alpha1.LoadBalancer{lb}}); err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointPattern, lb.Namespace, lb.Name, 0)
	wantTargetForName := map[string]int{"c": 10003, "a": 10001, "b": 10002}
	for _, ep := range lb.Spec.Endpoints[0].Ports {
		portKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, ep.Port, ep.Protocol)
		got, ok := pa.Lookup(endpointKey, portKey)
		if !ok {
			t.Errorf("endpoint %q (port=%d): no entry in allocator; LoadState failed to match scrambled status", ep.Name, ep.Port)
			continue
		}
		if got != wantTargetForName[ep.Name] {
			t.Errorf("endpoint %q: got allocated port %d, want %d", ep.Name, got, wantTargetForName[ep.Name])
		}
	}
}

// TestLoadState_HealsDuplicateRoutePort drives LoadState for the upgrade
// scenario: two Routes share a backend service and each Route's status
// carries the same TargetPort. LoadState must heal the duplicate.
func TestLoadState_HealsDuplicateRoutePort(t *testing.T) {
	mkRoute := func(name string) kubelbv1alpha1.Route {
		r := kubelbv1alpha1.Route{}
		r.Name = name
		r.Namespace = "tenant-test"
		r.Spec.Source.Kubernetes = &kubelbv1alpha1.KubernetesSource{}
		r.Spec.Source.Kubernetes.Services = []kubelbv1alpha1.UpstreamService{{
			Service: corev1.Service{},
		}}
		r.Spec.Source.Kubernetes.Services[0].Name = "foo"
		r.Spec.Source.Kubernetes.Services[0].Namespace = "app-ns"
		r.Spec.Source.Kubernetes.Route.SetName(name)
		r.Status.Resources.Services = map[string]kubelbv1alpha1.RouteServiceStatus{
			"app-ns/foo": {
				Ports: []corev1.ServicePort{
					{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(42000)},
				},
			},
		}
		return r
	}

	pa := NewPortAllocator()
	routes := []kubelbv1alpha1.Route{mkRoute("route-a"), mkRoute("route-b")}
	if err := pa.LoadState(context.Background(), &stubReader{routes: routes}); err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	keyA := fmt.Sprintf(kubelb.EnvoyEndpointRoutePattern, "tenant-test", "app-ns", "foo", "route-a")
	keyB := fmt.Sprintf(kubelb.EnvoyEndpointRoutePattern, "tenant-test", "app-ns", "foo", "route-b")
	portKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, int32(80), corev1.ProtocolTCP)

	_, aHas := pa.Lookup(keyA, portKey)
	_, bHas := pa.Lookup(keyB, portKey)
	if aHas == bHas {
		t.Fatalf("expected exactly one route to retain the port after heal; got A=%v B=%v", aHas, bHas)
	}
}

// TestPortAllocator_ConcurrentReadWrite hammers readers and writers on shared
// keys concurrently. Without the reader locks it triggers a data race (and the
// runtime "concurrent map read and map write" fatal) under -race.
func TestPortAllocator_ConcurrentReadWrite(t *testing.T) {
	pa := NewPortAllocator()
	const goroutines = 8
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			ep := fmt.Sprintf("ep-%d", n)
			for j := 0; j < 500; j++ {
				pa.AllocatePorts(ep, []string{"a", "b", "c"})
			}
		}(i)
		go func(n int) {
			defer wg.Done()
			ep := fmt.Sprintf("ep-%d", n)
			for j := 0; j < 500; j++ {
				pa.Lookup(ep, "a")
				_ = pa.GetPortLookupTable()
			}
		}(i)
	}
	wg.Wait()
}

// TestHealLoadStateCollisions_EndpointPatternKeys uses realistic kubelb pattern
// keys to catch accidental coupling to key format.
func TestHealLoadStateCollisions_EndpointPatternKeys(t *testing.T) {
	routeAKey := fmt.Sprintf(kubelb.EnvoyEndpointRoutePattern, "ns", "ns", "foo", "routeA")
	routeBKey := fmt.Sprintf(kubelb.EnvoyEndpointRoutePattern, "ns", "ns", "foo", "routeB")
	lookupTable := LookupTable{
		routeAKey: map[string]int{"tcp/80": 42000},
		routeBKey: map[string]int{"tcp/80": 42000},
	}
	healLoadStateCollisions(lookupTable)
	total := len(lookupTable[routeAKey]) + len(lookupTable[routeBKey])
	if total != 1 {
		t.Fatalf("expected exactly one endpointKey to keep the port, got total=%d (%+v)", total, lookupTable)
	}
}

// envoyStaticTCPPorts mirrors envoy.StaticTCPPorts(); the allocator cannot import that
// package. internal/envoy has the test that keeps the real list complete.
var envoyStaticTCPPorts = []int{9001, 19001, 19002, 19003, 19004}

func healedCount(t *testing.T) float64 {
	t.Helper()
	m := &dto.Metric{}
	if err := managermetrics.PortAllocatorCollisionsHealed.Write(m); err != nil {
		t.Fatalf("counter Write failed: %v", err)
	}
	return m.GetCounter().GetValue()
}

// Draw all eligible ports so a reserved or static port leaking into the pool
// is caught even when a small random sample would miss it.
func TestAllocatePortExcludesReservedAndStaticPorts(t *testing.T) {
	const staticPort = 12345 // Also reserve a static port outside the block.
	pa := NewPortAllocator(staticPort)
	available := (endPort - startPort) - (reservedEndPort - reservedStartPort + 1) - 1
	seen := make(map[int]bool, available)
	for range available {
		port := pa.allocatePort()
		if !IsAllocatable(port) || port == staticPort || seen[port] {
			t.Fatalf("allocated reserved, static, or duplicate port %d", port)
		}
		seen[port] = true
	}
}

func TestIsAllocatable(t *testing.T) {
	cases := map[int]bool{
		0:                     false,
		9090:                  false,
		startPort - 1:         false,
		startPort:             true,
		reservedStartPort - 1: true,
		reservedStartPort:     false,
		19001:                 false,
		19004:                 false,
		reservedEndPort:       false,
		reservedEndPort + 1:   true,
		endPort - 1:           true,
		endPort:               false,
	}
	for port, want := range cases {
		if got := IsAllocatable(port); got != want {
			t.Errorf("IsAllocatable(%d) = %v, want %v", port, got, want)
		}
	}
}

func lbWithStatusTargetPort(protocol corev1.Protocol, targetPort intstr.IntOrString) kubelbv1alpha1.LoadBalancer {
	lb := kubelbv1alpha1.LoadBalancer{}
	lb.Name = "test-lb"
	lb.Namespace = "tenant-test"
	lb.Spec.Endpoints = []kubelbv1alpha1.LoadBalancerEndpoints{{
		Ports: []kubelbv1alpha1.EndpointPort{{Name: "http", Port: 30900, Protocol: protocol}},
	}}
	lb.Spec.Ports = []kubelbv1alpha1.LoadBalancerPort{{Name: "http", Port: 9090, Protocol: protocol}}
	lb.Status.Service.Ports = []kubelbv1alpha1.ServicePort{{
		ServicePort:        corev1.ServicePort{Name: "http", Port: 9090, Protocol: protocol, TargetPort: targetPort},
		UpstreamTargetPort: 30900,
	}}
	return lb
}

// TestLoadState_DropsUnusableLoadBalancerPorts loads the states behind
// kubermatic/kubelb#661 and asserts each gets a fresh, usable port on the next
// allocation, the way a manager restart on the fixed version heals them.
func TestLoadState_DropsUnusableLoadBalancerPorts(t *testing.T) {
	cases := []struct {
		name       string
		targetPort intstr.IntOrString
	}{
		{"stats listener", intstr.FromInt(19001)},
		{"shutdown manager", intstr.FromInt(19002)},
		{"readiness listener", intstr.FromInt(19003)},
		{"health check listener", intstr.FromInt(19004)},
		// A targetPort removed from the status by hand is read back as 0.
		{"targetPort removed from the status", intstr.IntOrString{}},
		// Kubernetes defaults a Service targetPort of 0 to the Service port, and
		// the manager then writes that value into the status.
		{"service port written back to the status", intstr.FromInt(9090)},
		{"admin port", intstr.FromInt(9001)},
		{"above the range", intstr.FromInt(endPort)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lb := lbWithStatusTargetPort(corev1.ProtocolTCP, tc.targetPort)
			pa := NewPortAllocator(envoyStaticTCPPorts...)
			before := healedCount(t)
			if err := pa.LoadState(context.Background(), &stubReader{lbs: []kubelbv1alpha1.LoadBalancer{lb}}); err != nil {
				t.Fatalf("LoadState: %v", err)
			}

			endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointPattern, lb.Namespace, lb.Name, 0)
			portKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, int32(30900), corev1.ProtocolTCP)
			if port, ok := pa.Lookup(endpointKey, portKey); ok {
				t.Fatalf("LoadState kept unusable port %d", port)
			}
			if got := healedCount(t) - before; got != 1 {
				t.Fatalf("expected the healed counter to grow by 1, got %v", got)
			}

			if err := pa.AllocatePortsForLoadBalancers(kubelbv1alpha1.LoadBalancerList{Items: []kubelbv1alpha1.LoadBalancer{lb}}); err != nil {
				t.Fatalf("AllocatePortsForLoadBalancers: %v", err)
			}
			port, ok := pa.Lookup(endpointKey, portKey)
			if !ok {
				t.Fatalf("no port allocated after the heal")
			}
			if !IsAllocatable(port) {
				t.Fatalf("port %d allocated after the heal is not usable", port)
			}
			// The hub Service must target the new port, never 0.
			svcPorts := kubelb.CreateServicePorts(&lb, &corev1.Service{}, pa)
			if len(svcPorts) != 1 || svcPorts[0].TargetPort.IntValue() != port {
				t.Fatalf("Service targetPort = %+v, want %d", svcPorts, port)
			}
		})
	}
}

// TestLoadState_KeepsUsablePorts asserts LoadState does not move ports that work.
// A port inside the reserved block that nothing binds is one of them: it is no
// longer handed out, but an existing allocation keeps serving.
func TestLoadState_KeepsUsablePorts(t *testing.T) {
	for _, tc := range []struct {
		port     int
		protocol corev1.Protocol
	}{
		{startPort, corev1.ProtocolTCP},
		{19000, corev1.ProtocolTCP},
		{19002, corev1.ProtocolUDP}, // The shutdown manager listens on TCP.
		{19003, corev1.ProtocolUDP}, // The Envoy readiness listener uses TCP.
		{19005, corev1.ProtocolTCP},
		{19050, corev1.ProtocolTCP},
		{reservedEndPort, corev1.ProtocolTCP},
		{41349, corev1.ProtocolTCP},
		{endPort - 1, corev1.ProtocolTCP},
	} {
		t.Run(fmt.Sprintf("%d/%s", tc.port, tc.protocol), func(t *testing.T) {
			lb := lbWithStatusTargetPort(tc.protocol, intstr.FromInt(tc.port))
			pa := NewPortAllocator(envoyStaticTCPPorts...)
			before := healedCount(t)
			if err := pa.LoadState(context.Background(), &stubReader{lbs: []kubelbv1alpha1.LoadBalancer{lb}}); err != nil {
				t.Fatalf("LoadState: %v", err)
			}
			endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointPattern, lb.Namespace, lb.Name, 0)
			portKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, int32(30900), tc.protocol)
			got, ok := pa.Lookup(endpointKey, portKey)
			if !ok || got != tc.port {
				t.Fatalf("LoadState moved a usable port: got %d (found=%v), want %d", got, ok, tc.port)
			}
			if healed := healedCount(t) - before; healed != 0 {
				t.Fatalf("healed counter grew by %v for a usable port", healed)
			}
		})
	}
}

// TestLoadState_DropsStaticRoutePort covers the Route side: the same allocator
// serves Route services, and their ports are stored in the Route status.
func TestLoadState_DropsStaticRoutePort(t *testing.T) {
	r := kubelbv1alpha1.Route{}
	r.Name = "route-a"
	r.Namespace = "tenant-test"
	r.Spec.Source.Kubernetes = &kubelbv1alpha1.KubernetesSource{}
	r.Spec.Source.Kubernetes.Services = []kubelbv1alpha1.UpstreamService{{Service: corev1.Service{}}}
	r.Spec.Source.Kubernetes.Services[0].Name = "foo"
	r.Spec.Source.Kubernetes.Services[0].Namespace = "app-ns"
	r.Spec.Source.Kubernetes.Services[0].Spec.Ports = []corev1.ServicePort{
		{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP},
		{Name: "https", Port: 443, Protocol: corev1.ProtocolTCP},
		{Name: "dns", Port: 53, Protocol: corev1.ProtocolUDP},
	}
	r.Spec.Source.Kubernetes.Route.SetName(r.Name)
	r.Status.Resources.Services = map[string]kubelbv1alpha1.RouteServiceStatus{
		"app-ns/foo": {
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(19003)},
				{Name: "https", Port: 443, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(42000)},
				{Name: "dns", Port: 53, Protocol: corev1.ProtocolUDP, TargetPort: intstr.FromInt(19003)},
			},
		},
	}

	pa := NewPortAllocator(envoyStaticTCPPorts...)
	if err := pa.LoadState(context.Background(), &stubReader{routes: []kubelbv1alpha1.Route{r}}); err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointRoutePattern, "tenant-test", "app-ns", "foo", "route-a")
	httpKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, int32(80), corev1.ProtocolTCP)
	httpsKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, int32(443), corev1.ProtocolTCP)
	if port, ok := pa.Lookup(endpointKey, httpKey); ok {
		t.Fatalf("LoadState kept static port %d for a Route service", port)
	}
	if port, ok := pa.Lookup(endpointKey, httpsKey); !ok || port != 42000 {
		t.Fatalf("LoadState disturbed the usable port of the same service: %d (found=%v)", port, ok)
	}

	if err := pa.AllocatePortsForRoutes([]kubelbv1alpha1.Route{r}); err != nil {
		t.Fatalf("AllocatePortsForRoutes: %v", err)
	}
	port, ok := pa.Lookup(endpointKey, httpKey)
	if !ok || !IsAllocatable(port) {
		t.Fatalf("expected a usable port after the heal, got %d (found=%v)", port, ok)
	}
	if kept, _ := pa.Lookup(endpointKey, httpsKey); kept != 42000 {
		t.Fatalf("the usable port moved during reallocation: %d", kept)
	}
	udpKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, int32(53), corev1.ProtocolUDP)
	if kept, ok := pa.Lookup(endpointKey, udpKey); !ok || kept != 19003 {
		t.Fatalf("expected the existing UDP listener on 19003, got %d (found=%v)", kept, ok)
	}
}
