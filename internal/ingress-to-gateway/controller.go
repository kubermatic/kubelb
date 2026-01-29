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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// Convert Ingress to HTTPRoute
	result := ConvertIngress(ingress, r.GatewayName, r.GatewayNamespace)

	if result.HTTPRoute == nil {
		return fmt.Errorf("conversion produced nil HTTPRoute")
	}

	// Add tracking label
	if result.HTTPRoute.Labels == nil {
		result.HTTPRoute.Labels = make(map[string]string)
	}
	result.HTTPRoute.Labels[LabelSourceIngress] = fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace)

	// Set owner reference for garbage collection
	if err := controllerutil.SetControllerReference(ingress, result.HTTPRoute, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Create or update HTTPRoute
	existing := &gwapiv1.HTTPRoute{}
	err := r.Get(ctx, types.NamespacedName{Name: result.HTTPRoute.Name, Namespace: result.HTTPRoute.Namespace}, existing)
	if err != nil {
		if kerrors.IsNotFound(err) {
			log.Info("Creating HTTPRoute", "name", result.HTTPRoute.Name)
			if err := r.Create(ctx, result.HTTPRoute); err != nil {
				return fmt.Errorf("failed to create HTTPRoute: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get HTTPRoute: %w", err)
		}
	} else {
		// Update existing HTTPRoute
		existing.Spec = result.HTTPRoute.Spec
		existing.Labels = result.HTTPRoute.Labels
		log.Info("Updating HTTPRoute", "name", result.HTTPRoute.Name)
		if err := r.Update(ctx, existing); err != nil {
			return fmt.Errorf("failed to update HTTPRoute: %w", err)
		}
	}

	// Update Ingress annotations with conversion status
	if err := r.updateIngressStatus(ctx, ingress, result.Warnings); err != nil {
		return fmt.Errorf("failed to update Ingress status: %w", err)
	}

	log.Info("Successfully converted Ingress to HTTPRoute",
		"httproute", result.HTTPRoute.Name,
		"warnings", len(result.Warnings))

	return nil
}

func (r *Reconciler) updateIngressStatus(ctx context.Context, ingress *networkingv1.Ingress, warnings []string) error {
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
	current.Annotations[AnnotationConvertedHTTPRoute] = fmt.Sprintf("%s/%s", ingress.Namespace, ingress.Name)

	return r.Update(ctx, current)
}

func (r *Reconciler) cleanup(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress) (ctrl.Result, error) {
	log.Info("Cleaning up HTTPRoute for deleted Ingress")

	// Delete the HTTPRoute
	httpRoute := &gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingress.Name,
			Namespace: ingress.Namespace,
		},
	}

	if err := r.Delete(ctx, httpRoute); err != nil && !kerrors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("failed to delete HTTPRoute: %w", err)
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(ingress, FinalizerName)
	if err := r.Update(ctx, ingress); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	log.Info("Cleanup complete")
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
