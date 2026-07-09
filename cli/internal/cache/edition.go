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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8c.io/kubelb/cli/internal/logger"

	"k8s.io/client-go/tools/clientcmd"
)

const (
	// CacheTTL defines how long the edition cache is valid
	CacheTTL = 48 * time.Hour
)

// EditionCache represents the cached edition information
type EditionCache struct {
	Edition   string    `json:"edition"`
	Timestamp time.Time `json:"timestamp"`
}

// LoadCache attempts to load cached edition information
// Returns the cached edition if valid, or empty string if cache miss/expired
func LoadCache(kubeconfig, tenant string) string {
	log := logger.Get()

	cacheFile, err := getCacheFilePath(kubeconfig, tenant)
	if err != nil {
		log.Debug("Failed to get cache file path", "error", err)
		return ""
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Debug("Failed to read cache file", "error", err)
		}
		return ""
	}

	var cache EditionCache
	if err := json.Unmarshal(data, &cache); err != nil {
		log.Debug("Failed to unmarshal cache", "error", err)
		return ""
	}

	// Check if cache is still valid
	if time.Since(cache.Timestamp) > CacheTTL {
		log.Debug("Cache expired", "age", time.Since(cache.Timestamp))
		return ""
	}

	log.Debug("Using cached edition", "edition", cache.Edition, "age", time.Since(cache.Timestamp))
	return cache.Edition
}

// SaveCache saves the detected edition to cache
func SaveCache(kubeconfig, tenant, edition string) error {
	log := logger.Get()

	cacheFile, err := getCacheFilePath(kubeconfig, tenant)
	if err != nil {
		return fmt.Errorf("failed to get cache file path: %w", err)
	}

	// Ensure cache directory exists
	cacheDir := filepath.Dir(cacheFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := EditionCache{
		Edition:   edition,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	log.Debug("Saved edition to cache", "edition", edition, "file", cacheFile)
	return nil
}

// getCacheFilePath generates a unique cache file path based on kubeconfig and tenant
func getCacheFilePath(kubeconfig, tenant string) (string, error) {
	// Get the current context from kubeconfig
	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return "", fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	currentContext := config.CurrentContext
	if currentContext == "" {
		return "", fmt.Errorf("no current context in kubeconfig")
	}

	// Create a unique identifier for this combination
	identifier := fmt.Sprintf("%s-%s-%s", kubeconfig, currentContext, tenant)
	hash := sha256.Sum256([]byte(identifier))
	hashStr := fmt.Sprintf("%x", hash)[:16] // Use first 16 chars of hash

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Build cache file path
	cacheFile := filepath.Join(homeDir, ".kubelb", "cache", fmt.Sprintf("edition-%s.json", hashStr))
	return cacheFile, nil
}
