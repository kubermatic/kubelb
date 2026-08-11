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
	"strings"
	"testing"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	portlookup "k8c.io/kubelb/internal/port-lookup"
	"k8c.io/kubelb/internal/resources/unstructured"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	teardownTenant          = "primary"
	teardownNamespace       = "tenant-primary"
	teardownConfigNamespace = "kubelb"
	teardownIngressName     = "default-echo"
	teardownServiceName     = "default-echo-svc"
	seededLoadBalancerIP    = "203.0.113.10"
)

func teardownScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		kubelbv1alpha1.AddToScheme,
		gwapiv1.Install,
	} {
		if err := add(s); err != nil {
			t.Fatalf("failed to build scheme: %v", err)
		}
	}
	return s
}

func teardownIngressSource(t *testing.T) kubelbv1alpha1.RouteSource {
	t.Helper()
	ingress := &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: networkingv1.SchemeGroupVersion.String(),
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "echo", Namespace: "default"},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: "app.example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: ptr.To(networkingv1.PathTypePrefix),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "echo",
									Port: networkingv1.ServiceBackendPort{Number: 80},
								},
							},
						}},
					},
				},
			}},
		},
	}

	unstruct, err := unstructured.ConvertObjectToUnstructured(ingress)
	if err != nil {
		t.Fatalf("failed to convert source to unstructured: %v", err)
	}
	return kubelbv1alpha1.RouteSource{
		Kubernetes: &kubelbv1alpha1.KubernetesSource{Route: *unstruct},
	}
}

// admittedRoute is a Route that was accepted on a previous reconcile: it carries
// the cleanup finalizer and a status pointing at the resources it generated.
func admittedRoute(t *testing.T) *kubelbv1alpha1.Route {
	t.Helper()
	return &kubelbv1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "route-uid",
			Namespace:  teardownNamespace,
			Finalizers: []string{CleanupFinalizer},
		},
		Spec: kubelbv1alpha1.RouteSpec{Source: teardownIngressSource(t)},
		Status: kubelbv1alpha1.RouteStatus{
			Resources: kubelbv1alpha1.RouteResourcesStatus{
				Route: kubelbv1alpha1.ResourceState{
					Name:          "echo",
					Namespace:     "default",
					GeneratedName: teardownIngressName,
					Conditions: []metav1.Condition{{
						Type:               kubelbv1alpha1.ConditionResourceAppliedSuccessfully.String(),
						Status:             metav1.ConditionTrue,
						Reason:             conditionReasonSuccessful,
						LastTransitionTime: metav1.Now(),
					}},
				},
				Services: map[string]kubelbv1alpha1.RouteServiceStatus{
					"default/echo": {
						ResourceState: kubelbv1alpha1.ResourceState{
							Name:          "echo",
							Namespace:     "default",
							GeneratedName: teardownServiceName,
						},
					},
				},
			},
		},
	}
}

// pendingRoute has never been reconciled: no status, nothing generated yet.
func pendingRoute(t *testing.T) *kubelbv1alpha1.Route {
	t.Helper()
	return &kubelbv1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "route-uid",
			Namespace:  teardownNamespace,
			Finalizers: []string{CleanupFinalizer},
		},
		Spec: kubelbv1alpha1.RouteSpec{Source: teardownIngressSource(t)},
	}
}

func generatedIngress() *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: teardownIngressName, Namespace: teardownNamespace},
	}
}

func generatedService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: teardownServiceName, Namespace: teardownNamespace},
	}
}

func newTeardownReconciler(t *testing.T, objects ...ctrlruntimeclient.Object) *RouteReconciler {
	t.Helper()
	scheme := teardownScheme(t)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&kubelbv1alpha1.Route{}).
		Build()

	return &RouteReconciler{
		Client:        client,
		Log:           logr.Discard(),
		Scheme:        scheme,
		Recorder:      events.NewFakeRecorder(16),
		Namespace:     teardownConfigNamespace,
		PortAllocator: portlookup.NewPortAllocator(),
	}
}

