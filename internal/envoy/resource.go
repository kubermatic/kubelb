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
	"math"
	"time"

	envoyAccessLog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	envoyCluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoyCore "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoyEndpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoyListener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoyRoute "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoyFileAccessLog "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	envoy_filter_http_router_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	envoyHttpManager "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoyTcpProxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoyUdpProxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/udp/udp_proxy/v3"
	envoyProxyProtocol "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/proxy_protocol/v3"
	envoyRawBuffer "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/raw_buffer/v3"
	envoyUpstreams "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	envoytypev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	portlookup "k8c.io/kubelb/internal/port-lookup"

	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	endpointAddressReferencePattern = "%s-address-%s"

	// Health check configuration constants
	defaultHealthCheckTimeoutSeconds           = 5
	defaultHealthCheckIntervalSeconds          = 5
	defaultHealthCheckUnhealthyThreshold       = 3
	defaultHealthCheckHealthyThreshold         = 2
	defaultHealthCheckNoTrafficIntervalSeconds = 5

	// Envoy extension type URL for HTTP protocol options
	envoyHTTPProtocolOptionsTypeURL = "envoy.extensions.upstreams.http.v3.HttpProtocolOptions"

	// accessLogFormat uses %DOWNSTREAM_REMOTE_ADDRESS% instead of %REQ(X-FORWARDED-FOR)%
	// since KubeLB only creates TcpProxy/UdpProxy which never set X-Forwarded-For.
	accessLogFormat = "[%START_TIME%] \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" %RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% %BYTES_SENT% %DURATION% %RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)% \"%DOWNSTREAM_REMOTE_ADDRESS%\" \"%REQ(USER-AGENT)%\" \"%REQ(X-REQUEST-ID)%\" \"%REQ(:AUTHORITY)%\" \"%UPSTREAM_HOST%\"\n"
)

