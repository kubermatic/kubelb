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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// HTTPRequest represents an incoming HTTP request to be forwarded
type HTTPRequest struct {
	RequestID string            `json:"request_id"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"` // base64 encoded
}

// HTTPResponse represents the response from the local service
type HTTPResponse struct {
	RequestID  string            `json:"request_id"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"` // base64 encoded
}

// Error represents an error in tunnel communication
type Error struct {
	RequestID    string `json:"request_id"`
	ErrorMessage string `json:"error_message"`
	ErrorCode    int    `json:"error_code"`
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int32

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// CircuitBreaker implements circuit breaker pattern for tunnel connections
type CircuitBreaker struct {
	state         int32 // atomic access to CircuitBreakerState
	failureCount  int32 // atomic
	lastFailTime  int64 // atomic, unix timestamp
	successCount  int32 // atomic, for half-open state
	maxFailures   int32
	timeout       time.Duration // time to wait before trying half-open
	halfOpenLimit int32         // max requests to allow in half-open state
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int32, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:         int32(CircuitBreakerClosed),
		maxFailures:   maxFailures,
		timeout:       timeout,
		halfOpenLimit: 3, // Allow 3 requests in half-open state
	}
}

// CanRequest checks if a request can be made
func (cb *CircuitBreaker) CanRequest() bool {
	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))
	now := time.Now().Unix()

	switch state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		lastFail := atomic.LoadInt64(&cb.lastFailTime)
		if now-lastFail > int64(cb.timeout.Seconds()) {
			// Try to transition to half-open
			if atomic.CompareAndSwapInt32(&cb.state, int32(CircuitBreakerOpen), int32(CircuitBreakerHalfOpen)) {
				atomic.StoreInt32(&cb.successCount, 0)
			}
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		return atomic.LoadInt32(&cb.successCount) < cb.halfOpenLimit
	default:
		return false
	}
}

// OnSuccess records a successful request
func (cb *CircuitBreaker) OnSuccess() {
	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))

	switch state {
	case CircuitBreakerClosed:
		atomic.StoreInt32(&cb.failureCount, 0)
	case CircuitBreakerHalfOpen:
		successCount := atomic.AddInt32(&cb.successCount, 1)
		if successCount >= cb.halfOpenLimit {
			// Transition back to closed
			atomic.StoreInt32(&cb.state, int32(CircuitBreakerClosed))
			atomic.StoreInt32(&cb.failureCount, 0)
		}
	}
}

// OnFailure records a failed request
func (cb *CircuitBreaker) OnFailure() {
	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))
	now := time.Now().Unix()

	switch state {
	case CircuitBreakerClosed:
		failures := atomic.AddInt32(&cb.failureCount, 1)
		if failures >= cb.maxFailures {
			atomic.StoreInt32(&cb.state, int32(CircuitBreakerOpen))
			atomic.StoreInt64(&cb.lastFailTime, now)
		}
	case CircuitBreakerHalfOpen:
		// Transition back to open on any failure
		atomic.StoreInt32(&cb.state, int32(CircuitBreakerOpen))
		atomic.StoreInt64(&cb.lastFailTime, now)
		atomic.StoreInt32(&cb.failureCount, 1)
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	return CircuitBreakerState(atomic.LoadInt32(&cb.state))
}

// Health tracks health metrics for tunnels
type Health struct {
	hostname           string
	circuitBreaker     *CircuitBreaker
	lastRequestTime    int64 // atomic, unix timestamp
	totalRequests      int64 // atomic
	successfulRequests int64 // atomic
	failedRequests     int64 // atomic
	averageLatency     int64 // atomic, in milliseconds
	createdAt          time.Time
	mu                 sync.RWMutex
	recentLatencies    []int64 // sliding window for latency calculation
	maxLatencySamples  int
}

// NewTunnelHealth creates a new tunnel health tracker
func NewTunnelHealth(hostname string) *Health {
	return &Health{
		hostname:          hostname,
		circuitBreaker:    NewCircuitBreaker(5, 30*time.Second), // 5 failures, 30s timeout
		createdAt:         time.Now(),
		maxLatencySamples: 100, // Keep last 100 latency samples
		recentLatencies:   make([]int64, 0, 100),
	}
}

