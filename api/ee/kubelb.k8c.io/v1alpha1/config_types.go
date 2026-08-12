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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EnvoyProxyTopology string

const (
	EnvoyProxyTopologyShared    EnvoyProxyTopology = "shared"
	EnvoyProxyTopologyDedicated EnvoyProxyTopology = "dedicated"
	EnvoyProxyTopologyGlobal    EnvoyProxyTopology = "global"
)

type BackendTransportMode string

const (
	BackendTransportModeDirect BackendTransportMode = "Direct"
	BackendTransportModeMTLS   BackendTransportMode = "MTLS"
)

type BackendTransportUDPMode string

const (
	BackendTransportUDPModeTunnel BackendTransportUDPMode = "Tunnel"
	BackendTransportUDPModeDirect BackendTransportUDPMode = "Direct"
)

type TenantProxyServiceType string

const (
	TenantProxyServiceTypeNodePort     TenantProxyServiceType = "NodePort"
	TenantProxyServiceTypeLoadBalancer TenantProxyServiceType = "LoadBalancer"
)

type TenantProxyWorkload string

const (
	TenantProxyWorkloadDaemonSet  TenantProxyWorkload = "DaemonSet"
	TenantProxyWorkloadDeployment TenantProxyWorkload = "Deployment"
)

type BackendTransport struct {
	// Mode controls how management Envoy connects to tenant backends.
	// Direct preserves the existing node-address plus workload NodePort topology.
	// MTLS routes L7 and L4 TCP traffic through a KubeLB-managed tenant Envoy proxy.
	// MTLS is a Beta / Technical Preview feature: safe to enable and supported,
	// but its configuration surface may still change between releases with
	// migration instructions. See https://docs.kubermatic.com/kubermatic/main/architecture/feature-stages/
	// +kubebuilder:validation:Enum=Direct;MTLS
	// +kubebuilder:default=Direct
	// +optional
	Mode BackendTransportMode `json:"mode,omitempty"`

	// UDP configures how UDP traffic reaches tenant backends when Mode is MTLS.
	// It has no effect in Direct mode.
	// +optional
	UDP BackendTransportUDP `json:"udp,omitempty"`

	// TenantProxy tunes the KubeLB-managed tenant Envoy proxy used in the
	// MTLS topology. It has no effect in Direct mode.
	// +optional
	TenantProxy TenantProxy `json:"tenantProxy,omitempty"`
}