// newCacheMissReconciler makes every Ingress read miss, which is what an
// informer cache does for an object that was created moments ago. Flipping the
// returned toggle restores normal reads so assertions can observe the cluster.
func newCacheMissReconciler(t *testing.T, objects ...ctrlruntimeclient.Object) (*RouteReconciler, *bool) {
	t.Helper()
	blocked := true

	scheme := teardownScheme(t)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&kubelbv1alpha1.Route{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c ctrlruntimeclient.WithWatch, key ctrlruntimeclient.ObjectKey, obj ctrlruntimeclient.Object, opts ...ctrlruntimeclient.GetOption) error {
				if _, ok := obj.(*networkingv1.Ingress); ok && blocked {
					return kerrors.NewNotFound(schema.GroupResource{Group: networkingv1.GroupName, Resource: "ingresses"}, key.Name)
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).
		Build()

	return &RouteReconciler{
		Client:        client,
		Log:           logr.Discard(),
		Scheme:        scheme,
		Recorder:      events.NewFakeRecorder(16),
		Namespace:     teardownConfigNamespace,
		PortAllocator: portlookup.NewPortAllocator(),
	}, &blocked
}

func manageIngressRoute(t *testing.T, r *RouteReconciler, route *kubelbv1alpha1.Route) {
	t.Helper()
	err := r.manageRoutes(context.Background(), logr.Discard(), route, &kubelbv1alpha1.Config{}, &kubelbv1alpha1.Tenant{}, kubelbv1alpha1.AnnotationSettings{})
	if err != nil {
		t.Fatalf("manageRoutes returned an error: %v", err)
	}
}

func recordedResource(t *testing.T, r *RouteReconciler, route *kubelbv1alpha1.Route) kubelbv1alpha1.ResourceState {
	t.Helper()
	updated := &kubelbv1alpha1.Route{}
	if err := r.Get(context.Background(), ctrlruntimeclient.ObjectKeyFromObject(route), updated); err != nil {
		t.Fatalf("failed to get the Route: %v", err)
	}
	return updated.Status.Resources.Route
}

// The owner reference only collects the generated resource when the Route
// itself is deleted. On the disable path the Route survives, so it has to be
// deleted explicitly or it keeps serving.
func TestCleanupDeletesGeneratedRouteResource(t *testing.T) {
	tests := []struct {
		name        string
		resetStatus bool
	}{
		{name: "teardown resets the status", resetStatus: true},
		{name: "deletion keeps the status", resetStatus: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := admittedRoute(t)
			r := newTeardownReconciler(t, route, generatedIngress(), generatedService())

			ctx := context.Background()
			if _, err := r.cleanup(ctx, route, tt.resetStatus); err != nil {
				t.Fatalf("cleanup returned an error: %v", err)
			}

			if err := r.Get(ctx, ctrlruntimeclient.ObjectKey{Name: teardownIngressName, Namespace: teardownNamespace}, &networkingv1.Ingress{}); !kerrors.IsNotFound(err) {
				t.Errorf("expected the generated Ingress to be deleted, got %v", err)
			}
			if err := r.Get(ctx, ctrlruntimeclient.ObjectKey{Name: teardownServiceName, Namespace: teardownNamespace}, &corev1.Service{}); !kerrors.IsNotFound(err) {
				t.Errorf("expected the generated Service to be deleted, got %v", err)
			}

			updated := &kubelbv1alpha1.Route{}
			if err := r.Get(ctx, ctrlruntimeclient.ObjectKeyFromObject(route), updated); err != nil {
				t.Fatalf("failed to get the Route: %v", err)
			}

			generatedName := updated.Status.Resources.Route.GeneratedName
			if tt.resetStatus && generatedName != "" {
				t.Errorf("expected the status to be reset, got generatedName %q", generatedName)
			}
			if !tt.resetStatus && generatedName != teardownIngressName {
				t.Errorf("expected the status to be kept, got generatedName %q", generatedName)
			}
			if apimeta.FindStatusCondition(updated.Status.Resources.Route.Conditions, kubelbv1alpha1.ConditionResourceAppliedSuccessfully.String()) == nil {
				t.Error("expected the applied condition to survive cleanup")
			}
		})
	}
}

// A Route with nothing recorded in its status generated nothing, so there is
// no name to delete and no kind to resolve.
func TestCleanupSkipsRouteWithoutGeneratedResource(t *testing.T) {
	route := pendingRoute(t)
	r := newTeardownReconciler(t, route)

	if _, err := r.cleanup(context.Background(), route, true); err != nil {
		t.Fatalf("cleanup returned an error: %v", err)
	}
}

