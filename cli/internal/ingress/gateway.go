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

package ingress

import (
	"context"
	"fmt"

	"k8c.io/kubelb/pkg/conversion"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// applyGateway creates or updates the Gateway resource.
// It merges listeners from the new TLS configuration with any existing listeners.
func applyGateway(ctx context.Context, k8sClient client.Client, opts *conversion.Options, tlsListeners []conversion.TLSListener, managedBy string) (*gwapiv1.Gateway, bool, error) {
	// Verify namespace exists before attempting to create Gateway
	ns := &corev1.Namespace{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: opts.GatewayNamespace}, ns); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, fmt.Errorf("gateway namespace %q does not exist; create it first with: kubectl create namespace %s", opts.GatewayNamespace, opts.GatewayNamespace)
		}
		return nil, false, fmt.Errorf("failed to check gateway namespace: %w", err)
	}

	desired := conversion.BuildGateway(conversion.GatewayConfig{
		Name:         opts.GatewayName,
		Namespace:    opts.GatewayNamespace,
		ClassName:    opts.GatewayClassName,
		TLSListeners: tlsListeners,
		Annotations:  opts.GatewayAnnotations,
		ManagedBy:    managedBy,
	})

	existing := &gwapiv1.Gateway{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      opts.GatewayName,
		Namespace: opts.GatewayNamespace,
	}, existing)

	if apierrors.IsNotFound(err) {
		if err := k8sClient.Create(ctx, desired); err != nil {
			if apierrors.IsForbidden(err) {
				return nil, false, fmt.Errorf("permission denied creating Gateway in namespace %q: %w", opts.GatewayNamespace, err)
			}
			return nil, false, fmt.Errorf("failed to create Gateway %s/%s: %w", opts.GatewayNamespace, opts.GatewayName, err)
		}
		return desired, true, nil
	}

	if err != nil {
		return nil, false, fmt.Errorf("failed to get Gateway: %w", err)
	}

	existing.Spec.Listeners = conversion.MergeListeners(existing.Spec.Listeners, desired.Spec.Listeners)

	if existing.Annotations == nil {
		existing.Annotations = make(map[string]string)
	}
	for k, v := range desired.Annotations {
		existing.Annotations[k] = v
	}

	if err := k8sClient.Update(ctx, existing); err != nil {
		return nil, false, fmt.Errorf("failed to update Gateway: %w", err)
	}

	return existing, false, nil
}
