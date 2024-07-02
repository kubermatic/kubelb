/*
Copyright 2023 The KubeLB Authors.

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

package portlookup

import (
	"fmt"

	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
)

// AllocatePortsForLoadBalancers allocates ports to the given load balancers. If a port is already allocated, it will be skipped.
func (pa *PortAllocator) AllocatePortsForLoadBalancers(loadBalancers kubelbv1alpha1.LoadBalancerList) error {
	for _, lb := range loadBalancers.Items {
		for i, lbEndpoint := range lb.Spec.Endpoints {
			endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointPattern, lb.Namespace, lb.Name, i)

			var keys []string
			for _, lbEndpointPort := range lbEndpoint.Ports {
				keys = append(keys, fmt.Sprintf(kubelb.EnvoyListenerPattern, lbEndpointPort.Port, lbEndpointPort.Protocol))
			}
			// If a port is already allocated, it will be skipped.
			pa.AllocatePorts(endpointKey, keys)
		}
	}
	return nil
}

// AllocatePortsForRoutes allocates ports for the routes. If a port is already allocated, it will be skipped.
func (pa *PortAllocator) AllocatePortsForRoutes(routes []kubelbv1alpha1.Route) error {
	for _, route := range routes {
		if route.Spec.Source.Kubernetes == nil {
			continue
		}

		for _, svc := range route.Spec.Source.Kubernetes.Services {
			endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointRoutePattern, route.Namespace, svc.Namespace, svc.Name)
			var keys []string
			for _, svcPort := range svc.Spec.Ports {
				keys = append(keys, fmt.Sprintf(kubelb.EnvoyListenerPattern, svcPort.Port, svcPort.Protocol))
			}
			// If a port is already allocated, it will be skipped.
			pa.AllocatePorts(endpointKey, keys)
		}
	}
	return nil
}

// DeallocatePortsForLoadBalancer deallocates ports against the given load balancer.
func (pa *PortAllocator) DeallocatePortsForLoadBalancer(loadBalancer kubelbv1alpha1.LoadBalancer) error {
	var endpointKeys []string

	for i := range loadBalancer.Spec.Endpoints {
		endpointKeys = append(endpointKeys, fmt.Sprintf(kubelb.EnvoyEndpointPattern, loadBalancer.Namespace, loadBalancer.Name, i))
	}

	pa.DeallocateEndpoints(endpointKeys)
	return nil
}
