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
	"time"

	"github.com/go-logr/logr"

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"

	"k8c.io/kubelb/internal/ingress-to-gateway/annotations"
	"k8c.io/kubelb/internal/ingress-to-gateway/policies"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Reconciler reconciles Ingress objects and converts them to HTTPRoutes
type Reconciler struct {
	ctrlclient.Client

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	GatewayName                 string
	GatewayNamespace            string
	GatewayClassName            string
	IngressClass                string
	DomainReplace               string
	DomainSuffix                string
	PropagateExternalDNS        bool
	CleanupStale                bool
	GatewayAnnotations          map[string]string
	DisableEnvoyGatewayFeatures bool
}

// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=grpcroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.envoyproxy.io,resources=securitypolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.envoyproxy.io,resources=backendtrafficpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.envoyproxy.io,resources=clienttrafficpolicies,verbs=get;list;watch;create;update;patch;delete

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)

	log.V(1).Info("Reconciling Ingress for conversion")

	ingress := &networkingv1.Ingress{}
	if err := r.Get(ctx, req.NamespacedName, ingress); err != nil {
		if kerrors.IsNotFound(err) {
			// Ingress deleted - cleanup routes if CleanupStale enabled
			if r.CleanupStale {
				if err := r.cleanupRoutesForDeletedIngress(ctx, log, req.NamespacedName); err != nil {
					return ctrl.Result{}, err
				}
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// If being deleted and cleanup enabled, clean up routes
	if ingress.DeletionTimestamp != nil {
		if r.CleanupStale {
			if err := r.cleanupRoutesForDeletedIngress(ctx, log, req.NamespacedName); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Check if we should convert this Ingress
	decision := r.shouldConvert(ingress)
	if !decision.shouldConvert {
		log.V(1).Info("Skipping Ingress conversion", "reason", decision.skipReason)
		if decision.annotate {
			if err := r.annotateSkipped(ctx, ingress, decision.skipReason); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Convert and reconcile
	requeue, err := r.reconcile(ctx, log, ingress)
	if err != nil {
		log.Error(err, "reconciliation failed")
		return ctrl.Result{}, err
	}

	if requeue {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress) (requeue bool, err error) {
	// Fetch Services for port resolution
	services := r.fetchServicesForIngress(ctx, log, ingress)

	// Convert Ingress to HTTPRoutes or GRPCRoutes (one per host)
	result := ConvertIngressWithServices(ConversionInput{
		Ingress:          ingress,
		GatewayName:      r.GatewayName,
		GatewayNamespace: r.GatewayNamespace,
		Services:         services,
		DomainReplace:    r.DomainReplace,
		DomainSuffix:     r.DomainSuffix,
	})

	if len(result.HTTPRoutes) == 0 && len(result.GRPCRoutes) == 0 {
		return false, fmt.Errorf("conversion produced no routes")
	}

	// Reconcile Gateway first (create/update with listeners and annotations)
	if err := r.reconcileGateway(ctx, log, ingress, result.TLSListeners); err != nil {
		return false, fmt.Errorf("failed to reconcile Gateway: %w", err)
	}

	// Extract external-dns annotations for routes
	externalDNSAnnotations := r.extractHTTPRouteAnnotations(ingress)

	// Track created route names for status annotation
	var httpRouteNames []string
	var grpcRouteNames []string

	// Reconcile HTTPRoutes
	for _, httpRoute := range result.HTTPRoutes {
		if httpRoute.Labels == nil {
			httpRoute.Labels = make(map[string]string)
		}
		httpRoute.Labels[policies.LabelSourceIngress] = fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace)

		if httpRoute.Annotations == nil {
			httpRoute.Annotations = make(map[string]string)
		}
		for k, v := range externalDNSAnnotations {
			httpRoute.Annotations[k] = v
		}

		existing := &gwapiv1.HTTPRoute{}
		err := r.Get(ctx, types.NamespacedName{Name: httpRoute.Name, Namespace: httpRoute.Namespace}, existing)
		if err != nil {
			if kerrors.IsNotFound(err) {
				log.Info("Creating HTTPRoute", "name", httpRoute.Name)
				if err := r.Create(ctx, httpRoute); err != nil {
					return false, fmt.Errorf("failed to create HTTPRoute: %w", err)
				}
			} else {
				return false, fmt.Errorf("failed to get HTTPRoute: %w", err)
			}
		} else {
			existing.Spec = httpRoute.Spec
			existing.Labels = httpRoute.Labels
			existing.Annotations = httpRoute.Annotations
			log.Info("Updating HTTPRoute", "name", httpRoute.Name)
			if err := r.Update(ctx, existing); err != nil {
				return false, fmt.Errorf("failed to update HTTPRoute: %w", err)
			}
		}

		httpRouteNames = append(httpRouteNames, fmt.Sprintf("%s/%s", httpRoute.Namespace, httpRoute.Name))
	}

	// Reconcile GRPCRoutes
	for _, grpcRoute := range result.GRPCRoutes {
		if grpcRoute.Labels == nil {
			grpcRoute.Labels = make(map[string]string)
		}
		grpcRoute.Labels[policies.LabelSourceIngress] = fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace)

		if grpcRoute.Annotations == nil {
			grpcRoute.Annotations = make(map[string]string)
		}
		for k, v := range externalDNSAnnotations {
			grpcRoute.Annotations[k] = v
		}

		existing := &gwapiv1.GRPCRoute{}
		err := r.Get(ctx, types.NamespacedName{Name: grpcRoute.Name, Namespace: grpcRoute.Namespace}, existing)
		if err != nil {
			if kerrors.IsNotFound(err) {
				log.Info("Creating GRPCRoute", "name", grpcRoute.Name)
				if err := r.Create(ctx, grpcRoute); err != nil {
					return false, fmt.Errorf("failed to create GRPCRoute: %w", err)
				}
			} else {
				return false, fmt.Errorf("failed to get GRPCRoute: %w", err)
			}
		} else {
			existing.Spec = grpcRoute.Spec
			existing.Labels = grpcRoute.Labels
			existing.Annotations = grpcRoute.Annotations
			log.Info("Updating GRPCRoute", "name", grpcRoute.Name)
			if err := r.Update(ctx, existing); err != nil {
				return false, fmt.Errorf("failed to update GRPCRoute: %w", err)
			}
		}

		grpcRouteNames = append(grpcRouteNames, fmt.Sprintf("%s/%s", grpcRoute.Namespace, grpcRoute.Name))
	}

	// Clean up stale routes (hosts removed from Ingress)
	if r.CleanupStale {
		if err := r.cleanupStaleHTTPRoutes(ctx, log, ingress, httpRouteNames); err != nil {
			return false, fmt.Errorf("failed to cleanup stale HTTPRoutes: %w", err)
		}
		if err := r.cleanupStaleGRPCRoutes(ctx, log, ingress, grpcRouteNames); err != nil {
			return false, fmt.Errorf("failed to cleanup stale GRPCRoutes: %w", err)
		}
	}

	// Reconcile Envoy Gateway policies (if enabled)
	var policyWarnings []string
	if !r.DisableEnvoyGatewayFeatures && len(result.HTTPRoutes) > 0 {
		var err error
		policyWarnings, err = r.reconcilePolicies(ctx, log, ingress, result.HTTPRoutes[0].Name)
		if err != nil {
			log.Error(err, "Failed to reconcile Envoy Gateway policies")
			// Don't fail the reconciliation, just add warning
			policyWarnings = append(policyWarnings, fmt.Sprintf("failed to reconcile policies: %v", err))
		}
	}

	// Verify route acceptance (staged validation)
	allWarnings := append([]string{}, result.Warnings...)
	allWarnings = append(allWarnings, policyWarnings...)
	routeAcceptance := make(map[string]bool)
	hasTimeout := false

	for _, httpRoute := range result.HTTPRoutes {
		acceptance := r.waitForRouteAcceptance(ctx, httpRoute.Name, httpRoute.Namespace)
		routeAcceptance[httpRoute.Name] = acceptance.accepted
		if !acceptance.accepted {
			if acceptance.reason == "Timeout" {
				hasTimeout = true
				log.V(1).Info("HTTPRoute acceptance timed out", "name", httpRoute.Name)
			} else {
				allWarnings = append(allWarnings, fmt.Sprintf("HTTPRoute %s not accepted: %s", httpRoute.Name, acceptance.reason))
				r.Recorder.Eventf(ingress, corev1.EventTypeWarning, "RouteNotAccepted",
					"HTTPRoute %s not accepted by Gateway: %s", httpRoute.Name, acceptance.reason)
			}
		}
	}

	for _, grpcRoute := range result.GRPCRoutes {
		acceptance := r.waitForGRPCRouteAcceptance(ctx, grpcRoute.Name, grpcRoute.Namespace)
		routeAcceptance[grpcRoute.Name] = acceptance.accepted
		if !acceptance.accepted {
			if acceptance.reason == "Timeout" {
				hasTimeout = true
				log.V(1).Info("GRPCRoute acceptance timed out", "name", grpcRoute.Name)
			} else {
				allWarnings = append(allWarnings, fmt.Sprintf("GRPCRoute %s not accepted: %s", grpcRoute.Name, acceptance.reason))
				r.Recorder.Eventf(ingress, corev1.EventTypeWarning, "RouteNotAccepted",
					"GRPCRoute %s not accepted by Gateway: %s", grpcRoute.Name, acceptance.reason)
			}
		}
	}

	// Update Ingress annotations with conversion status
	if err := r.updateIngressStatusStaged(ctx, ingress, allWarnings, httpRouteNames, grpcRouteNames, routeAcceptance, hasTimeout); err != nil {
		return false, fmt.Errorf("failed to update Ingress status: %w", err)
	}

	// Emit events for warnings
	r.emitWarningEvents(ingress, allWarnings)

	log.Info("Successfully converted Ingress",
		"httproutes", len(result.HTTPRoutes),
		"grpcroutes", len(result.GRPCRoutes),
		"warnings", len(allWarnings))

	// Requeue if routes timed out waiting for acceptance
	return hasTimeout, nil
}

func (r *Reconciler) updateIngressStatusStaged(ctx context.Context, ingress *networkingv1.Ingress, warnings, httpRouteNames, grpcRouteNames []string, routeAcceptance map[string]bool, hasTimeout bool) error {
	// Re-fetch to avoid conflicts
	current := &networkingv1.Ingress{}
	if err := r.Get(ctx, types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, current); err != nil {
		return err
	}

	if current.Annotations == nil {
		current.Annotations = make(map[string]string)
	}

	// Determine status based on acceptance results
	allAccepted := true
	for _, accepted := range routeAcceptance {
		if !accepted {
			allAccepted = false
			break
		}
	}

	var status string
	switch {
	case hasTimeout:
		status = ConversionStatusPending
	case allAccepted && len(warnings) == 0:
		status = ConversionStatusConverted
	default:
		status = ConversionStatusPartial
	}

	if len(warnings) > 0 {
		current.Annotations[AnnotationConversionWarnings] = strings.Join(warnings, "; ")
	} else {
		delete(current.Annotations, AnnotationConversionWarnings)
	}

	current.Annotations[AnnotationConversionStatus] = status

	if len(httpRouteNames) > 0 {
		current.Annotations[AnnotationConvertedHTTPRoute] = strings.Join(httpRouteNames, ",")
	} else {
		delete(current.Annotations, AnnotationConvertedHTTPRoute)
	}

	if len(grpcRouteNames) > 0 {
		current.Annotations[AnnotationConvertedGRPCRoute] = strings.Join(grpcRouteNames, ",")
	} else {
		delete(current.Annotations, AnnotationConvertedGRPCRoute)
	}

	return r.Update(ctx, current)
}

// cleanupStaleHTTPRoutes removes HTTPRoutes that were created from this Ingress but are no longer desired
func (r *Reconciler) cleanupStaleHTTPRoutes(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress, desiredRouteNames []string) error {
	// Build set of desired route names for quick lookup
	desired := make(map[string]bool, len(desiredRouteNames))
	for _, name := range desiredRouteNames {
		desired[name] = true
	}

	// List all HTTPRoutes with source label pointing to this Ingress
	sourceLabel := fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace)
	httpRouteList := &gwapiv1.HTTPRouteList{}
	if err := r.List(ctx, httpRouteList,
		ctrlclient.InNamespace(ingress.Namespace),
		ctrlclient.MatchingLabels{policies.LabelSourceIngress: sourceLabel}); err != nil {
		return fmt.Errorf("failed to list HTTPRoutes: %w", err)
	}

	// Delete any HTTPRoutes not in desired set
	for i := range httpRouteList.Items {
		route := &httpRouteList.Items[i]
		routeKey := fmt.Sprintf("%s/%s", route.Namespace, route.Name)
		if !desired[routeKey] {
			log.Info("Deleting stale HTTPRoute", "name", route.Name, "namespace", route.Namespace)
			if err := r.Delete(ctx, route); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to delete stale HTTPRoute %s: %w", routeKey, err)
				}
			}
		}
	}

	return nil
}

// cleanupStaleGRPCRoutes removes GRPCRoutes that were created from this Ingress but are no longer desired
func (r *Reconciler) cleanupStaleGRPCRoutes(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress, desiredRouteNames []string) error {
	// Build set of desired route names for quick lookup
	desired := make(map[string]bool, len(desiredRouteNames))
	for _, name := range desiredRouteNames {
		desired[name] = true
	}

	// List all GRPCRoutes with source label pointing to this Ingress
	sourceLabel := fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace)
	grpcRouteList := &gwapiv1.GRPCRouteList{}
	if err := r.List(ctx, grpcRouteList,
		ctrlclient.InNamespace(ingress.Namespace),
		ctrlclient.MatchingLabels{policies.LabelSourceIngress: sourceLabel}); err != nil {
		return fmt.Errorf("failed to list GRPCRoutes: %w", err)
	}

	// Delete any GRPCRoutes not in desired set
	for i := range grpcRouteList.Items {
		route := &grpcRouteList.Items[i]
		routeKey := fmt.Sprintf("%s/%s", route.Namespace, route.Name)
		if !desired[routeKey] {
			log.Info("Deleting stale GRPCRoute", "name", route.Name, "namespace", route.Namespace)
			if err := r.Delete(ctx, route); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to delete stale GRPCRoute %s: %w", routeKey, err)
				}
			}
		}
	}

	return nil
}

