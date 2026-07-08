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
	"context"
	"encoding/json"
	"fmt"
	"time"

	kubelbee "k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/cli/internal/config"
	"k8c.io/kubelb/cli/internal/logger"
	"k8c.io/kubelb/cli/internal/output"
	"k8c.io/kubelb/cli/internal/ui"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func Create(ctx context.Context, k8s client.Client, cfg *config.Config, opts CreateOptions) error {
	if cfg.IsCE() {
		return ErrTunnelNotAvailable
	}

	log := logger.WithTunnel(opts.Name).WithOperation("create")

	if opts.Port <= 0 || opts.Port > 65535 {
		log.Error("invalid port specified", "port", opts.Port)
		return fmt.Errorf("invalid port: %d (must be between 1 and 65535)", opts.Port)
	}

	// Create tunnel resource
	annotations := map[string]string{
		"kubelb.k8c.io/cli-resource": "true",
	}

	// If no explicit hostname provided, request wildcard domain for dynamic assignment
	if opts.Hostname == "" {
		annotations["kubelb.k8c.io/request-wildcard-domain"] = "true"
		log.Debug("requesting wildcard domain for dynamic hostname assignment")
	} else {
		log.Debug("using explicit hostname", "hostname", opts.Hostname)
	}

	tunnel := &kubelbee.Tunnel{
		ObjectMeta: metav1.ObjectMeta{
			Name:        opts.Name,
			Namespace:   cfg.TenantNamespace,
			Annotations: annotations,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kubelb-cli",
				"kubelb.k8c.io/created-by":     "cli",
			},
		},
		Spec: kubelbee.TunnelSpec{
			Hostname: opts.Hostname,
		},
	}

	ui.Progress("Creating tunnel %s...", opts.Name)
	log.Debug("tunnel resource configured", "annotations_count", len(annotations))

	// Check if tunnel already exists
	existingTunnel := &kubelbee.Tunnel{}
	err := k8s.Get(ctx, client.ObjectKey{
		Namespace: cfg.TenantNamespace,
		Name:      opts.Name,
	}, existingTunnel)

	switch {
	case err == nil:
		// Tunnel already exists
		log.Info("tunnel already exists, using existing resource")
		ui.Success("Tunnel %s already exists", opts.Name)
		tunnel = existingTunnel
	case !apierrors.IsNotFound(err):
		log.Error("failed to check existing tunnel", "error", err)
		return fmt.Errorf("failed to check existing tunnel: %w", err)
	default:
		// Tunnel doesn't exist, create it
		log.Debug("creating new tunnel resource")
		if err := k8s.Create(ctx, tunnel); err != nil {
			log.Error("failed to create tunnel", "error", err)
			return fmt.Errorf("failed to create tunnel: %w", err)
		}
		log.Debug("tunnel resource created successfully")
		ui.Success("Tunnel %s created", opts.Name)
	}

	// If connecting, start connection process in parallel with waiting
	switch {
	case opts.Connect && opts.Wait:
		connectErrCh := make(chan error, 1)
		connectStarted := make(chan bool, 1)

		go func() {
			for {
				var t kubelbee.Tunnel
				if err := k8s.Get(ctx, client.ObjectKey{
					Namespace: cfg.TenantNamespace,
					Name:      opts.Name,
				}, &t); err == nil && t.Status.ConnectionManagerURL != "" && t.Status.Phase == kubelbee.TunnelPhaseReady {
					fmt.Printf("✓ Connection manager URL available, establishing tunnel connection...\n")
					connectStarted <- true
					connectErrCh <- Connect(ctx, k8s, cfg, opts.Name, opts.Port)
					return
				}
				select {
				case <-ctx.Done():
					connectErrCh <- ctx.Err()
					return
				case <-time.After(500 * time.Millisecond):
					// Continue polling
				}
			}
		}()

		fmt.Printf("Waiting for tunnel to be ready...\n")
		if err := waitForTunnelReady(ctx, k8s, cfg.TenantNamespace, opts.Name, opts.Connect); err != nil {
			return fmt.Errorf("tunnel creation failed: %w", err)
		}

		if err := k8s.Get(ctx, client.ObjectKey{
			Namespace: cfg.TenantNamespace,
			Name:      opts.Name,
		}, tunnel); err != nil {
			return fmt.Errorf("failed to get tunnel status: %w", err)
		}

		if !opts.Connect {
			if err := displayTunnel(tunnel, opts.Output); err != nil {
				return fmt.Errorf("failed to display tunnel: %w", err)
			}
		}

		// Wait for connection to complete or timeout
		select {
		case <-connectStarted:
			// Connection started, wait for it to complete
			return <-connectErrCh
		case <-time.After(5 * time.Second):
			// If connection hasn't started after tunnel is ready, check status and try manually
			var t kubelbee.Tunnel
			if err := k8s.Get(ctx, client.ObjectKey{
				Namespace: cfg.TenantNamespace,
				Name:      opts.Name,
			}, &t); err != nil {
				return fmt.Errorf("failed to get tunnel status: %w", err)
			}
			if t.Status.Phase != kubelbee.TunnelPhaseReady {
				return fmt.Errorf("tunnel is not ready for connection (status: %s)", t.Status.Phase)
			}
			fmt.Printf("\nConnecting to tunnel...\n")
			return Connect(ctx, k8s, cfg, opts.Name, opts.Port)
		}
	case opts.Wait:
		fmt.Printf("Waiting for tunnel to be ready...\n")
		if err := waitForTunnelReady(ctx, k8s, cfg.TenantNamespace, opts.Name, opts.Connect); err != nil {
			return fmt.Errorf("tunnel creation failed: %w", err)
		}
		if err := k8s.Get(ctx, client.ObjectKey{
			Namespace: cfg.TenantNamespace,
			Name:      opts.Name,
		}, tunnel); err != nil {
			return fmt.Errorf("failed to get tunnel status: %w", err)
		}
		if err := displayTunnel(tunnel, opts.Output); err != nil {
			return fmt.Errorf("failed to display tunnel: %w", err)
		}
	case opts.Connect:
		fmt.Printf("\nConnecting to tunnel...\n")
		return Connect(ctx, k8s, cfg, opts.Name, opts.Port)
	}

	return nil
}

