//go:build !e2e

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
	// MaxGatewaysPerTenant is the maximum number of Gateway objects allowed per tenant.
	MaxGatewaysPerTenant = 1

	// ParentGatewayName is the required gateway name in CE.
	ParentGatewayName = "kubelb"

	// GatewayClassName is the required gateway class name.
	GatewayClassName = "kubelb"
)

// ShouldReconcileGatewayByName checks if a Gateway should be reconciled based on naming rules.
// CE: enforces strict naming - gateway must be named "kubelb".
func ShouldReconcileGatewayByName(gateway *gwapiv1.Gateway) bool {
	return gateway.Name == ParentGatewayName
}

// ShouldReconcileResource checks if a Gateway API resource should be reconciled.
// Enforces strict naming - gateway must be named "kubelb", routes must reference it.
func ShouldReconcileResource(obj ctrlclient.Object, useGatewayClass bool) bool {
	switch resource := obj.(type) {
	case *gwapiv1.Gateway:
		if resource.Name != ParentGatewayName {
			return false
		}
		if useGatewayClass && resource.Spec.GatewayClassName != GatewayClassName {
			return false
		}
		return true

	case *gwapiv1.HTTPRoute:
		if len(resource.Spec.ParentRefs) == 1 && resource.Spec.ParentRefs[0].Name == ParentGatewayName {
			return true
		}
		return false

	case *gwapiv1.GRPCRoute:
		if len(resource.Spec.ParentRefs) == 1 && resource.Spec.ParentRefs[0].Name == ParentGatewayName {
			return true
		}
		return false

	default:
		return false
	}
}