// cleanupRoutesForDeletedIngress removes all routes and policies created from a deleted Ingress
func (r *Reconciler) cleanupRoutesForDeletedIngress(ctx context.Context, log logr.Logger, ingressKey types.NamespacedName) error {
	sourceLabel := fmt.Sprintf("%s.%s", ingressKey.Name, ingressKey.Namespace)

	// Delete all HTTPRoutes with this source label
	httpRouteList := &gwapiv1.HTTPRouteList{}
	if err := r.List(ctx, httpRouteList,
		ctrlclient.InNamespace(ingressKey.Namespace),
		ctrlclient.MatchingLabels{policies.LabelSourceIngress: sourceLabel}); err != nil {
		return fmt.Errorf("failed to list HTTPRoutes for cleanup: %w", err)
	}

	for i := range httpRouteList.Items {
		route := &httpRouteList.Items[i]
		log.Info("Deleting HTTPRoute for deleted Ingress", "name", route.Name, "namespace", route.Namespace)
		if err := r.Delete(ctx, route); err != nil {
			if !kerrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete HTTPRoute %s/%s: %w", route.Namespace, route.Name, err)
			}
		}
	}

	// Delete all GRPCRoutes with this source label
	grpcRouteList := &gwapiv1.GRPCRouteList{}
	if err := r.List(ctx, grpcRouteList,
		ctrlclient.InNamespace(ingressKey.Namespace),
		ctrlclient.MatchingLabels{policies.LabelSourceIngress: sourceLabel}); err != nil {
		return fmt.Errorf("failed to list GRPCRoutes for cleanup: %w", err)
	}

	for i := range grpcRouteList.Items {
		route := &grpcRouteList.Items[i]
		log.Info("Deleting GRPCRoute for deleted Ingress", "name", route.Name, "namespace", route.Namespace)
		if err := r.Delete(ctx, route); err != nil {
			if !kerrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete GRPCRoute %s/%s: %w", route.Namespace, route.Name, err)
			}
		}
	}

	// Delete all Envoy Gateway policies with this source label (if enabled)
	if !r.DisableEnvoyGatewayFeatures {
		if err := r.cleanupPoliciesForDeletedIngress(ctx, log, ingressKey); err != nil {
			return fmt.Errorf("failed to cleanup policies for deleted Ingress: %w", err)
		}
	}

	return nil
}

