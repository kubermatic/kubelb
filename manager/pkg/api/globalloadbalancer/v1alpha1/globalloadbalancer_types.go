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

// LoadBalancerPort contains information on service's port.
type LoadBalancerPort struct {

	// The name of this port within the service. This must be a DNS_LABEL.
	// All ports within a Spec must have unique names. When considering
	// the endpoints for a Service, this must match the 'name' field in the
	// EndpointPort.
	// Optional if only one ServicePort is defined on this service.
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// The IP protocol for this port. Supports "TCP", "UDP", and "SCTP".
	// Default is TCP.
	// +optional
	// +kubebuilder:default:=TCP
	// +kubebuilder:validation:Enum=TCP;UDP;SCTP
	Protocol corev1.Protocol `json:"protocol,omitempty" protobuf:"bytes,2,opt,name=protocol,casttype=Protocol"`

	// The port that will be exposed by the LoadBalancer.
	Port int32 `json:"port" protobuf:"varint,3,opt,name=port"`

	// Number or name of the port to access on the pods targeted by the service.
	// Number must be in the range 1 to 65535. If this is not specified, the value
	// of the 'port' field is used (an identity map).
	// Should be same value as endpoints port
	// Todo: is this needed?
	//TargetPort int32 `json:"targetPort,omitempty" protobuf:"bytes,4,opt,name=targetPort"`
}

// EndpointPort is a tuple that describes a single port.
type EndpointPort struct {

	// The name of this port.  This must match the 'name' field in the
	// corresponding ServicePort.
	// Must be a DNS_LABEL.
	// Optional only if one port is defined.
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// The port number of the endpoint.
	Port int32 `json:"port" protobuf:"varint,2,opt,name=port"`

	// The IP protocol for this port.
	// Must be UDP, TCP, or SCTP.
	// Default is TCP.
	// +optional
	// +kubebuilder:default:=TCP
	// +kubebuilder:validation:Enum=TCP;UDP;SCTP
	Protocol corev1.Protocol `json:"protocol,omitempty" protobuf:"bytes,3,opt,name=protocol,casttype=Protocol"`
}

// EndpointAddress is a tuple that describes single IP address.
type EndpointAddress struct {

	// The IP of this endpoint.
	// May not be loopback (127.0.0.0/8), link-local (169.254.0.0/16),
	// or link-local multicast ((224.0.0.0/24).
	// TODO: This should allow hostname or IP, See #4447.
	IP string `json:"ip" protobuf:"bytes,1,opt,name=ip"`
	// The Hostname of this endpoint
	// +optional
	Hostname string `json:"hostname,omitempty" protobuf:"bytes,3,opt,name=hostname"`
}

// LoadBalancerEndpoints is a group of addresses with a common set of ports. The
// expanded set of endpoints is the Cartesian product of Addresses x Ports.
// For example, given:
//   {
//     Addresses: [{"ip": "10.10.1.1"}, {"ip": "10.10.2.2"}],
//     Ports:     [{"name": "a", "port": 8675}, {"name": "b", "port": 309}]
//   }
// The resulting set of endpoints can be viewed as:
//     a: [ 10.10.1.1:8675, 10.10.2.2:8675 ],
//     b: [ 10.10.1.1:309, 10.10.2.2:309 ]
type LoadBalancerEndpoints struct {
	// IP addresses which offer the related ports that are marked as ready. These endpoints
	// should be considered safe for load balancers and clients to utilize.
	//+kubebuilder:validation:MinItems:=1
	Addresses []EndpointAddress `json:"addresses,omitempty" protobuf:"bytes,1,rep,name=addresses"`

	// Port numbers available on the related IP addresses.
	//+kubebuilder:validation:MinItems:=1
	Ports []EndpointPort `json:"ports,omitempty" protobuf:"bytes,3,rep,name=ports"`
}

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// GlobalLoadBalancerSpec defines the desired state of GlobalLoadBalancer
type GlobalLoadBalancerSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// Type of the load balancer
	// This deciedes which kind of load balancing will be used and which features are enabled
	// +required
	Type LoadBalancerType `json:"type,omitempty"`

	// The list of ports that are exposed by the load balancer service.
	// only needed for layer 4
	// +optional
	Ports []LoadBalancerPort `json:"ports,omitempty"`

	// Sets of addresses and ports that comprise an exposed user service on a cluster.
	// +required
	//+kubebuilder:validation:MinItems:=1
	Endpoints []LoadBalancerEndpoints `json:"endpoints,omitempty"`
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
// +kubebuilder:resource:shortName=glb
// +genclient
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
