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
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CachedTunnelAuth represents cached tunnel authentication data with TTL
type CachedTunnelAuth struct {
	Token       string
	TokenExpiry time.Time
	Hostname    string
	TenantName  string
	TunnelName  string
	CachedAt    time.Time
	TTL         time.Duration
}

// IsValid checks if the cached entry is still valid (not expired)
func (c *CachedTunnelAuth) IsValid() bool {
	return time.Now().Before(c.CachedAt.Add(c.TTL)) && time.Now().Before(c.TokenExpiry)
}

// Cache manages cached tunnel authentication data
type Cache struct {
	cache      sync.Map
	defaultTTL time.Duration
	hits       int64 // atomic counter for cache hits
	misses     int64 // atomic counter for cache misses
	evictions  int64 // atomic counter for evictions
}

// NewTunnelCache creates a new tunnel cache with default TTL
func NewTunnelCache(defaultTTL time.Duration) *Cache {
	if defaultTTL == 0 {
		defaultTTL = 5 * time.Minute // Default 5 minutes
	}
	return &Cache{
		defaultTTL: defaultTTL,
	}
}

// cacheKey generates a cache key for tunnel authentication
func (tc *Cache) cacheKey(tenantName, tunnelName string) string {
	return fmt.Sprintf("%s/%s", tenantName, tunnelName)
}

// Get retrieves cached tunnel auth data if valid
func (tc *Cache) Get(tenantName, tunnelName string) (*CachedTunnelAuth, bool) {
	key := tc.cacheKey(tenantName, tunnelName)
	if value, ok := tc.cache.Load(key); ok {
		cached := value.(*CachedTunnelAuth)
		if cached.IsValid() {
			atomic.AddInt64(&tc.hits, 1)
			return cached, true
		}
		// Entry expired, remove it
		tc.cache.Delete(key)
		atomic.AddInt64(&tc.evictions, 1)
	}
	atomic.AddInt64(&tc.misses, 1)
	return nil, false
}

// Set stores tunnel auth data in cache with TTL
func (tc *Cache) Set(tenantName, tunnelName, token, hostname string, tokenExpiry time.Time) {
	key := tc.cacheKey(tenantName, tunnelName)
	cached := &CachedTunnelAuth{
		Token:       token,
		TokenExpiry: tokenExpiry,
		Hostname:    hostname,
		TenantName:  tenantName,
		TunnelName:  tunnelName,
		CachedAt:    time.Now(),
		TTL:         tc.defaultTTL,
	}
	tc.cache.Store(key, cached)
}

// Delete removes an entry from cache
func (tc *Cache) Delete(tenantName, tunnelName string) {
	key := tc.cacheKey(tenantName, tunnelName)
	if _, ok := tc.cache.LoadAndDelete(key); ok {
		atomic.AddInt64(&tc.evictions, 1)
	}
}

// EvictExpired removes all expired entries from cache
func (tc *Cache) EvictExpired() int {
	var evicted int
	tc.cache.Range(func(key, value interface{}) bool {
		cached := value.(*CachedTunnelAuth)
		if !cached.IsValid() {
			tc.cache.Delete(key)
			evicted++
		}
		return true
	})
	atomic.AddInt64(&tc.evictions, int64(evicted))
	return evicted
}

// GetStats returns cache statistics
func (tc *Cache) GetStats() map[string]interface{} {
	var size int
	tc.cache.Range(func(_, _ interface{}) bool {
		size++
		return true
	})

	hits := atomic.LoadInt64(&tc.hits)
	misses := atomic.LoadInt64(&tc.misses)
	total := hits + misses

	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"size":        size,
		"hits":        hits,
		"misses":      misses,
		"evictions":   atomic.LoadInt64(&tc.evictions),
		"hit_rate":    hitRate,
		"ttl_seconds": tc.defaultTTL.Seconds(),
	}
}