// cleanupPoliciesForDeletedIngress removes all policies created from a deleted Ingress
func (r *Reconciler) cleanupPoliciesForDeletedIngress(ctx context.Context, log logr.Logger, ingressKey types.NamespacedName) error {
	sourceLabel := fmt.Sprintf("%s.%s", ingressKey.Name, ingressKey.Namespace)

	// Delete all SecurityPolicies
	securityPolicies := &egv1alpha1.SecurityPolicyList{}
	if err := r.List(ctx, securityPolicies,
		ctrlclient.InNamespace(ingressKey.Namespace),
		ctrlclient.MatchingLabels{policies.LabelSourceIngress: sourceLabel}); err != nil {
		return fmt.Errorf("failed to list SecurityPolicies for cleanup: %w", err)
	}
	for i := range securityPolicies.Items {
		policy := &securityPolicies.Items[i]
		log.Info("Deleting SecurityPolicy for deleted Ingress", "name", policy.Name)
		if err := r.Delete(ctx, policy); err != nil && !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete SecurityPolicy %s: %w", policy.Name, err)
		}
	}

	// Delete all BackendTrafficPolicies
	backendPolicies := &egv1alpha1.BackendTrafficPolicyList{}
	if err := r.List(ctx, backendPolicies,
		ctrlclient.InNamespace(ingressKey.Namespace),
		ctrlclient.MatchingLabels{policies.LabelSourceIngress: sourceLabel}); err != nil {
		return fmt.Errorf("failed to list BackendTrafficPolicies for cleanup: %w", err)
	}
	for i := range backendPolicies.Items {
		policy := &backendPolicies.Items[i]
		log.Info("Deleting BackendTrafficPolicy for deleted Ingress", "name", policy.Name)
		if err := r.Delete(ctx, policy); err != nil && !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete BackendTrafficPolicy %s: %w", policy.Name, err)
		}
	}

	// Delete all ClientTrafficPolicies
	clientPolicies := &egv1alpha1.ClientTrafficPolicyList{}
	if err := r.List(ctx, clientPolicies,
		ctrlclient.InNamespace(ingressKey.Namespace),
		ctrlclient.MatchingLabels{policies.LabelSourceIngress: sourceLabel}); err != nil {
		return fmt.Errorf("failed to list ClientTrafficPolicies for cleanup: %w", err)
	}
	for i := range clientPolicies.Items {
		policy := &clientPolicies.Items[i]
		log.Info("Deleting ClientTrafficPolicy for deleted Ingress", "name", policy.Name)
		if err := r.Delete(ctx, policy); err != nil && !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ClientTrafficPolicy %s: %w", policy.Name, err)
		}
	}

	return nil
}

