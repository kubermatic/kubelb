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

package kubelb

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/config"
	"k8c.io/kubelb/internal/kubelb"
	portlookup "k8c.io/kubelb/internal/port-lookup"
	serviceHelpers "k8c.io/kubelb/internal/resources/service"
	"k8c.io/kubelb/internal/resources/unstructured"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	RouteControllerName = "route-controller"
	CleanupFinalizer    = "kubelb.k8c.io/cleanup"
)

// RouteReconciler reconciles a Route Object
type RouteReconciler struct {
	ctrlclient.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	Namespace          string
	PortAllocator      *portlookup.PortAllocator
	EnvoyProxyTopology EnvoyProxyTopology
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=grpcroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=grpcroutes/status,verbs=get;update;patch

func (r *RouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)

	log.Info("Reconciling")

	resource := &kubelbv1alpha1.Route{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Resource is marked for deletion
	if resource.DeletionTimestamp != nil {
		if controllerutil.ContainsFinalizer(resource, CleanupFinalizer) {
			return r.cleanup(ctx, resource)
		}
		// Finalizer doesn't exist so clean up is already done
		return reconcile.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(resource, CleanupFinalizer) {
		if ok := controllerutil.AddFinalizer(resource, CleanupFinalizer); !ok {
			log.Error(nil, "Failed to add finalizer for the Route")
			return ctrl.Result{Requeue: true}, nil
		}
		if err := r.Update(ctx, resource); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	err := r.reconcile(ctx, log, resource)
	if err != nil {
		log.Error(err, "reconciling failed")
	}

	return reconcile.Result{}, err
}

func (r *RouteReconciler) reconcile(ctx context.Context, log logr.Logger, route *kubelbv1alpha1.Route) error {
	resourceNamespace := route.Namespace
	if config.IsGlobalTopology() {
		resourceNamespace = r.Namespace
	}

	// Create or update services based on the route.
	err := r.manageServices(ctx, log, route, resourceNamespace)
	if err != nil {
		return fmt.Errorf("failed to create or update services: %w", err)
	}

	// Create or update the route object.
	err = r.manageRoutes(ctx, log, route, resourceNamespace)
	if err != nil {
		return fmt.Errorf("failed to create or update route: %w", err)
	}

	return nil
}

func (r *RouteReconciler) cleanup(ctx context.Context, route *kubelbv1alpha1.Route) (ctrl.Result, error) {
	// Route will be removed automatically because of owner reference. We need to take care of removing
	// the services while ensuring that the services are not being used by other routes.

	if route.Status.Resources.Services == nil {
		return reconcile.Result{}, nil
	}

	ns := route.Namespace
	if config.IsGlobalTopology() {
		ns = r.Namespace
	}

	for _, value := range route.Status.Resources.Services {
		log := r.Log.WithValues("name", value.Name, "namespace", value.Namespace)
		log.V(1).Info("Deleting service", "name", value.GeneratedName, "namespace", ns)
		svc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      value.GeneratedName,
				Namespace: ns,
			},
		}
		if err := r.Client.Delete(ctx, &svc); err != nil {
			if !kerrors.IsNotFound(err) {
				return reconcile.Result{}, fmt.Errorf("failed to delete service: %w", err)
			}
		}
	}

	// De-allocate the ports allocated for the services.
	if err := r.PortAllocator.DeallocatePortsForRoute(*route); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to deallocate ports: %w", err)
	}

	controllerutil.RemoveFinalizer(route, CleanupFinalizer)
	if err := r.Update(ctx, route); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *RouteReconciler) manageServices(ctx context.Context, log logr.Logger, route *kubelbv1alpha1.Route, resourceNamespace string) error {
	if route.Spec.Source.Kubernetes == nil {
		return nil
	}

	// Before creating/updating services, ensure that the orphaned services are cleaned up.
	err := r.cleanupOrphanedServices(ctx, log, route, resourceNamespace)
	if err != nil {
		return fmt.Errorf("failed to cleanup orphaned services: %w", err)
	}

	// Allocate ports for the services. These ports are then used as the target ports for the services.
	if err := r.PortAllocator.AllocatePortsForRoutes([]kubelbv1alpha1.Route{*route}); err != nil {
		return err
	}

	appName := envoyApplicationName(r.EnvoyProxyTopology, route.Namespace)
	services := []corev1.Service{}
	for _, service := range route.Spec.Source.Kubernetes.Services {
		// Transform the service into desired state.
		svc := serviceHelpers.GenerateServiceForLBCluster(service.Service, appName, route.Namespace, resourceNamespace, r.PortAllocator, r.EnvoyProxyTopology.IsGlobalTopology())
		services = append(services, svc)
	}

	routeStatus := route.Status.DeepCopy()
	for _, svc := range services {
		log.V(4).Info("Creating/Updating service", "name", svc.Name, "namespace", svc.Namespace)
		var err error
		if err = serviceHelpers.CreateOrUpdateService(ctx, r.Client, &svc); err != nil {
			// We only log the error and set the condition to false. The error will be set in the status.
			log.Error(err, "failed to create or update Service", "name", svc.Name, "namespace", svc.Namespace)
			errorMessage := fmt.Errorf("failed to create or update Service: %w", err)
			r.Recorder.Eventf(route, corev1.EventTypeWarning, "ServiceApplyFailed", errorMessage.Error())
		}
		updateServiceStatus(routeStatus, &svc, err)
	}
	return r.UpdateRouteStatus(ctx, route, *routeStatus)
}

