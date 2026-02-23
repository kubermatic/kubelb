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

package metricsutil

// MetricDescription describes a Prometheus metric for documentation generation.
type MetricDescription struct {
	// Name is the full Prometheus metric name (e.g., "kubelb_ccm_service_reconcile_total").
	Name string
	// Help is the human-readable description of what the metric measures.
	Help string
	// Type is the Prometheus metric type: Counter, Gauge, Histogram, or Summary.
	Type string
	// Labels are the label names used by this metric.
	Labels []string
	// Component identifies which KubeLB component exposes this metric (manager, ccm, envoycp).
	Component string
}

// Metric types as constants for consistency.
const (
	MetricTypeCounter   = "Counter"
	MetricTypeGauge     = "Gauge"
	MetricTypeHistogram = "Histogram"
	MetricTypeSummary   = "Summary"
)

// Component names as constants.
const (
	ComponentManager = "manager"
	ComponentCCM     = "ccm"
	ComponentEnvoyCP = "envoycp"
)
