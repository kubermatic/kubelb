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

	"k8c.io/kubelb/internal/metricsutil"
)

// factory creates metrics and captures their descriptions for documentation.
var factory = metricsutil.NewFactory(metricsutil.SubsystemCCM, metricsutil.ComponentCCM)

// Reconciliation counters - track the number and outcome of reconciliation attempts.
var (
	// ServiceReconcileTotal counts the total number of Service reconciliation attempts.
	ServiceReconcileTotal = factory.NewCounterVec(
		"service_reconcile_total",
		"Total number of Service reconciliation attempts",
		[]string{metricsutil.LabelNamespace, metricsutil.LabelResult},
	)

	// IngressReconcileTotal counts the total number of Ingress reconciliation attempts.
	IngressReconcileTotal = factory.NewCounterVec(
		"ingress_reconcile_total",
		"Total number of Ingress reconciliation attempts",
		[]string{metricsutil.LabelNamespace, metricsutil.LabelResult},
	)

	// GatewayReconcileTotal counts the total number of Gateway reconciliation attempts.
	GatewayReconcileTotal = factory.NewCounterVec(
		"gateway_reconcile_total",
		"Total number of Gateway reconciliation attempts",
		[]string{metricsutil.LabelNamespace, metricsutil.LabelResult},
	)

	// HTTPRouteReconcileTotal counts the total number of HTTPRoute reconciliation attempts.
	HTTPRouteReconcileTotal = factory.NewCounterVec(
		"httproute_reconcile_total",
		"Total number of HTTPRoute reconciliation attempts",
		[]string{metricsutil.LabelNamespace, metricsutil.LabelResult},
	)

	// GRPCRouteReconcileTotal counts the total number of GRPCRoute reconciliation attempts.
	GRPCRouteReconcileTotal = factory.NewCounterVec(
		"grpcroute_reconcile_total",
		"Total number of GRPCRoute reconciliation attempts",
		[]string{metricsutil.LabelNamespace, metricsutil.LabelResult},
	)

	// NodeReconcileTotal counts the total number of Node reconciliation attempts.
	NodeReconcileTotal = factory.NewCounterVec(
		"node_reconcile_total",
		"Total number of Node reconciliation attempts",
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
	// ServiceReconcileDuration tracks the duration of Service reconciliations.
	ServiceReconcileDuration = factory.NewHistogramVec(
		"service_reconcile_duration_seconds",
		"Duration of Service reconciliations in seconds",
		[]string{metricsutil.LabelNamespace},
		nil,
	)

	// IngressReconcileDuration tracks the duration of Ingress reconciliations.
	IngressReconcileDuration = factory.NewHistogramVec(
		"ingress_reconcile_duration_seconds",
		"Duration of Ingress reconciliations in seconds",
		[]string{metricsutil.LabelNamespace},
		nil,
	)

	// GatewayReconcileDuration tracks the duration of Gateway reconciliations.
	GatewayReconcileDuration = factory.NewHistogramVec(
		"gateway_reconcile_duration_seconds",
		"Duration of Gateway reconciliations in seconds",
		[]string{metricsutil.LabelNamespace},
		nil,
	)

	// HTTPRouteReconcileDuration tracks the duration of HTTPRoute reconciliations.
	HTTPRouteReconcileDuration = factory.NewHistogramVec(
		"httproute_reconcile_duration_seconds",
		"Duration of HTTPRoute reconciliations in seconds",
		[]string{metricsutil.LabelNamespace},
		nil,
	)

	// GRPCRouteReconcileDuration tracks the duration of GRPCRoute reconciliations.
	GRPCRouteReconcileDuration = factory.NewHistogramVec(
		"grpcroute_reconcile_duration_seconds",
		"Duration of GRPCRoute reconciliations in seconds",
		[]string{metricsutil.LabelNamespace},
		nil,
	)

	// NodeReconcileDuration tracks the duration of Node reconciliations.
	NodeReconcileDuration = factory.NewHistogramVec(
		"node_reconcile_duration_seconds",
		"Duration of Node reconciliations in seconds",
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

// Resource gauges - track the current count of managed resources.
var (
	// ManagedServicesTotal tracks the total number of LoadBalancer services managed by CCM.
	ManagedServicesTotal = factory.NewGaugeVec(
		"managed_services",
		"Current number of LoadBalancer services managed by CCM",
		[]string{metricsutil.LabelNamespace},
	)

	// ManagedIngressesTotal tracks the total number of Ingresses managed by CCM.
	ManagedIngressesTotal = factory.NewGaugeVec(
		"managed_ingresses",
		"Current number of Ingresses managed by CCM",
		[]string{metricsutil.LabelNamespace},
	)

	// ManagedGatewaysTotal tracks the total number of Gateways managed by CCM.
	ManagedGatewaysTotal = factory.NewGaugeVec(
		"managed_gateways",
		"Current number of Gateways managed by CCM",
		[]string{metricsutil.LabelNamespace},
	)

	// ManagedHTTPRoutesTotal tracks the total number of HTTPRoutes managed by CCM.
	ManagedHTTPRoutesTotal = factory.NewGaugeVec(
		"managed_httproutes",
		"Current number of HTTPRoutes managed by CCM",
		[]string{metricsutil.LabelNamespace},
	)

	// ManagedGRPCRoutesTotal tracks the total number of GRPCRoutes managed by CCM.
	ManagedGRPCRoutesTotal = factory.NewGaugeVec(
		"managed_grpcroutes",
		"Current number of GRPCRoutes managed by CCM",
		[]string{metricsutil.LabelNamespace},
	)

	// NodesTotal tracks the total number of nodes in the cluster.
	NodesTotal = factory.NewGauge(
		"nodes",
		"Current number of nodes in the cluster",
	)
)

// KubeLB cluster connectivity metrics.
var (
	// KubeLBClusterConnected indicates whether the CCM is connected to the KubeLB cluster.
	// Value is 1 if connected, 0 otherwise.
	KubeLBClusterConnected = factory.NewGauge(
		"kubelb_cluster_connected",
		"Whether the CCM is connected to the KubeLB cluster (1=connected, 0=disconnected)",
	)

	// KubeLBClusterOperationsTotal counts operations performed on the KubeLB cluster.
	KubeLBClusterOperationsTotal = factory.NewCounterVec(
		"kubelb_cluster_operations_total",
		"Total number of operations performed on the KubeLB cluster",
		[]string{metricsutil.LabelOperation, metricsutil.LabelResult},
	)

	// KubeLBClusterLatency tracks the latency of operations to the KubeLB cluster.
	KubeLBClusterLatency = factory.NewHistogramVec(
		"kubelb_cluster_latency_seconds",
		"Latency of operations to the KubeLB cluster in seconds",
		[]string{metricsutil.LabelOperation},
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
		SyncSecretReconcileDuration,
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
	metricsutil.MustRegister(allCollectors()...)
}

// ListMetrics returns descriptions of all CCM metrics for documentation generation.
func ListMetrics() []metricsutil.MetricDescription {
	return factory.Descriptions()
}
