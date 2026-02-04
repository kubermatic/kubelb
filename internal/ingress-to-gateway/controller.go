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

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/go-logr/logr"

	"k8c.io/kubelb/pkg/conversion"
	"k8c.io/kubelb/pkg/conversion/annotations"
	"k8c.io/kubelb/pkg/conversion/policies"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
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
	Recorder events.EventRecorder

	GatewayName                 string
	GatewayNamespace            string
	GatewayClassName            string
	IngressClass                string
	DomainReplace               string
	DomainSuffix                string
	PropagateExternalDNS        bool
	GatewayAnnotations          map[string]string
	DisableEnvoyGatewayFeatures bool
	CopyTLSSecrets              bool
}

// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=grpcroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.envoyproxy.io,resources=securitypolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.envoyproxy.io,resources=backendtrafficpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.envoyproxy.io,resources=clienttrafficpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)

	log.V(1).Info("Reconciling Ingress for conversion")

	ingress := &networkingv1.Ingress{}
	if err := r.Get(ctx, req.NamespacedName, ingress); err != nil {
		if kerrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if ingress.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

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
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress) (requeue bool, err error) {
	services := r.fetchServicesForIngress(ctx, log, ingress)
	result := conversion.ConvertIngressWithServices(conversion.Input{
		Ingress:            ingress,
		GatewayName:        r.GatewayName,
		GatewayNamespace:   r.GatewayNamespace,
		Services:           services,
		DomainReplace:      r.DomainReplace,
		DomainSuffix:       r.DomainSuffix,
		SkipPolicyWarnings: !r.DisableEnvoyGatewayFeatures, // skip warnings when policies ARE auto-created
	})

	if len(result.HTTPRoutes) == 0 && len(result.GRPCRoutes) == 0 {
		return false, fmt.Errorf("conversion produced no routes")
	}

	// Sync TLS secrets to Gateway namespace before reconciling Gateway
	tlsListeners, err := r.syncTLSSecrets(ctx, log, result.TLSListeners)
	if err != nil {
		return false, fmt.Errorf("failed to sync TLS secrets: %w", err)
	}

	// Reconcile Gateway first (create/update with listeners and annotations)
	if err := r.reconcileGateway(ctx, log, ingress, tlsListeners); err != nil {
		return false, fmt.Errorf("failed to reconcile Gateway: %w", err)
	}

	// Extract external-dns annotations for routes
	externalDNSAnnotations := r.extractHTTPRouteAnnotations(ingress)

	// Track created route names for status annotation
	var httpRouteNames []string
	var grpcRouteNames []string

	// Reconcile HTTPRoutes
	for _, httpRoute := range result.HTTPRoutes {
		applyRouteMetadata(httpRoute, ingress, externalDNSAnnotations)

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
		applyRouteMetadata(grpcRoute, ingress, externalDNSAnnotations)

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

	// Verify route
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
				r.Recorder.Eventf(ingress, nil, corev1.EventTypeWarning, "RouteNotAccepted", "Reconciling",
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
				r.Recorder.Eventf(ingress, nil, corev1.EventTypeWarning, "RouteNotAccepted", "Reconciling",
					"GRPCRoute %s not accepted by Gateway: %s", grpcRoute.Name, acceptance.reason)
			}
		}
	}

	// Update Ingress annotations with conversion status
	needsRequeue, err := r.updateIngressStatusStaged(ctx, ingress, allWarnings, httpRouteNames, grpcRouteNames, routeAcceptance, hasTimeout)
	if err != nil {
		return false, fmt.Errorf("failed to update Ingress status: %w", err)
	}

	// Emit events for warnings
	r.emitWarningEvents(ingress, allWarnings)

	log.Info("Successfully converted Ingress",
		"httproutes", len(result.HTTPRoutes),
		"grpcroutes", len(result.GRPCRoutes),
		"warnings", len(allWarnings),
		"requeue", needsRequeue)

	return needsRequeue, nil
}

// statusDecision holds the computed status and whether to requeue for verification
type statusDecision struct {
	status          string
	requeue         bool
	clearVerifyTime bool
}

