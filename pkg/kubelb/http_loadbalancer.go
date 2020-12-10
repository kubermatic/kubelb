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
	"k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	"k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MapHttpLoadBalancer(userIngress *v1beta1.Ingress, clusterName string) *v1alpha1.HTTPLoadBalancer {

	var rules []v1alpha1.IngressRule

	for _, rule := range userIngress.Spec.Rules {
		var paths []v1alpha1.HTTPIngressPath

		for _, path := range rule.HTTP.Paths {
			paths = append(paths, v1alpha1.HTTPIngressPath{
				Path: path.Path,
				Backend: v1alpha1.IngressBackend{
					ServiceName: path.Backend.ServiceName,
					ServicePort: path.Backend.ServicePort,
				},
			})
		}
		rules = append(rules, v1alpha1.IngressRule{
			IngressRuleValue: v1alpha1.IngressRuleValue{
				HTTP: &v1alpha1.HTTPIngressRuleValue{
					Paths: paths,
				},
			},
		})
	}

	return &v1alpha1.HTTPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NamespacedName(&userIngress.ObjectMeta),
			Namespace: clusterName,
			Labels: map[string]string{
				LabelOriginNamespace: userIngress.Namespace,
				LabelOriginName:      userIngress.Name,
			},
		},
		Spec: v1alpha1.HTTPLoadBalancerSpec{
			Rules: rules,
		},
	}
}

func HttpLoadBalancerIsDesiredState(actual, desired *v1alpha1.HTTPLoadBalancer) bool {
	return false
}
