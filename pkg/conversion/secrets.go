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

package conversion

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// AnnotationSourceNamespace tracks the original namespace of a copied secret
	AnnotationSourceNamespace = "kubelb.k8c.io/source-namespace"
)

// CopyTLSSecret copies a TLS secret from source namespace to target namespace.
// It creates a new secret if one doesn't exist, or updates the existing one.
// The copied secret is labeled with managed-by and annotated with source namespace.
func CopyTLSSecret(ctx context.Context, k8sClient client.Client, sourceNS, secretName, targetNS string) error {
	return CopyTLSSecretWithManagedBy(ctx, k8sClient, sourceNS, secretName, targetNS, ControllerName)
}

// CopyTLSSecretWithManagedBy copies a TLS secret with custom managed-by label value.
func CopyTLSSecretWithManagedBy(ctx context.Context, k8sClient client.Client, sourceNS, secretName, targetNS, managedBy string) error {
	// Get source secret
	sourceSecret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: sourceNS, Name: secretName}, sourceSecret); err != nil {
		return fmt.Errorf("failed to get source secret: %w", err)
	}

	// Check if target secret already exists
	targetSecret := &corev1.Secret{}
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: targetNS, Name: secretName}, targetSecret)
	if err == nil {
		// Secret exists, update it
		targetSecret.Data = sourceSecret.Data
		targetSecret.Type = sourceSecret.Type
		if err := k8sClient.Update(ctx, targetSecret); err != nil {
			return fmt.Errorf("failed to update target secret: %w", err)
		}
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to check target secret: %w", err)
	}

	// Create new secret in target namespace
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: targetNS,
			Labels: map[string]string{
				LabelManagedBy: managedBy,
			},
			Annotations: map[string]string{
				AnnotationSourceNamespace: sourceNS,
			},
		},
		Type: sourceSecret.Type,
		Data: sourceSecret.Data,
	}

	if err := k8sClient.Create(ctx, newSecret); err != nil {
		return fmt.Errorf("failed to create target secret: %w", err)
	}
	return nil
}
