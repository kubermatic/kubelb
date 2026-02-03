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
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"

	"k8c.io/kubelb/pkg/conversion"

	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// reconcileGateway ensures the Gateway exists and has required listeners
func (r *Reconciler) reconcileGateway(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress, tlsListeners []conversion.TLSListener) error {
	gatewayNs := r.GatewayNamespace
	if gatewayNs == "" {
		gatewayNs = ingress.Namespace
	}

	// Build desired Gateway config
	config := conversion.GatewayConfig{
		Name:         r.GatewayName,
		Namespace:    gatewayNs,
		ClassName:    r.GatewayClassName,
		TLSListeners: tlsListeners,
		Annotations:  r.extractGatewayAnnotations(ingress),
	}

	// Get or create Gateway
	existing := &gwapiv1.Gateway{}
	err := r.Get(ctx, types.NamespacedName{Name: config.Name, Namespace: config.Namespace}, existing)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Create new Gateway
			gateway := conversion.BuildGateway(config)
			log.Info("Creating Gateway", "name", gateway.Name, "namespace", gateway.Namespace)
			return r.Create(ctx, gateway)
		}
		return fmt.Errorf("failed to get Gateway: %w", err)
	}

	// Update existing Gateway
	updated := false

	// Merge listeners
	newListeners := conversion.MergeListeners(existing.Spec.Listeners, conversion.BuildListeners(config.TLSListeners))
	if !listenersEqual(existing.Spec.Listeners, newListeners) {
		existing.Spec.Listeners = newListeners
		updated = true
	}

	// Merge annotations
	if existing.Annotations == nil {
		existing.Annotations = make(map[string]string)
	}
	for k, v := range config.Annotations {
		if existing.Annotations[k] != v {
			existing.Annotations[k] = v
			updated = true
		}
	}

	if updated {
		log.Info("Updating Gateway", "name", existing.Name, "namespace", existing.Namespace)
		return r.Update(ctx, existing)
	}

	return nil
}

// extractGatewayAnnotations extracts annotations to propagate to Gateway
func (r *Reconciler) extractGatewayAnnotations(ingress *networkingv1.Ingress) map[string]string {
	result := make(map[string]string)

	// Add user-specified gateway annotations
	for k, v := range r.GatewayAnnotations {
		result[k] = v
	}

	// external-dns target annotation (only target goes to Gateway)
	if r.PropagateExternalDNS && ingress.Annotations != nil {
		if target, ok := ingress.Annotations[conversion.ExternalDNSTarget]; ok {
			result[conversion.ExternalDNSTarget] = target
		}
	}

	return result
}

// extractHTTPRouteAnnotations extracts external-dns annotations for HTTPRoute (excluding target)
func (r *Reconciler) extractHTTPRouteAnnotations(ingress *networkingv1.Ingress) map[string]string {
	result := make(map[string]string)
	if ingress.Annotations == nil || !r.PropagateExternalDNS {
		return result
	}

	for k, v := range ingress.Annotations {
		if strings.HasPrefix(k, conversion.ExternalDNSAnnotationPrefix) && k != conversion.ExternalDNSTarget {
			result[k] = v
		}
	}

	return result
}

// listenersEqual checks if two listener slices are equivalent.
// Compares all relevant fields, not just names.
func listenersEqual(a, b []gwapiv1.Listener) bool {
	if len(a) != len(b) {
		return false
	}

	aMap := make(map[gwapiv1.SectionName]gwapiv1.Listener)
	for _, l := range a {
		aMap[l.Name] = l
	}

	for _, lb := range b {
		la, ok := aMap[lb.Name]
		if !ok {
			return false
		}
		if !listenerConfigEqual(la, lb) {
			return false
		}
	}

	return true
}

// listenerConfigEqual compares relevant listener config fields
func listenerConfigEqual(a, b gwapiv1.Listener) bool {
	// Compare basic fields
	if a.Port != b.Port || a.Protocol != b.Protocol {
		return false
	}

	// Compare hostname (both nil, or both equal)
	if (a.Hostname == nil) != (b.Hostname == nil) {
		return false
	}
	if a.Hostname != nil && *a.Hostname != *b.Hostname {
		return false
	}

	// Compare AllowedRoutes
	if (a.AllowedRoutes == nil) != (b.AllowedRoutes == nil) {
		return false
	}
	if a.AllowedRoutes != nil {
		if (a.AllowedRoutes.Namespaces == nil) != (b.AllowedRoutes.Namespaces == nil) {
			return false
		}
		if a.AllowedRoutes.Namespaces != nil {
			if (a.AllowedRoutes.Namespaces.From == nil) != (b.AllowedRoutes.Namespaces.From == nil) {
				return false
			}
			if a.AllowedRoutes.Namespaces.From != nil && *a.AllowedRoutes.Namespaces.From != *b.AllowedRoutes.Namespaces.From {
				return false
			}
		}
	}

	// Compare TLS config
	if (a.TLS == nil) != (b.TLS == nil) {
		return false
	}
	if a.TLS != nil {
		// Compare TLS mode
		if (a.TLS.Mode == nil) != (b.TLS.Mode == nil) {
			return false
		}
		if a.TLS.Mode != nil && *a.TLS.Mode != *b.TLS.Mode {
			return false
		}

		// Compare certificate refs
		if len(a.TLS.CertificateRefs) != len(b.TLS.CertificateRefs) {
			return false
		}
		for i := range a.TLS.CertificateRefs {
			if a.TLS.CertificateRefs[i].Name != b.TLS.CertificateRefs[i].Name {
				return false
			}
			// Compare namespace if set
			aNs := a.TLS.CertificateRefs[i].Namespace
			bNs := b.TLS.CertificateRefs[i].Namespace
			if (aNs == nil) != (bNs == nil) {
				return false
			}
			if aNs != nil && *aNs != *bNs {
				return false
			}
		}
	}

	return true
}
