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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"k8c.io/kubelb/internal/ingress-to-gateway/annotations"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// ConversionInput holds input data for conversion including Services for port resolution
type ConversionInput struct {
	Ingress          *networkingv1.Ingress
	GatewayName      string
	GatewayNamespace string
	// Services maps namespace/name to Service for port resolution (optional)
	Services map[types.NamespacedName]*corev1.Service
	// DomainReplace is the domain suffix to strip from hostnames (e.g., "example.com")
	DomainReplace string
	// DomainSuffix is the replacement domain suffix (e.g., "new.io")
	DomainSuffix string
}

// TLSListener represents a TLS listener configuration for the Gateway
type TLSListener struct {
	Hostname   gwapiv1.Hostname
	SecretName string
	// SecretNamespace is the namespace of the secret (same as Ingress namespace)
	SecretNamespace string
}

// ConversionResult holds the converted HTTPRoutes, Gateway configs, and any warnings
type ConversionResult struct {
	// HTTPRoutes contains one HTTPRoute per unique host (or one for no-host rules)
	HTTPRoutes []*gwapiv1.HTTPRoute
	// GRPCRoutes contains GRPCRoutes for backends with backend-protocol: GRPC/GRPCS
	GRPCRoutes []*gwapiv1.GRPCRoute
	// TLSListeners contains HTTPS listener configs to add to Gateway
	TLSListeners []TLSListener
	// Warnings contains messages about unconvertible annotations or configuration
	Warnings []string
	// ProcessedAnnotations lists the annotation keys that were processed
	ProcessedAnnotations []string
}

// ConvertIngress converts a Kubernetes Ingress to Gateway API HTTPRoutes.
// Returns one HTTPRoute per unique host to avoid cross-host path matching issues.
//
// Deprecated: Use ConvertIngressWithServices for better port resolution.
func ConvertIngress(ingress *networkingv1.Ingress, gatewayName, gatewayNamespace string) ConversionResult {
	return ConvertIngressWithServices(ConversionInput{
		Ingress:          ingress,
		GatewayName:      gatewayName,
		GatewayNamespace: gatewayNamespace,
		Services:         nil,
	})
}

// ConvertIngressWithServices converts an Ingress with Service lookup for port resolution.
func ConvertIngressWithServices(input ConversionInput) ConversionResult {
	ingress := input.Ingress
	result := ConversionResult{
		Warnings: []string{},
	}

	// Check if this Ingress should generate GRPCRoute instead of HTTPRoute
	isGRPC := isGRPCIngress(ingress)

	// Convert annotations to filters (only for HTTPRoute, not gRPC)
	var annotationResult annotations.AnnotationConversionResult
	if !isGRPC {
		annotationConverter := annotations.NewConverter()
		annotationResult = annotationConverter.Convert(ingress.Annotations)
		result.Warnings = append(result.Warnings, annotationResult.Warnings...)
		result.ProcessedAnnotations = annotationResult.Processed
	} else {
		// For gRPC, skip annotation conversion but note it
		result.Warnings = append(result.Warnings, "backend-protocol=GRPC detected, generating GRPCRoute (annotation filters not applicable to GRPCRoute)")
	}

	// Convert TLS configuration to Gateway listeners
	var tlsWarnings []string
	result.TLSListeners, tlsWarnings = convertTLSConfig(ingress.Spec.TLS, ingress.Namespace, input.DomainReplace, input.DomainSuffix)
	result.Warnings = append(result.Warnings, tlsWarnings...)

	// Generate parent reference
	parentRef := buildParentRef(input.GatewayName, input.GatewayNamespace, ingress.Namespace)

	if isGRPC {
		// Generate GRPCRoutes
		result.GRPCRoutes, result.Warnings = convertToGRPCRoutes(input, parentRef, result.Warnings)
	} else {
		// Generate HTTPRoutes
		result.HTTPRoutes, result.Warnings = convertToHTTPRoutes(input, parentRef, annotationResult.Filters, result.Warnings)
	}

	if len(result.HTTPRoutes) == 0 && len(result.GRPCRoutes) == 0 {
		result.Warnings = append(result.Warnings, "no routes generated, Ingress may be misconfigured")
	}

	return result
}

