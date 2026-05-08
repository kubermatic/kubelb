/*
                  Kubermatic Enterprise Read-Only License
                         Version 1.0 ("KERO-1.0”)
                     Copyright © 2024 Kubermatic GmbH

   1.	You may only view, read and display for studying purposes the source
      code of the software licensed under this license, and, to the extent
      explicitly provided under this license, the binary code.
   2.	Any use of the software which exceeds the foregoing right, including,
      without limitation, its execution, compilation, copying, modification
      and distribution, is expressly prohibited.
   3.	THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND,
      EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
      MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
      IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
      CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
      TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
      SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

   END OF TERMS AND CONDITIONS
*/

package v1alpha1

import corev1 "k8s.io/api/core/v1"

const (
	// DefaultAddressName is the default name for the Addresses object.
	DefaultAddressName = "default"
	// CLIResourceAnnotation is the annotation key for the resource name in the CLI.
	CLIResourceAnnotation = "kubelb.k8c.io/cli-generated"

	// ConditionReady indicates a resource has been successfully reconciled.
	ConditionReady = "Ready"
	// ConditionSynced indicates a SyncSecret has been successfully propagated.
	ConditionSynced = "Synced"
)

// LoadBalancerEndpoints is a group of addresses with a common set of ports. The
// expanded set of endpoints is the Cartesian product of Addresses x Ports.
// For example, given:
//
//	{
//	  Addresses: [{"ip": "10.10.1.1"}, {"ip": "10.10.2.2"}],
//	  Ports:     [{"name": "a", "port": 8675}, {"name": "b", "port": 309}]
//	}
//
// The resulting set of endpoints can be viewed as:
//
//	a: [ 10.10.1.1:8675, 10.10.2.2:8675 ],
//	b: [ 10.10.1.1:309, 10.10.2.2:309 ]
type LoadBalancerEndpoints struct {
	// Name is the name of the endpoints.
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// IP addresses which offer the related ports that are marked as ready. These endpoints
	// should be considered safe for load balancers and clients to utilize.
	//+kubebuilder:validation:MinItems:=1
	Addresses []EndpointAddress `json:"addresses,omitempty" protobuf:"bytes,1,rep,name=addresses"`

	// AddressesReference is a reference to the Addresses object that contains the IP addresses.
	// If this field is set, the Addresses field will be ignored.
	// +optional
	AddressesReference *corev1.ObjectReference `json:"addressesReference,omitempty" protobuf:"bytes,2,opt,name=addressesReference"`

	// Port numbers available on the related IP addresses.
	// This field is ignored for routes that are using kubernetes resources as the source.
	// +optional
	// +kubebuilder:validation:MinItems=1
	Ports []EndpointPort `json:"ports,omitempty" protobuf:"bytes,3,rep,name=ports"`
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

	// The IP protocol for this port. Defaults to "TCP".
	// +kubebuilder:validation:Enum=TCP;UDP
	Protocol corev1.Protocol `json:"protocol,omitempty" protobuf:"bytes,3,opt,name=protocol,casttype=Protocol"`
}

// EndpointAddress is a tuple that describes a single endpoint address. At least
// one of IP or Hostname must be set.
// +kubebuilder:validation:XValidation:rule="size(self.ip) > 0 || size(self.hostname) > 0",message="at least one of ip or hostname must be set"
type EndpointAddress struct {
	// The IP of the endpoint. This can be an IPv4 or IPv6 address.
	// The IP address must not be IP CIDR, Loopback (127.0.0.0/8), link-local (169.254.0.0/16), or link-local multicast ((224.0.0.0/24) addresses.
	// +optional
	IP string `json:"ip,omitempty" protobuf:"bytes,1,opt,name=ip"`
	// The Hostname of this endpoint. Used when the backend has no stable IP and
	// must be resolved by DNS. If both ip and hostname are set, ip wins.
	// +optional
	Hostname string `json:"hostname,omitempty" protobuf:"bytes,3,opt,name=hostname"`
}

type Annotations map[string]string

// +kubebuilder:validation:Enum=all;service;ingress;gateway;httproute;grpcroute;tcproute;udproute;tlsroute
type AnnotatedResource string

