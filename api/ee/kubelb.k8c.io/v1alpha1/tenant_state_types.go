/*
                  Kubermatic Enterprise Read-Only License
                         Version 1.0 ("KERO-1.0”)
                     Copyright © 2025 Kubermatic GmbH

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TenantStateSpec defines the desired state of TenantState.
type TenantStateSpec struct {
	// TenantState is used to represent the status of a tenant so it's spec is empty.
}

// TenantStateStatus defines the observed state of TenantState
type TenantStateStatus struct {
	Version     Version            `json:"version,omitempty"`
	LastUpdated metav1.Time        `json:"lastUpdated,omitempty"`
	Conditions  []metav1.Condition `json:"conditions,omitempty"`

	Tunnel         TunnelState       `json:"tunnel,omitempty"`
	LoadBalancer   LoadBalancerState `json:"loadBalancer,omitempty"`
	AllowedDomains []string          `json:"allowedDomains,omitempty"`
}

type TunnelState struct {
	Disable              bool   `json:"disable,omitempty"`
	Limit                int    `json:"limit,omitempty"`
	ConnectionManagerURL string `json:"connectionManagerURL,omitempty"`
}

type LoadBalancerState struct {
	Disable bool `json:"disable,omitempty"`
	Limit   int  `json:"limit,omitempty"`
}

type Version struct {
	GitVersion string `json:"gitVersion,omitempty"`
	GitCommit  string `json:"gitCommit,omitempty"`
	BuildDate  string `json:"buildDate,omitempty"`
	Edition    string `json:"edition,omitempty"`
}

// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// TenantState is the Schema for the tenants API
type TenantState struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantStateSpec   `json:"spec,omitempty"`
	Status TenantStateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TenantStateList contains a list of TenantState
type TenantStateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TenantState `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TenantState{}, &TenantStateList{})
}
