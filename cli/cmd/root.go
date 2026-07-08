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
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	kubelbce "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	kubelbee "k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/cli/internal/cache"
	"k8c.io/kubelb/cli/internal/config"
	"k8c.io/kubelb/cli/internal/constants"
	"k8c.io/kubelb/cli/internal/edition"
	"k8c.io/kubelb/cli/internal/logger"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// List of commands that don't require kubeconfig or tenant name etc.
const (
	CmdNameVersion    = "version"
	CmdNameHelp       = "help"
	CmdNameCompletion = "completion"
	CmdNameDocs       = "docs"
	CmdNameIngress    = "ingress" // Ingress/serve commands only need kubeconfig, not tenant
	CmdNameServe      = "serve"
)

var (
	kubeconfig string
	tenant     string
	timeout    time.Duration

	// Logging configuration
	verbosityLevel int
	logLevel       string
	logFormat      string
	logFile        string
	quiet          bool

	cfg       *config.Config
	k8sClient client.Client

	skipConfigCommands = []string{
		CmdNameVersion,
		CmdNameHelp,
		CmdNameCompletion,
		CmdNameDocs,
	}
)

var rootCmd = &cobra.Command{
	Use:   "kubelb",
	Short: "KubeLB CLI - Manage load balancers and create secure tunnels",
	Long: `KubeLB CLI provides tools to manage KubeLB load balancers and create secure tunnels
to expose local services through the KubeLB infrastructure.`,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// Initialize logger first (always needed)
		if err := initializeLogger(); err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}

		// Skip config loading for commands that don't need it
		if shouldSkipConfig(cmd) {
			return nil
		}

		// Ingress and serve commands only need kubeconfig, not tenant
		if needsMinimalClient(cmd) {
			return initializeMinimalClient()
		}

		// Load configuration
		var err error
		cfg, err = config.LoadConfig(kubeconfig, tenant)
		if err != nil {
			return err
		}

		// Detect edition and create appropriate Kubernetes client
		k8sClient, err = createKubernetesClientWithEditionDetection(cmd.Context(), cfg)
		if err != nil {
			return err
		}

		return nil
	},
}

func Execute() error {
	// Create base context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Apply timeout if specified (long-running commands like serve override this)
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return rootCmd.ExecuteContext(ctx)
}

func createKubernetesClientWithEditionDetection(ctx context.Context, cfg *config.Config) (client.Client, error) {
	log := logger.Get()

	restConfig, err := config.CreateKubernetesConfig(cfg.KubeConfig)
	if err != nil {
		return nil, err
	}

	// Try to load edition from cache first
	cachedEdition := cache.LoadCache(cfg.KubeConfig, cfg.Tenant)
	var detectedEdition edition.Edition

	if cachedEdition != "" {
		// Use cached edition
		detectedEdition = edition.Edition(cachedEdition)
		log.Debug("Loaded edition from cache", "edition", detectedEdition)
	} else {
		// Perform detection
		detectedEdition, err = edition.DetectEdition(ctx, restConfig, cfg.TenantNamespace)
		if err != nil {
			log.Warn("Failed to detect KubeLB edition, defaulting to CE", "error", err)
			detectedEdition = edition.EditionCE
		}
		// Save to cache for future use
		if err := cache.SaveCache(cfg.KubeConfig, cfg.Tenant, string(detectedEdition)); err != nil {
			log.Debug("Failed to save edition to cache", "error", err)
		}
	}

	cfg.Edition = string(detectedEdition)
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := gwapiv1.Install(scheme); err != nil {
		return nil, fmt.Errorf("failed to add Gateway API types to scheme: %w", err)
	}

	if detectedEdition.IsEE() {
		if err := kubelbee.AddToScheme(scheme); err != nil {
			return nil, fmt.Errorf("failed to add EE API types to scheme: %w", err)
		}
	} else {
		if err := kubelbce.AddToScheme(scheme); err != nil {
			return nil, fmt.Errorf("failed to add CE API types to scheme: %w", err)
		}
	}

	k8sClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	return k8sClient, nil
}

func shouldSkipConfig(cmd *cobra.Command) bool {
	// Skip if it's the root command (no parent)
	if cmd.Parent() == nil {
		return true
	}

	cmdName := cmd.Name()
	for _, skipCmd := range skipConfigCommands {
		if cmdName == skipCmd {
			return true
		}
	}

	return false
}

