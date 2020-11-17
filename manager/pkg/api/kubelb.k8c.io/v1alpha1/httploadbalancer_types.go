/*
Copyright 2020 Kubermatic GmbH.

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

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HTTPLoadBalancerSpec defines the desired state of HTTPLoadBalancer
type HTTPLoadBalancerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of HTTPLoadBalancer. Edit HTTPLoadBalancer_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// HTTPLoadBalancerStatus defines the observed state of HTTPLoadBalancer
type HTTPLoadBalancerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=httplb
// +genclient

// HTTPLoadBalancer is the Schema for the httploadbalancers API
type HTTPLoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HTTPLoadBalancerSpec   `json:"spec,omitempty"`
	Status HTTPLoadBalancerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HTTPLoadBalancerList contains a list of HTTPLoadBalancer
type HTTPLoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HTTPLoadBalancer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HTTPLoadBalancer{}, &HTTPLoadBalancerList{})
}
