/*
Copyright 2026 The KubeLB Authors.

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

package ingressconversion

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"

	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// FinalizerName is the finalizer used by this controller
	FinalizerName = "kubelb.k8c.io/ingress-conversion"

	// LabelSourceIngress marks HTTPRoutes created by this controller
	LabelSourceIngress = "kubelb.k8c.io/source-ingress"
)

// Reconciler reconciles Ingress objects and converts them to HTTPRoutes
type Reconciler struct {
	ctrlclient.Client

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder

	GatewayName      string
	GatewayNamespace string
	GatewayClassName string
	DomainSuffix     string
}

// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)

	log.V(1).Info("Reconciling Ingress for conversion")

	ingress := &networkingv1.Ingress{}
	if err := r.Get(ctx, req.NamespacedName, ingress); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Handle deletion
	if ingress.DeletionTimestamp != nil {
		if controllerutil.ContainsFinalizer(ingress, FinalizerName) {
			return r.cleanup(ctx, log, ingress)
		}
		return reconcile.Result{}, nil
	}

	// Check if we should convert this Ingress
	if !r.shouldConvert(ingress) {
		log.V(1).Info("Skipping Ingress conversion")
		return reconcile.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(ingress, FinalizerName) {
		controllerutil.AddFinalizer(ingress, FinalizerName)
		if err := r.Update(ctx, ingress); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	// Convert and reconcile
	if err := r.reconcile(ctx, log, ingress); err != nil {
		log.Error(err, "reconciliation failed")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress) error {
	// Convert Ingress to HTTPRoutes (one per host)
	result := ConvertIngress(ingress, r.GatewayName, r.GatewayNamespace)

	if len(result.HTTPRoutes) == 0 {
		return fmt.Errorf("conversion produced no HTTPRoutes")
	}

	// Track created route names for status annotation
	var routeNames []string

	for _, httpRoute := range result.HTTPRoutes {
		// Add tracking label
		if httpRoute.Labels == nil {
			httpRoute.Labels = make(map[string]string)
		}
		httpRoute.Labels[LabelSourceIngress] = fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace)

		// Set owner reference for garbage collection
		if err := controllerutil.SetControllerReference(ingress, httpRoute, r.Scheme); err != nil {
			return fmt.Errorf("failed to set owner reference: %w", err)
		}

		// Create or update HTTPRoute
		existing := &gwapiv1.HTTPRoute{}
		err := r.Get(ctx, types.NamespacedName{Name: httpRoute.Name, Namespace: httpRoute.Namespace}, existing)
		if err != nil {
			if kerrors.IsNotFound(err) {
				log.Info("Creating HTTPRoute", "name", httpRoute.Name)
				if err := r.Create(ctx, httpRoute); err != nil {
					return fmt.Errorf("failed to create HTTPRoute: %w", err)
				}
			} else {
				return fmt.Errorf("failed to get HTTPRoute: %w", err)
			}
		} else {
			// Update existing HTTPRoute
			existing.Spec = httpRoute.Spec
			existing.Labels = httpRoute.Labels
			log.Info("Updating HTTPRoute", "name", httpRoute.Name)
			if err := r.Update(ctx, existing); err != nil {
				return fmt.Errorf("failed to update HTTPRoute: %w", err)
			}
		}

		routeNames = append(routeNames, fmt.Sprintf("%s/%s", httpRoute.Namespace, httpRoute.Name))
	}

	// Update Ingress annotations with conversion status
	if err := r.updateIngressStatus(ctx, ingress, result.Warnings, routeNames); err != nil {
		return fmt.Errorf("failed to update Ingress status: %w", err)
	}

	log.Info("Successfully converted Ingress to HTTPRoutes",
		"httproutes", len(result.HTTPRoutes),
		"warnings", len(result.Warnings))

	return nil
}

func (r *Reconciler) updateIngressStatus(ctx context.Context, ingress *networkingv1.Ingress, warnings, routeNames []string) error {
	// Re-fetch to avoid conflicts
	current := &networkingv1.Ingress{}
	if err := r.Get(ctx, types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, current); err != nil {
		return err
	}

	if current.Annotations == nil {
		current.Annotations = make(map[string]string)
	}

	status := ConversionStatusConverted
	if len(warnings) > 0 {
		status = ConversionStatusPartial
		current.Annotations[AnnotationConversionWarnings] = strings.Join(warnings, "; ")
	}

	current.Annotations[AnnotationConversionStatus] = status
	current.Annotations[AnnotationConvertedHTTPRoute] = strings.Join(routeNames, ",")

	return r.Update(ctx, current)
}

func (r *Reconciler) cleanup(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress) (ctrl.Result, error) {
	log.Info("Cleaning up HTTPRoutes for deleted Ingress")

	// Delete all HTTPRoutes owned by this Ingress via label selector
	routeList := &gwapiv1.HTTPRouteList{}
	labelSelector := ctrlclient.MatchingLabels{
		LabelSourceIngress: fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace),
	}
	if err := r.List(ctx, routeList, ctrlclient.InNamespace(ingress.Namespace), labelSelector); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list HTTPRoutes: %w", err)
	}

	for i := range routeList.Items {
		route := &routeList.Items[i]
		log.V(1).Info("Deleting HTTPRoute", "name", route.Name)
		if err := r.Delete(ctx, route); err != nil && !kerrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("failed to delete HTTPRoute %s: %w", route.Name, err)
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(ingress, FinalizerName)
	if err := r.Update(ctx, ingress); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	log.Info("Cleanup complete", "deleted", len(routeList.Items))
	return reconcile.Result{}, nil
}

// shouldConvert determines if an Ingress should be converted to HTTPRoute
func (r *Reconciler) shouldConvert(ingress *networkingv1.Ingress) bool {
	annotations := ingress.GetAnnotations()
	if annotations == nil {
		return true
	}

	// Skip if explicitly marked to skip conversion
	if annotations[AnnotationSkipConversion] == "true" {
		return false
	}

	// Skip canary Ingresses (NGINX-specific feature not supported in Gateway API)
	if annotations[NginxCanary] == "true" {
		return false
	}

	return true
}

func (r *Reconciler) resourceFilter() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if obj, ok := e.Object.(*networkingv1.Ingress); ok {
				return r.shouldConvert(obj)
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if obj, ok := e.ObjectNew.(*networkingv1.Ingress); ok {
				return r.shouldConvert(obj)
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Always process deletes to clean up HTTPRoutes
			_, ok := e.Object.(*networkingv1.Ingress)
			return ok
		},
		GenericFunc: func(e event.GenericEvent) bool {
			if obj, ok := e.Object.(*networkingv1.Ingress); ok {
				return r.shouldConvert(obj)
			}
			return false
		},
	}
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&networkingv1.Ingress{}, builder.WithPredicates(r.resourceFilter())).
		Owns(&gwapiv1.HTTPRoute{}).
		Complete(r)
}
