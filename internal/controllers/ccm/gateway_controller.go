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
	GatewayControllerName = "gateway-controller"

	// EventReasonGatewayClassNotAccepted is the Event reason used to tell a
	// tenant that their Gateway names a GatewayClass KubeLB does not serve.
	// Gateway API defines no condition reason for "no controller claims this
	// class", so the signal is an Event rather than a status write.
	EventReasonGatewayClassNotAccepted = "GatewayClassNotAccepted"
)

// GatewayReconciler reconciles a Gateway Object
type GatewayReconciler struct {
	ctrlclient.Client

	LBManager       ctrl.Manager
	ClusterName     string
	UseGatewayClass bool

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gatewayclasses,verbs=get;list;watch

func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)
	startTime := time.Now()

	// Track reconciliation duration
	defer func() {
		ccmmetrics.GatewayReconcileDuration.WithLabelValues(req.Namespace).Observe(time.Since(startTime).Seconds())
	}()

	log.Info("Reconciling Gateway")

	resource := &gwapiv1.Gateway{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		ccmmetrics.GatewayReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
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
		r.warnGatewayClassNotAccepted(ctx, log, resource)
		ccmmetrics.GatewayReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSkipped).Inc()
		return reconcile.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(resource, CleanupFinalizer) {
		if ok := controllerutil.AddFinalizer(resource, CleanupFinalizer); !ok {
			log.Error(nil, "Failed to add finalizer for the Gateway")
			return ctrl.Result{Requeue: true}, nil //nolint:staticcheck // SA1019
		}

		if err := r.Update(ctx, resource); err != nil {
			ccmmetrics.GatewayReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	err := r.reconcile(ctx, log, resource)
	if err != nil {
		log.Error(err, "reconciling failed")
		ccmmetrics.GatewayReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return reconcile.Result{}, err
	}

	// Update managed gateways gauge
	gatewayList := &gwapiv1.GatewayList{}
	if err := r.List(ctx, gatewayList, ctrlclient.InNamespace(req.Namespace)); err == nil {
		count := 0
		for _, gw := range gatewayList.Items {
			if r.shouldReconcile(&gw) && gw.DeletionTimestamp == nil {
				count++
			}
		}
		ccmmetrics.ManagedGatewaysTotal.WithLabelValues(req.Namespace).Set(float64(count))
	}

	ccmmetrics.GatewayReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSuccess).Inc()
	return reconcile.Result{}, nil
}

func (r *GatewayReconciler) reconcile(ctx context.Context, log logr.Logger, gateway *gwapiv1.Gateway) error {
	// Create/update the corresponding Route in LB cluster.
	err := reconcileSourceForRoute(ctx, log, r.Client, r.LBManager.GetClient(), gateway, nil, r.ClusterName)
	if err != nil {
		return fmt.Errorf("failed to reconcile source for route: %w", err)
	}

	// Route was reconciled successfully, now we need to update the status of the Resource.
	route := kubelbv1alpha1.Route{}
	err = r.LBManager.GetClient().Get(ctx, types.NamespacedName{Name: string(gateway.UID), Namespace: r.ClusterName}, &route)
	if err != nil {
		return fmt.Errorf("failed to get Route from LB cluster: %w", err)
	}

	// Update the status of the Resource
	if len(route.Status.Resources.Route.GeneratedName) > 0 {
		// First we need to ensure that status is available in the Route
		resourceStatus := route.Status.Resources.Route.Status
		jsonData, err := json.Marshal(resourceStatus.Raw)
		if err != nil || string(jsonData) == kubelb.DefaultRouteStatus {
			// Status is not available in the Route, so we need to wait for it
			return nil
		}

		// Convert rawExtension to gwapiv1.GatewayStatus
		status := gwapiv1.GatewayStatus{}
		if err := yaml.UnmarshalStrict(resourceStatus.Raw, &status); err != nil {
			return fmt.Errorf("failed to unmarshal Gateway status: %w", err)
		}

		log.V(3).Info("updating Gateway status", "name", gateway.Name, "namespace", gateway.Namespace)
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}, gateway); err != nil {
				return err
			}
			original := gateway.DeepCopy()
			for i := range status.Conditions {
				status.Conditions[i].ObservedGeneration = gateway.Generation
			}
			for i := range status.Listeners {
				for j := range status.Listeners[i].Conditions {
					status.Listeners[i].Conditions[j].ObservedGeneration = gateway.Generation
				}
			}
			gateway.Status = status
			if reflect.DeepEqual(original.Status, gateway.Status) {
				return nil
			}
			// update the status
			return r.Status().Patch(ctx, gateway, ctrlclient.MergeFrom(original))
		})
	}
	return nil
}