// reconcilePolicies creates or updates Envoy Gateway policies for the Ingress.
//
//nolint:gocyclo // complexity is due to handling 3 policy types with similar patterns
func (r *Reconciler) reconcilePolicies(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress, routeName string) ([]string, error) {
	// Convert annotations to policies
	policyResult := annotations.ConvertToPolicies(annotations.PolicyConversionInput{
		IngressName:      ingress.Name,
		IngressNamespace: ingress.Namespace,
		RouteName:        routeName,
		GatewayName:      r.GatewayName,
		Annotations:      ingress.Annotations,
	})

	var policyNames []string

	// Fetch the HTTPRoute to use as owner reference
	// Policies should be owned by the route (not Ingress) so they persist after migration
	httpRoute := &gwapiv1.HTTPRoute{}
	routeErr := r.Get(ctx, types.NamespacedName{Name: routeName, Namespace: ingress.Namespace}, httpRoute)
	if routeErr != nil && !kerrors.IsNotFound(routeErr) {
		log.Error(routeErr, "Failed to get HTTPRoute for owner reference", "route", routeName)
	}

	// Reconcile SecurityPolicies
	for _, policy := range policyResult.SecurityPolicies {
		if policy == nil {
			continue
		}
		if policy.Labels == nil {
			policy.Labels = make(map[string]string)
		}
		policy.Labels[policies.LabelSourceIngress] = fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace)

		// Set owner reference to HTTPRoute for automatic GC cleanup
		// Route owns policies so they share lifecycle (not tied to Ingress which gets deleted after migration)
		if routeErr == nil {
			if err := controllerutil.SetControllerReference(httpRoute, policy, r.Scheme); err != nil {
				log.Error(err, "Failed to set owner reference on SecurityPolicy", "name", policy.Name)
			}
		}

		existing := &egv1alpha1.SecurityPolicy{}
		err := r.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, existing)
		if err != nil {
			if kerrors.IsNotFound(err) {
				log.Info("Creating SecurityPolicy", "name", policy.Name)
				if err := r.Create(ctx, policy); err != nil {
					return policyResult.Warnings, fmt.Errorf("failed to create SecurityPolicy: %w", err)
				}
			} else {
				return policyResult.Warnings, fmt.Errorf("failed to get SecurityPolicy: %w", err)
			}
		} else {
			existing.Spec = policy.Spec
			existing.Labels = policy.Labels
			existing.OwnerReferences = policy.OwnerReferences
			log.Info("Updating SecurityPolicy", "name", policy.Name)
			if err := r.Update(ctx, existing); err != nil {
				return policyResult.Warnings, fmt.Errorf("failed to update SecurityPolicy: %w", err)
			}
		}
		policyNames = append(policyNames, policy.Name)
	}

	// Reconcile BackendTrafficPolicies
	for _, policy := range policyResult.BackendTrafficPolicies {
		if policy == nil {
			continue
		}
		if policy.Labels == nil {
			policy.Labels = make(map[string]string)
		}
		policy.Labels[policies.LabelSourceIngress] = fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace)

		// Set owner reference to HTTPRoute for automatic GC cleanup
		if routeErr == nil {
			if err := controllerutil.SetControllerReference(httpRoute, policy, r.Scheme); err != nil {
				log.Error(err, "Failed to set owner reference on BackendTrafficPolicy", "name", policy.Name)
			}
		}

		existing := &egv1alpha1.BackendTrafficPolicy{}
		err := r.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, existing)
		if err != nil {
			if kerrors.IsNotFound(err) {
				log.Info("Creating BackendTrafficPolicy", "name", policy.Name)
				if err := r.Create(ctx, policy); err != nil {
					return policyResult.Warnings, fmt.Errorf("failed to create BackendTrafficPolicy: %w", err)
				}
			} else {
				return policyResult.Warnings, fmt.Errorf("failed to get BackendTrafficPolicy: %w", err)
			}
		} else {
			existing.Spec = policy.Spec
			existing.Labels = policy.Labels
			existing.OwnerReferences = policy.OwnerReferences
			log.Info("Updating BackendTrafficPolicy", "name", policy.Name)
			if err := r.Update(ctx, existing); err != nil {
				return policyResult.Warnings, fmt.Errorf("failed to update BackendTrafficPolicy: %w", err)
			}
		}
		policyNames = append(policyNames, policy.Name)
	}

	// Reconcile ClientTrafficPolicies
	for _, policy := range policyResult.ClientTrafficPolicies {
		if policy == nil {
			continue
		}
		if policy.Labels == nil {
			policy.Labels = make(map[string]string)
		}
		policy.Labels[policies.LabelSourceIngress] = fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace)

		// Set owner reference to HTTPRoute for automatic GC cleanup
		if routeErr == nil {
			if err := controllerutil.SetControllerReference(httpRoute, policy, r.Scheme); err != nil {
				log.Error(err, "Failed to set owner reference on ClientTrafficPolicy", "name", policy.Name)
			}
		}

		existing := &egv1alpha1.ClientTrafficPolicy{}
		err := r.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, existing)
		if err != nil {
			if kerrors.IsNotFound(err) {
				log.Info("Creating ClientTrafficPolicy", "name", policy.Name)
				if err := r.Create(ctx, policy); err != nil {
					return policyResult.Warnings, fmt.Errorf("failed to create ClientTrafficPolicy: %w", err)
				}
			} else {
				return policyResult.Warnings, fmt.Errorf("failed to get ClientTrafficPolicy: %w", err)
			}
		} else {
			existing.Spec = policy.Spec
			existing.Labels = policy.Labels
			existing.OwnerReferences = policy.OwnerReferences
			log.Info("Updating ClientTrafficPolicy", "name", policy.Name)
			if err := r.Update(ctx, existing); err != nil {
				return policyResult.Warnings, fmt.Errorf("failed to update ClientTrafficPolicy: %w", err)
			}
		}
		policyNames = append(policyNames, policy.Name)
	}

	// Cleanup stale policies
	if r.CleanupStale {
		if err := r.cleanupStalePolicies(ctx, log, ingress, policyNames); err != nil {
			log.Error(err, "Failed to cleanup stale policies")
		}
	}

	return policyResult.Warnings, nil
}

