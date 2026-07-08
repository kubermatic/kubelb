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

package ingress

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8c.io/kubelb/pkg/conversion"
	"k8c.io/kubelb/pkg/conversion/annotations"
	"k8c.io/kubelb/pkg/conversion/policies"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/yaml"
)

const (
	managedByValue              = "kubelb-cli"
	routeAcceptanceTimeout      = 30 * time.Second
	routeAcceptancePollInterval = 2 * time.Second
)

// ConvertOptions holds options for the convert command
type ConvertOptions struct {
	OutputDir             string
	DryRun                bool
	Names                 []string // Specific ingresses to convert (empty = all)
	Namespace             string   // Filter by namespace (empty = all namespaces)
	SkipAcceptancePolling bool     // Skip waiting for route acceptance (for UI/async use)
}

// ConvertResult tracks conversion outcomes
type ConvertResult struct {
	Converted      []string
	Skipped        []string
	Failed         map[string]error
	GatewayCreated bool
}

// Convert performs the conversion of Ingress resources.
// For apply mode, it orchestrates the full conversion workflow.
// For dry-run and file output, it generates preview YAML.
func Convert(ctx context.Context, k8sClient client.Client, opts *conversion.Options, convertOpts ConvertOptions) (*ConvertResult, error) {
	ingresses, err := buildIngressList(ctx, k8sClient, convertOpts)
	if err != nil {
		return nil, err
	}

	if convertOpts.OutputDir != "" {
		return writeConversionsToFiles(ctx, k8sClient, opts, ingresses, convertOpts.OutputDir)
	}

	if convertOpts.DryRun {
		return printConversionsDryRun(ctx, k8sClient, opts, ingresses)
	}

	return applyConversions(ctx, k8sClient, opts, ingresses, convertOpts.SkipAcceptancePolling)
}

// ingressConversion holds the conversion result for a single ingress
type ingressConversion struct {
	ingress      *networkingv1.Ingress
	convResult   conversion.Result
	policyResult *policyConversionResult
}

// applyConversions performs the full conversion and apply workflow
func applyConversions(ctx context.Context, k8sClient client.Client, opts *conversion.Options, ingressRefs []types.NamespacedName, skipPolling bool) (*ConvertResult, error) {
	result := &ConvertResult{
		Failed: make(map[string]error),
	}

	// Phase 1: Convert all ingresses
	conversions, allTLSListeners := convertIngresses(ctx, k8sClient, opts, ingressRefs, result)

	// Phase 2: Apply Gateway with all TLS listeners
	if len(conversions) > 0 {
		if err := applyGatewayAndSecrets(ctx, k8sClient, opts, allTLSListeners, result); err != nil {
			return result, err
		}
	}

	// Phase 3: Apply routes and policies for each ingress
	applyRoutesAndPolicies(ctx, k8sClient, conversions, skipPolling, result)

	return result, nil
}

// convertIngresses converts all ingresses and collects TLS listeners
func convertIngresses(ctx context.Context, k8sClient client.Client, opts *conversion.Options, ingressRefs []types.NamespacedName, result *ConvertResult) ([]ingressConversion, []conversion.TLSListener) {
	var conversions []ingressConversion
	var allTLSListeners []conversion.TLSListener

	for _, ref := range ingressRefs {
		key := ref.Namespace + "/" + ref.Name

		var ing networkingv1.Ingress
		if err := k8sClient.Get(ctx, ref, &ing); err != nil {
			if apierrors.IsNotFound(err) {
				result.Failed[key] = fmt.Errorf("ingress not found")
			} else {
				result.Failed[key] = err
			}
			continue
		}

		if conversion.ShouldSkip(&ing) {
			result.Skipped = append(result.Skipped, key)
			fmt.Printf("Skipping %s (marked for skip)\n", key)
			continue
		}

		if opts.IngressClass != "" && !conversion.MatchesIngressClass(&ing, opts.IngressClass) {
			result.Skipped = append(result.Skipped, key)
			continue
		}

		services, err := conversion.FetchServicesForIngress(ctx, k8sClient, &ing)
		if err != nil {
			result.Failed[key] = fmt.Errorf("failed to fetch services: %w", err)
			continue
		}

		convResult := conversion.ConvertIngressWithServices(conversion.Input{
			Ingress:            &ing,
			GatewayName:        opts.GatewayName,
			GatewayNamespace:   opts.GatewayNamespace,
			Services:           services,
			DomainReplace:      opts.DomainReplace,
			DomainSuffix:       opts.DomainSuffix,
			SkipPolicyWarnings: !opts.DisableEnvoyGatewayFeatures,
		})

		allTLSListeners = append(allTLSListeners, convResult.TLSListeners...)

		var policyResult *policyConversionResult
		if !opts.DisableEnvoyGatewayFeatures && len(convResult.HTTPRoutes) > 0 {
			policyResult = buildPoliciesFromAnnotations(&ing, convResult.HTTPRoutes[0].Name)
		}

		conversions = append(conversions, ingressConversion{
			ingress:      &ing,
			convResult:   convResult,
			policyResult: policyResult,
		})
	}

	return conversions, allTLSListeners
}

