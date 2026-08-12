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
	"testing"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	portlookup "k8c.io/kubelb/internal/port-lookup"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	annotationTestNamespace = "tenant-annotations"
	annotationTestLBName    = "lb-uid-1"
	annotationTestService   = "envoy-lb-uid-1"
	annotationTestApp       = "envoy-app"

	providerAnnotation   = "load-balancer.hetzner.cloud/location"
	thirdPartyAnnotation = "cloud-controller.example.com/state"
)

func annotationTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("corev1 scheme: %v", err)
	}
	if err := kubelbv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("kubelb scheme: %v", err)
	}
	return s
}

func annotatedLoadBalancer(annotations map[string]string) *kubelbv1alpha1.LoadBalancer {
	return &kubelbv1alpha1.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:        annotationTestLBName,
			Namespace:   annotationTestNamespace,
			Annotations: annotations,
			Labels: map[string]string{
				kubelb.LabelOriginName:      "app",
				kubelb.LabelOriginNamespace: "default",
			},
		},
		Spec: kubelbv1alpha1.LoadBalancerSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []kubelbv1alpha1.LoadBalancerPort{{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP}},
			Endpoints: []kubelbv1alpha1.LoadBalancerEndpoints{{
				Ports: []kubelbv1alpha1.EndpointPort{{Name: "http", Port: 30080, Protocol: corev1.ProtocolTCP}},
			}},
		},
	}
}

func getGeneratedService(t *testing.T, client ctrlruntimeclient.Client) *corev1.Service {
	t.Helper()
	svc := &corev1.Service{}
	key := types.NamespacedName{Name: annotationTestService, Namespace: annotationTestNamespace}
	if err := client.Get(context.Background(), key, svc); err != nil {
		t.Fatalf("get generated service: %v", err)
	}
	return svc
}

// A tenant that removes a cloud-provider annotation from its Service must see it
// removed from the generated Service too, otherwise the annotation can never be
// un-set. Annotations set by third parties on the generated Service are not ours to
// remove and have to survive.
func TestReconcileService_AnnotationRemovalPropagates(t *testing.T) {
	settings := kubelbv1alpha1.AnnotationSettings{PropagateAllAnnotations: ptr.To(true)}
	s := annotationTestScheme(t)
	client := fake.NewClientBuilder().WithScheme(s).Build()
	r := &LoadBalancerReconciler{Client: client, Scheme: s}
	allocator := portlookup.NewPortAllocator()
	ctx := context.Background()

	lb := annotatedLoadBalancer(map[string]string{providerAnnotation: "nbg1"})
	if err := r.reconcileService(ctx, lb, annotationTestService, annotationTestApp, annotationTestNamespace, allocator, nil, settings); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}

	// A third party annotates the generated Service.
	svc := getGeneratedService(t, client)
	svc.Annotations[thirdPartyAnnotation] = "ready"
	if err := client.Update(ctx, svc); err != nil {
		t.Fatalf("annotate service: %v", err)
	}

	// The tenant removes the provider annotation from its Service, which drops it
	// from the LoadBalancer CR.
	if err := r.reconcileService(ctx, annotatedLoadBalancer(nil), annotationTestService, annotationTestApp, annotationTestNamespace, allocator, nil, settings); err != nil {
		t.Fatalf("reconcile after removal: %v", err)
	}

	got := getGeneratedService(t, client)
	if value, ok := got.Annotations[providerAnnotation]; ok {
		t.Fatalf("annotation %s should have been removed, still set to %q", providerAnnotation, value)
	}
	if got.Annotations[thirdPartyAnnotation] != "ready" {
		t.Fatalf("third party annotation must be preserved, got %+v", got.Annotations)
	}
}

// Value changes keep propagating, and re-reconciling an unchanged LoadBalancer must
// not rewrite the Service (no update hot loop from the bookkeeping annotation).
func TestReconcileService_AnnotationUpdateIsStable(t *testing.T) {
	settings := kubelbv1alpha1.AnnotationSettings{PropagateAllAnnotations: ptr.To(true)}
	s := annotationTestScheme(t)
	client := fake.NewClientBuilder().WithScheme(s).Build()
	r := &LoadBalancerReconciler{Client: client, Scheme: s}
	allocator := portlookup.NewPortAllocator()
	ctx := context.Background()

	if err := r.reconcileService(ctx, annotatedLoadBalancer(map[string]string{providerAnnotation: "nbg1"}), annotationTestService, annotationTestApp, annotationTestNamespace, allocator, nil, settings); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}
	if err := r.reconcileService(ctx, annotatedLoadBalancer(map[string]string{providerAnnotation: "fsn1"}), annotationTestService, annotationTestApp, annotationTestNamespace, allocator, nil, settings); err != nil {
		t.Fatalf("reconcile after value change: %v", err)
	}

	got := getGeneratedService(t, client)
	if got.Annotations[providerAnnotation] != "fsn1" {
		t.Fatalf("expected updated annotation value, got %+v", got.Annotations)
	}
	resourceVersion := got.ResourceVersion

	if err := r.reconcileService(ctx, annotatedLoadBalancer(map[string]string{providerAnnotation: "fsn1"}), annotationTestService, annotationTestApp, annotationTestNamespace, allocator, nil, settings); err != nil {
		t.Fatalf("no-op reconcile: %v", err)
	}
	if got = getGeneratedService(t, client); got.ResourceVersion != resourceVersion {
		t.Fatalf("unchanged LoadBalancer rewrote the Service (resourceVersion %s -> %s)", resourceVersion, got.ResourceVersion)
	}
}