// isGRPCIngress checks if the Ingress uses gRPC backend protocol
func isGRPCIngress(ingress *networkingv1.Ingress) bool {
	if ingress.Annotations == nil {
		return false
	}
	protocol := strings.ToUpper(ingress.Annotations[NginxBackendProtocol])
	return protocol == "GRPC" || protocol == "GRPCS"
}

// convertToHTTPRoutes generates HTTPRoutes from Ingress rules
func convertToHTTPRoutes(input ConversionInput, parentRef gwapiv1.ParentReference, filters []gwapiv1.HTTPRouteFilter, warnings []string) ([]*gwapiv1.HTTPRoute, []string) {
	ingress := input.Ingress
	var httpRoutes []*gwapiv1.HTTPRoute

	// Group rules by host
	hostRules := make(map[string][]gwapiv1.HTTPRouteRule)
	const noHostKey = ""

	for i, rule := range ingress.Spec.Rules {
		host := transformHostname(rule.Host, input.DomainReplace, input.DomainSuffix)
		if rule.HTTP == nil {
			warnings = append(warnings, fmt.Sprintf("rule[%d] has no HTTP configuration, skipped", i))
			continue
		}

		for _, path := range rule.HTTP.Paths {
			httpRouteRule, ruleWarnings := convertPathWithServices(path, ingress.Namespace, input.Services)
			warnings = append(warnings, ruleWarnings...)

			// Add annotation filters to the rule
			if len(filters) > 0 {
				httpRouteRule.Filters = append(httpRouteRule.Filters, filters...)
			}

			hostRules[host] = append(hostRules[host], httpRouteRule)
		}
	}

	// Handle default backend
	var defaultRule *gwapiv1.HTTPRouteRule
	if ingress.Spec.DefaultBackend != nil {
		rule, defaultWarnings := convertDefaultBackendWithServices(ingress.Spec.DefaultBackend, ingress.Namespace, input.Services)
		if rule != nil && len(filters) > 0 {
			rule.Filters = append(rule.Filters, filters...)
		}
		defaultRule = rule
		warnings = append(warnings, defaultWarnings...)
	}

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

		if defaultRule != nil {
			httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, *defaultRule)
		}

		httpRoutes = append(httpRoutes, httpRoute)
	}

	// Process no-host rules (catch-all)
	if noHostRules, ok := hostRules[noHostKey]; ok {
		if defaultRule != nil {
			noHostRules = append(noHostRules, *defaultRule)
		}

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
				Rules: noHostRules,
			},
		}
		httpRoutes = append(httpRoutes, httpRoute)
	} else if defaultRule != nil && len(hostRules) == 0 {
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
		httpRoutes = append(httpRoutes, httpRoute)
	}

	return httpRoutes, warnings
}