// cleanupStalePolicies removes policies that were created from this Ingress but are no longer needed.
func (r *Reconciler) cleanupStalePolicies(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress, desiredPolicyNames []string) error {
	desired := make(map[string]bool, len(desiredPolicyNames))
	for _, name := range desiredPolicyNames {
		desired[name] = true
	}

	sourceLabel := fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace)

	// Cleanup stale SecurityPolicies
	securityPolicies := &egv1alpha1.SecurityPolicyList{}
	if err := r.List(ctx, securityPolicies,
		ctrlclient.InNamespace(ingress.Namespace),
		ctrlclient.MatchingLabels{policies.LabelSourceIngress: sourceLabel}); err != nil {
		return fmt.Errorf("failed to list SecurityPolicies: %w", err)
	}
	for i := range securityPolicies.Items {
		policy := &securityPolicies.Items[i]
		if !desired[policy.Name] {
			log.Info("Deleting stale SecurityPolicy", "name", policy.Name)
			if err := r.Delete(ctx, policy); err != nil && !kerrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete SecurityPolicy %s: %w", policy.Name, err)
			}
		}
	}

	// Cleanup stale BackendTrafficPolicies
	backendPolicies := &egv1alpha1.BackendTrafficPolicyList{}
	if err := r.List(ctx, backendPolicies,
		ctrlclient.InNamespace(ingress.Namespace),
		ctrlclient.MatchingLabels{policies.LabelSourceIngress: sourceLabel}); err != nil {
		return fmt.Errorf("failed to list BackendTrafficPolicies: %w", err)
	}
	for i := range backendPolicies.Items {
		policy := &backendPolicies.Items[i]
		if !desired[policy.Name] {
			log.Info("Deleting stale BackendTrafficPolicy", "name", policy.Name)
			if err := r.Delete(ctx, policy); err != nil && !kerrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete BackendTrafficPolicy %s: %w", policy.Name, err)
			}
		}
	}

	// Cleanup stale ClientTrafficPolicies
	clientPolicies := &egv1alpha1.ClientTrafficPolicyList{}
	if err := r.List(ctx, clientPolicies,
		ctrlclient.InNamespace(ingress.Namespace),
		ctrlclient.MatchingLabels{policies.LabelSourceIngress: sourceLabel}); err != nil {
		return fmt.Errorf("failed to list ClientTrafficPolicies: %w", err)
	}
	for i := range clientPolicies.Items {
		policy := &clientPolicies.Items[i]
		if !desired[policy.Name] {
			log.Info("Deleting stale ClientTrafficPolicy", "name", policy.Name)
			if err := r.Delete(ctx, policy); err != nil && !kerrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete ClientTrafficPolicy %s: %w", policy.Name, err)
			}
		}
	}

	return nil
}

