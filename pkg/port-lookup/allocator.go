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
	"encoding/json"
	"math/rand"
	"sync"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	GlobalLookupConfigmapName string = "kubelb-port-lookup"
	GlobalLookupConfigmapKey  string = "LOOKUP_TABLE"
	startPort                 int    = 10000
	endPort                   int    = 65535
)

// LookupTable is a lookup table for ports. It maps endpoint keys to port keys to the actual allocated ports.
type LookupTable map[string]map[string]int

type PortAllocator struct {
	mu sync.Mutex

	client     client.Client
	namespace  string
	portLookup LookupTable
	// portLookupReverse is a reverse lookup table for available ports. It is used to quickly determine if a port is available.
	portLookupReverse map[int]bool
}

func NewPortAllocator(client client.Client, namespace string) *PortAllocator {
	pa := &PortAllocator{
		portLookup:        make(LookupTable),
		portLookupReverse: make(map[int]bool),
		client:            client,
		namespace:         namespace,
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
	// TODO: We should probably do something smarter here. The infinite loop is a bit scary.
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
}

// LoadState loads the port lookup table from the global configmap.
func (pa *PortAllocator) LoadState(ctx context.Context, apiReader client.Reader) error {
	lookupTable := make(LookupTable)
	lookupConfigmap := &corev1.ConfigMap{}

	// We use the API reader here because the cache may not be fully synced yet.
	err := apiReader.Get(ctx, types.NamespacedName{
		Namespace: pa.namespace,
		Name:      GlobalLookupConfigmapName,
	}, lookupConfigmap)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		lookupConfigmap.Name = GlobalLookupConfigmapName
		lookupConfigmap.Namespace = pa.namespace
		lookupConfigmap.Data = map[string]string{
			GlobalLookupConfigmapKey: "{}",
		}
		err = pa.client.Create(ctx, lookupConfigmap)
		if err != nil {
			return err
		}
	} else {
		err = json.Unmarshal([]byte(lookupConfigmap.Data[GlobalLookupConfigmapKey]), &lookupTable)
		if err != nil {
			return err
		}
	}

	pa.portLookup = lookupTable
	pa.recomputeAvailablePorts()
	return nil
}

// UpdateState updates the global configmap with the current port lookup table.
func (pa *PortAllocator) UpdateState(ctx context.Context) error {
	lookupConfigmap := &corev1.ConfigMap{}
	err := pa.client.Get(ctx, types.NamespacedName{
		Namespace: pa.namespace,
		Name:      GlobalLookupConfigmapName,
	}, lookupConfigmap)
	if err != nil {
		return err
	}

	lookupBytes, err := json.Marshal(pa.portLookup)
	if err != nil {
		return err
	}

	lookupConfigmap.Data[GlobalLookupConfigmapKey] = string(lookupBytes)
	err = pa.client.Update(ctx, lookupConfigmap)
	if err != nil {
		return err
	}
	return nil
}