const (
	AnnotatedResourceAll       AnnotatedResource = "all"
	AnnotatedResourceService   AnnotatedResource = "service"
	AnnotatedResourceIngress   AnnotatedResource = "ingress"
	AnnotatedResourceGateway   AnnotatedResource = "gateway"
	AnnotatedResourceHTTPRoute AnnotatedResource = "httproute"
	AnnotatedResourceGRPCRoute AnnotatedResource = "grpcroute"
	AnnotatedResourceTCPRoute  AnnotatedResource = "tcproute"
	AnnotatedResourceUDPRoute  AnnotatedResource = "udproute"
	AnnotatedResourceTLSRoute  AnnotatedResource = "tlsroute"
)

type AnnotationSettings struct {
	// PropagatedAnnotations defines the set of annotation key patterns that will be propagated to load balancing resources.
	// Keys support shell-style glob patterns (e.g. "nginx.ingress.kubernetes.io/*"). Keep the value empty to allow any value;
	// otherwise the value is a comma-separated list of permitted values for exact match.
	// Tenant configuration has higher precedence than the annotations specified at the Config level.
	// +optional
	PropagatedAnnotations *map[string]string `json:"propagatedAnnotations,omitempty"`

	// PropagateAllAnnotations defines whether all annotations will be propagated to load balancing resources.
	// If set to true, PropagatedAnnotations is ignored. DeniedAnnotations still applies on top of this flag.
	// Tenant configuration has higher precedence than the value specified at the Config level.
	// +optional
	PropagateAllAnnotations *bool `json:"propagateAllAnnotations,omitempty"`

	// DeniedAnnotations is a list of annotation key patterns that are excluded from propagation, regardless of
	// PropagateAllAnnotations or PropagatedAnnotations. Patterns support shell-style globbing (e.g. "nginx.ingress.kubernetes.io/*").
	// Tenant configuration has higher precedence than the value specified at the Config level.
	// +optional
	DeniedAnnotations []string `json:"deniedAnnotations,omitempty"`

	// DefaultAnnotations defines the list of annotations(key-value pairs) that will be set on the load balancing resources if not already present. A special key `all` can be used to apply the same
	// set of annotations to all resources.
	// Tenant configuration has higher precedence than the annotations specified at the Config level.
	// +optional
	DefaultAnnotations map[AnnotatedResource]Annotations `json:"defaultAnnotations,omitempty"`
}

// CircuitBreaker defines the Circuit Breaker configuration for Envoy clusters.
// Circuit breakers prevent cascading failures by limiting connections/requests to upstream clusters. For more info: https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/circuit_breaking
type CircuitBreaker struct {
	// MaxConnections is the maximum number of connections that Envoy will establish to all endpoints in the cluster.
	// If not specified, the default is 1024.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4294967295
	// +optional
	MaxConnections *int64 `json:"maxConnections,omitempty"`

	// MaxPendingRequests is the maximum number of pending requests that Envoy will queue to the cluster.
	// If not specified, the default is 1024.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4294967295
	// +optional
	MaxPendingRequests *int64 `json:"maxPendingRequests,omitempty"`

	// MaxParallelRequests is the maximum number of parallel requests that Envoy will make to the cluster.
	// This is applicable to HTTP/2 and gRPC connections.
	// If not specified, the default is 1024.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4294967295
	// +optional
	MaxParallelRequests *int64 `json:"maxParallelRequests,omitempty"`

	// MaxParallelRetries is the maximum number of parallel retries that Envoy will make to the cluster.
	// If not specified, the default is 3.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4294967295
	// +optional
	MaxParallelRetries *int64 `json:"maxParallelRetries,omitempty"`

	// MaxRequestsPerConnection is the maximum number of requests that Envoy will make over a single connection
	// to the cluster. If not specified, there is no limit.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4294967295
	// +optional
	MaxRequestsPerConnection *int64 `json:"maxRequestsPerConnection,omitempty"`

	// PerEndpoint configures circuit breaker thresholds that apply to individual endpoints rather than the whole cluster.
	// +optional
	PerEndpoint *PerEndpointCircuitBreaker `json:"perEndpoint,omitempty"`
}

// PerEndpointCircuitBreaker defines circuit breaker thresholds that apply to individual endpoints.
type PerEndpointCircuitBreaker struct {
	// MaxConnections is the maximum number of connections that Envoy will establish to a single endpoint.
	// If not specified, the default is 1024.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4294967295
	// +optional
	MaxConnections *int64 `json:"maxConnections,omitempty"`
}
