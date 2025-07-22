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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"

	kubelbiov1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	return &kubelbiov1alpha1.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(userService.UID),
			Namespace: clusterName,
			Labels: map[string]string{
				LabelOriginNamespace: userService.Namespace,
				LabelOriginName:      userService.Name,
				LabelTenantName:      clusterName,
			},
			Annotations: userService.Annotations,
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

	return reflect.DeepEqual(actual.Annotations, desired.Annotations)
}

// GenerateHostname generates a random hostname using tenant or config wildcard domain as base
func GenerateHostname(tenant kubelbiov1alpha1.DNSSettings, config kubelbiov1alpha1.ConfigDNSSettings) string {
	// Determine the base domain to use
	var baseDomain string

	// Prefer tenant wildcard domain over global config
	if tenant.WildcardDomain != nil && *tenant.WildcardDomain != "" {
		baseDomain = *tenant.WildcardDomain
	} else if config.WildcardDomain != "" {
		baseDomain = config.WildcardDomain
	} else {
		// No wildcard domain configured
		return ""
	}

	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("lb-%d.%s", metav1.Now().Unix(), baseDomain)
	}

	// Convert to hex string
	randomPrefix := hex.EncodeToString(randomBytes)

	// Remove leading asterisk(*) from wildcard domain if present
	baseDomain = strings.TrimPrefix(baseDomain, "*.")
	baseDomain = strings.TrimPrefix(baseDomain, "**.")
	baseDomain = strings.TrimPrefix(baseDomain, "*")
	return fmt.Sprintf("%s.%s", randomPrefix, baseDomain)
}

// ShouldConfigureHostname determines whether hostname configuration should be enabled
func ShouldConfigureHostname(loadBalancer *kubelbiov1alpha1.LoadBalancer, tenant *kubelbiov1alpha1.Tenant, config *kubelbiov1alpha1.Config) bool {
	if loadBalancer.Spec.Hostname != "" {
		// Ensure that explicit hostname is allowed at tenant or global level.
		if tenant.Spec.DNS.AllowExplicitHostnames != nil && *tenant.Spec.DNS.AllowExplicitHostnames {
			return true
		}
		if config.Spec.DNS.AllowExplicitHostnames && tenant.Spec.DNS.AllowExplicitHostnames == nil {
			return true
		}
		return false
	}

	// For wildcard domain, we need to check if the annotation to request wildcard domain is set.
	if val, ok := loadBalancer.Annotations[AnnotationRequestWildcardDomain]; !ok {
		return false
	} else if val != "true" {
		return false
	}

	// Request for wildcard domain.
	if tenant.Spec.DNS.WildcardDomain != nil && *tenant.Spec.DNS.WildcardDomain != "" {
		return true
	}
	if config.Spec.DNS.WildcardDomain != "" && tenant.Spec.DNS.WildcardDomain == nil {
		return true
	}
	return false
}

// PortAllocator defines the interface for port allocation
type PortAllocator interface {
	Lookup(endpointKey, portKey string) (int, bool)
}

// CreateServicePorts creates service ports for the load balancer
func CreateServicePorts(loadBalancer *kubelbiov1alpha1.LoadBalancer, existingService *corev1.Service, portAllocator PortAllocator, topology string) []corev1.ServicePort {
	desiredPorts := make([]corev1.ServicePort, 0, len(loadBalancer.Spec.Ports))
	for i, lbPort := range loadBalancer.Spec.Ports {
		// If the port name is not set, we match the port by index.
		targetPort := loadBalancer.Spec.Endpoints[0].Ports[i].Port
		// Find the port name in the endpoints ports.
		for _, port := range loadBalancer.Spec.Endpoints[0].Ports {
			if port.Name == lbPort.Name {
				targetPort = port.Port
				break
			}
		}

		// For global topology, look up allocated port
		if topology == "global" {
			endpointKey := fmt.Sprintf(EnvoyEndpointPattern, loadBalancer.Namespace, loadBalancer.Name, 0)
			portKey := fmt.Sprintf(EnvoyListenerPattern, targetPort, lbPort.Protocol)
			if value, exists := portAllocator.Lookup(endpointKey, portKey); exists {
				targetPort = int32(value)
			}
		}

		// Try to find matching existing port to preserve NodePort if possible
		var existingPort *corev1.ServicePort
		for j := range existingService.Spec.Ports {
			if existingService.Spec.Ports[j].Name == lbPort.Name || (existingService.Spec.Ports[j].Port == lbPort.Port && existingService.Spec.Ports[j].Protocol == lbPort.Protocol) {
				existingPort = &existingService.Spec.Ports[j]
				break
			}
		}

		port := corev1.ServicePort{
			Name:       lbPort.Name,
			Port:       lbPort.Port,
			TargetPort: intstr.FromInt(int(targetPort)),
			Protocol:   lbPort.Protocol,
		}

		// Preserve NodePort if it exists and matches the desired configuration
		if existingPort != nil && existingPort.NodePort != 0 {
			port.NodePort = existingPort.NodePort
		}

		desiredPorts = append(desiredPorts, port)
	}

	// Sort ports by name for consistent ordering
	sort.Slice(desiredPorts, func(i, j int) bool {
		return desiredPorts[i].Name < desiredPorts[j].Name
	})

	return desiredPorts
}