// RecordRequest records a request attempt
func (th *Health) RecordRequest(success bool, latency time.Duration) {
	now := time.Now().Unix()
	atomic.StoreInt64(&th.lastRequestTime, now)
	atomic.AddInt64(&th.totalRequests, 1)

	if success {
		atomic.AddInt64(&th.successfulRequests, 1)
		th.circuitBreaker.OnSuccess()
	} else {
		atomic.AddInt64(&th.failedRequests, 1)
		th.circuitBreaker.OnFailure()
	}

	// Update latency metrics
	latencyMs := latency.Milliseconds()
	th.mu.Lock()
	th.recentLatencies = append(th.recentLatencies, latencyMs)
	if len(th.recentLatencies) > th.maxLatencySamples {
		th.recentLatencies = th.recentLatencies[1:]
	}

	// Calculate average latency
	var sum int64
	for _, l := range th.recentLatencies {
		sum += l
	}
	if len(th.recentLatencies) > 0 {
		atomic.StoreInt64(&th.averageLatency, sum/int64(len(th.recentLatencies)))
	}
	th.mu.Unlock()
}

// CanServeRequest checks if tunnel can serve requests
func (th *Health) CanServeRequest() bool {
	return th.circuitBreaker.CanRequest()
}

// GetHealthStats returns current health statistics
func (th *Health) GetHealthStats() map[string]interface{} {
	return map[string]interface{}{
		"hostname":            th.hostname,
		"circuit_breaker":     th.circuitBreaker.GetState(),
		"total_requests":      atomic.LoadInt64(&th.totalRequests),
		"successful_requests": atomic.LoadInt64(&th.successfulRequests),
		"failed_requests":     atomic.LoadInt64(&th.failedRequests),
		"average_latency_ms":  atomic.LoadInt64(&th.averageLatency),
		"last_request_time":   atomic.LoadInt64(&th.lastRequestTime),
		"uptime_seconds":      time.Since(th.createdAt).Seconds(),
	}
}

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxAttempts   int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		BaseDelay:     100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
	}
}

// CalculateDelay calculates delay for retry attempt with exponential backoff
func (rc *RetryConfig) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return rc.BaseDelay
	}

	delay := float64(rc.BaseDelay) * math.Pow(rc.BackoffFactor, float64(attempt-1))
	if delay > float64(rc.MaxDelay) {
		delay = float64(rc.MaxDelay)
	}

	return time.Duration(delay)
}

// ShouldRetry determines if an error should trigger a retry
func ShouldRetry(err error, statusCode int) bool {
	if err != nil {
		// Network errors should be retried
		if strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "connection reset") {
			return true
		}
	}

	// Retry on 5xx server errors but not 4xx client errors
	return statusCode >= 500 && statusCode < 600
}

// ConnectionManager manages tunnel connections and provides HTTP/2 service
type ConnectionManager struct {
	httpAddr       string
	requestTimeout time.Duration
	kubeClient     ctrlruntimeclient.Client

	registry         *Registry
	responseChannels sync.Map // map[requestID]chan *HTTPResponse
	tunnelHealth     sync.Map // map[hostname]*Health
	tunnelCache      *Cache   // TTL-based cache for tunnel authentication
	cleanupCtx       context.Context
	cleanupCancel    context.CancelFunc

	httpServer *http.Server
}

// ConnectionManagerConfig holds configuration for the connection manager
type ConnectionManagerConfig struct {
	HTTPAddr       string
	RequestTimeout time.Duration
	KubeClient     ctrlruntimeclient.Client
	CacheTTL       time.Duration // TTL for tunnel authentication cache
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(config *ConnectionManagerConfig) (*ConnectionManager, error) {
	// Create registry
	registry := NewRegistry()

	if config.RequestTimeout == 0 {
		config.RequestTimeout = 3 * time.Minute
	}

	// Create tunnel cache with configured TTL
	tunnelCache := NewTunnelCache(config.CacheTTL)

	// Create cleanup context
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())

	cm := &ConnectionManager{
		httpAddr:       config.HTTPAddr,
		requestTimeout: config.RequestTimeout,
		kubeClient:     config.KubeClient,
		registry:       registry,
		tunnelCache:    tunnelCache,
		cleanupCtx:     cleanupCtx,
		cleanupCancel:  cleanupCancel,
	}

	// Start response channel cleanup routine
	go cm.startResponseChannelCleanup()

	return cm, nil
}