// convertToGRPCRoutes generates GRPCRoutes from Ingress rules
func convertToGRPCRoutes(input ConversionInput, parentRef gwapiv1.ParentReference, warnings []string) ([]*gwapiv1.GRPCRoute, []string) {
	ingress := input.Ingress
	var grpcRoutes []*gwapiv1.GRPCRoute

	// Group rules by host
	hostRules := make(map[string][]gwapiv1.GRPCRouteRule)
	const noHostKey = ""

	for i, rule := range ingress.Spec.Rules {
		host := transformHostname(rule.Host, input.DomainReplace, input.DomainSuffix)
		if rule.HTTP == nil {
			warnings = append(warnings, fmt.Sprintf("rule[%d] has no HTTP configuration, skipped", i))
			continue
		}

		for _, path := range rule.HTTP.Paths {
			grpcRule, ruleWarnings := convertPathToGRPCRule(path, ingress.Namespace, input.Services)
			warnings = append(warnings, ruleWarnings...)
			hostRules[host] = append(hostRules[host], grpcRule)
		}
	}

	// Handle default backend
	var defaultRule *gwapiv1.GRPCRouteRule
	if ingress.Spec.DefaultBackend != nil {
		rule, defaultWarnings := convertDefaultBackendToGRPCRule(ingress.Spec.DefaultBackend, ingress.Namespace, input.Services)
		defaultRule = rule
		warnings = append(warnings, defaultWarnings...)
	}

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

		routeName := grpcRouteName(ingress.Name, host)
		grpcRoute := &gwapiv1.GRPCRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      routeName,
				Namespace: ingress.Namespace,
				Labels:    copyLabels(ingress.Labels),
			},
			Spec: gwapiv1.GRPCRouteSpec{
				CommonRouteSpec: gwapiv1.CommonRouteSpec{
					ParentRefs: []gwapiv1.ParentReference{parentRef},
				},
				Hostnames: []gwapiv1.Hostname{gwapiv1.Hostname(host)},
				Rules:     rules,
			},
		}

		if defaultRule != nil {
			grpcRoute.Spec.Rules = append(grpcRoute.Spec.Rules, *defaultRule)
		}

		grpcRoutes = append(grpcRoutes, grpcRoute)
	}

	// Process no-host rules (catch-all)
	if noHostRules, ok := hostRules[noHostKey]; ok {
		if defaultRule != nil {
			noHostRules = append(noHostRules, *defaultRule)
		}

		grpcRoute := &gwapiv1.GRPCRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ingress.Name + "-grpc",
				Namespace: ingress.Namespace,
				Labels:    copyLabels(ingress.Labels),
			},
			Spec: gwapiv1.GRPCRouteSpec{
				CommonRouteSpec: gwapiv1.CommonRouteSpec{
					ParentRefs: []gwapiv1.ParentReference{parentRef},
				},
				Rules: noHostRules,
			},
		}
		grpcRoutes = append(grpcRoutes, grpcRoute)
	} else if defaultRule != nil && len(hostRules) == 0 {
		grpcRoute := &gwapiv1.GRPCRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ingress.Name + "-grpc",
				Namespace: ingress.Namespace,
				Labels:    copyLabels(ingress.Labels),
			},
			Spec: gwapiv1.GRPCRouteSpec{
				CommonRouteSpec: gwapiv1.CommonRouteSpec{
					ParentRefs: []gwapiv1.ParentReference{parentRef},
				},
				Rules: []gwapiv1.GRPCRouteRule{*defaultRule},
			},
		}
		grpcRoutes = append(grpcRoutes, grpcRoute)
	}

	return grpcRoutes, warnings
}

// grpcRouteName generates GRPCRoute name from Ingress name and host.
// If truncation is needed, appends a short hash to avoid collisions.
func grpcRouteName(ingressName, host string) string {
	if host == "" {
		return ingressName + "-grpc"
	}
	// Sanitize host for use in name (replace dots with dashes)
	sanitized := strings.ReplaceAll(host, ".", "-")
	sanitized = strings.ReplaceAll(sanitized, "*", "wildcard")

	fullName := fmt.Sprintf("%s-grpc-%s", ingressName, sanitized)

	// K8s name limit is 253 chars, but keep under 63 for DNS label compatibility
	const maxLen = 63
	if len(fullName) <= maxLen {
		return fullName
	}

	// Truncation needed - append hash suffix to avoid collisions
	hash := sha256.Sum256([]byte(host))
	hashSuffix := hex.EncodeToString(hash[:])[:8]

	maxTruncLen := maxLen - 1 - len(hashSuffix)
	truncated := fullName[:maxTruncLen]
	truncated = strings.TrimRight(truncated, "-")

	return fmt.Sprintf("%s-%s", truncated, hashSuffix)
}