func (r *RouteReconciler) cleanupOrphanedServices(ctx context.Context, log logr.Logger, route *kubelbv1alpha1.Route, resourceNamespace string) error {
	// Get all the services based on route.
	desiredServices := map[string]bool{}
	for _, service := range route.Spec.Source.Kubernetes.Services {
		name := serviceHelpers.GetServiceName(service.Service)
		key := fmt.Sprintf(kubelb.RouteServiceMapKey, service.Service.Namespace, name)
		desiredServices[key] = true
	}

	if route.Status.Resources.Services == nil {
		return nil
	}

	for key, value := range route.Status.Resources.Services {
		if _, ok := desiredServices[key]; !ok {
			// Service is not desired, so delete it.
			log.V(4).Info("Deleting orphaned service", "name", value.GeneratedName, "namespace", resourceNamespace)
			svc := corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      value.GeneratedName,
					Namespace: resourceNamespace,
				},
			}
			if err := r.Client.Delete(ctx, &svc); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to delete orphaned service: %w", err)
				}
			}
			delete(route.Status.Resources.Services, key)

			endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointRoutePattern, route.Namespace, value.Namespace, svc.Name)
			// De-allocate the ports allocated for the service.
			r.PortAllocator.DeallocateEndpoints([]string{endpointKey})
		}
	}
	return nil
}

func (r *RouteReconciler) UpdateRouteStatus(ctx context.Context, route *kubelbv1alpha1.Route, status kubelbv1alpha1.RouteStatus) error {
	key := ctrlclient.ObjectKeyFromObject(route)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Fetch the current state
		if err := r.Client.Get(ctx, key, route); err != nil {
			return err
		}

		// Update the status
		original := route.DeepCopy()
		route.Status = status

		// If the status has not changed, no need to update.
		if reflect.DeepEqual(original.Status, route.Status) {
			return nil
		}

		// Update the route
		return r.Client.Status().Patch(ctx, route, ctrlruntimeclient.MergeFrom(original))
	})
}

