// Copyright 2020 Envoyproxy Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.
package envoy

import (
	"context"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/pkg/errors"
	"net"
	"strconv"
	"strings"

	"google.golang.org/grpc"

	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	runtimeservice "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
)

const (
	grpcMaxConcurrentStreams = 1000000
)

func registerServer(grpcServer *grpc.Server, server serverv3.Server) {
	// register services
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, server)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, server)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, server)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, server)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, server)
	secretservice.RegisterSecretDiscoveryServiceServer(grpcServer, server)
	runtimeservice.RegisterRuntimeDiscoveryServiceServer(grpcServer, server)
}

type server struct {
	Cache         cachev3.SnapshotCache
	listenAddress string
	listenPort    uint32
	enableAdmin   bool
}

func NewServer(listenAddress string, enableDebug bool) (*server, error) {

	portString := strings.Split(listenAddress, ":")[1]
	port, err := strconv.ParseUint(portString, 10, 32)
	if err != nil {
		return nil, err
	}

	return &server{
		listenAddress: listenAddress,
		listenPort:    uint32(port),
		Cache:         cachev3.NewSnapshotCache(false, cachev3.IDHash{}, Logger{enableDebug}),
		enableAdmin:   enableDebug,
	}, nil

}

// Start the Envoy control plane server.
func (s *server) Start(stopChan <-chan struct{}) error {
	ctx := context.Background()
	// Create a Cache
	srv3 := serverv3.NewServer(ctx, s.Cache, nil)

	// gRPC golang library sets a very small upper bound for the number gRPC/h2
	// streams over a single TCP connection. If a proxy multiplexes requests over
	// a single connection to the management server, then it might lead to
	// availability problems.
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))
	grpcServer := grpc.NewServer(grpcOptions...)

	lis, err := net.Listen("tcp", s.listenAddress)
	if err != nil {
		return errors.Wrap(err, "envoy control plane server failed while start listening")
	}

	registerServer(grpcServer, srv3)

	//s.Log.Infow("starting management service", "listen-address", s.listenAddress)
	if err = grpcServer.Serve(lis); err != nil {
		return errors.Wrap(err, "envoy control plane server failed while start serving incoming connections")
	}
	<-stopChan
	return nil
}
