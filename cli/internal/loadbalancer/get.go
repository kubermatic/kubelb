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
	"context"
	"fmt"

	kubelb "k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/cli/internal/config"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func Get(ctx context.Context, k8sClient client.Client, cfg *config.Config, name string) error {
	var loadbalancer kubelb.LoadBalancer
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: cfg.TenantNamespace,
	}, &loadbalancer)
	if err != nil {
		return fmt.Errorf("failed to get load balancer %s: %w", name, err)
	}

	// Remove managedFields for cleaner output
	loadbalancer.ManagedFields = nil
	loadbalancer.Namespace = ""

	// Convert to YAML
	yamlData, err := yaml.Marshal(&loadbalancer)
	if err != nil {
		return fmt.Errorf("failed to marshal load balancer to YAML: %w", err)
	}
	fmt.Print(string(yamlData))
	return nil
}
