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
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type options struct {
	metricsAddr          string
	envoyCPMetricsAddr   string
	envoyListenAddress   string
	enableLeaderElection bool
	probeAddr            string
	enableDebugMode      bool
	namespace            string

	// Envoy configuration
	envoyProxyTopology         string
	envoyProxyUseDaemonset     bool
	envoyProxySinglePodPerNode bool
	envoyProxyReplicas         int
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
	flag.StringVar(&opt.metricsAddr, "metrics-addr", ":9443", "The address the metric endpoint for the default controller manager binds to.")
	flag.StringVar(&opt.envoyCPMetricsAddr, "envoy-cp-metrics-addr", ":9444", "The address the metric endpoint for the envoy control-plane manager binds to.")
	flag.StringVar(&opt.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&opt.enableLeaderElection, "enable-leader-election", true,
		"Enable leader election for controller kubelb. Enabling this will ensure there is only one active controller kubelb.")
	flag.BoolVar(&opt.enableDebugMode, "debug", false, "Enables debug mode")
	flag.StringVar(&opt.envoyProxyTopology, "envoy-proxy-topology", "shared", "The deployment topology for Envoy Proxy. Valid values are: shared, dedicated, and global.")
	flag.BoolVar(&opt.envoyProxyUseDaemonset, "envoy-proxy-use-daemonset", false, "Envoy Proxy will run as daemonset. If set to true, --envoy-proxy-replicas will be ignored. If set to false, deployment will be used instead.")
	flag.BoolVar(&opt.envoyProxySinglePodPerNode, "envoy-proxy-single-pod-per-node", true, "Envoy proxy pods will be spread across nodes. This ensures that multiple replicas are not running on the same node.")
	flag.IntVar(&opt.envoyProxyReplicas, "envoy-proxy-replicas", 1, "Number of replicas for envoy proxy.")
	flag.StringVar(&opt.namespace, "namespace", "", "The namespace where the controller will run.")

	if len(opt.namespace) == 0 {
		// Retrieve controller namespace
		ns, _ := os.LookupEnv("NAMESPACE")
		if len(ns) == 0 {
			setupLog.Error(nil, "unable to determine controller namespace. Please set NAMESPACE environment variable or use --namespace flag.")
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

	if opt.envoyProxyUseDaemonset && opt.envoyProxySinglePodPerNode {
		setupLog.Error(nil, "invalid value for --envoy-proxy-use-daemonset and --envoy-proxy-single-pod-per-node. Both cannot be set to true")
		os.Exit(1)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                        scheme,
		Metrics:                       metricsserver.Options{BindAddress: opt.metricsAddr},
		HealthProbeBindAddress:        opt.probeAddr,
		LeaderElection:                opt.enableLeaderElection,
		LeaderElectionID:              "19f32e7b.kubelb.k8c.io",
		LeaderElectionReleaseOnCancel: true,
		LeaderElectionNamespace:       opt.namespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start kubelb controller manager")
		os.Exit(1)
	}

	envoyMgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:         scheme,
		Metrics:        metricsserver.Options{BindAddress: opt.envoyCPMetricsAddr},
		LeaderElection: false,
	})
	if err != nil {
		setupLog.Error(err, "unable to start kubelb envoy cache manager")
		os.Exit(1)
	}

	envoyServer, err := envoy.NewServer(opt.envoyListenAddress, opt.enableDebugMode)
	if err != nil {
		setupLog.Error(err, "unable to create envoy server")
		os.Exit(1)
	}

	if err := envoyMgr.Add(envoyServer); err != nil {
		setupLog.Error(err, "failed to register envoy config server with controller-runtime kubelb")
		os.Exit(1)
	}

	// setup signal handler
	ctx := ctrl.SetupSignalHandler()

	// For Global topology, we need to ensure that the port lookup table exists. If it doesn't, we create it since it's managed by this controller.
	var portAllocator *portlookup.PortAllocator
	if envoyProxyTopology == kubelb.EnvoyProxyTopologyGlobal {
		portAllocator = portlookup.NewPortAllocator()
		if err := portAllocator.LoadState(ctx, mgr.GetAPIReader()); err != nil {
			setupLog.Error(err, ("unable to load port lookup state"))
			os.Exit(1)
		}
	}

	if err = (&kubelb.LoadBalancerReconciler{
		Client:                     mgr.GetClient(),
		Cache:                      mgr.GetCache(),
		Scheme:                     mgr.GetScheme(),
		EnvoyBootstrap:             envoyServer.GenerateBootstrap(),
		EnvoyProxyTopology:         envoyProxyTopology,
		EnvoyProxyReplicas:         opt.envoyProxyReplicas,
		EnvoyProxyUseDaemonset:     opt.envoyProxyUseDaemonset,
		EnvoyProxySinglePodPerNode: opt.envoyProxySinglePodPerNode,
		Namespace:                  opt.namespace,
		PortAllocator:              portAllocator,
	}).SetupWithManager(ctx, mgr); err != nil {
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

	if err = (&kubelb.EnvoyCPReconciler{
		Client:             envoyMgr.GetClient(),
		EnvoyCache:         envoyServer.Cache,
		EnvoyProxyTopology: envoyProxyTopology,
		PortAllocator:      portAllocator,
	}).SetupWithManager(envoyMgr); err != nil {
		setupLog.Error(err, "unable to create envoy control-plane controller", "controller", "LoadBalancer")
		os.Exit(1)
	}

	go func() {
		setupLog.Info("starting kubelb envoy manager")

		if err := envoyMgr.Start(ctx); err != nil {
			setupLog.Error(err, "problem running kubelb envoy manager")
			os.Exit(1)
		}
	}()
	setupLog.Info("starting kubelb manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running kubelb")
		os.Exit(1)
	}
}
