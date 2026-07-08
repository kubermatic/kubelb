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

package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"k8c.io/kubelb/cli/internal/ui"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start web dashboard for interactive Ingress migration",
	Long: heredoc.Doc(`
		Start a local web server with an interactive dashboard for Ingress-to-Gateway
		API migration. Provides a visual interface for the same operations available
		through the CLI subcommands.

		The dashboard has two views:

		Ingresses: Browse all Ingress resources with conversion status, preview
		generated Gateway API YAML, and convert individually or in batch.

		Gateway API: Inspect all created resources (Gateways, HTTPRoutes, GRPCRoutes,
		Envoy Gateway policies) with their acceptance status.
	`),
	RunE: func(_ *cobra.Command, _ []string) error {
		return runServe()
	},
	Example: `kubelb serve --addr 127.0.0.1:8080`,
}

func init() {
	// Loopback by default: the UI exposes unauthenticated mutating endpoints
	// that act with the CLI's kubeconfig privileges.
	serveCmd.Flags().StringVar(&serveAddr, "addr", "127.0.0.1:8080", "Address to listen on")
	registerConversionFlags(serveCmd)
}

var serveAddr string

func runServe() error {
	// EG scheme needed for policy operations in UI
	if err := ensureEnvoyGatewayScheme(); err != nil {
		fmt.Printf("Warning: Envoy Gateway types not available, policies will be disabled: %v\n", err)
	}

	uiServer, err := ui.NewServer(k8sClient, conversionOpts)
	if err != nil {
		return fmt.Errorf("failed to create UI server: %w", err)
	}

	httpServer := &http.Server{
		Addr:    serveAddr,
		Handler: uiServer.Handler(),
	}

	// Create our own signal context (ignores global timeout)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		addr := serveAddr
		if strings.HasPrefix(addr, ":") {
			addr = "localhost" + addr
		}
		fmt.Printf("Starting web UI at http://%s\n", addr)
		fmt.Println("Press Ctrl+C to stop")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for signal or error
	select {
	case <-ctx.Done():
		fmt.Println("\nShutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
