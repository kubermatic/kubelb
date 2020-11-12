package l4

import (
	kubelbiov1alpha1 "k8c.io/kubelb/manager/pkg/api/globalloadbalancer/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MapDeployment(glb *kubelbiov1alpha1.GlobalLoadBalancer) *appsv1.Deployment {

	var replicas int32 = 1
	var envoyListenerPorts []corev1.ContainerPort

	for _, lbServicePort := range glb.Spec.Ports {
		envoyListenerPorts = append(envoyListenerPorts, corev1.ContainerPort{
			ContainerPort: lbServicePort.Port,
		})
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      glb.Name,
			Namespace: glb.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": glb.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      glb.Name,
					Namespace: glb.Namespace,
					Labels:    map[string]string{"app": glb.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  glb.Name,
							Image: "envoyproxy/envoy:v1.16.0",
							Ports: envoyListenerPorts,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      glb.Name,
									MountPath: "/etc/envoy",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{{
						Name: glb.Name,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: glb.Name},
							},
						},
					}},
				},
			},
		},
	}
}
