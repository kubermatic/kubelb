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

package kubelb

import (
	"context"
	"fmt"
	"testing"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/envoy"
	"k8c.io/kubelb/internal/kubelb"
	portlookup "k8c.io/kubelb/internal/port-lookup"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// A LoadBalancer whose port was allocated on a static port of the Envoy proxy pod
// (kubermatic/kubelb#661) gets a new port when the manager starts. The new port
// has to reach the LoadBalancer status in the same reconcile: the allocator is
// loaded from the status, so a status that keeps the old port makes every manager
// start move the listener again.
//
// The client in this test answers Service reads the way the informer cache does
// right after a write: with the object from before the update.
func TestLoadBalancerStatusFollowsReallocatedPort(t *testing.T) {
	const collidingPort = envoy.EnvoyReadinessPort
	ctx := context.Background()
	s := annotationTestScheme(t)

	lb := annotatedLoadBalancer(nil)
	tenant := &kubelbv1alpha1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: RemoveTenantPrefix(lb.Namespace)}}
	lb.Status.Service.Ports = []kubelbv1alpha1.ServicePort{{
		ServicePort:        corev1.ServicePort{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(collidingPort)},
		UpstreamTargetPort: 30080,
	}}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: annotationTestService, Namespace: annotationTestNamespace},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(collidingPort)}},
		},
	}

	apiClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(lb, svc, tenant).
		WithStatusSubresource(&kubelbv1alpha1.LoadBalancer{}).
		Build()
	staleService := getGeneratedService(t, apiClient)
	cachedClient := interceptor.NewClient(apiClient, interceptor.Funcs{
		Get: func(ctx context.Context, c ctrlruntimeclient.WithWatch, key ctrlruntimeclient.ObjectKey, obj ctrlruntimeclient.Object, opts ...ctrlruntimeclient.GetOption) error {
			if out, ok := obj.(*corev1.Service); ok && key == ctrlruntimeclient.ObjectKeyFromObject(staleService) {
				staleService.DeepCopyInto(out)
				return nil
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})

	// The manager starts: the colliding port is dropped on load and reallocated.
	allocator := portlookup.NewPortAllocator(envoy.StaticTCPPorts()...)
	if err := allocator.LoadState(ctx, apiClient); err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	r := &LoadBalancerReconciler{Client: cachedClient, Scheme: s, PortAllocator: allocator}
	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: ctrlruntimeclient.ObjectKeyFromObject(lb)}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointPattern, lb.Namespace, lb.Name, 0)
	portKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, int32(30080), corev1.ProtocolTCP)
	newPort, ok := allocator.Lookup(endpointKey, portKey)
	if !ok || newPort == collidingPort {
		t.Fatalf("expected a fresh port instead of %d, got %d (found=%v)", collidingPort, newPort, ok)
	}

	written := getGeneratedService(t, apiClient)
	if len(written.Spec.Ports) != 1 || written.Spec.Ports[0].TargetPort.IntValue() != newPort {
		t.Fatalf("persisted Service ports = %+v, want targetPort %d", written.Spec.Ports, newPort)
	}
	stored := &kubelbv1alpha1.LoadBalancer{}
	if err := apiClient.Get(ctx, ctrlruntimeclient.ObjectKeyFromObject(lb), stored); err != nil {
		t.Fatalf("get loadbalancer: %v", err)
	}
	if len(stored.Status.Service.Ports) != 1 || stored.Status.Service.Ports[0].TargetPort.IntValue() != newPort {
		t.Fatalf("status targetPort = %+v, want %d", stored.Status.Service.Ports, newPort)
	}

	// The next manager start loads the new port and has nothing left to heal.
	restarted := portlookup.NewPortAllocator(envoy.StaticTCPPorts()...)
	if err := restarted.LoadState(ctx, apiClient); err != nil {
		t.Fatalf("LoadState after restart: %v", err)
	}
	if got, ok := restarted.Lookup(endpointKey, portKey); !ok || got != newPort {
		t.Fatalf("after a restart the allocator holds %d (found=%v), want %d", got, ok, newPort)
	}
}
