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

// Package policies provides Envoy Gateway policy generation for Ingress conversion.
package policies

import (
	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"
)

// PolicyConversionResult holds generated Envoy Gateway policies from annotation conversion.
type PolicyConversionResult struct {
	// SecurityPolicies contains generated SecurityPolicy resources (CORS, IP access, auth)
	SecurityPolicies []*egv1alpha1.SecurityPolicy
	// BackendTrafficPolicies contains generated BackendTrafficPolicy resources (timeouts, rate limits, session affinity)
	BackendTrafficPolicies []*egv1alpha1.BackendTrafficPolicy
	// ClientTrafficPolicies contains generated ClientTrafficPolicy resources (body size limits)
	ClientTrafficPolicies []*egv1alpha1.ClientTrafficPolicy
}

// IsEmpty returns true if no policies were generated.
func (r *PolicyConversionResult) IsEmpty() bool {
	return len(r.SecurityPolicies) == 0 &&
		len(r.BackendTrafficPolicies) == 0 &&
		len(r.ClientTrafficPolicies) == 0
}

// Merge combines another PolicyConversionResult into this one.
func (r *PolicyConversionResult) Merge(other PolicyConversionResult) {
	r.SecurityPolicies = append(r.SecurityPolicies, other.SecurityPolicies...)
	r.BackendTrafficPolicies = append(r.BackendTrafficPolicies, other.BackendTrafficPolicies...)
	r.ClientTrafficPolicies = append(r.ClientTrafficPolicies, other.ClientTrafficPolicies...)
}
