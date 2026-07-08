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

package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	kubelbce "k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/cli/internal/config"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Delete(ctx context.Context, k8s client.Client, cfg *config.Config, name string, force bool) error {
	return DeleteWithOptions(ctx, k8s, cfg, name, force, true)
}

func DeleteWithOptions(ctx context.Context, k8s client.Client, cfg *config.Config, name string, force bool, verbose bool) error {
	if cfg.IsCE() {
		return ErrTunnelNotAvailable
	}

	// Check if tunnel exists
	tunnel := &kubelbce.Tunnel{}
	if err := k8s.Get(ctx, client.ObjectKey{
		Namespace: cfg.TenantNamespace,
		Name:      name,
	}, tunnel); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tunnel %q not found", name)
		}
		return fmt.Errorf("failed to get tunnel: %w", err)
	}

	// Show tunnel information only if verbose
	if verbose {
		fmt.Printf("Tunnel: %s\n", tunnel.Name)
		if tunnel.Status.Hostname != "" {
			fmt.Printf("Hostname: %s\n", tunnel.Status.Hostname)
		}
		if tunnel.Status.URL != "" {
			fmt.Printf("URL: %s\n", tunnel.Status.URL)
		}
		fmt.Printf("Status: %s\n", tunnel.Status.Phase)

		// Check if tunnel was created by CLI
		managedBy, exists := tunnel.Labels["app.kubernetes.io/managed-by"]
		if !exists || managedBy != "kubelb-cli" {
			fmt.Printf("\n⚠️  Warning: This tunnel was not created by the kubelb CLI.\n")
			fmt.Printf("   It may have been created manually or by another tool.\n")
		}
	}

	// Ask for confirmation unless force is specified
	if !force {
		fmt.Printf("\nAre you sure you want to delete this tunnel? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	// Delete the tunnel
	if err := k8s.Delete(ctx, tunnel); err != nil {
		return fmt.Errorf("failed to delete tunnel: %w", err)
	}

	fmt.Printf("✓ Tunnel %s deleted successfully\n", name)
	return nil
}
