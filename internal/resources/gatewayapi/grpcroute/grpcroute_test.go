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

package grpcroute

import (
	"context"
	"testing"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	tenantNamespace = "tenant-primary"
	originNamespace = "default"
	routeName       = "demo"
	backendName     = "backend"
	mirrorName      = "mirror"
)

// The old code indexed the rule-level filter slice with the backendRef index, so it
// missed this rewrite and panicked when the rule itself had no filters.
func TestCreateOrUpdateGRPCRoute_RewritesBackendRefFilterMirror(t *testing.T) {
	s := runtime.NewScheme()
	if err := gwapiv1.Install(s); err != nil {
		t.Fatalf("gateway api scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(s).Build()

	object := &gwapiv1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{Name: routeName, Namespace: originNamespace},
		Spec: gwapiv1.GRPCRouteSpec{
			Rules: []gwapiv1.GRPCRouteRule{{
				BackendRefs: []gwapiv1.GRPCBackendRef{{
					BackendRef: gwapiv1.BackendRef{
						BackendObjectReference: gwapiv1.BackendObjectReference{Name: backendName},
					},
					Filters: []gwapiv1.GRPCRouteFilter{{
						Type: gwapiv1.GRPCRouteFilterRequestMirror,
						RequestMirror: &gwapiv1.HTTPRequestMirrorFilter{
							BackendRef: gwapiv1.BackendObjectReference{Name: mirrorName},
						},
					}},
				}},
			}},
		},
	}

	referencedServices := []metav1.ObjectMeta{
		{Name: backendName, Namespace: originNamespace},
		{Name: mirrorName, Namespace: originNamespace},
	}

	if err := CreateOrUpdateGRPCRoute(context.Background(), logr.Discard(), client, object,
		referencedServices, tenantNamespace, routeName, &kubelbv1alpha1.Tenant{}, kubelbv1alpha1.AnnotationSettings{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	applied := &gwapiv1.GRPCRoute{}
	key := types.NamespacedName{Namespace: tenantNamespace, Name: kubelb.GenerateName(routeName, originNamespace)}
	if err := client.Get(context.Background(), key, applied); err != nil {
		t.Fatalf("get %s: %v", key, err)
	}

	gotMirror := applied.Spec.Rules[0].BackendRefs[0].Filters[0].RequestMirror.BackendRef
	wantMirror := gwapiv1.ObjectName(kubelb.GenerateRouteServiceName(routeName, mirrorName, originNamespace))
	if gotMirror.Name != wantMirror {
		t.Errorf("mirror backendRef name = %q, want %q", gotMirror.Name, wantMirror)
	}
	if gotMirror.Namespace != nil {
		t.Errorf("mirror backendRef namespace = %q, want nil", *gotMirror.Namespace)
	}
}