// applyGatewayAndSecrets applies the Gateway and copies TLS secrets if needed
func applyGatewayAndSecrets(ctx context.Context, k8sClient client.Client, opts *conversion.Options, allTLSListeners []conversion.TLSListener, result *ConvertResult) error {
	gw, created, err := applyGateway(ctx, k8sClient, opts, allTLSListeners, managedByValue)
	if err != nil {
		return fmt.Errorf("failed to apply Gateway: %w", err)
	}
	result.GatewayCreated = created
	if created {
		fmt.Printf("Created Gateway %s/%s\n", gw.Namespace, gw.Name)
	} else {
		fmt.Printf("Updated Gateway %s/%s\n", gw.Namespace, gw.Name)
	}

	if opts.CopyTLSSecrets {
		copyTLSSecrets(ctx, k8sClient, allTLSListeners, opts.GatewayNamespace)
	}
	return nil
}

// copyTLSSecrets copies TLS secrets to the gateway namespace
func copyTLSSecrets(ctx context.Context, k8sClient client.Client, listeners []conversion.TLSListener, gatewayNS string) {
	for _, tls := range listeners {
		if tls.SourceSecretNamespace != gatewayNS && tls.SourceSecretName != "" {
			if err := conversion.CopyTLSSecretWithManagedBy(ctx, k8sClient, tls.SourceSecretNamespace, tls.SourceSecretName, gatewayNS, managedByValue); err != nil {
				fmt.Printf("Warning: failed to copy TLS secret %s/%s: %v\n", tls.SourceSecretNamespace, tls.SourceSecretName, err)
			} else {
				fmt.Printf("Copied TLS secret %s/%s to %s\n", tls.SourceSecretNamespace, tls.SourceSecretName, gatewayNS)
			}
		}
	}
}

// applyRoutesAndPolicies applies routes and policies for each converted ingress
func applyRoutesAndPolicies(ctx context.Context, k8sClient client.Client, conversions []ingressConversion, skipPolling bool, result *ConvertResult) {
	for _, conv := range conversions {
		key := conv.ingress.Namespace + "/" + conv.ingress.Name

		applyRoutes(ctx, k8sClient, conv, key, result)
		applyPolicies(ctx, k8sClient, conv.policyResult)

		for _, w := range conv.convResult.Warnings {
			fmt.Printf("Warning [%s]: %s\n", key, w)
		}

		accepted := true
		if !skipPolling {
			accepted = pollRouteAcceptance(ctx, k8sClient, conv.convResult.HTTPRoutes, conv.convResult.GRPCRoutes)
		}

		if err := updateIngressStatus(ctx, k8sClient, conv.ingress, conv.convResult, accepted); err != nil {
			fmt.Printf("Warning: failed to update ingress status: %v\n", err)
		}

		if _, hasFailed := result.Failed[key]; !hasFailed {
			result.Converted = append(result.Converted, key)
		}
	}
}

