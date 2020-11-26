package kubelb

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ predicate.Predicate = &MatchingAnnotationPredicate{}

type MatchingAnnotationPredicate struct {
	AnnotationName  string
	AnnotationValue string
}

// Create returns true if the Create event should be processed
func (r *MatchingAnnotationPredicate) Create(e event.CreateEvent) bool {
	return r.Match(e.Meta.GetAnnotations())
}

// Delete returns true if the Delete event should be processed
func (r *MatchingAnnotationPredicate) Delete(e event.DeleteEvent) bool {
	return r.Match(e.Meta.GetAnnotations())
}

// Update returns true if the Update event should be processed
func (r *MatchingAnnotationPredicate) Update(e event.UpdateEvent) bool {
	return r.Match(e.MetaNew.GetAnnotations())
}

// Generic returns true if the Generic event should be processed
func (r *MatchingAnnotationPredicate) Generic(e event.GenericEvent) bool {
	return r.Match(e.Meta.GetAnnotations())
}

func (r *MatchingAnnotationPredicate) Match(annotations map[string]string) bool {
	val, ok := annotations[r.AnnotationName]
	return !(ok && val == "") && val == r.AnnotationValue
}
