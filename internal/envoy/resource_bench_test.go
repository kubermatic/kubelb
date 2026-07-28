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
	"fmt"
	"testing"

	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	portlookup "k8c.io/kubelb/internal/port-lookup"

	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// The manager regenerates a tenant's entire snapshot on every reconcile, so
// snapshot generation cost scales with the tenant's total resource count rather
// than with what actually changed. These benchmarks pin down that cost along the
// two axes that grow independently in practice: number of LoadBalancers/Routes,
// and number of endpoints behind each one.

func benchEndpointAddresses(n int) []kubelbv1alpha1.EndpointAddress {
	addrs := make([]kubelbv1alpha1.EndpointAddress, n)
	for i := range addrs {
		addrs[i] = kubelbv1alpha1.EndpointAddress{
			IP: fmt.Sprintf("10.%d.%d.%d", i/65536%256, i/256%256, i%256),
		}
	}
	return addrs
}

func benchLoadBalancers(count, endpointsPer int) []kubelbv1alpha1.LoadBalancer {
	lbs := make([]kubelbv1alpha1.LoadBalancer, count)
	for i := range lbs {
		lb := inlineAddressLB()
		lb.Name = fmt.Sprintf("lb-%d", i)
		lb.UID = k8stypes.UID(fmt.Sprintf("lb-uid-%d", i))
		lb.Spec.Endpoints[0].Addresses = benchEndpointAddresses(endpointsPer)
		lbs[i] = lb
	}
	return lbs
}

func benchRoutes(count, endpointsPer int) []kubelbv1alpha1.Route {
	routes := make([]kubelbv1alpha1.Route, count)
	for i := range routes {
		route := ingressSourceRoute()
		name := fmt.Sprintf("route-%d", i)
		route.Name = name
		route.Spec.Source.Kubernetes.Route.SetName(name)
		svc := &route.Spec.Source.Kubernetes.Services[0].Service
		svc.Name = name
		svc.UID = k8stypes.UID(fmt.Sprintf("svc-uid-%d", i))
		route.Spec.Endpoints[0].Addresses = benchEndpointAddresses(endpointsPer)
		routes[i] = route
	}
	return routes
}

func benchSnapshot(b *testing.B, lbs []kubelbv1alpha1.LoadBalancer, routes []kubelbv1alpha1.Route) {
	b.Helper()

	ctx := context.Background()
	cl := fake.NewClientBuilder().Build()

	// Ports are allocated once, as they are in the manager. Without distinct
	// listener ports dedupListenersByAddress collapses everything onto a single
	// listener and the benchmark stops scaling.
	pa := portlookup.NewPortAllocator()
	if err := pa.AllocatePortsForLoadBalancers(kubelbv1alpha1.LoadBalancerList{Items: lbs}); err != nil {
		b.Fatalf("allocate LB ports: %v", err)
	}
	if err := pa.AllocatePortsForRoutes(routes); err != nil {
		b.Fatalf("allocate route ports: %v", err)
	}

	// A snapshot that silently collapses to a handful of clusters would make the
	// numbers meaningless, so confirm the fixture actually scales before timing.
	snapshot, err := MapSnapshot(ctx, cl, lbs, routes, pa, "bench-snapshot", ResolveHeaderLimits(nil))
	if err != nil {
		b.Fatalf("MapSnapshot() error = %v", err)
	}
	if got, want := len(snapshot.GetResources(resource.ClusterType)), len(lbs)+len(routes); got != want {
		b.Fatalf("fixture produced %d clusters, want %d", got, want)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := MapSnapshot(ctx, cl, lbs, routes, pa, "bench-snapshot", ResolveHeaderLimits(nil)); err != nil {
			b.Fatalf("MapSnapshot() error = %v", err)
		}
	}
}

func BenchmarkMapSnapshotLoadBalancers(b *testing.B) {
	for _, tc := range []struct{ count, endpointsPer int }{
		{10, 10},
		{100, 10},
		{100, 100},
		{10, 1000},
	} {
		b.Run(fmt.Sprintf("lbs=%d/endpoints=%d", tc.count, tc.endpointsPer), func(b *testing.B) {
			benchSnapshot(b, benchLoadBalancers(tc.count, tc.endpointsPer), nil)
		})
	}
}

func BenchmarkMapSnapshotRoutes(b *testing.B) {
	for _, tc := range []struct{ count, endpointsPer int }{
		{10, 10},
		{100, 10},
		{100, 100},
		{10, 1000},
	} {
		b.Run(fmt.Sprintf("routes=%d/endpoints=%d", tc.count, tc.endpointsPer), func(b *testing.B) {
			benchSnapshot(b, nil, benchRoutes(tc.count, tc.endpointsPer))
		})
	}
}

// A tenant running both L4 and L7 shares one snapshot, which is the shape most
// real tenants have.
func BenchmarkMapSnapshotMixed(b *testing.B) {
	benchSnapshot(b, benchLoadBalancers(50, 50), benchRoutes(50, 50))
}