// applyRoutes applies HTTP and gRPC routes for a conversion
func applyRoutes(ctx context.Context, k8sClient client.Client, conv ingressConversion, key string, result *ConvertResult) {
	for _, route := range conv.convResult.HTTPRoutes {
		route.Labels = ensureManagedByLabel(route.Labels, managedByValue)
		if err := applyHTTPRoute(ctx, k8sClient, route); err != nil {
			result.Failed[key] = fmt.Errorf("failed to apply HTTPRoute %s: %w", route.Name, err)
			continue
		}
		fmt.Printf("Applied HTTPRoute %s/%s\n", route.Namespace, route.Name)
	}

	for _, route := range conv.convResult.GRPCRoutes {
		route.Labels = ensureManagedByLabel(route.Labels, managedByValue)
		if err := applyGRPCRoute(ctx, k8sClient, route); err != nil {
			result.Failed[key] = fmt.Errorf("failed to apply GRPCRoute %s: %w", route.Name, err)
			continue
		}
		fmt.Printf("Applied GRPCRoute %s/%s\n", route.Namespace, route.Name)
	}
}

// applyPolicies applies security and backend traffic policies
func applyPolicies(ctx context.Context, k8sClient client.Client, policyResult *policyConversionResult) {
	if policyResult == nil {
		return
	}

	if policyResult.securityPolicy != nil {
		if err := applyPolicy(ctx, k8sClient, policyResult.securityPolicy); err != nil {
			fmt.Printf("Warning: failed to apply SecurityPolicy: %v\n", err)
		} else {
			fmt.Printf("Applied SecurityPolicy %s/%s\n", policyResult.securityPolicy.GetNamespace(), policyResult.securityPolicy.GetName())
		}
	}

	if policyResult.backendTrafficPolicy != nil {
		if err := applyPolicy(ctx, k8sClient, policyResult.backendTrafficPolicy); err != nil {
			fmt.Printf("Warning: failed to apply BackendTrafficPolicy: %v\n", err)
		} else {
			fmt.Printf("Applied BackendTrafficPolicy %s/%s\n", policyResult.backendTrafficPolicy.GetNamespace(), policyResult.backendTrafficPolicy.GetName())
		}
	}
}

// policyConversionResult holds converted policies
type policyConversionResult struct {
	securityPolicy       client.Object
	backendTrafficPolicy client.Object
}

// buildPoliciesFromAnnotations creates EG policies from ingress annotations
func buildPoliciesFromAnnotations(ing *networkingv1.Ingress, routeName string) *policyConversionResult {
	if ing.Annotations == nil {
		return nil
	}

	result := &policyConversionResult{}
	base := policies.NewPolicyBuilder(ing.Name, ing.Namespace, routeName)

	// Build SecurityPolicy
	secBuilder := policies.NewSecurityPolicyBuilder(base)

	// CORS
	if ing.Annotations[annotations.EnableCORS] == "true" {
		secBuilder.SetCORS(
			policies.ParseStringList(ing.Annotations[annotations.CORSAllowOrigin]),
			policies.ParseStringList(ing.Annotations[annotations.CORSAllowMethods]),
			policies.ParseStringList(ing.Annotations[annotations.CORSAllowHeaders]),
			policies.ParseStringList(ing.Annotations[annotations.CORSExposeHeaders]),
			policies.ParseInt64(ing.Annotations[annotations.CORSMaxAge]),
			policies.ParseBool(ing.Annotations[annotations.CORSAllowCredentials]),
		)
	}

	// IP allowlist
	if cidrs := ing.Annotations[annotations.WhitelistSourceRange]; cidrs != "" {
		secBuilder.SetIPAllowlist(policies.ParseCIDRList(cidrs))
	}

	// IP denylist
	if cidrs := ing.Annotations[annotations.DenylistSourceRange]; cidrs != "" {
		secBuilder.SetIPDenylist(policies.ParseCIDRList(cidrs))
	}

	// Basic auth
	if secretRef := ing.Annotations[annotations.AuthSecret]; secretRef != "" {
		secBuilder.SetBasicAuth(secretRef, ing.Namespace)
	}

	if !secBuilder.IsEmpty() {
		result.securityPolicy = secBuilder.Build()
	}

	// Build BackendTrafficPolicy
	btpBuilder := policies.NewBackendTrafficPolicyBuilder(base)

	// Timeouts
	if timeout := ing.Annotations[annotations.ProxyConnectTimeout]; timeout != "" {
		btpBuilder.SetConnectTimeout(timeout)
	}
	if timeout := ing.Annotations[annotations.ProxyReadTimeout]; timeout != "" {
		btpBuilder.SetRequestTimeout(timeout)
	}

	// Rate limiting
	if rps := policies.ParseInt(ing.Annotations[annotations.LimitRPS]); rps > 0 {
		btpBuilder.SetRateLimitRPS(rps)
	} else if rpm := policies.ParseInt(ing.Annotations[annotations.LimitRPM]); rpm > 0 {
		btpBuilder.SetRateLimitRPM(rpm)
	}

	// Max connections
	if maxConns := policies.ParseInt(ing.Annotations[annotations.LimitConnections]); maxConns > 0 {
		btpBuilder.SetMaxConnections(maxConns)
	}

	if !btpBuilder.IsEmpty() {
		result.backendTrafficPolicy = btpBuilder.Build()
	}

	if result.securityPolicy == nil && result.backendTrafficPolicy == nil {
		return nil
	}
	return result
}