// Start starts the connection manager
func (cm *ConnectionManager) Start(ctx context.Context) error {
	log := klog.FromContext(ctx)

	// Start HTTP server for both Envoy and tunnel connections
	if err := cm.startHTTPServer(ctx); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	log.Info("Connection manager started", "httpAddr", cm.httpAddr)

	return nil
}

// startHTTPServer starts the HTTP server for receiving Envoy requests
func (cm *ConnectionManager) startHTTPServer(ctx context.Context) error {
	log := klog.FromContext(ctx)

	// Create HTTP mux
	mux := http.NewServeMux()

	// Handle all requests to forward through tunnels
	mux.HandleFunc("/", cm.handleTunnelRequest)

	// Tunnel connection endpoints
	mux.HandleFunc("/tunnel/connect", cm.handleTunnelConnect)
	mux.HandleFunc("/tunnel/response", cm.handleTunnelResponse)

	// Health check endpoint
	mux.HandleFunc("/health", cm.handleHealth)

	// Health metrics endpoint
	mux.HandleFunc("/metrics", cm.handleMetrics)

	// Create HTTP server
	cm.httpServer = &http.Server{
		Addr:    cm.httpAddr,
		Handler: mux,
		// Longer timeouts to support SSE connections
		ReadTimeout:  10 * time.Minute, // Time to read request headers and body
		WriteTimeout: 0,                // No write timeout for SSE (long-lived connections)
		IdleTimeout:  15 * time.Minute, // Time to keep idle connections open
	}

	// Start serving in goroutine
	go func() {
		log.Info("Starting HTTP server", "addr", cm.httpAddr)
		if err := cm.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(err, "HTTP server stopped")
		}
	}()

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		log.Info("Shutting down HTTP server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := cm.httpServer.Shutdown(shutdownCtx); err != nil {
			log.Error(err, "Error shutting down HTTP server")
		} else {
			log.Info("HTTP server stopped gracefully")
		}
	}()

	return nil
}