func MapSnapshot(ctx context.Context, client ctrlclient.Client, loadBalancers []kubelbv1alpha1.LoadBalancer, routes []kubelbv1alpha1.Route, portAllocator *portlookup.PortAllocator) (*envoycache.Snapshot, error) {
	var listener []types.Resource
	var cluster []types.Resource

	addressesMap := make(map[string][]kubelbv1alpha1.EndpointAddress)
	for _, lb := range loadBalancers {
		// multiple endpoints represent multiple clusters
		for i, lbEndpoint := range lb.Spec.Endpoints {
			if lbEndpoint.AddressesReference != nil {
				// Check if map already contains the key
				if val, ok := addressesMap[fmt.Sprintf(endpointAddressReferencePattern, lb.Namespace, lbEndpoint.AddressesReference.Name)]; ok {
					lb.Spec.Endpoints[i].Addresses = val
					lbEndpoint.Addresses = val
				} else {
					// Load addresses from reference
					var addresses kubelbv1alpha1.Addresses
					if err := client.Get(ctx, ctrlclient.ObjectKey{Namespace: lb.Namespace, Name: lbEndpoint.AddressesReference.Name}, &addresses); err != nil {
						return nil, fmt.Errorf("failed to get addresses: %w", err)
					}
					addressesMap[fmt.Sprintf(endpointAddressReferencePattern, lb.Namespace, lbEndpoint.AddressesReference.Name)] = addresses.Spec.Addresses
					lb.Spec.Endpoints[i].Addresses = addresses.Spec.Addresses
					lbEndpoint.Addresses = addresses.Spec.Addresses
				}
			}

			for _, lbEndpointPort := range lbEndpoint.Ports {
				var lbEndpoints []*envoyEndpoint.LbEndpoint
				key := fmt.Sprintf(kubelb.EnvoyResourceIdentifierPattern, lb.Namespace, lb.Name, i, lbEndpointPort.Port, lbEndpointPort.Protocol)

				// each address -> one port
				for _, lbEndpointAddress := range lbEndpoint.Addresses {
					lbEndpoints = append(lbEndpoints, makeEndpoint(lbEndpointAddress, uint32(lbEndpointPort.Port)))
				}

				port := uint32(lbEndpointPort.Port)
				if portAllocator != nil {
					endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointPattern, lb.Namespace, lb.Name, i)
					portKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, lbEndpointPort.Port, lbEndpointPort.Protocol)
					if value, exists := portAllocator.Lookup(endpointKey, portKey); exists {
						port = uint32(value)
					}
				}

				switch lbEndpointPort.Protocol {
				case corev1.ProtocolTCP:
					listener = append(listener, makeTCPListener(key, key, port))
				case corev1.ProtocolUDP:
					listener = append(listener, makeUDPListener(key, key, port))
				}
				proxyProtocol := lbEndpointPort.Protocol == corev1.ProtocolTCP && lb.Annotations[kubelb.AnnotationProxyProtocol] == "v2"
				cluster = append(cluster, makeCluster(key, lbEndpoints, lbEndpointPort.Protocol, "", proxyProtocol))
			}
		}
	}

	for _, route := range routes {
		if route.Spec.Source.Kubernetes == nil {
			continue
		}

		// Determine route kind for listener type selection
		routeKind := getRouteKind(&route)
		isHTTP := IsHTTPRoute(routeKind)

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
		originalRouteName := getOriginalRouteName(&route)
		for _, svc := range source.Services {
			endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointRoutePattern, route.Namespace, svc.Namespace, svc.Name, originalRouteName)
			for _, port := range svc.Spec.Ports {
				portLookupKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, port.Port, port.Protocol)
				var lbEndpoints []*envoyEndpoint.LbEndpoint
				for _, address := range route.Spec.Endpoints {
					for _, routeEndpoints := range address.Addresses {
						lbEndpoints = append(lbEndpoints, makeEndpoint(routeEndpoints, uint32(port.NodePort)))
					}
				}

				listenerPort := uint32(port.Port)
				if value, exists := portAllocator.Lookup(endpointKey, portLookupKey); exists {
					listenerPort = uint32(value)
				}

				key := fmt.Sprintf(kubelb.EnvoyRoutePortIdentifierPattern, route.Namespace, svc.Namespace, svc.Name, originalRouteName, svc.UID, port.Port, port.Protocol)

				switch port.Protocol {
				case corev1.ProtocolTCP:
					// Use HTTP Connection Manager for L7 HTTP routes, TCP Proxy for L4 routes
					if isHTTP {
						listener = append(listener, makeHTTPListener(key, key, listenerPort))
					} else {
						listener = append(listener, makeTCPListener(key, key, listenerPort))
					}
				case corev1.ProtocolUDP:
					listener = append(listener, makeUDPListener(key, key, listenerPort))
				}
				cluster = append(cluster, makeCluster(key, lbEndpoints, port.Protocol, routeKind, false))
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

func makeCluster(clusterName string, lbEndpoints []*envoyEndpoint.LbEndpoint, protocol corev1.Protocol, routeKind string, proxyProtocol bool) *envoyCluster.Cluster {
	defaultHealthCheck := []*envoyCore.HealthCheck{
		{
			Timeout:            &durationpb.Duration{Seconds: defaultHealthCheckTimeoutSeconds},
			Interval:           &durationpb.Duration{Seconds: defaultHealthCheckIntervalSeconds},
			UnhealthyThreshold: &wrapperspb.UInt32Value{Value: defaultHealthCheckUnhealthyThreshold},
			HealthyThreshold:   &wrapperspb.UInt32Value{Value: defaultHealthCheckHealthyThreshold},
			// Start sending health checks after 5 seconds to a new cluster. The default is 60 seconds.
			NoTrafficInterval: &durationpb.Duration{Seconds: defaultHealthCheckNoTrafficIntervalSeconds},
			HealthChecker: &envoyCore.HealthCheck_TcpHealthCheck_{
				TcpHealthCheck: &envoyCore.HealthCheck_TcpHealthCheck{
					// This will use empty payload to perform connect-only health check.
					Send:    nil,
					Receive: []*envoyCore.HealthCheck_Payload{},
				}},
		},
	}

	if protocol == corev1.ProtocolUDP {
		// UDP health checks are not supported in Envoy, so we set defaultHealthCheck to nil
		defaultHealthCheck = nil
	}

	cluster := &envoyCluster.Cluster{
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
		DnsLookupFamily:               envoyCluster.Cluster_AUTO,
		HealthChecks:                  defaultHealthCheck,
		PerConnectionBufferLimitBytes: wrapperspb.UInt32(32768), // 32KB buffer limit
		CommonLbConfig: &envoyCluster.Cluster_CommonLbConfig{
			HealthyPanicThreshold: &envoytypev3.Percent{Value: 0},
		},
	}

	// gRPC requires HTTP/2, while HTTPRoute/Ingress use HTTP/1.1 for NodePort backends
	if routeKind == "GRPCRoute" {
		httpOpts := &envoyUpstreams.HttpProtocolOptions{
			UpstreamProtocolOptions: &envoyUpstreams.HttpProtocolOptions_ExplicitHttpConfig_{
				ExplicitHttpConfig: &envoyUpstreams.HttpProtocolOptions_ExplicitHttpConfig{
					ProtocolConfig: &envoyUpstreams.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{
						Http2ProtocolOptions: &envoyCore.Http2ProtocolOptions{},
					},
				},
			},
		}
		cluster.TypedExtensionProtocolOptions = map[string]*anypb.Any{
			envoyHTTPProtocolOptionsTypeURL: MustMarshalAny(httpOpts),
		}
	} else if IsHTTPRoute(routeKind) {
		httpOpts := &envoyUpstreams.HttpProtocolOptions{
			UpstreamProtocolOptions: &envoyUpstreams.HttpProtocolOptions_ExplicitHttpConfig_{
				ExplicitHttpConfig: &envoyUpstreams.HttpProtocolOptions_ExplicitHttpConfig{
					ProtocolConfig: &envoyUpstreams.HttpProtocolOptions_ExplicitHttpConfig_HttpProtocolOptions{
						HttpProtocolOptions: &envoyCore.Http1ProtocolOptions{},
					},
				},
			},
		}
		cluster.TypedExtensionProtocolOptions = map[string]*anypb.Any{
			envoyHTTPProtocolOptionsTypeURL: MustMarshalAny(httpOpts),
		}
	}

	if proxyProtocol {
		ppConfig := &envoyProxyProtocol.ProxyProtocolUpstreamTransport{
			Config: &envoyCore.ProxyProtocolConfig{
				Version: envoyCore.ProxyProtocolConfig_V2,
			},
			TransportSocket: &envoyCore.TransportSocket{
				Name: wellknown.TransportSocketRawBuffer,
				ConfigType: &envoyCore.TransportSocket_TypedConfig{
					TypedConfig: MustMarshalAny(&envoyRawBuffer.RawBuffer{}),
				},
			},
		}
		cluster.TransportSocket = &envoyCore.TransportSocket{
			Name: "envoy.transport_sockets.upstream_proxy_protocol",
			ConfigType: &envoyCore.TransportSocket_TypedConfig{
				TypedConfig: MustMarshalAny(ppConfig),
			},
		}
	}

	return cluster
}

func makeEndpoint(addr kubelbv1alpha1.EndpointAddress, port uint32) *envoyEndpoint.LbEndpoint {
	address := addr.IP
	if address == "" {
		address = addr.Hostname
	}
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

func makeFileAccessLog() *envoyFileAccessLog.FileAccessLog {
	return &envoyFileAccessLog.FileAccessLog{
		Path: "/dev/stdout",
		AccessLogFormat: &envoyFileAccessLog.FileAccessLog_LogFormat{
			LogFormat: &envoyCore.SubstitutionFormatString{
				Format: &envoyCore.SubstitutionFormatString_TextFormat{
					TextFormat: accessLogFormat,
				},
			},
		},
	}
}

func makeTCPListener(clusterName string, listenerName string, listenerPort uint32) *envoyListener.Listener {
	tcpProxyAccessLogAny := marshalAny(makeFileAccessLog())
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
	pbst := marshalAny(tcpProxy)

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
	udpAccessLogAny := marshalAny(makeFileAccessLog())
	udpProxy := &envoyUdpProxy.UdpProxyConfig{
		StatPrefix: listenerName,
		RouteSpecifier: &envoyUdpProxy.UdpProxyConfig_Cluster{
			Cluster: clusterName,
		},
		AccessLog: []*envoyAccessLog.AccessLog{
			{
				Name: "envoy.file_access_log",
				ConfigType: &envoyAccessLog.AccessLog_TypedConfig{
					TypedConfig: udpAccessLogAny,
				},
			},
		},
	}

	pbst := marshalAny(udpProxy)

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

// getOriginalRouteName extracts the original route name from a Route resource.
func getOriginalRouteName(route *kubelbv1alpha1.Route) string {
	if route.Spec.Source.Kubernetes != nil && route.Spec.Source.Kubernetes.Route.GetName() != "" {
		return route.Spec.Source.Kubernetes.Route.GetName()
	}
	if labels := route.GetLabels(); labels != nil {
		if name := labels[kubelb.LabelOriginName]; name != "" {
			return name
		}
	}
	return route.Name
}

// makeHTTPListener creates an HTTP Connection Manager listener for L7 HTTP routes.
func makeHTTPListener(listenerName string, clusterName string, listenerPort uint32) *envoyListener.Listener {
	routeConfig := &envoyRoute.RouteConfiguration{
		Name: listenerName + "_route",
		VirtualHosts: []*envoyRoute.VirtualHost{{
			Name:    "default",
			Domains: []string{"*"},
			Routes: []*envoyRoute.Route{{
				Match: &envoyRoute.RouteMatch{
					PathSpecifier: &envoyRoute.RouteMatch_Prefix{Prefix: "/"},
				},
				Action: &envoyRoute.Route_Route{
					Route: &envoyRoute.RouteAction{
						ClusterSpecifier: &envoyRoute.RouteAction_Cluster{Cluster: clusterName},
					},
				},
			}},
		}},
	}

	accessLogAny, err := anypb.New(makeFileAccessLog())
	if err != nil {
		panic(err)
	}

	httpConnManager := &envoyHttpManager.HttpConnectionManager{
		CodecType:  envoyHttpManager.HttpConnectionManager_AUTO,
		StatPrefix: listenerName,
		RouteSpecifier: &envoyHttpManager.HttpConnectionManager_RouteConfig{
			RouteConfig: routeConfig,
		},
		HttpFilters: []*envoyHttpManager.HttpFilter{{
			Name: wellknown.Router,
			ConfigType: &envoyHttpManager.HttpFilter_TypedConfig{
				TypedConfig: MustMarshalAny(&envoy_filter_http_router_v3.Router{}),
			},
		}},
		AccessLog: []*envoyAccessLog.AccessLog{
			{
				Name: "envoy.access_loggers.file",
				ConfigType: &envoyAccessLog.AccessLog_TypedConfig{
					TypedConfig: accessLogAny,
				},
			},
		},
		// HTTP/2 protocol settings - max values to ensure KubeLB never bottlenecks
		// EG/Ingress handles limits at edge; KubeLB should be transparent passthrough
		Http2ProtocolOptions: &envoyCore.Http2ProtocolOptions{
			MaxConcurrentStreams:        wrapperspb.UInt32(math.MaxInt32),
			InitialStreamWindowSize:     wrapperspb.UInt32(math.MaxInt32),
			InitialConnectionWindowSize: wrapperspb.UInt32(math.MaxInt32),
		},
		// Common HTTP protocol options
		CommonHttpProtocolOptions: &envoyCore.HttpProtocolOptions{
			IdleTimeout: durationpb.New(60 * time.Second),
		},
		// XFF and remote address handling
		UseRemoteAddress:  &wrapperspb.BoolValue{Value: true},
		XffNumTrustedHops: 1, // Trust 1 hop (Ingress/Gateway controller in front)
		GenerateRequestId: &wrapperspb.BoolValue{Value: true},
	}

	httpConnManagerAny, err := anypb.New(httpConnManager)
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
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &envoyListener.Filter_TypedConfig{
					TypedConfig: httpConnManagerAny,
				},
			}},
		}},
	}
}

func MustMarshalAny(pb proto.Message) *anypb.Any {
	a, err := anypb.New(pb)
	if err != nil {
		panic(err.Error())
	}

	return a
}

func IsHTTPRoute(routeKind string) bool {
	switch routeKind {
	case "HTTPRoute", "GRPCRoute", "Ingress":
		return true
	default:
		return false
	}
}

func getRouteKind(route *kubelbv1alpha1.Route) string {
	if route.Spec.Source.Kubernetes != nil && route.Spec.Source.Kubernetes.Route.GetKind() != "" {
		return route.Spec.Source.Kubernetes.Route.GetKind()
	}
	if labels := route.GetLabels(); labels != nil {
		return labels[kubelb.LabelOriginResourceKind]
	}
	return ""
}
