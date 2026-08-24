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

	kubelbiov1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// conflictOnceClient rejects the first Update with a conflict, after writing a
// competing change so the caller's copy really is stale. A caller that reuses
// its original copy keeps losing on resourceVersion; only one that re-reads
// before retrying can succeed.
type conflictOnceClient struct {
	ctrlclient.Client
	updates int
}

func (c *conflictOnceClient) Update(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.UpdateOption) error {
	c.updates++
	if c.updates == 1 {
		var stored kubelbiov1alpha1.Addresses
		if err := c.Get(ctx, ctrlclient.ObjectKeyFromObject(obj), &stored); err != nil {
			return err
		}
		stored.Labels = map[string]string{"competing-writer": "true"}
		if err := c.Client.Update(ctx, &stored); err != nil {
			return err
		}
		return kerrors.NewConflict(
			schema.GroupResource{Group: "kubelb.k8c.io", Resource: "addresses"},
			obj.GetName(),
			errors.New("the object has been modified"),
		)
	}
	return c.Client.Update(ctx, obj, opts...)
}

func nodeWithInternalIP(name, ip string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
			Addresses:  []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: ip}},
		},
	}
}

func TestNodeReconcileRequeuesWithoutErrorWhenNodeHasNoAddress(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-without-address"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
		},
	}).Build()
	r := &KubeLBNodeReconciler{
		Client:              client,
		Log:                 logr.Discard(),
		EndpointAddressType: corev1.NodeInternalIP,
	}

	got, err := r.Reconcile(context.Background(), ctrl.Request{})
	if err != nil {
		t.Fatalf("Reconcile() error = %v, want nil for transient missing address", err)
	}
	want := ctrl.Result{RequeueAfter: requeueAfter}
	if got != want {
		t.Fatalf("Reconcile() result = %v, want %v", got, want)
	}
}

// Every CCM in a tenant writes the same Addresses object, so a conflict on
// update is routine. Losing the reconcile to it leaves the tenant's endpoints
// stale until the next node event.
func TestNodeReconcilerRetriesAddressesUpdateOnConflict(t *testing.T) {
	const clusterName = "tenant-a"

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("corev1.AddToScheme() error = %v", err)
	}
	if err := kubelbiov1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("kubelbiov1alpha1.AddToScheme() error = %v", err)
	}

	tenantClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		nodeWithInternalIP("node-1", "10.0.0.1"),
		nodeWithInternalIP("node-2", "10.0.0.2"),
	).Build()

	existing := &kubelbiov1alpha1.Addresses{
		ObjectMeta: metav1.ObjectMeta{Name: kubelbiov1alpha1.DefaultAddressName, Namespace: clusterName},
		Spec: kubelbiov1alpha1.AddressesSpec{
			Addresses: []kubelbiov1alpha1.EndpointAddress{{IP: "192.168.0.1"}},
		},
	}
	kubeLBClient := &conflictOnceClient{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build(),
	}

	r := &KubeLBNodeReconciler{
		Client:              tenantClient,
		KubeLBClient:        kubeLBClient,
		ClusterName:         clusterName,
		Log:                 logr.Discard(),
		Scheme:              scheme,
		EndpointAddressType: corev1.NodeInternalIP,
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil after retrying the conflict", err)
	}

	if kubeLBClient.updates < 2 {
		t.Fatalf("Update called %d times, want at least 2 (the conflict must be retried)", kubeLBClient.updates)
	}

	var got kubelbiov1alpha1.Addresses
	if err := kubeLBClient.Get(context.Background(), types.NamespacedName{
		Name: kubelbiov1alpha1.DefaultAddressName, Namespace: clusterName,
	}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	want := []string{"10.0.0.1", "10.0.0.2"}
	if len(got.Spec.Addresses) != len(want) {
		t.Fatalf("addresses = %+v, want %v", got.Spec.Addresses, want)
	}
	for i, ip := range want {
		if got.Spec.Addresses[i].IP != ip {
			t.Errorf("addresses[%d].IP = %q, want %q", i, got.Spec.Addresses[i].IP, ip)
		}
	}
}
