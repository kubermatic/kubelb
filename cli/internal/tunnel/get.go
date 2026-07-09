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
	"fmt"

	kubelbce "k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/cli/internal/config"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func Get(ctx context.Context, k8s client.Client, cfg *config.Config, name string) error {
	if cfg.IsCE() {
		return ErrTunnelNotAvailable
	}

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

	// Create a clean copy and remove unwanted fields for display
	cleanTunnel := tunnel.DeepCopy()
	cleanTunnel.Status.Resources = kubelbce.TunnelResources{}
	cleanTunnel.ManagedFields = nil
	cleanTunnel.Namespace = ""

	// Display tunnel in YAML format
	data, err := yaml.Marshal(cleanTunnel)
	if err != nil {
		return fmt.Errorf("failed to marshal tunnel to YAML: %w", err)
	}

	fmt.Print(string(data))
	return nil
}
