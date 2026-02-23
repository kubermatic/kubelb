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
	"context"
	"fmt"
	"math/rand"
	"slices"
	"sync"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	managermetrics "k8c.io/kubelb/internal/metricsutil/manager"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	startPort int = 10000
	endPort   int = 65535
)

// LookupTable is a lookup table for ports. It maps endpoint keys to port keys to the actual allocated ports.
type LookupTable map[string]map[string]int

type PortAllocator struct {
	mu sync.Mutex

	portLookup LookupTable
	// portLookupReverse is a reverse lookup table for available ports. It is used to quickly determine if a port is available.
	portLookupReverse map[int]bool
}

func NewPortAllocator() *PortAllocator {
	pa := &PortAllocator{
		portLookup:        make(LookupTable),
		portLookupReverse: make(map[int]bool),
	}
	return pa
}

func (pa *PortAllocator) GetPortLookupTable() LookupTable {
	return pa.portLookup
}
func (pa *PortAllocator) Lookup(endpointKey, portKey string) (int, bool) {
	if endpointLookup, exists := pa.portLookup[endpointKey]; exists {
		if port, exists := endpointLookup[portKey]; exists {
			return port, true
		}
	}
	return -1, false
}

// AllocatePorts allocates ports for the given keys. If a key already exists in the lookup table, it is ignored.
func (pa *PortAllocator) AllocatePorts(endpointKey string, portkeys []string) bool {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	if _, exists := pa.portLookup[endpointKey]; !exists {
		pa.portLookup[endpointKey] = make(map[string]int)
	}

	// Ensure that ports are allocated for all keys.
	updated := false
	for _, k := range portkeys {
		if _, exists := pa.portLookup[endpointKey][k]; !exists {
			pa.portLookup[endpointKey][k] = pa.allocatePort()
			updated = true
		}
	}

	// Remove ports that are no longer needed.
	for k := range pa.portLookup[endpointKey] {
		if !slices.Contains(portkeys, k) {
			delete(pa.portLookup[endpointKey], k)
			updated = true
		}
	}

	pa.recomputeAvailablePorts()
	return updated
}

// DeallocatePorts deallocates ports for the given keys. If a key does not exist in the lookup table, it is ignored.
func (pa *PortAllocator) DeallocatePorts(endpointKey string, portkeys []string) bool {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	// Remove ports that are no longer needed.
	updated := false
	if _, exists := pa.portLookup[endpointKey]; exists {
		for _, k := range portkeys {
			delete(pa.portLookup[endpointKey], k)
			updated = true
		}
	}

	pa.recomputeAvailablePorts()
	return updated
}

// DeallocateEndpoints deallocates all ports for the given endpoint key. If the endpoint key does not exist in the lookup table, it is ignored
func (pa *PortAllocator) DeallocateEndpoints(endpointKeys []string) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	// Remove endpoints which would result in all ports against them being deallocated.
	for _, k := range endpointKeys {
		delete(pa.portLookup, k)
	}

	pa.recomputeAvailablePorts()
}

func (pa *PortAllocator) allocatePort() int {
	for {
		port := rand.Intn(endPort-startPort) + startPort
		if _, exists := pa.portLookupReverse[port]; !exists {
			return port
		}
	}
}

// recomputeAvailablePorts recomputes the reverse lookup table for available ports.
func (pa *PortAllocator) recomputeAvailablePorts() {
	pa.portLookupReverse = make(map[int]bool)
	for _, ep := range pa.portLookup {
		for _, port := range ep {
			pa.portLookupReverse[port] = true
		}
	}

	// Update metrics
	managermetrics.PortAllocatorAllocatedPorts.Set(float64(len(pa.portLookupReverse)))
	managermetrics.PortAllocatorEndpoints.Set(float64(len(pa.portLookup)))
}

// LoadState loads the port lookup table from the existing loadbalancers.
func (pa *PortAllocator) LoadState(ctx context.Context, apiReader client.Reader) error {
	lookupTable := make(LookupTable)

	// We use the API reader here because the cache may not be fully synced yet.
	loadBalancers := &kubelbv1alpha1.LoadBalancerList{}
	err := apiReader.List(ctx, loadBalancers)
	if err != nil {
		return err
	}

	for _, lb := range loadBalancers.Items {
		for i, lbEndpoint := range lb.Spec.Endpoints {
			endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointPattern, lb.Namespace, lb.Name, i)
			if _, exists := lookupTable[endpointKey]; !exists {
				lookupTable[endpointKey] = make(map[string]int)
			}

			for _, lbEndpointPort := range lbEndpoint.Ports {
				portKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, lbEndpointPort.Port, lbEndpointPort.Protocol)
				for _, port := range lb.Status.Service.Ports {
					// Name is not guaranteed to be set, so we need to check for port and protocol as well.
					if port.UpstreamTargetPort == lbEndpointPort.Port && port.Protocol == lbEndpointPort.Protocol && port.Name == lbEndpointPort.Name {
						lookupTable[endpointKey][portKey] = port.TargetPort.IntValue()
						break
					}
				}
			}
		}
	}

	routes := &kubelbv1alpha1.RouteList{}
	err = apiReader.List(ctx, routes)
	if err != nil {
		return err
	}

	for _, route := range routes.Items {
		if route.Spec.Source.Kubernetes == nil {
			continue
		}
		// Get original route name from source or labels
		originalRouteName := route.Name
		if route.Spec.Source.Kubernetes.Route.GetName() != "" {
			originalRouteName = route.Spec.Source.Kubernetes.Route.GetName()
		} else if name := route.GetLabels()[kubelb.LabelOriginName]; name != "" {
			originalRouteName = name
		}
		for _, svc := range route.Spec.Source.Kubernetes.Services {
			endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointRoutePattern, route.Namespace, svc.Namespace, svc.Name, originalRouteName)
			if _, exists := lookupTable[endpointKey]; !exists {
				lookupTable[endpointKey] = make(map[string]int)
			}

			// The assigned port is stored in the status of the service.
			if route.Status.Resources.Services != nil {
				key := fmt.Sprintf(kubelb.RouteServiceMapKey, svc.Namespace, kubelb.GetName(&svc))
				if svcPort, exists := route.Status.Resources.Services[key]; exists {
					for _, port := range svcPort.Ports {
						portKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, port.Port, port.Protocol)
						lookupTable[endpointKey][portKey] = port.TargetPort.IntValue()
					}
				}
			}
		}
	}
	pa.portLookup = lookupTable
	pa.recomputeAvailablePorts()
	return nil
}