// emitWarningEvents emits Kubernetes events for conversion warnings
func (r *Reconciler) emitWarningEvents(ingress *networkingv1.Ingress, warnings []string) {
	if r.Recorder == nil || len(warnings) == 0 {
		return
	}

	// Emit a single event summarizing warnings
	if len(warnings) <= 3 {
		for _, warning := range warnings {
			r.Recorder.Eventf(ingress, corev1.EventTypeWarning, "ConversionWarning", "%s", warning)
		}
	} else {
		// Too many warnings, summarize
		r.Recorder.Eventf(ingress, corev1.EventTypeWarning, "ConversionWarning",
			"Conversion completed with %d warnings. First: %s", len(warnings), warnings[0])
	}
}

// conversionDecision holds the result of determining whether to convert an Ingress
type conversionDecision struct {
	shouldConvert bool
	skipReason    string
	annotate      bool
}

// shouldConvert determines if an Ingress should be converted to HTTPRoute
func (r *Reconciler) shouldConvert(ingress *networkingv1.Ingress) conversionDecision {
	annotations := ingress.GetAnnotations()

	// Explicit skip - don't annotate
	if annotations != nil && annotations[AnnotationSkipConversion] == boolTrue {
		return conversionDecision{shouldConvert: false, annotate: false}
	}

	// Already converted (full or partial) - skip unless user removes annotation
	if annotations != nil {
		status := annotations[AnnotationConversionStatus]
		if status == ConversionStatusConverted || status == ConversionStatusPartial {
			return conversionDecision{shouldConvert: false, annotate: false}
		}
	}

	// Canary - skip WITH annotation
	if annotations != nil && annotations[NginxCanary] == boolTrue {
		return conversionDecision{
			shouldConvert: false,
			skipReason:    SkipReasonCanary,
			annotate:      true,
		}
	}

	// IngressClass filtering - don't annotate (different controller)
	if r.IngressClass != "" {
		ingressClass := ""
		switch {
		case ingress.Spec.IngressClassName != nil:
			ingressClass = *ingress.Spec.IngressClassName
		case annotations != nil:
			ingressClass = annotations[AnnotationIngressClass]
		}
		if ingressClass != r.IngressClass {
			return conversionDecision{shouldConvert: false, annotate: false}
		}
	}

	return conversionDecision{shouldConvert: true}
}

