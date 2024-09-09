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

package envoy

import (
	"context"
	"fmt"
	"time"

	envoyAccessLog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	envoyCluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoyCore "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoyEndpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoyListener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoyFileAccessLog "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	envoyTcpProxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoyUdpProxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/udp/udp_proxy/v3"
	envoytypev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
        ctrl "sigs.k8s.io/controller-runtime"
	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	portlookup "k8c.io/kubelb/internal/port-lookup"

	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	endpointAddressReferencePattern = "%s-address-%s"
)

func MapSnapshot(ctx context.Context, client ctrlclient.Client, loadBalancers []kubelbv1alpha1.LoadBalancer, routes []kubelbv1alpha1.Route, portAllocator *portlookup.PortAllocator, globalEnvoyProxyTopology bool) (*envoycache.Snapshot, error) {
	var listener []types.Resource
	var cluster []types.Resource
	
        log := ctrl.LoggerFrom(ctx)
	addressesMap := make(map[string][]kubelbv1alpha1.EndpointAddress)
	for _, lb := range loadBalancers {
		// multiple endpoints represent multiple clusters
		for i, lbEndpoint := range lb.Spec.Endpoints {
			if lbEndpoint.AddressesReference != nil {
				// Check if map already contains the key
				if val, ok := addressesMap[fmt.Sprintf(endpointAddressReferencePattern, lb.Namespace, lbEndpoint.AddressesReference.Name)]; ok {
					lb.Spec.Endpoints[i].Addresses = val
					continue
				}

				// Load addresses from reference
				var addresses kubelbv1alpha1.Addresses
				if err := client.Get(ctx, ctrlclient.ObjectKey{Namespace: lb.Namespace, Name: lbEndpoint.AddressesReference.Name}, &addresses); err != nil {
					return nil, fmt.Errorf("failed to get addresses: %w", err)
				}
				addressesMap[fmt.Sprintf(endpointAddressReferencePattern, lb.Namespace, lbEndpoint.AddressesReference.Name)] = addresses.Spec.Addresses
				lb.Spec.Endpoints[i].Addresses = addresses.Spec.Addresses
				lbEndpoint.Addresses = addresses.Spec.Addresses
			}

			for _, lbEndpointPort := range lbEndpoint.Ports {
				var lbEndpoints []*envoyEndpoint.LbEndpoint
				key := fmt.Sprintf(kubelb.EnvoyResourceIdentifierPattern, lb.Namespace, lb.Name, i, lbEndpointPort.Port, lbEndpointPort.Protocol)

				// each address -> one port
				for _, lbEndpointAddress := range lbEndpoint.Addresses {
					lbEndpoints = append(lbEndpoints, makeEndpoint(lbEndpointAddress.IP, uint32(lbEndpointPort.Port)))
				}

				port := uint32(lbEndpointPort.Port)
				if globalEnvoyProxyTopology && portAllocator != nil {
					endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointPattern, lb.Namespace, lb.Name, i)
					portKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, lbEndpointPort.Port, lbEndpointPort.Protocol)
					if value, exists := portAllocator.Lookup(endpointKey, portKey); exists {
						port = uint32(value)
					}
				}

				if lbEndpointPort.Protocol == corev1.ProtocolTCP {
					listener = append(listener, makeTCPListener(key, key, port))
				} else if lbEndpointPort.Protocol == corev1.ProtocolUDP {
					listener = append(listener, makeUDPListener(key, key, port))
				}
				log.V(2).Info("adding cluster", "key", key, "ports", len(lbEndpoint.Ports))
				cluster = append(cluster, makeCluster(key, lbEndpoints))
			}
		}
	}

	for _, route := range routes {
		if route.Spec.Source.Kubernetes == nil {
			continue
		}
		for i, routeendpoint := range route.Spec.Endpoints {
			if routeendpoint.AddressesReference != nil {
				// Check if map already contains the key
				if val, ok := addressesMap[fmt.Sprintf(endpointAddressReferencePattern, route.Namespace, routeendpoint.AddressesReference.Name)]; ok {
					route.Spec.Endpoints[i].Addresses = val
					continue
				}

				// Load addresses from reference
				var addresses kubelbv1alpha1.Addresses
				if err := client.Get(ctx, ctrlclient.ObjectKey{Namespace: route.Namespace, Name: routeendpoint.AddressesReference.Name}, &addresses); err != nil {
					return nil, fmt.Errorf("failed to get addresses: %w", err)
				}
				addressesMap[fmt.Sprintf(endpointAddressReferencePattern, route.Namespace, routeendpoint.AddressesReference.Name)] = addresses.Spec.Addresses
				route.Spec.Endpoints[i].Addresses = addresses.Spec.Addresses
			}
		}
		source := route.Spec.Source.Kubernetes
		for _, svc := range source.Services {
			endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointRoutePattern, route.Namespace, svc.Namespace, svc.Name)
			for _, port := range svc.Spec.Ports {
				portLookupKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, port.Port, port.Protocol)
				var lbEndpoints []*envoyEndpoint.LbEndpoint
				for _, address := range route.Spec.Endpoints {
					for _, routeEndpoints := range address.Addresses {
						lbEndpoints = append(lbEndpoints, makeEndpoint(routeEndpoints.IP, uint32(port.NodePort)))
					}
				}

				listenerPort := uint32(port.Port)
				if value, exists := portAllocator.Lookup(endpointKey, portLookupKey); exists {
					listenerPort = uint32(value)
				}

				key := fmt.Sprintf(kubelb.EnvoyRoutePortIdentifierPattern, route.Namespace, svc.Namespace, svc.Name, svc.UID, port.Port, port.Protocol)

				if port.Protocol == corev1.ProtocolTCP {
					listener = append(listener, makeTCPListener(key, key, listenerPort))
				} else if port.Protocol == corev1.ProtocolUDP {
					listener = append(listener, makeUDPListener(key, key, listenerPort))
				}
				cluster = append(cluster, makeCluster(key, lbEndpoints))
			}
		}
	}

	var content []byte
	var resources []types.Resource
	resources = append(resources, cluster...)
	resources = append(resources, listener...)
	for _, r := range resources {
		mr, err := envoycache.MarshalResource(r)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal resource: %w", err)
		}
		content = append(content, mr...)
	}
	version := envoycache.HashResource(content)

	return envoycache.NewSnapshot(
		version,
		map[resource.Type][]types.Resource{
			resource.ClusterType:  cluster,
			resource.ListenerType: listener,
		},
	)
}

