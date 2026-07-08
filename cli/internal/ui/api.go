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

package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"

	"k8c.io/kubelb/cli/internal/ingress"
	"k8c.io/kubelb/pkg/conversion"
	"k8c.io/kubelb/pkg/conversion/policies"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/yaml"
)

const (
	conditionAccepted  = "Accepted"
	gatewayAPIVersion  = "gateway.networking.k8s.io/v1"
	envoyAPIVersion    = "gateway.envoyproxy.io/v1alpha1"
	kindGateway        = "Gateway"
	kindHTTPRoute      = "HTTPRoute"
	kindGRPCRoute      = "GRPCRoute"
	kindBackendPolicy  = "BackendTrafficPolicy"
	kindSecurityPolicy = "SecurityPolicy"
	kindClientPolicy   = "ClientTrafficPolicy"
	statusUnknown      = "Unknown"
	statusTrue         = "True"
)

// IngressListResponse is the JSON response for listing ingresses
type IngressListResponse struct {
	Summary   SummaryResponse   `json:"summary"`
	Ingresses []IngressResponse `json:"ingresses"`
	Config    ConfigInfo        `json:"config"`
}

// ConfigInfo describes the current conversion configuration
type ConfigInfo struct {
	GatewayName       string `json:"gatewayName"`
	GatewayNamespace  string `json:"gatewayNamespace"`
	GatewayClass      string `json:"gatewayClass"`
	IngressClass      string `json:"ingressClass,omitempty"`
	DomainReplace     string `json:"domainReplace,omitempty"`
	DomainSuffix      string `json:"domainSuffix,omitempty"`
	PropagateExtDNS   bool   `json:"propagateExternalDns"`
	CopyTLSSecrets    bool   `json:"copyTlsSecrets"`
	DisableEGFeatures bool   `json:"disableEnvoyGatewayFeatures"`
}

