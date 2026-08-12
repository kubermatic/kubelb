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

package kubernetes

import (
	"maps"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

// CompareAnnotations compares two annotation maps while ignoring the last-applied-configuration annotation
func CompareAnnotations(a, b map[string]string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Early check to avoid creating copies of the maps.
	if equality.Semantic.DeepEqual(a, b) {
		return true
	}

	// Create copies to avoid mutating the original maps.
	aCopy := make(map[string]string)
	bCopy := make(map[string]string)
	maps.Copy(aCopy, a)
	maps.Copy(bCopy, b)
	delete(aCopy, corev1.LastAppliedConfigAnnotation)
	delete(bCopy, corev1.LastAppliedConfigAnnotation)
	return equality.Semantic.DeepEqual(aCopy, bCopy)
}

// ManagedAnnotationsKey records, on a generated resource, the annotation keys KubeLB
// propagated onto it during the previous reconciliation. It is what lets
// ReconcileAnnotations tell an annotation it owns from one a third party added.
const ManagedAnnotationsKey = "kubelb.k8c.io/managed-annotations"

// ReconcileAnnotations merges desired into existing like MergeAnnotations, but also
// drops the keys KubeLB propagated on an earlier pass and no longer wants. Without
// this, removing an annotation upstream never reaches the generated resource and a
// cloud-provider annotation can never be un-set. Keys KubeLB never set - annotations
// configured by third party controllers on the generated resource - are left alone.
func ReconcileAnnotations(existing, desired map[string]string) map[string]string {
	merged := make(map[string]string, len(existing)+len(desired))
	maps.Copy(merged, existing)

	for _, key := range strings.Split(existing[ManagedAnnotationsKey], ",") {
		if _, keep := desired[key]; !keep && key != "" {
			delete(merged, key)
		}
	}
	maps.Copy(merged, desired)

	managed := make([]string, 0, len(desired))
	for key := range desired {
		if key != ManagedAnnotationsKey {
			managed = append(managed, key)
		}
	}
	if len(managed) == 0 {
		delete(merged, ManagedAnnotationsKey)
		return merged
	}
	sort.Strings(managed)
	merged[ManagedAnnotationsKey] = strings.Join(managed, ",")
	return merged
}

func MergeAnnotations(existing, desired map[string]string) map[string]string {
	// First, check if both are equal. If they are, return the existing annotations.
	if CompareAnnotations(existing, desired) {
		return existing
	}

	// Merge desired annotations with the existing annotations.
	// While creating native resources against the KubeLB CRs, we don't care about the annotation settings and would like to retain all the annotations
	// configured by third party controllers on the existing resource.
	merged := make(map[string]string)
	maps.Copy(merged, existing)
	maps.Copy(merged, desired)
	return merged
}
