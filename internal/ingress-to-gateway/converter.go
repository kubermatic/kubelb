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
	"encoding/json"
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
	// HTTPRoutes contains converted routes (consolidated by rule signature when possible)
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

// hasRedirectFilter returns true if any filter is a RequestRedirect type.
// Gateway API does not allow RequestRedirect filters with backendRefs.
func hasRedirectFilter(filters []gwapiv1.HTTPRouteFilter) bool {
	for _, f := range filters {
		if f.Type == gwapiv1.HTTPRouteFilterRequestRedirect {
			return true
		}
	}
	return false
}

// ConvertIngress converts a Kubernetes Ingress to Gateway API HTTPRoutes.
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

// hostRuleGroup holds hosts that share the same rules
type hostRuleGroup struct {
	hosts []string
	rules []gwapiv1.HTTPRouteRule
}

// convertToHTTPRoutes generates HTTPRoutes from Ingress rules.
// Hosts with identical rules are consolidated into a single HTTPRoute.
func convertToHTTPRoutes(input ConversionInput, parentRef gwapiv1.ParentReference, filters []gwapiv1.HTTPRouteFilter, warnings []string) ([]*gwapiv1.HTTPRoute, []string) {
	ingress := input.Ingress
	var httpRoutes []*gwapiv1.HTTPRoute

	// Check for use-regex annotation
	useRegex := ingress.Annotations != nil && ingress.Annotations[annotations.UseRegex] == "true"

	// Collect rules per host
	hostRules := make(map[string][]gwapiv1.HTTPRouteRule)
	const noHostKey = ""

	// Check if redirect filters are present - Gateway API does not allow
	// RequestRedirect filters with backendRefs in the same rule
	hasRedirect := hasRedirectFilter(filters)

	for _, rule := range ingress.Spec.Rules {
		host := transformHostname(rule.Host, input.DomainReplace, input.DomainSuffix)
		if rule.HTTP == nil {
			// Rule has host but no HTTP paths - track host for default backend routing
			if host != "" {
				if _, exists := hostRules[host]; !exists {
					hostRules[host] = []gwapiv1.HTTPRouteRule{}
				}
			}
			continue
		}

		for _, path := range rule.HTTP.Paths {
			httpRouteRule, ruleWarnings := convertPathWithServices(path, ingress.Namespace, input.Services, useRegex)
			warnings = append(warnings, ruleWarnings...)

			if len(filters) > 0 {
				httpRouteRule.Filters = append(httpRouteRule.Filters, filters...)
			}

			// Gateway API: RequestRedirect filter must NOT be used with backendRefs
			if hasRedirect {
				httpRouteRule.BackendRefs = nil
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
			// Gateway API: RequestRedirect filter must NOT be used with backendRefs
			if hasRedirect {
				rule.BackendRefs = nil
			}
		}
		defaultRule = rule
		warnings = append(warnings, defaultWarnings...)
	}

	// Group hosts by rule signature (hosts with identical rules go together)
	ruleGroups := groupHostsByRules(hostRules, defaultRule)

	// Generate HTTPRoutes from groups
	for i, group := range ruleGroups {
		routeName := generateRouteName(ingress.Name, group.hosts, i, len(ruleGroups))

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
				Rules: group.rules,
			},
		}

		// Add hostnames if any (empty means catch-all)
		if len(group.hosts) > 0 && group.hosts[0] != noHostKey {
			hostnames := make([]gwapiv1.Hostname, len(group.hosts))
			for j, h := range group.hosts {
				hostnames[j] = gwapiv1.Hostname(h)
			}
			httpRoute.Spec.Hostnames = hostnames
		}

		httpRoutes = append(httpRoutes, httpRoute)
	}

	// Handle case where only default backend exists (no rules)
	if len(ruleGroups) == 0 && defaultRule != nil {
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

// groupHostsByRules consolidates hosts that share identical rules into groups.
// Each group will become a single HTTPRoute with multiple hostnames.
func groupHostsByRules(hostRules map[string][]gwapiv1.HTTPRouteRule, defaultRule *gwapiv1.HTTPRouteRule) []hostRuleGroup {
	const noHostKey = ""

	// Create signature for each host's rules
	signatureToHosts := make(map[string][]string)
	signatureToRules := make(map[string][]gwapiv1.HTTPRouteRule)

	for host, rules := range hostRules {
		// Append default rule if present
		finalRules := rules
		if defaultRule != nil {
			finalRules = append(finalRules, *defaultRule)
		}

		sig := rulesSignature(finalRules)
		signatureToHosts[sig] = append(signatureToHosts[sig], host)
		signatureToRules[sig] = finalRules
	}

	// Sort signatures for deterministic output
	signatures := make([]string, 0, len(signatureToHosts))
	for sig := range signatureToHosts {
		signatures = append(signatures, sig)
	}
	sort.Strings(signatures)

	// Build groups
	var groups []hostRuleGroup
	for _, sig := range signatures {
		hosts := signatureToHosts[sig]
		sort.Strings(hosts) // Sort hosts within group

		// Separate no-host from actual hosts
		var actualHosts []string
		hasNoHost := false
		for _, h := range hosts {
			if h == noHostKey {
				hasNoHost = true
			} else {
				actualHosts = append(actualHosts, h)
			}
		}

		// If we have both no-host and actual hosts, split them
		if hasNoHost && len(actualHosts) > 0 {
			// No-host gets its own group
			groups = append(groups, hostRuleGroup{
				hosts: []string{noHostKey},
				rules: signatureToRules[sig],
			})
			// Actual hosts get their group
			if len(actualHosts) > 0 {
				groups = append(groups, hostRuleGroup{
					hosts: actualHosts,
					rules: signatureToRules[sig],
				})
			}
		} else {
			groups = append(groups, hostRuleGroup{
				hosts: hosts,
				rules: signatureToRules[sig],
			})
		}
	}

	return groups
}

// rulesSignature generates a deterministic signature for a set of rules
func rulesSignature(rules []gwapiv1.HTTPRouteRule) string {
	// Serialize rules to JSON for comparison
	data, err := json.Marshal(rules)
	if err != nil {
		// Fallback to simple hash
		return fmt.Sprintf("rules-%d", len(rules))
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// generateRouteName creates a route name based on Ingress name.
// - Single route: uses Ingress name directly
// - Multiple routes with different rules: uses Ingress name with index suffix
func generateRouteName(ingressName string, _ []string, index, totalGroups int) string {
	// If only one group, use Ingress name directly
	if totalGroups == 1 {
		return ingressName
	}

	// Multiple groups - need suffix
	return fmt.Sprintf("%s-%d", ingressName, index+1)
}

// grpcHostRuleGroup holds hosts that share the same gRPC rules
type grpcHostRuleGroup struct {
	hosts []string
	rules []gwapiv1.GRPCRouteRule
}

// convertToGRPCRoutes generates GRPCRoutes from Ingress rules
func convertToGRPCRoutes(input ConversionInput, parentRef gwapiv1.ParentReference, warnings []string) ([]*gwapiv1.GRPCRoute, []string) {
	ingress := input.Ingress
	var grpcRoutes []*gwapiv1.GRPCRoute

	// Collect rules per host
	hostRules := make(map[string][]gwapiv1.GRPCRouteRule)
	const noHostKey = ""

	for _, rule := range ingress.Spec.Rules {
		host := transformHostname(rule.Host, input.DomainReplace, input.DomainSuffix)
		if rule.HTTP == nil {
			// Rule has host but no HTTP paths - track host for default backend routing
			if host != "" {
				if _, exists := hostRules[host]; !exists {
					hostRules[host] = []gwapiv1.GRPCRouteRule{}
				}
			}
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

	// Group hosts by rule signature
	ruleGroups := groupGRPCHostsByRules(hostRules, defaultRule)

	// Generate GRPCRoutes from groups
	for i, group := range ruleGroups {
		routeName := generateGRPCRouteName(ingress.Name, group.hosts, i, len(ruleGroups))

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
				Rules: group.rules,
			},
		}

		// Add hostnames if any
		if len(group.hosts) > 0 && group.hosts[0] != noHostKey {
			hostnames := make([]gwapiv1.Hostname, len(group.hosts))
			for j, h := range group.hosts {
				hostnames[j] = gwapiv1.Hostname(h)
			}
			grpcRoute.Spec.Hostnames = hostnames
		}

		grpcRoutes = append(grpcRoutes, grpcRoute)
	}

	// Handle case where only default backend exists
	if len(ruleGroups) == 0 && defaultRule != nil {
		grpcRoute := &gwapiv1.GRPCRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ingress.Name,
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

// groupGRPCHostsByRules consolidates hosts that share identical gRPC rules
func groupGRPCHostsByRules(hostRules map[string][]gwapiv1.GRPCRouteRule, defaultRule *gwapiv1.GRPCRouteRule) []grpcHostRuleGroup {
	const noHostKey = ""

	signatureToHosts := make(map[string][]string)
	signatureToRules := make(map[string][]gwapiv1.GRPCRouteRule)

	for host, rules := range hostRules {
		finalRules := rules
		if defaultRule != nil {
			finalRules = append(finalRules, *defaultRule)
		}

		sig := grpcRulesSignature(finalRules)
		signatureToHosts[sig] = append(signatureToHosts[sig], host)
		signatureToRules[sig] = finalRules
	}

	signatures := make([]string, 0, len(signatureToHosts))
	for sig := range signatureToHosts {
		signatures = append(signatures, sig)
	}
	sort.Strings(signatures)

	var groups []grpcHostRuleGroup
	for _, sig := range signatures {
		hosts := signatureToHosts[sig]
		sort.Strings(hosts)

		var actualHosts []string
		hasNoHost := false
		for _, h := range hosts {
			if h == noHostKey {
				hasNoHost = true
			} else {
				actualHosts = append(actualHosts, h)
			}
		}

		if hasNoHost && len(actualHosts) > 0 {
			groups = append(groups, grpcHostRuleGroup{
				hosts: []string{noHostKey},
				rules: signatureToRules[sig],
			})
			if len(actualHosts) > 0 {
				groups = append(groups, grpcHostRuleGroup{
					hosts: actualHosts,
					rules: signatureToRules[sig],
				})
			}
		} else {
			groups = append(groups, grpcHostRuleGroup{
				hosts: hosts,
				rules: signatureToRules[sig],
			})
		}
	}

	return groups
}

// grpcRulesSignature generates a deterministic signature for gRPC rules
func grpcRulesSignature(rules []gwapiv1.GRPCRouteRule) string {
	data, err := json.Marshal(rules)
	if err != nil {
		return fmt.Sprintf("grpc-rules-%d", len(rules))
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// generateGRPCRouteName creates a gRPC route name
func generateGRPCRouteName(ingressName string, _ []string, index, totalGroups int) string {
	if totalGroups == 1 {
		return ingressName
	}
	return fmt.Sprintf("%s-%d", ingressName, index+1)
}

// convertPathToGRPCRule converts an Ingress path to a GRPCRouteRule
func convertPathToGRPCRule(path networkingv1.HTTPIngressPath, namespace string, services map[types.NamespacedName]*corev1.Service) (gwapiv1.GRPCRouteRule, []string) {
	var warnings []string
	rule := gwapiv1.GRPCRouteRule{}

	// For gRPC, path-based matching is limited
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

	// Set port with Service lookup fallback
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
		if len(tls.Hosts) > 0 && tls.SecretName == "" {
			warnings = append(warnings,
				fmt.Sprintf("TLS config has hosts %v but no secretName; Gateway will need manual certificate configuration or cert-manager",
					tls.Hosts))
		}

		for _, host := range tls.Hosts {
			listener := TLSListener{
				Hostname:        gwapiv1.Hostname(transformHostname(host, domainReplace, domainSuffix)),
				SecretName:      tls.SecretName,
				SecretNamespace: namespace,
			}
			listeners = append(listeners, listener)
		}

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

func buildParentRef(gatewayName, gatewayNamespace, routeNamespace string) gwapiv1.ParentReference {
	ref := gwapiv1.ParentReference{
		Name: gwapiv1.ObjectName(gatewayName),
	}

	if gatewayNamespace != "" && gatewayNamespace != routeNamespace {
		ns := gwapiv1.Namespace(gatewayNamespace)
		ref.Namespace = &ns
	}

	return ref
}

func convertPathWithServices(path networkingv1.HTTPIngressPath, namespace string, services map[types.NamespacedName]*corev1.Service, useRegex bool) (gwapiv1.HTTPRouteRule, []string) {
	var warnings []string
	rule := gwapiv1.HTTPRouteRule{}

	// Convert path match
	match := gwapiv1.HTTPRouteMatch{}
	if path.Path != "" {
		pathMatch := gwapiv1.HTTPPathMatch{
			Value: &path.Path,
		}

		pathType := convertPathTypeWithRegex(path.PathType, useRegex)
		pathMatch.Type = &pathType

		if path.PathType != nil && *path.PathType == networkingv1.PathTypeImplementationSpecific && !useRegex {
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
		return gwapiv1.PathMatchPathPrefix
	default:
		return gwapiv1.PathMatchPathPrefix
	}
}

// convertPathTypeWithRegex converts Ingress PathType to Gateway API PathMatchType.
// If useRegex is true, returns RegularExpression regardless of the PathType.
func convertPathTypeWithRegex(pathType *networkingv1.PathType, useRegex bool) gwapiv1.PathMatchType {
	if useRegex {
		return gwapiv1.PathMatchRegularExpression
	}
	return convertPathType(pathType)
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

	switch {
	case svc.Port.Number != 0:
		port := svc.Port.Number
		if port < 1 || port > 65535 {
			warnings = append(warnings, fmt.Sprintf("service %q has invalid port number %d (must be 1-65535)", svc.Name, port))
		}
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
			warnings = append(warnings, fmt.Sprintf("service %q uses named port %q which cannot be resolved; specify port number instead", svc.Name, svc.Port.Name))
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
func transformHostname(host, replace, suffix string) string {
	if replace == "" || suffix == "" {
		return host
	}

	wildcardPrefix := ""
	checkHost := host
	if strings.HasPrefix(host, "*.") {
		wildcardPrefix = "*."
		checkHost = host[2:]
	}

	if checkHost == replace {
		return wildcardPrefix + suffix
	}

	if strings.HasSuffix(checkHost, "."+replace) {
		prefix := strings.TrimSuffix(checkHost, "."+replace)
		return wildcardPrefix + prefix + "." + suffix
	}

	return host
}
