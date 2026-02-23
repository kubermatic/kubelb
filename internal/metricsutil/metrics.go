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

// Package metrics provides Prometheus metrics for KubeLB components.
package metricsutil

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Subsystem names for different KubeLB components.
	SubsystemManager = "kubelb_manager"
	SubsystemCCM     = "kubelb_ccm"
	SubsystemEnvoyCP = "kubelb_envoy_control_plane"
)

// Common label names used across metrics.
const (
	LabelNamespace = "namespace"
	LabelTenant    = "tenant"
	LabelResult    = "result"
	LabelRouteType = "route_type"
	LabelTopology  = "topology"
	LabelOperation = "operation"
)

// Result label values for reconciliation outcomes.
const (
	ResultSuccess = "success"
	ResultError   = "error"
	ResultSkipped = "skipped"
)

// Route type label values.
const (
	RouteTypeIngress   = "ingress"
	RouteTypeGateway   = "gateway"
	RouteTypeHTTPRoute = "httproute"
	RouteTypeGRPCRoute = "grpcroute"
)

// Topology label values for Envoy proxy deployment modes.
const (
	TopologyShared    = "shared"
	TopologyDedicated = "dedicated"
	TopologyGlobal    = "global"
)

// DefaultBuckets defines histogram buckets for reconciliation latency.
// Ranges from 1ms to 60s to capture both fast and slow reconciliations.
var DefaultBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}

// NewCounterVec creates a new CounterVec with the KubeLB namespace.
func NewCounterVec(subsystem, name, help string, labels []string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	}, labels)
}

// NewGaugeVec creates a new GaugeVec with the KubeLB namespace.
func NewGaugeVec(subsystem, name, help string, labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	}, labels)
}

// NewGauge creates a new Gauge with the KubeLB namespace.
func NewGauge(subsystem, name, help string) prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	})
}

// NewHistogramVec creates a new HistogramVec with the KubeLB namespace.
// If buckets is nil, DefaultBuckets will be used.
func NewHistogramVec(subsystem, name, help string, labels []string, buckets []float64) *prometheus.HistogramVec {
	if buckets == nil {
		buckets = DefaultBuckets
	}
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
		Buckets:   buckets,
	}, labels)
}
