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

// ConfigSpec defines the desired state of the Config
type ConfigSpec struct {
	AnnotationSettings `json:",inline"`

	// EnvoyProxy defines the desired state of the Envoy Proxy
	EnvoyProxy   EnvoyProxy                 `json:"envoyProxy,omitempty"`
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

	// WAF defines WAF-related settings.
	// +optional
	WAF WAFSettings `json:"waf,omitempty"`
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
	// DEPRECATION NOTICE: The values "dedicated" and "global" are deprecated and will be removed in a future release. Both will now default to shared topology.
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
	// Defaults to "docker.io/envoyproxy/gateway:v1.3.0"
	// +kubebuilder:default="docker.io/envoyproxy/gateway:v1.3.0"
	// +optional
	ShutdownManagerImage string `json:"shutdownManagerImage,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Config is the object that represents the Config for the KubeLB management controller.
type Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ConfigSpec `json:"spec,omitempty"`
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