// convertPathToGRPCRule converts an Ingress path to a GRPCRouteRule
func convertPathToGRPCRule(path networkingv1.HTTPIngressPath, namespace string, services map[types.NamespacedName]*corev1.Service) (gwapiv1.GRPCRouteRule, []string) {
	var warnings []string
	rule := gwapiv1.GRPCRouteRule{}

	// For gRPC, path-based matching is limited
	// Ingress path /foo.Bar/Baz could map to service=foo.Bar, method=Baz
	// But typically, Ingress paths for gRPC are just / or a generic prefix
	if path.Path != "" && path.Path != "/" {
		warnings = append(warnings, fmt.Sprintf("path %q will be ignored for GRPCRoute; use service/method matching in GRPCRoute spec if needed", path.Path))
	}

	// Convert backend
	if path.Backend.Service != nil {
		backendRef, backendWarnings := convertServiceBackendToGRPC(path.Backend.Service, namespace, services)
		rule.BackendRefs = []gwapiv1.GRPCBackendRef{backendRef}
		warnings = append(warnings, backendWarnings...)
	} else if path.Backend.Resource != nil {
		warnings = append(warnings, fmt.Sprintf("resource backend %q not supported; only Service backends are converted", path.Backend.Resource.Name))
	}

	return rule, warnings
}

// convertDefaultBackendToGRPCRule converts a default backend to a GRPCRouteRule
func convertDefaultBackendToGRPCRule(backend *networkingv1.IngressBackend, namespace string, services map[types.NamespacedName]*corev1.Service) (*gwapiv1.GRPCRouteRule, []string) {
	var warnings []string

	if backend.Service == nil {
		if backend.Resource != nil {
			return nil, []string{fmt.Sprintf("default backend uses resource %q which is not supported", backend.Resource.Name)}
		}
		return nil, []string{"default backend has no service reference"}
	}

	backendRef, backendWarnings := convertServiceBackendToGRPC(backend.Service, namespace, services)
	warnings = append(warnings, backendWarnings...)

	rule := gwapiv1.GRPCRouteRule{
		BackendRefs: []gwapiv1.GRPCBackendRef{backendRef},
	}

	return &rule, warnings
}

// convertServiceBackendToGRPC converts a service backend to GRPCBackendRef
func convertServiceBackendToGRPC(svc *networkingv1.IngressServiceBackend, namespace string, services map[types.NamespacedName]*corev1.Service) (gwapiv1.GRPCBackendRef, []string) {
	var warnings []string

	backendRef := gwapiv1.GRPCBackendRef{
		BackendRef: gwapiv1.BackendRef{
			BackendObjectReference: gwapiv1.BackendObjectReference{
				Name: gwapiv1.ObjectName(svc.Name),
			},
		},
	}

	// Set port with Service lookup fallback (same logic as HTTP)
	switch {
	case svc.Port.Number != 0:
		port := svc.Port.Number
		backendRef.Port = &port

	case svc.Port.Name != "":
		if services != nil {
			key := types.NamespacedName{Name: svc.Name, Namespace: namespace}
			if service, ok := services[key]; ok {
				if port, found := findPortByName(service, svc.Port.Name); found {
					backendRef.Port = &port
				} else {
					warnings = append(warnings, fmt.Sprintf("service %q has no port named %q", svc.Name, svc.Port.Name))
				}
			} else {
				warnings = append(warnings, fmt.Sprintf("service %q not found for port resolution", svc.Name))
			}
		} else {
			warnings = append(warnings, fmt.Sprintf("service %q uses named port %q which cannot be resolved", svc.Name, svc.Port.Name))
		}

	default:
		if services != nil {
			key := types.NamespacedName{Name: svc.Name, Namespace: namespace}
			if service, ok := services[key]; ok {
				switch len(service.Spec.Ports) {
				case 1:
					port := service.Spec.Ports[0].Port
					backendRef.Port = &port
				case 0:
					warnings = append(warnings, fmt.Sprintf("service %q has no ports defined", svc.Name))
				default:
					warnings = append(warnings, fmt.Sprintf("service %q has multiple ports; specify port explicitly", svc.Name))
				}
			} else {
				warnings = append(warnings, fmt.Sprintf("service %q not found for port resolution", svc.Name))
			}
		} else {
			warnings = append(warnings, fmt.Sprintf("service %q has no port specified", svc.Name))
		}
	}

	return backendRef, warnings
}

