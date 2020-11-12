/*
Copyright 2019 The Sponson Authors.
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

package l4

import (
	"bytes"
	envoyAccessLog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	envoyBootstrap "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	envoyCluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoyCore "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoyEndpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoyListener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoyFileAccessLog "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	envoyTcpProxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes"
	kubelbiov1alpha1 "k8c.io/kubelb/manager/pkg/api/globalloadbalancer/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"strings"
	"time"
)

func toEnvoyConfig(glb *kubelbiov1alpha1.GlobalLoadBalancer, kubernetesClusterName string) string {

	var listener []*envoyListener.Listener
	var cluster []*envoyCluster.Cluster

	for _, lbServicePort := range glb.Spec.Ports {

		var envoyClusterName string
		if lbServicePort.Name != "" {
			envoyClusterName = strings.Join([]string{envoyClusterName, lbServicePort.Name}, "-")
		} else {
			envoyClusterName = kubernetesClusterName
		}

		if lbServicePort.Protocol == corev1.ProtocolTCP {
			listener = append(listener, makeTCPListener(envoyClusterName, lbServicePort.Name, uint32(lbServicePort.Port)))
		} else {
			//Todo: log unsupported
		}
	}

	//multiple endpoints represent multiple clusters
	for _, lbEndpoint := range glb.Spec.Endpoints {

		for _, lbEndpointPorts := range lbEndpoint.Ports {

			var lbEndpoints []*envoyEndpoint.LbEndpoint
			var envoyClusterName string

			if lbEndpointPorts.Name != "" {
				envoyClusterName = strings.Join([]string{envoyClusterName, lbEndpointPorts.Name}, "-")
			} else {
				envoyClusterName = kubernetesClusterName
			}

			//each address -> one port
			for _, lbEndpointAddress := range lbEndpoint.Addresses {
				lbEndpoints = append(lbEndpoints, makeEndpoint(lbEndpointAddress.IP, uint32(lbEndpointPorts.Port)))
			}

			cluster = append(cluster, makeCluster(envoyClusterName, lbEndpoints))

		}
	}

	cfg := &envoyBootstrap.Bootstrap{
		StaticResources: &envoyBootstrap.Bootstrap_StaticResources{
			Listeners: listener,
			Clusters:  cluster,
		},
	}

	var jsonBuf bytes.Buffer
	if err := new(jsonpb.Marshaler).Marshal(&jsonBuf, cfg); err != nil {
		panic(err)
	}

	return jsonBuf.String()
}

func makeCluster(clusterName string, lbEndpoints []*envoyEndpoint.LbEndpoint) *envoyCluster.Cluster {
	return &envoyCluster.Cluster{
		Name:                 clusterName,
		ConnectTimeout:       ptypes.DurationProto(5 * time.Second),
		ClusterDiscoveryType: &envoyCluster.Cluster_Type{Type: envoyCluster.Cluster_STATIC},
		LbPolicy:             envoyCluster.Cluster_ROUND_ROBIN,
		LoadAssignment: &envoyEndpoint.ClusterLoadAssignment{
			ClusterName: clusterName,
			Endpoints: []*envoyEndpoint.LocalityLbEndpoints{{
				LbEndpoints: lbEndpoints,
			}},
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
