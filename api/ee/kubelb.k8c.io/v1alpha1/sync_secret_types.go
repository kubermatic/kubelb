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

// SyncSecretStatus defines the observed state of SyncSecret.
type SyncSecretStatus struct {
	// ObservedGeneration is the most recent generation observed for this SyncSecret by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is the current lifecycle phase of the SyncSecret.
	// +optional
	Phase SyncSecretPhase `json:"phase,omitempty"`

	// Conditions represents the latest available observations of the SyncSecret's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// SyncSecretPhase represents the lifecycle phase of a SyncSecret.
type SyncSecretPhase string

const (
	// SyncSecretPhasePending means the SyncSecret has not yet been synced.
	SyncSecretPhasePending SyncSecretPhase = "Pending"
	// SyncSecretPhaseSynced means the SyncSecret has been successfully synced to a Secret.
	SyncSecretPhaseSynced SyncSecretPhase = "Synced"
	// SyncSecretPhaseFailed means the SyncSecret sync failed.
	SyncSecretPhaseFailed SyncSecretPhase = "Failed"
	// SyncSecretPhaseTerminating means the SyncSecret is being deleted.
	SyncSecretPhaseTerminating SyncSecretPhase = "Terminating"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".type",name="Type",type="string"
// +kubebuilder:printcolumn:JSONPath=".status.phase",name="Phase",type="string"
// +kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name="Age",type="date"

// SyncSecret is a wrapper over Kubernetes Secret object. This is used to sync secrets from tenants to the LB cluster in a controlled and secure way.
type SyncSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Source: https://pkg.go.dev/k8s.io/api/core/v1#Secret

	// +optional
	Immutable *bool `json:"immutable,omitempty" protobuf:"varint,5,opt,name=immutable"`
	// +optional
	Data map[string][]byte `json:"data,omitempty" protobuf:"bytes,2,rep,name=data"`

	// +k8s:conversion-gen=false
	// +optional
	StringData map[string]string `json:"stringData,omitempty" protobuf:"bytes,4,rep,name=stringData"`

	// +optional
	Type corev1.SecretType `json:"type,omitempty" protobuf:"bytes,3,opt,name=type,casttype=SecretType"`

	// +optional
	Status SyncSecretStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SyncSecretList contains a list of SyncSecrets
type SyncSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncSecret `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SyncSecret{}, &SyncSecretList{})
}
