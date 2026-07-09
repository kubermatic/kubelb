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
	"testing"

	"k8c.io/kubelb/pkg/conversion"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCopyTLSSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-tls-secret",
			Namespace: "source-ns",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sourceSecret).
		Build()

	ctx := context.Background()

	// Test copying secret using kubelb function
	err := conversion.CopyTLSSecretWithManagedBy(ctx, client, "source-ns", "my-tls-secret", "target-ns", "kubelb-cli")
	if err != nil {
		t.Fatalf("CopyTLSSecretWithManagedBy failed: %v", err)
	}

	// Verify secret was copied
	copiedSecret := &corev1.Secret{}
	err = client.Get(ctx, types.NamespacedName{Namespace: "target-ns", Name: "my-tls-secret"}, copiedSecret)
	if err != nil {
		t.Fatalf("Failed to get copied secret: %v", err)
	}

	if copiedSecret.Type != corev1.SecretTypeTLS {
		t.Errorf("Secret type = %v, want %v", copiedSecret.Type, corev1.SecretTypeTLS)
	}

	if string(copiedSecret.Data["tls.crt"]) != "cert-data" {
		t.Errorf("tls.crt = %v, want cert-data", string(copiedSecret.Data["tls.crt"]))
	}

	if string(copiedSecret.Data["tls.key"]) != "key-data" {
		t.Errorf("tls.key = %v, want key-data", string(copiedSecret.Data["tls.key"]))
	}

	// Verify labels
	if copiedSecret.Labels[conversion.LabelManagedBy] != "kubelb-cli" {
		t.Errorf("managed-by label = %v, want kubelb-cli", copiedSecret.Labels[conversion.LabelManagedBy])
	}

	// Verify annotations
	if copiedSecret.Annotations[conversion.AnnotationSourceNamespace] != "source-ns" {
		t.Errorf("source-namespace annotation = %v, want source-ns", copiedSecret.Annotations[conversion.AnnotationSourceNamespace])
	}
}

func TestCopyTLSSecret_Update(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-tls-secret",
			Namespace: "source-ns",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("new-cert-data"),
			"tls.key": []byte("new-key-data"),
		},
	}

	existingTarget := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-tls-secret",
			Namespace: "target-ns",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("old-cert-data"),
			"tls.key": []byte("old-key-data"),
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sourceSecret, existingTarget).
		Build()

	ctx := context.Background()

	// Test updating secret using kubelb function
	err := conversion.CopyTLSSecretWithManagedBy(ctx, client, "source-ns", "my-tls-secret", "target-ns", "kubelb-cli")
	if err != nil {
		t.Fatalf("CopyTLSSecretWithManagedBy failed: %v", err)
	}

	// Verify secret was updated
	updatedSecret := &corev1.Secret{}
	err = client.Get(ctx, types.NamespacedName{Namespace: "target-ns", Name: "my-tls-secret"}, updatedSecret)
	if err != nil {
		t.Fatalf("Failed to get updated secret: %v", err)
	}

	if string(updatedSecret.Data["tls.crt"]) != "new-cert-data" {
		t.Errorf("tls.crt = %v, want new-cert-data", string(updatedSecret.Data["tls.crt"]))
	}
}

func TestCopyTLSSecret_SourceNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	ctx := context.Background()

	err := conversion.CopyTLSSecretWithManagedBy(ctx, client, "source-ns", "nonexistent", "target-ns", "kubelb-cli")
	if err == nil {
		t.Error("Expected error for nonexistent source secret")
	}
}
