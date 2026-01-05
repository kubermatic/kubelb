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

// Package ccm provides Prometheus metrics for the KubeLB Cloud Controller Manager component.
package ccm

import (
	"github.com/prometheus/client_golang/prometheus"

	"k8c.io/kubelb/internal/metrics"
)

// Reconciliation counters - track the number and outcome of reconciliation attempts.
var (
	// ServiceReconcileTotal counts the total number of Service reconciliation attempts.
	ServiceReconcileTotal = metrics.NewCounterVec(
		metrics.SubsystemCCM,
		"service_reconcile_total",
		"Total number of Service reconciliation attempts",
		[]string{metrics.LabelNamespace, metrics.LabelResult},
	)

	// IngressReconcileTotal counts the total number of Ingress reconciliation attempts.
	IngressReconcileTotal = metrics.NewCounterVec(
		metrics.SubsystemCCM,
		"ingress_reconcile_total",
		"Total number of Ingress reconciliation attempts",
		[]string{metrics.LabelNamespace, metrics.LabelResult},
	)

	// GatewayReconcileTotal counts the total number of Gateway reconciliation attempts.
	GatewayReconcileTotal = metrics.NewCounterVec(
		metrics.SubsystemCCM,
		"gateway_reconcile_total",
		"Total number of Gateway reconciliation attempts",
		[]string{metrics.LabelNamespace, metrics.LabelResult},
	)

	// HTTPRouteReconcileTotal counts the total number of HTTPRoute reconciliation attempts.
	HTTPRouteReconcileTotal = metrics.NewCounterVec(
		metrics.SubsystemCCM,
		"httproute_reconcile_total",
		"Total number of HTTPRoute reconciliation attempts",
		[]string{metrics.LabelNamespace, metrics.LabelResult},
	)

	// GRPCRouteReconcileTotal counts the total number of GRPCRoute reconciliation attempts.
	GRPCRouteReconcileTotal = metrics.NewCounterVec(
		metrics.SubsystemCCM,
		"grpcroute_reconcile_total",
		"Total number of GRPCRoute reconciliation attempts",
		[]string{metrics.LabelNamespace, metrics.LabelResult},
	)

	// NodeReconcileTotal counts the total number of Node reconciliation attempts.
	NodeReconcileTotal = metrics.NewCounterVec(
		metrics.SubsystemCCM,
		"node_reconcile_total",
		"Total number of Node reconciliation attempts",
		[]string{metrics.LabelResult},
	)

	// SyncSecretReconcileTotal counts the total number of SyncSecret reconciliation attempts.
	SyncSecretReconcileTotal = metrics.NewCounterVec(
		metrics.SubsystemCCM,
		"sync_secret_reconcile_total",
		"Total number of SyncSecret reconciliation attempts",
		[]string{metrics.LabelNamespace, metrics.LabelResult},
	)
)

// Reconciliation duration histograms - track how long reconciliations take.
var (
	// ServiceReconcileDuration tracks the duration of Service reconciliations.
	ServiceReconcileDuration = metrics.NewHistogramVec(
		metrics.SubsystemCCM,
		"service_reconcile_duration_seconds",
		"Duration of Service reconciliations in seconds",
		[]string{metrics.LabelNamespace},
		nil,
	)

	// IngressReconcileDuration tracks the duration of Ingress reconciliations.
	IngressReconcileDuration = metrics.NewHistogramVec(
		metrics.SubsystemCCM,
		"ingress_reconcile_duration_seconds",
		"Duration of Ingress reconciliations in seconds",
		[]string{metrics.LabelNamespace},
		nil,
	)

	// GatewayReconcileDuration tracks the duration of Gateway reconciliations.
	GatewayReconcileDuration = metrics.NewHistogramVec(
		metrics.SubsystemCCM,
		"gateway_reconcile_duration_seconds",
		"Duration of Gateway reconciliations in seconds",
		[]string{metrics.LabelNamespace},
		nil,
	)

	// HTTPRouteReconcileDuration tracks the duration of HTTPRoute reconciliations.
	HTTPRouteReconcileDuration = metrics.NewHistogramVec(
		metrics.SubsystemCCM,
		"httproute_reconcile_duration_seconds",
		"Duration of HTTPRoute reconciliations in seconds",
		[]string{metrics.LabelNamespace},
		nil,
	)

	// GRPCRouteReconcileDuration tracks the duration of GRPCRoute reconciliations.
	GRPCRouteReconcileDuration = metrics.NewHistogramVec(
		metrics.SubsystemCCM,
		"grpcroute_reconcile_duration_seconds",
		"Duration of GRPCRoute reconciliations in seconds",
		[]string{metrics.LabelNamespace},
		nil,
	)

	// NodeReconcileDuration tracks the duration of Node reconciliations.
	NodeReconcileDuration = metrics.NewHistogramVec(
		metrics.SubsystemCCM,
		"node_reconcile_duration_seconds",
		"Duration of Node reconciliations in seconds",
		[]string{},
		nil,
	)
)

