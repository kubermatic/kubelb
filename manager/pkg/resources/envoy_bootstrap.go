package resources

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

//Todo: get values from the actual control-plane
const controlPlaneAddress = "manager-envoycp.kubelb.svc"
const controlPlanePort = 8001

func GenerateBootstrap() string {

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
															PortValue: controlPlanePort,
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
		//Todo: enable only if dev
		Admin: &envoyBootstrap.Admin{
			AccessLogPath: "/dev/null",
			Address: &envoyCore.Address{
				Address: &envoyCore.Address_SocketAddress{SocketAddress: &envoyCore.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &envoyCore.SocketAddress_PortValue{
						PortValue: 9001,
					},
				}},
			},
		},
	}

	var jsonBuf bytes.Buffer
	if err := new(jsonpb.Marshaler).Marshal(&jsonBuf, cfg); err != nil {
		panic(err)
	}

	return jsonBuf.String()
}
