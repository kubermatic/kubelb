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
	"time"

	envoyBootstrap "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	envoyCluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoyCore "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoyEndpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoyListener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/planetscale/vtprotobuf/types/known/wrapperspb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

const xdsClusterName = "xds_cluster"

const controlPlaneAddress = "envoycp.kubelb.svc"

const EnvoyAdminPort = 9001

const EnvoyStatsPort = 19001
const EnvoyProbePort = 19003

// By default, the admin interface is only accessible from localhost, to prevent
// potential security issues while keeping it available for the stats and probe listeners.
var EnvoyAdminListenerAddress = "127.0.0.1"

func (s *Server) GenerateBootstrap() string {
	// If debug is enabled, allow external access to the admin interface
	if s.enableDebug {
		EnvoyAdminListenerAddress = "0.0.0.0"
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
			Listeners: []*envoyListener.Listener{
				getProbesListener(),
			},
			Clusters: []*envoyCluster.Cluster{
				{
					Name:                 xdsClusterName,
					ConnectTimeout:       durationpb.New(5 * time.Second),
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
					Http2ProtocolOptions: &envoyCore.Http2ProtocolOptions{},
					CircuitBreakers: &envoyCluster.CircuitBreakers{
						Thresholds: []*envoyCluster.CircuitBreakers_Thresholds{
							{
								Priority:           envoyCore.RoutingPriority_HIGH,
								MaxConnections:     &wrapperspb.UInt32Value{Value: 100000},
								MaxPendingRequests: &wrapperspb.UInt32Value{Value: 100000},
								MaxRequests:        &wrapperspb.UInt32Value{Value: 60000000},
								MaxRetries:         &wrapperspb.UInt32Value{Value: 50},
							},
							{
								Priority:           envoyCore.RoutingPriority_DEFAULT,
								MaxConnections:     &wrapperspb.UInt32Value{Value: 100000},
								MaxPendingRequests: &wrapperspb.UInt32Value{Value: 100000},
								MaxRequests:        &wrapperspb.UInt32Value{Value: 60000000},
								MaxRetries:         &wrapperspb.UInt32Value{Value: 50},
							},
						},
					},
				},
				{
					Name:                 "admin_cluster",
					ConnectTimeout:       durationpb.New(1 * time.Second),
					ClusterDiscoveryType: &envoyCluster.Cluster_Type{Type: envoyCluster.Cluster_STATIC},
					LbPolicy:             envoyCluster.Cluster_ROUND_ROBIN,
					LoadAssignment: &envoyEndpoint.ClusterLoadAssignment{
						ClusterName: "admin_cluster",
						Endpoints: []*envoyEndpoint.LocalityLbEndpoints{{
							LbEndpoints: []*envoyEndpoint.LbEndpoint{{
								HostIdentifier: &envoyEndpoint.LbEndpoint_Endpoint{
									Endpoint: &envoyEndpoint.Endpoint{
										Address: &envoyCore.Address{
											Address: &envoyCore.Address_SocketAddress{
												SocketAddress: &envoyCore.SocketAddress{
													Address: EnvoyAdminListenerAddress,
													PortSpecifier: &envoyCore.SocketAddress_PortValue{
														PortValue: EnvoyAdminPort,
													},
												},
											},
										},
									},
								},
							}},
						}},
					},
				},
			},
		},
		Admin: &envoyBootstrap.Admin{
			Address: &envoyCore.Address{
				Address: &envoyCore.Address_SocketAddress{SocketAddress: &envoyCore.SocketAddress{
					Address: EnvoyAdminListenerAddress,
					PortSpecifier: &envoyCore.SocketAddress_PortValue{
						PortValue: EnvoyAdminPort,
					},
				}},
			},
		},
	}

	// Stats listener and cluster rely on admin being enabled
	if s.enableEnvoyMonitoring {
		cfg.StaticResources.Listeners = append(cfg.StaticResources.Listeners, getStatsListener())
	}

	jsonBytes, err := protojson.Marshal(cfg)
	if err != nil {
		panic(err)
	}

	return string(jsonBytes)
}

