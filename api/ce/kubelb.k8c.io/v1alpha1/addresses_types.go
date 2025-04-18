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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddressesSpec defines the desired state of Addresses
type AddressesSpec struct {
	// Addresses contains a list of addresses.
	// +kubebuilder:validation:MinItems:=1
	Addresses []EndpointAddress `json:"addresses,omitempty" protobuf:"bytes,1,rep,name=addresses"`
}

// AddressesStatus defines the observed state of Addresses
type AddressesStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Addresses is the Schema for the addresses API
type Addresses struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AddressesSpec   `json:"spec,omitempty"`
	Status AddressesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AddressesList contains a list of Addresses
type AddressesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Addresses `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Addresses{}, &AddressesList{})
}
