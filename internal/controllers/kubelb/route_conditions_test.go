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
	"fmt"
	"reflect"
	"testing"
	"time"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	kubelbconst "k8c.io/kubelb/internal/kubelb"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func testService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "default",
			UID:       types.UID("test-uid"),
			Labels: map[string]string{
				kubelbconst.LabelOriginName:      "test-svc",
				kubelbconst.LabelOriginNamespace: "default",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
		},
	}
}

func testHTTPRoute() *gwapiv1.HTTPRoute {
	route := &gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "default",
			Labels: map[string]string{
				kubelbconst.LabelOriginName:      "test-route",
				kubelbconst.LabelOriginNamespace: "default",
			},
		},
	}
	route.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "HTTPRoute",
	})
	return route
}

func TestSetCondition_NilSlice_CreatesCondition(t *testing.T) {
	var conditions []metav1.Condition
	setCondition(&conditions, nil)

	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conditions))
	}
	c := conditions[0]
	if c.Status != metav1.ConditionTrue {
		t.Errorf("expected ConditionTrue, got %s", c.Status)
	}
	if c.Reason != "InstallationSuccessful" {
		t.Errorf("expected InstallationSuccessful, got %s", c.Reason)
	}
	if c.LastTransitionTime.IsZero() {
		t.Error("expected non-zero LastTransitionTime")
	}
}

func TestSetCondition_SameStatus_PreservesTimestamp(t *testing.T) {
	pastTime := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	conditions := []metav1.Condition{
		{
			Type:               kubelbv1alpha1.ConditionResourceAppliedSuccessfully.String(),
			Status:             metav1.ConditionTrue,
			Reason:             "InstallationSuccessful",
			Message:            "Success",
			LastTransitionTime: pastTime,
		},
	}

	setCondition(&conditions, nil)

	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conditions))
	}
	if !conditions[0].LastTransitionTime.Equal(&pastTime) {
		t.Errorf("expected timestamp preserved at %v, got %v", pastTime, conditions[0].LastTransitionTime)
	}
}

func TestSetCondition_StatusTransition_UpdatesTimestamp(t *testing.T) {
	pastTime := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	conditions := []metav1.Condition{
		{
			Type:               kubelbv1alpha1.ConditionResourceAppliedSuccessfully.String(),
			Status:             metav1.ConditionTrue,
			Reason:             "InstallationSuccessful",
			Message:            "Success",
			LastTransitionTime: pastTime,
		},
	}

	setCondition(&conditions, fmt.Errorf("something broke"))

	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conditions))
	}
	c := conditions[0]
	if c.Status != metav1.ConditionFalse {
		t.Errorf("expected ConditionFalse, got %s", c.Status)
	}
	if c.LastTransitionTime.Equal(&pastTime) {
		t.Error("expected timestamp to be updated on status transition")
	}
}

func TestUpdateServiceStatus_Idempotent(t *testing.T) {
	routeStatus := &kubelbv1alpha1.RouteStatus{}
	routeStatus.Resources.Services = make(map[string]kubelbv1alpha1.RouteServiceStatus)

	svc := testService()

	updateServiceStatus(routeStatus, svc, nil)
	first := routeStatus.DeepCopy()

	updateServiceStatus(routeStatus, svc, nil)

	if !reflect.DeepEqual(first, routeStatus) {
		t.Error("expected identical RouteStatus after two calls with same input")
	}
}

func TestUpdateResourceStatus_Idempotent(t *testing.T) {
	routeStatus := &kubelbv1alpha1.RouteStatus{}

	obj := testHTTPRoute()

	updateResourceStatus(routeStatus, obj, nil)
	first := routeStatus.DeepCopy()

	updateResourceStatus(routeStatus, obj, nil)

	if !reflect.DeepEqual(first, routeStatus) {
		t.Error("expected identical RouteStatus after two calls with same input")
	}
}
