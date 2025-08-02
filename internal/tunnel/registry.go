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

package tunnel

import (
	"context"
	"fmt"
	"sync"
)

// Connection represents an active tunnel connection
type Connection struct {
	Hostname    string
	TargetPort  string
	Token       string
	RequestChan chan *HTTPRequest
	Context     context.Context
	Cancel      context.CancelFunc
}

// Registry manages active tunnel connections
type Registry struct {
	// Map: hostname -> active tunnel connection
	activeTunnels map[string]*Connection
	// Mutex for thread-safe access
	mutex sync.RWMutex
}

// NewRegistry creates a new tunnel registry
func NewRegistry() *Registry {
	return &Registry{
		activeTunnels: make(map[string]*Connection),
	}
}

// RegisterTunnel registers a new tunnel connection. This enforces one active tunnel per hostname.
func (tr *Registry) RegisterTunnel(ctx context.Context, hostname, token, targetPort string, requestChan chan *HTTPRequest) error {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()

	fmt.Printf("[REGISTRY] Registering tunnel: hostname=%q, targetPort=%s, hasToken=%v\n", hostname, targetPort, token != "")

	// Close existing tunnel if any.
	if existingTunnel, exists := tr.activeTunnels[hostname]; exists {
		fmt.Printf("[REGISTRY] Closing existing tunnel for hostname %q\n", hostname)
		existingTunnel.Cancel()
		delete(tr.activeTunnels, hostname)
	}

	// Create context with cancel for this tunnel
	tunnelCtx, cancel := context.WithCancel(ctx)

	// Register new tunnel
	tr.activeTunnels[hostname] = &Connection{
		Hostname:    hostname,
		TargetPort:  targetPort,
		Token:       token,
		RequestChan: requestChan,
		Context:     tunnelCtx,
		Cancel:      cancel,
	}

	allTunnels := make([]string, 0, len(tr.activeTunnels))
	for h := range tr.activeTunnels {
		allTunnels = append(allTunnels, h)
	}
	fmt.Printf("[REGISTRY] Tunnel registered successfully: hostname=%q, totalTunnels=%d, allTunnels=%v\n", hostname, len(tr.activeTunnels), allTunnels)

	return nil
}

// GetActiveTunnel retrieves an active tunnel by hostname
func (tr *Registry) GetActiveTunnel(hostname string) (*Connection, bool) {
	tr.mutex.RLock()
	defer tr.mutex.RUnlock()

	allTunnels := make([]string, 0, len(tr.activeTunnels))
	for h := range tr.activeTunnels {
		allTunnels = append(allTunnels, h)
	}

	tunnel, exists := tr.activeTunnels[hostname]
	fmt.Printf("[REGISTRY] GetActiveTunnel: hostname=%q, found=%v, totalTunnels=%d, allTunnels=%v\n", hostname, exists, len(tr.activeTunnels), allTunnels)

	return tunnel, exists
}

// UnregisterTunnel removes a tunnel from the registry
func (tr *Registry) UnregisterTunnel(hostname string) {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()

	if tunnel, exists := tr.activeTunnels[hostname]; exists {
		tunnel.Cancel()
		delete(tr.activeTunnels, hostname)
	}
}

// GetAllTunnels returns all active tunnel hostnames
func (tr *Registry) GetAllTunnels() []string {
	tr.mutex.RLock()
	defer tr.mutex.RUnlock()

	hostnames := make([]string, 0, len(tr.activeTunnels))
	for hostname := range tr.activeTunnels {
		hostnames = append(hostnames, hostname)
	}
	return hostnames
}

// ValidateToken validates if a token has access to a hostname
func (tr *Registry) ValidateToken(token, hostname string) error {
	tr.mutex.RLock()
	defer tr.mutex.RUnlock()

	tunnel, exists := tr.activeTunnels[hostname]
	if !exists {
		return fmt.Errorf("no active tunnel for hostname: %s", hostname)
	}

	if tunnel.Token != token {
		return fmt.Errorf("invalid token for hostname: %s", hostname)
	}

	return nil
}
