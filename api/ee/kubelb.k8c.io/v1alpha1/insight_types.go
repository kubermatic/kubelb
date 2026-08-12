/*
                  Kubermatic Enterprise Read-Only License
                         Version 1.0 ("KERO-1.0")
                     Copyright © 2026 Kubermatic GmbH

   1.	You may only view, read and display for studying purposes the source
      code of the software licensed under this license, and, to the extent
      explicitly provided under this license, the binary code.
   2.	Any use of the software which exceeds the foregoing right, including,
      without limitation, its execution, compilation, copying, modification
      and distribution, is expressly prohibited.
   3.	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
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

// Labels set by the insights engine on every Insight it owns. They exist so
// findings can be listed and aggregated with a label selector instead of
// parsing spec fields.
const (
	LabelInsightCheck    = "kubelb.k8c.io/insight-check"
	LabelInsightSeverity = "kubelb.k8c.io/insight-severity"
	LabelInsightCategory = "kubelb.k8c.io/insight-category"
)

// InsightCategory groups findings by the kind of problem they describe.
// +kubebuilder:validation:Enum=security;reliability;cost;hygiene;migration
type InsightCategory string

const (
	InsightCategorySecurity    InsightCategory = "security"
	InsightCategoryReliability InsightCategory = "reliability"
	InsightCategoryCost        InsightCategory = "cost"
	InsightCategoryHygiene     InsightCategory = "hygiene"
	InsightCategoryMigration   InsightCategory = "migration"
)

// InsightSeverity is how much the finding matters. The values match the
// OpenReports severity enum so findings can be mirrored into Report objects
// without a translation table.
// +kubebuilder:validation:Enum=critical;high;medium;low;info
type InsightSeverity string

const (
	InsightSeverityCritical InsightSeverity = "critical"
	InsightSeverityHigh     InsightSeverity = "high"
	InsightSeverityMedium   InsightSeverity = "medium"
	InsightSeverityLow      InsightSeverity = "low"
	InsightSeverityInfo     InsightSeverity = "info"
)

// InsightTriageState is the operator's verdict on a finding.
// +kubebuilder:validation:Enum=Acknowledged;Snoozed;Dismissed
type InsightTriageState string

const (
	// InsightTriageAcknowledged means the finding is seen and accepted as work
	// to do. It keeps counting towards the posture score.
	InsightTriageAcknowledged InsightTriageState = "Acknowledged"
	// InsightTriageSnoozed hides the finding until snoozeUntil passes, after
	// which it reopens on its own.
	InsightTriageSnoozed InsightTriageState = "Snoozed"
	// InsightTriageDismissed closes the finding for good. A dismissed finding
	// that is detected again stays dismissed.
	InsightTriageDismissed InsightTriageState = "Dismissed"
)

// InsightDismissalReason explains why a finding was dismissed. It is required
// on dismissal so the fleet-wide dismissal mix stays analysable.
// +kubebuilder:validation:Enum=working_as_intended;accepted_risk;false_positive;low_priority;other
type InsightDismissalReason string

const (
	InsightDismissalWorkingAsIntended InsightDismissalReason = "working_as_intended"
	InsightDismissalAcceptedRisk      InsightDismissalReason = "accepted_risk"
	InsightDismissalFalsePositive     InsightDismissalReason = "false_positive"
	InsightDismissalLowPriority       InsightDismissalReason = "low_priority"
	InsightDismissalOther             InsightDismissalReason = "other"
)

// InsightState is the effective state of a finding, computed by the engine from
// the detection result and the operator's triage.
// +kubebuilder:validation:Enum=Open;Acknowledged;Snoozed;Dismissed;Fixed
type InsightState string

const (
	InsightStateOpen         InsightState = "Open"
	InsightStateAcknowledged InsightState = "Acknowledged"
	InsightStateSnoozed      InsightState = "Snoozed"
	InsightStateDismissed    InsightState = "Dismissed"
	// InsightStateFixed means the engine no longer detects the finding. It is
	// machine-observed, never set by an operator.
	InsightStateFixed InsightState = "Fixed"
)

// InsightEvidenceType describes what an evidence entry points at.
// +kubebuilder:validation:Enum=FieldRef;Condition;ObjectRef
type InsightEvidenceType string

const (
	// InsightEvidenceFieldRef points at a field on an object, e.g.
	// "Config/default#spec.waf.skipValidation".
	InsightEvidenceFieldRef InsightEvidenceType = "FieldRef"
	// InsightEvidenceCondition points at a status condition, e.g.
	// "TenantState/default#BackendTransportChangePending".
	InsightEvidenceCondition InsightEvidenceType = "Condition"
	// InsightEvidenceObjectRef points at a whole object.
	InsightEvidenceObjectRef InsightEvidenceType = "ObjectRef"
)

// InsightTargetRef identifies an object the finding is about.
type InsightTargetRef struct {
	// APIVersion of the target.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	APIVersion string `json:"apiVersion"`

	// Kind of the target.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Kind string `json:"kind"`

	// Name of the target.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// Namespace of the target. Empty for cluster-scoped objects.
	// +kubebuilder:validation:MaxLength=253
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// InsightEvidence is a pointer into live cluster state that supports the
// finding. Evidence is always a reference, never a copy, so an Insight cannot
// go stale against the object it describes.
type InsightEvidence struct {
	// Type of reference.
	Type InsightEvidenceType `json:"type"`

	// Ref is the reference itself, in "<Kind>/<name>#<field or condition>" form.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	Ref string `json:"ref"`

	// Note explains what the reference shows.
	// +kubebuilder:validation:MaxLength=512
	// +optional
	Note string `json:"note,omitempty"`
}

// InsightRemediation describes how to resolve a finding. KubeLB never applies
// it: the snippet is documentation, not an action.
type InsightRemediation struct {
	// Summary is the one-line fix.
	// +kubebuilder:validation:MaxLength=1024
	// +optional
	Summary string `json:"summary,omitempty"`

	// Snippet is an optional YAML example of the fix. It is text only and is
	// never applied by KubeLB.
	// +kubebuilder:validation:MaxLength=8192
	// +optional
	Snippet string `json:"snippet,omitempty"`
}

// InsightTriage is the operator's verdict on a finding. It is the only part of
// an Insight that users write; the engine preserves it verbatim across sweeps.
// +kubebuilder:validation:XValidation:rule="(self.state == 'Dismissed') == has(self.reason)",message="reason is required if and only if state is Dismissed"
// +kubebuilder:validation:XValidation:rule="(self.state == 'Snoozed') == has(self.snoozeUntil)",message="snoozeUntil is required if and only if state is Snoozed"
type InsightTriage struct {
	// State is the verdict.
	State InsightTriageState `json:"state"`

	// Reason explains a dismissal. Required when state is Dismissed, forbidden
	// otherwise.
	// +optional
	Reason InsightDismissalReason `json:"reason,omitempty"`

	// SnoozeUntil is when the finding reopens. Required when state is Snoozed,
	// forbidden otherwise.
	// +optional
	SnoozeUntil *metav1.Time `json:"snoozeUntil,omitempty"`
}

// InsightSpec is the finding. Everything except triage is written by the
// insights engine and is overwritten on every sweep.
type InsightSpec struct {
	// Check is the registry ID of the check that produced this finding, e.g.
	// KLB001. It is immutable: a check ID is a permanent contract that docs,
	// dashboards and suppression lists reference.
	// +kubebuilder:validation:Pattern=`^KLB[0-9]{3}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="check is immutable"
	Check string `json:"check"`

	// Slug is the human-readable name of the check, e.g. waf-detection-only.
	// +kubebuilder:validation:MaxLength=63
	Slug string `json:"slug"`

	// Category groups the finding.
	Category InsightCategory `json:"category"`

	// Severity is how much the finding matters.
	Severity InsightSeverity `json:"severity"`

	// Message describes this specific finding, including any fleet-relative
	// context ("4 of 6 tenants with public routes enforce WAF").
	// +kubebuilder:validation:MaxLength=1024
	Message string `json:"message"`

	// TargetRefs are the objects the finding is about.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=32
	TargetRefs []InsightTargetRef `json:"targetRefs"`

	// Evidence points at the live state that produced the finding.
	// +kubebuilder:validation:MaxItems=16
	// +optional
	Evidence []InsightEvidence `json:"evidence,omitempty"`

	// Remediation describes how to fix the finding.
	// +optional
	Remediation InsightRemediation `json:"remediation,omitempty"`

	// DocsURL links to the check's documentation.
	// +kubebuilder:validation:MaxLength=512
	// +optional
	DocsURL string `json:"docsURL,omitempty"`

	// Triage is the operator's verdict. It is the only user-owned field on this
	// object: the engine reads it and never writes it.
	// +optional
	Triage *InsightTriage `json:"triage,omitempty"`
}

// InsightStatus is the engine-computed effective state of a finding.
type InsightStatus struct {
	// State combines the detection result with the operator's triage.
	// +optional
	State InsightState `json:"state,omitempty"`

	// FirstSeen is when the finding was first detected. It survives a
	// fix-and-reappear cycle so flapping stays visible.
	// +optional
	FirstSeen *metav1.Time `json:"firstSeen,omitempty"`

	// LastEvaluated is the last sweep that considered this finding.
	// +optional
	LastEvaluated *metav1.Time `json:"lastEvaluated,omitempty"`

	// FixedAt is when the engine stopped detecting the finding. Fixed insights
	// are deleted after a retention period.
	// +optional
	FixedAt *metav1.Time `json:"fixedAt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=kubelb
// +kubebuilder:printcolumn:JSONPath=".spec.check",name="Check",type="string"
// +kubebuilder:printcolumn:JSONPath=".spec.severity",name="Severity",type="string"
// +kubebuilder:printcolumn:JSONPath=".status.state",name="State",type="string"
// +kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name="Age",type="date"

// Insight is a single finding produced by the KubeLB insights engine: a
// configuration or posture problem the management cluster can see and the
// operator can act on. Insights are operator-facing; they are not synced to
// tenant clusters.
type Insight struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InsightSpec   `json:"spec,omitempty"`
	Status InsightStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InsightList contains a list of Insight.
type InsightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Insight `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Insight{}, &InsightList{})
}