// determineConversionStatus computes the status based on acceptance, warnings, and verification state.
// Two-phase verification: routes must be accepted on two separate reconciles before marking "converted".
func (r *Reconciler) determineConversionStatus(current *networkingv1.Ingress, routeAcceptance map[string]bool, warnings []string, hasTimeout bool) statusDecision {
	allAccepted := true
	for _, accepted := range routeAcceptance {
		if !accepted {
			allAccepted = false
			break
		}
	}

	// If routes timed out or aren't accepted, stay pending or partial
	if hasTimeout {
		return statusDecision{status: conversion.ConversionStatusPending, requeue: true, clearVerifyTime: true}
	}

	if !allAccepted {
		return statusDecision{status: conversion.ConversionStatusPartial, requeue: false, clearVerifyTime: true}
	}

	// Routes are accepted - check if this is first or second verification pass
	verifyTime := current.Annotations[conversion.AnnotationVerificationTimestamp]
	if verifyTime == "" {
		// First pass: routes look accepted, but need verification pass
		// Mark pending and requeue
		return statusDecision{status: conversion.ConversionStatusPending, requeue: true, clearVerifyTime: false}
	}

	// Second pass: parse verification timestamp
	ts, err := time.Parse(time.RFC3339, verifyTime)
	if err != nil {
		// Invalid timestamp, treat as first pass
		return statusDecision{status: conversion.ConversionStatusPending, requeue: true, clearVerifyTime: false}
	}

	// Require 5s between first verification and final status
	verifyDelay := 5 * time.Second
	if time.Since(ts) < verifyDelay {
		// Not enough time elapsed, requeue
		return statusDecision{status: conversion.ConversionStatusPending, requeue: true, clearVerifyTime: false}
	}

	// Verified! Routes still accepted after delay
	if len(warnings) > 0 {
		return statusDecision{status: conversion.ConversionStatusPartial, requeue: false, clearVerifyTime: true}
	}
	return statusDecision{status: conversion.ConversionStatusConverted, requeue: false, clearVerifyTime: true}
}

func (r *Reconciler) updateIngressStatusStaged(ctx context.Context, ingress *networkingv1.Ingress, warnings, httpRouteNames, grpcRouteNames []string, routeAcceptance map[string]bool, hasTimeout bool) (bool, error) {
	// Re-fetch to avoid conflicts
	current := &networkingv1.Ingress{}
	if err := r.Get(ctx, types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, current); err != nil {
		return false, err
	}

	if current.Annotations == nil {
		current.Annotations = make(map[string]string)
	}

	decision := r.determineConversionStatus(current, routeAcceptance, warnings, hasTimeout)

	// Set or clear verification timestamp
	if decision.clearVerifyTime {
		delete(current.Annotations, conversion.AnnotationVerificationTimestamp)
	} else if current.Annotations[conversion.AnnotationVerificationTimestamp] == "" {
		// First verification pass - set timestamp
		current.Annotations[conversion.AnnotationVerificationTimestamp] = time.Now().Format(time.RFC3339)
	}

	if len(warnings) > 0 {
		current.Annotations[conversion.AnnotationConversionWarnings] = strings.Join(warnings, "; ")
	} else {
		delete(current.Annotations, conversion.AnnotationConversionWarnings)
	}

	current.Annotations[conversion.AnnotationConversionStatus] = decision.status

	if len(httpRouteNames) > 0 {
		current.Annotations[conversion.AnnotationConvertedHTTPRoute] = strings.Join(httpRouteNames, ",")
	} else {
		delete(current.Annotations, conversion.AnnotationConvertedHTTPRoute)
	}

	if len(grpcRouteNames) > 0 {
		current.Annotations[conversion.AnnotationConvertedGRPCRoute] = strings.Join(grpcRouteNames, ",")
	} else {
		delete(current.Annotations, conversion.AnnotationConvertedGRPCRoute)
	}

	return decision.requeue, r.Update(ctx, current)
}

