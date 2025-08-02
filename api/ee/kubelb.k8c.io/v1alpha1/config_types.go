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
}

// TunnelSettings defines the global settings for Tunnel resources.
type TunnelSettings struct {
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
	// This is only used for determining the hostname for LoadBalancer resources at LoadBalancer.Spec.Hostname.
	// +optional
	WildcardDomain string `json:"wildcardDomain,omitempty"`

	// AllowExplicitHostnames is a flag that can be used to allow explicit hostnames to be used for DNS resources.
	// This is only used when LoadBalancer.Spec.Hostname is set.
	// +optional
	AllowExplicitHostnames bool `json:"allowExplicitHostnames,omitempty"`
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
	// +kubebuilder:validation:XValidation:rule="self == oldSelf || (self != oldSelf && oldSelf == 'dedicated')",message="Value is immutable and only allowed change is from dedicated(deprecated) to shared/global"

	// Topology defines the deployment topology for Envoy Proxy. Valid values are: shared and global.
	// DEPRECATION NOTICE: The value "dedicated" is deprecated and will be removed in a future release. Dedicated topology will now default to shared topology.
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

func (c *Config) IsGlobalTopology() bool {
	return c.Spec.EnvoyProxy.Topology == EnvoyProxyTopologyGlobal
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
