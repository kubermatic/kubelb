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
	grpcrouteHelpers "k8c.io/kubelb/internal/resources/gatewayapi/grpcroute"
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
	GatewayGRPCRouteControllerName = "gateway-grpcroute-controller"
)

// GRPCRouteReconciler reconciles an GRPCRoute Object
type GRPCRouteReconciler struct {
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
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=grpcroutes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=grpcroutes/status,verbs=get;update;patch

func (r *GRPCRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)
	startTime := time.Now()

	// Track reconciliation duration
	defer func() {
		ccmmetrics.GRPCRouteReconcileDuration.WithLabelValues(req.Namespace).Observe(time.Since(startTime).Seconds())
	}()

	log.Info("Reconciling GRPCRoute")

	resource := &gwapiv1.GRPCRoute{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		ccmmetrics.GRPCRouteReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
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
		ccmmetrics.GRPCRouteReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSkipped).Inc()
		return reconcile.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(resource, CleanupFinalizer) {
		controllerutil.AddFinalizer(resource, CleanupFinalizer)
		if err := r.Update(ctx, resource); err != nil {
			ccmmetrics.GRPCRouteReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	err := r.reconcile(ctx, log, resource)
	if err != nil {
		log.Error(err, "reconciling failed")
		ccmmetrics.GRPCRouteReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return reconcile.Result{}, err
	}

	// Update managed grpcroutes gauge
	grpcRouteList := &gwapiv1.GRPCRouteList{}
	if err := r.List(ctx, grpcRouteList, ctrlclient.InNamespace(req.Namespace)); err == nil {
		count := 0
		for _, gr := range grpcRouteList.Items {
			if r.shouldReconcile(&gr) && gr.DeletionTimestamp == nil {
				count++
			}
		}
		ccmmetrics.ManagedGRPCRoutesTotal.WithLabelValues(req.Namespace).Set(float64(count))
	}

	ccmmetrics.GRPCRouteReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSuccess).Inc()
	return reconcile.Result{}, nil
}

func (r *GRPCRouteReconciler) reconcile(ctx context.Context, log logr.Logger, grpcRoute *gwapiv1.GRPCRoute) error {
	// We need to traverse the GRPCRoute, find all the services associated with it, create/update the corresponding Route in LB cluster.
	originalServices := grpcrouteHelpers.GetServicesFromGRPCRoute(grpcRoute)
	err := reconcileSourceForRoute(ctx, log, r.Client, r.LBManager.GetClient(), grpcRoute, originalServices, nil, r.ClusterName)
	if err != nil {
		return fmt.Errorf("failed to reconcile source for route: %w", err)
	}

	// Route was reconciled successfully, now we need to update the status of the Resource.
	route := kubelbv1alpha1.Route{}
	err = r.LBManager.GetClient().Get(ctx, types.NamespacedName{Name: string(grpcRoute.UID), Namespace: r.ClusterName}, &route)
	if err != nil {
		return fmt.Errorf("failed to get Route from LB cluster: %w", err)
	}

	// Update the status of the GRPCRoute
	if len(route.Status.Resources.Route.GeneratedName) > 0 {
		// First we need to ensure that status is available in the Route
		resourceStatus := route.Status.Resources.Route.Status
		jsonData, err := json.Marshal(resourceStatus.Raw)
		if err != nil || string(jsonData) == kubelb.DefaultRouteStatus {
			// Status is not available in the Route, so we need to wait for it
			return nil
		}

		// Convert rawExtension to gwapiv1.GRPCRouteStatus
		status := gwapiv1.GRPCRouteStatus{}
		if err := yaml.UnmarshalStrict(resourceStatus.Raw, &status); err != nil {
			return fmt.Errorf("failed to unmarshal GRPCRoute status: %w", err)
		}

		log.V(3).Info("updating GRPCRoute status", "name", grpcRoute.Name, "namespace", grpcRoute.Namespace)
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, types.NamespacedName{Name: grpcRoute.Name, Namespace: grpcRoute.Namespace}, grpcRoute); err != nil {
				return err
			}
			original := grpcRoute.DeepCopy()
			grpcRoute.Status = status
			if reflect.DeepEqual(original.Status, grpcRoute.Status) {
				return nil
			}
			// update the status
			return r.Status().Patch(ctx, grpcRoute, ctrlclient.MergeFrom(original))
		})
	}
	return nil
}

func (r *GRPCRouteReconciler) cleanup(ctx context.Context, grpcRoute *gwapiv1.GRPCRoute) (ctrl.Result, error) {
	impactedServices := grpcrouteHelpers.GetServicesFromGRPCRoute(grpcRoute)
	services := corev1.ServiceList{}
	err := r.List(ctx, &services, ctrlclient.InNamespace(grpcRoute.Namespace), ctrlclient.MatchingLabels{kubelb.LabelManagedBy: kubelb.LabelControllerName})
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
	err = cleanupRoute(ctx, r.LBManager.GetClient(), string(grpcRoute.UID), r.ClusterName)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to cleanup route: %w", err)
	}

	controllerutil.RemoveFinalizer(grpcRoute, CleanupFinalizer)
	if err := r.Update(ctx, grpcRoute); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return reconcile.Result{}, nil
}

// enqueueResources is a handler.MapFunc to be used to enqueue requests for reconciliation
// for GRPCRoutes against the corresponding service.
func (r *GRPCRouteReconciler) enqueueResources() handler.MapFunc {
	return func(_ context.Context, o ctrlclient.Object) []ctrl.Request {
		result := []reconcile.Request{}
		grpcRouteList := &gwapiv1.GRPCRouteList{}
		if err := r.List(context.Background(), grpcRouteList, ctrlclient.InNamespace(o.GetNamespace())); err != nil {
			return nil
		}

		for _, grpcRoute := range grpcRouteList.Items {
			if !r.shouldReconcile(&grpcRoute) {
				continue
			}

			services := grpcrouteHelpers.GetServicesFromGRPCRoute(&grpcRoute)
			for _, serviceRef := range services {
				if (serviceRef.Name == o.GetName() || fmt.Sprintf(serviceHelpers.NodePortServicePattern, serviceRef.Name) == o.GetName()) && serviceRef.Namespace == o.GetNamespace() {
					result = append(result, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      grpcRoute.Name,
							Namespace: grpcRoute.Namespace,
						},
					})
				}
			}
		}
		return result
	}
}

func (r *GRPCRouteReconciler) resourceFilter() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if obj, ok := e.Object.(*gwapiv1.GRPCRoute); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if obj, ok := e.ObjectNew.(*gwapiv1.GRPCRoute); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if obj, ok := e.Object.(*gwapiv1.GRPCRoute); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			if obj, ok := e.Object.(*gwapiv1.GRPCRoute); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
	}
}

// shouldReconcile returns true if the GRPCRoute should be reconciled by the controller.
func (r *GRPCRouteReconciler) shouldReconcile(grpcRoute *gwapiv1.GRPCRoute) bool {
	return gatewayhelper.ShouldReconcileResource(grpcRoute, false)
}

func (r *GRPCRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gwapiv1.GRPCRoute{}, builder.WithPredicates(r.resourceFilter())).
		Named(GatewayGRPCRouteControllerName).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueResources()),
		).
		WatchesRawSource(
			source.Kind(r.LBManager.GetCache(), &kubelbv1alpha1.Route{},
				handler.TypedEnqueueRequestsFromMapFunc[*kubelbv1alpha1.Route](enqueueRoutes("GRPCRoute.gateway.networking.k8s.io", r.ClusterName))),
		).
		Complete(r)
}
