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

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Config holds the configuration for the KubeLB CLI
type Config struct {
	KubeConfig         string
	Tenant             string
	TenantNamespace    string
	InsecureSkipVerify bool
	Edition            string // "CE" or "EE"
}

// LoadConfig loads configuration from flags, environment variables, and defaults
func LoadConfig(kubeconfigFlag, tenantFlag string) (*Config, error) {
	cfg := &Config{}

	var err error
	cfg.KubeConfig, err = resolveKubeConfig(kubeconfigFlag)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve kubeconfig: %w", err)
	}

	cfg.Tenant, err = resolveTenant(tenantFlag)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tenant: %w", err)
	}
	cfg.TenantNamespace = GetTenantNamespace(cfg.Tenant)

	// Read InsecureSkipVerify from environment variable
	cfg.InsecureSkipVerify, err = resolveInsecureSkipVerify()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve insecure skip verify: %w", err)
	}

	return cfg, nil
}

// resolveKubeConfig resolves the kubeconfig path with proper precedence
func resolveKubeConfig(flagValue string) (string, error) {
	// 1. Command line flag has highest priority
	if flagValue != "" {
		return flagValue, nil
	}

	// 2. Environment variable KUBECONFIG
	if envKubeConfig := os.Getenv("KUBECONFIG"); envKubeConfig != "" {
		return envKubeConfig, nil
	}

	// 3. Require explicit kubeconfig - no in-cluster or default discovery
	return "", fmt.Errorf("kubeconfig is required: use --kubeconfig flag or KUBECONFIG environment variable")
}

// resolveTenant resolves the tenant name with proper precedence
func resolveTenant(flagValue string) (string, error) {
	// 1. Command line flag has highest priority
	if flagValue != "" {
		return flagValue, nil
	}

	// 2. Environment variable TENANT_NAME
	if envTenant := os.Getenv("TENANT_NAME"); envTenant != "" {
		return envTenant, nil
	}

	// 3. No tenant specified - this is an error for commands that require it
	return "", fmt.Errorf("tenant name is required: use --tenant flag or TENANT_NAME environment variable")
}

// resolveInsecureSkipVerify resolves the InsecureSkipVerify setting from environment variable
func resolveInsecureSkipVerify() (bool, error) {
	// Read from KUBELB_INSECURE_SKIP_VERIFY environment variable
	envValue := os.Getenv("KUBELB_INSECURE_SKIP_VERIFY")
	if envValue == "" {
		return false, nil // Default to secure behavior
	}

	value, err := strconv.ParseBool(envValue)
	if err != nil {
		return false, fmt.Errorf("invalid value for KUBELB_INSECURE_SKIP_VERIFY: %q (must be true/false)", envValue)
	}

	return value, nil
}

// CreateKubernetesConfig creates a Kubernetes REST config from the resolved configuration
func CreateKubernetesConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		return nil, fmt.Errorf("kubeconfig path is required")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig from %s: %w", kubeconfigPath, err)
	}
	return config, nil
}

func GetTenantNamespace(tenant string) string {
	if strings.HasPrefix(tenant, "tenant-") {
		return tenant
	}
	return fmt.Sprintf("tenant-%s", tenant)
}

func (c *Config) ValidateConfig() error {
	if c.Tenant == "" {
		return fmt.Errorf("tenant name is required")
	}
	return nil
}

func (c *Config) IsCE() bool {
	return c.Edition == "CE"
}
