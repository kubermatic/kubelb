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
	"testing"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"

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
