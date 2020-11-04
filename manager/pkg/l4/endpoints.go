package l4

import (
	kubelbiov1alpha1 "k8c.io/kubelb/manager/pkg/api/globalloadbalancer/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MapEndpoints(glb *kubelbiov1alpha1.GlobalLoadBalancer) *corev1.Endpoints {

	var subsets []corev1.EndpointSubset

	for _, lbEndpointsSubset := range glb.Spec.Subsets {

		var endpointPorts []corev1.EndpointPort
		var endpointAddresses []corev1.EndpointAddress

		for _, endpointPort := range lbEndpointsSubset.Ports {
			endpointPorts = append(endpointPorts, corev1.EndpointPort{
				Port:     endpointPort.Port,
				Protocol: endpointPort.Protocol,
			})
		}

		for _, endpointAddress := range lbEndpointsSubset.Addresses {
			endpointAddresses = append(endpointAddresses, corev1.EndpointAddress{
				IP:       endpointAddress.IP,
				Hostname: endpointAddress.Hostname,
			})
		}

		subsets = append(subsets, corev1.EndpointSubset{
			Ports:     endpointPorts,
			Addresses: endpointAddresses,
		})
	}

	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      glb.Name,
			Namespace: glb.Namespace,
		},
		Subsets: subsets,
	}
}

func EndpointsIsDesiredState(actual, desired *corev1.Endpoints) bool {

	if len(actual.Subsets) != len(desired.Subsets) {
		return false
	}

	endpointPortIsDesiredState := func(actual, desired corev1.EndpointPort) bool {
		return actual.Port == desired.Port &&
			actual.Protocol == desired.Protocol
	}

	endpointAddressIsDesiredState := func(actual, desired corev1.EndpointAddress) bool {
		return actual.Hostname == desired.Hostname &&
			actual.IP == desired.IP
	}

	for i := 0; i < len(desired.Subsets); i++ {

		for a := 0; a < len(desired.Subsets[i].Addresses); a++ {
			if !endpointAddressIsDesiredState(desired.Subsets[i].Addresses[a], actual.Subsets[i].Addresses[a]) {
				return false
			}
		}
		for p := 0; p < len(desired.Subsets[i].Ports); p++ {
			if !endpointPortIsDesiredState(desired.Subsets[i].Ports[p], actual.Subsets[i].Ports[p]) {
				return false
			}
		}
	}

	return true

}
