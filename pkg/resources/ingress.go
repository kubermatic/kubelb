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

package resources

import (
	kubelbiov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	netv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MapIngress(httpLoadBalancer *kubelbiov1alpha1.HTTPLoadBalancer) *netv1beta1.Ingress {

	var rules []netv1beta1.IngressRule

	for _, rule := range httpLoadBalancer.Spec.Rules {

		var paths []netv1beta1.HTTPIngressPath

		for _, path := range rule.HTTP.Paths {
			paths = append(paths, netv1beta1.HTTPIngressPath{
				Path: path.Path,
				Backend: netv1beta1.IngressBackend{
					ServiceName: path.Backend.ServiceName,
					ServicePort: path.Backend.ServicePort,
				},
			})
		}

		rules = append(rules, netv1beta1.IngressRule{
			IngressRuleValue: netv1beta1.IngressRuleValue{
				HTTP: &netv1beta1.HTTPIngressRuleValue{
					Paths: paths,
				},
			},
		})
	}

	return &netv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      httpLoadBalancer.Name,
			Namespace: httpLoadBalancer.Namespace,
			Labels:    map[string]string{"app": httpLoadBalancer.Name},
		},
		Spec: netv1beta1.IngressSpec{
			Rules: rules,
		},
	}

}
