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
	"fmt"

	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"

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
const LabelAppKubernetesInstance = "app.kubernetes.io/instance"    // mysql-abcxzy"
const LabelAppKubernetesVersion = "app.kubernetes.io/version"      // 5.7.21
const LabelAppKubernetesComponent = "app.kubernetes.io/component"  // database
const LabelAppKubernetesPartOf = "app.kubernetes.io/part-of"       // wordpress
const LabelAppKubernetesManagedBy = "app.kubernetes.io/managed-by" // helm

const EnvoyResourceIdentifierPattern = "%s-%s-ep-%d-port-%d-%s"
const EnvoyEndpointPattern = "%s-%s-ep-%d"
const EnvoyEndpointRoutePattern = "tenant-%s-route-%s-%s"
const EnvoyRoutePortIdentifierPattern = "tenant-%s-route-%s-%s-svc-%s-port-%d-%s"
const EnvoyListenerPattern = "%v-%s"
const RouteServiceMapKey = "%s/%s"
const DefaultRouteStatus = "{}"

const ServiceKind = "Service"

const NameSuffixLength = 4

func GenerateName(appendUID bool, uid, name, namespace string) string {
	output := fmt.Sprintf("%s-%s", namespace, name)
	uidSuffix := uid[len(uid)-NameSuffixLength:]

	// If the output is longer than 63 characters, truncate the name and append a suffix
	if len(output) > 63 || (appendUID && len(output)+len(uidSuffix) > 63) {
		output = output[:63-NameSuffixLength+1]
		output = fmt.Sprintf("%s-%s", output, uidSuffix)
	} else if appendUID {
		output = fmt.Sprintf("%s-%s", output, uidSuffix)
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

func PropagateAnnotations(loadbalancer map[string]string, annotations kubelbv1alpha1.AnnotationSettings) map[string]string {
	if loadbalancer == nil {
		loadbalancer = make(map[string]string)
	}

	if annotations.PropagateAllAnnotations != nil && *annotations.PropagateAllAnnotations {
		return loadbalancer
	}
	permitted := make(map[string]string)

	if annotations.PropagatedAnnotations != nil {
		permitted = *annotations.PropagatedAnnotations
	}

	a := make(map[string]string)
	permittedMap := make(map[string][]string)
	for k, v := range permitted {
		if _, found := permittedMap[k]; !found {
			permittedMap[k] = []string{v}
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
	return a
}
