//go:build !e2e

/*
Copyright 2024 The KubeLB Authors.

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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestShouldReconcileResource_Gateway(t *testing.T) {
	tests := []struct {
		name            string
		gateway         *gwapiv1.Gateway
		useGatewayClass bool
		want            bool
	}{
		{
			name: "gateway named kubelb with correct class",
			gateway: &gwapiv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "kubelb"},
				Spec:       gwapiv1.GatewaySpec{GatewayClassName: "kubelb"},
			},
			useGatewayClass: true,
			want:            true,
		},
		{
			name: "gateway named kubelb without class check",
			gateway: &gwapiv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "kubelb"},
				Spec:       gwapiv1.GatewaySpec{GatewayClassName: "other"},
			},
			useGatewayClass: false,
			want:            true,
		},
		{
			name: "gateway named kubelb with wrong class",
			gateway: &gwapiv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "kubelb"},
				Spec:       gwapiv1.GatewaySpec{GatewayClassName: "other"},
			},
			useGatewayClass: true,
			want:            false,
		},
		{
			name: "gateway with different name rejected",
			gateway: &gwapiv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "my-gateway"},
				Spec:       gwapiv1.GatewaySpec{GatewayClassName: "kubelb"},
			},
			useGatewayClass: true,
			want:            false,
		},
		{
			name: "gateway with different name rejected even without class check",
			gateway: &gwapiv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "test-gw"},
				Spec:       gwapiv1.GatewaySpec{GatewayClassName: "kubelb"},
			},
			useGatewayClass: false,
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldReconcileResource(tt.gateway, tt.useGatewayClass)
			if got != tt.want {
				t.Errorf("ShouldReconcileResource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldReconcileResource_HTTPRoute(t *testing.T) {
	tests := []struct {
		name      string
		httpRoute *gwapiv1.HTTPRoute
		want      bool
	}{
		{
			name: "route referencing kubelb gateway",
			httpRoute: &gwapiv1.HTTPRoute{
				Spec: gwapiv1.HTTPRouteSpec{
					CommonRouteSpec: gwapiv1.CommonRouteSpec{
						ParentRefs: []gwapiv1.ParentReference{
							{Name: "kubelb"},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "route referencing different gateway rejected",
			httpRoute: &gwapiv1.HTTPRoute{
				Spec: gwapiv1.HTTPRouteSpec{
					CommonRouteSpec: gwapiv1.CommonRouteSpec{
						ParentRefs: []gwapiv1.ParentReference{
							{Name: "my-gateway"},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "route with multiple parentRefs rejected",
			httpRoute: &gwapiv1.HTTPRoute{
				Spec: gwapiv1.HTTPRouteSpec{
					CommonRouteSpec: gwapiv1.CommonRouteSpec{
						ParentRefs: []gwapiv1.ParentReference{
							{Name: "kubelb"},
							{Name: "other"},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "route with no parentRefs rejected",
			httpRoute: &gwapiv1.HTTPRoute{
				Spec: gwapiv1.HTTPRouteSpec{
					CommonRouteSpec: gwapiv1.CommonRouteSpec{
						ParentRefs: []gwapiv1.ParentReference{},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldReconcileResource(tt.httpRoute, false)
			if got != tt.want {
				t.Errorf("ShouldReconcileResource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldReconcileResource_GRPCRoute(t *testing.T) {
	tests := []struct {
		name      string
		grpcRoute *gwapiv1.GRPCRoute
		want      bool
	}{
		{
			name: "route referencing kubelb gateway",
			grpcRoute: &gwapiv1.GRPCRoute{
				Spec: gwapiv1.GRPCRouteSpec{
					CommonRouteSpec: gwapiv1.CommonRouteSpec{
						ParentRefs: []gwapiv1.ParentReference{
							{Name: "kubelb"},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "route referencing different gateway rejected",
			grpcRoute: &gwapiv1.GRPCRoute{
				Spec: gwapiv1.GRPCRouteSpec{
					CommonRouteSpec: gwapiv1.CommonRouteSpec{
						ParentRefs: []gwapiv1.ParentReference{
							{Name: "my-gateway"},
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldReconcileResource(tt.grpcRoute, false)
			if got != tt.want {
				t.Errorf("ShouldReconcileResource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMaxGatewaysPerTenant(t *testing.T) {
	if MaxGatewaysPerTenant != 1 {
		t.Errorf("MaxGatewaysPerTenant = %d, want 1 for CE build", MaxGatewaysPerTenant)
	}
}

func TestShouldReconcileGatewayByName(t *testing.T) {
	tests := []struct {
		name    string
		gateway *gwapiv1.Gateway
		want    bool
	}{
		{
			name: "gateway named kubelb",
			gateway: &gwapiv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "kubelb"},
			},
			want: true,
		},
		{
			name: "gateway with different name rejected",
			gateway: &gwapiv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "my-gateway"},
			},
			want: false,
		},
		{
			name: "gateway with test name rejected",
			gateway: &gwapiv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "test-gw"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldReconcileGatewayByName(tt.gateway)
			if got != tt.want {
				t.Errorf("ShouldReconcileGatewayByName() = %v, want %v", got, tt.want)
			}
		})
	}
}
