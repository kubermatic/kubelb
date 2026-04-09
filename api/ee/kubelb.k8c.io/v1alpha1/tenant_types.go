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

// TenantEnvoyProxy defines tenant-level overrides for Envoy Proxy configuration.
type TenantEnvoyProxy struct {
	// Replicas is the number of Envoy Proxy replicas for this tenant.
	// This field is ignored if Config.Spec.EnvoyProxy.UseDaemonset is true.
	// +kubebuilder:validation:Minimum=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Resources defines the resource requirements for the Envoy Proxy container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// TenantSpec defines the desired state of Tenant
type TenantSpec struct {
	AnnotationSettings `json:",inline"`
	LoadBalancer       LoadBalancerSettings `json:"loadBalancer,omitempty"`
	Ingress            IngressSettings      `json:"ingress,omitempty"`
	GatewayAPI         GatewayAPISettings   `json:"gatewayAPI,omitempty"`
	DNS                DNSSettings          `json:"dns,omitempty"`
	Certificates       CertificatesSettings `json:"certificates,omitempty"`
	Tunnel             TenantTunnelSettings `json:"tunnel,omitempty"`

	// EnvoyProxy defines tenant-level overrides for Envoy Proxy configuration.
	// Fields set here take precedence over Config.Spec.EnvoyProxy.
	// +optional
	EnvoyProxy *TenantEnvoyProxy `json:"envoyProxy,omitempty"`

	// CircuitBreaker defines the circuit breaker configuration for this tenant's Envoy clusters.
	// Overrides Config-level settings.
	// +optional
	CircuitBreaker *CircuitBreaker `json:"circuitBreaker,omitempty"`

	// +kubebuilder:default={"**"}

	// List of allowed domains for the tenant. This is used to restrict the domains that can be used
	// for the tenant. If specified, applies on all the components such as Ingress, GatewayAPI, DNS, certificates, etc.
	// Examples:
	// - ["*.example.com"] -> this allows subdomains at the root level such as example.com and test.example.com but won't allow domains at one level above like test.test.example.com
	// - ["**.example.com"] -> this allows all subdomains of example.com such as test.dns.example.com and dns.example.com
	// - ["example.com"] -> this allows only example.com
	// - ["**"] or ["*"] -> this allows all domains
	// Note: "**" was added as a special case to allow any levels of subdomains that come before it. "*" works for only 1 level.
	// Default: value is ["**"] and all domains are allowed.
	// +optional
	AllowedDomains []string `json:"allowedDomains,omitempty"`
}

// LoadBalancerSettings defines the settings for the load balancers.
type LoadBalancerSettings struct {
	// Class is the class of the load balancer to use.
	// This has higher precedence than the value specified in the Config.
	// +optional
	Class *string `json:"class,omitempty"`

	// Limit is the maximum number of load balancers to create.
	// If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this
	// is that it is not possible for KubeLB to know which resources are safe to remove.
	Limit *int `json:"limit,omitempty"`

	// Disable is a flag that can be used to disable L4 load balancing for a tenant.
	Disable bool `json:"disable,omitempty"`
}

// IngressSettings defines the settings for the ingress.
type IngressSettings struct {
	// Class is the class of the ingress to use.
	// This has higher precedence than the value specified in the Config.
	// +optional
	Class *string `json:"class,omitempty"`

	// Disable is a flag that can be used to disable Ingress for a tenant.
	Disable bool `json:"disable,omitempty"`
}

// GatewayAPISettings defines the settings for the gateway API.
type GatewayAPISettings struct {
	// Class is the class of the gateway API to use. This can be used to specify a specific gateway API implementation.
	// This has higher precedence than the value specified in the Config.
	// +optional
	Class *string `json:"class,omitempty"`

	// Disable is a flag that can be used to disable Gateway API for a tenant.
	Disable bool `json:"disable,omitempty"`

	// DefaultGateway is the default gateway reference to use for the tenant. This is only used for load balancer hostname and tunneling.
	// +optional
	DefaultGateway *corev1.ObjectReference `json:"defaultGateway,omitempty"`

	GatewaySettings GatewaySettings `json:"gateway,omitempty"`

	GatewayAPIsSettings `json:",inline"`
}

type GatewayAPIsSettings struct {
	DisableHTTPRoute            bool `json:"disableHTTPRoute,omitempty"`
	DisableGRPCRoute            bool `json:"disableGRPCRoute,omitempty"`
	DisableTCPRoute             bool `json:"disableTCPRoute,omitempty"`
	DisableUDPRoute             bool `json:"disableUDPRoute,omitempty"`
	DisableTLSRoute             bool `json:"disableTLSRoute,omitempty"`
	DisableBackendTrafficPolicy bool `json:"disableBackendTrafficPolicy,omitempty"`
	DisableClientTrafficPolicy  bool `json:"disableClientTrafficPolicy,omitempty"`
}

// GatewaySettings defines the settings for the gateway resource.
type GatewaySettings struct {
	// Limit is the maximum number of gateways to create.
	// If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this
	// is that it is not possible for KubeLB to know which resources are safe to remove.
	Limit *int `json:"limit,omitempty"`
}

// DNSSettings defines the tenant specific settings for DNS management and automation.
type DNSSettings struct {
	// Disable is a flag that can be used to disable DNS automation for a tenant.
	Disable bool `json:"disable,omitempty"`

	// AllowedDomains is a list of allowed domains for automated DNS management. Has a higher precedence than the value specified in the Config.
	// If empty, the value specified in `tenant.spec.allowedDomains` will be used.
	// Examples:
	// - ["*.example.com"] -> this allows subdomains at the root level such as example.com and test.example.com but won't allow domains at one level above like test.test.example.com
	// - ["**.example.com"] -> this allows all subdomains of example.com such as test.dns.example.com and dns.example.com
	// - ["example.com"] -> this allows only example.com
	// - ["**"] or ["*"] -> this allows all domains
	// Note: "**" was added as a special case to allow any levels of subdomains that come before it. "*" works for only 1 level.
	AllowedDomains []string `json:"allowedDomains,omitempty"`

	// WildcardDomain is the domain that will be used as the base domain to create wildcard DNS records for DNS resources.
	// This is only used for determining the hostname for LoadBalancer and Tunnel resources.
	// +optional
	WildcardDomain *string `json:"wildcardDomain,omitempty"`

	// AllowExplicitHostnames is a flag that can be used to allow explicit hostnames to be used for DNS resources.
	// This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set.
	// +optional
	AllowExplicitHostnames *bool `json:"allowExplicitHostnames,omitempty"`

	// UseDNSAnnotations is a flag that can be used to add DNS annotations to DNS resources.
	// This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set.
	// +optional
	UseDNSAnnotations *bool `json:"useDNSAnnotations,omitempty"`

	// UseCertificateAnnotations is a flag that can be used to add Certificate annotations to Certificate resources.
	// This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set.
	// +optional
	UseCertificateAnnotations *bool `json:"useCertificateAnnotations,omitempty"`
}

// CertificatesSettings defines the settings for the certificates.
type CertificatesSettings struct {
	// Disable is a flag that can be used to disable certificate automation for a tenant.
	Disable bool `json:"disable,omitempty"`

	// DefaultClusterIssuer is the Cluster Issuer to use for the certificates by default. This is applied when the cluster issuer is not specified in the annotations on the resource itself.
	DefaultClusterIssuer *string `json:"defaultClusterIssuer,omitempty"`

	// AllowedDomains is a list of allowed domains for automated Certificate management. Has a higher precedence than the value specified in the Config.
	// If empty, the value specified in `tenant.spec.allowedDomains` will be used.
	// Examples:
	// - ["*.example.com"] -> this allows subdomains at the root level such as example.com and test.example.com but won't allow domains at one level above like test.test.example.com
	// - ["**.example.com"] -> this allows all subdomains of example.com such as test.dns.example.com and dns.example.com
	// - ["example.com"] -> this allows only example.com
	// - ["**"] or ["*"] -> this allows all domains
	// Note: "**" was added as a special case to allow any levels of subdomains that come before it. "*" works for only 1 level.
	AllowedDomains []string `json:"allowedDomains,omitempty"`
}

// TenantTunnelSettings defines the settings for the tunnel.
type TenantTunnelSettings struct {
	// Limit is the maximum number of tunnels to create.
	// If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this
	// is that it is not possible for KubeLB to know which resources are safe to remove.
	Limit *int `json:"limit,omitempty"`

	// Disable is a flag that can be used to disable tunneling for a tenant.
	Disable bool `json:"disable,omitempty"`
}

// TenantStatus defines the observed state of Tenant
type TenantStatus struct {
	// ObservedGeneration is the most recent generation observed for this Tenant by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is the current lifecycle phase of the Tenant.
	// +optional
	Phase TenantPhase `json:"phase,omitempty"`

	// Conditions represents the latest available observations of the Tenant's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// TenantPhase represents the lifecycle phase of a Tenant.
type TenantPhase string

const (
	// TenantPhasePending means the Tenant is being provisioned.
	TenantPhasePending TenantPhase = "Pending"
	// TenantPhaseReady means the Tenant has been successfully reconciled.
	TenantPhaseReady TenantPhase = "Ready"
	// TenantPhaseFailed means the Tenant reconciliation failed.
	TenantPhaseFailed TenantPhase = "Failed"
	// TenantPhaseTerminating means the Tenant is being deleted.
	TenantPhaseTerminating TenantPhase = "Terminating"
)

// +kubebuilder:resource:scope=Cluster
// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.phase",name="Phase",type="string"
// +kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name="Age",type="date"

// Tenant is the Schema for the tenants API
type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec,omitempty"`
	Status TenantStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TenantList contains a list of Tenant
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tenant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Tenant{}, &TenantList{})
}
