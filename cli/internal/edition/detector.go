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

package edition

import (
	"context"
	"fmt"
	"strings"

	kubelb "k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Edition string

const (
	EditionCE       Edition = "CE"
	EditionEE       Edition = "EE"
	EditionUnknown  Edition = "Unknown"
	TenantStateName         = "default"
)

// DetectEdition detects whether the KubeLB backend is CE or EE by checking TenantState resource
func DetectEdition(ctx context.Context, restConfig *rest.Config, tenantNamespace string) (Edition, error) {
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return EditionUnknown, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Try to get the default TenantState resource
	tenantStateGVR := schema.GroupVersionResource{
		Group:    kubelb.GroupVersion.Group,
		Version:  kubelb.GroupVersion.Version,
		Resource: "tenantstates",
	}

	tenantState, err := dynamicClient.Resource(tenantStateGVR).Namespace(tenantNamespace).Get(ctx, TenantStateName, metav1.GetOptions{})
	if err != nil {
		tenantState, err = dynamicClient.Resource(tenantStateGVR).Namespace(tenantNamespace).Get(ctx, tenantNamespace, metav1.GetOptions{})
		if err != nil {
			return detectEditionByCRDs(ctx, dynamicClient)
		}
	}

	edition, err := extractEditionFromTenantState(tenantState)
	if err != nil {
		return detectEditionByCRDs(ctx, dynamicClient)
	}
	return edition, nil
}

// extractEditionFromTenantState extracts the edition from TenantState status
func extractEditionFromTenantState(tenantState *unstructured.Unstructured) (Edition, error) {
	status, found, err := unstructured.NestedMap(tenantState.Object, "status")
	if err != nil || !found {
		return EditionUnknown, fmt.Errorf("status not found in TenantState")
	}

	editionStr, found, err := unstructured.NestedString(status, "version", "edition")
	if err != nil || !found {
		return EditionUnknown, fmt.Errorf("edition not found in TenantState status")
	}

	switch strings.ToUpper(editionStr) {
	case "EE", "ENTERPRISE":
		return EditionEE, nil
	case "CE", "COMMUNITY":
		return EditionCE, nil
	default:
		return EditionUnknown, fmt.Errorf("unknown edition: %s", editionStr)
	}
}

// detectEditionByCRDs attempts to detect edition by checking which CRDs are available
func detectEditionByCRDs(ctx context.Context, dynamicClient dynamic.Interface) (Edition, error) {
	tunnelGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
	_, err := dynamicClient.Resource(tunnelGVR).Get(ctx, "tunnels.kubelb.k8c.io", metav1.GetOptions{})
	if err == nil {
		return EditionEE, nil
	}
	return EditionCE, nil
}

func (e Edition) IsEE() bool {
	return e == EditionEE
}
