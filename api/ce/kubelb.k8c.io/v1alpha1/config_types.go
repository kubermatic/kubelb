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