// waitForTunnelReady waits for the tunnel to be ready with progressive status updates
func waitForTunnelReady(ctx context.Context, k8s client.Client, namespace, name string, quiet bool) error {
	var lastStatus string
	var lastHostname string
	var lastURL string
	shownConditions := make(map[string]bool)

	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		tunnel := &kubelbee.Tunnel{}
		if err := k8s.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		}, tunnel); err != nil {
			return false, err
		}

		// Show hostname assignment progress
		if tunnel.Status.Hostname != "" && tunnel.Status.Hostname != lastHostname {
			fmt.Printf("✓ Hostname assigned: %s\n", tunnel.Status.Hostname)
			lastHostname = tunnel.Status.Hostname
		}

		// Show URL availability progress (skip in quiet mode)
		if tunnel.Status.URL != "" && tunnel.Status.URL != lastURL {
			if !quiet {
				fmt.Printf("✓ URL ready\n")
			}
			lastURL = tunnel.Status.URL
		}

		// Show phase changes (skip success states in quiet mode, always show failures)
		currentStatus := string(tunnel.Status.Phase)
		if currentStatus != lastStatus {
			switch tunnel.Status.Phase {
			case kubelbee.TunnelPhasePending:
				if !quiet {
					fmt.Printf("⏳ Tunnel provisioning...\n")
				}
			case kubelbee.TunnelPhaseReady:
				if !quiet {
					fmt.Printf("✓ Tunnel ready\n")
				}
			case kubelbee.TunnelPhaseFailed:
				fmt.Printf("✗ Tunnel failed\n")
			}
			lastStatus = currentStatus
		}
		if !quiet {
			showConditionsProgress(tunnel.Status.Conditions, shownConditions)
		}

		// Check tunnel status
		switch tunnel.Status.Phase {
		case kubelbee.TunnelPhaseReady:
			// Don't perform health check here as it causes "tunnel not found" errors
			// The tunnel registration happens after this check
			return true, nil
		case kubelbee.TunnelPhaseFailed:
			return false, fmt.Errorf("tunnel provisioning failed")
		case kubelbee.TunnelPhaseTerminating:
			return false, fmt.Errorf("tunnel is being terminated")
		default:
			// Still pending, continue waiting
			return false, nil
		}
	})
}

// showConditionsProgress displays progress for tunnel conditions
func showConditionsProgress(conditions []metav1.Condition, shownConditions map[string]bool) {
	for _, condition := range conditions {
		if condition.Status == metav1.ConditionTrue && !shownConditions[condition.Type] {
			switch condition.Type {
			case "DNSReady":
				fmt.Printf("✓ DNS ready\n")
				shownConditions[condition.Type] = true
			case "TLSReady":
				fmt.Printf("✓ TLS ready\n")
				shownConditions[condition.Type] = true
			case "EndpointReady":
				fmt.Printf("✓ Endpoint ready\n")
				shownConditions[condition.Type] = true
			}
		}
	}
}

// displayTunnel displays tunnel information in the requested format
func displayTunnel(tunnel *kubelbee.Tunnel, format string) error {
	switch format {
	case "yaml":
		data, err := yaml.Marshal(tunnel)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
	case "json":
		data, err := json.MarshalIndent(tunnel, "", "  ")
		if err != nil {
			return err
		}
		fmt.Print(string(data))
	case "summary":
		fallthrough
	default:
		fmt.Printf("✓ Tunnel created: %s\n", tunnel.Name)
		switch {
		case tunnel.Status.URL != "":
			fmt.Printf("%s\n", output.FormatPublicURL(tunnel.Status.URL))
		case tunnel.Status.Hostname != "":
			fmt.Printf("Hostname: %s\n", tunnel.Status.Hostname)
		default:
			// Check if wildcard domain was requested
			if annotation, exists := tunnel.Annotations["kubelb.k8c.io/request-wildcard-domain"]; exists && annotation == "true" {
				fmt.Printf("Hostname: Waiting for dynamic assignment (wildcard domain requested)\n")
			}
		}
		fmt.Printf("Status: %s\n", tunnel.Status.Phase)
		if tunnel.Status.ConnectionManagerURL != "" {
			fmt.Printf("Connection: Use 'kubelb tunnel connect %s --port <port>' to start\n", tunnel.Name)
		}
	}
	return nil
}
