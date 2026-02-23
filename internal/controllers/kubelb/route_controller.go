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

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	"k8c.io/kubelb/internal/metricsutil"
	managermetrics "k8c.io/kubelb/internal/metricsutil/manager"
	portlookup "k8c.io/kubelb/internal/port-lookup"
	gatewayHelpers "k8c.io/kubelb/internal/resources/gatewayapi/gateway"
	grpcrouteHelpers "k8c.io/kubelb/internal/resources/gatewayapi/grpcroute"
	httprouteHelpers "k8c.io/kubelb/internal/resources/gatewayapi/httproute"
	ingressHelpers "k8c.io/kubelb/internal/resources/ingress"
	serviceHelpers "k8c.io/kubelb/internal/resources/service"
	"k8c.io/kubelb/internal/resources/unstructured"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
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
	ctrlruntimeclient.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder

	Namespace         string
	PortAllocator     *portlookup.PortAllocator
	DisableGatewayAPI bool
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
	startTime := time.Now()

	// Determine route type for metrics - will be updated once we fetch the resource
	routeType := metricsutil.RouteTypeIngress

	// Track reconciliation duration
	defer func() {
		managermetrics.RouteReconcileDuration.WithLabelValues(req.Namespace).Observe(time.Since(startTime).Seconds())
	}()

	log.Info("Reconciling")

	resource := &kubelbv1alpha1.Route{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		managermetrics.RouteReconcileTotal.WithLabelValues(req.Namespace, routeType, metricsutil.ResultError).Inc()
		return reconcile.Result{}, err
	}

	// Determine route type from source
	routeType = getRouteType(resource)

	// Before proceeding further we need to make sure that the resource is reconcilable.
	tenant, config, err := GetTenantAndConfig(ctx, r.Client, r.Namespace, RemoveTenantPrefix(resource.Namespace))
	if err != nil {
		log.Error(err, "unable to fetch Tenant and Config, cannot proceed")
		managermetrics.RouteReconcileTotal.WithLabelValues(req.Namespace, routeType, metricsutil.ResultError).Inc()
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

	shouldReconcile, disabled, err := r.shouldReconcile(ctx, resource, tenant, config)
	if err != nil {
		log.Error(err, "unable to determine if the Route should be reconciled")
		managermetrics.RouteReconcileTotal.WithLabelValues(req.Namespace, routeType, metricsutil.ResultError).Inc()
		return reconcile.Result{}, err
	}

	// If the resource is disabled, we need to clean up the resources
	if controllerutil.ContainsFinalizer(resource, CleanupFinalizer) && disabled {
		return r.cleanup(ctx, resource)
	}

	if !shouldReconcile {
		managermetrics.RouteReconcileTotal.WithLabelValues(req.Namespace, routeType, metricsutil.ResultSkipped).Inc()
		return reconcile.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(resource, CleanupFinalizer) {
		if ok := controllerutil.AddFinalizer(resource, CleanupFinalizer); !ok {
			log.Error(nil, "Failed to add finalizer for the Route")
			return ctrl.Result{Requeue: true}, nil
		}
		if err := r.Update(ctx, resource); err != nil {
			managermetrics.RouteReconcileTotal.WithLabelValues(req.Namespace, routeType, metricsutil.ResultError).Inc()
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	err = r.reconcile(ctx, log, resource, config, tenant)
	if err != nil {
		log.Error(err, "reconciling failed")
		managermetrics.RouteReconcileTotal.WithLabelValues(req.Namespace, routeType, metricsutil.ResultError).Inc()
		return reconcile.Result{}, err
	}

	// Update route count gauge per namespace/type
	routeList := &kubelbv1alpha1.RouteList{}
	if listErr := r.List(ctx, routeList, ctrlruntimeclient.InNamespace(req.Namespace)); listErr == nil {
		routesByType := map[string]int{}
		for _, rt := range routeList.Items {
			t := getRouteType(&rt)
			routesByType[t]++
		}
		for t, count := range routesByType {
			managermetrics.RoutesTotal.WithLabelValues(req.Namespace, RemoveTenantPrefix(req.Namespace), t).Set(float64(count))
		}
	}

	managermetrics.RouteReconcileTotal.WithLabelValues(req.Namespace, routeType, metricsutil.ResultSuccess).Inc()
	return reconcile.Result{}, nil
}

func (r *RouteReconciler) reconcile(ctx context.Context, log logr.Logger, route *kubelbv1alpha1.Route, config *kubelbv1alpha1.Config, tenant *kubelbv1alpha1.Tenant) error {
	annotations := GetAnnotations(tenant, config)

	// Create or update services based on the route.
	err := r.manageServices(ctx, log, route, annotations)
	if err != nil {
		return fmt.Errorf("failed to create or update services: %w", err)
	}

	// Create or update the route object.
	err = r.manageRoutes(ctx, log, route, config, tenant, annotations)
	if err != nil {
		return fmt.Errorf("failed to create or update route: %w", err)
	}

	return nil
}

func (r *RouteReconciler) cleanup(ctx context.Context, route *kubelbv1alpha1.Route) (ctrl.Result, error) {
	// Route will be removed automatically because of owner reference. We need to take care of removing
	// the services while ensuring that the services are not being used by other routes.
	for _, value := range route.Status.Resources.Services {
		log := r.Log.WithValues("name", value.Name, "namespace", value.Namespace)
		svc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      value.GeneratedName,
				Namespace: route.Namespace,
			},
		}

		log.V(1).Info("Deleting service", "name", value.GeneratedName, "namespace", value.Namespace)

		if err := r.Delete(ctx, &svc); err != nil {
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

func (r *RouteReconciler) manageServices(ctx context.Context, log logr.Logger, route *kubelbv1alpha1.Route, annotations kubelbv1alpha1.AnnotationSettings) error {
	if route.Spec.Source.Kubernetes == nil {
		return nil
	}

	// Before creating/updating services, ensure that the orphaned services are cleaned up.
	err := r.cleanupOrphanedServices(ctx, log, route)
	if err != nil {
		return fmt.Errorf("failed to cleanup orphaned services: %w", err)
	}

	// Allocate ports for the services. These ports are then used as the target ports for the services.
	if err := r.PortAllocator.AllocatePortsForRoutes([]kubelbv1alpha1.Route{*route}); err != nil {
		return err
	}

	// Get original route name for port allocation key consistency
	originalRouteName := route.Name
	if route.Spec.Source.Kubernetes.Route.GetName() != "" {
		originalRouteName = route.Spec.Source.Kubernetes.Route.GetName()
	} else if name := route.GetLabels()[kubelb.LabelOriginName]; name != "" {
		originalRouteName = name
	}

	appName := route.Namespace
	services := []corev1.Service{}
	for _, service := range route.Spec.Source.Kubernetes.Services {
		// Transform the service into desired state.
		svc := serviceHelpers.GenerateServiceForLBCluster(service.Service, appName, route.Namespace, originalRouteName, r.PortAllocator, annotations)
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
			r.Recorder.Eventf(route, nil, corev1.EventTypeWarning, "ServiceApplyFailed", "Reconciling", errorMessage.Error())
		}
		updateServiceStatus(routeStatus, &svc, err)
	}

	// Cleanup orphaned services from naming change (old: ns-svc, new: ns-route-svc)
	r.cleanupRenamedServices(ctx, log, route, originalRouteName)

	return r.UpdateRouteStatus(ctx, route, *routeStatus)
}

func (r *RouteReconciler) cleanupOrphanedServices(ctx context.Context, log logr.Logger, route *kubelbv1alpha1.Route) error {
	// Get all the services based on route.
	desiredServices := map[string]bool{}
	for _, service := range route.Spec.Source.Kubernetes.Services {
		name := serviceHelpers.GetServiceName(service.Service)
		key := fmt.Sprintf(kubelb.RouteServiceMapKey, service.Namespace, name)
		desiredServices[key] = true
	}

	if route.Status.Resources.Services == nil {
		return nil
	}

	for key, value := range route.Status.Resources.Services {
		if _, ok := desiredServices[key]; !ok {
			// Service is not desired, so delete it.
			log.V(4).Info("Deleting orphaned service", "name", value.GeneratedName, "namespace", route.Namespace)
			svc := corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      value.GeneratedName,
					Namespace: route.Namespace,
				},
			}
			if err := r.Delete(ctx, &svc); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to delete orphaned service: %w", err)
				}
			}
			delete(route.Status.Resources.Services, key)

			// Get original route name for endpoint key
			originalRouteName := route.Name
			if route.Spec.Source.Kubernetes != nil && route.Spec.Source.Kubernetes.Route.GetName() != "" {
				originalRouteName = route.Spec.Source.Kubernetes.Route.GetName()
			} else if name := route.GetLabels()[kubelb.LabelOriginName]; name != "" {
				originalRouteName = name
			}
			endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointRoutePattern, route.Namespace, value.Namespace, svc.Name, originalRouteName)

			// De-allocate the ports allocated for the service.
			r.PortAllocator.DeallocateEndpoints([]string{endpointKey})
		}
	}
	return nil
}

