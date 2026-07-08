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

package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEditionCache(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create a test kubeconfig file
	testKubeconfig := filepath.Join(tmpDir, "kubeconfig")
	kubeconfigContent := `
apiVersion: v1
kind: Config
current-context: test-context
contexts:
- name: test-context
  context:
    cluster: test-cluster
    user: test-user
clusters:
- name: test-cluster
  cluster:
    server: https://localhost:6443
users:
- name: test-user
  user:
    token: test-token
`
	if err := os.WriteFile(testKubeconfig, []byte(kubeconfigContent), 0644); err != nil {
		t.Fatalf("Failed to create test kubeconfig: %v", err)
	}

	// Test saving and loading cache
	tenant := "test-tenant"
	edition := "EE"

	// Initially, cache should be empty
	cached := LoadCache(testKubeconfig, tenant)
	if cached != "" {
		t.Errorf("Expected empty cache, got %s", cached)
	}

	// Save to cache
	if err := SaveCache(testKubeconfig, tenant, edition); err != nil {
		t.Fatalf("Failed to save cache: %v", err)
	}

	// Load from cache should return the saved value
	cached = LoadCache(testKubeconfig, tenant)
	if cached != edition {
		t.Errorf("Expected cached edition %s, got %s", edition, cached)
	}

	// Test with different tenant - should have separate cache
	tenant2 := "another-tenant"
	cached2 := LoadCache(testKubeconfig, tenant2)
	if cached2 != "" {
		t.Errorf("Expected empty cache for different tenant, got %s", cached2)
	}

	// Save different edition for tenant2
	edition2 := "CE"
	if err := SaveCache(testKubeconfig, tenant2, edition2); err != nil {
		t.Fatalf("Failed to save cache for tenant2: %v", err)
	}

	// Verify both caches are independent
	cached = LoadCache(testKubeconfig, tenant)
	if cached != edition {
		t.Errorf("Expected cached edition %s for tenant1, got %s", edition, cached)
	}

	cached2 = LoadCache(testKubeconfig, tenant2)
	if cached2 != edition2 {
		t.Errorf("Expected cached edition %s for tenant2, got %s", edition2, cached2)
	}
}

func TestCacheExpiration(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create a test kubeconfig file
	testKubeconfig := filepath.Join(tmpDir, "kubeconfig")
	kubeconfigContent := `
apiVersion: v1
kind: Config
current-context: test-context
contexts:
- name: test-context
  context:
    cluster: test-cluster
    user: test-user
clusters:
- name: test-cluster
  cluster:
    server: https://localhost:6443
users:
- name: test-user
  user:
    token: test-token
`
	if err := os.WriteFile(testKubeconfig, []byte(kubeconfigContent), 0644); err != nil {
		t.Fatalf("Failed to create test kubeconfig: %v", err)
	}

	tenant := "test-tenant"
	edition := "EE"

	// Save cache with an old timestamp
	cacheFile, err := getCacheFilePath(testKubeconfig, tenant)
	if err != nil {
		t.Fatalf("Failed to get cache file path: %v", err)
	}

	// Create cache directory
	cacheDir := filepath.Dir(cacheFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Create an expired cache entry (older than 48 hours)
	oldCache := EditionCache{
		Edition:   edition,
		Timestamp: time.Now().Add(-49 * time.Hour),
	}

	// Write the expired cache manually
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	data, err := oldCache.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal cache: %v", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	// Load should return empty string because cache is expired
	cached := LoadCache(testKubeconfig, tenant)
	if cached != "" {
		t.Errorf("Expected empty cache for expired entry, got %s", cached)
	}
}

// Helper method for marshaling
func (e EditionCache) MarshalJSON() ([]byte, error) {
	type Alias EditionCache
	return json.Marshal(&struct {
		Alias
	}{
		Alias: (Alias)(e),
	})
}
