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

package httproute

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	utils "k8c.io/kubelb/internal/controllers"
	"k8c.io/kubelb/internal/kubelb"
	"k8c.io/kubelb/internal/resources"

	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// createOrUpdateHTTPRoute creates or updates the HTTPRoute object in the cluster.
func CreateOrUpdateHTTPRoute(ctx context.Context, log logr.Logger, client ctrlclient.Client, object *gwapiv1.HTTPRoute, referencedServices []metav1.ObjectMeta, namespace string,
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
	object.Annotations = kubelb.PropagateAnnotations(object.Annotations, annotations, kubelbv1alpha1.AnnotatedResourceHTTPRoute)

	// Process labels
	object.Labels = kubelb.AddKubeLBLabels(object.Labels, object.Name, object.Namespace, "")

	object.Name = kubelb.GenerateName(globalTopology, string(object.UID), object.Name, object.Namespace)
	object.Namespace = namespace
	object.SetUID("") // Reset UID to generate a new UID for the object

	log.V(4).Info("Creating/Updating HTTPRoute", "name", object.Name, "namespace", object.Namespace)
	// Check if it already exists.
	key := ctrlclient.ObjectKey{Namespace: object.Namespace, Name: object.Name}
	existingObject := &gwapiv1.HTTPRoute{}
	if err := client.Get(ctx, key, existingObject); err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get HTTPRoute: %w", err)
		}
		err := client.Create(ctx, object)
		if err != nil {
			return fmt.Errorf("failed to create HTTPRoute: %w", err)
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
		return fmt.Errorf("failed to update HTTPRoute: %w", err)
	}
	return nil
}

// GetServicesFromHTTPRoute returns a list of services referenced by the given HTTPRoute.
func GetServicesFromHTTPRoute(httpRoute *gwapiv1.HTTPRoute) []types.NamespacedName {
	serviceReferences := make([]types.NamespacedName, 0)
	for _, rule := range httpRoute.Spec.Rules {
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
					serviceReference.Namespace = httpRoute.Namespace
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
					serviceReference.Namespace = httpRoute.Namespace
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
							serviceReference.Namespace = httpRoute.Namespace
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

// CreateHTTPRouteForHostname creates or updates an HTTPRoute resource for hostname configuration
func CreateHTTPRouteForHostname(ctx context.Context, client ctrlclient.Client, loadBalancer *kubelbv1alpha1.LoadBalancer, svcName string, hostname string, tenant *kubelbv1alpha1.Tenant, config *kubelbv1alpha1.Config) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "httproute")
	log.V(2).Info("creating httproute", "hostname", hostname, "service", svcName)

	// Determine gateway class
	var gatewayClass *string
	if tenant.Spec.GatewayAPI.Class != nil {
		gatewayClass = tenant.Spec.GatewayAPI.Class
	} else if config.Spec.GatewayAPI.Class != nil {
		gatewayClass = config.Spec.GatewayAPI.Class
	}

	// Create HTTPRoute resource
	httpRouteName := fmt.Sprintf("%s-httproute", loadBalancer.Name)
	namespace := loadBalancer.Namespace

	// Create HTTPRoute
	port := gwapiv1.PortNumber(loadBalancer.Spec.Ports[0].Port)
	serviceKind := gwapiv1.Kind("Service")

	httpRoute := &gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      httpRouteName,
			Namespace: namespace,
			Labels: map[string]string{
				kubelb.LabelLoadBalancerName:      loadBalancer.Name,
				kubelb.LabelLoadBalancerNamespace: loadBalancer.Namespace,
			},
			Annotations: make(map[string]string),
		},
		Spec: gwapiv1.HTTPRouteSpec{
			CommonRouteSpec: gwapiv1.CommonRouteSpec{
				ParentRefs: []gwapiv1.ParentReference{},
			},
			Hostnames: []gwapiv1.Hostname{
				gwapiv1.Hostname(hostname),
			},
			Rules: []gwapiv1.HTTPRouteRule{
				{
					BackendRefs: []gwapiv1.HTTPBackendRef{
						{
							BackendRef: gwapiv1.BackendRef{
								BackendObjectReference: gwapiv1.BackendObjectReference{
									Name: gwapiv1.ObjectName(svcName),
									Port: &port,
									Kind: &serviceKind,
								},
							},
						},
					},
				},
			},
		},
	}

	// If gateway class is specified, add parent reference
	if gatewayClass != nil {
		// Parent reference would typically point to a Gateway resource
		// For now, we'll just set the gateway class in annotations
		httpRoute.Annotations["gateway.networking.k8s.io/class"] = *gatewayClass
	}

	// Add annotations from config/tenant
	annotations := GetAnnotations(tenant, config)
	httpRoute.Annotations = kubelb.PropagateAnnotations(loadBalancer.Annotations, annotations, kubelbv1alpha1.AnnotatedResourceHTTPRoute)

	// Add cert-manager and external-dns annotations for automated DNS and TLS

	httpRoute.Annotations[resources.CertManagerClusterIssuerAnnotation] = "letsencrypt-prod"
	httpRoute.Annotations[resources.ExternalDNSHostnameAnnotation] = hostname
	httpRoute.Annotations[resources.ExternalDNSTTLAnnotation] = "10"

	// Set controller reference to LoadBalancer so HTTPRoute gets auto-deleted when LoadBalancer is deleted
	if err := ctrl.SetControllerReference(loadBalancer, httpRoute, client.Scheme()); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Check if HTTPRoute already exists
	existingHTTPRoute := &gwapiv1.HTTPRoute{}
	err := client.Get(ctx, types.NamespacedName{Name: httpRouteName, Namespace: namespace}, existingHTTPRoute)

	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get httproute: %w", err)
	}

	if kerrors.IsNotFound(err) {
		// Create new HTTPRoute
		if err := client.Create(ctx, httpRoute); err != nil {
			return fmt.Errorf("failed to create httproute: %w", err)
		}
		log.V(2).Info("created httproute", "name", httpRouteName)
	} else {
		// Update existing HTTPRoute if needed
		if !equality.Semantic.DeepEqual(existingHTTPRoute.Spec, httpRoute.Spec) ||
			!equality.Semantic.DeepEqual(existingHTTPRoute.Labels, httpRoute.Labels) ||
			!utils.CompareAnnotations(existingHTTPRoute.Annotations, httpRoute.Annotations) {
			existingHTTPRoute.Spec = httpRoute.Spec
			existingHTTPRoute.Labels = httpRoute.Labels
			existingHTTPRoute.Annotations = httpRoute.Annotations
			if err := client.Update(ctx, existingHTTPRoute); err != nil {
				return fmt.Errorf("failed to update httproute: %w", err)
			}
			log.V(2).Info("updated httproute", "name", httpRouteName)
		}
	}

	return nil
}

// GetAnnotations is a placeholder function - this will need to be imported or implemented
func GetAnnotations(tenant *kubelbv1alpha1.Tenant, config *kubelbv1alpha1.Config) kubelbv1alpha1.AnnotationSettings {
	// This should be implemented based on the actual GetAnnotations function from the controller
	// For now, returning empty annotations
	return kubelbv1alpha1.AnnotationSettings{}
}