func (r *RouteReconciler) manageRoutes(ctx context.Context, log logr.Logger, route *kubelbv1alpha1.Route, resourceNamespace string) error {
	if route.Spec.Source.Kubernetes == nil {
		return nil
	}

	resource, err := unstructured.ConvertUnstructuredToObject(&route.Spec.Source.Kubernetes.Route)
	if err != nil {
		return fmt.Errorf("failed to convert route to object: %w", err)
	}

	ownerReference := metav1.OwnerReference{
		APIVersion: route.APIVersion,
		Kind:       route.Kind,
		Name:       route.Name,
		UID:        route.UID,
	}

	// Set owner reference for the resource.
	resource.SetOwnerReferences([]metav1.OwnerReference{ownerReference})

	// Get the services referenced by the route.
	var referencedServices []metav1.ObjectMeta
	for _, service := range route.Spec.Source.Kubernetes.Services {
		name := serviceHelpers.GetServiceName(service.Service)
		objectMeta := metav1.ObjectMeta{
			Name:      name,
			Namespace: service.Service.Namespace,
			UID:       service.Service.UID,
		}
		referencedServices = append(referencedServices, objectMeta)
	}

	routeStatus := route.Status.DeepCopy()

	// Determine the type of the resource and call the appropriate method
	switch v := resource.(type) {
	case *v1.Ingress: // v1 "k8s.io/api/networking/v1"
		err = r.createOrUpdateIngress(ctx, log, v, referencedServices, resourceNamespace)
		if err == nil {
			// Retrieve updated object to get the status.
			key := client.ObjectKey{Namespace: v.Namespace, Name: v.Name}
			res := &v1.Ingress{}
			if err := r.Client.Get(ctx, key, res); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to get Ingress: %w", err)
				}
			}
			updateResourceStatus(routeStatus, res, err)
		}

	case *gwapiv1.Gateway: // v1 "sigs.k8s.io/gateway-api/apis/v1"
		err = r.createOrUpdateGateway(ctx, log, v, resourceNamespace)
		if err == nil {
			// Retrieve updated object to get the status.
			key := client.ObjectKey{Namespace: v.Namespace, Name: v.Name}
			res := &gwapiv1.Gateway{}
			if err := r.Client.Get(ctx, key, res); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to get Gateway: %w", err)
				}
			}
			updateResourceStatus(routeStatus, res, err)
		}

	case *gwapiv1.HTTPRoute: // v1 "sigs.k8s.io/gateway-api/apis/v1"
		err = r.createOrUpdateHTTPRoute(ctx, log, v, referencedServices, resourceNamespace)
		if err == nil {
			// Retrieve updated object to get the status.
			key := client.ObjectKey{Namespace: v.Namespace, Name: v.Name}
			res := &gwapiv1.HTTPRoute{}
			if err := r.Client.Get(ctx, key, res); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to get HTTPRoute: %w", err)
				}
			}
			updateResourceStatus(routeStatus, res, err)
		}

	case *gwapiv1.GRPCRoute: // v1 "sigs.k8s.io/gateway-api/apis/v1"
		err = r.createOrUpdateGRPCRoute(ctx, log, v, referencedServices, resourceNamespace)
		if err == nil {
			// Retrieve updated object to get the status.
			key := client.ObjectKey{Namespace: v.Namespace, Name: v.Name}
			res := &gwapiv1.GRPCRoute{}
			if err := r.Client.Get(ctx, key, res); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to get GRPCRoute: %w", err)
				}
			}
			updateResourceStatus(routeStatus, res, err)
		}

	default:
		log.V(4).Info("Unsupported resource type")
	}

	return r.UpdateRouteStatus(ctx, route, *routeStatus)
}

