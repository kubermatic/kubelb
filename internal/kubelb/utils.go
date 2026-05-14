/*
Copyright 2020 The KubeLB Authors.

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

package kubelb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/resources"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const LabelOriginNamespace = "kubelb.k8c.io/origin-ns"
const LabelOriginName = "kubelb.k8c.io/origin-name"
const LabelOriginResourceKind = "kubelb.k8c.io/origin-resource-kind"

const LabelLoadBalancerNamespace = "kubelb.k8c.io/lb-namespace"
const LabelLoadBalancerName = "kubelb.k8c.io/lb-name"
const LabelTenantName = "kubelb.k8c.io/tenant"
const LabelManagedBy = "kubelb.k8c.io/managed-by"
const LabelControllerName = "kubelb"

const LabelAppKubernetesName = "app.kubernetes.io/name"            // mysql
const LabelAppKubernetesType = "app.kubernetes.io/type"            // mysql
const LabelAppKubernetesInstance = "app.kubernetes.io/instance"    // mysql-abcxzy"
const LabelAppKubernetesVersion = "app.kubernetes.io/version"      // 5.7.21
const LabelAppKubernetesComponent = "app.kubernetes.io/component"  // database
const LabelAppKubernetesPartOf = "app.kubernetes.io/part-of"       // wordpress
const LabelAppKubernetesManagedBy = "app.kubernetes.io/managed-by" // helm

const EnvoyResourceIdentifierPattern = "%s-%s-ep-%d-port-%d-%s"
const EnvoyEndpointPattern = "%s-%s-ep-%d"

// EnvoyEndpointRoutePattern includes route name for per-route port allocation
const EnvoyEndpointRoutePattern = "tenant-%s-route-%s-%s-%s"

// EnvoyRoutePortIdentifierPattern includes route name for unique listener keys
const EnvoyRoutePortIdentifierPattern = "tenant-%s-route-%s-%s-%s-svc-%s-port-%d-%s"
const EnvoyListenerPattern = "%v-%s"
const RouteServiceMapKey = "%s/%s"
const DefaultRouteStatus = "{}"

const ServiceKind = "Service"
const NameSuffixLength = 4

// AnnotationValueTrue is the canonical truthy value for boolean-style
// KubeLB annotations.
const AnnotationValueTrue = "true"

// We limit the name length slightly lower than the kubernetes limit of 63 characters to avoid issues with the name length.
// In case if the name exceeds the limit, we truncate the name and append a suffix ensuring that it's always less than 63 characters.
const MaxNameLength = 60

const AnnotationRequestWildcardDomain = "kubelb.k8c.io/request-wildcard-domain"
const AnnotationProxyProtocol = "kubelb.k8c.io/proxy-protocol"

func GenerateName(name, namespace string) string {
	output := fmt.Sprintf("%s-%s", namespace, name)

	if len(output) >= MaxNameLength {
		hash := sha256.Sum256([]byte(output))
		suffix := hex.EncodeToString(hash[:])[:NameSuffixLength]
		output = fmt.Sprintf("%s-%s", output[:MaxNameLength-(NameSuffixLength+1)], suffix)
	}

	return output
}

// GenerateRouteServiceName generates a unique service name that includes route identifier.
// Format: namespace-routeName-serviceName[-uid] (truncated to MaxNameLength)
func GenerateRouteServiceName(routeName, serviceName, namespace string) string {
	output := fmt.Sprintf("%s-%s-%s", namespace, routeName, serviceName)

	if len(output) >= MaxNameLength {
		hash := sha256.Sum256([]byte(output))
		suffix := hex.EncodeToString(hash[:])[:NameSuffixLength]
		output = fmt.Sprintf("%s-%s", output[:MaxNameLength-(NameSuffixLength+1)], suffix)
	}

	return output
}

func GetName(obj client.Object) string {
	name := obj.GetName()
	if labels := obj.GetLabels(); labels != nil {
		if _, ok := labels[LabelOriginName]; ok {
			name = labels[LabelOriginName]
		}
	}
	return name
}

func GetNamespace(obj client.Object) string {
	namespace := obj.GetNamespace()
	if labels := obj.GetLabels(); labels != nil {
		if _, ok := labels[LabelOriginNamespace]; ok {
			namespace = labels[LabelOriginNamespace]
		}
	}
	return namespace
}

// PropagateAnnotations returns the set of annotations to apply to a downstream
// resource (Service/Ingress/Route/Gateway) based on the source resource's annotations
// and the tenant/config AnnotationSettings. The returned map is always a fresh
// allocation; the source map is never mutated. Precedence:
//  1. DeniedAnnotations (glob) — always wins.
//  2. PropagateAllAnnotations=true — keep everything not denied.
//  3. PropagatedAnnotations — allow-list with glob keys and exact value match.
//  4. DefaultAnnotations — merged in last; never overrides existing keys.
func PropagateAnnotations(source map[string]string, settings kubelbv1alpha1.AnnotationSettings, resource kubelbv1alpha1.AnnotatedResource) map[string]string {
	out := make(map[string]string, len(source))
	denied := compileDenyMatcher(settings.DeniedAnnotations)
	propagateAll := settings.PropagateAllAnnotations != nil && *settings.PropagateAllAnnotations
	allowed := compileAllowMatcher(settings.PropagatedAnnotations)

	for k, v := range source {
		if denied(k) {
			continue
		}
		if propagateAll || allowed(k, v) {
			out[k] = v
		}
	}
	return mergeDefaultAnnotations(out, settings, resource)
}

// compileDenyMatcher returns a function reporting whether key matches any deny pattern.
// Patterns support shell-style globbing via path.Match. Invalid patterns are treated as
// literal exact matches so that bad config can't silently bypass a denial.
func compileDenyMatcher(patterns []string) func(key string) bool {
	if len(patterns) == 0 {
		return func(string) bool { return false }
	}
	return func(key string) bool {
		for _, p := range patterns {
			if matchPattern(p, key) {
				return true
			}
		}
		return false
	}
}

// compileAllowMatcher compiles the allow-list (key pattern -> comma-separated values, empty value means any)
// into a closure that reports whether (key, value) is permitted. Keys support globbing.
func compileAllowMatcher(allow *map[string]string) func(key, value string) bool {
	if allow == nil || len(*allow) == 0 {
		return func(string, string) bool { return false }
	}
	type rule struct {
		pattern string
		values  []string // nil means any value
	}
	rules := make([]rule, 0, len(*allow))
	for k, v := range *allow {
		r := rule{pattern: k}
		if v != "" {
			parts := strings.Split(v, ",")
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			r.values = parts
		}
		rules = append(rules, r)
	}
	return func(key, value string) bool {
		for _, r := range rules {
			if !matchPattern(r.pattern, key) {
				continue
			}
			if r.values == nil {
				return true
			}
			for _, allowed := range r.values {
				if allowed == value {
					return true
				}
			}
		}
		return false
	}
}

// matchPattern reports whether key matches the (possibly glob) pattern. If the pattern is
// malformed for path.Match, it falls back to an exact-string comparison.
func matchPattern(pattern, key string) bool {
	matched, err := path.Match(pattern, key)
	if err != nil {
		return pattern == key
	}
	return matched
}

// mergeDefaultAnnotations writes resource-specific defaults (then "all" defaults)
// into out, without overwriting existing keys. out is mutated and returned.
func mergeDefaultAnnotations(out map[string]string, settings kubelbv1alpha1.AnnotationSettings, resource kubelbv1alpha1.AnnotatedResource) map[string]string {
	if settings.DefaultAnnotations == nil {
		return out
	}
	for k, v := range settings.DefaultAnnotations[resource] {
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}
	for k, v := range settings.DefaultAnnotations[kubelbv1alpha1.AnnotatedResourceAll] {
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}
	return out
}

func AddKubeLBLabels(labels map[string]string, name, namespace, gvk string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[LabelOriginName] = name
	labels[LabelOriginNamespace] = namespace
	labels[LabelManagedBy] = LabelControllerName

	if gvk != "" {
		labels[LabelOriginResourceKind] = gvk
	}
	return labels
}

// AddDNSAndCertificateAnnotations adds DNS and certificate annotations based on tenant and config settings
func AddDNSAndCertificateAnnotations(annotations map[string]string, tenant *kubelbv1alpha1.Tenant, config *kubelbv1alpha1.Config, hostname string, skipCertificateAnnotations bool) {
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Add DNS annotations if enabled
	var shouldAddDNS bool
	if tenant.Spec.DNS.UseDNSAnnotations != nil {
		// Tenant has explicit setting - use it
		shouldAddDNS = *tenant.Spec.DNS.UseDNSAnnotations
	} else {
		// Tenant setting is null - fall back to config
		shouldAddDNS = config.Spec.DNS.UseDNSAnnotations
	}

	if shouldAddDNS {
		annotations[resources.ExternalDNSHostnameAnnotation] = hostname
		annotations[resources.ExternalDNSTTLAnnotation] = resources.ExternalDNSTTLDefault
	}

	// Add certificate annotations if enabled and not skipped
	if !skipCertificateAnnotations {
		var shouldAddCert bool
		if tenant.Spec.DNS.UseCertificateAnnotations != nil {
			// Tenant has explicit setting - use it
			shouldAddCert = *tenant.Spec.DNS.UseCertificateAnnotations
		} else {
			// Tenant setting is null - fall back to config
			shouldAddCert = config.Spec.DNS.UseCertificateAnnotations
		}

		if shouldAddCert {
			// Determine which cluster issuer to use
			if tenant.Spec.Certificates.DefaultClusterIssuer != nil {
				annotations[resources.CertManagerClusterIssuerAnnotation] = *tenant.Spec.Certificates.DefaultClusterIssuer
			} else if config.Spec.Certificates.DefaultClusterIssuer != nil {
				annotations[resources.CertManagerClusterIssuerAnnotation] = *config.Spec.Certificates.DefaultClusterIssuer
			}
		}
	}
}
