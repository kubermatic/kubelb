package resources

import (
	kubelbiov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MapDeployment(tcpLoadBalancer *kubelbiov1alpha1.TCPLoadBalancer, conf string) *appsv1.Deployment {

	var replicas int32 = 1
	var envoyListenerPorts []corev1.ContainerPort

	for _, lbServicePort := range tcpLoadBalancer.Spec.Ports {
		envoyListenerPorts = append(envoyListenerPorts, corev1.ContainerPort{
			ContainerPort: lbServicePort.Port,
		})
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tcpLoadBalancer.Name,
			Namespace: tcpLoadBalancer.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": tcpLoadBalancer.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tcpLoadBalancer.Name,
					Namespace: tcpLoadBalancer.Namespace,
					Labels:    map[string]string{"app": tcpLoadBalancer.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  tcpLoadBalancer.Name,
							Image: "envoyproxy/envoy:v1.16.0",
							Args: []string{
								"--config-yaml", conf,
								"--service-node", tcpLoadBalancer.Name,
								"--service-cluster", tcpLoadBalancer.Namespace,
							},
							Ports: envoyListenerPorts,
						},
					},
				},
			},
		},
	}
}
