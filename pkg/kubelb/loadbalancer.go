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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubelbiov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
)

func MapLoadBalancer(userService *corev1.Service, clusterEndpoints []string, clusterName string) *kubelbiov1alpha1.TCPLoadBalancer {
	var lbServicePorts []kubelbiov1alpha1.TCPLoadBalancerPort
	var lbEndpointSubsets []kubelbiov1alpha1.TCPLoadBalancerEndpoints
	var lbEndpointPorts []kubelbiov1alpha1.EndpointPort

	// mapping into load balancing service and endpoint subset ports
	for _, port := range userService.Spec.Ports {
		lbServicePorts = append(lbServicePorts, kubelbiov1alpha1.TCPLoadBalancerPort{
			Name:     port.Name,
			Port:     port.Port,
			Protocol: port.Protocol,
		})

		lbEndpointPorts = append(lbEndpointPorts, kubelbiov1alpha1.EndpointPort{
			Name:     port.Name,
			Port:     port.NodePort,
			Protocol: port.Protocol,
		})
	}

	var endpointAddresses []kubelbiov1alpha1.EndpointAddress
	for _, endpoint := range clusterEndpoints {
		endpointAddresses = append(endpointAddresses, kubelbiov1alpha1.EndpointAddress{
			IP: endpoint,
		})
	}

	lbEndpointSubsets = append(lbEndpointSubsets, kubelbiov1alpha1.TCPLoadBalancerEndpoints{
		Addresses: endpointAddresses,
		Ports:     lbEndpointPorts,
	})

	return &kubelbiov1alpha1.TCPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NamespacedName(&userService.ObjectMeta),
			Namespace: clusterName,
			Labels: map[string]string{
				LabelOriginNamespace: userService.Namespace,
				LabelOriginName:      userService.Name,
			},
		},
		Spec: kubelbiov1alpha1.TCPLoadBalancerSpec{
			Ports:     lbServicePorts,
			Endpoints: lbEndpointSubsets,
			Type:      userService.Spec.Type,
		},
	}
}

func LoadBalancerIsDesiredState(actual, desired *kubelbiov1alpha1.TCPLoadBalancer) bool {
	if actual.Spec.Type != desired.Spec.Type {
		return false
	}

	if len(actual.Spec.Ports) != len(desired.Spec.Ports) {
		return false
	}

	loadBalancerPortIsDesiredState := func(actual, desired kubelbiov1alpha1.TCPLoadBalancerPort) bool {
		return actual.Protocol == desired.Protocol &&
			actual.Port == desired.Port
	}

	for i := 0; i < len(desired.Spec.Ports); i++ {
		if !loadBalancerPortIsDesiredState(actual.Spec.Ports[i], desired.Spec.Ports[i]) {
			return false
		}
	}

	if len(actual.Spec.Endpoints) != len(desired.Spec.Endpoints) {
		return false
	}

	endpointPortIsDesiredState := func(actual, desired kubelbiov1alpha1.EndpointPort) bool {
		return actual.Port == desired.Port &&
			actual.Protocol == desired.Protocol
	}

	endpointAddressIsDesiredState := func(actual, desired kubelbiov1alpha1.EndpointAddress) bool {
		return actual.Hostname == desired.Hostname &&
			actual.IP == desired.IP
	}

	for i := 0; i < len(desired.Spec.Endpoints); i++ {

		if len(desired.Spec.Endpoints[i].Addresses) != len(actual.Spec.Endpoints[i].Addresses) {
			return false
		}

		if len(desired.Spec.Endpoints[i].Ports) != len(actual.Spec.Endpoints[i].Ports) {
			return false
		}

		for a := 0; a < len(desired.Spec.Endpoints[i].Addresses); a++ {
			if !endpointAddressIsDesiredState(desired.Spec.Endpoints[i].Addresses[a], actual.Spec.Endpoints[i].Addresses[a]) {
				return false
			}
		}
		for p := 0; p < len(desired.Spec.Endpoints[i].Ports); p++ {
			if !endpointPortIsDesiredState(desired.Spec.Endpoints[i].Ports[p], actual.Spec.Endpoints[i].Ports[p]) {
				return false
			}
		}
	}

	return true
}
