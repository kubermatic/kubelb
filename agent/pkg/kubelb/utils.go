package kubelb

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

//Todo: user addressType corev1.NodeAddressType
func GetEndpoints(nodes *corev1.NodeList, addressType corev1.NodeAddressType) []string {
	var clusterEndpoints []string
	for _, node := range nodes.Items {
		var internalIp string
		for _, address := range node.Status.Addresses {
			if address.Type == addressType {
				internalIp = address.Address
			}
		}
		clusterEndpoints = append(clusterEndpoints, internalIp)
	}
	return clusterEndpoints
}

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
