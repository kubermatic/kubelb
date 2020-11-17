package l4

import (
	kubelbiov1alpha1 "k8c.io/kubelb/manager/pkg/api/kubelb.k8c.io/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MapDeployment(tcpLB *kubelbiov1alpha1.TCPLoadBalancer) *appsv1.Deployment {

	var replicas int32 = 1
	var envoyListenerPorts []corev1.ContainerPort

	for _, lbServicePort := range tcpLB.Spec.Ports {
		envoyListenerPorts = append(envoyListenerPorts, corev1.ContainerPort{
			ContainerPort: lbServicePort.Port,
		})
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tcpLB.Name,
			Namespace: tcpLB.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": tcpLB.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tcpLB.Name,
					Namespace: tcpLB.Namespace,
					Labels:    map[string]string{"app": tcpLB.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  tcpLB.Name,
							Image: "envoyproxy/envoy:v1.16.0",
							Ports: envoyListenerPorts,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      tcpLB.Name,
									MountPath: "/etc/envoy",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{{
						Name: tcpLB.Name,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: tcpLB.Name},
							},
						},
					}},
				},
			},
		},
	}
}