// handleTunnelRequest handles incoming HTTP requests and forwards them through tunnels
func (cm *ConnectionManager) handleTunnelRequest(w http.ResponseWriter, r *http.Request) {
	// Extract host from x-tunnel-host header first (from Envoy), fallback to Host header
	host := r.Header.Get("X-Tunnel-Host")
	if host == "" {
		host = r.Host
		if host == "" {
			host = r.Header.Get("Host")
		}
	}
	// Remove port if present
	hostBeforePortRemoval := host
	if colonIndex := strings.Index(host, ":"); colonIndex > 0 {
		host = host[:colonIndex]
	}

	if hostBeforePortRemoval != host {
		klog.Info("Port removed from host", "before", hostBeforePortRemoval, "after", host)
	}

	// Extract tunnel metadata from headers (from Envoy)
	tunnelName := r.Header.Get("X-Tunnel-Name")
	tenantName := r.Header.Get("X-Tenant-Name")

	log := klog.FromContext(r.Context()).WithValues("host", host, "method", r.Method, "path", r.URL.Path, "tunnelName", tunnelName, "tenantName", tenantName)

	activeTunnels := cm.registry.GetAllTunnels()
	// Find tunnel for this host
	conn, exists := cm.registry.GetActiveTunnel(host)
	if !exists {
		log.Info("No tunnel found for host", "host", host, "availableTunnels", activeTunnels)
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	// Get or create health tracker for this tunnel
	var health *Health
	if h, ok := cm.tunnelHealth.Load(host); ok {
		health = h.(*Health)
	} else {
		health = NewTunnelHealth(host)
		cm.tunnelHealth.Store(host, health)
	}

	// Check circuit breaker before proceeding
	if !health.CanServeRequest() {
		log.V(4).Info("Circuit breaker open for tunnel", "host", host, "state", health.circuitBreaker.GetState())
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	// Validate tunnel authentication by checking if the tunnel is actually registered
	// The fact that we found the tunnel in the registry means it's authenticated
	// Additional validation could be added here if needed

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error(err, "Failed to read request body")
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Hop-by-hop headers that should not be forwarded
	hopByHopHeaders := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"Te":                  true,
		"Trailers":            true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}

	// Convert headers to map - preserve all values by joining with comma
	headers := make(map[string]string)
	for key, values := range r.Header {
		// Skip hop-by-hop headers
		if hopByHopHeaders[key] {
			continue
		}
		if len(values) > 0 {
			// Join multiple values with comma (standard HTTP header behavior)
			headers[key] = strings.Join(values, ", ")
		}
	}

	// Ensure X-Forwarded headers are included (they should already be in headers map)
	// But explicitly set them to ensure they're not lost
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		headers["X-Forwarded-For"] = xForwardedFor
	}
	if xForwardedProto := r.Header.Get("X-Forwarded-Proto"); xForwardedProto != "" {
		headers["X-Forwarded-Proto"] = xForwardedProto
	}

	// Ensure critical headers for proper content handling
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		headers["Content-Type"] = contentType
	}
	if contentLength := r.Header.Get("Content-Length"); contentLength != "" {
		headers["Content-Length"] = contentLength
	}

	// Preserve caching headers for better performance
	cachingHeaders := []string{
		"Cache-Control",
		"ETag",
		"Last-Modified",
		"If-None-Match",
		"If-Modified-Since",
		"Expires",
		"Vary",
		"Age",
	}
	for _, h := range cachingHeaders {
		if value := r.Header.Get(h); value != "" {
			headers[h] = value
		}
	}

	// Generate request ID
	requestID := uuid.New().String()

	// Create HTTP request to forward
	httpReq := &HTTPRequest{
		RequestID: requestID,
		Method:    r.Method,
		Path:      r.URL.RequestURI(),
		Headers:   headers,
		Body:      base64.StdEncoding.EncodeToString(body),
	}

	// Create response channel
	respChan := make(chan *HTTPResponse, 1)
	cm.responseChannels.Store(requestID, respChan)
	defer cm.responseChannels.Delete(requestID)

	// Send request to tunnel with retry logic
	startTime := time.Now()
	err = cm.sendRequestWithRetry(conn, httpReq, log)

	if err != nil {
		// Record failure and return error
		health.RecordRequest(false, time.Since(startTime))
		log.Error(err, "Failed to send request to tunnel after retries")
		http.Error(w, "Failed to forward request", http.StatusBadGateway)
		return
	}

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		// Record successful request
		totalLatency := time.Since(startTime)
		health.RecordRequest(true, totalLatency)

		// Decode response body
		respBody, err := base64.StdEncoding.DecodeString(resp.Body)
		if err != nil {
			log.Error(err, "Failed to decode response body")
			http.Error(w, "Invalid response body", http.StatusInternalServerError)
			return
		}

		// Set response headers
		for key, value := range resp.Headers {
			w.Header().Set(key, value)
		}

		// Set status code
		w.WriteHeader(resp.StatusCode)

		// Write response body
		if len(respBody) > 0 {
			if _, err := w.Write(respBody); err != nil {
				log.Error(err, "Failed to write response body")
			}
		}

	case <-time.After(cm.requestTimeout):
		// Record timeout as failure
		health.RecordRequest(false, time.Since(startTime))
		log.Error(nil, "Request timeout", "requestID", requestID)
		http.Error(w, "Request timeout", http.StatusGatewayTimeout)

	case <-r.Context().Done():
		// Don't record client cancellations as failures
		log.V(4).Info("Request canceled by client", "requestID", requestID)
		// Don't send error response for canceled requests

	case <-conn.Context.Done():
		// Record tunnel disconnection as failure
		health.RecordRequest(false, time.Since(startTime))
		log.Error(nil, "Tunnel disconnected during request", "requestID", requestID)
		http.Error(w, "Tunnel disconnected", http.StatusBadGateway)
	}
}

// handleHealth handles health check requests
func (cm *ConnectionManager) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	activeTunnels := cm.registry.GetAllTunnels()
	healthyTunnels := 0

	// Count healthy tunnels (circuit breaker not open)
	for _, hostname := range activeTunnels {
		if h, ok := cm.tunnelHealth.Load(hostname); ok {
			health := h.(*Health)
			if health.CanServeRequest() {
				healthyTunnels++
			}
		} else {
			healthyTunnels++ // New tunnels are considered healthy
		}
	}

	response := fmt.Sprintf(`{"status":"healthy","tunnels":{"total":%d,"healthy":%d,"unhealthy":%d}}`,
		len(activeTunnels), healthyTunnels, len(activeTunnels)-healthyTunnels)

	if _, err := w.Write([]byte(response)); err != nil {
		// Log error but don't fail the health check
		klog.V(2).Info("Failed to write health check response", "error", err)
	}
}

