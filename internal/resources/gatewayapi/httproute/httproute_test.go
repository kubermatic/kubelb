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

package httproute

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

func scheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := gwapiv1.Install(s); err != nil {
		t.Fatalf("gateway api scheme: %v", err)
	}
	return s
}

func referencedServices() []metav1.ObjectMeta {
	return []metav1.ObjectMeta{
		{Name: backendName, Namespace: originNamespace},
		{Name: mirrorName, Namespace: originNamespace},
	}
}

// routeWithBackendRefFilter hangs the RequestMirror filter off the backendRef and
// leaves the rule itself without filters, which is what made the old code index a
// zero-length slice.
func routeWithBackendRefFilter() *gwapiv1.HTTPRoute {
	return &gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: routeName, Namespace: originNamespace},
		Spec: gwapiv1.HTTPRouteSpec{
			Rules: []gwapiv1.HTTPRouteRule{{
				BackendRefs: []gwapiv1.HTTPBackendRef{{
					BackendRef: gwapiv1.BackendRef{
						BackendObjectReference: gwapiv1.BackendObjectReference{Name: backendName},
					},
					Filters: []gwapiv1.HTTPRouteFilter{{
						Type: gwapiv1.HTTPRouteFilterRequestMirror,
						RequestMirror: &gwapiv1.HTTPRequestMirrorFilter{
							BackendRef: gwapiv1.BackendObjectReference{Name: mirrorName},
						},
					}},
				}},
			}},
		},
	}
}

// The old code indexed the rule-level filter slice with the backendRef index, so it
// missed this rewrite and panicked when the rule had fewer filters than backendRefs.
func TestCreateOrUpdateHTTPRoute_RewritesBackendRefFilterMirror(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(scheme(t)).Build()
	object := routeWithBackendRefFilter()

	if err := CreateOrUpdateHTTPRoute(context.Background(), logr.Discard(), client, object,
		referencedServices(), tenantNamespace, routeName, &kubelbv1alpha1.Tenant{}, kubelbv1alpha1.AnnotationSettings{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	applied := &gwapiv1.HTTPRoute{}
	key := types.NamespacedName{Namespace: tenantNamespace, Name: kubelb.GenerateName(routeName, originNamespace)}
	if err := client.Get(context.Background(), key, applied); err != nil {
		t.Fatalf("get %s: %v", key, err)
	}

	backendRef := applied.Spec.Rules[0].BackendRefs[0]
	wantBackend := gwapiv1.ObjectName(kubelb.GenerateRouteServiceName(routeName, backendName, originNamespace))
	if backendRef.Name != wantBackend {
		t.Errorf("backendRef name = %q, want %q", backendRef.Name, wantBackend)
	}

	gotMirror := backendRef.Filters[0].RequestMirror.BackendRef
	wantMirror := gwapiv1.ObjectName(kubelb.GenerateRouteServiceName(routeName, mirrorName, originNamespace))
	if gotMirror.Name != wantMirror {
		t.Errorf("mirror backendRef name = %q, want %q", gotMirror.Name, wantMirror)
	}
	if gotMirror.Namespace != nil {
		t.Errorf("mirror backendRef namespace = %q, want nil", *gotMirror.Namespace)
	}
}

// Guards against the backendRef-level rewrite clobbering this one.
func TestCreateOrUpdateHTTPRoute_RewritesRuleLevelMirror(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(scheme(t)).Build()
	object := &gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: routeName, Namespace: originNamespace},
		Spec: gwapiv1.HTTPRouteSpec{
			Rules: []gwapiv1.HTTPRouteRule{{
				Filters: []gwapiv1.HTTPRouteFilter{{
					Type: gwapiv1.HTTPRouteFilterRequestMirror,
					RequestMirror: &gwapiv1.HTTPRequestMirrorFilter{
						BackendRef: gwapiv1.BackendObjectReference{Name: mirrorName},
					},
				}},
				BackendRefs: []gwapiv1.HTTPBackendRef{{
					BackendRef: gwapiv1.BackendRef{
						BackendObjectReference: gwapiv1.BackendObjectReference{Name: backendName},
					},
				}},
			}},
		},
	}

	if err := CreateOrUpdateHTTPRoute(context.Background(), logr.Discard(), client, object,
		referencedServices(), tenantNamespace, routeName, &kubelbv1alpha1.Tenant{}, kubelbv1alpha1.AnnotationSettings{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	applied := &gwapiv1.HTTPRoute{}
	key := types.NamespacedName{Namespace: tenantNamespace, Name: kubelb.GenerateName(routeName, originNamespace)}
	if err := client.Get(context.Background(), key, applied); err != nil {
		t.Fatalf("get %s: %v", key, err)
	}

	got := applied.Spec.Rules[0].Filters[0].RequestMirror.BackendRef
	want := gwapiv1.ObjectName(kubelb.GenerateRouteServiceName(routeName, mirrorName, originNamespace))
	if got.Name != want {
		t.Errorf("rule filter mirror name = %q, want %q", got.Name, want)
	}
}