// applyHTTPRoute creates or updates an HTTPRoute
func applyHTTPRoute(ctx context.Context, k8sClient client.Client, route *gwapiv1.HTTPRoute) error {
	existing := &gwapiv1.HTTPRoute{}
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: route.Namespace, Name: route.Name}, existing)
	if apierrors.IsNotFound(err) {
		return k8sClient.Create(ctx, route)
	}
	if err != nil {
		return err
	}

	existing.Spec = route.Spec
	existing.Labels = route.Labels
	return k8sClient.Update(ctx, existing)
}

// applyGRPCRoute creates or updates a GRPCRoute
func applyGRPCRoute(ctx context.Context, k8sClient client.Client, route *gwapiv1.GRPCRoute) error {
	existing := &gwapiv1.GRPCRoute{}
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: route.Namespace, Name: route.Name}, existing)
	if apierrors.IsNotFound(err) {
		return k8sClient.Create(ctx, route)
	}
	if err != nil {
		return err
	}

	existing.Spec = route.Spec
	existing.Labels = route.Labels
	return k8sClient.Update(ctx, existing)
}

// applyPolicy creates or updates a policy resource
func applyPolicy(ctx context.Context, k8sClient client.Client, policy client.Object) error {
	existing := policy.DeepCopyObject().(client.Object)
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: policy.GetNamespace(), Name: policy.GetName()}, existing)
	if apierrors.IsNotFound(err) {
		return k8sClient.Create(ctx, policy)
	}
	if err != nil {
		return err
	}

	// For updates, we need to preserve the resource version
	policy.SetResourceVersion(existing.GetResourceVersion())
	return k8sClient.Update(ctx, policy)
}

// pollRouteAcceptance waits for routes to be accepted by the Gateway
func pollRouteAcceptance(ctx context.Context, k8sClient client.Client, httpRoutes []*gwapiv1.HTTPRoute, grpcRoutes []*gwapiv1.GRPCRoute) bool {
	if len(httpRoutes) == 0 && len(grpcRoutes) == 0 {
		return true
	}

	deadline := time.Now().Add(routeAcceptanceTimeout)
	ticker := time.NewTicker(routeAcceptancePollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if time.Now().After(deadline) {
				fmt.Println("Warning: route acceptance check timed out")
				return false
			}

			allAccepted := true

			// Check HTTPRoutes
			for _, route := range httpRoutes {
				var current gwapiv1.HTTPRoute
				if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: route.Namespace, Name: route.Name}, &current); err != nil {
					allAccepted = false
					continue
				}
				if !isRouteAccepted(current.Status.Parents) {
					allAccepted = false
				}
			}

			// Check GRPCRoutes
			for _, route := range grpcRoutes {
				var current gwapiv1.GRPCRoute
				if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: route.Namespace, Name: route.Name}, &current); err != nil {
					allAccepted = false
					continue
				}
				if !isRouteAccepted(current.Status.Parents) {
					allAccepted = false
				}
			}

			if allAccepted {
				return true
			}
		}
	}
}