func makeCluster(clusterName string, lbEndpoints []*envoyEndpoint.LbEndpoint) *envoyCluster.Cluster {
	return &envoyCluster.Cluster{
		Name:                 clusterName,
		ConnectTimeout:       durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &envoyCluster.Cluster_Type{Type: envoyCluster.Cluster_STRICT_DNS},
		LbPolicy:             envoyCluster.Cluster_ROUND_ROBIN,
		LoadAssignment: &envoyEndpoint.ClusterLoadAssignment{
			ClusterName: clusterName,
			Endpoints: []*envoyEndpoint.LocalityLbEndpoints{{
				LbEndpoints: lbEndpoints,
			}},
		},
		DnsLookupFamily: envoyCluster.Cluster_V4_ONLY,
		HealthChecks: []*envoyCore.HealthCheck{
			{
				Timeout:            &duration.Duration{Seconds: 5},
				Interval:           &duration.Duration{Seconds: 5},
				UnhealthyThreshold: &wrappers.UInt32Value{Value: 3},
				HealthyThreshold:   &wrappers.UInt32Value{Value: 3},
				HealthChecker: &envoyCore.HealthCheck_TcpHealthCheck_{
					TcpHealthCheck: &envoyCore.HealthCheck_TcpHealthCheck{},
				},
			},
		},
		CommonLbConfig: &envoyCluster.Cluster_CommonLbConfig{
			HealthyPanicThreshold: &envoytypev3.Percent{Value: 0},
		},
	}
}

func makeEndpoint(address string, port uint32) *envoyEndpoint.LbEndpoint {
	return &envoyEndpoint.LbEndpoint{
		HostIdentifier: &envoyEndpoint.LbEndpoint_Endpoint{
			Endpoint: &envoyEndpoint.Endpoint{
				Address: &envoyCore.Address{
					Address: &envoyCore.Address_SocketAddress{
						SocketAddress: &envoyCore.SocketAddress{
							Protocol: envoyCore.SocketAddress_TCP,
							Address:  address,
							PortSpecifier: &envoyCore.SocketAddress_PortValue{
								PortValue: port,
							},
						},
					},
				},
			},
		},
	}
}

func makeTCPListener(clusterName string, listenerName string, listenerPort uint32) *envoyListener.Listener {
	tcpProxyAccessLog := &envoyFileAccessLog.FileAccessLog{
		Path: "/dev/stdout",
	}
	tcpProxyAccessLogAny, err := anypb.New(tcpProxyAccessLog)
	if err != nil {
		panic(err)
	}

	tcpProxy := &envoyTcpProxy.TcpProxy{
		StatPrefix: listenerName,
		ClusterSpecifier: &envoyTcpProxy.TcpProxy_Cluster{
			Cluster: clusterName,
		},
		AccessLog: []*envoyAccessLog.AccessLog{
			{
				Name: "envoy.file_access_log",
				ConfigType: &envoyAccessLog.AccessLog_TypedConfig{
					TypedConfig: tcpProxyAccessLogAny,
				},
			},
		},
	}
	pbst, err := anypb.New(tcpProxy)
	if err != nil {
		panic(err)
	}

	return &envoyListener.Listener{
		Name: listenerName,
		Address: &envoyCore.Address{
			Address: &envoyCore.Address_SocketAddress{
				SocketAddress: &envoyCore.SocketAddress{
					Protocol: envoyCore.SocketAddress_TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &envoyCore.SocketAddress_PortValue{
						PortValue: listenerPort,
					},
				},
			},
		},
		FilterChains: []*envoyListener.FilterChain{{
			Filters: []*envoyListener.Filter{{
				Name: wellknown.TCPProxy,
				ConfigType: &envoyListener.Filter_TypedConfig{
					TypedConfig: pbst,
				},
			}},
		}},
	}
}

func makeUDPListener(clusterName string, listenerName string, listenerPort uint32) *envoyListener.Listener {
	udpProxy := &envoyUdpProxy.UdpProxyConfig{
		StatPrefix: listenerName,
		RouteSpecifier: &envoyUdpProxy.UdpProxyConfig_Cluster{
			Cluster: clusterName,
		},
	}

	pbst, err := anypb.New(udpProxy)
	if err != nil {
		panic(err)
	}

	return &envoyListener.Listener{
		Name: listenerName,
		Address: &envoyCore.Address{
			Address: &envoyCore.Address_SocketAddress{
				SocketAddress: &envoyCore.SocketAddress{
					Protocol: envoyCore.SocketAddress_UDP,
					Address:  "0.0.0.0",
					PortSpecifier: &envoyCore.SocketAddress_PortValue{
						PortValue: listenerPort,
					},
				},
			},
		},
		ListenerFilters: []*envoyListener.ListenerFilter{
			{
				Name: "envoy.filters.udp_listener.udp_proxy",
				ConfigType: &envoyListener.ListenerFilter_TypedConfig{
					TypedConfig: pbst,
				},
			},
		},
		ReusePort: true,
	}
}