// annotateSkipped adds skip annotations to an Ingress
func (r *Reconciler) annotateSkipped(ctx context.Context, ingress *networkingv1.Ingress, reason string) error {
	current := &networkingv1.Ingress{}
	if err := r.Get(ctx, types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, current); err != nil {
		return err
	}

	// Idempotency check
	if current.Annotations != nil &&
		current.Annotations[AnnotationConversionStatus] == ConversionStatusSkipped &&
		current.Annotations[AnnotationConversionSkipReason] == reason {
		return nil
	}

	if current.Annotations == nil {
		current.Annotations = make(map[string]string)
	}
	current.Annotations[AnnotationConversionStatus] = ConversionStatusSkipped
	current.Annotations[AnnotationConversionSkipReason] = reason

	return r.Update(ctx, current)
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

// isAlreadyConverted checks if an Ingress has been successfully converted
func isAlreadyConverted(obj *networkingv1.Ingress) bool {
	ann := obj.GetAnnotations()
	if ann == nil {
		return false
	}
	status := ann[AnnotationConversionStatus]
	return status == ConversionStatusConverted || status == ConversionStatusPartial
}

func (r *Reconciler) resourceFilter() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			obj, ok := e.Object.(*networkingv1.Ingress)
			if !ok {
				return false
			}
			if isAlreadyConverted(obj) {
				return false
			}
			decision := r.shouldConvert(obj)
			// Process if we should convert OR if we need to annotate
			return decision.shouldConvert || decision.annotate
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			obj, ok := e.ObjectNew.(*networkingv1.Ingress)
			if !ok {
				return false
			}
			if isAlreadyConverted(obj) {
				return false
			}
			decision := r.shouldConvert(obj)
			// Process if we should convert OR if we need to annotate
			return decision.shouldConvert || decision.annotate
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Always process deletes to clean up HTTPRoutes
			_, ok := e.Object.(*networkingv1.Ingress)
			return ok
		},
		GenericFunc: func(e event.GenericEvent) bool {
			obj, ok := e.Object.(*networkingv1.Ingress)
			if !ok {
				return false
			}
			if isAlreadyConverted(obj) {
				return false
			}
			decision := r.shouldConvert(obj)
			// Process if we should convert OR if we need to annotate
			return decision.shouldConvert || decision.annotate
		},
	}
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&networkingv1.Ingress{}, builder.WithPredicates(r.resourceFilter())).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(r)
}

