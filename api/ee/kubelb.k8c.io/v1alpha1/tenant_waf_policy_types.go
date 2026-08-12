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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TenantWAFPolicySpec defines the desired state of TenantWAFPolicy.
// Exactly one targeting method must be used: targetRef, targetSelector, or default.
// Setting multiple targeting methods is invalid. Policies without any targeting are ignored.
// Feature stage: Beta
// +kubebuilder:validation:XValidation:rule="!(has(self.targetRef) && has(self.targetSelector))",message="targetRef and targetSelector are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!((has(self.default) && self.default) && has(self.targetRef))",message="default and targetRef are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!((has(self.default) && self.default) && has(self.targetSelector))",message="default and targetSelector are mutually exclusive"
type TenantWAFPolicySpec struct {
	// Default when set to true applies this policy to all of this tenant's routes.
	// It is the tenant-scoped analogue of WAFPolicy.global and never affects other
	// tenants or global config.
	// Mutually exclusive with TargetRef and TargetSelector.
	// Policies without default, targetRef, or targetSelector are ignored.
	// +optional
	Default bool `json:"default,omitempty"`

	// TargetRef identifies a specific route by name and optionally namespace.
	// For tenant policies, Kind is HTTPRoute or GRPCRoute and
	// namespace/originNamespace refer to the tenant-cluster namespace.
	// Mutually exclusive with Default and TargetSelector.
	// +optional
	TargetRef *WAFTargetRef `json:"targetRef,omitempty"`

	// TargetSelector selects routes or HTTPRoute/GRPCRoute resources by label.
	// It checks whether the route has the labels or the labels of the HTTPRoute/GRPCRoute resource. In case of a
	// conflict, the labels of the Route resource takes precedence.
	// Mutually exclusive with Default and TargetRef.
	// +optional
	TargetSelector *metav1.LabelSelector `json:"targetSelector,omitempty"`

	// Directives contains SecLang/ModSecurity directives passed to Coraza.
	// Reference: https://coraza.io/docs/seclang/directives/
	//
	// Tenant directives are untrusted. They are validated at sync time by
	// SanitizeTenantDirectives, a default-deny allowlist: dangerous directives
	// (SecRemoteRules, filesystem Include, log/path directives, exec/setenv, and
	// ctl actions targeting admin rule IDs) are rejected. The MaxItems/MaxLength
	// caps below are structural CRD limits; an admin can tighten them further at
	// runtime via Config.spec.waf.maxDirectivesPerPolicy and maxDirectiveLength.
	//
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MaxLength=1024
	// +optional
	Directives []string `json:"directives,omitempty"`

	// FailureMode defines behavior when WAF filter creation fails.
	// - Closed: Block traffic if WAF cannot be applied (default)
	// - Open: Allow traffic without WAF protection
	// Tenants may set this, but an admin enforceFailureMode on Config or Tenant
	// overrides the tenant-chosen value.
	// +kubebuilder:default=Closed
	// +optional
	FailureMode WAFFailureMode `json:"failureMode,omitempty"`
}

// TenantWAFPolicyStatus defines the observed state of TenantWAFPolicy.
type TenantWAFPolicyStatus struct {
	// Conditions describe the current state of the TenantWAFPolicy.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:JSONPath=".spec.targetRef.name",name="Target",type="string"
// +kubebuilder:printcolumn:JSONPath=".status.conditions[?(@.type==\"Valid\")].status",name="Valid",type="string"

// TenantWAFPolicy defines a tenant-authored Web Application Firewall policy for
// L7 routes. Unlike the cluster-scoped WAFPolicy, it is namespaced and created
// by tenants in their own tenant cluster. It applies to HTTPRoute and GRPCRoute
// resources owned by that tenant only.
type TenantWAFPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantWAFPolicySpec   `json:"spec,omitempty"`
	Status TenantWAFPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TenantWAFPolicyList contains a list of TenantWAFPolicy.
type TenantWAFPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TenantWAFPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TenantWAFPolicy{}, &TenantWAFPolicyList{})
}
