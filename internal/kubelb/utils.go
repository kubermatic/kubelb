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

func PropagateAnnotations(loadbalancer map[string]string, annotations kubelbv1alpha1.AnnotationSettings, resource kubelbv1alpha1.AnnotatedResource) map[string]string {
	if loadbalancer == nil {
		loadbalancer = make(map[string]string)
	}

	if annotations.PropagateAllAnnotations != nil && *annotations.PropagateAllAnnotations {
		return applyDefaultAnnotations(loadbalancer, annotations, resource)
	}
	permitted := make(map[string]string)

	if annotations.PropagatedAnnotations != nil {
		permitted = *annotations.PropagatedAnnotations
	}

	a := make(map[string]string)
	permittedMap := make(map[string][]string)
	for k, v := range permitted {
		if _, found := permittedMap[k]; !found {
			if v == "" {
				permittedMap[k] = []string{}
			} else {
				filterValues := strings.Split(v, ",")
				for i, v := range filterValues {
					filterValues[i] = strings.TrimSpace(v)
				}
				permittedMap[k] = filterValues
			}
		}
	}

	for k, v := range loadbalancer {
		if valuesFilter, ok := permittedMap[k]; ok {
			if len(valuesFilter) == 0 {
				a[k] = v
			} else {
				for _, vf := range valuesFilter {
					if v == vf {
						a[k] = v
						break
					}
				}
			}
		}
	}
	return applyDefaultAnnotations(a, annotations, resource)
}

func applyDefaultAnnotations(loadbalancer map[string]string, annotations kubelbv1alpha1.AnnotationSettings, resource kubelbv1alpha1.AnnotatedResource) map[string]string {
	if annotations.DefaultAnnotations != nil {
		if _, ok := annotations.DefaultAnnotations[resource]; ok {
			// Merge the default annotations with the loadbalancer annotations while giving precedence to the loadbalancer annotations. If an annotation
			// already exists in the loadbalancer, it will not be overridden by the default annotations.
			for k, v := range annotations.DefaultAnnotations[resource] {
				if _, ok := loadbalancer[k]; !ok {
					loadbalancer[k] = v
				}
			}
		}

		// Apply the default annotations for all resources if they are set.
		if _, ok := annotations.DefaultAnnotations[kubelbv1alpha1.AnnotatedResourceAll]; ok {
			// Merge the default annotations with the loadbalancer annotations while giving precedence to the loadbalancer annotations. If an annotation
			// already exists in the loadbalancer, it will not be overridden by the default annotations.
			for k, v := range annotations.DefaultAnnotations[kubelbv1alpha1.AnnotatedResourceAll] {
				if _, ok := loadbalancer[k]; !ok {
					loadbalancer[k] = v
				}
			}
		}
	}
	return loadbalancer
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