func (r *RouteReconciler) createOrUpdateGateway(ctx context.Context, log logr.Logger, gateway *gwapiv1.Gateway, namespace string) error {
	// Check if Gateway with the same name but different namespace already exists. If it does, log an error as we don't support
	// multiple Gateway objects.
	gateways := &gwapiv1.GatewayList{}
	if err := r.Client.List(ctx, gateways, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("failed to list Gateways: %w", err)
	}

	found := false
	for _, existingGateway := range gateways.Items {
		if existingGateway.Name == gateway.Name {
			found = true
			break
		}
	}

	if !found && len(gateways.Items) >= 1 {
		return fmt.Errorf("multiple Gateway objects are not supported")
	}

	// Create/Update the Gateway object in the cluster.
	gateway.Namespace = namespace
	gateway.SetUID("") // Reset UID to generate a new UID for the Gateway object

	// Set the GatewayClassName if it is specified in the configuration.
	if config.GetConfig().Spec.GatewayClassName != nil {
		gateway.Spec.GatewayClassName = gwapiv1.ObjectName(*config.GetConfig().Spec.GatewayClassName)
	}

	log.V(4).Info("Creating/Updating Gateway", "name", gateway.Name, "namespace", gateway.Namespace)
	// Check if it already exists.
	gatewayKey := client.ObjectKey{Namespace: gateway.Namespace, Name: gateway.Name}
	existingGateway := &gwapiv1.Gateway{}
	if err := r.Client.Get(ctx, gatewayKey, existingGateway); err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get Gateway: %w", err)
		}
		err := r.Client.Create(ctx, gateway)
		if err != nil {
			return fmt.Errorf("failed to create Gateway: %w", err)
		}
		return nil
	}

	// Update the Gateway object if it is different from the existing one.
	if equality.Semantic.DeepEqual(existingGateway.Spec, gateway.Spec) &&
		equality.Semantic.DeepEqual(existingGateway.Labels, gateway.Labels) &&
		equality.Semantic.DeepEqual(existingGateway.Annotations, gateway.Annotations) {
		return nil
	}

	// Required to update the object.
	gateway.ResourceVersion = existingGateway.ResourceVersion
	gateway.UID = existingGateway.UID

	if err := r.Client.Update(ctx, gateway); err != nil {
		return fmt.Errorf("failed to update Gateway: %w", err)
	}
	return nil
}

// createOrUpdateHTTPRoute creates or updates the HTTPRoute object in the cluster.
func (r *RouteReconciler) createOrUpdateHTTPRoute(ctx context.Context, log logr.Logger, object *gwapiv1.HTTPRoute, referencedServices []metav1.ObjectMeta, namespace string) error {
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
							object.Spec.Rules[i].Filters[j].RequestMirror.BackendRef.Name = gwapiv1.ObjectName(kubelb.GenerateName(r.EnvoyProxyTopology.IsGlobalTopology(), string(service.UID), service.Name, service.Namespace))
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
							object.Spec.Rules[i].BackendRefs[j].Name = gwapiv1.ObjectName(kubelb.GenerateName(r.EnvoyProxyTopology.IsGlobalTopology(), string(service.UID), service.Name, service.Namespace))
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
									object.Spec.Rules[i].Filters[j].RequestMirror.BackendRef.Name = gwapiv1.ObjectName(kubelb.GenerateName(r.EnvoyProxyTopology.IsGlobalTopology(), string(service.UID), service.Name, service.Namespace))
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

	object.Name = kubelb.GenerateName(false, string(object.UID), object.Name, object.Namespace)
	object.Namespace = namespace
	object.SetUID("") // Reset UID to generate a new UID for the object

	log.V(4).Info("Creating/Updating HTTPRoute", "name", object.Name, "namespace", object.Namespace)
	// Check if it already exists.
	key := client.ObjectKey{Namespace: object.Namespace, Name: object.Name}
	existingObject := &gwapiv1.HTTPRoute{}
	if err := r.Client.Get(ctx, key, existingObject); err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get HTTPRoute: %w", err)
		}
		err := r.Client.Create(ctx, object)
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

	if err := r.Client.Update(ctx, object); err != nil {
		return fmt.Errorf("failed to update HTTPRoute: %w", err)
	}
	return nil
}