// cleanupRenamedServices deletes orphaned services from naming scheme change.
// Old scheme: ns-svc[-uid], New scheme: ns-route-svc[-uid]
func (r *RouteReconciler) cleanupRenamedServices(ctx context.Context, log logr.Logger, route *kubelbv1alpha1.Route, originalRouteName string) {
	if route.Status.Resources.Services == nil || route.Spec.Source.Kubernetes == nil {
		return
	}

	for _, service := range route.Spec.Source.Kubernetes.Services {
		name := serviceHelpers.GetServiceName(service.Service)
		key := fmt.Sprintf(kubelb.RouteServiceMapKey, service.Namespace, name)

		statusSvc, exists := route.Status.Resources.Services[key]
		if !exists {
			continue
		}

		// Compute expected name with new naming scheme
		expectedName := kubelb.GenerateRouteServiceName(
			originalRouteName,
			name,
			service.Namespace,
		)

		// If GeneratedName differs from expected, it's an orphan from old naming
		if statusSvc.GeneratedName != expectedName {
			log.V(4).Info("Deleting orphaned service from naming change",
				"old", statusSvc.GeneratedName,
				"new", expectedName)

			svc := corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      statusSvc.GeneratedName,
					Namespace: route.Namespace,
				},
			}

			if err := r.Delete(ctx, &svc); err != nil && !kerrors.IsNotFound(err) {
				log.Error(err, "failed to delete orphaned service", "name", statusSvc.GeneratedName)
			}
		}
	}
}