// convertTLSConfig converts Ingress TLS config to Gateway listener configurations
func convertTLSConfig(tlsConfigs []networkingv1.IngressTLS, namespace, domainReplace, domainSuffix string) ([]TLSListener, []string) {
	var listeners []TLSListener
	var warnings []string

	for _, tls := range tlsConfigs {
		// Warn if TLS has hosts but no secret (relies on cert-manager or external provisioning)
		if len(tls.Hosts) > 0 && tls.SecretName == "" {
			warnings = append(warnings,
				fmt.Sprintf("TLS config has hosts %v but no secretName; Gateway will need manual certificate configuration or cert-manager",
					tls.Hosts))
		}

		// Each TLS config can have multiple hosts sharing the same secret
		for _, host := range tls.Hosts {
			listener := TLSListener{
				Hostname:        gwapiv1.Hostname(transformHostname(host, domainReplace, domainSuffix)),
				SecretName:      tls.SecretName,
				SecretNamespace: namespace,
			}
			listeners = append(listeners, listener)
		}

		// If no hosts specified but secret exists, it's a catch-all TLS
		if len(tls.Hosts) == 0 && tls.SecretName != "" {
			listener := TLSListener{
				Hostname:        "*",
				SecretName:      tls.SecretName,
				SecretNamespace: namespace,
			}
			listeners = append(listeners, listener)
		}
	}

	return listeners, warnings
}

