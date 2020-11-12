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
	accessLog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	bootstrap "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoyEndpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	fileAccessLog "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	tp "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes"
	kubelbiov1alpha1 "k8c.io/kubelb/manager/pkg/api/globalloadbalancer/v1alpha1"
	"time"
)

const (
	ListenerName = "listener_0"
	ListenerPort = 8080
)

func toEnvoyConfig(endpoints []kubelbiov1alpha1.LoadBalancerEndpoints, clusterName string) string {

	cfg := &bootstrap.Bootstrap{
		StaticResources: &bootstrap.Bootstrap_StaticResources{
			Listeners: []*listener.Listener{makeTCPListener(clusterName)},
			Clusters:  []*cluster.Cluster{makeCluster(endpoints, clusterName)},
		},
	}

	var jsonBuf bytes.Buffer
	if err := new(jsonpb.Marshaler).Marshal(&jsonBuf, cfg); err != nil {
		panic(err)
	}

	return jsonBuf.String()
}

func makeCluster(endpoints []kubelbiov1alpha1.LoadBalancerEndpoints, clusterName string) *cluster.Cluster {
	return &cluster.Cluster{
		Name:                 clusterName,
		ConnectTimeout:       ptypes.DurationProto(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_STATIC},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment:       makeEndpoint(endpoints, clusterName),
		DnsLookupFamily:      cluster.Cluster_V4_ONLY,
	}
}

func makeEndpoint(endpoints []kubelbiov1alpha1.LoadBalancerEndpoints, clusterName string) *envoyEndpoint.ClusterLoadAssignment {

	return &envoyEndpoint.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []*envoyEndpoint.LocalityLbEndpoints{{
			LbEndpoints: []*envoyEndpoint.LbEndpoint{{
				HostIdentifier: &envoyEndpoint.LbEndpoint_Endpoint{
					Endpoint: &envoyEndpoint.Endpoint{
						Address: &core.Address{
							Address: &core.Address_SocketAddress{
								SocketAddress: &core.SocketAddress{
									Protocol: core.SocketAddress_TCP,
									Address:  endpoints[0].Addresses[0].IP,
									PortSpecifier: &core.SocketAddress_PortValue{
										PortValue: uint32(endpoints[0].Ports[0].Port),
									},
								},
							},
						},
					},
				},
			}},
		}},
	}
}

func makeTCPListener(ClusterName string) *listener.Listener {

	tcpProxyAccessLog := &fileAccessLog.FileAccessLog{
		Path: "/dev/stdout",
	}
	tcpProxyAccessLogAny, err := ptypes.MarshalAny(tcpProxyAccessLog)
	if err != nil {
		panic(err)
	}

	var (
		tcpProxy = &tp.TcpProxy{
			StatPrefix: "ingress_tcp_1",
			ClusterSpecifier: &tp.TcpProxy_Cluster{
				Cluster: ClusterName,
			},
			AccessLog: []*accessLog.AccessLog{
				{
					Name: "envoy.file_access_log",
					ConfigType: &accessLog.AccessLog_TypedConfig{
						TypedConfig: tcpProxyAccessLogAny,
					},
				},
			},
		}
	)
	pbst, err := ptypes.MarshalAny(tcpProxy)
	if err != nil {
		panic(err)
	}

	return &listener.Listener{
		Name: ListenerName,
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: ListenerPort,
					},
				},
			},
		},
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{{
				Name: wellknown.TCPProxy,
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: pbst,
				},
			}},
		}},
	}
}
