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

package envoy

import (
	"encoding/json"
	"slices"
	"testing"

	"k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	portlookup "k8c.io/kubelb/internal/port-lookup"
)

// bootstrapListenerPorts returns the port of every listener in the generated
// bootstrap, including the admin interface.
func bootstrapListenerPorts(t *testing.T) []int {
	t.Helper()
	server, err := NewServer("127.0.0.1:0", false)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	var bootstrap struct {
		Admin struct {
			Address bootstrapAddress `json:"address"`
		} `json:"admin"`
		StaticResources struct {
			Listeners []struct {
				Name    string           `json:"name"`
				Address bootstrapAddress `json:"address"`
			} `json:"listeners"`
		} `json:"staticResources"`
	}
	if err := json.Unmarshal([]byte(server.GenerateBootstrap(&v1alpha1.Config{})), &bootstrap); err != nil {
		t.Fatalf("failed to parse the bootstrap: %v", err)
	}

	ports := []int{bootstrap.Admin.Address.SocketAddress.PortValue}
	for _, l := range bootstrap.StaticResources.Listeners {
		// Envoy defaults an omitted socket protocol to TCP.
		if protocol := l.Address.SocketAddress.Protocol; protocol != "" && protocol != "TCP" {
			t.Fatalf("static listener %q uses %s, update the allocator's static port policy", l.Name, protocol)
		}
		ports = append(ports, l.Address.SocketAddress.PortValue)
	}
	if len(bootstrap.StaticResources.Listeners) == 0 || slices.Contains(ports, 0) {
		t.Fatalf("could not read the listener ports from the bootstrap: %v", ports)
	}
	return ports
}

type bootstrapAddress struct {
	SocketAddress struct {
		Protocol  string `json:"protocol"`
		PortValue int    `json:"portValue"`
	} `json:"socketAddress"`
}

// TestStaticPortsCoverBootstrap fails when a static listener is added to the
// bootstrap without adding its port to StaticTCPPorts. The port allocator only knows
// about the ports in that list.
func TestStaticPortsCoverBootstrap(t *testing.T) {
	static := StaticTCPPorts()
	for _, port := range bootstrapListenerPorts(t) {
		if !slices.Contains(static, port) {
			t.Errorf("bootstrap listener port %d is missing from StaticTCPPorts()", port)
		}
	}
	if !slices.Contains(static, ShutdownManagerPort) {
		t.Errorf("shutdown manager port %d is missing from StaticTCPPorts()", ShutdownManagerPort)
	}
}

// TestStaticPortsAreNeverAllocatable asserts every static port is either outside
// the allocator's range or inside its reserved block, so that an allocator built
// without the list cannot hand one out either. A dynamic listener on a static
// port is rejected by Envoy, and the Service that targets the port then reaches
// the static listener instead of its backend (kubermatic/kubelb#661).
func TestStaticPortsAreNeverAllocatable(t *testing.T) {
	for _, port := range StaticTCPPorts() {
		if portlookup.IsAllocatable(port) {
			t.Errorf("static port %d can be handed out by the port allocator; move it into the reserved block", port)
		}
	}
}
