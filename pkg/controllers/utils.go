/*
Copyright 2020 The KubeLB Authors.

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

package controllers

import (
	"context"

	"k8c.io/kubelb/pkg/kubelb"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
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
	return r.Match(e.Object.GetAnnotations())
}

// Delete returns true if the Delete event should be processed
func (r *MatchingAnnotationPredicate) Delete(e event.DeleteEvent) bool {
	return r.Match(e.Object.GetAnnotations())
}

// Update returns true if the Update event should be processed
func (r *MatchingAnnotationPredicate) Update(e event.UpdateEvent) bool {
	return r.Match(e.ObjectNew.GetAnnotations())
}

// Generic returns true if the Generic event should be processed
func (r *MatchingAnnotationPredicate) Generic(e event.GenericEvent) bool {
	return r.Match(e.Object.GetAnnotations())
}

func (r *MatchingAnnotationPredicate) Match(annotations map[string]string) bool {
	val, ok := annotations[r.AnnotationName]
	return !(ok && val == "") && val == r.AnnotationValue
}

// Helper functions to check and remove string from a slice of strings.
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

// We only process LoadBalancers that have the kubelb.k8c.io/managed-by label set on the load balancer namespace.
func ByLabelExistsOnNamespace(ctx context.Context, client client.Client) predicate.Funcs {
	return workqueueFilter(func(o ctrlruntimeclient.Object) bool {
		ns := &corev1.Namespace{}
		if err := client.Get(ctx, types.NamespacedName{Name: o.GetNamespace()}, ns); err != nil {
			return false
		}
		if ns.Labels[kubelb.LabelManagedBy] == kubelb.LabelControllerName {
			return true
		}
		return false
	})
}

func workqueueFilter(filter func(o ctrlruntimeclient.Object) bool) predicate.Funcs {
	if filter == nil {
		return predicate.Funcs{}
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return filter(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return filter(e.ObjectOld) || filter(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return filter(e.Object)
		},
	}
}
