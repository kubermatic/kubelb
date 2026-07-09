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
	"strings"
	"text/tabwriter"

	"k8c.io/kubelb/pkg/conversion"

	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// List retrieves all ingresses and displays their conversion status
func List(ctx context.Context, k8sClient client.Client, opts *conversion.Options, namespace string) error {
	var ingresses networkingv1.IngressList
	listOpts := []client.ListOption{}
	if namespace != "" {
		listOpts = append(listOpts, client.InNamespace(namespace))
	}
	if err := k8sClient.List(ctx, &ingresses, listOpts...); err != nil {
		return fmt.Errorf("failed to list ingresses: %w", err)
	}

	infos := make([]Info, 0, len(ingresses.Items))
	for _, ing := range ingresses.Items {
		// Filter by ingress class if specified
		if opts.IngressClass != "" && !matchesIngressClass(&ing, opts.IngressClass) {
			continue
		}
		infos = append(infos, extractInfo(&ing))
	}

	summary := calculateSummary(infos)
	displaySummary(summary)
	displayIngressTable(infos)
	return nil
}

// matchesIngressClass checks if an Ingress matches the specified class.
// Delegates to kubelb's conversion.MatchesIngressClass.
func matchesIngressClass(ing *networkingv1.Ingress, class string) bool {
	return conversion.MatchesIngressClass(ing, class)
}

// extractInfo builds Info from an Ingress resource
func extractInfo(ing *networkingv1.Ingress) Info {
	info := Info{
		Name:      ing.Name,
		Namespace: ing.Namespace,
		Status:    StatusNew,
	}

	// Extract hosts
	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" {
			info.Hosts = append(info.Hosts, rule.Host)
		}
	}

	// Check if marked for skip via label or annotation
	if conversion.ShouldSkip(ing) {
		info.Status = StatusSkipped
		// Set skip reason if available
		if ing.Annotations != nil {
			if reason, ok := ing.Annotations[conversion.AnnotationConversionSkipReason]; ok {
				info.SkipReason = reason
			}
		}
		if info.SkipReason == "" {
			info.SkipReason = "marked with skip-conversion"
		}
		return info
	}

	if ing.Annotations == nil {
		return info
	}

	// Get conversion status
	if status, ok := ing.Annotations[conversion.AnnotationConversionStatus]; ok {
		info.Status = Status(status)
	}

	// Get converted routes
	if routes, ok := ing.Annotations[conversion.AnnotationConvertedHTTPRoute]; ok && routes != "" {
		info.HTTPRoutes = strings.Split(routes, ",")
	}
	if routes, ok := ing.Annotations[conversion.AnnotationConvertedGRPCRoute]; ok && routes != "" {
		info.GRPCRoutes = strings.Split(routes, ",")
	}

	// Get warnings
	if warnings, ok := ing.Annotations[conversion.AnnotationConversionWarnings]; ok && warnings != "" {
		info.Warnings = strings.Split(warnings, ";")
	}

	// Get skip reason
	if reason, ok := ing.Annotations[conversion.AnnotationConversionSkipReason]; ok {
		info.SkipReason = reason
	}

	return info
}

func calculateSummary(infos []Info) Summary {
	var s Summary
	s.Total = len(infos)
	for _, info := range infos {
		switch info.Status {
		case StatusConverted:
			s.Converted++
		case StatusPartial:
			s.Partial++
		case StatusPending:
			s.Pending++
		case StatusFailed:
			s.Failed++
		case StatusSkipped:
			s.Skipped++
		case StatusNew:
			s.New++
		}
	}
	return s
}

func displaySummary(s Summary) {
	fmt.Printf("Total: %d | Converted: %d | Partial: %d | Pending: %d | Failed: %d | Skipped: %d | New: %d\n\n",
		s.Total, s.Converted, s.Partial, s.Pending, s.Failed, s.Skipped, s.New)
}

func displayIngressTable(infos []Info) {
	if len(infos) == 0 {
		fmt.Println("No ingresses found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "NAMESPACE\tNAME\tSTATUS\tHOSTS\tROUTES\tWARNINGS")

	for _, info := range infos {
		hosts := "-"
		if len(info.Hosts) > 0 {
			if len(info.Hosts) == 1 {
				hosts = info.Hosts[0]
			} else {
				hosts = fmt.Sprintf("%s +%d", info.Hosts[0], len(info.Hosts)-1)
			}
		}

		routeCount := len(info.HTTPRoutes) + len(info.GRPCRoutes)
		routes := "-"
		if routeCount > 0 {
			routes = fmt.Sprintf("%d", routeCount)
		}

		warningCount := "-"
		if len(info.Warnings) > 0 {
			warningCount = fmt.Sprintf("%d", len(info.Warnings))
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			info.Namespace,
			info.Name,
			info.Status,
			hosts,
			routes,
			warningCount,
		)
	}
}