// TenantProxy configures the tenant-cluster Envoy proxy for the MTLS
// backend transport.
type TenantProxy struct {
	// ServiceType selects the Service type used to expose the tenant proxy
	// to the management Envoy. With NodePort (default), the CCM publishes
	// node addresses plus the allocated NodePort. With LoadBalancer, the
	// CCM publishes the Service's load balancer ingress IPs/hostnames and
	// the management Envoy dials the fixed tenant proxy port (15443).
	// +kubebuilder:validation:Enum=NodePort;LoadBalancer
	// +kubebuilder:default=NodePort
	// +optional
	ServiceType TenantProxyServiceType `json:"serviceType,omitempty"`

	// Workload selects how the tenant proxy pods are scheduled. DaemonSet
	// (default) runs one proxy per node. Deployment runs a fixed number of
	// replicas spread across nodes; the CCM then publishes only the node
	// addresses that host proxy pods so the management Envoy never dials a
	// node without a local proxy.
	// +kubebuilder:validation:Enum=DaemonSet;Deployment
	// +kubebuilder:default=DaemonSet
	// +optional
	Workload TenantProxyWorkload `json:"workload,omitempty"`

	// Replicas is the number of tenant proxy pods when Workload is
	// Deployment. Ignored for DaemonSet.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=2
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

type BackendTransportUDP struct {
	// Mode selects the UDP transport in the MTLS topology.
	// Tunnel wraps each UDP session in CONNECT-UDP over the encrypted mTLS
	// tenant proxy port. Direct is an escape hatch that keeps UDP on plain
	// per-service NodePorts (unencrypted) for workloads sensitive to the
	// tunnel's MTU overhead or Envoy's upstream CONNECT-UDP maturity.
	// +kubebuilder:validation:Enum=Tunnel;Direct
	// +kubebuilder:default=Tunnel
	// +optional
	Mode BackendTransportUDPMode `json:"mode,omitempty"`
}

// ConfigSpec defines the desired state of the Config
type ConfigSpec struct {
	AnnotationSettings `json:",inline"`

	// EnvoyProxy defines the desired state of the Envoy Proxy
	EnvoyProxy EnvoyProxy `json:"envoyProxy,omitempty"`

	// BackendTransport controls how management Envoy connects to tenant backends.
	// Defaults to Direct for backward compatibility.
	// +optional
	BackendTransport BackendTransport `json:"backendTransport,omitempty"`

	LoadBalancer LoadBalancerSettings       `json:"loadBalancer,omitempty"`
	Ingress      IngressSettings            `json:"ingress,omitempty"`
	GatewayAPI   GatewayAPISettings         `json:"gatewayAPI,omitempty"`
	DNS          ConfigDNSSettings          `json:"dns,omitempty"`
	Certificates ConfigCertificatesSettings `json:"certificates,omitempty"`
	Tunnel       TunnelSettings             `json:"tunnel,omitempty"`

	// CircuitBreaker defines the default circuit breaker configuration for all Envoy clusters.
	// These settings can be overridden at the Tenant level.
	// +optional
	CircuitBreaker *CircuitBreaker `json:"circuitBreaker,omitempty"`

	// Timeouts defines default Envoy timeouts applied to all routes and
	// load balancers in this cluster. Tenant and Route/LoadBalancer
	// settings override these defaults per-field.
	// +optional
	Timeouts *EnvoyTimeouts `json:"timeouts,omitempty"`

	// LoadBalancerPolicy defines the default load balancing policy for all Envoy clusters.
	// These settings can be overridden at the Tenant and LoadBalancer/Route level.
	// +optional
	LoadBalancerPolicy *LoadBalancerPolicy `json:"loadBalancerPolicy,omitempty"`

	// HealthCheck defines the default active health check for all Envoy clusters.
	// Whole-struct override: Tenant and LoadBalancer/Route settings replace this
	// entirely rather than merging per-field.
	// +optional
	HealthCheck *HealthCheck `json:"healthCheck,omitempty"`

	// WAF defines WAF-related settings.
	// +optional
	WAF WAFSettings `json:"waf,omitempty"`

	// Prometheus, when set, gives the manager a Prometheus query endpoint to
	// read metrics from. Optional and bring-your-own: KubeLB does not run a
	// Prometheus.
	// +optional
	Prometheus *PrometheusSettings `json:"prometheus,omitempty"`

	// NetworkPolicy defines the default network policy settings for all tenant namespaces.
	// Tenant has higher precedence than the settings specified at the Config level.
	// +optional
	NetworkPolicy NetworkPolicySettings `json:"networkPolicy,omitempty"`

	// Insights defines settings for the KubeLB insights engine. It only takes
	// effect when the manager runs with --enable-insights.
	// +optional
	Insights InsightsSettings `json:"insights,omitempty"`
}

// InsightsSettings defines the global settings for the insights engine.
type InsightsSettings struct {
	// DisabledChecks lists check IDs the engine must not run, for example
	// ["KLB010"]. Existing findings for a disabled check are removed on the
	// next sweep.
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:Pattern=`^KLB[0-9]{3}$`
	// +optional
	DisabledChecks []string `json:"disabledChecks,omitempty"`
}

// WAFSettings defines settings for the WAF (Web Application Firewall).
type WAFSettings struct {
	// WASMInitContainerImage overrides the image used for the WASM init container.
	// If empty, defaults to the kubelb-manager image detected at runtime.
	// +optional
	WASMInitContainerImage string `json:"wasmInitContainerImage,omitempty"`

	// SkipValidation skips directive validation for WAFPolicies.
	// When true, all WAFPolicies are marked as valid without parsing.
	// +optional
	SkipValidation bool `json:"skipValidation,omitempty"`

	// EnableTenantPolicies is the global opt-in for tenant-authored WAF policies
	// (TenantWAFPolicy). Defaults to false: when unset, TenantWAFPolicies are
	// ignored and their CRD/controller stay inert, so upgrades see zero behavior
	// change until an admin enables the feature.
	// +optional
	EnableTenantPolicies bool `json:"enableTenantPolicies,omitempty"`

	// EnforceFailureMode, when set, overrides the tenant-chosen failureMode on
	// every TenantWAFPolicy cluster-wide. A per-Tenant EnforceFailureMode takes
	// precedence over this value.
	// +optional
	EnforceFailureMode WAFFailureMode `json:"enforceFailureMode,omitempty"`

	// TenantPolicyLimit is the maximum number of TenantWAFPolicies allowed per tenant.
	// If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this
	// is that it is not possible for KubeLB to know which resources are safe to remove.
	// If nil, the number of TenantWAFPolicies per tenant is unlimited.
	// +optional
	TenantPolicyLimit *int `json:"tenantPolicyLimit,omitempty"`

	// MaxDirectivesPerPolicy is the runtime cap on the number of directive lines
	// per TenantWAFPolicy enforced by the sanitizer (multi-line directive items
	// are counted per line). Defaults to 64, matching the TenantWAFPolicy CRD item
	// cap. Set to 0 for unlimited.
	// +kubebuilder:default=64
	// +optional
	MaxDirectivesPerPolicy *int `json:"maxDirectivesPerPolicy,omitempty"`

	// MaxDirectiveLength is the runtime cap on the length of a single tenant
	// directive line enforced by the sanitizer. Defaults to 1024, matching the
	// TenantWAFPolicy CRD per-item length cap. Set to 0 for unlimited.
	// +kubebuilder:default=1024
	// +optional
	MaxDirectiveLength *int `json:"maxDirectiveLength,omitempty"`
}

// PrometheusSecretKeyReference selects one key from a Secret in the KubeLB
// manager namespace.
type PrometheusSecretKeyReference struct {
	// Name of the Secret.
	Name string `json:"name"`
	// Key within the Secret's data.
	Key string `json:"key"`
}

// PrometheusSettings configures the Prometheus query endpoint the manager
// reads metrics from.
type PrometheusSettings struct {
	// URL is the base URL of the Prometheus query API, for example
	// http://prometheus-operated.monitoring.svc:9090.
	// +kubebuilder:validation:Pattern=`^https?://.+`
	URL string `json:"url"`

	// BearerTokenSecretRef reads a bearer token used to authenticate to Prometheus.
	// +optional
	BearerTokenSecretRef *PrometheusSecretKeyReference `json:"bearerTokenSecretRef,omitempty"`

	// CACertSecretRef reads a PEM CA bundle used to verify a TLS Prometheus endpoint.
	// +optional
	CACertSecretRef *PrometheusSecretKeyReference `json:"caCertSecretRef,omitempty"`

	// InsecureSkipVerify disables TLS certificate verification for the endpoint.
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// TunnelSettings defines the global settings for Tunnel resources.
type TunnelSettings struct {
	// Limit is the maximum number of tunnels to create.
	// If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this
	// is that it is not possible for KubeLB to know which resources are safe to remove.
	Limit *int `json:"limit,omitempty"`

	// ConnectionManagerURL is the URL of the connection manager service that handles tunnel connections.
	// This is required if tunneling is enabled.
	// For example: "https://con.example.com"
	// +optional
	ConnectionManagerURL string `json:"connectionManagerURL,omitempty"`

	// Disable indicates whether tunneling feature should be disabled.
	// +optional
	Disable bool `json:"disable,omitempty"`
}

// ConfigDNSSettings defines the global settings for DNS management and automation.
type ConfigDNSSettings struct {
	// Disable is a flag that can be used to disable DNS automation globally for all the tenants.
	Disable bool `json:"disable,omitempty"`

	// WildcardDomain is the domain that will be used as the base domain to create wildcard DNS records for DNS resources.
	// This is only used for determining the hostname for LoadBalancer and Tunnel resources.
	// +optional
	WildcardDomain string `json:"wildcardDomain,omitempty"`

	// AllowExplicitHostnames is a flag that can be used to allow explicit hostnames to be used for DNS resources.
	// This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set.
	// +optional
	AllowExplicitHostnames bool `json:"allowExplicitHostnames,omitempty"`

	// UseDNSAnnotations is a flag that can be used to add DNS annotations to DNS resources.
	// This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set.
	// +optional
	UseDNSAnnotations bool `json:"useDNSAnnotations,omitempty"`

	// UseCertificateAnnotations is a flag that can be used to add Certificate annotations to Certificate resources.
	// This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set.
	// +optional
	UseCertificateAnnotations bool `json:"useCertificateAnnotations,omitempty"`
}

// ConfigCertificatesSettings defines the global settings for the certificates.
type ConfigCertificatesSettings struct {
	// Disable is a flag that can be used to disable certificate automation globally for all the tenants.
	Disable bool `json:"disable,omitempty"`

	// DefaultClusterIssuer is the Cluster Issuer to use for the certificates by default. This is applied when the cluster issuer is not specified in the annotations on the resource itself.
	DefaultClusterIssuer *string `json:"defaultClusterIssuer,omitempty"`
}

// EnvoyProxy defines the desired state of the EnvoyProxy
type EnvoyProxy struct {
	// +kubebuilder:validation:Enum=shared;dedicated;global
	// +kubebuilder:default=shared
	// +kubebuilder:validation:XValidation:rule="self == oldSelf || (self != oldSelf && (oldSelf == 'dedicated' || oldSelf == 'global'))",message="Value is immutable and only allowed change is from dedicated(deprecated) or global(deprecated) to shared"

	// Topology defines the deployment topology for Envoy Proxy. The only supported value is: shared.
	// DEPRECATION NOTICE: The values "dedicated" and "global" are deprecated and will be removed in a future release. They will now default to shared topology.
	// +optional
	Topology EnvoyProxyTopology `json:"topology,omitempty"`

	// UseDaemonset defines whether Envoy Proxy will run as daemonset. By default, Envoy Proxy will run as deployment.
	// If set to true, Replicas will be ignored.
	// +optional
	UseDaemonset bool `json:"useDaemonset,omitempty"`

	// Replicas defines the number of replicas for Envoy Proxy. This field is ignored if UseDaemonset is set to true.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// SinglePodPerNode defines whether Envoy Proxy pods will be spread across nodes. This ensures that multiple replicas are not running on the same node.
	// +optional
	SinglePodPerNode bool `json:"singlePodPerNode,omitempty"`

	// NodeSelector is used to select nodes to run Envoy Proxy. If specified, the node must have all the indicated labels.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations is used to schedule Envoy Proxy pods on nodes with matching taints.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Resources defines the resource requirements for Envoy Proxy.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Affinity is used to schedule Envoy Proxy pods on nodes with matching affinity.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Image defines the Envoy Proxy image to use.
	// +optional
	Image string `json:"image,omitempty"`

	// GracefulShutdown defines the graceful shutdown configuration for Envoy Proxy.
	// +optional
	GracefulShutdown *EnvoyProxyGracefulShutdown `json:"gracefulShutdown,omitempty"`

	// OverloadManager defines the overload manager configuration for Envoy XDS bootstrap.
	// +optional
	OverloadManager *EnvoyProxyOverloadManager `json:"overloadManager,omitempty"`

	// MaxEndpointsPerCluster limits the number of upstream endpoint addresses per Envoy cluster.
	// When set to a positive value, only the first N endpoints are included in the xDS as upstream addresses.
	// Defaults to 0, which means no limit.
	// +optional
	MaxEndpointsPerCluster int32 `json:"maxEndpointsPerCluster,omitempty"`

	// ImagePullSecrets is a list of references to secrets in the same namespace to use for pulling the Envoy Proxy image.
	// If not set, imagePullSecrets are auto-detected from the manager pod.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// PodMonitor enables creation of PodMonitor resources for Envoy Proxy pods
	// to enable metrics scraping by Prometheus Operator.
	// +optional
	PodMonitor *EnvoyProxyPodMonitor `json:"podMonitor,omitempty"`

	// HeaderLimits configures the client header size and count limits for the
	// KubeLB-managed Envoy Proxy. Unset fields default to Envoy's maximum so the
	// managed proxy never rejects headers that the edge proxy already accepted.
	// +optional
	HeaderLimits *EnvoyProxyHeaderLimits `json:"headerLimits,omitempty"`
}

// EnvoyProxyPodMonitor defines the PodMonitor configuration for Envoy Proxy
type EnvoyProxyPodMonitor struct {
	// Enabled controls whether a PodMonitor is created for Envoy Proxy pods.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// EnvoyProxyHeaderLimits configures the client header size and count limits for
// the KubeLB-managed Envoy Proxy. Envoy rejects requests whose headers exceed
// its 60 KiB default with HTTP 431; these fields raise that ceiling.
type EnvoyProxyHeaderLimits struct {
	// MaxRequestHeadersKb is the maximum request header block size in KiB.
	// Envoy's default is 60; defaults to 8192 (Envoy's maximum) when unset.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=8192
	MaxRequestHeadersKb *uint32 `json:"maxRequestHeadersKb,omitempty"`

	// MaxRequestHeadersCount is the maximum number of request headers.
	// Envoy's default is 100; defaults to 4096 when unset.
	// +optional
	// +kubebuilder:validation:Minimum=1
	MaxRequestHeadersCount *uint32 `json:"maxRequestHeadersCount,omitempty"`

	// MaxResponseHeadersKb is the maximum upstream response header block size in KiB.
	// Envoy's default is 60; defaults to 8192 (Envoy's maximum) when unset.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=8192
	MaxResponseHeadersKb *uint32 `json:"maxResponseHeadersKb,omitempty"`
}

// EnvoyProxyOverloadManager defines the overload manager configuration for Envoy XDS
type EnvoyProxyOverloadManager struct {
	// Enabled controls whether overload manager is enabled
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// MaxActiveDownstreamConnections is the maximum number of active downstream connections for the Envoy.
	// +optional
	MaxActiveDownstreamConnections uint64 `json:"maxActiveDownstreamConnections,omitempty"`

	// MaxHeapSizeBytes is the maximum heap size for the Envoy in bytes. On reaching the limit, the Envoy will start to reject new connections.
	// +optional
	MaxHeapSizeBytes uint64 `json:"maxHeapSizeBytes,omitempty"`
}

// EnvoyProxyGracefulShutdown defines the graceful shutdown configuration for Envoy Proxy
type EnvoyProxyGracefulShutdown struct {
	// Disabled controls whether graceful shutdown is disabled
	// +optional
	Disabled bool `json:"disabled,omitempty"`

	// DrainTimeout is the maximum time to wait for connections to drain.
	// Defaults to 60s. Must be less than TerminationGracePeriodSeconds.
	// +kubebuilder:default="60s"
	// +optional
	DrainTimeout *metav1.Duration `json:"drainTimeout,omitempty"`

	// MinDrainDuration is the minimum time to wait before checking connection count.
	// This prevents premature termination. Defaults to 5s.
	// +kubebuilder:default="5s"
	// +optional
	MinDrainDuration *metav1.Duration `json:"minDrainDuration,omitempty"`

	// TerminationGracePeriodSeconds is the grace period for pod termination.
	// Must be greater than DrainTimeout. Defaults to 300s.
	// +kubebuilder:default=300
	// +kubebuilder:validation:Minimum=30
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// ShutdownManagerImage is the Docker image for the shutdown-manager sidecar.
	// Defaults to "docker.io/envoyproxy/gateway:v1.8.3"
	// +optional
	ShutdownManagerImage string `json:"shutdownManagerImage,omitempty"`
}

// ConfigStatus defines the observed state of the Config.
type ConfigStatus struct {
	Version Version `json:"version,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.version.edition",name="Edition",type="string"
// +kubebuilder:printcolumn:JSONPath=".status.version.gitVersion",name="Version",type="string"

// Config is the object that represents the Config for the KubeLB management controller.
type Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigSpec   `json:"spec,omitempty"`
	Status ConfigStatus `json:"status,omitempty"`
}

func (c *Config) GetEnvoyProxyTopology() EnvoyProxyTopology {
	return c.Spec.EnvoyProxy.Topology
}

//+kubebuilder:object:root=true

// ConfigList contains a list of Config
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Config `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Config{}, &ConfigList{})
}
