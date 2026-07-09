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
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"k8c.io/kubelb/cli/internal/constants"
	"k8c.io/kubelb/cli/internal/loadbalancer"

	corev1 "k8s.io/api/core/v1"
)

var loadbalancerCmd = &cobra.Command{
	Use:     "loadbalancer",
	Short:   "Manage KubeLB load balancers",
	Long:    `Manage KubeLB load balancer configurations`,
	Aliases: []string{"lb"},
}

func loadbalancerListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List load balancers",
		Long: heredoc.Doc(`
		List all load balancers for the tenant.
	`),
		RunE:    runLoadbalancerList,
		Example: `kubelb loadbalancer list --tenant=mytenant --kubeconfig=./kubeconfig`,
	}
}

func loadbalancerGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get ID",
		Short: "Get a load balancer",
		Long: heredoc.Doc(`
			Retrieve a load balancer by ID and output it's complete YAML specification.
		`),
		Args:    cobra.ExactArgs(1),
		RunE:    runLoadbalancerGet,
		Example: `kubelb loadbalancer get nginx-loadbalancer --tenant=mytenant --kubeconfig=./kubeconfig`,
	}
}

func loadbalancerCreateCmd() *cobra.Command {
	var opts loadbalancer.CreateOptions

	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a load balancer",
		Long: heredoc.Doc(`
			Create a new HTTP load balancer with the specified endpoints.

			The load balancer supports HTTP routing and hostname-based access.

			Examples:
			  # Create HTTP load balancer with random hostname
			  kubelb lb create my-app --endpoints 10.0.1.1:8080

			  # Create HTTP load balancer with custom hostname
			  kubelb lb create my-app --endpoints 10.0.1.1:8080 --hostname app.example.com

			  # Create HTTP load balancer without a route
			  kubelb lb create my-app --endpoints 10.0.1.1:8080 --route=false
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			return runLoadbalancerCreate(cmd, opts)
		},
		Example: `kubelb loadbalancer create my-app --endpoints 10.0.1.1:8080,10.0.1.2:8080 --tenant=mytenant`,
	}

	cmd.Flags().StringVarP(&opts.Endpoints, "endpoints", "e", "", "Comma-separated list of IP:port pairs (required)")
	cmd.Flags().StringVar(&opts.Type, "type", string(corev1.ServiceTypeClusterIP), "LoadBalancer type (ClusterIP, LoadBalancer), defaults to ClusterIP")
	cmd.Flags().StringVarP(&opts.Protocol, "protocol", "p", constants.DefaultProtocol, "Protocol (http only)")
	cmd.Flags().BoolVar(&opts.Route, "route", true, "Create a route for HTTP traffic")
	cmd.Flags().StringVar(&opts.Hostname, "hostname", "", "Custom hostname for the route")
	cmd.Flags().BoolVar(&opts.Wait, "wait", true, "Wait for load balancer to be ready")
	cmd.Flags().StringVarP(&opts.Output, "output", "o", "summary", "Output format (summary, yaml, json)")

	if err := cmd.MarkFlagRequired("endpoints"); err != nil {
		panic(err)
	}

	return cmd
}

func loadbalancerDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete ID",
		Short: "Delete a load balancer",
		Long: heredoc.Doc(`
			Delete a load balancer by ID.

			This command will:
			- Check if the load balancer was created by the CLI
			- Display a warning if it wasn't created by the CLI
			- Ask for confirmation before deletion (unless --force is used)
			- Delete the load balancer resource
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLoadbalancerDelete(cmd, args, force)
		},
		Example: `kubelb loadbalancer delete nginx-loadbalancer --tenant=mytenant --kubeconfig=./kubeconfig`,
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force deletion without confirmation")
	return cmd
}

func init() {
	loadbalancerCmd.AddCommand(loadbalancerListCmd())
	loadbalancerCmd.AddCommand(loadbalancerGetCmd())
	loadbalancerCmd.AddCommand(loadbalancerCreateCmd())
	loadbalancerCmd.AddCommand(loadbalancerDeleteCmd())
}

func runLoadbalancerList(cmd *cobra.Command, _ []string) error {
	return loadbalancer.List(cmd.Context(), k8sClient, cfg)
}

func runLoadbalancerGet(cmd *cobra.Command, args []string) error {
	return loadbalancer.Get(cmd.Context(), k8sClient, cfg, args[0])
}

func runLoadbalancerCreate(cmd *cobra.Command, opts loadbalancer.CreateOptions) error {
	return loadbalancer.Create(cmd.Context(), k8sClient, cfg, opts)
}

func runLoadbalancerDelete(cmd *cobra.Command, args []string, force bool) error {
	return loadbalancer.Delete(cmd.Context(), k8sClient, cfg, args[0], force)
}
