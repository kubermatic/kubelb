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
