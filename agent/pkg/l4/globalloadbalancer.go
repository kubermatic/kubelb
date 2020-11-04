package l4

import (
	"k8c.io/kubelb/manager/pkg/api/globalloadbalancer/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MapGlobalLoadBalancer(userService *corev1.Service, clusterEndpoints []string, clusterName string) *v1alpha1.GlobalLoadBalancer {

	var lbServicePorts []v1alpha1.LoadBalancerPort
	var lbEndpointSubsets []v1alpha1.LoadBalancerEndpointSubset
	var lbEndpointPorts []v1alpha1.LoadBalancerEndpointPort

	//mapping into load balancing service and endpoint subset ports
	for _, port := range userService.Spec.Ports {
		lbServicePorts = append(lbServicePorts, v1alpha1.LoadBalancerPort{
			//Todo: use annotation to set the lb port
			Port:       80,
			Protocol:   port.Protocol,
			TargetPort: port.NodePort,
		})

		lbEndpointPorts = append(lbEndpointPorts, v1alpha1.LoadBalancerEndpointPort{
			Port:     port.NodePort,
			Protocol: port.Protocol,
		})

	}

	var endpointAddresses []v1alpha1.LoadBalancerEndpointAddress
	for _, endpoint := range clusterEndpoints {
		endpointAddresses = append(endpointAddresses, v1alpha1.LoadBalancerEndpointAddress{
			IP: endpoint,
		})
	}

	lbEndpointSubsets = append(lbEndpointSubsets, v1alpha1.LoadBalancerEndpointSubset{
		Addresses: endpointAddresses,
		Ports:     lbEndpointPorts,
	})

	return &v1alpha1.GlobalLoadBalancer{
		ObjectMeta: v1.ObjectMeta{
			Name:      userService.Name,
			Namespace: clusterName,
		},
		Spec: v1alpha1.GlobalLoadBalancerSpec{
			Type:    v1alpha1.Layer4,
			Ports:   lbServicePorts,
			Subsets: lbEndpointSubsets,
		},
	}
}

func GlobalLoadBalancerIsDesiredState(actual, desired *v1alpha1.GlobalLoadBalancer) bool {

	return true
}
