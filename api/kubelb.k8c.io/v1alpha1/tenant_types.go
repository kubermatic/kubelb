package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TenantSpec defines the desired state of Tenant
type TenantSpec struct {
	LoadBalancer LoadBalancerSettings `json:"loadBalancer,omitempty"`
	Ingress 	IngressSettings      `json:"ingress,omitempty"`
	GatewayAPI 	GatewayAPISettings      `json:"gatewayAPI,omitempty"`
}

type LoadBalancerSettings struct {
	// Class is the class of the load balancer to use.
	// +optional
	Class string `json:"class,omitempty"`

	// Annotations is the list of annotations that will be propagated to the LoadBalancer service.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// PropagateAllAnnotations defines whether all annotations will be propagated to the LoadBalancer service. If set to true, Annotations will be ignored.
	// +optional
	PropagateAllAnnotations bool `json:"propagateAllAnnotations,omitempty"`
}

type IngressSettings struct {
	// Class is the class of the ingress to use.
	// +optional
	Class string `json:"class,omitempty"`
}

type GatewayAPISettings struct {
	// Class is the class of the gateway API to use. This can be used to
	// +optional
	Class string `json:"class,omitempty"`
}

// TenantStatus defines the observed state of Tenant
type TenantStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

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
