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
							Image: "envoyproxy/envoy:v1.16-latest",
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
