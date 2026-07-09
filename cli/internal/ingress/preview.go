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

	"k8c.io/kubelb/pkg/conversion"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	gatewayAPIVersion = "gateway.networking.k8s.io/v1"
	kindHTTPRoute     = "HTTPRoute"
	kindGRPCRoute     = "GRPCRoute"
)

// PreviewResult contains the YAML output and warnings
type PreviewResult struct {
	OriginalYAML  string
	ConvertedYAML string
	Warnings      []string
}

// cleanIngress returns a cleaned copy of an Ingress (no managed fields, no last-applied annotation, with API version/kind set)
func cleanIngress(ing *networkingv1.Ingress) *networkingv1.Ingress {
	ingCopy := ing.DeepCopy()
	ingCopy.ManagedFields = nil
	ingCopy.APIVersion = "networking.k8s.io/v1"
	ingCopy.Kind = "Ingress"
	if ingCopy.Annotations != nil {
		delete(ingCopy.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
	}
	return ingCopy
}

// Get fetches an ingress, cleans it, and prints the YAML
func Get(ctx context.Context, k8sClient client.Client, namespace, name string) error {
	var ing networkingv1.Ingress
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &ing); err != nil {
		return fmt.Errorf("failed to get ingress %s/%s: %w", namespace, name, err)
	}
	cleaned := cleanIngress(&ing)
	data, err := yaml.Marshal(cleaned)
	if err != nil {
		return fmt.Errorf("failed to marshal ingress: %w", err)
	}
	fmt.Print(string(data))
	return nil
}

// Preview converts an Ingress and returns the generated YAML
func Preview(ctx context.Context, k8sClient client.Client, namespace, name string, opts *conversion.Options) (*PreviewResult, error) {
	var ing networkingv1.Ingress
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &ing); err != nil {
		return nil, fmt.Errorf("failed to get ingress %s/%s: %w", namespace, name, err)
	}

	if conversion.ShouldSkip(&ing) {
		return nil, fmt.Errorf("ingress %s/%s is marked to skip conversion", namespace, name)
	}

	cleaned := cleanIngress(&ing)
	originalYAML, err := yaml.Marshal(cleaned)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal original Ingress: %w", err)
	}

	previewResult, err := generatePreview(ctx, k8sClient, opts, []types.NamespacedName{{Namespace: namespace, Name: name}})
	if err != nil {
		return nil, err
	}

	var convertedYAML string

	if previewResult.Gateway != nil {
		gwYAML, err := yaml.Marshal(previewResult.Gateway)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Gateway: %w", err)
		}
		convertedYAML += "---\n" + string(gwYAML)
	}

	for _, route := range previewResult.HTTPRoutes {
		route.APIVersion = gatewayAPIVersion
		route.Kind = kindHTTPRoute
		routeYAML, err := yaml.Marshal(route)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal HTTPRoute: %w", err)
		}
		convertedYAML += "---\n" + string(routeYAML)
	}

	for _, route := range previewResult.GRPCRoutes {
		route.APIVersion = gatewayAPIVersion
		route.Kind = kindGRPCRoute
		routeYAML, err := yaml.Marshal(route)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal GRPCRoute: %w", err)
		}
		convertedYAML += "---\n" + string(routeYAML)
	}

	for _, policy := range previewResult.Policies {
		policyYAML, err := yaml.Marshal(policy)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal policy: %w", err)
		}
		convertedYAML += "---\n" + string(policyYAML)
	}

	return &PreviewResult{
		OriginalYAML:  string(originalYAML),
		ConvertedYAML: convertedYAML,
		Warnings:      previewResult.Warnings,
	}, nil
}

// BatchPreviewInput represents an ingress to preview
type BatchPreviewInput struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// BatchPreviewResult contains combined YAML for multiple ingresses
type BatchPreviewResult struct {
	ConvertedYAML string
	Warnings      []string
}

// BatchPreview converts multiple Ingresses and returns combined YAML with a single Gateway
func BatchPreview(ctx context.Context, k8sClient client.Client, opts *conversion.Options, ingresses []BatchPreviewInput) (*BatchPreviewResult, error) {
	if len(ingresses) == 0 {
		return nil, fmt.Errorf("no ingresses specified")
	}

	var refs []types.NamespacedName
	for _, ing := range ingresses {
		refs = append(refs, types.NamespacedName{Namespace: ing.Namespace, Name: ing.Name})
	}

	previewResult, err := generatePreview(ctx, k8sClient, opts, refs)
	if err != nil {
		return nil, err
	}

	var convertedYAML string

	if previewResult.Gateway != nil {
		gwYAML, err := yaml.Marshal(previewResult.Gateway)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Gateway: %w", err)
		}
		convertedYAML += "---\n" + string(gwYAML)
	}

	for _, route := range previewResult.HTTPRoutes {
		route.APIVersion = gatewayAPIVersion
		route.Kind = kindHTTPRoute
		routeYAML, err := yaml.Marshal(route)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal HTTPRoute: %w", err)
		}
		convertedYAML += "---\n" + string(routeYAML)
	}

	for _, route := range previewResult.GRPCRoutes {
		route.APIVersion = gatewayAPIVersion
		route.Kind = kindGRPCRoute
		routeYAML, err := yaml.Marshal(route)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal GRPCRoute: %w", err)
		}
		convertedYAML += "---\n" + string(routeYAML)
	}

	for _, policy := range previewResult.Policies {
		policyYAML, err := yaml.Marshal(policy)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal policy: %w", err)
		}
		convertedYAML += "---\n" + string(policyYAML)
	}

	return &BatchPreviewResult{
		ConvertedYAML: convertedYAML,
		Warnings:      previewResult.Warnings,
	}, nil
}

// PrintPreview displays the preview result (warnings + converted YAML only)
func PrintPreview(result *PreviewResult) {
	printWarnings(result.Warnings)
	fmt.Println(result.ConvertedYAML)
}

// PrintBatchPreview displays the batch preview result
func PrintBatchPreview(result *BatchPreviewResult) {
	printWarnings(result.Warnings)
	fmt.Println(result.ConvertedYAML)
}

func printWarnings(warnings []string) {
	if len(warnings) > 0 {
		fmt.Println("Warnings:")
		for _, w := range warnings {
			fmt.Printf("  - %s\n", w)
		}
		fmt.Println()
	}
}
