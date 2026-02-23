/*
Copyright 2024 The KubeLB Authors.

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

// Package envoycp provides Prometheus metrics for the KubeLB Envoy Control Plane component.
package envoycp

import (
	"github.com/prometheus/client_golang/prometheus"

	"k8c.io/kubelb/internal/metricsutil"
)

// factory creates metrics and captures their descriptions for documentation.
var factory = metricsutil.NewFactory(metricsutil.SubsystemEnvoyCP, metricsutil.ComponentEnvoyCP)

// Snapshot metrics - track Envoy xDS snapshot state.
var (
	// SnapshotsTotal tracks the total number of active snapshots.
	SnapshotsTotal = factory.NewGauge(
		"snapshots",
		"Current number of active Envoy snapshots",
	)

	// SnapshotUpdatesTotal counts the total number of snapshot updates.
	SnapshotUpdatesTotal = factory.NewCounterVec(
		"snapshot_updates_total",
		"Total number of Envoy snapshot updates",
		[]string{"snapshot_name"},
	)

	// SnapshotGenerationDuration tracks the duration of snapshot generation.
	SnapshotGenerationDuration = factory.NewHistogramVec(
		"snapshot_generation_duration_seconds",
		"Duration of Envoy snapshot generation in seconds",
		[]string{"snapshot_name"},
		nil,
	)
)

// xDS resource metrics - track the count of resources in snapshots.
var (
	// ClustersTotal tracks the number of clusters across all snapshots.
	ClustersTotal = factory.NewGaugeVec(
		"clusters",
		"Current number of clusters in the Envoy snapshot",
		[]string{"snapshot_name"},
	)

	// ListenersTotal tracks the number of listeners across all snapshots.
	ListenersTotal = factory.NewGaugeVec(
		"listeners",
		"Current number of listeners in the Envoy snapshot",
		[]string{"snapshot_name"},
	)

	// EndpointsTotal tracks the number of endpoints across all snapshots.
	EndpointsTotal = factory.NewGaugeVec(
		"endpoints",
		"Current number of endpoints in the Envoy snapshot",
		[]string{"snapshot_name"},
	)

	// RoutesTotal tracks the number of routes across all snapshots.
	RoutesTotal = factory.NewGaugeVec(
		"routes",
		"Current number of routes in the Envoy snapshot",
		[]string{"snapshot_name"},
	)

	// SecretsTotal tracks the number of secrets across all snapshots.
	SecretsTotal = factory.NewGaugeVec(
		"secrets",
		"Current number of secrets in the Envoy snapshot",
		[]string{"snapshot_name"},
	)
)

// gRPC server metrics - track xDS gRPC server activity.
var (
	// GRPCConnectionsTotal tracks the total number of gRPC connections.
	GRPCConnectionsTotal = factory.NewGauge(
		"grpc_connections",
		"Current number of active gRPC connections to the xDS server",
	)

	// GRPCRequestsTotal counts the total number of gRPC requests.
	GRPCRequestsTotal = factory.NewCounterVec(
		"grpc_requests_total",
		"Total number of gRPC xDS requests",
		[]string{"type_url"},
	)

	// GRPCResponsesTotal counts the total number of gRPC responses.
	GRPCResponsesTotal = factory.NewCounterVec(
		"grpc_responses_total",
		"Total number of gRPC xDS responses",
		[]string{"type_url"},
	)
)

// Cache metrics - track cache state and operations.
var (
	// CacheHitsTotal counts cache hits for snapshot lookups.
	CacheHitsTotal = factory.NewCounterVec(
		"cache_hits_total",
		"Total number of cache hits for snapshot lookups",
		[]string{"snapshot_name"},
	)

	// CacheMissesTotal counts cache misses for snapshot lookups.
	CacheMissesTotal = factory.NewCounterVec(
		"cache_misses_total",
		"Total number of cache misses for snapshot lookups",
		[]string{"snapshot_name"},
	)

	// CacheClearsTotal counts the total number of cache clears.
	CacheClearsTotal = factory.NewCounterVec(
		"cache_clears_total",
		"Total number of cache clears",
		[]string{"snapshot_name"},
	)
)

// Envoy proxy deployment metrics.
var (
	// EnvoyProxiesTotal tracks the total number of Envoy proxy deployments.
	EnvoyProxiesTotal = factory.NewGaugeVec(
		"envoy_proxies",
		"Current number of Envoy proxy deployments",
		[]string{metricsutil.LabelNamespace, metricsutil.LabelTopology},
	)

	// EnvoyProxyCreateTotal counts the total number of Envoy proxy creations.
	EnvoyProxyCreateTotal = factory.NewCounterVec(
		"envoy_proxy_create_total",
		"Total number of Envoy proxy deployments created",
		[]string{metricsutil.LabelNamespace},
	)

	// EnvoyProxyDeleteTotal counts the total number of Envoy proxy deletions.
	EnvoyProxyDeleteTotal = factory.NewCounterVec(
		"envoy_proxy_delete_total",
		"Total number of Envoy proxy deployments deleted",
		[]string{metricsutil.LabelNamespace},
	)
)

// allCollectors returns all metrics collectors for registration.
func allCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		// Snapshot metrics
		SnapshotsTotal,
		SnapshotUpdatesTotal,
		SnapshotGenerationDuration,
		// xDS resource metrics
		ClustersTotal,
		ListenersTotal,
		EndpointsTotal,
		RoutesTotal,
		SecretsTotal,
		// gRPC server metrics
		GRPCConnectionsTotal,
		GRPCRequestsTotal,
		GRPCResponsesTotal,
		// Cache metrics
		CacheHitsTotal,
		CacheMissesTotal,
		CacheClearsTotal,
		// Envoy proxy metrics
		EnvoyProxiesTotal,
		EnvoyProxyCreateTotal,
		EnvoyProxyDeleteTotal,
	}
}

// Register registers all EnvoyCP metrics with the controller-runtime metrics registry.
func Register() {
	metricsutil.MustRegister(allCollectors()...)
}

// ListMetrics returns descriptions of all EnvoyCP metrics for documentation generation.
func ListMetrics() []metricsutil.MetricDescription {
	return factory.Descriptions()
}
