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
	ingressHelpers "k8c.io/kubelb/internal/resources/ingress"
	serviceHelpers "k8c.io/kubelb/internal/resources/service"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
	"sigs.k8s.io/yaml"
)

const (
	IngressControllerName = "ingress-controller"
	IngressClassName      = "kubelb"
)

// IngressReconciler reconciles an Ingress Object
type IngressReconciler struct {
	ctrlclient.Client

	LBManager       ctrl.Manager
	ClusterName     string
	UseIngressClass bool

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/status,verbs=get;update;patch

func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)
	startTime := time.Now()

	// Track reconciliation duration
	defer func() {
		ccmmetrics.IngressReconcileDuration.WithLabelValues(req.Namespace).Observe(time.Since(startTime).Seconds())
	}()

	log.Info("Reconciling Ingress")

	resource := &networkingv1.Ingress{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		ccmmetrics.IngressReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
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
		ccmmetrics.IngressReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSkipped).Inc()
		return reconcile.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(resource, CleanupFinalizer) {
		if ok := controllerutil.AddFinalizer(resource, CleanupFinalizer); !ok {
			log.Error(nil, "Failed to add finalizer for the Ingress")
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.Update(ctx, resource); err != nil {
			ccmmetrics.IngressReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	err := r.reconcile(ctx, log, resource)
	if err != nil {
		log.Error(err, "reconciling failed")
		ccmmetrics.IngressReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return reconcile.Result{}, err
	}

	// Update managed ingresses gauge
	ingressList := &networkingv1.IngressList{}
	if err := r.List(ctx, ingressList, ctrlclient.InNamespace(req.Namespace)); err == nil {
		count := 0
		for _, ing := range ingressList.Items {
			if r.shouldReconcile(&ing) && ing.DeletionTimestamp == nil {
				count++
			}
		}
		ccmmetrics.ManagedIngressesTotal.WithLabelValues(req.Namespace).Set(float64(count))
	}

	ccmmetrics.IngressReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSuccess).Inc()
	return reconcile.Result{}, nil
}

func (r *IngressReconciler) reconcile(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress) error {
	// We need to traverse the Ingress, find all the services associated with it, create/update the corresponding Route in LB cluster.
	originalServices := ingressHelpers.GetServicesFromIngress(*ingress)
	err := reconcileSourceForRoute(ctx, log, r.Client, r.LBManager.GetClient(), ingress, originalServices, nil, r.ClusterName)
	if err != nil {
		return fmt.Errorf("failed to reconcile source for route: %w", err)
	}

	// Route was reconciled successfully, now we need to update the status of the Ingress.
	route := kubelbv1alpha1.Route{}
	err = r.LBManager.GetClient().Get(ctx, types.NamespacedName{Name: string(ingress.UID), Namespace: r.ClusterName}, &route)
	if err != nil {
		return fmt.Errorf("failed to get Route from LB cluster: %w", err)
	}

	// Update the status of the Ingress
	if len(route.Status.Resources.Route.GeneratedName) > 0 {
		// First we need to ensure that status is available in the Route
		resourceStatus := route.Status.Resources.Route.Status
		jsonData, err := json.Marshal(resourceStatus.Raw)
		if err != nil || string(jsonData) == kubelb.DefaultRouteStatus {
			// Status is not available in the Route, so we need to wait for it
			return nil
		}

		// Convert rawExtension to networkingv1.IngressStatus
		status := networkingv1.IngressStatus{}
		if err := yaml.UnmarshalStrict(resourceStatus.Raw, &status); err != nil {
			return fmt.Errorf("failed to unmarshal Ingress status: %w", err)
		}

		log.V(3).Info("updating Ingress status", "name", ingress.Name, "namespace", ingress.Namespace)
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, ingress); err != nil {
				return err
			}
			original := ingress.DeepCopy()
			ingress.Status = status
			if reflect.DeepEqual(original.Status, ingress.Status) {
				return nil
			}
			// update the status
			return r.Status().Patch(ctx, ingress, ctrlclient.MergeFrom(original))
		})
	}
	return nil
}

func (r *IngressReconciler) cleanup(ctx context.Context, ingress *networkingv1.Ingress) (ctrl.Result, error) {
	impactedServices := ingressHelpers.GetServicesFromIngress(*ingress)
	services := corev1.ServiceList{}
	err := r.List(ctx, &services, ctrlclient.InNamespace(ingress.Namespace), ctrlclient.MatchingLabels{kubelb.LabelManagedBy: kubelb.LabelControllerName})
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
	err = cleanupRoute(ctx, r.LBManager.GetClient(), string(ingress.UID), r.ClusterName)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to cleanup route: %w", err)
	}

	controllerutil.RemoveFinalizer(ingress, CleanupFinalizer)
	if err := r.Update(ctx, ingress); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return reconcile.Result{}, nil
}

// enqueueResources is a handler.MapFunc to be used to enqueue requests for reconciliation
// for Ingresses against the corresponding service.
func (r *IngressReconciler) enqueueResources() handler.MapFunc {
	return func(_ context.Context, o ctrlclient.Object) []ctrl.Request {
		result := []reconcile.Request{}
		ingressList := &networkingv1.IngressList{}
		if err := r.List(context.Background(), ingressList, ctrlclient.InNamespace(o.GetNamespace())); err != nil {
			return nil
		}

		for _, ingress := range ingressList.Items {
			if !r.shouldReconcile(&ingress) {
				continue
			}

			services := ingressHelpers.GetServicesFromIngress(ingress)
			for _, serviceRef := range services {
				if (serviceRef.Name == o.GetName() || fmt.Sprintf(serviceHelpers.NodePortServicePattern, serviceRef.Name) == o.GetName()) && serviceRef.Namespace == o.GetNamespace() {
					result = append(result, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      ingress.Name,
							Namespace: ingress.Namespace,
						},
					})
				}
			}
		}
		return result
	}
}

func (r *IngressReconciler) resourceFilter() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if obj, ok := e.Object.(*networkingv1.Ingress); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if obj, ok := e.ObjectNew.(*networkingv1.Ingress); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if obj, ok := e.Object.(*networkingv1.Ingress); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			if obj, ok := e.Object.(*networkingv1.Ingress); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
	}
}

func (r *IngressReconciler) shouldReconcile(ingress *networkingv1.Ingress) bool {
	if r.UseIngressClass {
		return ingress.Spec.IngressClassName != nil && *ingress.Spec.IngressClassName == IngressClassName
	}
	return true
}

func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(IngressControllerName).
		For(&networkingv1.Ingress{}, builder.WithPredicates(r.resourceFilter())).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueResources()),
		).
		WatchesRawSource(
			source.Kind(r.LBManager.GetCache(), &kubelbv1alpha1.Route{},
				handler.TypedEnqueueRequestsFromMapFunc[*kubelbv1alpha1.Route](enqueueRoutes("Ingress.networking.k8s.io", r.ClusterName))),
		).
		Complete(r)
}