// isRouteAccepted checks if a route has been accepted by at least one parent
func isRouteAccepted(parents []gwapiv1.RouteParentStatus) bool {
	for _, parent := range parents {
		for _, cond := range parent.Conditions {
			if cond.Type == "Accepted" && cond.Status == metav1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

// updateIngressStatus updates the ingress with conversion status annotations
func updateIngressStatus(ctx context.Context, k8sClient client.Client, ing *networkingv1.Ingress, convResult conversion.Result, accepted bool) error {
	if ing.Annotations == nil {
		ing.Annotations = make(map[string]string)
	}

	// Set status
	status := conversion.ConversionStatusConverted
	if !accepted {
		status = conversion.ConversionStatusPending
	}
	if len(convResult.Warnings) > 0 && accepted {
		status = conversion.ConversionStatusPartial
	}
	ing.Annotations[conversion.AnnotationConversionStatus] = status

	// Set converted routes
	var httpRouteNames, grpcRouteNames []string
	for _, route := range convResult.HTTPRoutes {
		httpRouteNames = append(httpRouteNames, route.Name)
	}
	for _, route := range convResult.GRPCRoutes {
		grpcRouteNames = append(grpcRouteNames, route.Name)
	}
	if len(httpRouteNames) > 0 {
		ing.Annotations[conversion.AnnotationConvertedHTTPRoute] = strings.Join(httpRouteNames, ",")
	}
	if len(grpcRouteNames) > 0 {
		ing.Annotations[conversion.AnnotationConvertedGRPCRoute] = strings.Join(grpcRouteNames, ",")
	}

	// Set warnings
	if len(convResult.Warnings) > 0 {
		ing.Annotations[conversion.AnnotationConversionWarnings] = strings.Join(convResult.Warnings, ";")
	}

	return k8sClient.Update(ctx, ing)
}

// ensureManagedByLabel adds managed-by label if not present
func ensureManagedByLabel(labels map[string]string, managedBy string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[conversion.LabelManagedBy] = managedBy
	return labels
}

// buildIngressList creates the list of ingresses to convert
func buildIngressList(ctx context.Context, k8sClient client.Client, convertOpts ConvertOptions) ([]types.NamespacedName, error) {
	if len(convertOpts.Names) > 0 {
		// Convert specific ingresses by name
		var result []types.NamespacedName
		for _, name := range convertOpts.Names {
			ns, n := parseIngressName(name, convertOpts.Namespace)
			result = append(result, types.NamespacedName{Namespace: ns, Name: n})
		}
		return result, nil
	}

	// List all ingresses
	var ingresses networkingv1.IngressList
	listOpts := []client.ListOption{}
	if convertOpts.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(convertOpts.Namespace))
	}
	if err := k8sClient.List(ctx, &ingresses, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list ingresses: %w", err)
	}

	var result []types.NamespacedName
	for _, ing := range ingresses.Items {
		result = append(result, types.NamespacedName{Namespace: ing.Namespace, Name: ing.Name})
	}
	return result, nil
}

// parseIngressName parses an ingress name which may include namespace (ns/name format)
func parseIngressName(name, defaultNS string) (namespace, ingressName string) {
	if strings.Contains(name, "/") {
		parts := strings.SplitN(name, "/", 2)
		return parts[0], parts[1]
	}
	return defaultNS, name
}

// writeConversionsToFiles generates preview and writes YAML to files
func writeConversionsToFiles(ctx context.Context, k8sClient client.Client, opts *conversion.Options, ingresses []types.NamespacedName, outputDir string) (*ConvertResult, error) {
	result := &ConvertResult{
		Failed: make(map[string]error),
	}

	// Generate preview for all ingresses
	previewResult, err := generatePreview(ctx, k8sClient, opts, ingresses)
	if err != nil {
		return nil, err
	}

	// Write Gateway
	if previewResult.Gateway != nil {
		gwDir := filepath.Join(outputDir, previewResult.Gateway.Namespace)
		if err := os.MkdirAll(gwDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create gateway directory: %w", err)
		}

		gwData, err := yaml.Marshal(previewResult.Gateway)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Gateway: %w", err)
		}
		gwFilename := filepath.Join(gwDir, fmt.Sprintf("%s-gateway.yaml", previewResult.Gateway.Name))
		if err := os.WriteFile(gwFilename, gwData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write Gateway file: %w", err)
		}
		fmt.Printf("Wrote %s\n", gwFilename)
		result.GatewayCreated = true
	}

	// Write HTTPRoutes
	for _, route := range previewResult.HTTPRoutes {
		dir := filepath.Join(outputDir, route.Namespace)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}

		route.APIVersion = gatewayAPIVersion
		route.Kind = kindHTTPRoute
		data, err := yaml.Marshal(route)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal HTTPRoute: %w", err)
		}
		filename := filepath.Join(dir, fmt.Sprintf("%s-httproute.yaml", route.Name))
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
		fmt.Printf("Wrote %s\n", filename)
	}

	// Write GRPCRoutes
	for _, route := range previewResult.GRPCRoutes {
		dir := filepath.Join(outputDir, route.Namespace)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}

		route.APIVersion = gatewayAPIVersion
		route.Kind = kindGRPCRoute
		data, err := yaml.Marshal(route)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal GRPCRoute: %w", err)
		}
		filename := filepath.Join(dir, fmt.Sprintf("%s-grpcroute.yaml", route.Name))
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
		fmt.Printf("Wrote %s\n", filename)
	}

	// Write Policies
	for _, policy := range previewResult.Policies {
		obj, ok := policy.(interface {
			GetName() string
			GetNamespace() string
		})
		if !ok {
			continue
		}

		dir := filepath.Join(outputDir, obj.GetNamespace())
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}

		data, err := yaml.Marshal(policy)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal policy: %w", err)
		}
		filename := filepath.Join(dir, fmt.Sprintf("%s-policy.yaml", obj.GetName()))
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
		fmt.Printf("Wrote %s\n", filename)
	}

	// Track only actually-converted ingresses (excludes skipped/filtered)
	result.Converted = previewResult.Converted

	// Print warnings
	for _, w := range previewResult.Warnings {
		fmt.Printf("Warning: %s\n", w)
	}

	return result, nil
}

