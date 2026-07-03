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
	"testing"

	"k8c.io/kubelb/internal/resources/reconciling"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const testGatewayCRDName = "gateways.gateway.networking.k8s.io"

type testCRDReconcilerFactory func(*apiextensionsv1.CustomResourceDefinition) reconciling.NamedCustomResourceDefinitionReconcilerFactory

func TestGatewayCRDReconcilerAddsKubeLBManagedByLabel(t *testing.T) {
	tests := []struct {
		name    string
		factory testCRDReconcilerFactory
	}{
		{
			name: "controller reconcile path",
			factory: func(source *apiextensionsv1.CustomResourceDefinition) reconciling.NamedCustomResourceDefinitionReconcilerFactory {
				return (&GatewayCRDReconciler{}).crdReconciler(testGatewayCRDName, source)
			},
		},
		{
			name: "initial install path",
			factory: func(source *apiextensionsv1.CustomResourceDefinition) reconciling.NamedCustomResourceDefinitionReconcilerFactory {
				return crdReconcilerFactory(testGatewayCRDName, source)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"gateway.networking.k8s.io/policy": "standard",
					},
					Annotations: map[string]string{
						"gateway.networking.k8s.io/channel": "standard",
					},
				},
			}

			name, reconciler := tt.factory(source)()
			if name != testGatewayCRDName {
				t.Fatalf("expected name %q, got %q", testGatewayCRDName, name)
			}

			got, err := reconciler(&apiextensionsv1.CustomResourceDefinition{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Labels["gateway.networking.k8s.io/policy"] != "standard" {
				t.Fatalf("expected upstream label to be preserved, got labels=%v", got.Labels)
			}
			if got.Labels["kubelb.k8c.io/managed-by"] != "kubelb" {
				t.Fatalf("expected KubeLB managed-by label, got labels=%v", got.Labels)
			}
			if got.Annotations["gateway.networking.k8s.io/channel"] != "standard" {
				t.Fatalf("expected upstream annotation to be preserved, got annotations=%v", got.Annotations)
			}
		})
	}
}
