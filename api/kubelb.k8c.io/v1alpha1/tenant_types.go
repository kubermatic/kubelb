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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TenantSpec defines the desired state of Tenant
type TenantSpec struct {
	LoadBalancer LoadBalancerSettings `json:"loadBalancer,omitempty"`
	// Ingress      IngressSettings      `json:"ingress,omitempty"`
	// GatewayAPI   GatewayAPISettings   `json:"gatewayAPI,omitempty"`
}

type LoadBalancerSettings struct {
	// Class is the class of the load balancer to use.
	// +optional
	Class *string `json:"class,omitempty"`

	// PropagatedAnnotations defines the list of annotations(key-value pairs) that will be propagated to the LoadBalancer service. Keep the `value` field empty in the key-value pair to allow any value.
	// This will have a higher precedence than the annotations specified at the Config level.
	// +optional
	PropagatedAnnotations *map[string]string `json:"propagatedAnnotations,omitempty"`

	// PropagateAllAnnotations defines whether all annotations will be propagated to the LoadBalancer service. If set to true, PropagatedAnnotations will be ignored.
	// This will have a higher precedence than the value specified at the Config level.
	// +optional
	PropagateAllAnnotations *bool `json:"propagateAllAnnotations,omitempty"`
}

// type IngressSettings struct {
// 	// Class is the class of the ingress to use.
// 	// +optional
// 	Class *string `json:"class,omitempty"`
// }

// type GatewayAPISettings struct {
// 	// Class is the class of the gateway API to use. This can be used to
// 	// +optional
// 	Class *string `json:"class,omitempty"`
// }

// TenantStatus defines the observed state of Tenant
type TenantStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:resource:scope=Cluster
// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Tenant is the Schema for the tenants API
type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec,omitempty"`
	Status TenantStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TenantList contains a list of Tenant
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tenant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Tenant{}, &TenantList{})
}