func (r *GatewayReconciler) cleanup(ctx context.Context, gateway *gwapiv1.Gateway) (ctrl.Result, error) {
	// Find the Route in LB cluster and delete it
	err := cleanupRoute(ctx, r.LBManager.GetClient(), string(gateway.UID), r.ClusterName)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to cleanup route: %w", err)
	}

	controllerutil.RemoveFinalizer(gateway, CleanupFinalizer)
	if err := r.Update(ctx, gateway); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *GatewayReconciler) resourceFilter() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if obj, ok := e.Object.(*gwapiv1.Gateway); ok {
				return r.shouldObserve(obj)
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj, okOld := e.ObjectOld.(*gwapiv1.Gateway)
			newObj, okNew := e.ObjectNew.(*gwapiv1.Gateway)
			if !okOld || !okNew {
				return false
			}
			if r.shouldReconcile(newObj) {
				return true
			}
			// Gateways of a foreign controller are only admitted when their spec
			// moved. Those controllers rewrite Gateway status continuously and
			// none of that can change whether KubeLB serves the class.
			return r.shouldObserve(newObj) && oldObj.Generation != newObj.Generation
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if obj, ok := e.Object.(*gwapiv1.Gateway); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			if obj, ok := e.Object.(*gwapiv1.Gateway); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
	}
}

// shouldReconcile checks if the Gateway should be reconciled by the controller.
// In Community Edition, only a single Gateway with the name "kubelb" is reconciled.
func (r *GatewayReconciler) shouldReconcile(gateway *gwapiv1.Gateway) bool {
	return gatewayhelper.ShouldReconcileResource(gateway, r.UseGatewayClass)
}

// shouldObserve widens shouldReconcile to the Gateways that carry KubeLB's own
// name but a GatewayClass it does not serve. Reconcile has to see them to warn
// their owner, and only Reconcile can afford the GatewayClass lookup that
// decides whether the warning is KubeLB's to emit.
func (r *GatewayReconciler) shouldObserve(gateway *gwapiv1.Gateway) bool {
	return r.shouldReconcile(gateway) || r.droppedOnGatewayClass(gateway)
}

// droppedOnGatewayClass reports whether the only thing keeping this Gateway out
// of the reconcile path is its GatewayClass. A Gateway with a different name is
// not KubeLB's to comment on, whatever class it names.
func (r *GatewayReconciler) droppedOnGatewayClass(gateway *gwapiv1.Gateway) bool {
	return r.UseGatewayClass &&
		gateway.Name == gatewayhelper.ParentGatewayName &&
		string(gateway.Spec.GatewayClassName) != gatewayhelper.GatewayClassName
}

// warnGatewayClassNotAccepted tells the tenant why their Gateway is being
// ignored, instead of leaving it at the Gateway API default status of
// "Accepted: Unknown, Pending" that is indistinguishable from a dead
// controller.
//
// The signal is an Event and never a status write: Gateway API reserves the
// status of a Gateway for the controller named by its GatewayClass, and KubeLB
// registers no GatewayClass and no controllerName in the tenant cluster, so it
// can never be that controller for a class it does not serve. An Event is
// additive and cannot fight whichever controller does own the class.
//
// A Gateway KubeLB never adopted stays untouched while a GatewayClass object of
// that name exists, because that object hands the Gateway to another controller
// and the warning would be both wrong and confusing there.
func (r *GatewayReconciler) warnGatewayClassNotAccepted(ctx context.Context, log logr.Logger, gateway *gwapiv1.Gateway) {
	if r.Recorder == nil || !r.droppedOnGatewayClass(gateway) {
		return
	}

	class := string(gateway.Spec.GatewayClassName)
	adopted := controllerutil.ContainsFinalizer(gateway, CleanupFinalizer)
	if !adopted && gatewayClassClaimed(ctx, r.Client, class) {
		return
	}

	log.Info("Gateway names a GatewayClass that KubeLB does not serve", "gatewayClass", class, "adopted", adopted)

	note := "GatewayClass %q is not served by KubeLB, this Gateway is ignored and no load balancer is provisioned for it. Served GatewayClass: %s"
	if adopted {
		note = "GatewayClass %q is no longer served by KubeLB, the load balancer configuration for this Gateway is being removed. Served GatewayClass: %s"
	}
	r.Recorder.Eventf(gateway, nil, corev1.EventTypeWarning, EventReasonGatewayClassNotAccepted, "Reconciling", note, class, gatewayhelper.GatewayClassName)
}

// gatewayClassClaimed reports whether a GatewayClass object of that name exists
// in the tenant cluster. KubeLB registers no GatewayClass and no controllerName
// there, it matches on the class name alone, so a GatewayClass that does exist
// always designates a different controller and its Gateways are that
// controller's to accept or reject. A failed lookup counts as claimed so that
// KubeLB never warns on a Gateway it cannot prove is unserved.
func gatewayClassClaimed(ctx context.Context, reader ctrlclient.Reader, name string) bool {
	if name == "" {
		return false
	}
	class := &gwapiv1.GatewayClass{}
	if err := reader.Get(ctx, ctrlclient.ObjectKey{Name: name}, class); err != nil {
		return !kerrors.IsNotFound(err)
	}
	return true
}

func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(GatewayControllerName).
		For(&gwapiv1.Gateway{}, builder.WithPredicates(r.resourceFilter())).
		WatchesRawSource(
			source.Kind(r.LBManager.GetCache(), &kubelbv1alpha1.Route{},
				handler.TypedEnqueueRequestsFromMapFunc[*kubelbv1alpha1.Route](enqueueRoutes("Gateway.gateway.networking.k8s.io", r.ClusterName))),
		).
		Complete(r)
}
