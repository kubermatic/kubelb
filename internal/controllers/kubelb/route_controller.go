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
	kuberneteshelper "k8c.io/kubelb/internal/kubernetes"
	serviceHelpers "k8c.io/kubelb/internal/resources/service"
	"k8c.io/kubelb/internal/resources/unstructured"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

	EnvoyProxyTopology EnvoyProxyTopology
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes/status,verbs=get;update;patch

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
		if kuberneteshelper.HasFinalizer(resource, CleanupFinalizer) {
			return r.cleanup(ctx, resource)
		}
		// Finalizer doesn't exist so clean up is already done
		return reconcile.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !kuberneteshelper.HasFinalizer(resource, CleanupFinalizer) {
		kuberneteshelper.AddFinalizer(resource, CleanupFinalizer)
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
	// Create or update services based on the route.
	err := r.manageServices(ctx, log, route)
	if err != nil {
		return fmt.Errorf("failed to create or update services: %w", err)
	}

	// Create or update the route object.
	err = r.manageRoutes(ctx, log, route)
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

	for _, value := range route.Status.Resources.Services {
		log := r.Log.WithValues("name", value.Name, "namespace", value.Namespace)
		log.V(1).Info("Deleting service", "name", value.Name, "namespace", value.Namespace)
		svc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      value.Name,
				Namespace: value.Namespace,
			},
		}
		if err := r.Client.Delete(ctx, &svc); err != nil {
			if !kerrors.IsNotFound(err) {
				return reconcile.Result{}, fmt.Errorf("failed to delete service: %w", err)
			}
		}
	}

	kuberneteshelper.RemoveFinalizer(route, CleanupFinalizer)
	if err := r.Update(ctx, route); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *RouteReconciler) manageServices(ctx context.Context, log logr.Logger, route *kubelbv1alpha1.Route) error {
	if route.Spec.Source.Kubernetes == nil {
		return nil
	}

	// Before creating/updating services, ensure that the orphaned services are cleaned up.
	err := r.cleanupOrphanedServices(ctx, log, route)
	if err != nil {
		return fmt.Errorf("failed to cleanup orphaned services: %w", err)
	}

	appName := envoyApplicationName(r.EnvoyProxyTopology, route.Namespace)
	services := []corev1.Service{}
	for _, service := range route.Spec.Source.Kubernetes.Services {
		// Transform the service into desired state.
		svc := serviceHelpers.GenerateServiceForLBCluster(service.Service, appName, route.Namespace)
		services = append(services, svc)
	}

	routeStatus := route.Status.DeepCopy()
	for _, svc := range services {
		log.V(4).Info("Creating/Updating service", "name", svc.Name, "namespace", svc.Namespace)
		var err error
		if _, err = ctrl.CreateOrUpdate(ctx, r.Client, &svc, func() error {
			return nil
		}); err != nil {
			// We only log the error and set the condition to false. The error will be set in the status.
			log.Error(err, "failed to create or update Service", "name", svc.Name, "namespace", svc.Namespace)
			errorMessage := fmt.Errorf("failed to create or update Service: %w", err)
			r.Recorder.Eventf(route, corev1.EventTypeWarning, "ServiceApplyFailed", errorMessage.Error())
		}
		updateResourceStatus(routeStatus, &svc, err)
	}
	return r.UpdateRouteStatus(ctx, route, *routeStatus)
}

func (r *RouteReconciler) cleanupOrphanedServices(ctx context.Context, log logr.Logger, route *kubelbv1alpha1.Route) error {
	// Get all the services based on route.
	desiredServices := map[string]bool{}
	for _, service := range route.Spec.Source.Kubernetes.Services {
		desiredServices[service.Namespace+"/"+service.Name] = true
	}

	if route.Status.Resources.Services == nil {
		return nil
	}

	for key, value := range route.Status.Resources.Services {
		if _, ok := desiredServices[key]; !ok {
			// Service is not desired, so delete it.
			log.V(4).Info("Deleting orphaned service", "name", value.Name, "namespace", value.Namespace)
			svc := corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      value.Name,
					Namespace: value.Namespace,
				},
			}
			if err := r.Client.Delete(ctx, &svc); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to delete orphaned service: %w", err)
				}
			}
			delete(route.Status.Resources.Services, key)
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

