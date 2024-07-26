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

package httproute

import (
	"k8c.io/kubelb/internal/kubelb"

	"k8s.io/apimachinery/pkg/types"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GetServicesFromHTTPRoute returns a list of services referenced by the given HTTPRoute.
func GetServicesFromHTTPRoute(httpRoute *gwapiv1.HTTPRoute) []types.NamespacedName {
	serviceReferences := make([]types.NamespacedName, 0)
	for _, rule := range httpRoute.Spec.Rules {
		// Collect services from the filters.
		for _, filter := range rule.Filters {
			if filter.RequestMirror != nil && (filter.RequestMirror.BackendRef.Kind == nil || *filter.RequestMirror.BackendRef.Kind == kubelb.ServiceKind) {
				ref := filter.RequestMirror.BackendRef
				serviceReference := types.NamespacedName{
					Name: string(ref.Name),
				}

				if ref.Namespace != nil {
					serviceReference.Namespace = string(*ref.Namespace)
				}
				serviceReferences = append(serviceReferences, serviceReference)
			}
		}

		// Collect services from the backend references.
		for _, ref := range rule.BackendRefs {
			if ref.Kind == nil || *ref.Kind == kubelb.ServiceKind {
				serviceReference := types.NamespacedName{
					Name: string(ref.Name),
				}

				if ref.Namespace != nil {
					serviceReference.Namespace = string(*ref.Namespace)
				}
				serviceReferences = append(serviceReferences, serviceReference)
			}

			// Collect services from the filters.
			if ref.Filters != nil {
				for _, filter := range ref.Filters {
					if filter.RequestMirror != nil && (filter.RequestMirror.BackendRef.Kind == nil || *filter.RequestMirror.BackendRef.Kind == kubelb.ServiceKind) {
						ref := filter.RequestMirror.BackendRef
						serviceReference := types.NamespacedName{
							Name: string(ref.Name),
						}

						if ref.Namespace != nil {
							serviceReference.Namespace = string(*ref.Namespace)
						}
						serviceReferences = append(serviceReferences, serviceReference)
					}
				}
			}
		}
	}

	// Remove duplicates from the list.
	keys := make(map[types.NamespacedName]bool)
	list := []types.NamespacedName{}
	for _, entry := range serviceReferences {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
