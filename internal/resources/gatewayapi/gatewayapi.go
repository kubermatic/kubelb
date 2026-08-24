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

// RewriteServiceBackendRef points ref at the service that was created against the Route in the LB
// cluster, provided ref resolves to one of referencedServices. Non-service references are ignored.
func RewriteServiceBackendRef(ref *gwapiv1.BackendObjectReference, referencedServices []metav1.ObjectMeta, routeName string) {
	if ref.Kind != nil && *ref.Kind != kubelb.ServiceKind {
		return
	}

	name, namespace := ref.Name, ref.Namespace
	for _, service := range referencedServices {
		if string(name) != service.Name {
			continue
		}
		if namespace != nil && string(*namespace) != service.Namespace {
			continue
		}
		ref.Name = gwapiv1.ObjectName(kubelb.GenerateRouteServiceName(routeName, service.Name, service.Namespace))
		// Set the namespace to nil since all the services are created in the same namespace as the Route.
		ref.Namespace = nil
	}
}
