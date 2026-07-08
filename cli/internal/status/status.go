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

package status

import (
	"context"
	"fmt"
	"time"

	kubelbce "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	kubelbee "k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/cli/internal/config"
	"k8c.io/kubelb/cli/internal/edition"
	"k8c.io/kubelb/cli/internal/logger"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Run executes the status command
func Run(ctx context.Context, k8sClient client.Client, cfg *config.Config) error {
	log := logger.Get()

	log.Debug("Fetching tenant status", "tenant", cfg.Tenant)

	// Detect edition
	editionType := edition.Edition(cfg.Edition)

	// Get TenantState
	if editionType.IsEE() {
		return getEETenantStatus(ctx, k8sClient, cfg)
	}
	return getCETenantStatus(ctx, k8sClient, cfg)
}

func getCETenantStatus(ctx context.Context, k8sClient client.Client, cfg *config.Config) error {
	log := logger.Get()

	tenantState := &kubelbce.TenantState{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Namespace: cfg.TenantNamespace,
		Name:      "default",
	}, tenantState)

	if err != nil {
		if errors.IsNotFound(err) {
			log.Error("TenantState not found", "tenant", cfg.Tenant)
			return fmt.Errorf("tenant state not found for tenant %s", cfg.Tenant)
		}
		return fmt.Errorf("failed to get tenant state: %w", err)
	}

	return displayCETenantStatus(tenantState)
}

func getEETenantStatus(ctx context.Context, k8sClient client.Client, cfg *config.Config) error {
	log := logger.Get()

	tenantState := &kubelbee.TenantState{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Namespace: cfg.TenantNamespace,
		Name:      "default",
	}, tenantState)

	if err != nil {
		if errors.IsNotFound(err) {
			log.Error("TenantState not found", "tenant", cfg.Tenant)
			return fmt.Errorf("tenant state not found for tenant %s", cfg.Tenant)
		}
		return fmt.Errorf("failed to get tenant state: %w", err)
	}

	return displayEETenantStatus(tenantState)
}

func displayCETenantStatus(state *kubelbce.TenantState) error {
	// Header
	fmt.Println("╔═══════════════════════════════════════════════╗")
	fmt.Println("║                KubeLB Status                  ║")
	fmt.Println("╚═══════════════════════════════════════════════╝")
	fmt.Println()

	// Edition
	fmt.Println("▶ Edition: Community Edition (CE)")
	if state.Status.Version.GitVersion != "" {
		fmt.Printf("  Version: %s\n", state.Status.Version.GitVersion)
	}
	fmt.Println()

	// Last Updated
	if !state.Status.LastUpdated.IsZero() {
		fmt.Println("▶ Last Updated")
		fmt.Printf("  %s (%s ago)\n", state.Status.LastUpdated.Format(time.RFC3339), formatAge(state.Status.LastUpdated.Time))
		fmt.Println()
	}

	return nil
}

func displayEETenantStatus(state *kubelbee.TenantState) error {
	// Header
	fmt.Println("╔═══════════════════════════════════════════════╗")
	fmt.Println("║                KubeLB Status                  ║")
	fmt.Println("╚═══════════════════════════════════════════════╝")
	fmt.Println()

	// Edition
	fmt.Println("▶ Edition: Enterprise Edition (EE)")
	if state.Status.Version.GitVersion != "" {
		fmt.Printf("  Version: %s\n", state.Status.Version.GitVersion)
	}
	fmt.Println()

	// LoadBalancer State
	fmt.Println("▶ LoadBalancer")
	if state.Status.LoadBalancer.Disable {
		fmt.Println("  Status: Disabled")
	} else {
		fmt.Println("  Status: Enabled")
	}
	if state.Status.LoadBalancer.Limit > 0 {
		fmt.Printf("  Limit: %d\n", state.Status.LoadBalancer.Limit)
	}
	fmt.Println()

	// Allowed Domains (EE only)
	if len(state.Status.AllowedDomains) > 0 {
		fmt.Println("▶ Allowed Domains")
		for _, domain := range state.Status.AllowedDomains {
			fmt.Printf("  • %s\n", domain)
		}
		fmt.Println()
	}

	// Tunnel State (EE only)
	fmt.Println("▶ Tunnel")
	if state.Status.Tunnel.Disable {
		fmt.Println("  Status: Disabled")
	} else {
		fmt.Println("  Status: Enabled")
	}
	if state.Status.Tunnel.Limit > 0 {
		fmt.Printf("  Limit: %d\n", state.Status.Tunnel.Limit)
	}
	if state.Status.Tunnel.ConnectionManagerURL != "" {
		fmt.Printf("  Connection Manager URL: %s\n", state.Status.Tunnel.ConnectionManagerURL)
	}
	fmt.Println()

	// Last Updated
	if !state.Status.LastUpdated.IsZero() {
		fmt.Println("▶ Last Updated")
		fmt.Printf("  %s (%s ago)\n", state.Status.LastUpdated.Format(time.RFC3339), formatAge(state.Status.LastUpdated.Time))
		fmt.Println()
	}

	return nil
}

func formatAge(t time.Time) string {
	duration := time.Since(t)

	if duration.Hours() > 24 {
		days := int(duration.Hours() / 24)
		hours := int(duration.Hours()) % 24
		return fmt.Sprintf("%dd%dh", days, hours)
	}

	if duration.Hours() > 1 {
		return fmt.Sprintf("%dh%dm", int(duration.Hours()), int(duration.Minutes())%60)
	}

	if duration.Minutes() > 1 {
		return fmt.Sprintf("%dm%ds", int(duration.Minutes()), int(duration.Seconds())%60)
	}

	return fmt.Sprintf("%ds", int(duration.Seconds()))
}
