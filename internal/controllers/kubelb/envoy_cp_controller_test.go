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

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"

	"sigs.k8s.io/controller-runtime/pkg/event"
)

// The xDS SnapshotCache is keyed by tenant namespace and holds an entry for the
// process lifetime. Tenant deletion must reconcile so that entry is cleared,
// otherwise it survives until the manager restarts.
func TestTenantSpecChangedPredicateTriggersOnDelete(t *testing.T) {
	if !tenantSpecChangedPredicate().Delete(event.DeleteEvent{Object: &kubelbv1alpha1.Tenant{}}) {
		t.Fatal("tenant deletion must enqueue a reconcile so the tenant's snapshot is cleared")
	}
}

// TestEnvoyProxyAnnotationsCarryResourceNamingVersion pins the pod template
// annotation that rolls every envoy proxy exactly once when the generated xDS
// resource names change. Without the roll, a running proxy sees a rename as new
// listeners added plus the old ones removed, and the removed listeners drain
// with their routes pointing at deleted clusters.
func TestEnvoyProxyAnnotationsCarryResourceNamingVersion(t *testing.T) {
	podMonitorConfig := &kubelbv1alpha1.Config{}
	podMonitorConfig.Spec.EnvoyProxy.PodMonitor = &kubelbv1alpha1.EnvoyProxyPodMonitor{Enabled: true}

	tests := []struct {
		name   string
		config *kubelbv1alpha1.Config
	}{
		{name: "prometheus scrape annotations", config: &kubelbv1alpha1.Config{}},
		{name: "pod monitor enabled", config: podMonitorConfig},
	}

	r := &EnvoyCPReconciler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.envoyProxyAnnotations(tt.config)[kubelb.AnnotationResourceNamingVersion]
			if got != kubelb.ResourceNamingVersion {
				t.Errorf("annotation %s = %q, want %q", kubelb.AnnotationResourceNamingVersion, got, kubelb.ResourceNamingVersion)
			}
		})
	}
}
