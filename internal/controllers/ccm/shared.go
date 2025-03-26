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

package ccm

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	"k8c.io/kubelb/internal/resources/route"
	serviceHelpers "k8c.io/kubelb/internal/resources/service"
	"k8c.io/kubelb/internal/resources/unstructured"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const (
	CleanupFinalizer = "kubelb.k8c.io/cleanup"
)

func reconcileSourceForRoute(ctx context.Context, log logr.Logger, client ctrlclient.Client, lbClient ctrlclient.Client, resource ctrlclient.Object, originalServices []types.NamespacedName, referenceGrants []gwapiv1alpha2.ReferenceGrant, namespace string) error {
	log.V(2).Info("reconciling source for producing route")

	unstructuredResource, err := unstructured.ConvertObjectToUnstructured(resource)
	if err != nil {
		return fmt.Errorf("failed to convert Ingress to unstructured: %w", err)
	}
	unstructuredResource = unstructured.NormalizeUnstructured(unstructuredResource)

	services, err := serviceHelpers.NormalizeAndReplicateServices(ctx, log, client, originalServices)
	if err != nil {
		return fmt.Errorf("failed to normalize services: %w", err)
	}

	routeSubresources := route.Subresources{
		Services:        services,
		ReferenceGrants: referenceGrants,
	}

	err = route.CreateRouteForResource(ctx, log, lbClient, *unstructuredResource, routeSubresources, namespace)
	if err != nil {
		return fmt.Errorf("failed to create route for resource: %w", err)
	}
	return nil
}

func cleanupRoute(ctx context.Context, client ctrlclient.Client, resourceUID string, namespace string) error {
	// Find the Route in LB cluster and delete it
	route := kubelbv1alpha1.Route{}
	err := client.Get(ctx, types.NamespacedName{Name: resourceUID, Namespace: namespace}, &route)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get Route from LB cluster: %w", err)
		}
	} else {
		err = client.Delete(ctx, &route)
		if err != nil {
			return fmt.Errorf("failed to delete Route: %w", err)
		}
	}
	return nil
}

func enqueueRoutes(gvk, clusterNamespace string) handler.TypedMapFunc[*kubelbv1alpha1.Route, reconcile.Request] {
	return handler.TypedMapFunc[*kubelbv1alpha1.Route, reconcile.Request](func(_ context.Context, route *kubelbv1alpha1.Route) []reconcile.Request {
		if route.GetNamespace() != clusterNamespace {
			return []reconcile.Request{}
		}

		originalNamespace, ok := route.GetLabels()[kubelb.LabelOriginNamespace]
		if !ok || originalNamespace == "" {
			// Can't process further
			return []reconcile.Request{}
		}

		originalName, ok := route.GetLabels()[kubelb.LabelOriginName]
		if !ok || originalName == "" {
			// Can't process further
			return []reconcile.Request{}
		}

		resourceGVK, ok := route.GetLabels()[kubelb.LabelOriginResourceKind]
		if !ok || originalName == "" {
			// Can't process further
			return []reconcile.Request{}
		}

		if gvk != resourceGVK {
			// Can't process further
			return []reconcile.Request{}
		}

		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      originalName,
					Namespace: originalNamespace,
				},
			},
		}
	})
}
