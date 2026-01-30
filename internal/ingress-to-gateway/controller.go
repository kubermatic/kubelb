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

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// LabelSourceIngress marks HTTPRoutes created from Ingress conversion (informational only)
	LabelSourceIngress = "kubelb.k8c.io/source-ingress"
)

// Reconciler reconciles Ingress objects and converts them to HTTPRoutes
type Reconciler struct {
	ctrlclient.Client

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder

	GatewayName          string
	GatewayNamespace     string
	GatewayClassName     string
	DomainReplace        string
	DomainSuffix         string
	PropagateCertManager bool
	PropagateExternalDNS bool
}

// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)

	log.V(1).Info("Reconciling Ingress for conversion")

	ingress := &networkingv1.Ingress{}
	if err := r.Get(ctx, req.NamespacedName, ingress); err != nil {
		if kerrors.IsNotFound(err) {
			// Ingress deleted - HTTPRoutes intentionally stay (migration complete)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Skip if being deleted - HTTPRoutes stay for migration
	if ingress.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	// Check if we should convert this Ingress
	if !r.shouldConvert(ingress) {
		log.V(1).Info("Skipping Ingress conversion")
		return ctrl.Result{}, nil
	}

	// Convert and reconcile
	if err := r.reconcile(ctx, log, ingress); err != nil {
		log.Error(err, "reconciliation failed")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress) error {
	// Fetch Services for port resolution
	services := r.fetchServicesForIngress(ctx, log, ingress)

	// Convert Ingress to HTTPRoutes (one per host)
	result := ConvertIngressWithServices(ConversionInput{
		Ingress:          ingress,
		GatewayName:      r.GatewayName,
		GatewayNamespace: r.GatewayNamespace,
		Services:         services,
		DomainReplace:    r.DomainReplace,
		DomainSuffix:     r.DomainSuffix,
	})

	if len(result.HTTPRoutes) == 0 {
		return fmt.Errorf("conversion produced no HTTPRoutes")
	}

	// Reconcile Gateway first (create/update with listeners and annotations)
	if err := r.reconcileGateway(ctx, log, ingress, result.TLSListeners); err != nil {
		return fmt.Errorf("failed to reconcile Gateway: %w", err)
	}

	// Extract external-dns annotations for HTTPRoutes
	externalDNSAnnotations := r.extractHTTPRouteAnnotations(ingress)

	// Track created route names for status annotation
	var routeNames []string

	for _, httpRoute := range result.HTTPRoutes {
		// Add tracking label (informational - no owner reference so HTTPRoute survives Ingress deletion)
		if httpRoute.Labels == nil {
			httpRoute.Labels = make(map[string]string)
		}
		httpRoute.Labels[LabelSourceIngress] = fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace)

		// Add external-dns annotations to HTTPRoute
		if httpRoute.Annotations == nil {
			httpRoute.Annotations = make(map[string]string)
		}
		for k, v := range externalDNSAnnotations {
			httpRoute.Annotations[k] = v
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
			existing.Annotations = httpRoute.Annotations
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

// fetchServicesForIngress fetches all Services referenced by the Ingress for port resolution
func (r *Reconciler) fetchServicesForIngress(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress) map[types.NamespacedName]*corev1.Service {
	services := make(map[types.NamespacedName]*corev1.Service)

	// Collect service names from default backend
	if ingress.Spec.DefaultBackend != nil && ingress.Spec.DefaultBackend.Service != nil {
		key := types.NamespacedName{
			Name:      ingress.Spec.DefaultBackend.Service.Name,
			Namespace: ingress.Namespace,
		}
		r.fetchService(ctx, log, key, services)
	}

	// Collect service names from rules
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				key := types.NamespacedName{
					Name:      path.Backend.Service.Name,
					Namespace: ingress.Namespace,
				}
				r.fetchService(ctx, log, key, services)
			}
		}
	}

	return services
}

func (r *Reconciler) fetchService(ctx context.Context, log logr.Logger, key types.NamespacedName, services map[types.NamespacedName]*corev1.Service) {
	if _, exists := services[key]; exists {
		return
	}

	svc := &corev1.Service{}
	if err := r.Get(ctx, key, svc); err != nil {
		if !kerrors.IsNotFound(err) {
			log.V(1).Info("Failed to fetch Service for port resolution", "service", key, "error", err)
		}
		return
	}
	services[key] = svc
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
		Complete(r)
}
