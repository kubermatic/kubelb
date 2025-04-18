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

// TenantSpec defines the desired state of Tenant
type TenantSpec struct {
	AnnotationSettings `json:",inline"`

	LoadBalancer LoadBalancerSettings `json:"loadBalancer,omitempty"`
	Ingress      IngressSettings      `json:"ingress,omitempty"`
	GatewayAPI   GatewayAPISettings   `json:"gatewayAPI,omitempty"`
}

// LoadBalancerSettings defines the settings for the load balancers.
type LoadBalancerSettings struct {
	// Class is the class of the load balancer to use.
	// This has higher precedence than the value specified in the Config.
	// +optional
	Class *string `json:"class,omitempty"`

	// Disable is a flag that can be used to disable L4 load balancing for a tenant.
	Disable bool `json:"disable,omitempty"`
}

// IngressSettings defines the settings for the ingress.
type IngressSettings struct {
	// Class is the class of the ingress to use.
	// This has higher precedence than the value specified in the Config.
	// +optional
	Class *string `json:"class,omitempty"`

	// Disable is a flag that can be used to disable Ingress for a tenant.
	Disable bool `json:"disable,omitempty"`
}

// GatewayAPISettings defines the settings for the gateway API.
type GatewayAPISettings struct {
	// Class is the class of the gateway API to use. This can be used to specify a specific gateway API implementation.
	// This has higher precedence than the value specified in the Config.
	// +optional
	Class *string `json:"class,omitempty"`

	// Disable is a flag that can be used to disable Gateway API for a tenant.
	Disable bool `json:"disable,omitempty"`
}

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
