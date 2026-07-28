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
	"context"
	"testing"
	"time"

	"k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
)

// On shutdown the xDS server must drain its streams. Without it the process
// exits with every ADS stream still open, so each connected proxy sees a reset
// mid-response and the whole fleet reconnects at once.
func TestServerStartDrainsOnContextCancellation(t *testing.T) {
	server, err := NewServer(&v1alpha1.Config{}, "127.0.0.1:0", false)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Start(ctx) }()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start() returned error = %v, want clean shutdown", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Start() did not return after context cancellation")
	}
}
