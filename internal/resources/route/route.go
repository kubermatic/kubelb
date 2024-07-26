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
	"reflect"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type Subresources struct {
	Services        []corev1.Service               `json:"services,omitempty"`
	ReferenceGrants []gwapiv1alpha2.ReferenceGrant `json:"referenceGrants,omitempty"`
}

func CreateRouteForResource(ctx context.Context, _ logr.Logger, client ctrlclient.Client, resource unstructured.Unstructured, subresources Subresources, namespace string) error {
	generateRoute := GenerateRoute(resource, subresources, namespace)
	return CreateUpdateRoute(ctx, client, generateRoute)
}

func GenerateRoute(resource unstructured.Unstructured, resources Subresources, namespace string) kubelbv1alpha1.Route {
	return kubelbv1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(resource.GetUID()),
			Namespace: namespace,
			Labels: map[string]string{
				kubelb.LabelOriginNamespace:    resource.GetNamespace(),
				kubelb.LabelOriginName:         resource.GetName(),
				kubelb.LabelOriginResourceKind: resource.GetObjectKind().GroupVersionKind().GroupKind().String(),
				kubelb.LabelManagedBy:          kubelb.LabelControllerName,
			},
		},
		Spec: kubelbv1alpha1.RouteSpec{
			// TODO(waleed): Once we have everything in place, figure out how this should look like.
			Endpoints: []kubelbv1alpha1.LoadBalancerEndpoints{
				{
					Name: "default",
					AddressesReference: &corev1.ObjectReference{
						Name: kubelbv1alpha1.DefaultAddressName,
					},
				},
			},
			Source: kubelbv1alpha1.RouteSource{
				Kubernetes: &kubelbv1alpha1.KubernetesSource{
					Route:           resource,
					Services:        kubelbv1alpha1.ConvertServicesToUpstreamServices(resources.Services),
					ReferenceGrants: kubelbv1alpha1.ConvertReferenceGrantsToUpstreamReferenceGrants(resources.ReferenceGrants),
				},
			},
		},
	}
}

func CreateUpdateRoute(ctx context.Context, client ctrlclient.Client, route kubelbv1alpha1.Route) error {
	existingRoute := kubelbv1alpha1.Route{}
	err := client.Get(ctx, types.NamespacedName{Name: route.Name, Namespace: route.Namespace}, &existingRoute)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get Route from LB cluster: %w", err)
	}

	if errors.IsNotFound(err) {
		existingRoute = route // If the Route doesn't exist, create it
		err = client.Create(ctx, &existingRoute)
		if err != nil {
			return fmt.Errorf("failed to create Route: %w", err)
		}
		return nil
	}

	if !reflect.DeepEqual(existingRoute.Spec, route.Spec) || !reflect.DeepEqual(existingRoute.Labels, route.Labels) || !reflect.DeepEqual(existingRoute.Annotations, route.Annotations) {
		existingRoute.Spec = route.Spec
		existingRoute.Labels = route.Labels
		existingRoute.Annotations = route.Annotations
		err = client.Update(ctx, &existingRoute)
		if err != nil {
			return fmt.Errorf("failed to update Route: %w", err)
		}
	}
	return nil
}
