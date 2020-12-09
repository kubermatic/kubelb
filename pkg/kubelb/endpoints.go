package kubelb

import corev1 "k8s.io/api/core/v1"

type Endpoints struct {
	ClusterEndpoints    []string
	EndpointAddressType corev1.NodeAddressType
}

func (r *Endpoints) GetEndpoints(nodes *corev1.NodeList) []string {
	var clusterEndpoints []string
	for _, node := range nodes.Items {
		var internalIp string
		for _, address := range node.Status.Addresses {
			if address.Type == r.EndpointAddressType {
				internalIp = address.Address
			}
		}
		clusterEndpoints = append(clusterEndpoints, internalIp)
	}
	return clusterEndpoints
}

func (r *Endpoints) EndpointIsDesiredState(desired *corev1.NodeList) bool {

	desiredEndpoints := r.GetEndpoints(desired)

	if len(r.ClusterEndpoints) != len(desiredEndpoints) {
		return false
	}

	for _, endpoint := range desiredEndpoints {
		if !contains(r.ClusterEndpoints, endpoint) {
			return false
		}
	}

	return true
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