// createOrUpdateGRPCRoute creates or updates the GRPCRoute object in the cluster.
func (r *RouteReconciler) createOrUpdateGRPCRoute(ctx context.Context, log logr.Logger, object *gwapiv1.GRPCRoute, referencedServices []metav1.ObjectMeta, namespace string) error {
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
							object.Spec.Rules[i].Filters[j].RequestMirror.BackendRef.Name = gwapiv1.ObjectName(kubelb.GenerateName(r.EnvoyProxyTopology.IsGlobalTopology(), string(service.UID), service.Name, service.Namespace))
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
							object.Spec.Rules[i].BackendRefs[j].Name = gwapiv1.ObjectName(kubelb.GenerateName(r.EnvoyProxyTopology.IsGlobalTopology(), string(service.UID), service.Name, service.Namespace))
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
									object.Spec.Rules[i].Filters[j].RequestMirror.BackendRef.Name = gwapiv1.ObjectName(kubelb.GenerateName(r.EnvoyProxyTopology.IsGlobalTopology(), string(service.UID), service.Name, service.Namespace))
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

	object.Name = kubelb.GenerateName(false, string(object.UID), object.Name, object.Namespace)
	object.Namespace = namespace
	object.SetUID("") // Reset UID to generate a new UID for the object

	log.V(4).Info("Creating/Updating GRPCRoute", "name", object.Name, "namespace", object.Namespace)
	// Check if it already exists.
	key := client.ObjectKey{Namespace: object.Namespace, Name: object.Name}
	existingObject := &gwapiv1.GRPCRoute{}
	if err := r.Client.Get(ctx, key, existingObject); err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get GRPCRoute: %w", err)
		}
		err := r.Client.Create(ctx, object)
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

	if err := r.Client.Update(ctx, object); err != nil {
		return fmt.Errorf("failed to update GRPCRoute: %w", err)
	}
	return nil
}

// createOrUpdateIngress creates or updates the Ingress object in the cluster.
func (r *RouteReconciler) createOrUpdateIngress(ctx context.Context, log logr.Logger, object *v1.Ingress, referencedServices []metav1.ObjectMeta, namespace string) error {
	// Name of the services referenced by the Ingress have to be updated to match the services created against the Route in the LB cluster.
	for i, rule := range object.Spec.Rules {
		for j, path := range rule.HTTP.Paths {
			for _, service := range referencedServices {
				if path.Backend.Service.Name == service.Name {
					object.Spec.Rules[i].HTTP.Paths[j].Backend.Service.Name = kubelb.GenerateName(r.EnvoyProxyTopology.IsGlobalTopology(), string(service.UID), service.Name, service.Namespace)
				}
			}
		}
	}

	if object.Spec.DefaultBackend != nil && object.Spec.DefaultBackend.Service != nil {
		for _, service := range referencedServices {
			if object.Spec.DefaultBackend.Service.Name == service.Name {
				object.Spec.DefaultBackend.Service.Name = kubelb.GenerateName(r.EnvoyProxyTopology.IsGlobalTopology(), string(service.UID), service.Name, service.Namespace)
			}
		}
	}

	object.Spec.IngressClassName = config.GetConfig().Spec.IngressClassName
	object.Name = kubelb.GenerateName(r.EnvoyProxyTopology.IsGlobalTopology(), string(object.UID), object.Name, object.Namespace)
	object.Namespace = namespace
	object.SetUID("") // Reset UID to generate a new UID for the object

	log.V(4).Info("Creating/Updating Ingress", "name", object.Name, "namespace", object.Namespace)
	// Check if it already exists.
	key := client.ObjectKey{Namespace: object.Namespace, Name: object.Name}
	existingObject := &v1.Ingress{}
	if err := r.Client.Get(ctx, key, existingObject); err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get Ingress: %w", err)
		}
		err := r.Client.Create(ctx, object)
		if err != nil {
			return fmt.Errorf("failed to create Ingress: %w", err)
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

	if err := r.Client.Update(ctx, object); err != nil {
		return fmt.Errorf("failed to update Ingress: %w", err)
	}
	return nil
}

