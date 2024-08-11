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

package unstructured

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func NormalizeUnstructured(obj *unstructured.Unstructured) *unstructured.Unstructured {
	clone := &unstructured.Unstructured{}

	clone.SetAPIVersion(obj.GetAPIVersion())
	clone.SetKind(obj.GetKind())
	clone.SetName(obj.GetName())
	clone.SetNamespace(obj.GetNamespace())
	clone.SetLabels(obj.GetLabels())
	clone.SetAnnotations(obj.GetAnnotations())
	clone.SetUID(obj.GetUID())
	clone.SetOwnerReferences(nil)

	clone.Object["spec"] = obj.Object["spec"]

	annotations := clone.GetAnnotations()
	delete(annotations, corev1.LastAppliedConfigAnnotation)
	clone.SetAnnotations(annotations)

	return clone
}

func ConvertObjectToUnstructured(object client.Object) (*unstructured.Unstructured, error) {
	unstruct, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		return nil, fmt.Errorf("failed to convert resource to unstructured: %w", err)
	}
	return &unstructured.Unstructured{Object: unstruct}, nil
}

func ConvertUnstructuredToObject(unstruct *unstructured.Unstructured) (client.Object, error) {
	var object client.Object
	gvk := unstruct.GetObjectKind().GroupVersionKind()

	switch gvk {
	case networkingv1.SchemeGroupVersion.WithKind("Ingress"):
		object = &networkingv1.Ingress{}
	case corev1.SchemeGroupVersion.WithKind("Service"):
		object = &corev1.Service{}
	case gwapiv1.SchemeGroupVersion.WithKind("Gateway"):
		object = &gwapiv1.Gateway{}
	case gwapiv1.SchemeGroupVersion.WithKind("HTTPRoute"):
		object = &gwapiv1.HTTPRoute{}
	case gwapiv1.SchemeGroupVersion.WithKind("GRPCRoute"):
		object = &gwapiv1.GRPCRoute{}

	default:
		// Not a known type, we can't process further.
		return nil, fmt.Errorf("unsupported type: %s", gvk)
	}

	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstruct.UnstructuredContent(), object)
	if err != nil {
		return nil, fmt.Errorf("failed to convert unstructured to resource: %w", err)
	}
	return object, nil
}
