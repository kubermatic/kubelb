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

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Manager wraps the ConnectionManager to implement the controller-runtime Runnable interface
type Manager struct {
	connectionManager *ConnectionManager
}

// NewManager creates a new tunnel manager that can be added to controller-runtime manager
func NewManager(config *ConnectionManagerConfig) (*Manager, error) {
	cm, err := NewConnectionManager(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection manager: %w", err)
	}

	return &Manager{
		connectionManager: cm,
	}, nil
}

// Start implements the Runnable interface for controller-runtime
func (m *Manager) Start(ctx context.Context) error {
	return m.connectionManager.Start(ctx)
}

// NeedLeaderElection implements the LeaderElectionRunnable interface
// Returns false as the tunnel server should run on all replicas for high availability
func (m *Manager) NeedLeaderElection() bool {
	return false
}

// GetConnectionManager returns the underlying connection manager
func (m *Manager) GetConnectionManager() *ConnectionManager {
	return m.connectionManager
}

// AddToManager adds the tunnel manager to the controller-runtime manager
func AddToManager(mgr manager.Manager, config *ConnectionManagerConfig) error {
	tm, err := NewManager(config)
	if err != nil {
		return err
	}

	return mgr.Add(tm)
}
