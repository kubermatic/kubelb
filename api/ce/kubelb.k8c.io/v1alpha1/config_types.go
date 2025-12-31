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
	EnvoyProxy EnvoyProxy `json:"envoyProxy,omitempty"`

	LoadBalancer LoadBalancerSettings `json:"loadBalancer,omitempty"`
	Ingress      IngressSettings      `json:"ingress,omitempty"`
	GatewayAPI   GatewayAPISettings   `json:"gatewayAPI,omitempty"`
	DNS          ConfigDNSSettings    `json:"dns,omitempty"`
	Certificates CertificatesSettings `json:"certificates,omitempty"`
}

// ConfigDNSSettings defines the global settings for DNS management and automation.
type ConfigDNSSettings struct {
	// WildcardDomain is the domain that will be used as the base domain to create wildcard DNS records for DNS resources.
	// This is only used for determining the hostname for LoadBalancer resources at LoadBalancer.Spec.Hostname.
	// +optional
	WildcardDomain string `json:"wildcardDomain,omitempty"`

	// AllowExplicitHostnames is a flag that can be used to allow explicit hostnames to be used for DNS resources.
	// This is only used when LoadBalancer.Spec.Hostname is set.
	// +optional
	AllowExplicitHostnames bool `json:"allowExplicitHostnames,omitempty"`

	// UseDNSAnnotations is a flag that can be used to add DNS annotations to DNS resources.
	// This is only used when LoadBalancer.Spec.Hostname is set.
	// +optional
	UseDNSAnnotations bool `json:"useDNSAnnotations,omitempty"`

	// UseCertificateAnnotations is a flag that can be used to add Certificate annotations to Certificate resources.
	// This is only used when LoadBalancer.Spec.Hostname is set.
	// +optional
	UseCertificateAnnotations bool `json:"useCertificateAnnotations,omitempty"`
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

	// Image defines the Envoy Proxy image to use.
	// +optional
	Image string `json:"image,omitempty"`

	// GracefulShutdown defines the graceful shutdown configuration for Envoy Proxy.
	// +optional
	GracefulShutdown *EnvoyProxyGracefulShutdown `json:"gracefulShutdown,omitempty"`
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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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

// +kubebuilder:object:root=true

// ConfigList contains a list of Config
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Config `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Config{}, &ConfigList{})
}
