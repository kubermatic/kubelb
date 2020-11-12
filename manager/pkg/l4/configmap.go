package l4

import (
	kubelbiov1alpha1 "k8c.io/kubelb/manager/pkg/api/globalloadbalancer/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MapConfigmap(glb *kubelbiov1alpha1.GlobalLoadBalancer, clusterName string) *corev1.ConfigMap {

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      glb.Name,
			Namespace: glb.Namespace,
		},
		Data: map[string]string{"envoy.yaml": toEnvoyConfig(glb, clusterName)},
	}
}
