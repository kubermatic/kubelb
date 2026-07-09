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
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/spf13/cobra"

	"k8c.io/kubelb/cli/internal/ingress"
	"k8c.io/kubelb/pkg/conversion"

	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Shared conversion options for all ingress subcommands
// Defaults are set here, env vars applied in init(), flags override both
var conversionOpts = &conversion.Options{}

var ingressCmd = &cobra.Command{
	Use:   "ingress",
	Short: "Migrate Ingress resources to Gateway API",
	Long: heredoc.Doc(`
		Tools for migrating NGINX Ingress resources to Gateway API (HTTPRoute).

		This command provides a complete workflow for converting Ingresses:
		- List ingresses with their conversion status
		- Get clean YAML for individual ingresses
		- Preview the generated HTTPRoute YAML before applying
		- Convert individual or multiple ingresses
		- Run a web dashboard for interactive migration

		All subcommands respect the same conversion options (gateway name, namespace, etc).
	`),
	Aliases: []string{"ing"},
}

func ingressListCmd() *cobra.Command {
	var namespace string
	var allNamespaces bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List ingresses with conversion status",
		Long: heredoc.Doc(`
			List all Ingress resources and their Gateway API conversion status.

			Status values:
			- converted: HTTPRoute exists and is accepted by Gateway
			- partial:   Some routes accepted, others pending/failed
			- pending:   HTTPRoute created but not yet accepted
			- skipped:   Ingress skipped (canary, explicit skip annotation)
			- failed:    Conversion encountered errors
		`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if allNamespaces {
				namespace = ""
			}
			return runIngressList(cmd, namespace)
		},
		Example: `kubelb ingress list -n default`,
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to filter (default: all namespaces)")
	cmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "List across all namespaces")
	return cmd
}

func ingressPreviewCmd() *cobra.Command {
	var namespace string
	var all bool
	var allNamespaces bool

	cmd := &cobra.Command{
		Use:   "preview [NAME]",
		Short: "Preview HTTPRoute YAML for an ingress",
		Long: heredoc.Doc(`
			Preview the Gateway API resources that would be generated from an Ingress.

			Shows the HTTPRoute YAML without applying it, allowing review before conversion.
			Also displays any warnings about unsupported annotations.

			Use --all to preview all ingresses in a namespace, or -A for all namespaces.
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSelectionFlags(args, all, allNamespaces, namespace); err != nil {
				return err
			}
			return runIngressPreview(cmd, args, namespace, all, allNamespaces)
		},
		Example: heredoc.Doc(`
			# Preview single ingress
			kubelb ingress preview my-app -n default

			# Preview all ingresses in a namespace
			kubelb ingress preview --all -n kube-system

			# Preview all ingresses across all namespaces
			kubelb ingress preview -A
		`),
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace of the ingress")
	cmd.Flags().BoolVar(&all, "all", false, "Preview all ingresses in namespace (requires -n)")
	cmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "Preview all ingresses across all namespaces")
	return cmd
}

func ingressConvertCmd() *cobra.Command {
	var outputDir string
	var dryRun bool
	var namespace string
	var all bool
	var allNamespaces bool

	cmd := &cobra.Command{
		Use:   "convert [NAME...]",
		Short: "Convert ingresses to Gateway API",
		Long: heredoc.Doc(`
			Convert one or more Ingress resources to Gateway API HTTPRoutes.

			By default, applies the resources directly to the cluster.
			Use --output-dir to export YAML files for GitOps workflows.
			Use --dry-run to preview changes without applying.

			Must specify NAME(s), --all (with -n), or -A. No implicit bulk conversion.
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSelectionFlags(args, all, allNamespaces, namespace); err != nil {
				return err
			}
			return runIngressConvert(cmd, args, outputDir, dryRun, namespace, all, allNamespaces)
		},
		Example: heredoc.Doc(`
			# Convert single ingress
			kubelb ingress convert my-app -n default

			# Convert all ingresses in a namespace
			kubelb ingress convert --all -n default

			# Convert all ingresses across all namespaces
			kubelb ingress convert -A

			# Export to files for GitOps
			kubelb ingress convert --all -n default --output-dir ./manifests

			# Preview changes without applying
			kubelb ingress convert my-app --dry-run
		`),
	}

	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "Export YAML files to directory instead of applying")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to filter ingresses")
	cmd.Flags().BoolVar(&all, "all", false, "Convert all ingresses in namespace (requires -n)")
	cmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "Convert all ingresses across all namespaces")

	return cmd
}

func ingressGetCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "get NAME",
		Short: "Display an Ingress resource as clean YAML",
		Long: heredoc.Doc(`
			Fetch an Ingress and display its YAML with cluster noise removed.

			Strips managed fields, last-applied-configuration annotation, and sets
			the proper API version and kind. Useful for inspecting an Ingress before
			deciding whether to convert it.
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if strings.Contains(name, "/") {
				parts := strings.SplitN(name, "/", 2)
				namespace = parts[0]
				name = parts[1]
			}
			if namespace == "" {
				namespace = defaultNamespace
			}
			return ingress.Get(cmd.Context(), k8sClient, namespace, name)
		},
		Example: heredoc.Doc(`
			# Get ingress YAML
			kubelb ingress get my-app -n default

			# Using namespace/name format
			kubelb ingress get default/my-app
		`),
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace of the ingress (default: default)")
	return cmd
}

// validateSelectionFlags checks mutual exclusion of NAME args, --all, and -A
func validateSelectionFlags(args []string, all, allNamespaces bool, namespace string) error {
	hasNames := len(args) > 0
	if hasNames && (all || allNamespaces) {
		return fmt.Errorf("cannot specify NAME(s) with --all or -A")
	}
	if all && allNamespaces {
		return fmt.Errorf("cannot use --all and -A together")
	}
	if all && namespace == "" {
		return fmt.Errorf("--all requires -n/--namespace")
	}
	if !hasNames && !all && !allNamespaces {
		return fmt.Errorf("specify NAME(s), --all (with -n), or -A")
	}
	return nil
}

// gatewayAnnotationsValue implements pflag.Value for parsing comma-separated key=value pairs
type gatewayAnnotationsValue struct {
	target *map[string]string
}

func (f *gatewayAnnotationsValue) String() string {
	if f.target == nil || *f.target == nil {
		return ""
	}
	var pairs []string
	for k, v := range *f.target {
		pairs = append(pairs, k+"="+v)
	}
	return strings.Join(pairs, ",")
}

func (f *gatewayAnnotationsValue) Set(value string) error {
	if *f.target == nil {
		*f.target = make(map[string]string)
	}
	if value == "" {
		return nil
	}
	for _, pair := range strings.Split(value, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			(*f.target)[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return nil
}

func (f *gatewayAnnotationsValue) Type() string {
	return "stringMap"
}

// Environment variable names for ingress conversion options
const defaultNamespace = "default"

const (
	EnvGatewayName                 = "KUBELB_GATEWAY_NAME"
	EnvGatewayNamespace            = "KUBELB_GATEWAY_NAMESPACE"
	EnvGatewayClass                = "KUBELB_GATEWAY_CLASS"
	EnvIngressClass                = "KUBELB_INGRESS_CLASS"
	EnvDomainReplace               = "KUBELB_DOMAIN_REPLACE"
	EnvDomainSuffix                = "KUBELB_DOMAIN_SUFFIX"
	EnvPropagateExternalDNS        = "KUBELB_PROPAGATE_EXTERNAL_DNS"
	EnvGatewayAnnotations          = "KUBELB_GATEWAY_ANNOTATIONS"
	EnvDisableEnvoyGatewayFeatures = "KUBELB_DISABLE_ENVOY_GATEWAY_FEATURES"
	EnvCopyTLSSecrets              = "KUBELB_COPY_TLS_SECRETS"
)

// initConversionOptsFromEnv initializes conversionOpts from env vars with defaults
func initConversionOptsFromEnv() {
	// String options with defaults
	conversionOpts.GatewayName = getEnvOrDefault(EnvGatewayName, "kubelb")
	conversionOpts.GatewayNamespace = getEnvOrDefault(EnvGatewayNamespace, "kubelb")
	conversionOpts.GatewayClassName = getEnvOrDefault(EnvGatewayClass, "kubelb")
	conversionOpts.IngressClass = os.Getenv(EnvIngressClass) // no default
	conversionOpts.DomainReplace = os.Getenv(EnvDomainReplace)
	conversionOpts.DomainSuffix = os.Getenv(EnvDomainSuffix)

	// Bool options with defaults
	conversionOpts.PropagateExternalDNS = getEnvBoolOrDefault(EnvPropagateExternalDNS, true)
	conversionOpts.CopyTLSSecrets = getEnvBoolOrDefault(EnvCopyTLSSecrets, true)
	conversionOpts.DisableEnvoyGatewayFeatures = getEnvBoolOrDefault(EnvDisableEnvoyGatewayFeatures, false)

	// Gateway annotations (comma-separated key=value)
	if annotations := os.Getenv(EnvGatewayAnnotations); annotations != "" {
		conversionOpts.GatewayAnnotations = parseAnnotations(annotations)
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvBoolOrDefault(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal
	}
	return b
}

func parseAnnotations(s string) map[string]string {
	result := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return result
}

// registerConversionFlags adds conversion option flags to the given command.
// Called for both ingressCmd and serveCmd.
func registerConversionFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&conversionOpts.GatewayName, "gateway-name", conversionOpts.GatewayName, "Gateway name [$KUBELB_GATEWAY_NAME]")
	cmd.PersistentFlags().StringVar(&conversionOpts.GatewayNamespace, "gateway-namespace", conversionOpts.GatewayNamespace, "Gateway namespace [$KUBELB_GATEWAY_NAMESPACE]")
	cmd.PersistentFlags().StringVar(&conversionOpts.GatewayClassName, "gateway-class", conversionOpts.GatewayClassName, "GatewayClass name [$KUBELB_GATEWAY_CLASS]")
	cmd.PersistentFlags().StringVar(&conversionOpts.IngressClass, "ingress-class", conversionOpts.IngressClass, "Only convert Ingresses with this class [$KUBELB_INGRESS_CLASS]")
	cmd.PersistentFlags().StringVar(&conversionOpts.DomainReplace, "domain-replace", conversionOpts.DomainReplace, "Source domain to strip [$KUBELB_DOMAIN_REPLACE]")
	cmd.PersistentFlags().StringVar(&conversionOpts.DomainSuffix, "domain-suffix", conversionOpts.DomainSuffix, "Target domain suffix [$KUBELB_DOMAIN_SUFFIX]")
	cmd.PersistentFlags().BoolVar(&conversionOpts.PropagateExternalDNS, "propagate-external-dns", conversionOpts.PropagateExternalDNS, "Propagate external-dns annotations [$KUBELB_PROPAGATE_EXTERNAL_DNS]")
	cmd.PersistentFlags().Var(&gatewayAnnotationsValue{target: &conversionOpts.GatewayAnnotations}, "gateway-annotations", "Gateway annotations (key=value,...) [$KUBELB_GATEWAY_ANNOTATIONS]")
	cmd.PersistentFlags().BoolVar(&conversionOpts.DisableEnvoyGatewayFeatures, "disable-envoy-gateway-features", conversionOpts.DisableEnvoyGatewayFeatures, "Disable Envoy Gateway policies [$KUBELB_DISABLE_ENVOY_GATEWAY_FEATURES]")
	cmd.PersistentFlags().BoolVar(&conversionOpts.CopyTLSSecrets, "copy-tls-secrets", conversionOpts.CopyTLSSecrets, "Copy TLS secrets to Gateway namespace [$KUBELB_COPY_TLS_SECRETS]")
}

func init() {
	// Initialize from env vars first (flags will override)
	initConversionOptsFromEnv()

	// Register conversion flags on ingressCmd
	registerConversionFlags(ingressCmd)

	// Add subcommands
	ingressCmd.AddCommand(ingressListCmd())
	ingressCmd.AddCommand(ingressPreviewCmd())
	ingressCmd.AddCommand(ingressConvertCmd())
	ingressCmd.AddCommand(ingressGetCmd())
}

// listIngressInputs lists ingresses and returns them as BatchPreviewInput
func listIngressInputs(cmd *cobra.Command, namespace string) ([]ingress.BatchPreviewInput, error) {
	var ingList networkingv1.IngressList
	listOpts := []client.ListOption{}
	if namespace != "" {
		listOpts = append(listOpts, client.InNamespace(namespace))
	}
	if err := k8sClient.List(cmd.Context(), &ingList, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list ingresses: %w", err)
	}
	var inputs []ingress.BatchPreviewInput
	for _, ing := range ingList.Items {
		inputs = append(inputs, ingress.BatchPreviewInput{Namespace: ing.Namespace, Name: ing.Name})
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("no ingresses found")
	}
	return inputs, nil
}

// ensureEnvoyGatewayScheme registers Envoy Gateway types to the client's scheme
// if EG features are enabled. Call this before any EG-related operations.
func ensureEnvoyGatewayScheme() error {
	if conversionOpts.DisableEnvoyGatewayFeatures {
		return nil
	}
	return egv1alpha1.AddToScheme(k8sClient.Scheme())
}

func runIngressList(cmd *cobra.Command, namespace string) error {
	// EG scheme needed for listing policies
	if err := ensureEnvoyGatewayScheme(); err != nil {
		// Non-fatal: continue without EG policy listing
		fmt.Printf("Warning: Envoy Gateway types not available: %v\n", err)
	}
	return ingress.List(cmd.Context(), k8sClient, conversionOpts, namespace)
}

func runIngressPreview(cmd *cobra.Command, args []string, namespace string, all, allNamespaces bool) error {
	if all || allNamespaces {
		ns := namespace
		if allNamespaces {
			ns = ""
		}
		inputs, err := listIngressInputs(cmd, ns)
		if err != nil {
			return err
		}
		result, err := ingress.BatchPreview(cmd.Context(), k8sClient, conversionOpts, inputs)
		if err != nil {
			return err
		}
		ingress.PrintBatchPreview(result)
		return nil
	}

	// Single NAME
	name := args[0]
	if strings.Contains(name, "/") {
		parts := strings.SplitN(name, "/", 2)
		namespace = parts[0]
		name = parts[1]
	}
	if namespace == "" {
		namespace = defaultNamespace
	}

	result, err := ingress.Preview(cmd.Context(), k8sClient, namespace, name, conversionOpts)
	if err != nil {
		return err
	}
	ingress.PrintPreview(result)
	return nil
}

func runIngressConvert(cmd *cobra.Command, args []string, outputDir string, dryRun bool, namespace string, _, allNamespaces bool) error {
	// EG scheme needed for policy creation
	if err := ensureEnvoyGatewayScheme(); err != nil {
		fmt.Printf("Warning: Envoy Gateway types not available, policies will be skipped: %v\n", err)
	}

	convertNS := namespace
	if allNamespaces {
		convertNS = ""
	} else if convertNS == "" && len(args) > 0 {
		// Default to "default" namespace for named ingresses, matching preview behavior
		convertNS = defaultNamespace
	}

	result, err := ingress.Convert(cmd.Context(), k8sClient, conversionOpts, ingress.ConvertOptions{
		OutputDir: outputDir,
		DryRun:    dryRun,
		Names:     args,
		Namespace: convertNS,
	})
	if err != nil {
		return err
	}
	ingress.PrintResult(result)
	return nil
}
