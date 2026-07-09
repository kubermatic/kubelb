/*
Copyright 2025 The KubeLB Authors.

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

package loadbalancer

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	kubelb "k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/cli/internal/config"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Delete(ctx context.Context, k8sClient client.Client, cfg *config.Config, name string, force bool) error {
	var loadbalancer kubelb.LoadBalancer
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: cfg.TenantNamespace,
	}, &loadbalancer)
	if err != nil {
		return fmt.Errorf("failed to get load balancer %s: %w", name, err)
	}

	// Check if the load balancer was created by CLI
	if !isManagedByCLI(loadbalancer) {
		fmt.Printf("⚠️  WARNING: Load balancer '%s' was not created by the CLI.\n", name)
		fmt.Printf("This resource may have been created manually or by another tool. Deleting it may cause unexpected issues.\n\n")
	}

	fmt.Printf("Load balancer to be deleted:\n")
	fmt.Printf("  Name: %s\n", loadbalancer.Name)
	if loadbalancer.Spec.Type != "" {
		fmt.Printf("  Type: %s\n", loadbalancer.Spec.Type)
	}
	if loadbalancer.Spec.Type == corev1.ServiceTypeLoadBalancer {
		if len(loadbalancer.Status.LoadBalancer.Ingress) > 0 {
			fmt.Printf("  Ingress Endpoints:\n")
			for _, ingress := range loadbalancer.Status.LoadBalancer.Ingress {
				if ingress.IP != "" {
					fmt.Printf("    - %s\n", ingress.IP)
				} else if ingress.Hostname != "" {
					fmt.Printf("    - %s\n", ingress.Hostname)
				}
			}
		}
	}
	fmt.Printf("\n")

	// Get confirmation unless force flag is used
	if !force {
		if !confirmDeletion(name) {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	err = k8sClient.Delete(ctx, &loadbalancer)
	if err != nil {
		return fmt.Errorf("failed to delete load balancer %s: %w", name, err)
	}

	fmt.Printf("✅ Load balancer '%s' deleted successfully.\n", name)
	return nil
}

func isManagedByCLI(lb kubelb.LoadBalancer) bool {
	return isCLIGenerated(lb)
}

func confirmDeletion(name string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Are you sure you want to delete load balancer '%s'? (y/N): ", name)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
