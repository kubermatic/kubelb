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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//
//// LoadBalancerStatus describes if the loadbalancing is readily created
//type LoadBalancerStatus string
//
//const (
//	// ReadyPhase LoadBalancer routing is set up and working
//	ReadyPhase LoadBalancerStatus = "Ready"
//	// CreatingPhase LoadBalancer routing is still being set up by the manager
//	CreatingPhase LoadBalancerStatus = "Creating"
//)

// LoadBalancerType Type of load balancing
type LoadBalancerType string

const (
	// Layer4 LoadBalancer
	Layer4 LoadBalancerType = "L4"
	// Layer7 LoadBalancer
	Layer7 LoadBalancerType = "L7"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Todo: Due to the usage of existing types the description is probably wrong. Check the generated CRD file
// GlobalLoadBalancerSpec defines the desired state of GlobalLoadBalancer
type GlobalLoadBalancerSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// Type of the load balancer
	// This deciedes which kind of load balancing will be used and which features are enabled
	// +required
	Type LoadBalancerType `json:"type,omitempty"`

	// The list of ports that are exposed by the load balancer service.
	Ports []corev1.ServicePort `json:"ports,omitempty"`

	// Sets of addresses and ports that comprise an exposed user service on a cluster.
	// +optional
	Subsets []corev1.EndpointSubset `json:"subsets,omitempty"`
}

// GlobalLoadBalancerStatus defines the observed state of GlobalLoadBalancer
type GlobalLoadBalancerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// LoadBalancer contains the current status of the load-balancer,
	// if one is present.
	// +optional
	LoadBalancer corev1.LoadBalancerStatus `json:"loadBalancer,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// GlobalLoadBalancer is the Schema for the globalloadbalancers API
type GlobalLoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GlobalLoadBalancerSpec   `json:"spec,omitempty"`
	Status GlobalLoadBalancerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GlobalLoadBalancerList contains a list of GlobalLoadBalancer
type GlobalLoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GlobalLoadBalancer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GlobalLoadBalancer{}, &GlobalLoadBalancerList{})
}
