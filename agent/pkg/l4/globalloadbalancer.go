package l4

import (
	"k8c.io/kubelb/manager/pkg/api/globalloadbalancer/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MapGlobalLoadBalancer(userService *corev1.Service, clusterEndpoints []string, clusterName string) *v1alpha1.GlobalLoadBalancer {

	var lbServicePorts []v1alpha1.LoadBalancerPort
	var lbEndpointSubsets []v1alpha1.LoadBalancerEndpoints
	var lbEndpointPorts []v1alpha1.EndpointPort

	//mapping into load balancing service and endpoint subset ports
	for _, port := range userService.Spec.Ports {
		lbServicePorts = append(lbServicePorts, v1alpha1.LoadBalancerPort{
			//Todo: use annotation to set the lb port
			Port:     80,
			Protocol: port.Protocol,
		})

		lbEndpointPorts = append(lbEndpointPorts, v1alpha1.EndpointPort{
			Port:     port.NodePort,
			Protocol: port.Protocol,
		})

	}

	var endpointAddresses []v1alpha1.EndpointAddress
	for _, endpoint := range clusterEndpoints {
		endpointAddresses = append(endpointAddresses, v1alpha1.EndpointAddress{
			IP: endpoint,
		})
	}

	lbEndpointSubsets = append(lbEndpointSubsets, v1alpha1.LoadBalancerEndpoints{
		Addresses: endpointAddresses,
		Ports:     lbEndpointPorts,
	})

	return &v1alpha1.GlobalLoadBalancer{
		ObjectMeta: v1.ObjectMeta{
			Name:      userService.Name,
			Namespace: clusterName,
		},
		Spec: v1alpha1.GlobalLoadBalancerSpec{
			Type:      v1alpha1.Layer4,
			Ports:     lbServicePorts,
			Endpoints: lbEndpointSubsets,
		},
	}
}

func GlobalLoadBalancerIsDesiredState(actual, desired *v1alpha1.GlobalLoadBalancer) bool {

	//Type
	if actual.Spec.Type != desired.Spec.Type {
		return false
	}

	if len(actual.Spec.Ports) != len(desired.Spec.Ports) {
		return false
	}

	loadBalancerPortIsDesiredState := func(actual, desired v1alpha1.LoadBalancerPort) bool {
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

	endpointPortIsDesiredState := func(actual, desired v1alpha1.EndpointPort) bool {
		return actual.Port == desired.Port &&
			actual.Protocol == desired.Protocol
	}

	endpointAddressIsDesiredState := func(actual, desired v1alpha1.EndpointAddress) bool {
		return actual.Hostname == desired.Hostname &&
			actual.IP == desired.IP
	}

	for i := 0; i < len(desired.Spec.Endpoints); i++ {

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
