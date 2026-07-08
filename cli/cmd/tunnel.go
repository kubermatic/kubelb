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

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"k8c.io/kubelb/cli/internal/tunnel"
)

var tunnelCmd = &cobra.Command{
	Use:     "tunnel",
	Short:   "Manage secure tunnels to expose local services",
	Long:    `Create and manage secure tunnels to expose local services through the KubeLB infrastructure`,
	Aliases: []string{},
}

func tunnelListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tunnels",
		Long: heredoc.Doc(`
		List all tunnels for the tenant.
	`),
		RunE:    runTunnelList,
		Example: `kubelb tunnel list --tenant=mytenant --kubeconfig=./kubeconfig`,
	}
}

func tunnelGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get NAME",
		Short: "Get a tunnel",
		Long: heredoc.Doc(`
			Retrieve a tunnel by name and output it's complete YAML specification.
		`),
		Args:    cobra.ExactArgs(1),
		RunE:    runTunnelGet,
		Example: `kubelb tunnel get my-app --tenant=mytenant --kubeconfig=./kubeconfig`,
	}
}

func tunnelCreateCmd() *cobra.Command {
	var opts tunnel.CreateOptions

	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a tunnel",
		Long: heredoc.Doc(`
			Create a new secure tunnel to expose a local service.

			The tunnel provides secure access to your local service through the KubeLB infrastructure.

			Examples:
			  # Create tunnel for local app on port 8080
			  kubelb tunnel create my-app --port 8080

			  # Create tunnel with custom hostname
			  kubelb tunnel create my-app --port 8080 --hostname app.example.com

			  # Create tunnel and connect immediately
			  kubelb tunnel create my-app --port 8080 --connect
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			return runTunnelCreate(cmd, opts)
		},
		Example: `kubelb tunnel create my-app --port 8080 --tenant=mytenant`,
	}

	cmd.Flags().IntVarP(&opts.Port, "port", "p", 0, "Local port to tunnel (required)")
	cmd.Flags().StringVar(&opts.Hostname, "hostname", "", "Custom hostname for the tunnel (default: auto-assigned wildcard domain)")
	cmd.Flags().BoolVar(&opts.Wait, "wait", true, "Wait for tunnel to be ready")
	cmd.Flags().BoolVar(&opts.Connect, "connect", false, "Connect to tunnel after creation")
	cmd.Flags().StringVarP(&opts.Output, "output", "o", "summary", "Output format (summary, yaml, json)")

	if err := cmd.MarkFlagRequired("port"); err != nil {
		panic(err)
	}

	return cmd
}

func tunnelDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a tunnel",
		Long: heredoc.Doc(`
			Delete a tunnel by name.

			This command will:
			- Check if the tunnel exists
			- Ask for confirmation before deletion (unless --force is used)
			- Delete the tunnel resource
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTunnelDelete(cmd, args, force)
		},
		Example: `kubelb tunnel delete my-app --tenant=mytenant --kubeconfig=./kubeconfig`,
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force deletion without confirmation")
	return cmd
}

func tunnelConnectCmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "connect NAME",
		Short: "Connect to an existing tunnel",
		Long: heredoc.Doc(`
			Connect to an existing tunnel to start forwarding traffic.

			This command establishes a secure connection to the tunnel and forwards
			traffic from the tunnel to your local service.
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTunnelConnect(cmd, args[0], port)
		},
		Example: `kubelb tunnel connect my-app --port 8080 --tenant=mytenant`,
	}

	cmd.Flags().IntVarP(&port, "port", "p", 0, "Local port to forward to (required)")
	if err := cmd.MarkFlagRequired("port"); err != nil {
		panic(err)
	}

	return cmd
}

// exposeCmd is an alias for tunnel create with auto-generated name
func exposeCmd() *cobra.Command {
	var opts tunnel.CreateOptions

	cmd := &cobra.Command{
		Use:   "expose PORT",
		Short: "Expose a local port via tunnel",
		Long: heredoc.Doc(`
			Expose a local port via secure tunnel with auto-generated name.

			This is a convenience command that creates a tunnel with an auto-generated
			name and immediately connects to it.

			Examples:
			  # Expose port 8080 with auto-generated tunnel name
			  kubelb expose 8080

			  # Expose port 3000 with custom hostname
			  kubelb expose 3000 --hostname api.example.com
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			portStr := args[0]
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return fmt.Errorf("invalid port: %s (must be a number)", portStr)
			}
			opts.Port = port
			opts.Name = tunnel.GenerateTunnelName()
			opts.Connect = true
			return runTunnelCreate(cmd, opts)
		},
		Example: `kubelb expose 8080 --tenant=mytenant`,
	}

	cmd.Flags().StringVar(&opts.Hostname, "hostname", "", "Custom hostname for the tunnel (default: auto-assigned wildcard domain)")
	cmd.Flags().BoolVar(&opts.Wait, "wait", true, "Wait for tunnel to be ready")
	cmd.Flags().StringVarP(&opts.Output, "output", "o", "summary", "Output format (summary, yaml, json)")

	return cmd
}

func init() {
	tunnelCmd.AddCommand(tunnelListCmd())
	tunnelCmd.AddCommand(tunnelGetCmd())
	tunnelCmd.AddCommand(tunnelCreateCmd())
	tunnelCmd.AddCommand(tunnelDeleteCmd())
	tunnelCmd.AddCommand(tunnelConnectCmd())
}

func runTunnelList(cmd *cobra.Command, _ []string) error {
	return tunnel.List(cmd.Context(), k8sClient, cfg)
}

func runTunnelGet(cmd *cobra.Command, args []string) error {
	return tunnel.Get(cmd.Context(), k8sClient, cfg, args[0])
}

func runTunnelCreate(cmd *cobra.Command, opts tunnel.CreateOptions) error {
	// If connecting (like expose command), use signal context for graceful Ctrl+C handling
	if opts.Connect {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		err := tunnel.Create(ctx, k8sClient, cfg, opts)

		// Don't show usage help when user gracefully cancels with Ctrl+C
		if errors.Is(err, context.Canceled) {
			return nil
		}

		return err
	}

	// For non-connecting operations, use the command context
	return tunnel.Create(cmd.Context(), k8sClient, cfg, opts)
}

func runTunnelDelete(cmd *cobra.Command, args []string, force bool) error {
	return tunnel.Delete(cmd.Context(), k8sClient, cfg, args[0], force)
}

func runTunnelConnect(_ *cobra.Command, tunnelName string, port int) error {
	// Create a separate context for tunnel connections that isn't bound by command timeout
	// Use context.Background() with signal handling instead of cmd.Context() which has the 4-minute timeout
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	err := tunnel.Connect(ctx, k8sClient, cfg, tunnelName, port)

	// Don't show usage help when user gracefully cancels with Ctrl+C or deletes tunnel
	if errors.Is(err, context.Canceled) {
		return nil
	}

	// Handle tunnel deletion as successful exit
	if err != nil && err.Error() == "tunnel_deleted" {
		return nil
	}

	return err
}
