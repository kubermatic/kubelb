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
	"fmt"
	"strings"

	kubelbce "k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FormatTunnelStatus returns a human-readable status string with color indicators
func FormatTunnelStatus(phase kubelbce.TunnelPhase) string {
	switch phase {
	case kubelbce.TunnelPhaseReady:
		return "✓ Ready"
	case kubelbce.TunnelPhasePending:
		return "⏳ Pending"
	case kubelbce.TunnelPhaseFailed:
		return "✗ Failed"
	case kubelbce.TunnelPhaseTerminating:
		return "🗑 Terminating"
	default:
		return string(phase)
	}
}

// TruncateString truncates a string to maxLength with ellipsis if needed
func TruncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	if maxLength <= 3 {
		return s[:maxLength]
	}
	return s[:maxLength-3] + "..."
}

// FormatConditions formats tunnel conditions into a readable string
func FormatConditions(conditions []metav1.Condition) string {
	if len(conditions) == 0 {
		return "No conditions"
	}

	var parts []string
	for _, condition := range conditions {
		status := "Unknown"
		switch condition.Status {
		case "True":
			status = "✓"
		case "False":
			status = "✗"
		}
		parts = append(parts, fmt.Sprintf("%s %s", status, condition.Type))
	}

	return strings.Join(parts, ", ")
}
