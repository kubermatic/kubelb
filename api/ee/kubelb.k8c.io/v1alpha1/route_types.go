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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtime "k8s.io/apimachinery/pkg/runtime"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
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
	// This field is automatically populated by the KubeLB CCM and in most cases, users should not set this field manually.
	Kubernetes *KubernetesSource `json:"kubernetes,omitempty"`
}

type KubernetesSource struct {
	// Resources contains the list of resources that are used as the source for the Route.
	// Allowed resources:
	// - networking.k8s.io/ingress
	// - gateway.networking.k8s.io/gateway
	// - gateway.networking.k8s.io/httproute
	// - gateway.networking.k8s.io/grpcroute
	// - gateway.networking.k8s.io/tlsroute
	// - gateway.networking.k8s.io/tcproute
	// - gateway.networking.k8s.io/udproute

	// +optional
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	Route unstructured.Unstructured `json:"resource,omitempty"`

	// Services contains the list of services that are used as the source for the Route.
	// +kubebuilder:pruning:PreserveUnknownFields
	Services []UpstreamService `json:"services,omitempty"`
}

// TODO(waleed): Evaluate if this is really worth it, semantically it makes sense but it adds a lot of boilerplate. Alternatively,
// we can simply use YQ and add the required markers/fields in the CRD. This will also make the CRD more readable.

// UpstreamService is a wrapper over the corev1.Service object.
// This is required as kubebuilder:validation:EmbeddedResource marker adds the x-kubernetes-embedded-resource to the array instead of
// the elements within it. Which results in a broken CRD; validation error. Without this marker, the embedded resource is not properly
// serialized to the CRD.
type UpstreamService struct {
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:EmbeddedResource
	corev1.Service `json:",inline"`
}

// UpstreamReferenceGrant is a wrapper over the sigs.k8s.io/gateway-api/apis/v1alpha2.ReferenceGrant object.
// This is required as kubebuilder:validation:EmbeddedResource marker adds the x-kubernetes-embedded-resource to the array instead of
// the elements within it. Which results in a broken CRD; validation error. Without this marker, the embedded resource is not properly
// serialized to the CRD.
type UpstreamReferenceGrant struct {
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:EmbeddedResource
	gwapiv1alpha2.ReferenceGrant `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".metadata.labels.kubelb\\.k8c\\.io/origin-name",name="OriginName",type="string"
// +kubebuilder:printcolumn:JSONPath=".metadata.labels.kubelb\\.k8c\\.io/origin-ns",name="OriginNamespace",type="string"
// +kubebuilder:printcolumn:JSONPath=".metadata.labels.kubelb\\.k8c\\.io/origin-resource-kind",name="OriginResource",type="string"

// Route is the object that represents a route in the cluster.
type Route struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteSpec   `json:"spec,omitempty"`
	Status RouteStatus `json:"status,omitempty"`
}

// RouteStatus defines the observed state of the Route.
type RouteStatus struct {
	// Resources contains the list of resources that are created/processed as a result of the Route.
	Resources RouteResourcesStatus `json:"resources,omitempty"`
}

type RouteResourcesStatus struct {
	Source string `json:"source,omitempty"`

	Services map[string]RouteServiceStatus `json:"services,omitempty"`

	Route ResourceState `json:"route,omitempty"`
}

type RouteServiceStatus struct {
	ResourceState `json:",inline"`
	Ports         []corev1.ServicePort `json:"ports,omitempty"`
}

type ResourceState struct {
	// APIVersion is the API version of the resource.
	APIVersion string `json:"apiVersion,omitempty"`

	// Kind is the kind of the resource.
	Kind string `json:"kind,omitempty"`

	// Name is the name of the resource.
	Name string `json:"name,omitempty"`

	// Namespace is the namespace of the resource.
	Namespace string `json:"namespace,omitempty"`

	// GeneratedName is the generated name of the resource.
	GeneratedName string `json:"generatedName,omitempty"`

	// Status is the actual status of the resource.
	Status runtime.RawExtension `json:"status,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type ConditionType string

const (
	ConditionResourceAppliedSuccessfully ConditionType = "ResourceAppliedSuccessfully"
)

func (t ConditionType) String() string {
	return string(t)
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

func ConvertServicesToUpstreamServices(services []corev1.Service) []UpstreamService {
	var upstreamServices []UpstreamService
	for _, service := range services {
		upstreamServices = append(upstreamServices, UpstreamService{
			Service: service,
		})
	}
	return upstreamServices
}

func ConvertReferenceGrantsToUpstreamReferenceGrants(referenceGrants []gwapiv1alpha2.ReferenceGrant) []UpstreamReferenceGrant {
	var upstreamReferenceGrants []UpstreamReferenceGrant
	for _, referenceGrant := range referenceGrants {
		upstreamReferenceGrants = append(upstreamReferenceGrants, UpstreamReferenceGrant{
			ReferenceGrant: referenceGrant,
		})
	}
	return upstreamReferenceGrants
}
