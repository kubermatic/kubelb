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
	"net"
	"strconv"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	runtimeservice "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	envoycpmetrics "k8c.io/kubelb/internal/metricsutil/envoycp"
)

const (
	grpcMaxConcurrentStreams = 1000000
)

func registerServer(grpcServer *grpc.Server, server serverv3.Server) {
	// register services
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(grpcServer, server)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, server)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, server)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, server)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, server)
	secretservice.RegisterSecretDiscoveryServiceServer(grpcServer, server)
	runtimeservice.RegisterRuntimeDiscoveryServiceServer(grpcServer, server)
}

type Server struct {
	config        *v1alpha1.Config
	Cache         cachev3.SnapshotCache
	listenAddress string
	listenPort    uint32
	enableAdmin   bool
}

func NewServer(config *v1alpha1.Config, listenAddress string, enableAdmin bool) (*Server, error) {
	portString := strings.Split(listenAddress, ":")[1]
	port, err := strconv.ParseUint(portString, 10, 32)
	if err != nil {
		return nil, err
	}

	return &Server{
		config:        config,
		listenAddress: listenAddress,
		listenPort:    uint32(port),
		enableAdmin:   enableAdmin,
		Cache:         cachev3.NewSnapshotCache(false, cachev3.IDHash{}, Logger{enableAdmin}),
	}, nil
}

// UpdateConfig updates the server's config reference.
func (s *Server) UpdateConfig(config *v1alpha1.Config) {
	s.config = config
}

// Start the Envoy control plane server.
func (s *Server) Start(ctx context.Context) error {
	// Create a Cache
	srv3 := serverv3.NewServer(ctx, s.Cache, &serverv3.CallbackFuncs{
		StreamOpenFunc: func(_ context.Context, _ int64, _ string) error {
			envoycpmetrics.GRPCConnectionsTotal.Inc()
			return nil
		},
		StreamClosedFunc: func(_ int64, _ *corev3.Node) {
			envoycpmetrics.GRPCConnectionsTotal.Dec()
		},
		StreamRequestFunc: func(_ int64, req *discoveryv3.DiscoveryRequest) error {
			envoycpmetrics.GRPCRequestsTotal.WithLabelValues(req.GetTypeUrl()).Inc()
			return nil
		},
		StreamResponseFunc: func(_ context.Context, _ int64, _ *discoveryv3.DiscoveryRequest, resp *discoveryv3.DiscoveryResponse) {
			envoycpmetrics.GRPCResponsesTotal.WithLabelValues(resp.GetTypeUrl()).Inc()
		},
	})

	// gRPC golang library sets a very small upper bound for the number gRPC/h2
	// streams over a single TCP connection. If a proxy multiplexes requests over
	// a single connection to the management server, then it might lead to
	// availability problems.
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))
	grpcServer := grpc.NewServer(grpcOptions...)

	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", s.listenAddress)
	if err != nil {
		return errors.Wrap(err, "envoy control plane server failed while start listening")
	}

	registerServer(grpcServer, srv3)

	// s.Log.Infow("starting management service", "listen-address", s.listenAddress)
	if err = grpcServer.Serve(lis); err != nil {
		return errors.Wrap(err, "envoy control plane server failed while start serving incoming connections")
	}

	return nil
}