// Resource gauges - track the current count of managed resources.
var (
	// ManagedServicesTotal tracks the total number of LoadBalancer services managed by CCM.
	ManagedServicesTotal = metrics.NewGaugeVec(
		metrics.SubsystemCCM,
		"managed_services",
		"Current number of LoadBalancer services managed by CCM",
		[]string{metrics.LabelNamespace},
	)

	// ManagedIngressesTotal tracks the total number of Ingresses managed by CCM.
	ManagedIngressesTotal = metrics.NewGaugeVec(
		metrics.SubsystemCCM,
		"managed_ingresses",
		"Current number of Ingresses managed by CCM",
		[]string{metrics.LabelNamespace},
	)

	// ManagedGatewaysTotal tracks the total number of Gateways managed by CCM.
	ManagedGatewaysTotal = metrics.NewGaugeVec(
		metrics.SubsystemCCM,
		"managed_gateways",
		"Current number of Gateways managed by CCM",
		[]string{metrics.LabelNamespace},
	)

	// ManagedHTTPRoutesTotal tracks the total number of HTTPRoutes managed by CCM.
	ManagedHTTPRoutesTotal = metrics.NewGaugeVec(
		metrics.SubsystemCCM,
		"managed_httproutes",
		"Current number of HTTPRoutes managed by CCM",
		[]string{metrics.LabelNamespace},
	)

	// ManagedGRPCRoutesTotal tracks the total number of GRPCRoutes managed by CCM.
	ManagedGRPCRoutesTotal = metrics.NewGaugeVec(
		metrics.SubsystemCCM,
		"managed_grpcroutes",
		"Current number of GRPCRoutes managed by CCM",
		[]string{metrics.LabelNamespace},
	)

	// NodesTotal tracks the total number of nodes in the cluster.
	NodesTotal = metrics.NewGauge(
		metrics.SubsystemCCM,
		"nodes",
		"Current number of nodes in the cluster",
	)
)

// KubeLB cluster connectivity metrics.
var (
	// KubeLBClusterConnected indicates whether the CCM is connected to the KubeLB cluster.
	// Value is 1 if connected, 0 otherwise.
	KubeLBClusterConnected = metrics.NewGauge(
		metrics.SubsystemCCM,
		"kubelb_cluster_connected",
		"Whether the CCM is connected to the KubeLB cluster (1=connected, 0=disconnected)",
	)

	// KubeLBClusterOperationsTotal counts operations performed on the KubeLB cluster.
	KubeLBClusterOperationsTotal = metrics.NewCounterVec(
		metrics.SubsystemCCM,
		"kubelb_cluster_operations_total",
		"Total number of operations performed on the KubeLB cluster",
		[]string{metrics.LabelOperation, metrics.LabelResult},
	)

	// KubeLBClusterLatency tracks the latency of operations to the KubeLB cluster.
	KubeLBClusterLatency = metrics.NewHistogramVec(
		metrics.SubsystemCCM,
		"kubelb_cluster_latency_seconds",
		"Latency of operations to the KubeLB cluster in seconds",
		[]string{metrics.LabelOperation},
		nil,
	)
)

// allCollectors returns all metrics collectors for registration.
func allCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		// Reconcile counters
		ServiceReconcileTotal,
		IngressReconcileTotal,
		GatewayReconcileTotal,
		HTTPRouteReconcileTotal,
		GRPCRouteReconcileTotal,
		NodeReconcileTotal,
		SyncSecretReconcileTotal,
		// Reconcile histograms
		ServiceReconcileDuration,
		IngressReconcileDuration,
		GatewayReconcileDuration,
		HTTPRouteReconcileDuration,
		GRPCRouteReconcileDuration,
		NodeReconcileDuration,
		// Resource gauges
		ManagedServicesTotal,
		ManagedIngressesTotal,
		ManagedGatewaysTotal,
		ManagedHTTPRoutesTotal,
		ManagedGRPCRoutesTotal,
		NodesTotal,
		// KubeLB cluster metrics
		KubeLBClusterConnected,
		KubeLBClusterOperationsTotal,
		KubeLBClusterLatency,
	}
}

// Register registers all CCM metrics with the controller-runtime metrics registry.
func Register() {
	metrics.MustRegister(allCollectors()...)
}
