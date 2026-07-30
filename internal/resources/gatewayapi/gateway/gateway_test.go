//go:build !e2e

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

package gateway

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const tenantNamespace = "tenant-primary"

func gatewayScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := gwapiv1.Install(s); err != nil {
		t.Fatalf("gateway api scheme: %v", err)
	}
	return s
}

// sourceGateway builds a Gateway as it arrives from the tenant cluster, still
// carrying its original namespace.
func sourceGateway(originNamespace string) *gwapiv1.Gateway {
	return &gwapiv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ParentGatewayName,
			Namespace: originNamespace,
		},
		Spec: gwapiv1.GatewaySpec{
			GatewayClassName: GatewayClassName,
			Listeners: []gwapiv1.Listener{{
				Name:     "http",
				Protocol: gwapiv1.HTTPProtocolType,
				Port:     80,
			}},
		},
	}
}

func applyGateway(t *testing.T, client ctrlclient.Client, originNamespace string) error {
	t.Helper()
	return CreateOrUpdateGateway(context.Background(), logr.Discard(), client, sourceGateway(originNamespace),
		tenantNamespace, &kubelbv1alpha1.Config{}, &kubelbv1alpha1.Tenant{}, kubelbv1alpha1.AnnotationSettings{})
}

// Renaming the Gateway would rotate the load balancer IP, so the name must stay
// stable across reconciles.
func TestCreateOrUpdateGateway_PreservesName(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(gatewayScheme(t)).Build()

	if err := applyGateway(t, client, "default"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gateway := &gwapiv1.Gateway{}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: tenantNamespace, Name: ParentGatewayName}, gateway); err != nil {
		t.Fatalf("Gateway should be created as %s/%s: %v", tenantNamespace, ParentGatewayName, err)
	}
	if got := gateway.Labels[kubelb.LabelOriginNamespace]; got != "default" {
		t.Errorf("origin namespace label = %q, want %q", got, "default")
	}
}

func TestCreateOrUpdateGateway_SameOriginIsIdempotent(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(gatewayScheme(t)).Build()

	if err := applyGateway(t, client, "default"); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if err := applyGateway(t, client, "default"); err != nil {
		t.Fatalf("second apply from the same origin must not conflict: %v", err)
	}

	gateways := &gwapiv1.GatewayList{}
	if err := client.List(context.Background(), gateways); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(gateways.Items) != 1 {
		t.Errorf("got %d Gateways, want 1", len(gateways.Items))
	}
}

// Previously the second claimant overwrote the first, leaving both Routes fighting
// over one Gateway.
func TestCreateOrUpdateGateway_RejectsConflictingOrigin(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(gatewayScheme(t)).Build()

	if err := applyGateway(t, client, "team-a"); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	err := applyGateway(t, client, "team-b")
	if err == nil {
		t.Fatal("expected a conflict error for a second origin namespace, got nil")
	}
	if !strings.Contains(err.Error(), "team-a") {
		t.Errorf("error should name the owning namespace, got: %v", err)
	}
	if !strings.Contains(err.Error(), ParentGatewayName) {
		t.Errorf("error should name the owning object, got: %v", err)
	}

	// The winner keeps the object untouched, so its load balancer IP is unaffected.
	gateway := &gwapiv1.Gateway{}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: tenantNamespace, Name: ParentGatewayName}, gateway); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := gateway.Labels[kubelb.LabelOriginNamespace]; got != "team-a" {
		t.Errorf("origin namespace label = %q, want the first claimant %q", got, "team-a")
	}
}

// Only reachable once an edition derives the management-cluster name from something
// other than the source name, which is why the guard matches the full origin.
func TestCreateOrUpdateGateway_RejectsConflictingOriginName(t *testing.T) {
	existing := &gwapiv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ParentGatewayName,
			Namespace: tenantNamespace,
			Labels: map[string]string{
				kubelb.LabelOriginName:      "other-gateway",
				kubelb.LabelOriginNamespace: "default",
			},
		},
		Spec: gwapiv1.GatewaySpec{GatewayClassName: GatewayClassName},
	}
	client := fake.NewClientBuilder().WithScheme(gatewayScheme(t)).WithObjects(existing).Build()

	err := applyGateway(t, client, "default")
	if err == nil {
		t.Fatal("expected a conflict error for a different origin name, got nil")
	}
	if !strings.Contains(err.Error(), "other-gateway") {
		t.Errorf("error should name the owning object, got: %v", err)
	}
}

// Adoption rather than rejection, so upgrades do not break existing tenants.
func TestCreateOrUpdateGateway_AdoptsUnlabelledGateway(t *testing.T) {
	existing := &gwapiv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ParentGatewayName,
			Namespace: tenantNamespace,
		},
		Spec: gwapiv1.GatewaySpec{GatewayClassName: GatewayClassName},
	}
	client := fake.NewClientBuilder().WithScheme(gatewayScheme(t)).WithObjects(existing).Build()

	if err := applyGateway(t, client, "default"); err != nil {
		t.Fatalf("an unlabelled Gateway should be adopted, got: %v", err)
	}

	gateway := &gwapiv1.Gateway{}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: tenantNamespace, Name: ParentGatewayName}, gateway); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := gateway.Labels[kubelb.LabelOriginNamespace]; got != "default" {
		t.Errorf("origin namespace label = %q, want %q", got, "default")
	}
}
