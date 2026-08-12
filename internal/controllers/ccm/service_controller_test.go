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
	"errors"
	"testing"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	testServiceName = "app"
	testServiceUID  = "svc-uid-1"
)

// fakeLBManager satisfies ctrl.Manager for reconcile() calls that only need
// its client.
type fakeLBManager struct {
	ctrl.Manager
	client ctrlclient.Client
}

func (m *fakeLBManager) GetClient() ctrlclient.Client {
	return m.client
}

func newTenantService(opts ...func(*corev1.Service)) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceName,
			Namespace:  testNS,
			UID:        testServiceUID,
			Finalizers: []string{CleanupFinalizer},
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Name: "http", Port: 80, NodePort: 30080, Protocol: corev1.ProtocolTCP}},
		},
	}
	for _, o := range opts {
		o(svc)
	}
	return svc
}

func newLoadBalancerMirror(name string) *kubelbv1alpha1.LoadBalancer {
	return &kubelbv1alpha1.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testCluster,
			Labels: map[string]string{
				kubelb.LabelOriginName:      testServiceName,
				kubelb.LabelOriginNamespace: testNS,
				kubelb.LabelTenantName:      testCluster,
			},
		},
		Spec: kubelbv1alpha1.LoadBalancerSpec{Type: corev1.ServiceTypeLoadBalancer},
	}
}

func newServiceReconciler(t *testing.T, svc *corev1.Service, mirrors ...*kubelbv1alpha1.LoadBalancer) (*KubeLBServiceReconciler, ctrlclient.Client, ctrlclient.Client) {
	t.Helper()
	s := newScheme(t)
	srcBuilder := fake.NewClientBuilder().WithScheme(s)
	if svc != nil {
		srcBuilder = srcBuilder.WithObjects(svc)
	}
	src := srcBuilder.Build()

	lbObjects := make([]ctrlclient.Object, 0, len(mirrors))
	for _, m := range mirrors {
		lbObjects = append(lbObjects, m)
	}
	lb := fake.NewClientBuilder().WithScheme(s).WithObjects(lbObjects...).Build()

	return &KubeLBServiceReconciler{
		Client:        src,
		KubeLBManager: &fakeLBManager{client: lb},
		Log:           logr.Discard(),
		ClusterName:   testCluster,
	}, src, lb
}

func serviceRequest() ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: testServiceName, Namespace: testNS}}
}

func assertMirrorGone(t *testing.T, lb ctrlclient.Client, name, msg string) {
	t.Helper()
	err := lb.Get(context.Background(), types.NamespacedName{Name: name, Namespace: testCluster}, &kubelbv1alpha1.LoadBalancer{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("%s: got err=%v", msg, err)
	}
}

// A tenant Service downgraded from LoadBalancer to ClusterIP no longer qualifies
// for KubeLB, so its management-cluster mirror (and the billed cloud LB behind it)
// must be torn down; leaving it behind leaks provider quota and keeps a public IP
// pointed at a NodePort that can later be reassigned to an unrelated Service.
func TestServiceReconcile_TypeDowngradeTearsDownMirror(t *testing.T) {
	svc := newTenantService(func(s *corev1.Service) { s.Spec.Type = corev1.ServiceTypeClusterIP })
	r, src, lb := newServiceReconciler(t, svc, newLoadBalancerMirror(testServiceUID))

	if _, err := r.Reconcile(context.Background(), serviceRequest()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	assertMirrorGone(t, lb, testServiceUID, "expected LoadBalancer mirror deleted after type downgrade")

	got := &corev1.Service{}
	if err := src.Get(context.Background(), serviceRequest().NamespacedName, got); err != nil {
		t.Fatalf("get service: %v", err)
	}
	if controllerutil.ContainsFinalizer(got, CleanupFinalizer) {
		t.Fatalf("cleanup finalizer not removed: %+v", got.Finalizers)
	}
}

// The origin Service vanished without the finalizer ever running (tenant cluster
// rebuilt, etcd restore, finalizers force-removed). Nothing else reaps the mirror.
func TestServiceReconcile_OriginGoneReapsMirror(t *testing.T) {
	r, _, lb := newServiceReconciler(t, nil, newLoadBalancerMirror(testServiceUID))

	if _, err := r.Reconcile(context.Background(), serviceRequest()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	assertMirrorGone(t, lb, testServiceUID, "expected orphaned LoadBalancer mirror deleted")
}

// Tenant cluster rebuilt: the Service comes back under the same name/namespace with
// a fresh UID, so the CCM creates a second mirror while the old one keeps its own
// billed cloud LB alive forever.
func TestServiceReconcile_RecreatedOriginReapsStaleUIDMirror(t *testing.T) {
	svc := newTenantService(func(s *corev1.Service) { s.UID = "svc-uid-2" })
	r, _, lb := newServiceReconciler(t, svc, newLoadBalancerMirror(testServiceUID), newLoadBalancerMirror("svc-uid-2"))

	if _, err := r.Reconcile(context.Background(), serviceRequest()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	assertMirrorGone(t, lb, testServiceUID, "expected stale-UID LoadBalancer mirror deleted")
	if err := lb.Get(context.Background(), types.NamespacedName{Name: "svc-uid-2", Namespace: testCluster}, &kubelbv1alpha1.LoadBalancer{}); err != nil {
		t.Fatalf("live mirror must be kept: %v", err)
	}
}

// A transient read failure says nothing about the Service's existence. It must be
// requeued via a returned error, and it must NOT reap the mirror - deleting a live
// mirror (and the cloud LB behind it) on an API hiccup would be far worse than the
// orphan the reap exists to prevent.
func TestServiceReconcile_TransientGetErrorDoesNotReap(t *testing.T) {
	s := newScheme(t)
	failing := interceptor.NewClient(
		fake.NewClientBuilder().WithScheme(s).Build(),
		interceptor.Funcs{
			Get: func(ctx context.Context, client ctrlclient.WithWatch, key ctrlclient.ObjectKey, obj ctrlclient.Object, opts ...ctrlclient.GetOption) error {
				return apierrors.NewInternalError(errors.New("etcdserver: leader changed"))
			},
		},
	)
	lb := fake.NewClientBuilder().WithScheme(s).WithObjects(newLoadBalancerMirror(testServiceUID)).Build()
	r := &KubeLBServiceReconciler{
		Client:        failing,
		KubeLBManager: &fakeLBManager{client: lb},
		Log:           logr.Discard(),
		ClusterName:   testCluster,
	}

	if _, err := r.Reconcile(context.Background(), serviceRequest()); err == nil {
		t.Fatal("expected the transient Get error to be returned for requeue, got nil")
	}

	if err := lb.Get(context.Background(), types.NamespacedName{Name: testServiceUID, Namespace: testCluster}, &kubelbv1alpha1.LoadBalancer{}); err != nil {
		t.Fatalf("mirror must survive a transient Get error, got: %v", err)
	}
}
