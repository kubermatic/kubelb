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
	envoycp "k8c.io/kubelb/internal/envoy"
	"k8c.io/kubelb/internal/kubelb"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// The envoy-proxy container carries all tenant data-plane traffic but shipped
// with no requests or limits, so it ran BestEffort — first in line for
// eviction and OOM kill on a node it shares with other proxies.
func TestEnvoyProxyContainerHasResourceDefaults(t *testing.T) {
	reconciler := &EnvoyCPReconciler{Namespace: teardownConfigNamespace, EnvoyServer: newTestEnvoyServer(t)}
	config := &kubelbv1alpha1.Config{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: teardownConfigNamespace},
	}
	tenant := &kubelbv1alpha1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"}}

	template := reconciler.getEnvoyProxyPodSpec(config, "tenant-a", "envoy-tenant-a", "snapshot", tenant)

	envoy := template.Spec.Containers[0]
	if envoy.Name != envoyProxyContainerName {
		t.Fatalf("containers[0] = %q, want %q", envoy.Name, envoyProxyContainerName)
	}
	for _, tc := range []struct {
		kind string
		list corev1.ResourceList
	}{
		{"requests", envoy.Resources.Requests},
		{"limits", envoy.Resources.Limits},
	} {
		if tc.list.Cpu().IsZero() {
			t.Fatalf("envoy-proxy has no CPU %s; the container runs BestEffort", tc.kind)
		}
		if tc.list.Memory().IsZero() {
			t.Fatalf("envoy-proxy has no memory %s; the container runs BestEffort", tc.kind)
		}
	}
}

func TestEnvoyProxyResourceDefaultsAreOverridable(t *testing.T) {
	reconciler := &EnvoyCPReconciler{Namespace: teardownConfigNamespace, EnvoyServer: newTestEnvoyServer(t)}
	custom := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
		Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4")},
	}

	config := &kubelbv1alpha1.Config{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: teardownConfigNamespace},
		Spec: kubelbv1alpha1.ConfigSpec{
			EnvoyProxy: kubelbv1alpha1.EnvoyProxy{Resources: &custom},
		},
	}
	tenant := &kubelbv1alpha1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"}}

	template := reconciler.getEnvoyProxyPodSpec(config, "tenant-a", "envoy-tenant-a", "snapshot", tenant)
	if got := template.Spec.Containers[0].Resources.Limits.Cpu().String(); got != "4" {
		t.Fatalf("Config resources must win over the default, CPU limit = %q", got)
	}

	tenantOverride := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("8")},
	}
	tenant.Spec.EnvoyProxy = &kubelbv1alpha1.TenantEnvoyProxy{Resources: &tenantOverride}
	template = reconciler.getEnvoyProxyPodSpec(config, "tenant-a", "envoy-tenant-a", "snapshot", tenant)
	if got := template.Spec.Containers[0].Resources.Limits.Cpu().String(); got != "8" {
		t.Fatalf("Tenant resources must win over Config, CPU limit = %q", got)
	}
}

func newTestEnvoyServer(t *testing.T) *envoycp.Server {
	t.Helper()
	server, err := envoycp.NewServer("0.0.0.0:8001", false)
	if err != nil {
		t.Fatalf("failed to create envoy server: %v", err)
	}
	return server
}