// SummaryResponse holds aggregate counts
type SummaryResponse struct {
	Total     int `json:"total"`
	Converted int `json:"converted"`
	Partial   int `json:"partial"`
	Pending   int `json:"pending"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
	New       int `json:"new"`
}

// IngressResponse is the JSON representation of an ingress
type IngressResponse struct {
	Name       string   `json:"name"`
	Namespace  string   `json:"namespace"`
	Status     string   `json:"status"`
	Hosts      []string `json:"hosts"`
	HTTPRoutes []string `json:"httproutes"`
	GRPCRoutes []string `json:"grpcroutes"`
	Warnings   []string `json:"warnings"`
	SkipReason string   `json:"skipReason,omitempty"`
}

// PreviewResponse is the JSON response for preview
type PreviewResponse struct {
	OriginalYAML  string   `json:"originalYaml"`
	ConvertedYAML string   `json:"convertedYaml"`
	Warnings      []string `json:"warnings"`
}

// ConvertResponse is the JSON response for conversion
type ConvertResponse struct {
	Success   bool                 `json:"success"`
	Message   string               `json:"message,omitempty"`
	Error     string               `json:"error,omitempty"`
	Warnings  []string             `json:"warnings,omitempty"`
	Ingresses *IngressListResponse `json:"ingresses,omitempty"` // Updated ingress list
	Routes    *RoutesListResponse  `json:"routes,omitempty"`    // Updated routes list
}

// BatchConvertRequest is the JSON request for batch conversion
type BatchConvertRequest struct {
	Ingresses []struct {
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	} `json:"ingresses"`
}

// BatchConvertResponse is the JSON response for batch conversion
type BatchConvertResponse struct {
	Converted []string             `json:"converted"`
	Skipped   []string             `json:"skipped"`
	Failed    map[string]string    `json:"failed"`
	Ingresses *IngressListResponse `json:"ingresses,omitempty"` // Updated ingress list
	Routes    *RoutesListResponse  `json:"routes,omitempty"`    // Updated routes list
}

// AnnotationInfo describes an annotation's conversion support
type AnnotationInfo struct {
	Annotation  string `json:"annotation"`
	Status      string `json:"status"` // "converted", "policy", "unsupported"
	GatewayAPI  string `json:"gatewayApi,omitempty"`
	Description string `json:"description,omitempty"`
}

// AnnotationsResponse is the JSON response for annotation matrix
type AnnotationsResponse struct {
	Converted   []AnnotationInfo `json:"converted"`
	Policy      []AnnotationInfo `json:"policy"`
	Unsupported []AnnotationInfo `json:"unsupported"`
}

// RoutesListResponse is the JSON response for listing Gateway API resources
type RoutesListResponse struct {
	Gateways               []GatewayResourceResponse `json:"gateways"`
	HTTPRoutes             []GatewayResourceResponse `json:"httpRoutes"`
	GRPCRoutes             []GatewayResourceResponse `json:"grpcRoutes"`
	BackendTrafficPolicies []GatewayResourceResponse `json:"backendTrafficPolicies"`
	SecurityPolicies       []GatewayResourceResponse `json:"securityPolicies"`
	ClientTrafficPolicies  []GatewayResourceResponse `json:"clientTrafficPolicies"`
}

// GatewayResourceResponse is the JSON representation of a Gateway API resource
type GatewayResourceResponse struct {
	Kind          string   `json:"kind"`
	Name          string   `json:"name"`
	Namespace     string   `json:"namespace"`
	Hostnames     []string `json:"hostnames,omitempty"`
	Gateway       string   `json:"gateway,omitempty"`
	TargetRef     string   `json:"targetRef,omitempty"`
	SourceIngress string   `json:"sourceIngress,omitempty"`
	ManagedBy     string   `json:"managedBy,omitempty"`
	Status        string   `json:"status"`
	StatusReason  string   `json:"statusReason,omitempty"`
	Addresses     []string `json:"addresses,omitempty"`
}

func (s *Server) handleListIngresses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var ingresses networkingv1.IngressList
	if err := s.client.List(ctx, &ingresses); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var infos []ingress.Info
	for _, ing := range ingresses.Items {
		if s.opts.IngressClass != "" && !matchesIngressClass(&ing, s.opts.IngressClass) {
			continue
		}
		infos = append(infos, extractInfo(&ing))
	}

	summary := calculateSummary(infos)
	resp := IngressListResponse{
		Summary: SummaryResponse{
			Total:     summary.Total,
			Converted: summary.Converted,
			Partial:   summary.Partial,
			Pending:   summary.Pending,
			Failed:    summary.Failed,
			Skipped:   summary.Skipped,
			New:       summary.New,
		},
		Ingresses: make([]IngressResponse, len(infos)),
		Config: ConfigInfo{
			GatewayName:       s.opts.GatewayName,
			GatewayNamespace:  s.opts.GatewayNamespace,
			GatewayClass:      s.opts.GatewayClassName,
			IngressClass:      s.opts.IngressClass,
			DomainReplace:     s.opts.DomainReplace,
			DomainSuffix:      s.opts.DomainSuffix,
			PropagateExtDNS:   s.opts.PropagateExternalDNS,
			CopyTLSSecrets:    s.opts.CopyTLSSecrets,
			DisableEGFeatures: s.opts.DisableEnvoyGatewayFeatures,
		},
	}

	for i, info := range infos {
		resp.Ingresses[i] = IngressResponse{
			Name:       info.Name,
			Namespace:  info.Namespace,
			Status:     string(info.Status),
			Hosts:      info.Hosts,
			HTTPRoutes: info.HTTPRoutes,
			GRPCRoutes: info.GRPCRoutes,
			Warnings:   info.Warnings,
			SkipReason: info.SkipReason,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")

	result, err := ingress.Preview(r.Context(), s.client, namespace, name, s.opts)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, PreviewResponse{
		OriginalYAML:  result.OriginalYAML,
		ConvertedYAML: result.ConvertedYAML,
		Warnings:      result.Warnings,
	})
}

func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")

	result, err := ingress.Convert(ctx, s.client, s.opts, ingress.ConvertOptions{
		Names:                 []string{namespace + "/" + name},
		SkipAcceptancePolling: true, // UI gets updated state via response
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ConvertResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if len(result.Failed) > 0 {
		for _, e := range result.Failed {
			writeJSON(w, http.StatusBadRequest, ConvertResponse{
				Success: false,
				Error:   e.Error(),
			})
			return
		}
	}

	// Fetch updated data for UI refresh
	ingressList := s.fetchIngressList(ctx)
	routesList := s.fetchRoutesList(ctx)

	writeJSON(w, http.StatusOK, ConvertResponse{
		Success:   true,
		Message:   "Converted successfully",
		Ingresses: ingressList,
		Routes:    routesList,
	})
}

func (s *Server) handleDeleteIngress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")

	var ing networkingv1.Ingress
	if err := s.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &ing); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	if err := s.client.Delete(ctx, &ing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (s *Server) handleSkipIngress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")

	var ing networkingv1.Ingress
	if err := s.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &ing); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	if ing.Annotations == nil {
		ing.Annotations = make(map[string]string)
	}
	ing.Annotations[conversion.AnnotationSkipConversion] = "true"
	ing.Annotations[conversion.AnnotationConversionSkipReason] = "skipped via UI"

	if err := s.client.Update(ctx, &ing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "skipped"})
}

func (s *Server) handleBatchConvert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req BatchConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	names := make([]string, len(req.Ingresses))
	for i, ing := range req.Ingresses {
		names[i] = ing.Namespace + "/" + ing.Name
	}

	result, err := ingress.Convert(ctx, s.client, s.opts, ingress.ConvertOptions{
		Names:                 names,
		SkipAcceptancePolling: true, // UI gets updated state via response
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	failed := make(map[string]string)
	for k, v := range result.Failed {
		failed[k] = v.Error()
	}

	ingressList := s.fetchIngressList(ctx)
	routesList := s.fetchRoutesList(ctx)

	writeJSON(w, http.StatusOK, BatchConvertResponse{
		Converted: result.Converted,
		Skipped:   result.Skipped,
		Failed:    failed,
		Ingresses: ingressList,
		Routes:    routesList,
	})
}

// BatchPreviewResponse is the JSON response for batch preview
type BatchPreviewResponse struct {
	ConvertedYAML string   `json:"convertedYaml"`
	Warnings      []string `json:"warnings"`
}

func (s *Server) handleBatchPreview(w http.ResponseWriter, r *http.Request) {
	var req BatchConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.Ingresses) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no ingresses specified"})
		return
	}

	// Convert to BatchPreviewInput
	inputs := make([]ingress.BatchPreviewInput, len(req.Ingresses))
	for i, ing := range req.Ingresses {
		inputs[i] = ingress.BatchPreviewInput{
			Namespace: ing.Namespace,
			Name:      ing.Name,
		}
	}

	result, err := ingress.BatchPreview(r.Context(), s.client, s.opts, inputs)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, BatchPreviewResponse{
		ConvertedYAML: result.ConvertedYAML,
		Warnings:      result.Warnings,
	})
}

func (s *Server) handleAnnotations(w http.ResponseWriter, _ *http.Request) {
	resp := AnnotationsResponse{
		Converted: []AnnotationInfo{
			{Annotation: "nginx.ingress.kubernetes.io/ssl-redirect", Status: "converted", GatewayAPI: "RequestRedirect (301)"},
			{Annotation: "nginx.ingress.kubernetes.io/force-ssl-redirect", Status: "converted", GatewayAPI: "RequestRedirect (308)"},
			{Annotation: "nginx.ingress.kubernetes.io/permanent-redirect", Status: "converted", GatewayAPI: "RequestRedirect (301)"},
			{Annotation: "nginx.ingress.kubernetes.io/rewrite-target", Status: "converted", GatewayAPI: "URLRewrite (ReplacePrefixMatch)"},
			{Annotation: "nginx.ingress.kubernetes.io/use-regex", Status: "converted", GatewayAPI: "pathType: RegularExpression"},
			{Annotation: "nginx.ingress.kubernetes.io/proxy-set-headers", Status: "converted", GatewayAPI: "RequestHeaderModifier"},
			{Annotation: "nginx.ingress.kubernetes.io/custom-headers", Status: "converted", GatewayAPI: "ResponseHeaderModifier"},
			{Annotation: "nginx.ingress.kubernetes.io/hsts*", Status: "converted", GatewayAPI: "ResponseHeaderModifier (Strict-Transport-Security)"},
		},
		Policy: []AnnotationInfo{
			{Annotation: "nginx.ingress.kubernetes.io/proxy-*-timeout", Status: "policy", GatewayAPI: "BackendTrafficPolicy"},
			{Annotation: "nginx.ingress.kubernetes.io/enable-cors", Status: "policy", GatewayAPI: "SecurityPolicy"},
			{Annotation: "nginx.ingress.kubernetes.io/cors-*", Status: "policy", GatewayAPI: "SecurityPolicy"},
			{Annotation: "nginx.ingress.kubernetes.io/limit-rps", Status: "policy", GatewayAPI: "BackendTrafficPolicy (rateLimit)"},
			{Annotation: "nginx.ingress.kubernetes.io/limit-rpm", Status: "policy", GatewayAPI: "BackendTrafficPolicy (rateLimit)"},
			{Annotation: "nginx.ingress.kubernetes.io/whitelist-source-range", Status: "policy", GatewayAPI: "SecurityPolicy (authorization)"},
			{Annotation: "nginx.ingress.kubernetes.io/auth-*", Status: "policy", GatewayAPI: "SecurityPolicy (basicAuth/extAuth)"},
		},
		Unsupported: []AnnotationInfo{
			{Annotation: "nginx.ingress.kubernetes.io/configuration-snippet", Status: "unsupported", Description: "Raw NGINX config not supported"},
			{Annotation: "nginx.ingress.kubernetes.io/canary-*", Status: "unsupported", Description: "Canary ingresses are skipped entirely"},
			{Annotation: "nginx.ingress.kubernetes.io/enable-modsecurity", Status: "unsupported", Description: "WAF is implementation-specific"},
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	resp := s.buildRoutesListResponse(r.Context())
	writeJSON(w, http.StatusOK, resp)
}

// buildRoutesListResponse builds the routes list response from cluster state
func (s *Server) buildRoutesListResponse(ctx context.Context) RoutesListResponse {
	resp := RoutesListResponse{
		Gateways:               []GatewayResourceResponse{},
		HTTPRoutes:             []GatewayResourceResponse{},
		GRPCRoutes:             []GatewayResourceResponse{},
		BackendTrafficPolicies: []GatewayResourceResponse{},
		SecurityPolicies:       []GatewayResourceResponse{},
		ClientTrafficPolicies:  []GatewayResourceResponse{},
	}

	resp.Gateways = s.listGatewayResources(ctx)
	resp.HTTPRoutes = s.listHTTPRouteResources(ctx)
	resp.GRPCRoutes = s.listGRPCRouteResources(ctx)

	if !s.opts.DisableEnvoyGatewayFeatures {
		resp.BackendTrafficPolicies = s.listBackendTrafficPolicies(ctx)
		resp.SecurityPolicies = s.listSecurityPolicies(ctx)
		resp.ClientTrafficPolicies = s.listClientTrafficPolicies(ctx)
	}

	return resp
}

func (s *Server) listGatewayResources(ctx context.Context) []GatewayResourceResponse {
	var result []GatewayResourceResponse
	var gateways gwapiv1.GatewayList
	if err := s.client.List(ctx, &gateways); err != nil {
		return result
	}

	for _, gw := range gateways.Items {
		status, statusReason := extractGatewayStatus(gw.Status.Conditions)
		managedBy := extractLabel(gw.Labels, conversion.LabelManagedBy)

		var addresses []string
		for _, addr := range gw.Status.Addresses {
			addresses = append(addresses, addr.Value)
		}

		result = append(result, GatewayResourceResponse{
			Kind:         kindGateway,
			Name:         gw.Name,
			Namespace:    gw.Namespace,
			ManagedBy:    managedBy,
			Status:       status,
			StatusReason: statusReason,
			Addresses:    addresses,
		})
	}
	return result
}

func (s *Server) listHTTPRouteResources(ctx context.Context) []GatewayResourceResponse {
	var result []GatewayResourceResponse
	var routes gwapiv1.HTTPRouteList
	if err := s.client.List(ctx, &routes); err != nil {
		return result
	}

	for _, route := range routes.Items {
		status, statusReason := extractRouteStatus(route.Status.Parents)
		result = append(result, GatewayResourceResponse{
			Kind:          kindHTTPRoute,
			Name:          route.Name,
			Namespace:     route.Namespace,
			Hostnames:     extractHostnames(route.Spec.Hostnames),
			Gateway:       extractParentRef(route.Spec.ParentRefs, route.Namespace),
			ManagedBy:     extractLabel(route.Labels, conversion.LabelManagedBy),
			SourceIngress: extractAnnotation(route.Annotations, "kubelb.k8c.io/source-ingress"),
			Status:        status,
			StatusReason:  statusReason,
		})
	}
	return result
}

func (s *Server) listGRPCRouteResources(ctx context.Context) []GatewayResourceResponse {
	var result []GatewayResourceResponse
	var routes gwapiv1.GRPCRouteList
	if err := s.client.List(ctx, &routes); err != nil {
		return result
	}

	for _, route := range routes.Items {
		status, statusReason := extractRouteStatus(route.Status.Parents)
		result = append(result, GatewayResourceResponse{
			Kind:          kindGRPCRoute,
			Name:          route.Name,
			Namespace:     route.Namespace,
			Hostnames:     extractHostnames(route.Spec.Hostnames),
			Gateway:       extractParentRef(route.Spec.ParentRefs, route.Namespace),
			ManagedBy:     extractLabel(route.Labels, conversion.LabelManagedBy),
			SourceIngress: extractAnnotation(route.Annotations, "kubelb.k8c.io/source-ingress"),
			Status:        status,
			StatusReason:  statusReason,
		})
	}
	return result
}

func (s *Server) listBackendTrafficPolicies(ctx context.Context) []GatewayResourceResponse {
	var result []GatewayResourceResponse
	var policyList egv1alpha1.BackendTrafficPolicyList
	if err := s.client.List(ctx, &policyList); err != nil {
		return result
	}

	for _, policy := range policyList.Items {
		result = append(result, GatewayResourceResponse{
			Kind:          kindBackendPolicy,
			Name:          policy.Name,
			Namespace:     policy.Namespace,
			TargetRef:     extractPolicyTargetRef(policy.Spec.TargetRef, policy.Namespace), //nolint:staticcheck
			SourceIngress: extractLabel(policy.Labels, policies.LabelSourceIngress),
			ManagedBy:     extractLabel(policy.Labels, conversion.LabelManagedBy),
			Status:        "Active",
		})
	}
	return result
}

func (s *Server) listSecurityPolicies(ctx context.Context) []GatewayResourceResponse {
	var result []GatewayResourceResponse
	var policyList egv1alpha1.SecurityPolicyList
	if err := s.client.List(ctx, &policyList); err != nil {
		return result
	}

	for _, policy := range policyList.Items {
		result = append(result, GatewayResourceResponse{
			Kind:          kindSecurityPolicy,
			Name:          policy.Name,
			Namespace:     policy.Namespace,
			TargetRef:     extractPolicyTargetRef(policy.Spec.TargetRef, policy.Namespace), //nolint:staticcheck
			SourceIngress: extractLabel(policy.Labels, policies.LabelSourceIngress),
			ManagedBy:     extractLabel(policy.Labels, conversion.LabelManagedBy),
			Status:        "Active",
		})
	}
	return result
}

func (s *Server) listClientTrafficPolicies(ctx context.Context) []GatewayResourceResponse {
	var result []GatewayResourceResponse
	var policyList egv1alpha1.ClientTrafficPolicyList
	if err := s.client.List(ctx, &policyList); err != nil {
		return result
	}

	for _, policy := range policyList.Items {
		result = append(result, GatewayResourceResponse{
			Kind:          kindClientPolicy,
			Name:          policy.Name,
			Namespace:     policy.Namespace,
			TargetRef:     extractPolicyTargetRef(policy.Spec.TargetRef, policy.Namespace), //nolint:staticcheck
			SourceIngress: extractLabel(policy.Labels, policies.LabelSourceIngress),
			ManagedBy:     extractLabel(policy.Labels, conversion.LabelManagedBy),
			Status:        "Active",
		})
	}
	return result
}

// extractGatewayStatus extracts status from gateway conditions
func extractGatewayStatus(conditions []metav1.Condition) (status, reason string) {
	status = statusUnknown
	for _, cond := range conditions {
		if cond.Type == conditionAccepted || cond.Type == "Programmed" {
			if cond.Status == statusTrue {
				status = "Ready"
			} else {
				status = "NotReady"
				reason = cond.Reason
			}
			break
		}
	}
	return
}

// extractLabel safely extracts a label value
func extractLabel(labels map[string]string, key string) string {
	if labels == nil {
		return ""
	}
	return labels[key]
}

// extractAnnotation safely extracts an annotation value
func extractAnnotation(annotations map[string]string, key string) string {
	if annotations == nil {
		return ""
	}
	return annotations[key]
}

// extractParentRef extracts the gateway reference from parent refs
func extractParentRef(refs []gwapiv1.ParentReference, defaultNS string) string {
	if len(refs) == 0 {
		return ""
	}
	ref := refs[0]
	ns := defaultNS
	if ref.Namespace != nil {
		ns = string(*ref.Namespace)
	}
	return ns + "/" + string(ref.Name)
}

// extractHostnames converts gateway hostnames to strings
func extractHostnames(hostnames []gwapiv1.Hostname) []string {
	result := make([]string, len(hostnames))
	for i, h := range hostnames {
		result[i] = string(h)
	}
	return result
}

// extractRouteStatus extracts status from route parent statuses
func extractRouteStatus(parents []gwapiv1.RouteParentStatus) (status, reason string) {
	status = statusUnknown
	for _, parent := range parents {
		for _, cond := range parent.Conditions {
			if cond.Type == conditionAccepted {
				if cond.Status == statusTrue {
					status = "Accepted"
				} else {
					status = "NotAccepted"
					reason = cond.Reason
				}
				return
			}
		}
	}
	return
}

// extractPolicyTargetRef extracts the target reference from a policy
func extractPolicyTargetRef(ref *gwapiv1.LocalPolicyTargetReferenceWithSectionName, _ string) string {
	if ref == nil {
		return ""
	}
	return string(ref.Kind) + "/" + string(ref.Name)
}

// ResourcePreviewResponse is the JSON response for resource preview
type ResourcePreviewResponse struct {
	YAML string `json:"yaml"`
}

// handleResourcePreview returns YAML for any Gateway API resource
func (s *Server) handleResourcePreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	kind := r.PathValue("kind")
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	key := types.NamespacedName{Namespace: namespace, Name: name}

	var obj any
	var err error

	switch kind {
	case kindGateway:
		var gw gwapiv1.Gateway
		err = s.client.Get(ctx, key, &gw)
		gw.ManagedFields = nil
		gw.APIVersion = gatewayAPIVersion
		gw.Kind = kindGateway
		obj = &gw
	case kindHTTPRoute:
		var route gwapiv1.HTTPRoute
		err = s.client.Get(ctx, key, &route)
		route.ManagedFields = nil
		route.APIVersion = gatewayAPIVersion
		route.Kind = kindHTTPRoute
		obj = &route
	case kindGRPCRoute:
		var route gwapiv1.GRPCRoute
		err = s.client.Get(ctx, key, &route)
		route.ManagedFields = nil
		route.APIVersion = gatewayAPIVersion
		route.Kind = kindGRPCRoute
		obj = &route
	case kindBackendPolicy:
		var policy egv1alpha1.BackendTrafficPolicy
		err = s.client.Get(ctx, key, &policy)
		policy.ManagedFields = nil
		policy.APIVersion = envoyAPIVersion
		policy.Kind = kindBackendPolicy
		obj = &policy
	case kindSecurityPolicy:
		var policy egv1alpha1.SecurityPolicy
		err = s.client.Get(ctx, key, &policy)
		policy.ManagedFields = nil
		policy.APIVersion = envoyAPIVersion
		policy.Kind = kindSecurityPolicy
		obj = &policy
	case kindClientPolicy:
		var policy egv1alpha1.ClientTrafficPolicy
		err = s.client.Get(ctx, key, &policy)
		policy.ManagedFields = nil
		policy.APIVersion = envoyAPIVersion
		policy.Kind = kindClientPolicy
		obj = &policy
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported resource kind: " + kind})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	yamlBytes, err := yaml.Marshal(obj)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to marshal: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, ResourcePreviewResponse{YAML: string(yamlBytes)})
}

// handleResourceDelete deletes a Gateway API resource
func (s *Server) handleResourceDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	kind := r.PathValue("kind")
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	key := types.NamespacedName{Namespace: namespace, Name: name}

	var obj client.Object
	switch kind {
	case kindGateway:
		obj = &gwapiv1.Gateway{}
	case kindHTTPRoute:
		obj = &gwapiv1.HTTPRoute{}
	case kindGRPCRoute:
		obj = &gwapiv1.GRPCRoute{}
	case kindBackendPolicy:
		obj = &egv1alpha1.BackendTrafficPolicy{}
	case kindSecurityPolicy:
		obj = &egv1alpha1.SecurityPolicy{}
	case kindClientPolicy:
		obj = &egv1alpha1.ClientTrafficPolicy{}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported resource kind: " + kind})
		return
	}

	if err := s.client.Get(ctx, key, obj); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	if err := s.client.Delete(ctx, obj); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// matchesIngressClass delegates to kubelb's conversion.MatchesIngressClass
func matchesIngressClass(ing *networkingv1.Ingress, class string) bool {
	return conversion.MatchesIngressClass(ing, class)
}

func extractInfo(ing *networkingv1.Ingress) ingress.Info {
	info := ingress.Info{
		Name:      ing.Name,
		Namespace: ing.Namespace,
		Status:    ingress.StatusNew,
	}

	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" {
			info.Hosts = append(info.Hosts, rule.Host)
		}
	}

	// Check if marked for skip via label or annotation
	if conversion.ShouldSkip(ing) {
		info.Status = ingress.StatusSkipped
		// Set skip reason if not already set
		if ing.Annotations != nil {
			if reason, ok := ing.Annotations[conversion.AnnotationConversionSkipReason]; ok {
				info.SkipReason = reason
			}
		}
		if info.SkipReason == "" {
			info.SkipReason = "marked with skip-conversion"
		}
		return info
	}

	if ing.Annotations == nil {
		return info
	}

	if status, ok := ing.Annotations[conversion.AnnotationConversionStatus]; ok {
		info.Status = ingress.Status(status)
	}
	if routes, ok := ing.Annotations[conversion.AnnotationConvertedHTTPRoute]; ok && routes != "" {
		info.HTTPRoutes = strings.Split(routes, ",")
	}
	if routes, ok := ing.Annotations[conversion.AnnotationConvertedGRPCRoute]; ok && routes != "" {
		info.GRPCRoutes = strings.Split(routes, ",")
	}
	if warnings, ok := ing.Annotations[conversion.AnnotationConversionWarnings]; ok && warnings != "" {
		info.Warnings = strings.Split(warnings, ";")
	}
	if reason, ok := ing.Annotations[conversion.AnnotationConversionSkipReason]; ok {
		info.SkipReason = reason
	}

	return info
}

func calculateSummary(infos []ingress.Info) ingress.Summary {
	var s ingress.Summary
	s.Total = len(infos)
	for _, info := range infos {
		switch info.Status {
		case ingress.StatusConverted:
			s.Converted++
		case ingress.StatusPartial:
			s.Partial++
		case ingress.StatusPending:
			s.Pending++
		case ingress.StatusFailed:
			s.Failed++
		case ingress.StatusSkipped:
			s.Skipped++
		case ingress.StatusNew:
			s.New++
		}
	}
	return s
}

// fetchIngressList returns the current ingress list for UI refresh
func (s *Server) fetchIngressList(ctx context.Context) *IngressListResponse {
	var ingresses networkingv1.IngressList
	if err := s.client.List(ctx, &ingresses); err != nil {
		return nil
	}

	var infos []ingress.Info
	for _, ing := range ingresses.Items {
		if s.opts.IngressClass != "" && !matchesIngressClass(&ing, s.opts.IngressClass) {
			continue
		}
		infos = append(infos, extractInfo(&ing))
	}

	summary := calculateSummary(infos)
	resp := &IngressListResponse{
		Summary: SummaryResponse{
			Total:     summary.Total,
			Converted: summary.Converted,
			Partial:   summary.Partial,
			Pending:   summary.Pending,
			Failed:    summary.Failed,
			Skipped:   summary.Skipped,
			New:       summary.New,
		},
		Ingresses: make([]IngressResponse, len(infos)),
		Config: ConfigInfo{
			GatewayName:       s.opts.GatewayName,
			GatewayNamespace:  s.opts.GatewayNamespace,
			GatewayClass:      s.opts.GatewayClassName,
			IngressClass:      s.opts.IngressClass,
			DomainReplace:     s.opts.DomainReplace,
			DomainSuffix:      s.opts.DomainSuffix,
			PropagateExtDNS:   s.opts.PropagateExternalDNS,
			CopyTLSSecrets:    s.opts.CopyTLSSecrets,
			DisableEGFeatures: s.opts.DisableEnvoyGatewayFeatures,
		},
	}

	for i, info := range infos {
		resp.Ingresses[i] = IngressResponse{
			Name:       info.Name,
			Namespace:  info.Namespace,
			Status:     string(info.Status),
			Hosts:      info.Hosts,
			HTTPRoutes: info.HTTPRoutes,
			GRPCRoutes: info.GRPCRoutes,
			Warnings:   info.Warnings,
			SkipReason: info.SkipReason,
		}
	}

	return resp
}

// fetchRoutesList returns the current Gateway API resources for UI refresh
func (s *Server) fetchRoutesList(ctx context.Context) *RoutesListResponse {
	resp := s.buildRoutesListResponse(ctx)
	return &resp
}
