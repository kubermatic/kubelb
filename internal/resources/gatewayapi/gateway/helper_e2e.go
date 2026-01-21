//go:build e2e

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

package gateway

import (
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// MaxGatewaysPerTenant - E2E: relaxed limit for parallel test execution.
	MaxGatewaysPerTenant = 100

	// ParentGatewayName is kept for compatibility but not enforced in E2E.
	ParentGatewayName = "kubelb"

	// GatewayClassName is the required gateway class name.
	GatewayClassName = "kubelb"
)

// ShouldReconcileGatewayByName checks if a Gateway should be reconciled based on naming rules.
// E2E: relaxed - any gateway name allowed for parallel test execution.
func ShouldReconcileGatewayByName(_ *gwapiv1.Gateway) bool {
	return true
}

// ShouldReconcileResource checks if a Gateway API resource should be reconciled.
// E2E: relaxed - any gateway with gatewayClassName: kubelb, any route with parentRefs.
func ShouldReconcileResource(obj ctrlclient.Object, useGatewayClass bool) bool {
	switch resource := obj.(type) {
	case *gwapiv1.Gateway:
		if useGatewayClass && resource.Spec.GatewayClassName != GatewayClassName {
			return false
		}
		return true

	case *gwapiv1.HTTPRoute:
		return len(resource.Spec.ParentRefs) > 0

	case *gwapiv1.GRPCRoute:
		return len(resource.Spec.ParentRefs) > 0

	default:
		return false
	}
}
