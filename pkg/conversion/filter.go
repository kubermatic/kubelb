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

package conversion

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ShouldSkip returns true if the Ingress should be skipped from conversion.
// Ingresses are skipped if:
// - They have kubelb.k8c.io/skip-conversion=true annotation OR label
// - They are canary ingresses (nginx.ingress.kubernetes.io/canary=true)
func ShouldSkip(ing *networkingv1.Ingress) bool {
	// Check labels first (more user-friendly since kubectl label is common)
	if ing.Labels != nil {
		if ing.Labels[AnnotationSkipConversion] == BoolTrue {
			return true
		}
	}
	// Check annotations
	if ing.Annotations != nil {
		// Skip if explicitly marked
		if ing.Annotations[AnnotationSkipConversion] == BoolTrue {
			return true
		}
		// Skip canary ingresses
		if ing.Annotations[NginxCanary] == BoolTrue {
			return true
		}
	}
	return false
}

// MatchesIngressClass checks if an Ingress matches the specified class.
// It checks spec.ingressClassName first, then falls back to the legacy
// kubernetes.io/ingress.class annotation.
func MatchesIngressClass(ing *networkingv1.Ingress, class string) bool {
	// Check spec.ingressClassName first
	if ing.Spec.IngressClassName != nil {
		return *ing.Spec.IngressClassName == class
	}
	// Fallback to legacy annotation
	if ing.Annotations != nil {
		if v, ok := ing.Annotations[AnnotationIngressClass]; ok {
			return v == class
		}
	}
	return false
}

// FetchServicesForIngress fetches Services referenced by the Ingress for port resolution.
// Returns a map of NamespacedName -> Service.
func FetchServicesForIngress(ctx context.Context, k8sClient client.Client, ing *networkingv1.Ingress) (map[types.NamespacedName]*corev1.Service, error) {
	services := make(map[types.NamespacedName]*corev1.Service)

	// Collect service names from Ingress
	serviceNames := make(map[string]struct{})
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		serviceNames[ing.Spec.DefaultBackend.Service.Name] = struct{}{}
	}
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				serviceNames[path.Backend.Service.Name] = struct{}{}
			}
		}
	}

	// Fetch services
	for name := range serviceNames {
		key := types.NamespacedName{Namespace: ing.Namespace, Name: name}
		var svc corev1.Service
		if err := k8sClient.Get(ctx, key, &svc); err != nil {
			// Non-fatal - conversion can proceed with warnings
			continue
		}
		services[key] = &svc
	}

	return services, nil
}
