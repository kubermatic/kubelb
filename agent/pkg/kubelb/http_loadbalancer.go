package kubelb

import (
	"k8c.io/kubelb/manager/pkg/api/kubelb.k8c.io/v1alpha1"
	"k8s.io/api/networking/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		ObjectMeta: v1.ObjectMeta{
			Name:      userIngress.Name,
			Namespace: clusterName,
		},
		Spec: v1alpha1.HTTPLoadBalancerSpec{
			Rules: rules,
		},
	}
}

func HttpLoadBalancerIsDesiredState(actual, desired *v1alpha1.HTTPLoadBalancer) bool {
	return false
}