// printConversionsDryRun generates preview and prints YAML to stdout
func printConversionsDryRun(ctx context.Context, k8sClient client.Client, opts *conversion.Options, ingresses []types.NamespacedName) (*ConvertResult, error) {
	result := &ConvertResult{
		Failed: make(map[string]error),
	}

	// Generate preview for all ingresses
	previewResult, err := generatePreview(ctx, k8sClient, opts, ingresses)
	if err != nil {
		return nil, err
	}

	// Print warnings first
	if len(previewResult.Warnings) > 0 {
		fmt.Println("# Warnings:")
		for _, w := range previewResult.Warnings {
			fmt.Printf("#   - %s\n", w)
		}
	}

	// Print Gateway
	if previewResult.Gateway != nil {
		gwData, err := yaml.Marshal(previewResult.Gateway)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Gateway: %w", err)
		}
		fmt.Println("---")
		fmt.Print(string(gwData))
		result.GatewayCreated = true
	}

	// Print HTTPRoutes
	for _, route := range previewResult.HTTPRoutes {
		route.APIVersion = gatewayAPIVersion
		route.Kind = kindHTTPRoute
		data, err := yaml.Marshal(route)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal HTTPRoute: %w", err)
		}
		fmt.Println("---")
		fmt.Print(string(data))
	}

	// Print GRPCRoutes
	for _, route := range previewResult.GRPCRoutes {
		route.APIVersion = gatewayAPIVersion
		route.Kind = kindGRPCRoute
		data, err := yaml.Marshal(route)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal GRPCRoute: %w", err)
		}
		fmt.Println("---")
		fmt.Print(string(data))
	}

	// Print Policies
	for _, policy := range previewResult.Policies {
		data, err := yaml.Marshal(policy)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal policy: %w", err)
		}
		fmt.Println("---")
		fmt.Print(string(data))
	}

	// Track only actually-converted ingresses (excludes skipped/filtered)
	result.Converted = previewResult.Converted

	return result, nil
}