// handleMetrics handles metrics requests
func (cm *ConnectionManager) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	metrics := make(map[string]interface{})
	metrics["timestamp"] = time.Now().Unix()
	metrics["tunnels"] = make(map[string]interface{})

	// Collect metrics for all active tunnels
	activeTunnels := cm.registry.GetAllTunnels()
	for _, hostname := range activeTunnels {
		if h, ok := cm.tunnelHealth.Load(hostname); ok {
			health := h.(*Health)
			metrics["tunnels"].(map[string]interface{})[hostname] = health.GetHealthStats()
		}
	}

	// Add summary metrics
	summary := make(map[string]interface{})
	var totalRequests, totalSuccessful, totalFailed int64
	var totalLatency int64
	activeHealthTrackers := 0

	cm.tunnelHealth.Range(func(_, value interface{}) bool {
		health := value.(*Health)
		totalRequests += atomic.LoadInt64(&health.totalRequests)
		totalSuccessful += atomic.LoadInt64(&health.successfulRequests)
		totalFailed += atomic.LoadInt64(&health.failedRequests)
		totalLatency += atomic.LoadInt64(&health.averageLatency)
		activeHealthTrackers++
		return true
	})

	summary["total_requests"] = totalRequests
	summary["successful_requests"] = totalSuccessful
	summary["failed_requests"] = totalFailed
	if activeHealthTrackers > 0 {
		summary["average_latency_ms"] = totalLatency / int64(activeHealthTrackers)
	} else {
		summary["average_latency_ms"] = 0
	}
	summary["active_tunnels"] = len(activeTunnels)
	summary["tracked_tunnels"] = activeHealthTrackers

	metrics["summary"] = summary

	// Add cache statistics
	metrics["cache"] = cm.tunnelCache.GetStats()

	if data, err := json.Marshal(metrics); err != nil {
		http.Error(w, "Failed to marshal metrics", http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(data); err != nil {
			klog.V(2).Info("Failed to write metrics response", "error", err)
		}
	}
}

