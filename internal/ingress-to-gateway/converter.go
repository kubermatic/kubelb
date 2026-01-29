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
	"fmt"
	"sort"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// ConversionResult holds the converted HTTPRoutes and any warnings
type ConversionResult struct {
	// HTTPRoutes contains one HTTPRoute per unique host (or one for no-host rules)
	HTTPRoutes []*gwapiv1.HTTPRoute
	Warnings   []string
}

// ConvertIngress converts a Kubernetes Ingress to Gateway API HTTPRoutes.
// Returns one HTTPRoute per unique host to avoid cross-host path matching issues.
func ConvertIngress(ingress *networkingv1.Ingress, gatewayName, gatewayNamespace string) ConversionResult {
	result := ConversionResult{
		Warnings: []string{},
	}

	// Group rules by host
	hostRules := make(map[string][]gwapiv1.HTTPRouteRule)
	const noHostKey = ""

	for _, rule := range ingress.Spec.Rules {
		host := rule.Host
		if rule.HTTP == nil {
			continue
		}

		for _, path := range rule.HTTP.Paths {
			httpRouteRule, warnings := convertPath(path)
			result.Warnings = append(result.Warnings, warnings...)
			hostRules[host] = append(hostRules[host], httpRouteRule)
		}
	}

	// Handle default backend - applies to all hosts or creates catch-all
	var defaultRule *gwapiv1.HTTPRouteRule
	if ingress.Spec.DefaultBackend != nil {
		rule, warnings := convertDefaultBackend(ingress.Spec.DefaultBackend)
		defaultRule = rule
		result.Warnings = append(result.Warnings, warnings...)
	}

	// Warn about TLS config (handled by Gateway, not HTTPRoute)
	if len(ingress.Spec.TLS) > 0 {
		result.Warnings = append(result.Warnings, "TLS configuration ignored; configure TLS on the Gateway listener instead")
	}

	// Generate HTTPRoutes per host
	parentRef := buildParentRef(gatewayName, gatewayNamespace, ingress.Namespace)

	// Sort hosts for deterministic output ordering
	hosts := make([]string, 0, len(hostRules))
	for host := range hostRules {
		if host != noHostKey {
			hosts = append(hosts, host)
		}
	}
	sort.Strings(hosts)

	// Process host-specific rules in sorted order
	for _, host := range hosts {
		rules := hostRules[host]

		routeName := httpRouteName(ingress.Name, host)
		httpRoute := &gwapiv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      routeName,
				Namespace: ingress.Namespace,
				Labels:    copyLabels(ingress.Labels),
			},
			Spec: gwapiv1.HTTPRouteSpec{
				CommonRouteSpec: gwapiv1.CommonRouteSpec{
					ParentRefs: []gwapiv1.ParentReference{parentRef},
				},
				Hostnames: []gwapiv1.Hostname{gwapiv1.Hostname(host)},
				Rules:     rules,
			},
		}

		// Add default backend as lowest-priority rule for this host
		if defaultRule != nil {
			httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, *defaultRule)
		}

		result.HTTPRoutes = append(result.HTTPRoutes, httpRoute)
	}

	// Process no-host rules (catch-all)
	if noHostRules, ok := hostRules[noHostKey]; ok {
		if defaultRule != nil {
			noHostRules = append(noHostRules, *defaultRule)
		}

		httpRoute := &gwapiv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ingress.Name, // no suffix for catch-all
				Namespace: ingress.Namespace,
				Labels:    copyLabels(ingress.Labels),
			},
			Spec: gwapiv1.HTTPRouteSpec{
				CommonRouteSpec: gwapiv1.CommonRouteSpec{
					ParentRefs: []gwapiv1.ParentReference{parentRef},
				},
				Rules: noHostRules,
			},
		}
		result.HTTPRoutes = append(result.HTTPRoutes, httpRoute)
	} else if defaultRule != nil && len(hostRules) == 0 {
		// Only default backend, no host-specific rules
		httpRoute := &gwapiv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ingress.Name,
				Namespace: ingress.Namespace,
				Labels:    copyLabels(ingress.Labels),
			},
			Spec: gwapiv1.HTTPRouteSpec{
				CommonRouteSpec: gwapiv1.CommonRouteSpec{
					ParentRefs: []gwapiv1.ParentReference{parentRef},
				},
				Rules: []gwapiv1.HTTPRouteRule{*defaultRule},
			},
		}
		result.HTTPRoutes = append(result.HTTPRoutes, httpRoute)
	}

	if len(result.HTTPRoutes) == 0 {
		result.Warnings = append(result.Warnings, "no HTTPRoutes generated, Ingress may be misconfigured")
	}

	return result
}

