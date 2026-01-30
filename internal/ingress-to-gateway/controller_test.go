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

package ingressconversion

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestShouldConvert(t *testing.T) {
	tests := []struct {
		name         string
		ingress      *networkingv1.Ingress
		ingressClass string
		want         bool
	}{
		{
			name: "basic ingress, no filter",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
			ingressClass: "",
			want:         true,
		},
		{
			name: "skip-conversion annotation",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						AnnotationSkipConversion: "true",
					},
				},
			},
			ingressClass: "",
			want:         false,
		},
		{
			name: "canary ingress",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						NginxCanary: "true",
					},
				},
			},
			ingressClass: "",
			want:         false,
		},
		{
			name: "spec.ingressClassName matches filter",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("nginx"),
				},
			},
			ingressClass: "nginx",
			want:         true,
		},
		{
			name: "spec.ingressClassName does not match filter",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
				},
			},
			ingressClass: "nginx",
			want:         false,
		},
		{
			name: "annotation ingressClass matches filter",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						AnnotationIngressClass: "nginx",
					},
				},
			},
			ingressClass: "nginx",
			want:         true,
		},
		{
			name: "annotation ingressClass does not match filter",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						AnnotationIngressClass: "traefik",
					},
				},
			},
			ingressClass: "nginx",
			want:         false,
		},
		{
			name: "no class specified, filter set",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
			ingressClass: "nginx",
			want:         false,
		},
		{
			name: "spec.ingressClassName takes precedence over annotation",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						AnnotationIngressClass: "traefik",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("nginx"),
				},
			},
			ingressClass: "nginx",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{
				IngressClass: tt.ingressClass,
			}
			got := r.shouldConvert(tt.ingress)
			if got != tt.want {
				t.Errorf("shouldConvert() = %v, want %v", got, tt.want)
			}
		})
	}
}
