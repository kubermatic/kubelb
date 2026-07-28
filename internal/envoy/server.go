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
	"time"

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
	"google.golang.org/grpc/keepalive"

	envoycpmetrics "k8c.io/kubelb/internal/metricsutil/envoycp"
)

const (
	grpcMaxConcurrentStreams = 1000000

	// xDS streams are long-lived and mostly idle, so a proxy that dies without
	// closing its TCP connection is otherwise only reaped by kernel timeouts and
	// its stream state leaks on the manager for hours.
	grpcKeepaliveTime    = 30 * time.Second
	grpcKeepaliveTimeout = 10 * time.Second

	// Bounding connection lifetime lets proxies redistribute over manager
	// replicas instead of pinning to whichever one they first reached. gRPC
	// closes an aged connection with GOAWAY plus this grace period, so the proxy
	// reconnects without ever seeing a mid-response reset.
	grpcMaxConnectionAge      = 30 * time.Minute
	grpcMaxConnectionAgeGrace = 5 * time.Minute

	// Envoy's own keepalive must not be rejected as too aggressive, which would
	// have the server tear the connection down as a policy violation.
	grpcKeepaliveMinTime = 10 * time.Second

	// Cap on draining so a stuck stream cannot block manager shutdown forever.
	grpcGracefulStopTimeout = 30 * time.Second
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
	Cache         cachev3.SnapshotCache
	listenAddress string
	listenPort    uint32
	enableAdmin   bool
}

func NewServer(listenAddress string, enableAdmin bool) (*Server, error) {
	portString := strings.Split(listenAddress, ":")[1]
	port, err := strconv.ParseUint(portString, 10, 32)
	if err != nil {
		return nil, err
	}

	return &Server{
		listenAddress: listenAddress,
		listenPort:    uint32(port),
		enableAdmin:   enableAdmin,
		Cache:         cachev3.NewSnapshotCache(false, cachev3.IDHash{}, Logger{enableAdmin}),
	}, nil
}

// Start the Envoy control plane server.
func (s *Server) Start(ctx context.Context) error {
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
			// A DiscoveryRequest carrying an ErrorDetail is a NACK: Envoy rejected
			// the config we pushed. Surface it so a rejected snapshot is not
			// silently invisible on the control plane.
			if detail := req.GetErrorDetail(); detail != nil {
				envoycpmetrics.XDSNACKsTotal.WithLabelValues(req.GetTypeUrl()).Inc()
				xdsLog.Info("Envoy NACKed xDS config",
					"type_url", req.GetTypeUrl(),
					"version", req.GetVersionInfo(),
					"nonce", req.GetResponseNonce(),
					"error", detail.GetMessage())
			}
			return nil
		},
		StreamResponseFunc: func(_ context.Context, _ int64, req *discoveryv3.DiscoveryRequest, _ *discoveryv3.DiscoveryResponse) {
			envoycpmetrics.GRPCResponsesTotal.WithLabelValues(req.GetTypeUrl()).Inc()
		},
	})

	// gRPC golang library sets a very small upper bound for the number gRPC/h2
	// streams over a single TCP connection. If a proxy multiplexes requests over
	// a single connection to the management server, then it might lead to
	// availability problems.
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions,
		grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:                  grpcKeepaliveTime,
			Timeout:               grpcKeepaliveTimeout,
			MaxConnectionAge:      grpcMaxConnectionAge,
			MaxConnectionAgeGrace: grpcMaxConnectionAgeGrace,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             grpcKeepaliveMinTime,
			PermitWithoutStream: true,
		}),
	)
	grpcServer := grpc.NewServer(grpcOptions...)

	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", s.listenAddress)
	if err != nil {
		return errors.Wrap(err, "envoy control plane server failed while start listening")
	}

	registerServer(grpcServer, srv3)

	// Drain on shutdown. Serve does not observe ctx, so without this the process
	// exits with every ADS stream still open and the whole proxy fleet
	// reconnects at once against the next manager.
	go func() {
		<-ctx.Done()
		stopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(grpcGracefulStopTimeout):
			grpcServer.Stop()
		}
	}()

	if err = grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return errors.Wrap(err, "envoy control plane server failed while start serving incoming connections")
	}

	return nil
}
