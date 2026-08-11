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
	"strings"
	"testing"

	"github.com/go-logr/logr"

	gatewayhelper "k8c.io/kubelb/internal/resources/gatewayapi/gateway"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func gatewaySignalScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := gwapiv1.Install(s); err != nil {
		t.Fatalf("failed to build scheme: %v", err)
	}
	return s
}

func gatewayNamed(class string, finalizers ...string) *gwapiv1.Gateway {
	return &gwapiv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:       gatewayhelper.ParentGatewayName,
			Namespace:  "default",
			Finalizers: finalizers,
		},
		Spec: gwapiv1.GatewaySpec{GatewayClassName: gwapiv1.ObjectName(class)},
	}
}

func newGatewaySignalReconciler(t *testing.T, useGatewayClass bool, objects ...ctrlruntimeclient.Object) (*GatewayReconciler, *events.FakeRecorder) {
	t.Helper()
	scheme := gatewaySignalScheme(t)
	recorder := events.NewFakeRecorder(16)

	return &GatewayReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objects...).
			Build(),
		Log:             logr.Discard(),
		Scheme:          scheme,
		Recorder:        recorder,
		UseGatewayClass: useGatewayClass,
	}, recorder
}

func warningFor(t *testing.T, recorder *events.FakeRecorder) string {
	t.Helper()
	select {
	case e := <-recorder.Events:
		return e
	default:
		return ""
	}
}

// The tenant otherwise sees the Gateway API default status,
// Accepted=Unknown/Pending, which is indistinguishable from a dead controller.
func TestWarnGatewayClassNotAcceptedEmitsWarning(t *testing.T) {
	gateway := gatewayNamed("nginx")
	r, recorder := newGatewaySignalReconciler(t, true, gateway)

	r.warnGatewayClassNotAccepted(context.Background(), logr.Discard(), gateway)

	event := warningFor(t, recorder)
	if !strings.Contains(event, EventReasonGatewayClassNotAccepted) {
		t.Errorf("expected a %s event, got %q", EventReasonGatewayClassNotAccepted, event)
	}
	if !strings.Contains(event, "nginx") {
		t.Errorf("expected the event to name the class, got %q", event)
	}
}

// A GatewayClass object that exists designates another controller, and that
// controller owns the Gateway's status. Warning there would be wrong.
func TestWarnGatewayClassNotAcceptedStaysQuietWhenClassIsClaimed(t *testing.T) {
	gateway := gatewayNamed("nginx")
	class := &gwapiv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}}
	r, recorder := newGatewaySignalReconciler(t, true, gateway, class)

	r.warnGatewayClassNotAccepted(context.Background(), logr.Discard(), gateway)

	if event := warningFor(t, recorder); event != "" {
		t.Errorf("expected no event for a claimed class, got %q", event)
	}
}

// A Gateway still carrying the finalizer was demonstrably adopted by KubeLB
// before the class mapping changed, so the warning is KubeLB's to emit even
// though another controller now owns the class.
func TestWarnGatewayClassNotAcceptedWarnsOnAdoptedGateway(t *testing.T) {
	gateway := gatewayNamed("nginx", CleanupFinalizer)
	class := &gwapiv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}}
	r, recorder := newGatewaySignalReconciler(t, true, gateway, class)

	r.warnGatewayClassNotAccepted(context.Background(), logr.Discard(), gateway)

	event := warningFor(t, recorder)
	if !strings.Contains(event, "no longer served") {
		t.Errorf("expected the teardown wording for an adopted Gateway, got %q", event)
	}
}

func TestWarnGatewayClassNotAcceptedIgnoresServedAndForeignGateways(t *testing.T) {
	tests := []struct {
		name            string
		gateway         *gwapiv1.Gateway
		useGatewayClass bool
	}{
		{
			name:            "class is served",
			gateway:         gatewayNamed(gatewayhelper.GatewayClassName),
			useGatewayClass: true,
		},
		{
			name:            "class matching is off",
			gateway:         gatewayNamed("nginx"),
			useGatewayClass: false,
		},
		{
			name: "another name, not KubeLB's to comment on",
			gateway: &gwapiv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "default"},
				Spec:       gwapiv1.GatewaySpec{GatewayClassName: "nginx"},
			},
			useGatewayClass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, recorder := newGatewaySignalReconciler(t, tt.useGatewayClass, tt.gateway)

			r.warnGatewayClassNotAccepted(context.Background(), logr.Discard(), tt.gateway)

			if event := warningFor(t, recorder); event != "" {
				t.Errorf("expected no event, got %q", event)
			}
		})
	}
}

// Reconcile has to see an unserved Gateway to warn about it, but a foreign
// controller rewriting that Gateway's status must not enqueue it.
func TestShouldObserveAdmitsUnservedGatewayNamedKubelb(t *testing.T) {
	r, _ := newGatewaySignalReconciler(t, true)

	unserved := gatewayNamed("nginx")
	if r.shouldReconcile(unserved) {
		t.Error("expected an unserved class to stay out of the reconcile path")
	}
	if !r.shouldObserve(unserved) {
		t.Error("expected an unserved class to be observed")
	}

	foreign := &gwapiv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "default"},
		Spec:       gwapiv1.GatewaySpec{GatewayClassName: "nginx"},
	}
	if r.shouldObserve(foreign) {
		t.Error("expected a Gateway with another name to be ignored entirely")
	}
}
