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

package ccm

import (
	"time"

	"k8c.io/kubelb/internal/metricsutil"
	ccmmetrics "k8c.io/kubelb/internal/metricsutil/ccm"
)

// recordKubeLBOperation wraps a KubeLB cluster operation with metrics tracking.
// It records the operation count (success/error), latency, and updates the connectivity gauge.
func recordKubeLBOperation(operation string, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start).Seconds()

	ccmmetrics.KubeLBClusterLatency.WithLabelValues(operation).Observe(duration)

	if err != nil {
		ccmmetrics.KubeLBClusterOperationsTotal.WithLabelValues(operation, metricsutil.ResultError).Inc()
		ccmmetrics.KubeLBClusterConnected.Set(0)
		return err
	}

	ccmmetrics.KubeLBClusterOperationsTotal.WithLabelValues(operation, metricsutil.ResultSuccess).Inc()
	ccmmetrics.KubeLBClusterConnected.Set(1)
	return nil
}