func (r *RouteReconciler) manageRoutes(ctx context.Context, log logr.Logger, route *kubelbv1alpha1.Route) error {
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
	case *v1.Ingress: // Assuming v1 "k8s.io/api/networking/v1"
		err = r.createOrUpdateIngress(ctx, log, v, referencedServices, route.Namespace)
	default:
		log.V(4).Info("Unsupported resource type")
	}

	updateResourceStatus(routeStatus, resource, err)
	return r.UpdateRouteStatus(ctx, route, *routeStatus)
}

// createOrUpdateIngress creates or updates the Ingress object in the cluster.
func (r *RouteReconciler) createOrUpdateIngress(ctx context.Context, log logr.Logger, ingress *v1.Ingress, referencedServices []metav1.ObjectMeta, namespace string) error {
	// Name of the services referenced by the Ingress have to be updated to match the services created against the Route in the LB cluster.
	for i, rule := range ingress.Spec.Rules {
		for j, path := range rule.HTTP.Paths {
			for _, service := range referencedServices {
				if path.Backend.Service.Name == service.Name {
					ingress.Spec.Rules[i].HTTP.Paths[j].Backend.Service.Name = kubelb.GenerateName(false, string(service.UID), service.Name, service.Namespace)
				}
			}
		}
	}

	if ingress.Spec.DefaultBackend != nil && ingress.Spec.DefaultBackend.Service != nil {
		for _, service := range referencedServices {
			if ingress.Spec.DefaultBackend.Service.Name == service.Name {
				ingress.Spec.DefaultBackend.Service.Name = kubelb.GenerateName(false, string(service.UID), service.Name, service.Namespace)
			}
		}
	}

	ingress.Spec.IngressClassName = config.GetConfig().Spec.IngressClassName
	ingress.Name = kubelb.GenerateName(false, string(ingress.UID), ingress.Name, ingress.Namespace)
	ingress.Namespace = namespace
	ingress.SetUID("") // Reset UID to generate a new UID for the Ingress object

	log.V(4).Info("Creating/Updating Ingress", "name", ingress.Name, "namespace", ingress.Namespace)

	// Create or update the Ingress object in the cluster.
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, ingress, func() error {
		return nil
	}); err != nil {
		return fmt.Errorf("failed to create or update Ingress: %w", err)
	}
	return nil
}

func updateResourceStatus(routeStatus *kubelbv1alpha1.RouteStatus, obj client.Object, err error) {
	status := kubelbv1alpha1.ResourceStatus{
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
	}

	conditionMessage := "Success"
	conditionStatus := metav1.ConditionTrue
	conditionReason := "InstallationSuccessful"
	if err != nil {
		conditionMessage = err.Error()
		conditionStatus = metav1.ConditionFalse
		conditionReason = "InstallationFailed"
	}
	status.Conditions = []metav1.Condition{
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

	var resourceStatus []byte

	switch resource := obj.(type) {
	case *v1.Ingress:
		resourceStatus, err = json.Marshal(resource.Status)
		if err != nil {
			// If we are unable to marshal the status, we set it to empty object. There is no need to fail the reconciliation.
			resourceStatus = []byte("{}")
		}
		status.Status = runtime.RawExtension{
			Raw: resourceStatus,
		}
		routeStatus.Resources.Route = status
	case *corev1.Service:
		resourceStatus, err = json.Marshal(resource.Status)
		if err != nil {
			// If we are unable to marshal the status, we set it to empty object. There is no need to fail the reconciliation.
			resourceStatus = []byte("{}")
		}
		svcName := resource.Name
		if resource.Labels[kubelb.LabelOriginName] != "" {
			svcName = resource.Labels[kubelb.LabelOriginName]
		}
		status.Name = svcName
		status.Status = runtime.RawExtension{
			Raw: resourceStatus,
		}
		if routeStatus.Resources.Services == nil {
			routeStatus.Resources.Services = make(map[string]kubelbv1alpha1.ResourceStatus)
		}
		routeStatus.Resources.Services[resource.Namespace+"/"+svcName] = status
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
