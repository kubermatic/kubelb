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

// Package manager provides Prometheus metrics for the KubeLB Manager component.
package manager

import (
	"github.com/prometheus/client_golang/prometheus"

	"k8c.io/kubelb/internal/metrics"
)

// Reconciliation counters - track the number and outcome of reconciliation attempts.
var (
	// LoadBalancerReconcileTotal counts the total number of LoadBalancer reconciliation attempts.
	LoadBalancerReconcileTotal = metrics.NewCounterVec(
		metrics.SubsystemManager,
		"loadbalancer_reconcile_total",
		"Total number of LoadBalancer reconciliation attempts",
		[]string{metrics.LabelNamespace, metrics.LabelResult},
	)

	// RouteReconcileTotal counts the total number of Route reconciliation attempts.
	RouteReconcileTotal = metrics.NewCounterVec(
		metrics.SubsystemManager,
		"route_reconcile_total",
		"Total number of Route reconciliation attempts",
		[]string{metrics.LabelNamespace, metrics.LabelRouteType, metrics.LabelResult},
	)

	// TenantReconcileTotal counts the total number of Tenant reconciliation attempts.
	TenantReconcileTotal = metrics.NewCounterVec(
		metrics.SubsystemManager,
		"tenant_reconcile_total",
		"Total number of Tenant reconciliation attempts",
		[]string{metrics.LabelResult},
	)

	// EnvoyCPReconcileTotal counts the total number of EnvoyCP reconciliation attempts.
	EnvoyCPReconcileTotal = metrics.NewCounterVec(
		metrics.SubsystemManager,
		"envoycp_reconcile_total",
		"Total number of Envoy control plane reconciliation attempts",
		[]string{metrics.LabelResult},
	)
)

// Reconciliation duration histograms - track how long reconciliations take.
var (
	// LoadBalancerReconcileDuration tracks the duration of LoadBalancer reconciliations.
	LoadBalancerReconcileDuration = metrics.NewHistogramVec(
		metrics.SubsystemManager,
		"loadbalancer_reconcile_duration_seconds",
		"Duration of LoadBalancer reconciliations in seconds",
		[]string{metrics.LabelNamespace},
		nil,
	)

	// RouteReconcileDuration tracks the duration of Route reconciliations.
	RouteReconcileDuration = metrics.NewHistogramVec(
		metrics.SubsystemManager,
		"route_reconcile_duration_seconds",
		"Duration of Route reconciliations in seconds",
		[]string{metrics.LabelNamespace},
		nil,
	)

	// TenantReconcileDuration tracks the duration of Tenant reconciliations.
	TenantReconcileDuration = metrics.NewHistogramVec(
		metrics.SubsystemManager,
		"tenant_reconcile_duration_seconds",
		"Duration of Tenant reconciliations in seconds",
		[]string{},
		nil,
	)

	// EnvoyCPReconcileDuration tracks the duration of EnvoyCP reconciliations.
	EnvoyCPReconcileDuration = metrics.NewHistogramVec(
		metrics.SubsystemManager,
		"envoycp_reconcile_duration_seconds",
		"Duration of Envoy control plane reconciliations in seconds",
		[]string{},
		nil,
	)
)

// Resource gauges - track the current count of resources.
var (
	// LoadBalancersTotal tracks the total number of LoadBalancer resources.
	LoadBalancersTotal = metrics.NewGaugeVec(
		metrics.SubsystemManager,
		"loadbalancers",
		"Current number of LoadBalancer resources",
		[]string{metrics.LabelNamespace, metrics.LabelTenant, metrics.LabelTopology},
	)

	// RoutesTotal tracks the total number of Route resources.
	RoutesTotal = metrics.NewGaugeVec(
		metrics.SubsystemManager,
		"routes",
		"Current number of Route resources",
		[]string{metrics.LabelNamespace, metrics.LabelTenant, metrics.LabelRouteType},
	)

	// TenantsTotal tracks the total number of Tenant resources.
	TenantsTotal = metrics.NewGauge(
		metrics.SubsystemManager,
		"tenants",
		"Current number of Tenant resources",
	)

	// EnvoyProxiesTotal tracks the total number of Envoy proxy deployments.
	EnvoyProxiesTotal = metrics.NewGaugeVec(
		metrics.SubsystemManager,
		"envoy_proxies",
		"Current number of Envoy proxy deployments",
		[]string{metrics.LabelNamespace, metrics.LabelTopology},
	)
)

// Port allocator metrics.
var (
	// PortAllocatorAllocatedPorts tracks the total number of allocated ports.
	PortAllocatorAllocatedPorts = metrics.NewGauge(
		metrics.SubsystemManager,
		"port_allocator_allocated_ports",
		"Current number of allocated ports in the port allocator",
	)

	// PortAllocatorEndpoints tracks the total number of endpoints being tracked.
	PortAllocatorEndpoints = metrics.NewGauge(
		metrics.SubsystemManager,
		"port_allocator_endpoints",
		"Current number of endpoints tracked by the port allocator",
	)
)

// EnvoyCP snapshot metrics.
var (
	// EnvoyCPSnapshotUpdatesTotal counts the total number of snapshot updates.
	EnvoyCPSnapshotUpdatesTotal = metrics.NewCounterVec(
		metrics.SubsystemManager,
		"envoycp_snapshot_updates_total",
		"Total number of Envoy snapshot updates",
		[]string{"snapshot_name"},
	)

	// EnvoyCPClusters tracks the number of clusters in the Envoy snapshot.
	EnvoyCPClusters = metrics.NewGaugeVec(
		metrics.SubsystemManager,
		"envoycp_clusters",
		"Current number of clusters in the Envoy snapshot",
		[]string{"snapshot_name"},
	)

	// EnvoyCPListeners tracks the number of listeners in the Envoy snapshot.
	EnvoyCPListeners = metrics.NewGaugeVec(
		metrics.SubsystemManager,
		"envoycp_listeners",
		"Current number of listeners in the Envoy snapshot",
		[]string{"snapshot_name"},
	)

	// EnvoyCPEndpoints tracks the number of endpoints in the Envoy snapshot.
	EnvoyCPEndpoints = metrics.NewGaugeVec(
		metrics.SubsystemManager,
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
		// Histograms
		LoadBalancerReconcileDuration,
		RouteReconcileDuration,
		TenantReconcileDuration,
		EnvoyCPReconcileDuration,
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
	metrics.MustRegister(allCollectors()...)
}
