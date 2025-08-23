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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TenantStateSpec defines the desired state of TenantState.
type TenantStateSpec struct {
	// TenantState is used to represent the status of a tenant so it's spec is empty.
}

// TenantStateStatus defines the observed state of TenantState
type TenantStateStatus struct {
	Version     Version            `json:"version,omitempty"`
	LastUpdated metav1.Time        `json:"lastUpdated,omitempty"`
	Conditions  []metav1.Condition `json:"conditions,omitempty"`

	LoadBalancer LoadBalancerState `json:"loadBalancer,omitempty"`
}

type LoadBalancerState struct {
	Disable bool `json:"disable,omitempty"`
}

type Version struct {
	GitVersion string `json:"gitVersion,omitempty"`
	GitCommit  string `json:"gitCommit,omitempty"`
	BuildDate  string `json:"buildDate,omitempty"`
	Edition    string `json:"edition,omitempty"`
}

// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.version.edition",name="Edition",type="string"

// TenantState is the Schema for the tenants API
type TenantState struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantStateSpec   `json:"spec,omitempty"`
	Status TenantStateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TenantStateList contains a list of TenantState
type TenantStateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TenantState `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TenantState{}, &TenantStateList{})
}
