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
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	"k8c.io/kubelb/internal/metricsutil"
	ccmmetrics "k8c.io/kubelb/internal/metricsutil/ccm"
	gatewayhelper "k8c.io/kubelb/internal/resources/gatewayapi/gateway"
	httprouteHelpers "k8c.io/kubelb/internal/resources/gatewayapi/httproute"
	serviceHelpers "k8c.io/kubelb/internal/resources/service"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/yaml"
)

const (
	GatewayHTTPRouteControllerName = "gateway-httproute-controller"
)

// HTTPRouteReconciler reconciles an HTTPRoute Object
type HTTPRouteReconciler struct {
	ctrlclient.Client

	LBManager   ctrl.Manager
	ClusterName string

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/status,verbs=get;update;patch

func (r *HTTPRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)
	startTime := time.Now()

	// Track reconciliation duration
	defer func() {
		ccmmetrics.HTTPRouteReconcileDuration.WithLabelValues(req.Namespace).Observe(time.Since(startTime).Seconds())
	}()

	log.Info("Reconciling HTTPRoute")

	resource := &gwapiv1.HTTPRoute{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		ccmmetrics.HTTPRouteReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
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

	if !r.shouldReconcile(resource) {
		ccmmetrics.HTTPRouteReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSkipped).Inc()
		return reconcile.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(resource, CleanupFinalizer) {
		if ok := controllerutil.AddFinalizer(resource, CleanupFinalizer); !ok {
			log.Error(nil, "Failed to add finalizer for the HTTPRoute")
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.Update(ctx, resource); err != nil {
			ccmmetrics.HTTPRouteReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	err := r.reconcile(ctx, log, resource)
	if err != nil {
		log.Error(err, "reconciling failed")
		ccmmetrics.HTTPRouteReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return reconcile.Result{}, err
	}

	// Update managed httproutes gauge
	httpRouteList := &gwapiv1.HTTPRouteList{}
	if err := r.List(ctx, httpRouteList, ctrlclient.InNamespace(req.Namespace)); err == nil {
		count := 0
		for _, hr := range httpRouteList.Items {
			if r.shouldReconcile(&hr) && hr.DeletionTimestamp == nil {
				count++
			}
		}
		ccmmetrics.ManagedHTTPRoutesTotal.WithLabelValues(req.Namespace).Set(float64(count))
	}

	ccmmetrics.HTTPRouteReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSuccess).Inc()
	return reconcile.Result{}, nil
}

func (r *HTTPRouteReconciler) reconcile(ctx context.Context, log logr.Logger, httpRoute *gwapiv1.HTTPRoute) error {
	// We need to traverse the HTTPRoute, find all the services associated with it, create/update the corresponding Route in LB cluster.
	originalServices := httprouteHelpers.GetServicesFromHTTPRoute(httpRoute)
	err := reconcileSourceForRoute(ctx, log, r.Client, r.LBManager.GetClient(), httpRoute, originalServices, nil, r.ClusterName)
	if err != nil {
		return fmt.Errorf("failed to reconcile source for route: %w", err)
	}

	// Route was reconciled successfully, now we need to update the status of the Resource.
	route := kubelbv1alpha1.Route{}
	err = r.LBManager.GetClient().Get(ctx, types.NamespacedName{Name: string(httpRoute.UID), Namespace: r.ClusterName}, &route)
	if err != nil {
		return fmt.Errorf("failed to get Route from LB cluster: %w", err)
	}

	// Update the status of the HTTPRoute
	if len(route.Status.Resources.Route.GeneratedName) > 0 {
		// First we need to ensure that status is available in the Route
		resourceStatus := route.Status.Resources.Route.Status
		jsonData, err := json.Marshal(resourceStatus.Raw)
		if err != nil || string(jsonData) == kubelb.DefaultRouteStatus {
			// Status is not available in the Route, so we need to wait for it
			return nil
		}

		// Convert rawExtension to gwapiv1.HTTPRouteStatus
		status := gwapiv1.HTTPRouteStatus{}
		if err := yaml.UnmarshalStrict(resourceStatus.Raw, &status); err != nil {
			return fmt.Errorf("failed to unmarshal HTTPRoute status: %w", err)
		}

		log.V(3).Info("updating HTTPRoute status", "name", httpRoute.Name, "namespace", httpRoute.Namespace)
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, types.NamespacedName{Name: httpRoute.Name, Namespace: httpRoute.Namespace}, httpRoute); err != nil {
				return err
			}
			original := httpRoute.DeepCopy()
			httpRoute.Status = status
			if reflect.DeepEqual(original.Status, httpRoute.Status) {
				return nil
			}
			// update the status
			return r.Status().Patch(ctx, httpRoute, ctrlclient.MergeFrom(original))
		})
	}
	return nil
}

