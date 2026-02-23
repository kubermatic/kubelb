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

import (
	"github.com/prometheus/client_golang/prometheus"
)

// MetricFactory creates Prometheus metrics and captures their descriptions for documentation.
// Using a factory ensures each metric is defined exactly once, eliminating duplication
// between metric definitions and documentation.
type MetricFactory struct {
	subsystem    string
	component    string
	descriptions []MetricDescription
}

// NewFactory creates a new MetricFactory for a specific component.
func NewFactory(subsystem, component string) *MetricFactory {
	return &MetricFactory{
		subsystem: subsystem,
		component: component,
	}
}

// NewCounterVec creates a CounterVec and captures its description.
func (f *MetricFactory) NewCounterVec(name, help string, labels []string) *prometheus.CounterVec {
	f.descriptions = append(f.descriptions, MetricDescription{
		Name:      f.subsystem + "_" + name,
		Help:      help,
		Type:      MetricTypeCounter,
		Labels:    labels,
		Component: f.component,
	})
	return NewCounterVec(f.subsystem, name, help, labels)
}

// NewGaugeVec creates a GaugeVec and captures its description.
func (f *MetricFactory) NewGaugeVec(name, help string, labels []string) *prometheus.GaugeVec {
	f.descriptions = append(f.descriptions, MetricDescription{
		Name:      f.subsystem + "_" + name,
		Help:      help,
		Type:      MetricTypeGauge,
		Labels:    labels,
		Component: f.component,
	})
	return NewGaugeVec(f.subsystem, name, help, labels)
}

// NewGauge creates a Gauge and captures its description.
func (f *MetricFactory) NewGauge(name, help string) prometheus.Gauge {
	f.descriptions = append(f.descriptions, MetricDescription{
		Name:      f.subsystem + "_" + name,
		Help:      help,
		Type:      MetricTypeGauge,
		Labels:    []string{},
		Component: f.component,
	})
	return NewGauge(f.subsystem, name, help)
}

// NewHistogramVec creates a HistogramVec and captures its description.
// If buckets is nil, DefaultBuckets will be used.
func (f *MetricFactory) NewHistogramVec(name, help string, labels []string, buckets []float64) *prometheus.HistogramVec {
	f.descriptions = append(f.descriptions, MetricDescription{
		Name:      f.subsystem + "_" + name,
		Help:      help,
		Type:      MetricTypeHistogram,
		Labels:    labels,
		Component: f.component,
	})
	return NewHistogramVec(f.subsystem, name, help, labels, buckets)
}

// Descriptions returns all metric descriptions captured by this factory.
func (f *MetricFactory) Descriptions() []MetricDescription {
	return f.descriptions
}