// httpRouteName generates HTTPRoute name from Ingress name and host.
// For host-specific routes, appends sanitized host suffix.
// If truncation is needed, appends a short hash to avoid collisions.
func httpRouteName(ingressName, host string) string {
	if host == "" {
		return ingressName
	}
	// Sanitize host for use in name (replace dots with dashes)
	sanitized := strings.ReplaceAll(host, ".", "-")
	sanitized = strings.ReplaceAll(sanitized, "*", "wildcard")

	fullName := fmt.Sprintf("%s-%s", ingressName, sanitized)

	// K8s name limit is 253 chars, but keep under 63 for DNS label compatibility
	const maxLen = 63
	if len(fullName) <= maxLen {
		return fullName
	}

	// Truncation needed - append hash suffix to avoid collisions
	// Hash the original host to generate unique suffix
	hash := sha256.Sum256([]byte(host))
	hashSuffix := hex.EncodeToString(hash[:])[:8] // First 8 chars of hash

	// Reserve space for hash suffix and separator
	// Format: <truncated>-<hash>
	maxTruncLen := maxLen - 1 - len(hashSuffix) // -1 for separator
	truncated := fullName[:maxTruncLen]

	// Ensure we don't end with a dash
	truncated = strings.TrimRight(truncated, "-")

	return fmt.Sprintf("%s-%s", truncated, hashSuffix)
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

func convertPathWithServices(path networkingv1.HTTPIngressPath, namespace string, services map[types.NamespacedName]*corev1.Service) (gwapiv1.HTTPRouteRule, []string) {
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
		backendRef, backendWarnings := convertServiceBackendWithLookup(path.Backend.Service, namespace, services)
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

// convertServiceBackendWithLookup converts a service backend with optional Service lookup for port resolution
func convertServiceBackendWithLookup(svc *networkingv1.IngressServiceBackend, namespace string, services map[types.NamespacedName]*corev1.Service) (gwapiv1.HTTPBackendRef, []string) {
	var warnings []string

	backendRef := gwapiv1.HTTPBackendRef{
		BackendRef: gwapiv1.BackendRef{
			BackendObjectReference: gwapiv1.BackendObjectReference{
				Name: gwapiv1.ObjectName(svc.Name),
			},
		},
	}

	// Set port with Service lookup fallback
	switch {
	case svc.Port.Number != 0:
		port := svc.Port.Number
		// Validate port range
		if port < 1 || port > 65535 {
			warnings = append(warnings, fmt.Sprintf("service %q has invalid port number %d (must be 1-65535)", svc.Name, port))
		}
		backendRef.Port = &port

	case svc.Port.Name != "":
		// Try to resolve named port via Service lookup
		if services != nil {
			key := types.NamespacedName{Name: svc.Name, Namespace: namespace}
			if service, ok := services[key]; ok {
				if port, found := findPortByName(service, svc.Port.Name); found {
					backendRef.Port = &port
				} else {
					warnings = append(warnings, fmt.Sprintf("service %q has no port named %q", svc.Name, svc.Port.Name))
				}
			} else {
				warnings = append(warnings, fmt.Sprintf("service %q not found for port resolution", svc.Name))
			}
		} else {
			warnings = append(warnings, fmt.Sprintf("service %q uses named port %q which cannot be resolved; specify port number instead", svc.Name, svc.Port.Name))
		}

	default:
		// Neither port number nor name - try to use single-port Service
		if services != nil {
			key := types.NamespacedName{Name: svc.Name, Namespace: namespace}
			if service, ok := services[key]; ok {
				switch len(service.Spec.Ports) {
				case 1:
					port := service.Spec.Ports[0].Port
					backendRef.Port = &port
				case 0:
					warnings = append(warnings, fmt.Sprintf("service %q has no ports defined", svc.Name))
				default:
					warnings = append(warnings, fmt.Sprintf("service %q has multiple ports; specify port explicitly", svc.Name))
				}
			} else {
				warnings = append(warnings, fmt.Sprintf("service %q not found for port resolution", svc.Name))
			}
		} else {
			warnings = append(warnings, fmt.Sprintf("service %q has no port specified; Gateway API requires explicit port", svc.Name))
		}
	}

	return backendRef, warnings
}

// findPortByName finds a port number by name in a Service
func findPortByName(svc *corev1.Service, portName string) (int32, bool) {
	for _, p := range svc.Spec.Ports {
		if p.Name == portName {
			return p.Port, true
		}
	}
	return 0, false
}

func convertDefaultBackendWithServices(backend *networkingv1.IngressBackend, namespace string, services map[types.NamespacedName]*corev1.Service) (*gwapiv1.HTTPRouteRule, []string) {
	var warnings []string

	if backend.Service == nil {
		if backend.Resource != nil {
			return nil, []string{fmt.Sprintf("default backend uses resource %q which is not supported", backend.Resource.Name)}
		}
		return nil, []string{"default backend has no service reference"}
	}

	backendRef, backendWarnings := convertServiceBackendWithLookup(backend.Service, namespace, services)
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

// transformHostname applies domain suffix replacement to a hostname.
// If replace is empty or suffix is empty, returns host unchanged.
// Handles wildcards: *.example.com → *.new.io
// Examples with replace="example.com", suffix="new.io":
//   - app.example.com → app.new.io
//   - foo.bar.example.com → foo.bar.new.io
//   - example.com → new.io
//   - *.example.com → *.new.io
//   - app.other.com → app.other.com (no match)
func transformHostname(host, replace, suffix string) string {
	if replace == "" || suffix == "" {
		return host
	}

	// Handle wildcard prefix
	wildcardPrefix := ""
	checkHost := host
	if strings.HasPrefix(host, "*.") {
		wildcardPrefix = "*."
		checkHost = host[2:]
	}

	// Check if host ends with replace domain
	if checkHost == replace {
		// Exact match (minus wildcard): example.com → new.io
		return wildcardPrefix + suffix
	}

	if strings.HasSuffix(checkHost, "."+replace) {
		// Subdomain match: app.example.com → app.new.io
		prefix := strings.TrimSuffix(checkHost, "."+replace)
		return wildcardPrefix + prefix + "." + suffix
	}

	// No match, return unchanged
	return host
}