// reconcilePolicies creates or updates Envoy Gateway policies for the Ingress.
func (r *Reconciler) reconcilePolicies(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress, routeName string) ([]string, error) {
	policyResult := annotations.ConvertToPolicies(annotations.PolicyConversionInput{
		IngressName:      ingress.Name,
		IngressNamespace: ingress.Namespace,
		RouteName:        routeName,
		GatewayName:      r.GatewayName,
		Annotations:      ingress.Annotations,
	})

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
		if routeErr == nil {
			r.setPolicyOwnerRef(policy, ingress, httpRoute, log)
		} else {
			r.setPolicyOwnerRef(policy, ingress, nil, log)
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
	}

	// Reconcile BackendTrafficPolicies
	for _, policy := range policyResult.BackendTrafficPolicies {
		if policy == nil {
			continue
		}
		if routeErr == nil {
			r.setPolicyOwnerRef(policy, ingress, httpRoute, log)
		} else {
			r.setPolicyOwnerRef(policy, ingress, nil, log)
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
	}

	// Reconcile ClientTrafficPolicies
	for _, policy := range policyResult.ClientTrafficPolicies {
		if policy == nil {
			continue
		}
		if routeErr == nil {
			r.setPolicyOwnerRef(policy, ingress, httpRoute, log)
		} else {
			r.setPolicyOwnerRef(policy, ingress, nil, log)
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
	}

	return policyResult.Warnings, nil
}

// emitWarningEvents emits Kubernetes events for conversion warnings
func (r *Reconciler) emitWarningEvents(ingress *networkingv1.Ingress, warnings []string) {
	if r.Recorder == nil || len(warnings) == 0 {
		return
	}

	// Emit a single event summarizing warnings
	if len(warnings) <= 3 {
		for _, warning := range warnings {
			r.Recorder.Eventf(ingress, nil, corev1.EventTypeWarning, "ConversionWarning", "Reconciling", "%s", warning)
		}
	} else {
		// Too many warnings, summarize
		r.Recorder.Eventf(ingress, nil, corev1.EventTypeWarning, "ConversionWarning", "Reconciling",
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
	if annotations != nil && annotations[conversion.AnnotationSkipConversion] == conversion.BoolTrue {
		return conversionDecision{shouldConvert: false, annotate: false}
	}

	// Already converted (full or partial) - skip unless user removes annotation
	if annotations != nil {
		status := annotations[conversion.AnnotationConversionStatus]
		if status == conversion.ConversionStatusConverted || status == conversion.ConversionStatusPartial {
			return conversionDecision{shouldConvert: false, annotate: false}
		}
	}

	// Canary - skip WITH annotation
	if annotations != nil && annotations[conversion.NginxCanary] == conversion.BoolTrue {
		return conversionDecision{
			shouldConvert: false,
			skipReason:    conversion.SkipReasonCanary,
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
			ingressClass = annotations[conversion.AnnotationIngressClass]
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
		current.Annotations[conversion.AnnotationConversionStatus] == conversion.ConversionStatusSkipped &&
		current.Annotations[conversion.AnnotationConversionSkipReason] == reason {
		return nil
	}

	if current.Annotations == nil {
		current.Annotations = make(map[string]string)
	}
	current.Annotations[conversion.AnnotationConversionStatus] = conversion.ConversionStatusSkipped
	current.Annotations[conversion.AnnotationConversionSkipReason] = reason

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

// syncTLSSecrets copies TLS secrets from source namespaces to the Gateway namespace.
// Returns the updated TLSListeners with synced secret names.
func (r *Reconciler) syncTLSSecrets(ctx context.Context, log logr.Logger, tlsListeners []conversion.TLSListener) ([]conversion.TLSListener, error) {
	if !r.CopyTLSSecrets {
		return tlsListeners, nil
	}

	result := make([]conversion.TLSListener, len(tlsListeners))
	copy(result, tlsListeners)

	seen := make(map[string]bool)
	for i, tls := range result {
		if tls.SourceSecretName == "" {
			continue
		}

		// Skip if source namespace is same as gateway namespace (no copy needed)
		if tls.SourceSecretNamespace == r.GatewayNamespace {
			continue
		}

		// Generate synced secret name: ingress-<namespace>-<secretname>
		syncedName := fmt.Sprintf("ingress-%s-%s", tls.SourceSecretNamespace, tls.SourceSecretName)

		// Dedupe - only sync once per source secret
		sourceKey := fmt.Sprintf("%s/%s", tls.SourceSecretNamespace, tls.SourceSecretName)
		if seen[sourceKey] {
			result[i].SecretName = syncedName
			continue
		}
		seen[sourceKey] = true

		// Fetch source secret
		sourceSecret := &corev1.Secret{}
		sourceKey = tls.SourceSecretNamespace + "/" + tls.SourceSecretName
		if err := r.Get(ctx, types.NamespacedName{
			Name:      tls.SourceSecretName,
			Namespace: tls.SourceSecretNamespace,
		}, sourceSecret); err != nil {
			if kerrors.IsNotFound(err) {
				log.V(1).Info("TLS secret not found in source namespace", "secret", sourceKey)
				continue
			}
			return nil, fmt.Errorf("failed to get TLS secret %s: %w", sourceKey, err)
		}

		// Create or update secret in Gateway namespace
		targetSecret := &corev1.Secret{}
		targetKey := types.NamespacedName{Name: syncedName, Namespace: r.GatewayNamespace}
		err := r.Get(ctx, targetKey, targetSecret)
		if err != nil {
			if kerrors.IsNotFound(err) {
				// Create new secret
				targetSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      syncedName,
						Namespace: r.GatewayNamespace,
						Labels: map[string]string{
							conversion.LabelManagedBy: conversion.ControllerName,
						},
						Annotations: map[string]string{
							"kubelb.k8c.io/source-secret": sourceKey,
						},
					},
					Type: sourceSecret.Type,
					Data: sourceSecret.Data,
				}
				log.Info("Creating synced TLS secret", "name", syncedName, "source", sourceKey)
				if err := r.Create(ctx, targetSecret); err != nil {
					return nil, fmt.Errorf("failed to create synced TLS secret %s: %w", syncedName, err)
				}
			} else {
				return nil, fmt.Errorf("failed to get target TLS secret %s: %w", syncedName, err)
			}
		} else {
			// Update existing secret if data changed
			if !secretDataEqual(sourceSecret.Data, targetSecret.Data) {
				targetSecret.Data = sourceSecret.Data
				targetSecret.Type = sourceSecret.Type
				log.Info("Updating synced TLS secret", "name", syncedName, "source", sourceKey)
				if err := r.Update(ctx, targetSecret); err != nil {
					return nil, fmt.Errorf("failed to update synced TLS secret %s: %w", syncedName, err)
				}
			}
		}

		// Update the listener to use the synced secret name
		result[i].SecretName = syncedName
	}

	return result, nil
}

// secretDataEqual compares two secret data maps
func secretDataEqual(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || string(v) != string(bv) {
			return false
		}
	}
	return true
}

// routeAcceptanceResult holds result of checking route acceptance
type routeAcceptanceResult struct {
	accepted bool
	reason   string
}

// ensureMetadata initializes Labels and Annotations maps if nil
func ensureMetadata(obj metav1.Object) {
	if obj.GetLabels() == nil {
		obj.SetLabels(make(map[string]string))
	}
	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(make(map[string]string))
	}
}

// applyRouteMetadata sets source ingress label and copies annotations to a route
func applyRouteMetadata(obj metav1.Object, ingress *networkingv1.Ingress, annotations map[string]string) {
	ensureMetadata(obj)
	labels := obj.GetLabels()
	labels[policies.LabelSourceIngress] = fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace)
	obj.SetLabels(labels)

	objAnnotations := obj.GetAnnotations()
	for k, v := range annotations {
		objAnnotations[k] = v
	}
	obj.SetAnnotations(objAnnotations)
}

// setPolicyOwnerRef sets the source ingress label and owner reference on a policy
func (r *Reconciler) setPolicyOwnerRef(policy ctrlclient.Object, ingress *networkingv1.Ingress, httpRoute *gwapiv1.HTTPRoute, log logr.Logger) {
	labels := policy.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[policies.LabelSourceIngress] = fmt.Sprintf("%s.%s", ingress.Name, ingress.Namespace)
	policy.SetLabels(labels)

	if httpRoute != nil {
		if err := controllerutil.SetControllerReference(httpRoute, policy, r.Scheme); err != nil {
			log.Error(err, "Failed to set owner reference on policy", "name", policy.GetName())
		}
	}
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

// isAlreadyConverted checks if an Ingress has been successfully converted
func isAlreadyConverted(obj *networkingv1.Ingress) bool {
	ann := obj.GetAnnotations()
	if ann == nil {
		return false
	}
	status := ann[conversion.AnnotationConversionStatus]
	return status == conversion.ConversionStatusConverted || status == conversion.ConversionStatusPartial
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
		Named(conversion.ControllerName).
		For(&networkingv1.Ingress{}, builder.WithPredicates(r.resourceFilter())).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(r)
}
