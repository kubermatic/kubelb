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
	for k, v := range a {
		aCopy[k] = v
	}
	for k, v := range b {
		bCopy[k] = v
	}
	delete(aCopy, corev1.LastAppliedConfigAnnotation)
	delete(bCopy, corev1.LastAppliedConfigAnnotation)
	return equality.Semantic.DeepEqual(aCopy, bCopy)
}
