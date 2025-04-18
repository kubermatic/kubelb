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

package grpcroute

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"

	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// createOrUpdateGRPCRoute creates or updates the GRPCRoute object in the cluster.
func CreateOrUpdateGRPCRoute(ctx context.Context, log logr.Logger, client ctrlclient.Client, object *gwapiv1.GRPCRoute, referencedServices []metav1.ObjectMeta, namespace string,
	_ *kubelbv1alpha1.Tenant, annotations kubelbv1alpha1.AnnotationSettings, globalTopology bool) error {
	// Name of the services referenced by the Object have to be updated to match the services created against the Route in the LB cluster.
	for i, rule := range object.Spec.Rules {
		for j, filter := range rule.Filters {
			if filter.RequestMirror != nil && (filter.RequestMirror.BackendRef.Kind == nil || *filter.RequestMirror.BackendRef.Kind == kubelb.ServiceKind) {
				ref := filter.RequestMirror.BackendRef
				for _, service := range referencedServices {
					if string(ref.Name) == service.Name {
						ns := ref.Namespace
						// Corresponding service found, update the name.
						if ns == nil || ns == (*gwapiv1.Namespace)(&service.Namespace) {
							object.Spec.Rules[i].Filters[j].RequestMirror.BackendRef.Name = gwapiv1.ObjectName(kubelb.GenerateName(globalTopology, string(service.UID), service.Name, service.Namespace))
							// Set the namespace to nil since all the services are created in the same namespace as the Route.
							object.Spec.Rules[i].Filters[j].RequestMirror.BackendRef.Namespace = nil
						}
					}
				}
			}
		}

		for j, ref := range rule.BackendRefs {
			if ref.Kind == nil || *ref.Kind == kubelb.ServiceKind {
				for _, service := range referencedServices {
					if string(ref.Name) == service.Name {
						ns := ref.Namespace
						// Corresponding service found, update the name.
						if ns == nil || ns == (*gwapiv1.Namespace)(&service.Namespace) {
							object.Spec.Rules[i].BackendRefs[j].Name = gwapiv1.ObjectName(kubelb.GenerateName(globalTopology, string(service.UID), service.Name, service.Namespace))
							// Set the namespace to nil since all the services are created in the same namespace as the Route.
							object.Spec.Rules[i].BackendRefs[j].Namespace = nil
						}
					}
				}
			}
			// Collect services from the filters.
			if ref.Filters != nil {
				for _, filter := range ref.Filters {
					if filter.RequestMirror != nil && (filter.RequestMirror.BackendRef.Kind == nil || *filter.RequestMirror.BackendRef.Kind == kubelb.ServiceKind) {
						ref := filter.RequestMirror.BackendRef
						for _, service := range referencedServices {
							if string(ref.Name) == service.Name {
								ns := ref.Namespace
								// Corresponding service found, update the name.
								if ns == nil || ns == (*gwapiv1.Namespace)(&service.Namespace) {
									object.Spec.Rules[i].Filters[j].RequestMirror.BackendRef.Name = gwapiv1.ObjectName(kubelb.GenerateName(globalTopology, string(service.UID), service.Name, service.Namespace))
									// Set the namespace to nil since all the services are created in the same namespace as the Route.
									object.Spec.Rules[i].Filters[j].RequestMirror.BackendRef.Namespace = nil
								}
							}
						}
					}
				}
			}
		}
	}

	// Process annotations.
	object.Annotations = kubelb.PropagateAnnotations(object.Annotations, annotations, kubelbv1alpha1.AnnotatedResourceGRPCRoute)

	// Process labels
	object.Labels = kubelb.AddKubeLBLabels(object.Labels, object.Name, object.Namespace, "")

	object.Name = kubelb.GenerateName(globalTopology, string(object.UID), object.Name, object.Namespace)
	object.Namespace = namespace
	object.SetUID("") // Reset UID to generate a new UID for the object

	log.V(4).Info("Creating/Updating GRPCRoute", "name", object.Name, "namespace", object.Namespace)
	// Check if it already exists.
	key := ctrlclient.ObjectKey{Namespace: object.Namespace, Name: object.Name}
	existingObject := &gwapiv1.GRPCRoute{}
	if err := client.Get(ctx, key, existingObject); err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get GRPCRoute: %w", err)
		}
		err := client.Create(ctx, object)
		if err != nil {
			return fmt.Errorf("failed to create GRPCRoute: %w", err)
		}
		return nil
	}

	// Update the Ingress object if it is different from the existing one.
	if equality.Semantic.DeepEqual(existingObject.Spec, object.Spec) &&
		equality.Semantic.DeepEqual(existingObject.Labels, object.Labels) &&
		equality.Semantic.DeepEqual(existingObject.Annotations, object.Annotations) {
		return nil
	}

	// Required to update the object.
	object.ResourceVersion = existingObject.ResourceVersion
	object.UID = existingObject.UID

	if err := client.Update(ctx, object); err != nil {
		return fmt.Errorf("failed to update GRPCRoute: %w", err)
	}
	return nil
}

// GetServicesFromGRPCRoute returns a list of services referenced by the given GRPCRoute.
func GetServicesFromGRPCRoute(grpcroute *gwapiv1.GRPCRoute) []types.NamespacedName {
	serviceReferences := make([]types.NamespacedName, 0)
	for _, rule := range grpcroute.Spec.Rules {
		// Collect services from the filters.
		for _, filter := range rule.Filters {
			if filter.RequestMirror != nil && (filter.RequestMirror.BackendRef.Kind == nil || *filter.RequestMirror.BackendRef.Kind == kubelb.ServiceKind) {
				ref := filter.RequestMirror.BackendRef
				serviceReference := types.NamespacedName{
					Name: string(ref.Name),
				}

				if ref.Namespace != nil {
					serviceReference.Namespace = string(*ref.Namespace)
				} else {
					serviceReference.Namespace = grpcroute.Namespace
				}
				serviceReferences = append(serviceReferences, serviceReference)
			}
		}

		// Collect services from the backend references.
		for _, ref := range rule.BackendRefs {
			if ref.Kind == nil || *ref.Kind == kubelb.ServiceKind {
				serviceReference := types.NamespacedName{
					Name: string(ref.Name),
				}

				if ref.Namespace != nil {
					serviceReference.Namespace = string(*ref.Namespace)
				} else {
					serviceReference.Namespace = grpcroute.Namespace
				}
				serviceReferences = append(serviceReferences, serviceReference)
			}

			// Collect services from the filters.
			if ref.Filters != nil {
				for _, filter := range ref.Filters {
					if filter.RequestMirror != nil && (filter.RequestMirror.BackendRef.Kind == nil || *filter.RequestMirror.BackendRef.Kind == kubelb.ServiceKind) {
						ref := filter.RequestMirror.BackendRef
						serviceReference := types.NamespacedName{
							Name: string(ref.Name),
						}

						if ref.Namespace != nil {
							serviceReference.Namespace = string(*ref.Namespace)
						} else {
							serviceReference.Namespace = grpcroute.Namespace
						}
						serviceReferences = append(serviceReferences, serviceReference)
					}
				}
			}
		}
	}

	// Remove duplicates from the list.
	keys := make(map[types.NamespacedName]bool)
	list := []types.NamespacedName{}
	for _, entry := range serviceReferences {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