func getProbesListener() *envoyListener.Listener {
	typedRouterFilterConfig, err := anypb.New(&envoyFiltersRouterV3.Router{})
	if err != nil {
		panic(err)
	}

	hcm := &envoyFiltersHcmV3.HttpConnectionManager{
		StatPrefix: "stats_probe",
		HttpFilters: []*envoyFiltersHcmV3.HttpFilter{{
			Name:       wellknown.Router,
			ConfigType: &envoyFiltersHcmV3.HttpFilter_TypedConfig{TypedConfig: typedRouterFilterConfig},
		}},
		RouteSpecifier: &envoyFiltersHcmV3.HttpConnectionManager_RouteConfig{
			RouteConfig: &envoyRoute.RouteConfiguration{
				Name: "local_route",
				VirtualHosts: []*envoyRoute.VirtualHost{{
					Name:    "probe_service",
					Domains: []string{"*"},
					Routes: []*envoyRoute.Route{{
						Match: &envoyRoute.RouteMatch{
							PathSpecifier: &envoyRoute.RouteMatch_Path{
								Path: "/ready",
							},
							Headers: []*envoyRoute.HeaderMatcher{{
								Name: ":method",
								HeaderMatchSpecifier: &envoyRoute.HeaderMatcher_ExactMatch{
									ExactMatch: "GET",
								},
							}},
						},
						Action: &envoyRoute.Route_Route{
							Route: &envoyRoute.RouteAction{
								ClusterSpecifier: &envoyRoute.RouteAction_Cluster{
									Cluster: "admin_cluster",
								},
							},
						},
					}},
				}},
			},
		},
	}

	typedConfig, err := anypb.New(hcm)
	if err != nil {
		panic(err)
	}

	return &envoyListener.Listener{
		Name: "probe_listener",
		Address: &envoyCore.Address{
			Address: &envoyCore.Address_SocketAddress{
				SocketAddress: &envoyCore.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &envoyCore.SocketAddress_PortValue{
						PortValue: EnvoyProbePort,
					},
				},
			},
		},
		FilterChains: []*envoyListener.FilterChain{{
			Filters: []*envoyListener.Filter{{
				Name:       wellknown.HTTPConnectionManager,
				ConfigType: &envoyListener.Filter_TypedConfig{TypedConfig: typedConfig},
			}},
		}},
	}
}

func getStatsListener() *envoyListener.Listener {
	typedRouterFilterConfig, err := anypb.New(&envoyFiltersRouterV3.Router{})
	if err != nil {
		panic(err)
	}

	hcm := &envoyFiltersHcmV3.HttpConnectionManager{
		StatPrefix: "stats_http",
		HttpFilters: []*envoyFiltersHcmV3.HttpFilter{{
			Name:       wellknown.Router,
			ConfigType: &envoyFiltersHcmV3.HttpFilter_TypedConfig{TypedConfig: typedRouterFilterConfig},
		}},
		RouteSpecifier: &envoyFiltersHcmV3.HttpConnectionManager_RouteConfig{
			RouteConfig: &envoyRoute.RouteConfiguration{
				Name: "local_route",
				VirtualHosts: []*envoyRoute.VirtualHost{{
					Name:    "stats_service",
					Domains: []string{"*"},
					Routes: []*envoyRoute.Route{{
						Match: &envoyRoute.RouteMatch{
							PathSpecifier: &envoyRoute.RouteMatch_Path{
								Path: "/stats/prometheus",
							},
							Headers: []*envoyRoute.HeaderMatcher{{
								Name: ":method",
								HeaderMatchSpecifier: &envoyRoute.HeaderMatcher_ExactMatch{
									ExactMatch: "GET",
								},
							}},
						},
						Action: &envoyRoute.Route_Route{
							Route: &envoyRoute.RouteAction{
								ClusterSpecifier: &envoyRoute.RouteAction_Cluster{
									Cluster: "admin_cluster",
								},
							},
						},
					}},
				}},
			},
		},
	}

	typedConfig, err := anypb.New(hcm)
	if err != nil {
		panic(err)
	}

	return &envoyListener.Listener{
		Name: "stats_listener",
		Address: &envoyCore.Address{
			Address: &envoyCore.Address_SocketAddress{
				SocketAddress: &envoyCore.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &envoyCore.SocketAddress_PortValue{
						PortValue: EnvoyStatsPort,
					},
				},
			},
		},
		FilterChains: []*envoyListener.FilterChain{{
			Filters: []*envoyListener.Filter{{
				Name:       wellknown.HTTPConnectionManager,
				ConfigType: &envoyListener.Filter_TypedConfig{TypedConfig: typedConfig},
			}},
		}},
	}
}