// needsMinimalClient checks if the command only needs kubeconfig (no tenant)
func needsMinimalClient(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case CmdNameIngress, CmdNameServe:
			return true
		}
	}
	return false
}

// initializeMinimalClient sets up k8s client without tenant (for ingress/serve commands)
func initializeMinimalClient() error {
	// Resolve kubeconfig
	kubeconfigPath := kubeconfig
	if kubeconfigPath == "" {
		kubeconfigPath = os.Getenv("KUBECONFIG")
	}
	if kubeconfigPath == "" {
		return fmt.Errorf("kubeconfig is required: use --kubeconfig flag or KUBECONFIG environment variable")
	}

	// Create REST config
	restConfig, err := config.CreateKubernetesConfig(kubeconfigPath)
	if err != nil {
		return err
	}

	// Create scheme with Gateway API types
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return err
	}
	if err := gwapiv1.Install(scheme); err != nil {
		return fmt.Errorf("failed to add Gateway API types to scheme: %w", err)
	}

	// Create client
	k8sClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return err
	}

	// Initialize minimal config for compatibility
	cfg = &config.Config{
		KubeConfig: kubeconfigPath,
	}

	return nil
}

// initializeLogger sets up the global logger based on CLI flags and environment variables.
func initializeLogger() error {
	config := logger.DefaultConfig()

	// Apply environment variables first
	if envLevel := os.Getenv("KUBELB_LOG_LEVEL"); envLevel != "" {
		config.Level = logger.ParseLevel(envLevel)
	}

	if envFormat := os.Getenv("KUBELB_LOG_FORMAT"); envFormat != "" {
		switch envFormat {
		case "cli":
			config.Format = logger.FormatCLI
		case "json":
			config.Format = logger.FormatJSON
		case "text":
			config.Format = logger.FormatText
		}
	}

	if envPath := os.Getenv("KUBELB_LOG_PATH"); envPath != "" {
		logFile = envPath
	}

	// Apply CLI flags (override environment variables)
	if logLevel != "" {
		config.Level = logger.ParseLevel(logLevel)
	}

	if logFormat != "" {
		switch logFormat {
		case "cli":
			config.Format = logger.FormatCLI
		case "json":
			config.Format = logger.FormatJSON
		case "text":
			config.Format = logger.FormatText
		default:
			return fmt.Errorf("invalid log format: %s (must be cli, json, or text)", logFormat)
		}
	}

	// Handle log file output
	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file %s: %w", logFile, err)
		}
		config.Output = file
	}

	// Apply quiet flag
	if quiet {
		config.Level = logger.LevelError
		verbosityLevel = 0
	}

	// Set verbosity level
	config.VerbosityLevel = verbosityLevel

	// Map verbosity level to log level if not explicitly set
	if logLevel == "" && os.Getenv("KUBELB_LOG_LEVEL") == "" {
		switch verbosityLevel {
		case 0:
			config.Level = logger.LevelError
		case 1:
			config.Level = logger.LevelInfo
		case 2:
			config.Level = logger.LevelInfo
		case 3:
			config.Level = logger.LevelDebug
		case 4:
			config.Level = logger.LevelTrace
		default:
			config.Level = logger.LevelInfo
		}
	}

	// Initialize the global logger
	logger.Setup(config)

	return nil
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to the kubeconfig for the tenant")
	rootCmd.PersistentFlags().StringVarP(&tenant, "tenant", "t", "", "Name of the tenant")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", constants.DefaultWaitTimeout, "Timeout for the command (e.g., 30s, 5m)")

	// Logging flags
	rootCmd.PersistentFlags().IntVarP(&verbosityLevel, "v", "v", 1, "Verbosity level (0-4): 0=errors only, 1=basic info, 2=detailed status, 3=debug info, 4=trace")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "Log level (error, warn, info, debug, trace) - overrides verbosity")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "", "Log format (cli, json, text) - defaults to cli")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "Log to file instead of stderr")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-essential output (equivalent to --v=0)")

	rootCmd.AddCommand(
		versionCmd(),
		statusCmd(),
		loadbalancerCmd,
		ingressCmd,
		serveCmd,
		tunnelCmd,
		exposeCmd(),
		docsCmd(),
	)
}