func (r *HTTPRouteReconciler) cleanup(ctx context.Context, httpRoute *gwapiv1.HTTPRoute) (ctrl.Result, error) {
	impactedServices := httprouteHelpers.GetServicesFromHTTPRoute(httpRoute)
	services := corev1.ServiceList{}
	err := r.List(ctx, &services, ctrlclient.InNamespace(httpRoute.Namespace), ctrlclient.MatchingLabels{kubelb.LabelManagedBy: kubelb.LabelControllerName})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list services: %w", err)
	}

	// Delete services created by the controller.
	for _, service := range services.Items {
		originalName := service.Name
		if service.Labels[kubelb.LabelOriginName] != "" {
			originalName = service.Labels[kubelb.LabelOriginName]
		}

		for _, serviceRef := range impactedServices {
			if serviceRef.Name == originalName && serviceRef.Namespace == service.Namespace {
				err := r.Delete(ctx, &service)
				if err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to delete service: %w", err)
				}
			}
		}
	}

	// Find the Route in LB cluster and delete it
	err = cleanupRoute(ctx, r.LBManager.GetClient(), string(httpRoute.UID), r.ClusterName)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to cleanup route: %w", err)
	}

	controllerutil.RemoveFinalizer(httpRoute, CleanupFinalizer)
	if err := r.Update(ctx, httpRoute); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return reconcile.Result{}, nil
}

// enqueueResources is a handler.MapFunc to be used to enqueue requests for reconciliation
// for HTTPRoutes against the corresponding service.
func (r *HTTPRouteReconciler) enqueueResources() handler.MapFunc {
	return func(_ context.Context, o ctrlclient.Object) []ctrl.Request {
		result := []reconcile.Request{}
		httpRouteList := &gwapiv1.HTTPRouteList{}
		if err := r.List(context.Background(), httpRouteList, ctrlclient.InNamespace(o.GetNamespace())); err != nil {
			return nil
		}

		for _, httpRoute := range httpRouteList.Items {
			if !r.shouldReconcile(&httpRoute) {
				continue
			}

			services := httprouteHelpers.GetServicesFromHTTPRoute(&httpRoute)
			for _, serviceRef := range services {
				if (serviceRef.Name == o.GetName() || fmt.Sprintf(serviceHelpers.NodePortServicePattern, serviceRef.Name) == o.GetName()) && serviceRef.Namespace == o.GetNamespace() {
					result = append(result, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      httpRoute.Name,
							Namespace: httpRoute.Namespace,
						},
					})
				}
			}
		}
		return result
	}
}

func (r *HTTPRouteReconciler) resourceFilter() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if obj, ok := e.Object.(*gwapiv1.HTTPRoute); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if obj, ok := e.ObjectNew.(*gwapiv1.HTTPRoute); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if obj, ok := e.Object.(*gwapiv1.HTTPRoute); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			if obj, ok := e.Object.(*gwapiv1.HTTPRoute); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
	}
}

// shouldReconcile returns true if the HTTPRoute should be reconciled by the controller.
// In Community Edition, the controller only reconciles HTTPRoutes against the gateway named "kubelb".
func (r *HTTPRouteReconciler) shouldReconcile(httpRoute *gwapiv1.HTTPRoute) bool {
	return gatewayhelper.ShouldReconcileResource(httpRoute, false)
}

func (r *HTTPRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(GatewayHTTPRouteControllerName).
		For(&gwapiv1.HTTPRoute{}, builder.WithPredicates(r.resourceFilter())).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueResources()),
		).
		WatchesRawSource(
			source.Kind(r.LBManager.GetCache(), &kubelbv1alpha1.Route{},
				handler.TypedEnqueueRequestsFromMapFunc[*kubelbv1alpha1.Route](enqueueRoutes("HTTPRoute.gateway.networking.k8s.io", r.ClusterName))),
		).
		Complete(r)
}
