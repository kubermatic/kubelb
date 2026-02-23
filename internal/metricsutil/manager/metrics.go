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

// Package manager provides Prometheus metrics for the KubeLB Manager component.
package manager

import (
	"github.com/prometheus/client_golang/prometheus"

	"k8c.io/kubelb/internal/metricsutil"
)

// factory creates metrics and captures their descriptions for documentation.
var factory = metricsutil.NewFactory(metricsutil.SubsystemManager, metricsutil.ComponentManager)

// Reconciliation counters - track the number and outcome of reconciliation attempts.
var (
	// LoadBalancerReconcileTotal counts the total number of LoadBalancer reconciliation attempts.
	LoadBalancerReconcileTotal = factory.NewCounterVec(
		"loadbalancer_reconcile_total",
		"Total number of LoadBalancer reconciliation attempts",
		[]string{metricsutil.LabelNamespace, metricsutil.LabelResult},
	)

	// RouteReconcileTotal counts the total number of Route reconciliation attempts.
	RouteReconcileTotal = factory.NewCounterVec(
		"route_reconcile_total",
		"Total number of Route reconciliation attempts",
		[]string{metricsutil.LabelNamespace, metricsutil.LabelRouteType, metricsutil.LabelResult},
	)

	// TenantReconcileTotal counts the total number of Tenant reconciliation attempts.
	TenantReconcileTotal = factory.NewCounterVec(
		"tenant_reconcile_total",
		"Total number of Tenant reconciliation attempts",
		[]string{metricsutil.LabelResult},
	)

	// EnvoyCPReconcileTotal counts the total number of EnvoyCP reconciliation attempts.
	EnvoyCPReconcileTotal = factory.NewCounterVec(
		"envoycp_reconcile_total",
		"Total number of Envoy control plane reconciliation attempts",
		[]string{metricsutil.LabelResult},
	)

	// SyncSecretReconcileTotal counts the total number of SyncSecret reconciliation attempts.
	SyncSecretReconcileTotal = factory.NewCounterVec(
		"sync_secret_reconcile_total",
		"Total number of SyncSecret reconciliation attempts",
		[]string{metricsutil.LabelNamespace, metricsutil.LabelResult},
	)
)

// Reconciliation duration histograms - track how long reconciliations take.
var (
	// LoadBalancerReconcileDuration tracks the duration of LoadBalancer reconciliations.
	LoadBalancerReconcileDuration = factory.NewHistogramVec(
		"loadbalancer_reconcile_duration_seconds",
		"Duration of LoadBalancer reconciliations in seconds",
		[]string{metricsutil.LabelNamespace},
		nil,
	)

	// RouteReconcileDuration tracks the duration of Route reconciliations.
	RouteReconcileDuration = factory.NewHistogramVec(
		"route_reconcile_duration_seconds",
		"Duration of Route reconciliations in seconds",
		[]string{metricsutil.LabelNamespace},
		nil,
	)

	// TenantReconcileDuration tracks the duration of Tenant reconciliations.
	TenantReconcileDuration = factory.NewHistogramVec(
		"tenant_reconcile_duration_seconds",
		"Duration of Tenant reconciliations in seconds",
		[]string{},
		nil,
	)

	// EnvoyCPReconcileDuration tracks the duration of EnvoyCP reconciliations.
	EnvoyCPReconcileDuration = factory.NewHistogramVec(
		"envoycp_reconcile_duration_seconds",
		"Duration of Envoy control plane reconciliations in seconds",
		[]string{},
		nil,
	)

	// SyncSecretReconcileDuration tracks the duration of SyncSecret reconciliations.
	SyncSecretReconcileDuration = factory.NewHistogramVec(
		"sync_secret_reconcile_duration_seconds",
		"Duration of SyncSecret reconciliations in seconds",
		[]string{metricsutil.LabelNamespace},
		nil,
	)
)

