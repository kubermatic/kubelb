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

package resources

import (
	"k8c.io/kubelb/internal/kubelb"
	"k8c.io/reconciler/pkg/reconciling"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NamespaceReconciler creates the namespace for Tenant.
func NamespaceReconciler(namespace string, ownerReference metav1.OwnerReference) reconciling.NamedNamespaceReconcilerFactory {
	return func() (string, reconciling.NamespaceReconciler) {
		return namespace, func(ns *corev1.Namespace) (*corev1.Namespace, error) {
			if ns.Labels == nil {
				ns.Labels = make(map[string]string)
			}
			ns.Labels[kubelb.LabelManagedBy] = kubelb.LabelControllerName

			ns.SetOwnerReferences([]metav1.OwnerReference{ownerReference})
			return ns, nil
		}
	}
}