// routeAcceptanceResult holds result of checking route acceptance
type routeAcceptanceResult struct {
	accepted bool
	reason   string
}

// waitForRouteAcceptance polls HTTPRoute status until Accepted or timeout
func (r *Reconciler) waitForRouteAcceptance(ctx context.Context, routeName, routeNS string) routeAcceptanceResult {
	timeout := 30 * time.Second
	interval := 2 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		route := &gwapiv1.HTTPRoute{}
		if err := r.Get(ctx, types.NamespacedName{Name: routeName, Namespace: routeNS}, route); err != nil {
			time.Sleep(interval)
			continue
		}

		// Check status.parents for our Gateway
		for _, parent := range route.Status.Parents {
			if string(parent.ParentRef.Name) != r.GatewayName {
				continue
			}
			for _, cond := range parent.Conditions {
				if cond.Type == string(gwapiv1.RouteConditionAccepted) {
					if cond.Status == metav1.ConditionTrue {
						return routeAcceptanceResult{accepted: true, reason: ""}
					}
					if cond.Status == metav1.ConditionFalse {
						return routeAcceptanceResult{accepted: false, reason: cond.Reason}
					}
				}
			}
		}
		time.Sleep(interval)
	}
	return routeAcceptanceResult{accepted: false, reason: "Timeout"}
}

// waitForGRPCRouteAcceptance polls GRPCRoute status until Accepted or timeout
func (r *Reconciler) waitForGRPCRouteAcceptance(ctx context.Context, routeName, routeNS string) routeAcceptanceResult {
	timeout := 30 * time.Second
	interval := 2 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		route := &gwapiv1.GRPCRoute{}
		if err := r.Get(ctx, types.NamespacedName{Name: routeName, Namespace: routeNS}, route); err != nil {
			time.Sleep(interval)
			continue
		}

		// Check status.parents for our Gateway
		for _, parent := range route.Status.Parents {
			if string(parent.ParentRef.Name) != r.GatewayName {
				continue
			}
			for _, cond := range parent.Conditions {
				if cond.Type == string(gwapiv1.RouteConditionAccepted) {
					if cond.Status == metav1.ConditionTrue {
						return routeAcceptanceResult{accepted: true, reason: ""}
					}
					if cond.Status == metav1.ConditionFalse {
						return routeAcceptanceResult{accepted: false, reason: cond.Reason}
					}
				}
			}
		}
		time.Sleep(interval)
	}
	return routeAcceptanceResult{accepted: false, reason: "Timeout"}
}
