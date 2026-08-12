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

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DefaultAddressName is the default name for the Addresses object.
	DefaultAddressName = "default"
	// TenantProxyAddressName is the name of the Addresses object the CCM
	// publishes with the reachable endpoints of the mTLS tenant proxy
	// (load balancer ingress addresses or proxy-bearing node addresses).
	TenantProxyAddressName = "tenant-proxy"
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

// NetworkPolicySettings defines the network policy configuration for tenants.
// Default policies:
//   - kubelb-deny-all-ingress: Default deny all ingress traffic to tenant namespace
//   - kubelb-allow-same-namespace: Allow pod-to-pod traffic within tenant namespace
//   - kubelb-allow-manager-ingress: Allow ingress from KubeLB manager namespace
//   - kubelb-allow-dns-egress: Allow DNS resolution via kube-system (port 53 UDP/TCP)
//   - kubelb-allow-xds-egress: Allow xDS control plane communication to manager (port 8001/TCP)
//   - kubelb-allow-metrics-ingress: Allow Prometheus metrics scraping (port 19001/TCP)
//   - kubelb-allow-envoy-ingress: Allow all ingress to envoy proxy pods for LoadBalancer traffic
//   - kubelb-allow-envoy-egress: Allow all egress from envoy proxy pods to reach tenant NodePorts
type NetworkPolicySettings struct {
	// Enable to install network policies by default for all tenants.
	// By default(null/false), network policy automation is disabled. This will be enabled by default in a future release.
	// +optional
	Enable *bool `json:"enable,omitempty"`

	// DisabledPolicies is a list of default policy names to skip (e.g. ["kubelb-deny-all-ingress"]).
	// +optional
	DisabledPolicies []string `json:"disabledPolicies,omitempty"`

	// AdditionalPolicies are extra named network policies created alongside remaining defaults.
	// +optional
	AdditionalPolicies []NamedNetworkPolicy `json:"additionalPolicies,omitempty"`
}

// NamedNetworkPolicy is a NetworkPolicySpec with an explicit name.
type NamedNetworkPolicy struct {
	// Name of the network policy.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Spec is the NetworkPolicySpec for this policy.
	Spec networkingv1.NetworkPolicySpec `json:"spec"`
}

// +kubebuilder:validation:Enum=RoundRobin;LeastRequest;Random
type LoadBalancerPolicy string

const (
	LoadBalancerPolicyRoundRobin   LoadBalancerPolicy = "RoundRobin"
	LoadBalancerPolicyLeastRequest LoadBalancerPolicy = "LeastRequest"
	LoadBalancerPolicyRandom       LoadBalancerPolicy = "Random"
)

// UpstreamTLSPolicy defines how KubeLB's Envoy proxy handles TLS to backends.
// +kubebuilder:validation:Enum=Insecure;Verify
type UpstreamTLSPolicy string

const (
	// UpstreamTLSPolicyInsecure enables TLS but skips certificate verification (ACCEPT_UNTRUSTED).
	// Use for self-signed certs, certs without SANs, or expired certs.
	UpstreamTLSPolicyInsecure UpstreamTLSPolicy = "Insecure"

	// UpstreamTLSPolicyVerify enables TLS and verifies the backend certificate against a provided CA.
	UpstreamTLSPolicyVerify UpstreamTLSPolicy = "Verify"
)

