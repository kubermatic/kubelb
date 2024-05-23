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
	runtime "k8s.io/apimachinery/pkg/runtime"
	gwapiv1a2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// RouteSpec defines the desired state of the Route.
type RouteSpec struct {
	// Sets of addresses and ports that comprise an exposed user service on a cluster.
	// +required
	//+kubebuilder:validation:MinItems=1
	Endpoints []LoadBalancerEndpoints `json:"endpoints,omitempty"`

	// Source contains the information about the source of the route. This is used when the route is created from external sources.
	// +optional
	Source RouteSource `json:"source,omitempty"`
}

type RouteSource struct {
	// Kubernetes contains the information about the Kubernetes source.
	Kubernetes *KubernetesSource `json:"kubernetes,omitempty"`
}

type KubernetesSource struct {
	// Resources contains the list of resources that are used as the source for the Route.
	// Allowed resources:
	// - networking.k8s.io/ingress
	// - gateway.networking.k8s.io/httproute
	// - gateway.networking.k8s.io/grpcroute
	// - gateway.networking.k8s.io/tcproute
	// - gateway.networking.k8s.io/udproute
	// +optional
	Route runtime.RawExtension `json:"resource,omitempty"`

	// Services contains the list of services that are used as the source for the Route.
	Services []corev1.Service `json:"services,omitempty"`

	// ReferenceGrants contains the list of ReferenceGrants that are used as the source for the Route.
	// ReferenceGrant identifies kinds of resources in other namespaces that are
	// trusted to reference the specified kinds of resources in the same namespace
	// as the policy.
	ReferenceGrants []gwapiv1a2.ReferenceGrant `json:"referenceGrants,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Route is the object that represents the Config for the KubeLB management controller.
type Route struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteSpec   `json:"spec,omitempty"`
	Status RouteStatus `json:"status,omitempty"`
}

// RouteStatus defines the observed state of the Route.
type RouteStatus struct {
}

//+kubebuilder:object:root=true

// RouteList contains a list of Routes
type RouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Route `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Route{}, &RouteList{})
}
