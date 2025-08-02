/*
Copyright 2025 The KubeLB Authors.

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

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/tunnel"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	var (
		httpAddr       = flag.String("http-addr", ":8080", "HTTP server address for Envoy and tunnel connections")
		requestTimeout = flag.Duration("request-timeout", 30*time.Second, "Timeout for forwarded requests")
	)

	klog.InitFlags(nil)
	flag.Parse()

	ctx := context.Background()
	log := klog.FromContext(ctx)

	// Initialize scheme
	if err := kubelbv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		log.Error(err, "Failed to add kubelb scheme")
		os.Exit(1)
	}

	// Create Kubernetes client
	kubeConfig := ctrl.GetConfigOrDie()
	kubeClient, err := client.New(kubeConfig, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		log.Error(err, "Failed to create Kubernetes client")
		os.Exit(1)
	}

	// Create configuration
	config := &tunnel.ConnectionManagerConfig{
		HTTPAddr:       *httpAddr,
		RequestTimeout: *requestTimeout,
		KubeClient:     kubeClient,
	}

	ctx, cancel := context.WithCancel(ctx)
	manager, err := tunnel.NewConnectionManager(config)
	if err != nil {
		log.Error(err, "Failed to create connection manager")
		cancel()
		os.Exit(1)
	}

	// Start connection manager
	if err := manager.Start(ctx); err != nil {
		log.Error(err, "Failed to start connection manager")
		cancel()
		os.Exit(1)
	}

	defer cancel()

	log.Info("Connection manager started successfully",
		"httpAddr", *httpAddr,
		"requestTimeout", *requestTimeout,
		"protocol", "HTTP/2 with SSE",
		"authentication", "token-based")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Info("Received shutdown signal")

	// Cancel context to trigger graceful shutdown
	cancel()

	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)
	manager.Stop()
	log.Info("Connection manager stopped")
}
