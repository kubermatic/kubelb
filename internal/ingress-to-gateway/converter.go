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

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// ConversionResult holds the converted HTTPRoute and any warnings
type ConversionResult struct {
	HTTPRoute *gwapiv1.HTTPRoute
	Warnings  []string
}

// ConvertIngress converts a Kubernetes Ingress to a Gateway API HTTPRoute
func ConvertIngress(ingress *networkingv1.Ingress, gatewayName, gatewayNamespace string) ConversionResult {
	result := ConversionResult{
		Warnings: []string{},
	}

	httpRoute := &gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingress.Name,
			Namespace: ingress.Namespace,
			Labels:    copyLabels(ingress.Labels),
		},
		Spec: gwapiv1.HTTPRouteSpec{},
	}

	// Set parent reference to Gateway
	httpRoute.Spec.ParentRefs = []gwapiv1.ParentReference{
		buildParentRef(gatewayName, gatewayNamespace, ingress.Namespace),
	}

	// Convert rules
	var hostnames []gwapiv1.Hostname
	var rules []gwapiv1.HTTPRouteRule

	for _, rule := range ingress.Spec.Rules {
		// Collect hostnames
		if rule.Host != "" {
			hostnames = append(hostnames, gwapiv1.Hostname(rule.Host))
		}

		// Convert HTTP paths to HTTPRoute rules
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				httpRouteRule, warnings := convertPath(path)
				rules = append(rules, httpRouteRule)
				result.Warnings = append(result.Warnings, warnings...)
			}
		}
	}

	// Handle default backend
	if ingress.Spec.DefaultBackend != nil {
		defaultRule, warnings := convertDefaultBackend(ingress.Spec.DefaultBackend)
		if defaultRule != nil {
			rules = append(rules, *defaultRule)
			result.Warnings = append(result.Warnings, warnings...)
		}
	}

	// If no rules were generated, add a catch-all
	if len(rules) == 0 {
		result.Warnings = append(result.Warnings, "no rules generated, Ingress may be misconfigured")
	}

	httpRoute.Spec.Hostnames = hostnames
	httpRoute.Spec.Rules = rules
	result.HTTPRoute = httpRoute

	return result
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
		backendRef := convertServiceBackend(path.Backend.Service)
		rule.BackendRefs = []gwapiv1.HTTPBackendRef{backendRef}
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

func convertServiceBackend(svc *networkingv1.IngressServiceBackend) gwapiv1.HTTPBackendRef {
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
	}
	// Note: Gateway API doesn't support port by name directly in BackendRef

	return backendRef
}

func convertDefaultBackend(backend *networkingv1.IngressBackend) (*gwapiv1.HTTPRouteRule, []string) {
	var warnings []string

	if backend.Service == nil {
		return nil, []string{"default backend has no service reference"}
	}

	// Create a catch-all rule for default backend
	rule := gwapiv1.HTTPRouteRule{
		BackendRefs: []gwapiv1.HTTPBackendRef{
			convertServiceBackend(backend.Service),
		},
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