// handleTunnelConnect handles SSE connections from tunnel clients
func (cm *ConnectionManager) handleTunnelConnect(w http.ResponseWriter, r *http.Request) {
	log := klog.FromContext(r.Context())

	// Check if this is an SSE request
	if r.Header.Get("Accept") != "text/event-stream" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Extract authentication headers
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	hostname := r.Header.Get("X-Tunnel-Hostname")
	targetPort := r.Header.Get("X-Target-Port")
	tunnelName := r.Header.Get("X-Tunnel-Name")
	tenantName := r.Header.Get("X-Tenant-Name")

	log.Info("Tunnel connection attempt", "hostname", hostname, "targetPort", targetPort, "tunnelName", tunnelName, "tenantName", tenantName)

	// Validate required fields
	if token == "" || hostname == "" || targetPort == "" || tunnelName == "" || tenantName == "" {
		http.Error(w, "Missing required headers", http.StatusBadRequest)
		return
	}

	// Validate token
	if err := cm.validateTunnelToken(r.Context(), hostname, token, tunnelName, tenantName); err != nil {
		log.Error(err, "Token validation failed", "hostname", hostname)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Create flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Create request channel for this tunnel
	requestChan := make(chan *HTTPRequest, 100) // Buffer for high-traffic scenarios

	// Register tunnel with channel-based communication
	if err := cm.registry.RegisterTunnel(r.Context(), hostname, token, targetPort, requestChan); err != nil {
		log.Error(err, "Failed to register tunnel", "hostname", hostname)
		http.Error(w, "Failed to register tunnel", http.StatusInternalServerError)
		return
	}

	// Ensure cleanup on exit
	defer func() {
		cm.registry.UnregisterTunnel(hostname)
		close(requestChan)
		// Invalidate cache entry when tunnel disconnects
		cm.tunnelCache.Delete(tenantName, tunnelName)
		log.Info("Tunnel connection closed", "hostname", hostname, "cacheInvalidated", true)
	}()

	log.Info("Tunnel registered successfully", "hostname", hostname, "targetPort", targetPort)

	// Send initial ping to confirm connection
	if err := cm.sendSSEEvent(w, flusher, "ping", "initial"); err != nil {
		log.Error(err, "Failed to send initial ping")
		return
	}

	// Start the SSE event loop
	cm.handleSSEEventLoop(r.Context(), w, flusher, requestChan, log)
}

// handleSSEEventLoop manages the SSE connection lifecycle
func (cm *ConnectionManager) handleSSEEventLoop(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, requestChan <-chan *HTTPRequest, log logr.Logger) {
	ticker := time.NewTicker(60 * time.Second) // Keep-alive ping every 60 seconds
	defer ticker.Stop()

	var pingCount int64
	var lastPingTime time.Time
	var pingFailureCount int
	const maxPingFailures = 5
	connectionStart := time.Now()

	// Log connection start
	log.Info("SSE event loop started", "connectionStart", connectionStart.Format(time.RFC3339))

	for {
		select {
		case <-ctx.Done():
			connectionDuration := time.Since(connectionStart)
			log.Info("SSE connection closed by client",
				"connectionDuration", connectionDuration.String(),
				"totalPings", pingCount,
				"pendingFailures", pingFailureCount,
				"avgPingInterval", time.Duration(connectionDuration.Nanoseconds()/max(pingCount, 1)).String())
			return

		case req := <-requestChan:
			if req == nil {
				connectionDuration := time.Since(connectionStart)
				log.Info("Request channel closed",
					"connectionDuration", connectionDuration.String(),
					"totalPings", pingCount,
					"pendingFailures", pingFailureCount)
				return
			}
			// Send request as SSE event
			data, err := json.Marshal(req)
			if err != nil {
				log.Error(err, "Failed to marshal request")
				continue
			}
			if err := cm.sendSSEEvent(w, flusher, "request", string(data)); err != nil {
				log.Error(err, "Failed to send SSE event")
				return
			}

		case <-ticker.C:
			// Send keepalive ping with backoff mechanism
			lastPingTime = time.Now()
			pingData := fmt.Sprintf("keepalive-%d", pingCount+1)

			if err := cm.sendSSEEvent(w, flusher, "ping", pingData); err != nil {
				pingFailureCount++
				log.V(1).Info("Keepalive ping failed",
					"failure", pingFailureCount,
					"maxFailures", maxPingFailures,
					"error", err,
					"pingCount", pingCount)

				if pingFailureCount >= maxPingFailures {
					log.Error(err, "Max ping failures reached, closing connection",
						"failures", pingFailureCount,
						"connectionDuration", time.Since(connectionStart).String())
					return
				}
				// Continue without incrementing pingCount on failure
				continue
			}

			// Reset failure counter and increment ping count on success
			if pingFailureCount > 0 {
				log.Info("Ping recovered after failures",
					"previousFailures", pingFailureCount,
					"pingCount", pingCount)
				pingFailureCount = 0
			}

			pingCount++
			log.V(4).Info("Keepalive ping sent", "pingCount", pingCount, "timestamp", lastPingTime.Format(time.RFC3339))
		}
	}
}

// handleTunnelResponse handles responses from tunnel clients
func (cm *ConnectionManager) handleTunnelResponse(w http.ResponseWriter, r *http.Request) {
	log := klog.FromContext(r.Context())

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read response body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error(err, "Failed to read response body")
		http.Error(w, "Failed to read response", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse response
	var resp HTTPResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		log.Error(err, "Failed to parse response")
		http.Error(w, "Invalid response format", http.StatusBadRequest)
		return
	}

	// Forward response to waiting request
	if ch, ok := cm.responseChannels.Load(resp.RequestID); ok {
		respChan := ch.(chan *HTTPResponse)
		select {
		case respChan <- &resp:
			// Response sent successfully
			w.WriteHeader(http.StatusOK)
		default:
			// Channel full or closed
			log.Error(nil, "Failed to forward response", "requestID", resp.RequestID)
			http.Error(w, "Failed to forward response", http.StatusInternalServerError)
		}
	} else {
		log.V(4).Info("No waiting request for response", "requestID", resp.RequestID)
		http.Error(w, "No matching request", http.StatusNotFound)
	}
}

// sendRequestToTunnel sends a request to the tunnel via channel
func (cm *ConnectionManager) sendRequestToTunnel(conn *Connection, req *HTTPRequest) error {
	select {
	case conn.RequestChan <- req:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending request to tunnel")
	case <-conn.Context.Done():
		return fmt.Errorf("tunnel connection closed")
	}
}

// sendRequestWithRetry sends a request with exponential backoff retry logic
func (cm *ConnectionManager) sendRequestWithRetry(conn *Connection, req *HTTPRequest, log logr.Logger) error {
	retryConfig := DefaultRetryConfig()

	for attempt := 1; attempt <= retryConfig.MaxAttempts; attempt++ {
		err := cm.sendRequestToTunnel(conn, req)

		if err == nil {
			if attempt > 1 {
				log.V(4).Info("Request succeeded after retry", "attempt", attempt, "requestID", req.RequestID)
			}
			return nil
		}

		// Check if we should retry this error
		if !ShouldRetry(err, 0) {
			log.V(4).Info("Error not retryable", "error", err, "requestID", req.RequestID)
			return err
		}

		// Don't retry on the last attempt
		if attempt == retryConfig.MaxAttempts {
			log.Error(err, "All retry attempts exhausted", "attempts", attempt, "requestID", req.RequestID)
			return fmt.Errorf("request failed after %d attempts: %w", attempt, err)
		}

		// Calculate delay for next attempt
		delay := retryConfig.CalculateDelay(attempt)
		log.V(4).Info("Request failed, retrying", "attempt", attempt, "nextDelay", delay, "error", err, "requestID", req.RequestID)

		// Wait before retrying
		select {
		case <-time.After(delay):
			// Continue to next retry
		case <-conn.Context.Done():
			return fmt.Errorf("tunnel connection closed during retry")
		}
	}

	return fmt.Errorf("unexpected end of retry loop")
}

// sendSSEEvent sends an SSE event
func (cm *ConnectionManager) sendSSEEvent(w io.Writer, flusher http.Flusher, event, data string) error {
	_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	if err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// validateTunnelToken validates the tunnel token using TTL cache with fallback to Kubernetes API
func (cm *ConnectionManager) validateTunnelToken(ctx context.Context, hostname, token, tunnelName, tenantName string) error {
	log := klog.FromContext(ctx)

	// Try cache first
	if cached, found := cm.tunnelCache.Get(tenantName, tunnelName); found {
		// Validate cached data matches request
		if cached.Hostname == hostname && cached.Token == token {
			log.V(4).Info("Token validation successful (cached)", "hostname", hostname, "tunnelName", tunnelName, "tenantName", tenantName, "cacheHit", true)
			return nil
		}
		// Token or hostname mismatch, remove from cache and fallback to K8s API
		log.V(4).Info("Cache entry invalid, evicting", "hostname", hostname, "tunnelName", tunnelName, "tenantName", tenantName,
			"cachedHostname", cached.Hostname, "tokenMatch", cached.Token == token)
		cm.tunnelCache.Delete(tenantName, tunnelName)
	}

	// Cache miss or invalid - fetch from Kubernetes API
	log.V(4).Info("Cache miss, validating via Kubernetes API", "hostname", hostname, "tunnelName", tunnelName, "tenantName", tenantName)

	// Get tunnel resource directly by name and tenant for security
	var tunnel kubelbv1alpha1.Tunnel
	tunnelCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := cm.kubeClient.Get(tunnelCtx, ctrlruntimeclient.ObjectKey{
		Namespace: tenantName, // Only look in the specified tenant namespace
		Name:      tunnelName,
	}, &tunnel); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("tunnel %s not found in tenant %s", tunnelName, tenantName)
		}
		return fmt.Errorf("failed to get tunnel %s/%s: %w", tenantName, tunnelName, err)
	}

	// Verify hostname matches
	if tunnel.Status.Hostname != hostname {
		return fmt.Errorf("hostname mismatch: tunnel %s/%s has hostname %s, expected %s",
			tenantName, tunnelName, tunnel.Status.Hostname, hostname)
	}

	// Get SyncSecret directly within the same tenant namespace
	syncSecretName := fmt.Sprintf("tunnel-auth-%s", tunnel.Name)
	var syncSecret kubelbv1alpha1.SyncSecret

	secretCtx, secretCancel := context.WithTimeout(ctx, 30*time.Second)
	defer secretCancel()

	if err := cm.kubeClient.Get(secretCtx, ctrlruntimeclient.ObjectKey{
		Namespace: tenantName, // Ensure secret is also in the correct tenant namespace
		Name:      syncSecretName,
	}, &syncSecret); err != nil {
		return fmt.Errorf("failed to get SyncSecret %s/%s: %w", tenantName, syncSecretName, err)
	}

	// Validate token
	expectedToken, exists := syncSecret.Data["token"]
	if !exists || len(expectedToken) == 0 {
		return fmt.Errorf("tunnel %s/%s has no token configured in SyncSecret", tenantName, tunnelName)
	}
	if string(expectedToken) != token {
		// Invalid token - ensure it's not cached
		cm.tunnelCache.Delete(tenantName, tunnelName)
		return fmt.Errorf("invalid token for tunnel %s/%s", tenantName, tunnelName)
	}

	// Parse token expiry for cache TTL validation
	var tokenExpiry time.Time
	if expiryData, exists := syncSecret.Data["tokenExpiry"]; exists && len(expiryData) > 0 {
		if parsedExpiry, err := time.Parse(time.RFC3339, string(expiryData)); err == nil {
			tokenExpiry = parsedExpiry
		}
	}
	// If no expiry or parse error, set a default expiry (cache TTL will handle it)
	if tokenExpiry.IsZero() {
		tokenExpiry = time.Now().Add(24 * time.Hour) // Default 24 hour expiry
	}

	// Store successful validation in cache
	cm.tunnelCache.Set(tenantName, tunnelName, token, hostname, tokenExpiry)

	log.V(4).Info("Token validation successful (Kubernetes API)", "hostname", hostname, "tunnelName", tunnelName, "tenantName", tenantName, "cacheHit", false)
	return nil
}

