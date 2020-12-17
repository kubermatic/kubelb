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
	"bytes"
	envoyBootstrap "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	envoyCluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoyCore "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoyEndpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"
	"time"
)

const xdsClusterName = "xds_cluster"

const controlPlaneAddress = "envoycp.kubelb.svc"

func (s *server) GenerateBootstrap() string {

	var adminCfg *envoyBootstrap.Admin = nil

	if s.enableAdmin {
		adminCfg = &envoyBootstrap.Admin{
			AccessLogPath: "/dev/null",
			Address: &envoyCore.Address{
				Address: &envoyCore.Address_SocketAddress{SocketAddress: &envoyCore.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &envoyCore.SocketAddress_PortValue{
						PortValue: 9001,
					},
				}},
			},
		}
	}

	cfg := &envoyBootstrap.Bootstrap{
		DynamicResources: &envoyBootstrap.Bootstrap_DynamicResources{
			LdsConfig: &envoyCore.ConfigSource{
				ResourceApiVersion: envoyCore.ApiVersion_V3,
				ConfigSourceSpecifier: &envoyCore.ConfigSource_ApiConfigSource{
					ApiConfigSource: &envoyCore.ApiConfigSource{
						ApiType:                   envoyCore.ApiConfigSource_GRPC,
						TransportApiVersion:       envoyCore.ApiVersion_V3,
						SetNodeOnFirstMessageOnly: true,
						GrpcServices: []*envoyCore.GrpcService{
							{
								TargetSpecifier: &envoyCore.GrpcService_EnvoyGrpc_{
									EnvoyGrpc: &envoyCore.GrpcService_EnvoyGrpc{
										ClusterName: xdsClusterName,
									}},
							},
						},
					},
				},
			},
			CdsConfig: &envoyCore.ConfigSource{
				ResourceApiVersion: envoyCore.ApiVersion_V3,
				ConfigSourceSpecifier: &envoyCore.ConfigSource_ApiConfigSource{
					ApiConfigSource: &envoyCore.ApiConfigSource{
						ApiType:                   envoyCore.ApiConfigSource_GRPC,
						TransportApiVersion:       envoyCore.ApiVersion_V3,
						SetNodeOnFirstMessageOnly: true,
						GrpcServices: []*envoyCore.GrpcService{
							{
								TargetSpecifier: &envoyCore.GrpcService_EnvoyGrpc_{
									EnvoyGrpc: &envoyCore.GrpcService_EnvoyGrpc{
										ClusterName: xdsClusterName,
									}},
							},
						},
					},
				},
			},
		},
		StaticResources: &envoyBootstrap.Bootstrap_StaticResources{
			Clusters: []*envoyCluster.Cluster{{
				Name:                 xdsClusterName,
				ConnectTimeout:       ptypes.DurationProto(5 * time.Second),
				ClusterDiscoveryType: &envoyCluster.Cluster_Type{Type: envoyCluster.Cluster_STRICT_DNS},
				LbPolicy:             envoyCluster.Cluster_ROUND_ROBIN,
				LoadAssignment: &envoyEndpoint.ClusterLoadAssignment{
					ClusterName: xdsClusterName,
					Endpoints: []*envoyEndpoint.LocalityLbEndpoints{
						{
							LbEndpoints: []*envoyEndpoint.LbEndpoint{
								{
									HostIdentifier: &envoyEndpoint.LbEndpoint_Endpoint{
										Endpoint: &envoyEndpoint.Endpoint{
											Address: &envoyCore.Address{
												Address: &envoyCore.Address_SocketAddress{
													SocketAddress: &envoyCore.SocketAddress{
														Address: controlPlaneAddress,
														PortSpecifier: &envoyCore.SocketAddress_PortValue{
															PortValue: s.listenPort,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				LoadBalancingPolicy: &envoyCluster.LoadBalancingPolicy{Policies: []*envoyCluster.LoadBalancingPolicy_Policy{{
					Name: "ROUND_ROBIN",
				}}},
				//Todo: investigate - envoy have problems to connect without
				Http2ProtocolOptions: &envoyCore.Http2ProtocolOptions{},
				CircuitBreakers: &envoyCluster.CircuitBreakers{
					Thresholds: []*envoyCluster.CircuitBreakers_Thresholds{
						{
							Priority:           envoyCore.RoutingPriority_HIGH,
							MaxConnections:     &wrappers.UInt32Value{Value: 100000},
							MaxPendingRequests: &wrappers.UInt32Value{Value: 100000},
							MaxRequests:        &wrappers.UInt32Value{Value: 60000000},
							MaxRetries:         &wrappers.UInt32Value{Value: 50},
						},
						{
							Priority:           envoyCore.RoutingPriority_DEFAULT,
							MaxConnections:     &wrappers.UInt32Value{Value: 100000},
							MaxPendingRequests: &wrappers.UInt32Value{Value: 100000},
							MaxRequests:        &wrappers.UInt32Value{Value: 60000000},
							MaxRetries:         &wrappers.UInt32Value{Value: 50},
						},
					},
				},
			}},
		},
		Admin: adminCfg,
	}

	var jsonBuf bytes.Buffer
	if err := new(jsonpb.Marshaler).Marshal(&jsonBuf, cfg); err != nil {
		panic(err)
	}

	return jsonBuf.String()
}
