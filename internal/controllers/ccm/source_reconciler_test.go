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

package ccm

import (
	"context"
	"testing"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	testNS      = "tenant-ns"
	testCluster = "tenant1"
	testIngName = "demo"
	testIngUID  = "ing-uid-1"
	testSvcName = "backend"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("corev1 scheme: %v", err)
	}
	if err := networkingv1.AddToScheme(s); err != nil {
		t.Fatalf("networkingv1 scheme: %v", err)
	}
	if err := kubelbv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("kubelb scheme: %v", err)
	}
	return s
}

func newIngress(opts ...func(*networkingv1.Ingress)) *networkingv1.Ingress {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testIngName,
			Namespace: testNS,
			UID:       testIngUID,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{Name: testSvcName},
							},
						}},
					},
				},
			}},
		},
	}
	for _, o := range opts {
		o(ing)
	}
	return ing
}

func newLoop(t *testing.T, src ctrlclient.Client, lb ctrlclient.Client) *SourceReconciler[*networkingv1.Ingress] {
	t.Helper()
	return &SourceReconciler[*networkingv1.Ingress]{
		Client:      src,
		LBClient:    lb,
		ClusterName: testCluster,
		Log:         logr.Discard(),
		Adapter:     &ingressAdapter{useClass: false},
	}
}

func req() ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: testIngName, Namespace: testNS}}
}

func TestSourceReconciler_NotFound(t *testing.T) {
	s := newScheme(t)
	src := fake.NewClientBuilder().WithScheme(s).Build()
	lb := fake.NewClientBuilder().WithScheme(s).Build()
	loop := newLoop(t, src, lb)

	got, err := loop.Reconcile(context.Background(), req())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.RequeueAfter != 0 {
		t.Fatalf("expected empty result, got %+v", got)
	}
}

func TestSourceReconciler_DeletionWithoutFinalizer(t *testing.T) {
	s := newScheme(t)
	now := metav1.Now()
	ing := newIngress(func(i *networkingv1.Ingress) {
		i.DeletionTimestamp = &now
		// Unrelated finalizer; the fake client GCs objects once the last finalizer is removed.
		i.Finalizers = []string{"keep-alive"}
	})
	src := fake.NewClientBuilder().WithScheme(s).WithObjects(ing).Build()
	lb := fake.NewClientBuilder().WithScheme(s).Build()
	loop := newLoop(t, src, lb)

	if _, err := loop.Reconcile(context.Background(), req()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	route := &kubelbv1alpha1.Route{}
	err := lb.Get(context.Background(), types.NamespacedName{Name: testIngUID, Namespace: testCluster}, route)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected Route to be absent, got err=%v", err)
	}
}

func TestSourceReconciler_DeletionWithFinalizer_CleansUp(t *testing.T) {
	s := newScheme(t)
	now := metav1.Now()
	ing := newIngress(func(i *networkingv1.Ingress) {
		i.DeletionTimestamp = &now
		i.Finalizers = []string{CleanupFinalizer}
	})
	managedSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSvcName,
			Namespace: testNS,
			Labels: map[string]string{
				kubelb.LabelManagedBy: kubelb.LabelControllerName,
			},
		},
	}
	route := &kubelbv1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: testIngUID, Namespace: testCluster},
	}
	src := fake.NewClientBuilder().WithScheme(s).WithObjects(ing, managedSvc).Build()
	lb := fake.NewClientBuilder().WithScheme(s).WithObjects(route).Build()
	loop := newLoop(t, src, lb)

	if _, err := loop.Reconcile(context.Background(), req()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	gotSvc := &corev1.Service{}
	if err := src.Get(context.Background(), types.NamespacedName{Name: testSvcName, Namespace: testNS}, gotSvc); !apierrors.IsNotFound(err) {
		t.Fatalf("expected managed service deleted, got err=%v", err)
	}

	gotRoute := &kubelbv1alpha1.Route{}
	if err := lb.Get(context.Background(), types.NamespacedName{Name: testIngUID, Namespace: testCluster}, gotRoute); !apierrors.IsNotFound(err) {
		t.Fatalf("expected route deleted, got err=%v", err)
	}

	// After finalizer removal the fake client may GC the object; either NotFound or
	// a present object without the cleanup finalizer is acceptable.
	gotIng := &networkingv1.Ingress{}
	err := src.Get(context.Background(), types.NamespacedName{Name: testIngName, Namespace: testNS}, gotIng)
	if err == nil {
		if controllerutil.ContainsFinalizer(gotIng, CleanupFinalizer) {
			t.Fatalf("finalizer not removed: %+v", gotIng.Finalizers)
		}
	} else if !apierrors.IsNotFound(err) {
		t.Fatalf("unexpected err fetching ingress: %v", err)
	}
}

func TestSourceReconciler_OutOfScope_Skipped(t *testing.T) {
	s := newScheme(t)
	cls := "other"
	ing := newIngress(func(i *networkingv1.Ingress) {
		i.Spec.IngressClassName = &cls
	})
	src := fake.NewClientBuilder().WithScheme(s).WithObjects(ing).Build()
	lb := fake.NewClientBuilder().WithScheme(s).Build()
	loop := newLoop(t, src, lb)
	loop.Adapter = &ingressAdapter{useClass: true}

	if _, err := loop.Reconcile(context.Background(), req()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if err := lb.Get(context.Background(), types.NamespacedName{Name: testIngUID, Namespace: testCluster}, &kubelbv1alpha1.Route{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected no route, got err=%v", err)
	}
}

func TestSourceReconciler_FinalizerAdded_OnFirstPass(t *testing.T) {
	s := newScheme(t)
	ing := newIngress()
	src := fake.NewClientBuilder().WithScheme(s).WithObjects(ing).Build()
	lb := fake.NewClientBuilder().WithScheme(s).Build()
	loop := newLoop(t, src, lb)

	// Finalizer must be persisted before any downstream Route work runs;
	// downstream may fail because the LB cluster has no Route yet, so the
	// returned error is intentionally not asserted on.
	_, _ = loop.Reconcile(context.Background(), req())

	got := &networkingv1.Ingress{}
	if err := src.Get(context.Background(), types.NamespacedName{Name: testIngName, Namespace: testNS}, got); err != nil {
		t.Fatalf("get ingress: %v", err)
	}
	if !controllerutil.ContainsFinalizer(got, CleanupFinalizer) {
		t.Fatalf("expected finalizer %q, got %+v", CleanupFinalizer, got.Finalizers)
	}
}

func TestIngressAdapter_ApplyRouteStatus(t *testing.T) {
	a := &ingressAdapter{}
	ing := newIngress()

	raw := []byte(`{"loadBalancer":{"ingress":[{"ip":"1.2.3.4"}]}}`)
	changed, err := a.ApplyRouteStatus(ing, raw)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true on first apply")
	}
	if len(ing.Status.LoadBalancer.Ingress) != 1 || ing.Status.LoadBalancer.Ingress[0].IP != "1.2.3.4" {
		t.Fatalf("unexpected status: %+v", ing.Status)
	}

	changed, err = a.ApplyRouteStatus(ing, raw)
	if err != nil {
		t.Fatalf("apply 2: %v", err)
	}
	if changed {
		t.Fatalf("expected changed=false on identical apply")
	}
}

func TestIngressAdapter_ApplyRouteStatus_BadJSON(t *testing.T) {
	a := &ingressAdapter{}
	ing := newIngress()
	if _, err := a.ApplyRouteStatus(ing, []byte("not-json")); err == nil {
		t.Fatalf("expected error for bad raw bytes")
	}
}