// httpRouteName generates HTTPRoute name from Ingress name and host.
// For host-specific routes, appends sanitized host suffix.
func httpRouteName(ingressName, host string) string {
	if host == "" {
		return ingressName
	}
	// Sanitize host for use in name (replace dots with dashes, truncate)
	sanitized := host
	// Replace dots with dashes
	for i := 0; i < len(sanitized); i++ {
		if sanitized[i] == '.' {
			sanitized = sanitized[:i] + "-" + sanitized[i+1:]
		}
	}
	// Truncate to keep total name under 253 chars (K8s limit)
	maxHostLen := 253 - len(ingressName) - 1 // -1 for separator
	if len(sanitized) > maxHostLen {
		sanitized = sanitized[:maxHostLen]
	}
	return fmt.Sprintf("%s-%s", ingressName, sanitized)
}

func buildParentRef(gatewayName, gatewayNamespace, routeNamespace string) gwapiv1.ParentReference {
	ref := gwapiv1.ParentReference{
		Name: gwapiv1.ObjectName(gatewayName),
	}

	// Only set namespace if different from route namespace
	if gatewayNamespace != "" && gatewayNamespace != routeNamespace {
		ns := gwapiv1.Namespace(gatewayNamespace)
		ref.Namespace = &ns
	}

	return ref
}

func convertPath(path networkingv1.HTTPIngressPath) (gwapiv1.HTTPRouteRule, []string) {
	var warnings []string
	rule := gwapiv1.HTTPRouteRule{}

	// Convert path match
	match := gwapiv1.HTTPRouteMatch{}
	if path.Path != "" {
		pathMatch := gwapiv1.HTTPPathMatch{
			Value: &path.Path,
		}

		pathType := convertPathType(path.PathType)
		pathMatch.Type = &pathType

		if path.PathType != nil && *path.PathType == networkingv1.PathTypeImplementationSpecific {
			warnings = append(warnings, fmt.Sprintf("path %q uses ImplementationSpecific type, converted to PathPrefix", path.Path))
		}

		match.Path = &pathMatch
	}

	rule.Matches = []gwapiv1.HTTPRouteMatch{match}

	// Convert backend
	if path.Backend.Service != nil {
		backendRef, backendWarnings := convertServiceBackend(path.Backend.Service)
		rule.BackendRefs = []gwapiv1.HTTPBackendRef{backendRef}
		warnings = append(warnings, backendWarnings...)
	} else if path.Backend.Resource != nil {
		// Resource backends (non-Service) are not supported
		warnings = append(warnings, fmt.Sprintf("resource backend %q not supported; only Service backends are converted", path.Backend.Resource.Name))
	}

	return rule, warnings
}

func convertPathType(pathType *networkingv1.PathType) gwapiv1.PathMatchType {
	if pathType == nil {
		return gwapiv1.PathMatchPathPrefix
	}

	switch *pathType {
	case networkingv1.PathTypeExact:
		return gwapiv1.PathMatchExact
	case networkingv1.PathTypePrefix:
		return gwapiv1.PathMatchPathPrefix
	case networkingv1.PathTypeImplementationSpecific:
		// Default to prefix for implementation-specific
		return gwapiv1.PathMatchPathPrefix
	default:
		return gwapiv1.PathMatchPathPrefix
	}
}

func convertServiceBackend(svc *networkingv1.IngressServiceBackend) (gwapiv1.HTTPBackendRef, []string) {
	var warnings []string

	backendRef := gwapiv1.HTTPBackendRef{
		BackendRef: gwapiv1.BackendRef{
			BackendObjectReference: gwapiv1.BackendObjectReference{
				Name: gwapiv1.ObjectName(svc.Name),
			},
		},
	}

	// Set port
	if svc.Port.Number != 0 {
		port := svc.Port.Number
		backendRef.Port = &port
	} else if svc.Port.Name != "" {
		// Named port requires Service lookup to resolve - not supported
		warnings = append(warnings, fmt.Sprintf("service %q uses named port %q which cannot be resolved; specify port number instead", svc.Name, svc.Port.Name))
	}

	return backendRef, warnings
}

func convertDefaultBackend(backend *networkingv1.IngressBackend) (*gwapiv1.HTTPRouteRule, []string) {
	var warnings []string

	if backend.Service == nil {
		if backend.Resource != nil {
			return nil, []string{fmt.Sprintf("default backend uses resource %q which is not supported", backend.Resource.Name)}
		}
		return nil, []string{"default backend has no service reference"}
	}

	backendRef, backendWarnings := convertServiceBackend(backend.Service)
	warnings = append(warnings, backendWarnings...)

	// Create a catch-all rule for default backend
	rule := gwapiv1.HTTPRouteRule{
		BackendRefs: []gwapiv1.HTTPBackendRef{backendRef},
	}

	// Match all paths with a prefix match on "/"
	pathPrefix := gwapiv1.PathMatchPathPrefix
	rootPath := "/"
	rule.Matches = []gwapiv1.HTTPRouteMatch{
		{
			Path: &gwapiv1.HTTPPathMatch{
				Type:  &pathPrefix,
				Value: &rootPath,
			},
		},
	}

	return &rule, warnings
}

func copyLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	copied := make(map[string]string, len(labels))
	for k, v := range labels {
		copied[k] = v
	}
	return copied
}
