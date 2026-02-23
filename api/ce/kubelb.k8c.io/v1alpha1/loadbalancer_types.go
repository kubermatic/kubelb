/*
Copyright 2020 The KubeLB Authors.

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

// PropagateAnnotation controls the annotations propagation. It is possible to provide optional list of comma separated
// values after '=', other annotations not reppresented by any key or not matching the optional values are dropped.
// To configure multiple different annotations, you can provide unique suffix e.g. "kubelb.k8c.io/propagate-annotation-1"
var PropagateAnnotation = "kubelb.k8c.io/propagate-annotation"

// LoadBalancerStatus defines the observed state of LoadBalancer
type LoadBalancerStatus struct {
	// LoadBalancer contains the current status of the load-balancer,
	// if one is present.
	// +optional
	LoadBalancer corev1.LoadBalancerStatus `json:"loadBalancer,omitempty" protobuf:"bytes,1,opt,name=loadBalancer"`

	// Service contains the current status of the LB service.
	// +optional
	Service ServiceStatus `json:"service,omitempty" protobuf:"bytes,2,opt,name=service"`

	// Hostname contains the status for hostname resources.
	// +optional
	Hostname *HostnameStatus `json:"hostname,omitempty" protobuf:"bytes,3,opt,name=hostname"`
}

type HostnameStatus struct {
	// Hostname contains the hostname of the load-balancer.
	// +optional
	Hostname string `json:"hostname,omitempty" protobuf:"bytes,1,opt,name=hostname"`

	// TLSEnabled is true if certificate is created for the hostname.
	// +optional
	TLSEnabled bool `json:"tlsEnabled,omitempty" protobuf:"bytes,2,opt,name=tlsEnabled"`

	// DNSRecordCreated is true if DNS record is created for the hostname.
	// +optional
	DNSRecordCreated bool `json:"dnsRecordCreated,omitempty" protobuf:"bytes,3,opt,name=dnsRecordCreated"`
}

type ServiceStatus struct {
	Ports []ServicePort `json:"ports,omitempty" protobuf:"bytes,1,rep,name=ports"`
}

// ServicePort contains information on service's port.
type ServicePort struct {
	corev1.ServicePort `json:",inline"`
	UpstreamTargetPort int32 `json:"upstreamTargetPort" protobuf:"bytes,4,opt,name=port"`
}

// LoadBalancerPort contains information on service's port.
type LoadBalancerPort struct {
	// The name of this port within the service. This must be a DNS_LABEL.
	// All ports within a Spec must have unique names. When considering
	// the endpoints for a Service, this must match the 'name' field in the
	// EndpointPort.
	// Optional if only one ServicePort is defined on this service.
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// The IP protocol for this port. Defaults to "TCP".
	// +kubebuilder:validation:Enum=TCP;UDP
	Protocol corev1.Protocol `json:"protocol,omitempty" protobuf:"bytes,2,opt,name=protocol,casttype=Protocol"`

	// The port that will be exposed by the LoadBalancer.
	Port int32 `json:"port" protobuf:"varint,3,opt,name=port"`
}

// LoadBalancerSpec defines the desired state of LoadBalancer
type LoadBalancerSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// Sets of addresses and ports that comprise an exposed user service on a cluster.
	// +required
	// +kubebuilder:validation:MinItems=1
	Endpoints []LoadBalancerEndpoints `json:"endpoints,omitempty"`

	// The list of ports that are exposed by the load balancer service.
	// only needed for layer 4
	// +optional
	Ports []LoadBalancerPort `json:"ports,omitempty"`

	// Hostname is the domain name at which the load balancer service will be accessible.
	// When hostname is set, KubeLB will create a route(ingress or httproute) for the service, and expose it with TLS on the given hostname. Currently, only HTTP protocol is supported
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// type determines how the Service is exposed. Defaults to ClusterIP. Valid
	// options are ExternalName, ClusterIP, NodePort, and LoadBalancer.
	// "ExternalName" maps to the specified externalName.
	// "ClusterIP" allocates a cluster-internal IP address for load-balancing to
	// endpoints. Endpoints are determined by the selector or if that is not
	// specified, by manual construction of an Endpoints object. If clusterIP is
	// "None", no virtual IP is allocated and the endpoints are published as a
	// set of endpoints rather than a stable IP.
	// "NodePort" builds on ClusterIP and allocates a port on every node which
	// routes to the clusterIP.
	// "LoadBalancer" builds on NodePort and creates an
	// external load-balancer (if supported in the current cloud) which routes
	// to the clusterIP.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
	// +optional
	// +kubebuilder:default=ClusterIP
	Type corev1.ServiceType `json:"type,omitempty" protobuf:"bytes,4,opt,name=type,casttype=ServiceType"`

	// externalTrafficPolicy denotes if this Service desires to route external traffic to
	// node-local or cluster-wide endpoints. "Local" preserves the client source IP and avoids
	// a second hop for LoadBalancer and Nodeport type services, but risks potentially imbalanced
	// traffic spreading. "Cluster" obscures the client source IP and may cause a second hop to
	// another node, but should have good overall load-spreading.
	// +optional
	ExternalTrafficPolicy corev1.ServiceExternalTrafficPolicy `json:"externalTrafficPolicy,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=lb
// +kubebuilder:printcolumn:JSONPath=".metadata.labels.kubelb\\.k8c\\.io/origin-name",name="OriginName",type="string"
// +kubebuilder:printcolumn:JSONPath=".metadata.labels.kubelb\\.k8c\\.io/origin-ns",name="OriginNamespace",type="string"
// +kubebuilder:printcolumn:JSONPath=".status.hostname.hostname",name="Hostname",type="string"
// +genclient

// LoadBalancer is the Schema for the loadbalancers API
type LoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerSpec   `json:"spec,omitempty"`
	Status LoadBalancerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoadBalancerList contains a list of LoadBalancer
type LoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LoadBalancer{}, &LoadBalancerList{})
}
