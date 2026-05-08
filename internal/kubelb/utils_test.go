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

func TestPropagateAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		source   map[string]string
		settings kubelbv1alpha1.AnnotationSettings
		resource kubelbv1alpha1.AnnotatedResource
		want     map[string]string
	}{
		{
			name:     "nil source, empty settings, returns empty map",
			source:   nil,
			settings: kubelbv1alpha1.AnnotationSettings{},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{},
		},
		{
			name:   "no allow list, source dropped",
			source: map[string]string{"foo": "bar"},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{}),
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{},
		},
		{
			name:   "propagate-all keeps source unchanged",
			source: map[string]string{"foo": "bar", "baz": "qux"},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagateAllAnnotations: ptrBool(true),
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{"foo": "bar", "baz": "qux"},
		},
		{
			name:   "exact key match, empty value means any value",
			source: map[string]string{"foo": "bar", "baz": "qux"},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{"foo": ""}),
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{"foo": "bar"},
		},
		{
			name:   "key match with comma-separated value list, value matches",
			source: map[string]string{"k": "b"},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{"k": "a,b,c"}),
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{"k": "b"},
		},
		{
			name:   "key match, value list trims whitespace",
			source: map[string]string{"k": "b"},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{"k": " a , b , c "}),
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{"k": "b"},
		},
		{
			name:   "key match but value not in list, dropped",
			source: map[string]string{"k": "z"},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{"k": "a,b,c"}),
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{},
		},
		{
			name:   "default annotations applied when key absent",
			source: map[string]string{},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{}),
				DefaultAnnotations: map[kubelbv1alpha1.AnnotatedResource]kubelbv1alpha1.Annotations{
					kubelbv1alpha1.AnnotatedResourceService: {"d": "1"},
				},
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{"d": "1"},
		},
		{
			name:   "source value wins over default",
			source: map[string]string{"d": "src"},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{"d": ""}),
				DefaultAnnotations: map[kubelbv1alpha1.AnnotatedResource]kubelbv1alpha1.Annotations{
					kubelbv1alpha1.AnnotatedResourceService: {"d": "default"},
				},
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{"d": "src"},
		},
		{
			name:   "all-resource defaults applied",
			source: map[string]string{},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{}),
				DefaultAnnotations: map[kubelbv1alpha1.AnnotatedResource]kubelbv1alpha1.Annotations{
					kubelbv1alpha1.AnnotatedResourceAll: {"a": "1"},
				},
			},
			resource: kubelbv1alpha1.AnnotatedResourceIngress,
			want:     map[string]string{"a": "1"},
		},
		{
			name:   "resource-specific default wins over all",
			source: map[string]string{},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{}),
				DefaultAnnotations: map[kubelbv1alpha1.AnnotatedResource]kubelbv1alpha1.Annotations{
					kubelbv1alpha1.AnnotatedResourceAll:     {"k": "all"},
					kubelbv1alpha1.AnnotatedResourceIngress: {"k": "ingress"},
				},
			},
			resource: kubelbv1alpha1.AnnotatedResourceIngress,
			want:     map[string]string{"k": "ingress"},
		},
		{
			name:   "propagate-all also applies defaults",
			source: map[string]string{"x": "1"},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagateAllAnnotations: ptrBool(true),
				DefaultAnnotations: map[kubelbv1alpha1.AnnotatedResource]kubelbv1alpha1.Annotations{
					kubelbv1alpha1.AnnotatedResourceService: {"d": "1"},
				},
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{"x": "1", "d": "1"},
		},
		{
			name:   "glob in allow key matches whole nginx ingress prefix",
			source: map[string]string{"nginx.ingress.kubernetes.io/rewrite-target": "/", "nginx.ingress.kubernetes.io/ssl-redirect": "true", "other": "x"},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{"nginx.ingress.kubernetes.io/*": ""}),
			},
			resource: kubelbv1alpha1.AnnotatedResourceIngress,
			want: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
				"nginx.ingress.kubernetes.io/ssl-redirect":   "true",
			},
		},
		{
			name:   "deny list drops key under propagate-all",
			source: map[string]string{"keep": "1", "drop.me/foo": "x", "drop.me/bar": "y"},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagateAllAnnotations: ptrBool(true),
				DeniedAnnotations:       []string{"drop.me/*"},
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{"keep": "1"},
		},
		{
			name:   "deny overrides allow",
			source: map[string]string{"a/x": "1", "a/y": "2"},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{"a/*": ""}),
				DeniedAnnotations:     []string{"a/x"},
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{"a/y": "2"},
		},
		{
			name:   "deny is exact when no glob",
			source: map[string]string{"a/x": "1", "a/xx": "2"},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagateAllAnnotations: ptrBool(true),
				DeniedAnnotations:       []string{"a/x"},
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{"a/xx": "2"},
		},
		{
			name:   "glob allow with value list still enforces exact value match",
			source: map[string]string{"x.io/a": "ok", "x.io/b": "bad"},
			settings: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: ptrMap(map[string]string{"x.io/*": "ok"}),
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{"x.io/a": "ok"},
		},
		{
			name:   "deny does not block default annotations",
			source: map[string]string{},
			settings: kubelbv1alpha1.AnnotationSettings{
				DeniedAnnotations: []string{"d"},
				DefaultAnnotations: map[kubelbv1alpha1.AnnotatedResource]kubelbv1alpha1.Annotations{
					kubelbv1alpha1.AnnotatedResourceService: {"d": "1"},
				},
			},
			resource: kubelbv1alpha1.AnnotatedResourceService,
			want:     map[string]string{"d": "1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var srcCopy map[string]string
			if tt.source != nil {
				srcCopy = map[string]string{}
				for k, v := range tt.source {
					srcCopy[k] = v
				}
			}
			got := PropagateAnnotations(tt.source, tt.settings, tt.resource)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("PropagateAnnotations result mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(srcCopy, tt.source); diff != "" {
				t.Errorf("PropagateAnnotations mutated input map (-before +after):\n%s", diff)
			}
		})
	}
}