// Disabling Ingress for the tenant used to leave the mirrored Ingress live
// while the Route was told the feature was off.
func TestReconcileTearsDownRouteWhenIngressIsDisabled(t *testing.T) {
	route := admittedRoute(t)
	tenant := &kubelbv1alpha1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: teardownTenant}}
	tenant.Spec.Ingress.Disable = true

	r := newTeardownReconciler(t, route, tenant, generatedIngress(), generatedService())

	ctx := context.Background()
	if _, err := r.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: route.Name, Namespace: route.Namespace},
	}); err != nil {
		t.Fatalf("Reconcile returned an error: %v", err)
	}

	if err := r.Get(ctx, ctrlruntimeclient.ObjectKey{Name: teardownIngressName, Namespace: teardownNamespace}, &networkingv1.Ingress{}); !kerrors.IsNotFound(err) {
		t.Errorf("expected the generated Ingress to be deleted, got %v", err)
	}

	updated := &kubelbv1alpha1.Route{}
	if err := r.Get(ctx, ctrlruntimeclient.ObjectKeyFromObject(route), updated); err != nil {
		t.Fatalf("failed to get the Route: %v", err)
	}
	if updated.Status.Resources.Route.GeneratedName != "" {
		t.Errorf("expected the status to be reset, got generatedName %q", updated.Status.Resources.Route.GeneratedName)
	}
	if len(updated.Finalizers) != 0 {
		t.Errorf("expected the finalizer to be removed, got %v", updated.Finalizers)
	}
}

// A read-after-write that misses must not wipe the record of what was applied:
// cleanup and the CCM's status projection both key off GeneratedName.
func TestManageRoutesRecordsResourceWhenReadAfterWriteMissesCache(t *testing.T) {
	route := pendingRoute(t)
	r, _ := newCacheMissReconciler(t, route)

	manageIngressRoute(t, r, route)

	applied := recordedResource(t, r, route)
	if applied.GeneratedName != teardownIngressName {
		t.Errorf("expected generatedName %q, got %q", teardownIngressName, applied.GeneratedName)
	}
	if applied.Name != "echo" || applied.Namespace != "default" {
		t.Errorf("expected the origin to be recorded as default/echo, got %s/%s", applied.Namespace, applied.Name)
	}
}

// The live object is what carries the sub-resource status, so it must win
// whenever the cache does have it.
func TestManageRoutesPrefersLiveResource(t *testing.T) {
	route := pendingRoute(t)
	existing := generatedIngress()
	existing.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{IP: seededLoadBalancerIP}}

	scheme := teardownScheme(t)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(route, existing).
		WithStatusSubresource(&kubelbv1alpha1.Route{}, &networkingv1.Ingress{}).
		Build()

	r := &RouteReconciler{
		Client:        client,
		Log:           logr.Discard(),
		Scheme:        scheme,
		Recorder:      events.NewFakeRecorder(16),
		Namespace:     teardownConfigNamespace,
		PortAllocator: portlookup.NewPortAllocator(),
	}

	manageIngressRoute(t, r, route)

	applied := recordedResource(t, r, route)
	if applied.GeneratedName != teardownIngressName {
		t.Errorf("expected generatedName %q, got %q", teardownIngressName, applied.GeneratedName)
	}
	if !strings.Contains(string(applied.Status.Raw), seededLoadBalancerIP) {
		t.Errorf("expected the live Ingress status to be recorded, got %s", applied.Status.Raw)
	}
}

// A teardown landing right after the first create must still find the mirrored
// resource: the wiped status was what left it orphaned and serving traffic.
func TestTeardownFindsResourceRecordedFromCacheMiss(t *testing.T) {
	route := pendingRoute(t)
	r, blocked := newCacheMissReconciler(t, route)

	manageIngressRoute(t, r, route)
	*blocked = false

	ctx := context.Background()
	if err := r.Get(ctx, ctrlruntimeclient.ObjectKey{Name: teardownIngressName, Namespace: teardownNamespace}, &networkingv1.Ingress{}); err != nil {
		t.Fatalf("expected the Ingress to have been created: %v", err)
	}

	if _, err := r.cleanup(ctx, route, true); err != nil {
		t.Fatalf("cleanup returned an error: %v", err)
	}

	if err := r.Get(ctx, ctrlruntimeclient.ObjectKey{Name: teardownIngressName, Namespace: teardownNamespace}, &networkingv1.Ingress{}); !kerrors.IsNotFound(err) {
		t.Errorf("expected the generated Ingress to be deleted, got %v", err)
	}
}
