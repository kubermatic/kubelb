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

package route

import (
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
)

// GetServicesFromIngress returns a list of services referenced by the given Ingress.
func GetServicesFromIngress(ingress networkingv1.Ingress) []types.NamespacedName {
	serviceReferences := make([]types.NamespacedName, 0)
	for _, rule := range ingress.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			serviceReferences = append(serviceReferences, types.NamespacedName{
				Name:      path.Backend.Service.Name,
				Namespace: ingress.Namespace,
			})
		}
	}

	if ingress.Spec.DefaultBackend != nil && ingress.Spec.DefaultBackend.Service != nil {
		serviceReferences = append(serviceReferences, types.NamespacedName{
			Name:      ingress.Spec.DefaultBackend.Service.Name,
			Namespace: ingress.Namespace,
		})
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
