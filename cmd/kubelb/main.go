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

package main

import (
	"flag"
	"os"

	"go.uber.org/zap/zapcore"
	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/pkg/controllers/kubelb"
	"k8c.io/kubelb/pkg/envoy"
	portlookup "k8c.io/kubelb/pkg/port-lookup"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type options struct {
	metricsAddr          string
	envoyListenAddress   string
	enableLeaderElection bool
	probeAddr            string
	enableDebugMode      bool
	namespace            string

	// Envoy configuration
	envoyProxyTopology string
	envoyProxyReplicas int
}

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("init")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kubelbk8ciov1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	opt := &options{}
	flag.StringVar(&opt.envoyListenAddress, "listen-address", ":8001", "Address to serve envoy control-plane on")
	flag.StringVar(&opt.metricsAddr, "metrics-addr", ":0", "The address the metric endpoint binds to.")
	flag.StringVar(&opt.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&opt.enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller kubelb. Enabling this will ensure there is only one active controller kubelb.")
	flag.BoolVar(&opt.enableDebugMode, "debug", false, "Enables debug mode")
	flag.StringVar(&opt.envoyProxyTopology, "envoy-proxy-topology", "shared", "The deployment topology for Envoy Proxy. Valid values are: shared, dedicated, and global.")
	flag.IntVar(&opt.envoyProxyReplicas, "envoy-proxy-replicas", 1, "Number of replicas for envoy proxy.")
	flag.StringVar(&opt.namespace, "namespace", "", "The namespace where the controller will run.")

	if len(opt.namespace) == 0 {
		// Retrieve controller namespace
		ns, _ := os.LookupEnv("NAMESPACE")
		if len(ns) == 0 {
			setupLog.Error(nil, "invalid value for --envoy-proxy-topology. Valid values are: shared, dedicated, and global")
			os.Exit(1)
		}
		opt.namespace = ns
	}

	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	envoyProxyTopology := kubelb.EnvoyProxyTopology(opt.envoyProxyTopology)
	if envoyProxyTopology != kubelb.EnvoyProxyTopologyDedicated && envoyProxyTopology != kubelb.EnvoyProxyTopologyShared && envoyProxyTopology != kubelb.EnvoyProxyTopologyGlobal {
		setupLog.Error(nil, "invalid value for --envoy-proxy-topology. Valid values are: shared, dedicated, and global")
		os.Exit(1)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                        scheme,
		MetricsBindAddress:            opt.metricsAddr,
		Port:                          9443,
		HealthProbeBindAddress:        opt.probeAddr,
		LeaderElection:                opt.enableLeaderElection,
		LeaderElectionID:              "19f32e7b.kubelb.k8c.io",
		LeaderElectionReleaseOnCancel: true,
		LeaderElectionNamespace:       opt.namespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start kubelb")
		os.Exit(1)
	}

	envoyServer, err := envoy.NewServer(opt.envoyListenAddress, opt.enableDebugMode)
	if err != nil {
		setupLog.Error(err, "unable to create envoy server")
		os.Exit(1)
	}

	if err := mgr.Add(envoyServer); err != nil {
		setupLog.Error(err, "failed to register envoy config server with controller-runtime kubelb")
		os.Exit(1)
	}

	// setup signal handler
	ctx := ctrl.SetupSignalHandler()

	// For Global topology, we need to ensure that the port lookup configmap exists. If it doesn't, we create it since it's managed by this controller.
	var portAllocator *portlookup.PortAllocator
	if envoyProxyTopology == kubelb.EnvoyProxyTopologyGlobal {
		portAllocator = portlookup.NewPortAllocator(mgr.GetClient(), opt.namespace)
		if err := portAllocator.LoadState(ctx, mgr.GetAPIReader()); err != nil {
			setupLog.Error(err, ("unable to load port lookup state"))
			os.Exit(1)
		}
	}

	if err = (&kubelb.LoadBalancerReconciler{
		Client:             mgr.GetClient(),
		Cache:              mgr.GetCache(),
		Scheme:             mgr.GetScheme(),
		EnvoyCache:         envoyServer.Cache,
		EnvoyBootstrap:     envoyServer.GenerateBootstrap(),
		EnvoyProxyTopology: envoyProxyTopology,
		EnvoyProxyReplicas: opt.envoyProxyReplicas,
		Namespace:          opt.namespace,
		PortAllocator:      portAllocator,
	}).SetupWithManager(mgr, ctx); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LoadBalancer")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting kubelb")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running kubelb")
		os.Exit(1)
	}
}
