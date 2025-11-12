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

package route

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	k8sutils "k8c.io/kubelb/internal/util/kubernetes"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// createOrUpdateIngress creates or updates the Ingress object in the cluster.
func CreateOrUpdateIngress(ctx context.Context, log logr.Logger, client ctrlclient.Client, object *networkingv1.Ingress, referencedServices []metav1.ObjectMeta, namespace string, config *kubelbv1alpha1.Config,
	tenant *kubelbv1alpha1.Tenant, annotations kubelbv1alpha1.AnnotationSettings) error {
	globalTopology := config.IsGlobalTopology()
	// Transformations to make it compliant with the LB cluster.
	// Name of the services referenced by the Ingress have to be updated to match the services created against the Route in the LB cluster.
	for i, rule := range object.Spec.Rules {
		for j, path := range rule.HTTP.Paths {
			for _, service := range referencedServices {
				if path.Backend.Service.Name == service.Name {
					object.Spec.Rules[i].HTTP.Paths[j].Backend.Service.Name = kubelb.GenerateName(globalTopology, string(service.UID), service.Name, service.Namespace)
				}
			}
		}
	}

	if object.Spec.DefaultBackend != nil && object.Spec.DefaultBackend.Service != nil {
		for _, service := range referencedServices {
			if object.Spec.DefaultBackend.Service.Name == service.Name {
				object.Spec.DefaultBackend.Service.Name = kubelb.GenerateName(globalTopology, string(service.UID), service.Name, service.Namespace)
			}
		}
	}

	// Class name depends on the chosen Ingress controller for the tenant or global cluster.
	if tenant.Spec.Ingress.Class != nil {
		object.Spec.IngressClassName = tenant.Spec.Ingress.Class
	} else if config.Spec.Ingress.Class != nil {
		object.Spec.IngressClassName = config.Spec.Ingress.Class
	}

	// Process annotations.
	object.Annotations = kubelb.PropagateAnnotations(object.Annotations, annotations, kubelbv1alpha1.AnnotatedResourceIngress)

	// Process secrets.
	if object.Spec.TLS != nil {
		for i := range object.Spec.TLS {
			secretName := k8sutils.GetSecretNameIfExists(ctx, client, object.Spec.TLS[i].SecretName, object.Namespace, namespace)
			if secretName != "" {
				object.Spec.TLS[i].SecretName = secretName
			}
		}
	}

	// Process labels
	object.Labels = kubelb.AddKubeLBLabels(object.Labels, object.Name, object.Namespace, "")

	// Update name and other fields before creating/updating the object.
	object.Name = kubelb.GenerateName(globalTopology, string(object.UID), object.Name, object.Namespace)
	object.Namespace = namespace
	object.SetUID("") // Reset UID to generate a new UID for the object

	log.V(4).Info("Creating/Updating Ingress", "name", object.Name, "namespace", object.Namespace)
	// Check if it already exists.
	key := ctrlclient.ObjectKey{Namespace: object.Namespace, Name: object.Name}
	existingObject := &networkingv1.Ingress{}
	if err := client.Get(ctx, key, existingObject); err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get Ingress: %w", err)
		}
		err := client.Create(ctx, object)
		if err != nil {
			return fmt.Errorf("failed to create Ingress: %w", err)
		}
		return nil
	}

	// Merge the annotations with the existing annotations to allow annotations that are configured by third party controllers on the existing service to be preserved.
	object.Annotations = k8sutils.MergeAnnotations(existingObject.Annotations, object.Annotations)

	// Update the Ingress object if it is different from the existing one.
	if equality.Semantic.DeepEqual(existingObject.Spec, object.Spec) &&
		equality.Semantic.DeepEqual(existingObject.Labels, object.Labels) &&
		k8sutils.CompareAnnotations(existingObject.Annotations, object.Annotations) {
		return nil
	}

	// Required to update the object.
	object.ResourceVersion = existingObject.ResourceVersion
	object.UID = existingObject.UID

	if err := client.Update(ctx, object); err != nil {
		return fmt.Errorf("failed to update Ingress: %w", err)
	}
	return nil
}

