/*
Copyright 2024 The KubeLB Authors.

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

package gatewayapi

import (
	"k8c.io/kubelb/internal/kubelb"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func NormalizeParentRefs(parentRefs []gwapiv1.ParentReference) []gwapiv1.ParentReference {
	for i := range parentRefs {
		parentRefs[i].Namespace = nil
	}
	return parentRefs
}

// RetargetBackendRefToGeneratedService rewrites a BackendObjectReference to point at the
// per-route Service generated in the LB cluster.
func RetargetBackendRefToGeneratedService(ref *gwapiv1.BackendObjectReference, referencedServices []metav1.ObjectMeta, routeName string) {
	for _, service := range referencedServices {
		if string(ref.Name) != service.Name {
			continue
		}
		if ref.Namespace != nil && string(*ref.Namespace) != service.Namespace {
			continue
		}
		ref.Name = gwapiv1.ObjectName(kubelb.GenerateRouteServiceName(routeName, service.Name, service.Namespace))
		ref.Namespace = nil
		return
	}
}