// startResponseChannelCleanup runs a background cleanup routine for abandoned response channels, stale health trackers, and expired cache entries
func (cm *ConnectionManager) startResponseChannelCleanup() {
	ticker := time.NewTicker(1 * time.Minute) // Cleanup every minute
	defer ticker.Stop()

	for {
		select {
		case <-cm.cleanupCtx.Done():
			return
		case <-ticker.C:
			// Cleanup stale health trackers for tunnels that no longer exist
			cm.cleanupStaleHealthTrackers()

			// Cleanup expired cache entries
			evicted := cm.tunnelCache.EvictExpired()
			if evicted > 0 {
				klog.V(2).Info("Evicted expired cache entries", "count", evicted)
			}

			// Note: Response channels are automatically cleaned up by handleTunnelRequest
			// with defer statements, so no manual cleanup needed for them
		}
	}
}

// cleanupStaleHealthTrackers removes health trackers for tunnels that no longer exist
func (cm *ConnectionManager) cleanupStaleHealthTrackers() {
	activeTunnels := make(map[string]bool)
	for _, hostname := range cm.registry.GetAllTunnels() {
		activeTunnels[hostname] = true
	}

	var staleTrackers []string
	cm.tunnelHealth.Range(func(key, value interface{}) bool {
		hostname := key.(string)
		health := value.(*Health)

		// Remove trackers for inactive tunnels that haven't had requests in 10 minutes
		if !activeTunnels[hostname] {
			lastRequest := atomic.LoadInt64(&health.lastRequestTime)
			if lastRequest > 0 && time.Now().Unix()-lastRequest > 600 { // 10 minutes
				staleTrackers = append(staleTrackers, hostname)
			}
		}
		return true
	})

	// Remove stale trackers
	for _, hostname := range staleTrackers {
		cm.tunnelHealth.Delete(hostname)
		klog.V(4).Info("Cleaned up stale health tracker", "hostname", hostname)
	}

	if len(staleTrackers) > 0 {
		klog.V(2).Info("Cleaned up stale health trackers", "count", len(staleTrackers))
	}
}

// Stop stops the connection manager
func (cm *ConnectionManager) Stop() {
	// Stop cleanup routine
	if cm.cleanupCancel != nil {
		cm.cleanupCancel()
	}

	if cm.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := cm.httpServer.Shutdown(ctx); err != nil {
			klog.V(2).Info("Failed to shutdown HTTP server gracefully", "error", err)
		}
	}
}

// GetActiveTunnels returns the list of active tunnel hostnames
func (cm *ConnectionManager) GetActiveTunnels() []string {
	return cm.registry.GetAllTunnels()
}

// GetRegistry returns the tunnel registry for testing
func (cm *ConnectionManager) GetRegistry() *Registry {
	return cm.registry
}