// UpstreamTLSConfig configures TLS for connections from KubeLB's Envoy proxy to backend endpoints.
// When not set, Envoy connects using plain TCP (no TLS).
type UpstreamTLSConfig struct {
	// Policy defines the upstream TLS verification mode.
	// +required
	Policy UpstreamTLSPolicy `json:"policy"`

	// CASecretRef references a Secret containing the CA certificate for backend verification.
	// The Secret must contain a "ca.crt" key. Required when policy is "Verify".
	// +optional
	CASecretRef *corev1.LocalObjectReference `json:"caSecretRef,omitempty"`
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

// +kubebuilder:validation:Enum=TCP;HTTP;GRPC
type HealthCheckType string

const (
	HealthCheckTypeTCP  HealthCheckType = "TCP"
	HealthCheckTypeHTTP HealthCheckType = "HTTP"
	HealthCheckTypeGRPC HealthCheckType = "GRPC"
)

// HealthCheck configures Envoy active health checking for the upstream clusters
// backing this resource. When unset, KubeLB applies a default TCP connect-only
// check. This is a whole-struct override: the effective check is taken from the
// first tier that sets it (Route/LoadBalancer > Tenant > Config > built-in
// default), never merged field-by-field across tiers. Fields left unset within
// the chosen tier fall back to the built-in defaults documented below.
// For more info: https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/health_checking
// +kubebuilder:validation:XValidation:rule="!has(self.http) || self.type == 'HTTP'",message="http may only be set when type is HTTP"
// +kubebuilder:validation:XValidation:rule="!has(self.grpc) || self.type == 'GRPC'",message="grpc may only be set when type is GRPC"
type HealthCheck struct {
	// Type of health check to perform. Defaults to TCP (connect-only) when unset.
	// +optional
	Type *HealthCheckType `json:"type,omitempty"`

	// Interval between health checks. Defaults to 5s.
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Timeout for each health check attempt. Defaults to 5s.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// HealthyThreshold is the number of consecutive successful checks before an
	// unhealthy endpoint is marked healthy. Defaults to 2.
	// +kubebuilder:validation:Minimum=1
	// +optional
	HealthyThreshold *int32 `json:"healthyThreshold,omitempty"`

	// UnhealthyThreshold is the number of consecutive failed checks before a
	// healthy endpoint is marked unhealthy. Defaults to 3.
	// +kubebuilder:validation:Minimum=1
	// +optional
	UnhealthyThreshold *int32 `json:"unhealthyThreshold,omitempty"`

	// HTTP configures an HTTP health check. Used only when Type is HTTP.
	// +optional
	HTTP *HTTPHealthCheck `json:"http,omitempty"`

	// GRPC configures a gRPC health check. Used only when Type is GRPC.
	// +optional
	GRPC *GRPCHealthCheck `json:"grpc,omitempty"`
}

// HTTPHealthCheck configures an HTTP/1.1 active health check.
type HTTPHealthCheck struct {
	// Path is the HTTP request path used for the health check. Defaults to "/".
	// +optional
	Path *string `json:"path,omitempty"`

	// Host is the value of the Host/authority header on the health check request.
	// Defaults to the cluster name (Envoy default) when unset.
	// +optional
	Host *string `json:"host,omitempty"`

	// ExpectedStatuses is the list of HTTP status codes considered healthy.
	// Defaults to [200] when unset. Each value must be in the range 100-599.
	// +kubebuilder:validation:items:Minimum=100
	// +kubebuilder:validation:items:Maximum=599
	// +optional
	ExpectedStatuses []int32 `json:"expectedStatuses,omitempty"`
}

// GRPCHealthCheck configures a gRPC active health check (grpc.health.v1.Health).
type GRPCHealthCheck struct {
	// ServiceName is the value passed as the service name in the gRPC health check
	// request. Empty checks overall server health. Optional.
	// +optional
	ServiceName *string `json:"serviceName,omitempty"`

	// Authority is the value of the :authority header on the gRPC health check
	// request. Defaults to the cluster name (Envoy default) when unset. Optional.
	// +optional
	Authority *string `json:"authority,omitempty"`
}

// EnvoyTimeouts configures upstream and connection timeouts on the
// KubeLB-managed Envoy proxy. Nil duration fields inherit from the
// next tier (Route/LB → Tenant → Config → built-in default). A value
// of 0s explicitly disables that timeout (Envoy semantics).
type EnvoyTimeouts struct {
	// Request is the total upstream request timeout for HTTP routes
	// (Envoy route.timeout). Built-in default: 0 (disabled).
	// Applies to: Ingress, HTTPRoute, GRPCRoute.
	// +optional
	Request *metav1.Duration `json:"request,omitempty"`

	// StreamIdle is the maximum time an HTTP stream can be idle without
	// any bytes flowing in either direction (Envoy stream_idle_timeout).
	// Built-in default: 1h.
	// Applies to: Ingress, HTTPRoute, GRPCRoute.
	// +optional
	StreamIdle *metav1.Duration `json:"streamIdle,omitempty"`

	// RequestHeaders is the maximum time to receive complete request
	// headers (Envoy request_headers_timeout). Built-in default: 0
	// (disabled). Applies to: Ingress, HTTPRoute, GRPCRoute.
	// +optional
	RequestHeaders *metav1.Duration `json:"requestHeaders,omitempty"`

	// IdleConnection is the maximum HTTP connection idle time
	// (Envoy common_http_protocol_options.idle_timeout). Built-in
	// default: 1h. Applies to: Ingress, HTTPRoute, GRPCRoute.
	// +optional
	IdleConnection *metav1.Duration `json:"idleConnection,omitempty"`

	// TCPIdle is the TCP proxy idle timeout (Envoy
	// tcp_proxy.idle_timeout). Built-in default: 1h.
	// Applies to: TCPRoute, TLSRoute, L4 LoadBalancer.
	// +optional
	TCPIdle *metav1.Duration `json:"tcpIdle,omitempty"`

	// Connect is the upstream cluster TCP connect timeout
	// (Envoy cluster.connect_timeout). Built-in default: 5s.
	// Applies to: all routes and L4 LoadBalancer.
	// +optional
	Connect *metav1.Duration `json:"connect,omitempty"`

	// UDPIdle is the UDP session idle timeout. When set, it applies to the
	// management Envoy UDP proxy sessions (Envoy udp_proxy idle_timeout)
	// and, in the MTLS topology, to the CONNECT-UDP tunnel streams on both
	// hops. When unset, the per-hop Envoy defaults apply (60s udp_proxy
	// session idle, 5m tunnel stream idle).
	// Applies to: UDPRoute and L4 LoadBalancer UDP ports.
	// +optional
	UDPIdle *metav1.Duration `json:"udpIdle,omitempty"`
}
