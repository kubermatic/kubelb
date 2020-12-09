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
