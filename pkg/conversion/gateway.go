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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GatewayConfig holds configuration for building a Gateway resource
type GatewayConfig struct {
	Name         string
	Namespace    string
	ClassName    string
	TLSListeners []TLSListener
	Annotations  map[string]string
	// ManagedBy sets the value for the kubelb.k8c.io/managed-by label.
	// Defaults to ControllerName if empty.
	ManagedBy string
}

// BuildGateway creates a Gateway resource from the provided configuration.
// The generated Gateway includes:
// - A default HTTP listener on port 80
// - HTTPS listeners for each TLS configuration on port 443
// - AllowedRoutes set to allow routes from all namespaces
func BuildGateway(config GatewayConfig) *gwapiv1.Gateway {
	managedBy := config.ManagedBy
	if managedBy == "" {
		managedBy = ControllerName
	}

	return &gwapiv1.Gateway{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "gateway.networking.k8s.io/v1",
			Kind:       "Gateway",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels: map[string]string{
				LabelManagedBy: managedBy,
			},
			Annotations: config.Annotations,
		},
		Spec: gwapiv1.GatewaySpec{
			GatewayClassName: gwapiv1.ObjectName(config.ClassName),
			Listeners:        BuildListeners(config.TLSListeners),
		},
	}
}

// BuildListeners creates Gateway listeners from TLS configurations.
// Returns:
// - A default HTTP listener on port 80 with AllowedRoutes from all namespaces
// - HTTPS listeners for each unique hostname on port 443
func BuildListeners(tlsListeners []TLSListener) []gwapiv1.Listener {
	listeners := []gwapiv1.Listener{
		// Always include HTTP listener
		{
			Name:     "http",
			Port:     80,
			Protocol: gwapiv1.HTTPProtocolType,
			AllowedRoutes: &gwapiv1.AllowedRoutes{
				Namespaces: &gwapiv1.RouteNamespaces{
					From: ptr.To(gwapiv1.NamespacesFromAll),
				},
			},
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

		listenerName := ListenerNameFromHostname(hostname)
		listener := gwapiv1.Listener{
			Name:     gwapiv1.SectionName(listenerName),
			Hostname: ptr.To(tls.Hostname),
			Port:     443,
			Protocol: gwapiv1.HTTPSProtocolType,
			AllowedRoutes: &gwapiv1.AllowedRoutes{
				Namespaces: &gwapiv1.RouteNamespaces{
					From: ptr.To(gwapiv1.NamespacesFromAll),
				},
			},
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

// ListenerNameFromHostname generates a listener name from hostname.
// Examples:
// - "example.com" -> "https-example-com"
// - "*.example.com" -> "https-wildcard-example-com"
// - "*" -> "https-wildcard"
func ListenerNameFromHostname(hostname string) string {
	if hostname == "*" {
		return "https-wildcard"
	}
	// Replace dots and wildcards with dashes
	name := strings.ReplaceAll(hostname, ".", "-")
	name = strings.ReplaceAll(name, "*", "wildcard")
	return "https-" + name
}

// MergeListeners merges desired listeners into existing.
// - Adds new listeners from desired set
// - Updates existing listeners if their name matches
// - Keeps existing listeners not in desired set (may belong to other Ingresses)
func MergeListeners(existing, desired []gwapiv1.Listener) []gwapiv1.Listener {
	desiredByName := make(map[gwapiv1.SectionName]gwapiv1.Listener)
	for _, l := range desired {
		desiredByName[l.Name] = l
	}

	result := make([]gwapiv1.Listener, 0, len(existing)+len(desired))

	// Process existing listeners - update if in desired set, keep otherwise
	for _, existing := range existing {
		if desiredListener, found := desiredByName[existing.Name]; found {
			result = append(result, desiredListener)
			delete(desiredByName, existing.Name)
		} else {
			result = append(result, existing)
		}
	}

	// Add remaining desired listeners (new ones)
	for _, l := range desired {
		if _, stillNeeded := desiredByName[l.Name]; stillNeeded {
			result = append(result, l)
		}
	}

	return result
}