// Resource gauges - track the current count of resources.
var (
	// LoadBalancersTotal tracks the total number of LoadBalancer resources.
	LoadBalancersTotal = factory.NewGaugeVec(
		"loadbalancers",
		"Current number of LoadBalancer resources",
		[]string{metricsutil.LabelNamespace, metricsutil.LabelTenant, metricsutil.LabelTopology},
	)

	// RoutesTotal tracks the total number of Route resources.
	RoutesTotal = factory.NewGaugeVec(
		"routes",
		"Current number of Route resources",
		[]string{metricsutil.LabelNamespace, metricsutil.LabelTenant, metricsutil.LabelRouteType},
	)

	// TenantsTotal tracks the total number of Tenant resources.
	TenantsTotal = factory.NewGauge(
		"tenants",
		"Current number of Tenant resources",
	)

	// EnvoyProxiesTotal tracks the total number of Envoy proxy deployments.
	EnvoyProxiesTotal = factory.NewGaugeVec(
		"envoy_proxies",
		"Current number of Envoy proxy deployments",
		[]string{metricsutil.LabelNamespace, metricsutil.LabelTopology},
	)
)

// Port allocator metrics.
var (
	// PortAllocatorAllocatedPorts tracks the total number of allocated ports.
	PortAllocatorAllocatedPorts = factory.NewGauge(
		"port_allocator_allocated_ports",
		"Current number of allocated ports in the port allocator",
	)

	// PortAllocatorEndpoints tracks the total number of endpoints being tracked.
	PortAllocatorEndpoints = factory.NewGauge(
		"port_allocator_endpoints",
		"Current number of endpoints tracked by the port allocator",
	)
)

// EnvoyCP snapshot metrics.
var (
	// EnvoyCPSnapshotUpdatesTotal counts the total number of snapshot updates.
	EnvoyCPSnapshotUpdatesTotal = factory.NewCounterVec(
		"envoycp_snapshot_updates_total",
		"Total number of Envoy snapshot updates",
		[]string{"snapshot_name"},
	)

	// EnvoyCPClusters tracks the number of clusters in the Envoy snapshot.
	EnvoyCPClusters = factory.NewGaugeVec(
		"envoycp_clusters",
		"Current number of clusters in the Envoy snapshot",
		[]string{"snapshot_name"},
	)

	// EnvoyCPListeners tracks the number of listeners in the Envoy snapshot.
	EnvoyCPListeners = factory.NewGaugeVec(
		"envoycp_listeners",
		"Current number of listeners in the Envoy snapshot",
		[]string{"snapshot_name"},
	)

	// EnvoyCPEndpoints tracks the number of endpoints in the Envoy snapshot.
	EnvoyCPEndpoints = factory.NewGaugeVec(
		"envoycp_endpoints",
		"Current number of endpoints in the Envoy snapshot",
		[]string{"snapshot_name"},
	)
)

// allCollectors returns all metrics collectors for registration.
func allCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		// Counters
		LoadBalancerReconcileTotal,
		RouteReconcileTotal,
		TenantReconcileTotal,
		EnvoyCPReconcileTotal,
		SyncSecretReconcileTotal,
		// Histograms
		LoadBalancerReconcileDuration,
		RouteReconcileDuration,
		TenantReconcileDuration,
		EnvoyCPReconcileDuration,
		SyncSecretReconcileDuration,
		// Resource gauges
		LoadBalancersTotal,
		RoutesTotal,
		TenantsTotal,
		EnvoyProxiesTotal,
		// Port allocator
		PortAllocatorAllocatedPorts,
		PortAllocatorEndpoints,
		// EnvoyCP snapshot
		EnvoyCPSnapshotUpdatesTotal,
		EnvoyCPClusters,
		EnvoyCPListeners,
		EnvoyCPEndpoints,
	}
}

// Register registers all Manager metrics with the controller-runtime metrics registry.
func Register() {
	metricsutil.MustRegister(allCollectors()...)
}

// ListMetrics returns descriptions of all Manager metrics for documentation generation.
func ListMetrics() []metricsutil.MetricDescription {
	return factory.Descriptions()
}