// previewOutput holds the generated resources for preview
type previewOutput struct {
	Gateway    *gwapiv1.Gateway
	HTTPRoutes []*gwapiv1.HTTPRoute
	GRPCRoutes []*gwapiv1.GRPCRoute
	Policies   []client.Object
	Warnings   []string
	Converted  []string // ingresses that were actually converted (ns/name)
}

// generatePreview converts ingresses and returns the generated resources
func generatePreview(ctx context.Context, k8sClient client.Client, opts *conversion.Options, ingresses []types.NamespacedName) (*previewOutput, error) {
	output := &previewOutput{}

	var allTLSListeners []conversion.TLSListener

	for _, ref := range ingresses {
		// Get ingress
		var ing networkingv1.Ingress
		if err := k8sClient.Get(ctx, ref, &ing); err != nil {
			output.Warnings = append(output.Warnings, fmt.Sprintf("failed to get ingress %s/%s: %v", ref.Namespace, ref.Name, err))
			continue
		}

		// Check if should skip
		if conversion.ShouldSkip(&ing) {
			output.Warnings = append(output.Warnings, fmt.Sprintf("skipping %s/%s (marked for skip)", ref.Namespace, ref.Name))
			continue
		}

		// Check ingress class filter
		if opts.IngressClass != "" && !conversion.MatchesIngressClass(&ing, opts.IngressClass) {
			continue
		}

		// Fetch services for port resolution
		services, _ := conversion.FetchServicesForIngress(ctx, k8sClient, &ing)

		// Convert ingress
		convResult := conversion.ConvertIngressWithServices(conversion.Input{
			Ingress:            &ing,
			GatewayName:        opts.GatewayName,
			GatewayNamespace:   opts.GatewayNamespace,
			Services:           services,
			DomainReplace:      opts.DomainReplace,
			DomainSuffix:       opts.DomainSuffix,
			SkipPolicyWarnings: !opts.DisableEnvoyGatewayFeatures,
		})

		// Track this ingress as actually converted
		output.Converted = append(output.Converted, ref.Namespace+"/"+ref.Name)

		// Collect results
		output.HTTPRoutes = append(output.HTTPRoutes, convResult.HTTPRoutes...)
		output.GRPCRoutes = append(output.GRPCRoutes, convResult.GRPCRoutes...)
		allTLSListeners = append(allTLSListeners, convResult.TLSListeners...)
		output.Warnings = append(output.Warnings, convResult.Warnings...)

		// Build policies if enabled
		if !opts.DisableEnvoyGatewayFeatures && len(convResult.HTTPRoutes) > 0 {
			policyResult := buildPoliciesFromAnnotations(&ing, convResult.HTTPRoutes[0].Name)
			if policyResult != nil {
				if policyResult.securityPolicy != nil {
					output.Policies = append(output.Policies, policyResult.securityPolicy)
				}
				if policyResult.backendTrafficPolicy != nil {
					output.Policies = append(output.Policies, policyResult.backendTrafficPolicy)
				}
			}
		}
	}

	// Build Gateway if we have listeners
	if len(allTLSListeners) > 0 || len(output.HTTPRoutes) > 0 || len(output.GRPCRoutes) > 0 {
		output.Gateway = conversion.BuildGateway(conversion.GatewayConfig{
			Name:         opts.GatewayName,
			Namespace:    opts.GatewayNamespace,
			ClassName:    opts.GatewayClassName,
			TLSListeners: allTLSListeners,
			Annotations:  opts.GatewayAnnotations,
			ManagedBy:    managedByValue,
		})
	}

	return output, nil
}

// PrintResult displays the conversion result summary
func PrintResult(result *ConvertResult) {
	if len(result.Converted) > 0 {
		fmt.Printf("\nConverted: %d\n", len(result.Converted))
		for _, name := range result.Converted {
			fmt.Printf("  - %s\n", name)
		}
	}

	if len(result.Skipped) > 0 {
		fmt.Printf("\nSkipped: %d\n", len(result.Skipped))
		for _, name := range result.Skipped {
			fmt.Printf("  - %s\n", name)
		}
	}

	if len(result.Failed) > 0 {
		fmt.Printf("\nFailed: %d\n", len(result.Failed))
		for name, err := range result.Failed {
			fmt.Printf("  - %s: %v\n", name, err)
		}
	}
}
