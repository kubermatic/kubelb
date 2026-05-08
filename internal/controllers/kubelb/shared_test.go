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
	"testing"

	"github.com/google/go-cmp/cmp"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
)

func ptrBool(b bool) *bool { return &b }

func ptrMap(m map[string]string) *map[string]string { return &m }

func TestGetAnnotations(t *testing.T) {
	tests := []struct {
		name   string
		tenant kubelbv1alpha1.TenantSpec
		config kubelbv1alpha1.ConfigSpec
		want   kubelbv1alpha1.AnnotationSettings
	}{
		{
			name:   "both empty",
			tenant: kubelbv1alpha1.TenantSpec{},
			config: kubelbv1alpha1.ConfigSpec{},
			want:   kubelbv1alpha1.AnnotationSettings{},
		},
		{
			name: "only config propagated annotations",
			config: kubelbv1alpha1.ConfigSpec{
				AnnotationSettings: kubelbv1alpha1.AnnotationSettings{
					PropagatedAnnotations: ptrMap(map[string]string{"k": ""}),
				},
			},
			want: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{"k": ""}),
			},
		},
		{
			name: "tenant propagated annotations override config",
			config: kubelbv1alpha1.ConfigSpec{
				AnnotationSettings: kubelbv1alpha1.AnnotationSettings{
					PropagatedAnnotations: ptrMap(map[string]string{"config": ""}),
				},
			},
			tenant: kubelbv1alpha1.TenantSpec{
				AnnotationSettings: kubelbv1alpha1.AnnotationSettings{
					PropagatedAnnotations: ptrMap(map[string]string{"tenant": ""}),
				},
			},
			want: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{"tenant": ""}),
			},
		},
		{
			name: "tenant propagate-all overrides config propagated list",
			config: kubelbv1alpha1.ConfigSpec{
				AnnotationSettings: kubelbv1alpha1.AnnotationSettings{
					PropagatedAnnotations: ptrMap(map[string]string{"k": ""}),
				},
			},
			tenant: kubelbv1alpha1.TenantSpec{
				AnnotationSettings: kubelbv1alpha1.AnnotationSettings{
					PropagateAllAnnotations: ptrBool(true),
				},
			},
			want: kubelbv1alpha1.AnnotationSettings{
				PropagateAllAnnotations: ptrBool(true),
			},
		},
		{
			name: "default annotations: tenant fully replaces config",
			config: kubelbv1alpha1.ConfigSpec{
				AnnotationSettings: kubelbv1alpha1.AnnotationSettings{
					DefaultAnnotations: map[kubelbv1alpha1.AnnotatedResource]kubelbv1alpha1.Annotations{
						kubelbv1alpha1.AnnotatedResourceService: {"c": "1"},
					},
				},
			},
			tenant: kubelbv1alpha1.TenantSpec{
				AnnotationSettings: kubelbv1alpha1.AnnotationSettings{
					DefaultAnnotations: map[kubelbv1alpha1.AnnotatedResource]kubelbv1alpha1.Annotations{
						kubelbv1alpha1.AnnotatedResourceService: {"t": "1"},
					},
				},
			},
			want: kubelbv1alpha1.AnnotationSettings{
				DefaultAnnotations: map[kubelbv1alpha1.AnnotatedResource]kubelbv1alpha1.Annotations{
					kubelbv1alpha1.AnnotatedResourceService: {"t": "1"},
				},
			},
		},
		{
			name: "tenant explicit false disables config propagate-all",
			config: kubelbv1alpha1.ConfigSpec{
				AnnotationSettings: kubelbv1alpha1.AnnotationSettings{
					PropagateAllAnnotations: ptrBool(true),
				},
			},
			tenant: kubelbv1alpha1.TenantSpec{
				AnnotationSettings: kubelbv1alpha1.AnnotationSettings{
					PropagateAllAnnotations: ptrBool(false),
				},
			},
			want: kubelbv1alpha1.AnnotationSettings{
				PropagateAllAnnotations: ptrBool(false),
			},
		},
		{
			name: "tenant PropagatedAnnotations replaces config propagate-all",
			config: kubelbv1alpha1.ConfigSpec{
				AnnotationSettings: kubelbv1alpha1.AnnotationSettings{
					PropagateAllAnnotations: ptrBool(true),
				},
			},
			tenant: kubelbv1alpha1.TenantSpec{
				AnnotationSettings: kubelbv1alpha1.AnnotationSettings{
					PropagatedAnnotations: ptrMap(map[string]string{"k": ""}),
				},
			},
			want: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{"k": ""}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant := &kubelbv1alpha1.Tenant{Spec: tt.tenant}
			config := &kubelbv1alpha1.Config{Spec: tt.config}
			got := GetAnnotations(tenant, config)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GetAnnotations mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