// GetServicesFromIngress returns a list of services referenced by the given Ingress.
func GetServicesFromIngress(ingress networkingv1.Ingress) []types.NamespacedName {
	serviceReferences := make([]types.NamespacedName, 0)
	for _, rule := range ingress.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			serviceReferences = append(serviceReferences, types.NamespacedName{
				Name:      path.Backend.Service.Name,
				Namespace: ingress.Namespace,
			})
		}
	}

	if ingress.Spec.DefaultBackend != nil && ingress.Spec.DefaultBackend.Service != nil {
		serviceReferences = append(serviceReferences, types.NamespacedName{
			Name:      ingress.Spec.DefaultBackend.Service.Name,
			Namespace: ingress.Namespace,
		})
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

// CreateIngressForHostname creates or updates an Ingress resource for hostname configuration
func CreateIngressForHostname(ctx context.Context, client ctrlclient.Client, loadBalancer *kubelbv1alpha1.LoadBalancer, svcName string, hostname string, tenant *kubelbv1alpha1.Tenant, config *kubelbv1alpha1.Config, annotations kubelbv1alpha1.AnnotationSettings) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "ingress")
	log.V(2).Info("creating ingress", "hostname", hostname, "service", svcName)

	// Create Ingress resource
	ingressName := fmt.Sprintf("%s-ingress", loadBalancer.Name)
	namespace := loadBalancer.Namespace

	// Determine ingress class
	var ingressClass *string
	if tenant.Spec.Ingress.Class != nil {
		ingressClass = tenant.Spec.Ingress.Class
	} else if config.Spec.Ingress.Class != nil {
		ingressClass = config.Spec.Ingress.Class
	}

	// Create Ingress
	pathTypePrefix := networkingv1.PathTypePrefix
	port := loadBalancer.Spec.Ports[0].Port

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: namespace,
			Labels: map[string]string{
				kubelb.LabelLoadBalancerName:      loadBalancer.Name,
				kubelb.LabelLoadBalancerNamespace: loadBalancer.Namespace,
			},
			Annotations: make(map[string]string),
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ingressClass,
			Rules: []networkingv1.IngressRule{
				{
					Host: hostname,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathTypePrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: svcName,
											Port: networkingv1.ServiceBackendPort{
												Number: port,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{hostname},
					SecretName: fmt.Sprintf("%s-tls", ingressName),
				},
			},
		},
	}

	ingress.Annotations = kubelb.PropagateAnnotations(loadBalancer.Annotations, annotations, kubelbv1alpha1.AnnotatedResourceIngress)
	kubelb.AddDNSAndCertificateAnnotations(ingress.Annotations, tenant, config, hostname, false)

	// Set controller reference to LoadBalancer so Ingress gets auto-deleted when LoadBalancer is deleted
	if err := ctrl.SetControllerReference(loadBalancer, ingress, client.Scheme()); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Check if Ingress already exists
	existingIngress := &networkingv1.Ingress{}
	err := client.Get(ctx, types.NamespacedName{Name: ingressName, Namespace: namespace}, existingIngress)

	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get ingress: %w", err)
	}

	if kerrors.IsNotFound(err) {
		// Create new Ingress
		if err := client.Create(ctx, ingress); err != nil {
			return fmt.Errorf("failed to create ingress: %w", err)
		}
		log.V(2).Info("created ingress", "name", ingressName)
	} else {
		// Merge the annotations with the existing annotations to allow annotations that are configured by third party controllers on the existing service to be preserved.
		ingress.Annotations = k8sutils.MergeAnnotations(existingIngress.Annotations, ingress.Annotations)

		// Ingress already exists, we need to check if it needs to be updated.
		if !equality.Semantic.DeepEqual(existingIngress.Spec, ingress.Spec) ||
			!equality.Semantic.DeepEqual(existingIngress.Labels, ingress.Labels) ||
			!k8sutils.CompareAnnotations(existingIngress.Annotations, ingress.Annotations) {
			// Update existing Ingress if needed
			existingIngress.Spec = ingress.Spec
			existingIngress.Labels = ingress.Labels
			existingIngress.Annotations = ingress.Annotations
			if err := client.Update(ctx, existingIngress); err != nil {
				return fmt.Errorf("failed to update ingress: %w", err)
			}
			log.V(2).Info("updated ingress", "name", ingressName)
		}
	}
	return nil
}
