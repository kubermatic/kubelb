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
	"fmt"
	"strings"

	kubelbiov1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	k8sutils "k8c.io/kubelb/internal/util/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MapLoadBalancer(userService *corev1.Service, clusterEndpoints []string, useAddressesReference bool, clusterName string) *kubelbiov1alpha1.LoadBalancer {
	var lbServicePorts []kubelbiov1alpha1.LoadBalancerPort
	var lbEndpointSubsets []kubelbiov1alpha1.LoadBalancerEndpoints
	var lbEndpointPorts []kubelbiov1alpha1.EndpointPort

	// mapping into load balancing service and endpoint subset ports
	for _, port := range userService.Spec.Ports {
		// Add a name for port if not set.
		name := fmt.Sprintf("%d-%s", port.Port, strings.ToLower(string(port.Protocol)))
		if port.Name != "" {
			name = strings.ToLower(port.Name)
		}

		lbServicePorts = append(lbServicePorts, kubelbiov1alpha1.LoadBalancerPort{
			Name:     name,
			Port:     port.Port,
			Protocol: port.Protocol,
		})

		lbEndpointPorts = append(lbEndpointPorts, kubelbiov1alpha1.EndpointPort{
			Name:     name,
			Port:     port.NodePort,
			Protocol: port.Protocol,
		})
	}

	lbEndpoints := kubelbiov1alpha1.LoadBalancerEndpoints{
		Ports: lbEndpointPorts,
	}

	if useAddressesReference {
		lbEndpoints.AddressesReference = &corev1.ObjectReference{
			Name: kubelbiov1alpha1.DefaultAddressName,
		}
	} else {
		var endpointAddresses []kubelbiov1alpha1.EndpointAddress
		for _, endpoint := range clusterEndpoints {
			endpointAddresses = append(endpointAddresses, kubelbiov1alpha1.EndpointAddress{
				IP: endpoint,
			})
		}
		lbEndpoints.Addresses = endpointAddresses
	}

	lbEndpointSubsets = append(lbEndpointSubsets, lbEndpoints)

	// Last applied configuration annotation is not relevant for the load balancer.
	annotations := userService.Annotations
	if annotations != nil {
		delete(annotations, corev1.LastAppliedConfigAnnotation)
	}

	return &kubelbiov1alpha1.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(userService.UID),
			Namespace: clusterName,
			Labels: map[string]string{
				LabelOriginNamespace: userService.Namespace,
				LabelOriginName:      userService.Name,
				LabelTenantName:      clusterName,
			},
			Annotations: annotations,
		},
		Spec: kubelbiov1alpha1.LoadBalancerSpec{
			Ports:     lbServicePorts,
			Endpoints: lbEndpointSubsets,
			Type:      userService.Spec.Type,
		},
	}
}

func LoadBalancerIsDesiredState(actual, desired *kubelbiov1alpha1.LoadBalancer) bool {
	if actual.Spec.Type != desired.Spec.Type {
		return false
	}

	if len(actual.Spec.Ports) != len(desired.Spec.Ports) {
		return false
	}

	loadBalancerPortIsDesiredState := func(actual, desired kubelbiov1alpha1.LoadBalancerPort) bool {
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

	return k8sutils.CompareAnnotations(actual.Annotations, desired.Annotations)
}