func updateServiceStatus(routeStatus *kubelbv1alpha1.RouteStatus, svc *corev1.Service, err error) {
	originalName := serviceHelpers.GetServiceName(*svc)
	originalNamespace := serviceHelpers.GetServiceNamespace(*svc)
	status := kubelbv1alpha1.RouteServiceStatus{
		ResourceState: kubelbv1alpha1.ResourceState{
			GeneratedName: svc.GetName(),
			Namespace:     originalNamespace,
			Name:          originalName,
		},
		Ports: svc.Spec.Ports,
	}
	status.Conditions = generateConditions(err)

	svcStatus, err := json.Marshal(svc.Status)
	if err != nil {
		// If we are unable to marshal the status, we set it to empty object. There is no need to fail the reconciliation.
		svcStatus = []byte(kubelb.DefaultRouteStatus)
	}

	status.Status = runtime.RawExtension{
		Raw: svcStatus,
	}
	if routeStatus.Resources.Services == nil {
		routeStatus.Resources.Services = make(map[string]kubelbv1alpha1.RouteServiceStatus)
	}
	key := fmt.Sprintf(kubelb.RouteServiceMapKey, originalNamespace, originalName)
	routeStatus.Resources.Services[key] = status
}

func updateResourceStatus(routeStatus *kubelbv1alpha1.RouteStatus, obj client.Object, err error) {
	status := kubelbv1alpha1.ResourceState{
		GeneratedName: obj.GetName(),
		Namespace:     kubelb.GetNamespace(obj),
		Name:          kubelb.GetName(obj),
		APIVersion:    obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:          obj.GetObjectKind().GroupVersionKind().Kind,
	}
	status.Conditions = generateConditions(err)

	switch resource := obj.(type) {
	case *v1.Ingress:
		status.Status = getResourceStatus(resource.Status)
		routeStatus.Resources.Route = status
	case *gwapiv1.Gateway:
		status.Status = getResourceStatus(resource.Status)
		routeStatus.Resources.Route = status
	case *gwapiv1.HTTPRoute:
		status.Status = getResourceStatus(resource.Status)
		routeStatus.Resources.Route = status
	case *gwapiv1.GRPCRoute:
		status.Status = getResourceStatus(resource.Status)
		routeStatus.Resources.Route = status
	}
}

func getResourceStatus(v any) runtime.RawExtension {
	resourceStatus, err := json.Marshal(v)
	if err != nil {
		// If we are unable to marshal the status, we set it to empty object. There is no need to fail the reconciliation.
		resourceStatus = []byte(kubelb.DefaultRouteStatus)
	}
	return runtime.RawExtension{
		Raw: resourceStatus,
	}
}

func generateConditions(err error) []metav1.Condition {
	conditionMessage := "Success"
	conditionStatus := metav1.ConditionTrue
	conditionReason := "InstallationSuccessful"
	if err != nil {
		conditionMessage = err.Error()
		conditionStatus = metav1.ConditionFalse
		conditionReason = "InstallationFailed"
	}
	return []metav1.Condition{
		{
			Type:   kubelbv1alpha1.ConditionResourceAppliedSuccessfully.String(),
			Reason: conditionReason,
			Status: conditionStatus,
			LastTransitionTime: metav1.Time{
				Time: time.Now(),
			},
			Message: conditionMessage,
		},
	}
}

func envoyApplicationName(topology EnvoyProxyTopology, namespace string) string {
	switch topology {
	case EnvoyProxyTopologyShared:
		return namespace
	case EnvoyProxyTopologyGlobal:
		return EnvoyGlobalCache
	}
	return ""
}

func (r *RouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// 1. Watch for changes in Route object.
	// 2. Skip reconciliation if generation is not changed; only status/metadata changed.
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubelbv1alpha1.Route{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
