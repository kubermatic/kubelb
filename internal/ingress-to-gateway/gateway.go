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

	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GatewayConfig holds configuration for Gateway reconciliation
type GatewayConfig struct {
	Name         string
	Namespace    string
	ClassName    string
	TLSListeners []TLSListener
	Annotations  map[string]string
}

// reconcileGateway ensures the Gateway exists and has required listeners
func (r *Reconciler) reconcileGateway(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress, tlsListeners []TLSListener) error {
	gatewayNs := r.GatewayNamespace
	if gatewayNs == "" {
		gatewayNs = ingress.Namespace
	}

	// Build desired Gateway config
	config := GatewayConfig{
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
			gateway := buildGateway(config)
			log.Info("Creating Gateway", "name", gateway.Name, "namespace", gateway.Namespace)
			return r.Create(ctx, gateway)
		}
		return fmt.Errorf("failed to get Gateway: %w", err)
	}

	// Update existing Gateway
	updated := false

	// Merge listeners
	newListeners := mergeListeners(existing.Spec.Listeners, buildListeners(config.TLSListeners))
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
		if target, ok := ingress.Annotations[ExternalDNSTarget]; ok {
			result[ExternalDNSTarget] = target
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
		if strings.HasPrefix(k, ExternalDNSAnnotationPrefix) && k != ExternalDNSTarget {
			result[k] = v
		}
	}

	return result
}

// buildGateway creates a new Gateway resource
func buildGateway(config GatewayConfig) *gwapiv1.Gateway {
	gateway := &gwapiv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels: map[string]string{
				LabelManagedBy: ControllerName,
			},
			Annotations: config.Annotations,
		},
		Spec: gwapiv1.GatewaySpec{
			GatewayClassName: gwapiv1.ObjectName(config.ClassName),
			Listeners:        buildListeners(config.TLSListeners),
		},
	}

	return gateway
}

// buildListeners creates Gateway listeners from TLS config
func buildListeners(tlsListeners []TLSListener) []gwapiv1.Listener {
	listeners := []gwapiv1.Listener{
		// Always include HTTP listener
		{
			Name:     "http",
			Port:     80,
			Protocol: gwapiv1.HTTPProtocolType,
		},
	}

	// Add HTTPS listeners for each TLS host
	seen := make(map[string]bool)
	for _, tls := range tlsListeners {
		hostname := string(tls.Hostname)
		// Dedupe by hostname
		if seen[hostname] {
			continue
		}
		seen[hostname] = true

		listenerName := listenerNameFromHostname(hostname)
		listener := gwapiv1.Listener{
			Name:     gwapiv1.SectionName(listenerName),
			Hostname: ptr.To(tls.Hostname),
			Port:     443,
			Protocol: gwapiv1.HTTPSProtocolType,
			TLS: &gwapiv1.ListenerTLSConfig{
				Mode: ptr.To(gwapiv1.TLSModeTerminate),
				CertificateRefs: []gwapiv1.SecretObjectReference{
					{
						Name: gwapiv1.ObjectName(tls.SecretName),
					},
				},
			},
		}

		listeners = append(listeners, listener)
	}

	return listeners
}

// listenerNameFromHostname generates a listener name from hostname
func listenerNameFromHostname(hostname string) string {
	if hostname == "*" {
		return "https-wildcard"
	}
	// Replace dots and wildcards with dashes
	name := strings.ReplaceAll(hostname, ".", "-")
	name = strings.ReplaceAll(name, "*", "wildcard")
	return "https-" + name
}

// mergeListeners merges desired listeners into existing.
// - Adds new listeners from desired set
// - Updates existing listeners if their config differs (e.g., TLS secret changed)
// - Note: Stale listener removal requires separate cleanup logic since Gateway is shared
func mergeListeners(existing, desired []gwapiv1.Listener) []gwapiv1.Listener {
	// Build map of desired listeners by name
	desiredByName := make(map[gwapiv1.SectionName]gwapiv1.Listener)
	for _, l := range desired {
		desiredByName[l.Name] = l
	}

	result := make([]gwapiv1.Listener, 0, len(existing)+len(desired))

	// Process existing listeners - update if in desired set, keep otherwise
	for _, existing := range existing {
		if desiredListener, found := desiredByName[existing.Name]; found {
			// Update existing listener with desired config
			result = append(result, desiredListener)
			delete(desiredByName, existing.Name)
		} else {
			// Keep existing listener (may belong to another Ingress)
			result = append(result, existing)
		}
	}

	// Add any remaining desired listeners (new ones)
	for _, l := range desired {
		if _, stillNeeded := desiredByName[l.Name]; stillNeeded {
			result = append(result, l)
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
