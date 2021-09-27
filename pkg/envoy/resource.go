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
	"strconv"
	"strings"
	"time"

	envoyAccessLog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	envoyCluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoyCore "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoyEndpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoyListener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoyFileAccessLog "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	envoyTcpProxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoyUdpProxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/udp/udp_proxy/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	corev1 "k8s.io/api/core/v1"

	kubelbiov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
)

const defaultPortName = "port"

func MapSnapshot(tcpLoadBalancer *kubelbiov1alpha1.TCPLoadBalancer, version string) cache.Snapshot {

	var listener []types.Resource
	var cluster []types.Resource

	for currentLbPort, lbServicePort := range tcpLoadBalancer.Spec.Ports {

		envoyClusterName := tcpLoadBalancer.Namespace
		if lbServicePort.Name == "" {
			lbServicePort.Name = defaultPortName + strconv.Itoa(currentLbPort)
		}
		envoyClusterName = strings.Join([]string{envoyClusterName, lbServicePort.Name}, "-")

		if lbServicePort.Protocol == corev1.ProtocolTCP {
			listener = append(listener, makeTCPListener(envoyClusterName, lbServicePort.Name, uint32(tcpLoadBalancer.Spec.Endpoints[0].Ports[currentLbPort].Port)))
		} else if lbServicePort.Protocol == corev1.ProtocolUDP {
			listener = append(listener, makeUDPListener(envoyClusterName, lbServicePort.Name, uint32(tcpLoadBalancer.Spec.Endpoints[0].Ports[currentLbPort].Port)))
		}

	}

	//multiple endpoints represent multiple clusters
	for _, lbEndpoint := range tcpLoadBalancer.Spec.Endpoints {

		for i, lbEndpointPorts := range lbEndpoint.Ports {

			var lbEndpoints []*envoyEndpoint.LbEndpoint
			envoyClusterName := tcpLoadBalancer.Namespace

			if lbEndpointPorts.Name == "" {
				lbEndpointPorts.Name = defaultPortName + strconv.Itoa(i)
			}
			envoyClusterName = strings.Join([]string{envoyClusterName, lbEndpointPorts.Name}, "-")

			//each address -> one port
			for _, lbEndpointAddress := range lbEndpoint.Addresses {
				lbEndpoints = append(lbEndpoints, makeEndpoint(lbEndpointAddress.IP, uint32(lbEndpointPorts.Port)))
			}
			cluster = append(cluster, makeCluster(envoyClusterName, lbEndpoints))
		}
	}

	return cache.NewSnapshot(
		version,
		[]types.Resource{}, // endpoints
		cluster,            //cluster
		[]types.Resource{}, //routes
		listener,           //listener
		[]types.Resource{}, // runtimes
		[]types.Resource{}, // secrets
	)
}

func makeCluster(clusterName string, lbEndpoints []*envoyEndpoint.LbEndpoint) *envoyCluster.Cluster {
	return &envoyCluster.Cluster{
		Name:                 clusterName,
		ClusterDiscoveryType: &envoyCluster.Cluster_Type{Type: envoyCluster.Cluster_STRICT_DNS},
		ConnectTimeout:       ptypes.DurationProto(5 * time.Second),
		LbPolicy:             envoyCluster.Cluster_ROUND_ROBIN,
		LoadAssignment: &envoyEndpoint.ClusterLoadAssignment{
			ClusterName: clusterName,
			Endpoints: []*envoyEndpoint.LocalityLbEndpoints{{
				LbEndpoints: lbEndpoints,
			}},
		},
		HealthChecks: []*envoyCore.HealthCheck{
			{
				Timeout:            &duration.Duration{Seconds: 5},
				Interval:           &duration.Duration{Seconds: 15},
				UnhealthyThreshold: &wrappers.UInt32Value{Value: 3},
				HealthyThreshold:   &wrappers.UInt32Value{Value: 5},
				HealthChecker: &envoyCore.HealthCheck_TcpHealthCheck_{
					TcpHealthCheck: &envoyCore.HealthCheck_TcpHealthCheck{},
				},
			},
		},
		CommonLbConfig: &envoyCluster.Cluster_CommonLbConfig{
			HealthyPanicThreshold: &envoy_type_v3.Percent{Value: 0},
		},
		DnsLookupFamily: envoyCluster.Cluster_V4_ONLY,
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
				HealthCheckConfig: &envoyEndpoint.Endpoint_HealthCheckConfig{},
			},
		},
	}
}

func makeTCPListener(clusterName string, listenerName string, listenerPort uint32) *envoyListener.Listener {

	tcpProxyAccessLog := &envoyFileAccessLog.FileAccessLog{
		Path: "/dev/stdout",
	}
	tcpProxyAccessLogAny, err := ptypes.MarshalAny(tcpProxyAccessLog)
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
				Filter: &envoyAccessLog.AccessLogFilter{
					FilterSpecifier: &envoyAccessLog.AccessLogFilter_ResponseFlagFilter{
						ResponseFlagFilter: &envoyAccessLog.ResponseFlagFilter{
							Flags: []string{"UH", "UF", "UO", "NR", "URX", "DT"},
						},
					},
				},
			},
		},
	}
	pbst, err := ptypes.MarshalAny(tcpProxy)
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

	pbst, err := ptypes.MarshalAny(udpProxy)
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