package kubelb

import (
	kubelbiov1alpha1 "k8c.io/kubelb/manager/pkg/api/kubelb.k8c.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

const LabelOriginNamespace = "kubelb.k8c.io/origin-ns"
const LabelOriginName = "kubelb.k8c.io/origin-name"

func MapTcpLoadBalancer(userService *corev1.Service, clusterEndpoints []string, clusterName string) *kubelbiov1alpha1.TCPLoadBalancer {

	var lbServicePorts []kubelbiov1alpha1.LoadBalancerPort
	var lbEndpointSubsets []kubelbiov1alpha1.LoadBalancerEndpoints
	var lbEndpointPorts []kubelbiov1alpha1.EndpointPort

	//mapping into load balancing service and endpoint subset ports
	for _, port := range userService.Spec.Ports {
		lbServicePorts = append(lbServicePorts, kubelbiov1alpha1.LoadBalancerPort{
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

	lbEndpointSubsets = append(lbEndpointSubsets, kubelbiov1alpha1.LoadBalancerEndpoints{
		Addresses: endpointAddresses,
		Ports:     lbEndpointPorts,
	})

	return &kubelbiov1alpha1.TCPLoadBalancer{
		ObjectMeta: v1.ObjectMeta{
			Name:      strings.Join([]string{userService.Namespace, userService.Name}, "-"),
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

func TcpLoadBalancerIsDesiredState(actual, desired *kubelbiov1alpha1.TCPLoadBalancer) bool {

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

	return true
}