func (r *RouteReconciler) UpdateRouteStatus(ctx context.Context, route *kubelbv1alpha1.Route, status kubelbv1alpha1.RouteStatus) error {
	key := ctrlruntimeclient.ObjectKeyFromObject(route)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Fetch the current state
		if err := r.Get(ctx, key, route); err != nil {
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
		return r.Status().Patch(ctx, route, ctrlruntimeclient.MergeFrom(original))
	})
}

func (r *RouteReconciler) manageRoutes(ctx context.Context, log logr.Logger, route *kubelbv1alpha1.Route, config *kubelbv1alpha1.Config, tenant *kubelbv1alpha1.Tenant, annotations kubelbv1alpha1.AnnotationSettings) error {
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
		Controller: ptr.To(true),
	}

	// Set owner reference for the resource.
	resource.SetOwnerReferences([]metav1.OwnerReference{ownerReference})

	// Get original route name for service naming consistency with port allocation
	originalRouteName := route.Name
	if route.Spec.Source.Kubernetes.Route.GetName() != "" {
		originalRouteName = route.Spec.Source.Kubernetes.Route.GetName()
	} else if name := route.GetLabels()[kubelb.LabelOriginName]; name != "" {
		originalRouteName = name
	}

	// Get the services referenced by the route.
	var referencedServices []metav1.ObjectMeta
	for _, service := range route.Spec.Source.Kubernetes.Services {
		name := serviceHelpers.GetServiceName(service.Service)
		objectMeta := metav1.ObjectMeta{
			Name:      name,
			Namespace: service.Namespace,
			UID:       service.UID,
		}
		referencedServices = append(referencedServices, objectMeta)
	}

	routeStatus := route.Status.DeepCopy()

	// Determine the type of the resource and call the appropriate method
	switch v := resource.(type) {
	case *v1.Ingress: // v1 "k8s.io/api/networking/v1"
		err = ingressHelpers.CreateOrUpdateIngress(ctx, log, r.Client, v, referencedServices, route.Namespace, originalRouteName, config, tenant, annotations)
		if err == nil {
			// Retrieve updated object to get the status.
			key := ctrlruntimeclient.ObjectKey{Namespace: v.Namespace, Name: v.Name}
			res := &v1.Ingress{}
			if err := r.Get(ctx, key, res); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to get Ingress: %w", err)
				}
			}
			updateResourceStatus(routeStatus, res, err)
		}

	case *gwapiv1.Gateway: // v1 "sigs.k8s.io/gateway-api/apis/v1"
		err = gatewayHelpers.CreateOrUpdateGateway(ctx, log, r.Client, v, route.Namespace, config, tenant, annotations)
		if err == nil {
			// Retrieve updated object to get the status.
			key := ctrlruntimeclient.ObjectKey{Namespace: v.Namespace, Name: v.Name}
			res := &gwapiv1.Gateway{}
			if err := r.Get(ctx, key, res); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to get Gateway: %w", err)
				}
			}
			updateResourceStatus(routeStatus, res, err)
		}

	case *gwapiv1.HTTPRoute: // v1 "sigs.k8s.io/gateway-api/apis/v1"
		err = httprouteHelpers.CreateOrUpdateHTTPRoute(ctx, log, r.Client, v, referencedServices, route.Namespace, originalRouteName, tenant, annotations)
		if err == nil {
			// Retrieve updated object to get the status.
			key := ctrlruntimeclient.ObjectKey{Namespace: v.Namespace, Name: v.Name}
			res := &gwapiv1.HTTPRoute{}
			if err := r.Get(ctx, key, res); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to get HTTPRoute: %w", err)
				}
			}
			updateResourceStatus(routeStatus, res, err)
		}

	case *gwapiv1.GRPCRoute: // v1 "sigs.k8s.io/gateway-api/apis/v1"
		err = grpcrouteHelpers.CreateOrUpdateGRPCRoute(ctx, log, r.Client, v, referencedServices, route.Namespace, originalRouteName, tenant, annotations)
		if err == nil {
			// Retrieve updated object to get the status.
			key := ctrlruntimeclient.ObjectKey{Namespace: v.Namespace, Name: v.Name}
			res := &gwapiv1.GRPCRoute{}
			if err := r.Get(ctx, key, res); err != nil {
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

func (r *RouteReconciler) shouldReconcile(ctx context.Context, route *kubelbv1alpha1.Route, tenant *kubelbv1alpha1.Tenant, config *kubelbv1alpha1.Config) (bool, bool, error) {
	log := ctrl.LoggerFrom(ctx)

	// First step is to determine the route type.
	if route.Spec.Source.Kubernetes == nil {
		// There is no source defined.
		return false, false, nil
	}

	resource, err := unstructured.ConvertUnstructuredToObject(&route.Spec.Source.Kubernetes.Route)
	if err != nil {
		return false, false, fmt.Errorf("failed to convert route to object: %w", err)
	}

	switch v := resource.(type) {
	case *v1.Ingress:
		// Ensure that Ingress is enabled
		if config.Spec.Ingress.Disable {
			log.Error(fmt.Errorf("ingress is disabled at the global level"), "cannot proceed")
			return false, true, nil
		} else if tenant.Spec.Ingress.Disable {
			log.Error(fmt.Errorf("ingress is disabled at the tenant level"), "cannot proceed")
			return false, true, nil
		}

	case *gwapiv1.Gateway:
		// Ensure that Gateway is enabled
		if isGatewayAPIDisabled(log, r.DisableGatewayAPI, *config, *tenant) {
			return false, true, nil
		}

		if !gatewayHelpers.ShouldReconcileGatewayByName(v) {
			return false, false, nil
		}

	case *gwapiv1.GRPCRoute:
		// Ensure that Gateway is enabled
		if isGatewayAPIDisabled(log, r.DisableGatewayAPI, *config, *tenant) {
			return false, true, nil
		}

	case *gwapiv1.HTTPRoute:
		// Ensure that Gateway is enabled
		if isGatewayAPIDisabled(log, r.DisableGatewayAPI, *config, *tenant) {
			return false, true, nil
		}

	default:
		log.Error(fmt.Errorf("resource %v is not supported", v.GetObjectKind().GroupVersionKind().GroupKind().String()), "cannot proceed")
		return false, false, nil
	}
	return true, false, nil
}

func isGatewayAPIDisabled(log logr.Logger, disableGatewayAPI bool, config kubelbv1alpha1.Config, tenant kubelbv1alpha1.Tenant) bool {
	if disableGatewayAPI {
		log.Error(fmt.Errorf("gateway api is disabled at the global level"), "cannot proceed")
		return true
	}
	// Ensure that Gateway API is enabled
	if config.Spec.GatewayAPI.Disable {
		log.Error(fmt.Errorf("gateway api is disabled at the global level"), "cannot proceed")
		return true
	} else if tenant.Spec.GatewayAPI.Disable {
		log.Error(fmt.Errorf("gateway api is disabled at the tenant level"), "cannot proceed")
		return true
	}
	return false
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

func updateResourceStatus(routeStatus *kubelbv1alpha1.RouteStatus, obj ctrlruntimeclient.Object, err error) {
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

func (r *RouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// 1. Watch for changes in Route object.
	// 2. Skip reconciliation if generation is not changed; only status/metadata changed.
	controller := ctrl.NewControllerManagedBy(mgr).
		Named(RouteControllerName).
		For(&kubelbv1alpha1.Route{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&kubelbv1alpha1.Config{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueRoutesForConfig()),
		).
		Watches(
			&kubelbv1alpha1.Tenant{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueRoutesForTenant()),
		).
		Owns(&v1.Ingress{})

	if !r.DisableGatewayAPI {
		controller = controller.Owns(&gwapiv1.Gateway{}).
			Owns(&gwapiv1.HTTPRoute{}).
			Owns(&gwapiv1.GRPCRoute{})
	}
	return controller.Complete(r)
}

// enqueueRoutesForConfig is a handler.MapFunc to be used to enqueue requests for reconciliation
// for Routes if some change is made to the controller config.
func (r *RouteReconciler) enqueueRoutesForConfig() handler.MapFunc {
	return func(ctx context.Context, _ ctrlruntimeclient.Object) []ctrl.Request {
		result := []reconcile.Request{}

		// List all routes. We don't care about the namespace here.
		routes := &kubelbv1alpha1.RouteList{}
		err := r.List(ctx, routes)
		if err != nil {
			return result
		}

		for _, route := range routes.Items {
			result = append(result, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      route.Name,
					Namespace: route.Namespace,
				},
			})
		}
		return result
	}
}

// enqueueRoutesForTenant is a handler.MapFunc to be used to enqueue requests for reconciliation
// for Routes if some change is made to the tenant config.
func (r *RouteReconciler) enqueueRoutesForTenant() handler.MapFunc {
	return func(ctx context.Context, o ctrlruntimeclient.Object) []ctrl.Request {
		result := []reconcile.Request{}

		namespace := fmt.Sprintf(tenantNamespacePattern, o.GetName())

		// List all routes in tenant namespace
		routes := &kubelbv1alpha1.RouteList{}
		err := r.List(ctx, routes, ctrlruntimeclient.InNamespace(namespace))
		if err != nil {
			return result
		}

		for _, lb := range routes.Items {
			result = append(result, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      lb.Name,
					Namespace: lb.Namespace,
				},
			})
		}
		return result
	}
}

// getRouteType determines the route type from the Route resource for metrics labeling.
func getRouteType(route *kubelbv1alpha1.Route) string {
	if route.Spec.Source.Kubernetes == nil {
		return metricsutil.RouteTypeIngress
	}

	// Get kind from the embedded unstructured Route resource
	kind := route.Spec.Source.Kubernetes.Route.GetKind()
	switch kind {
	case "Gateway":
		return metricsutil.RouteTypeGateway
	case "HTTPRoute":
		return metricsutil.RouteTypeHTTPRoute
	case "GRPCRoute":
		return metricsutil.RouteTypeGRPCRoute
	default:
		return metricsutil.RouteTypeIngress
	}
}
