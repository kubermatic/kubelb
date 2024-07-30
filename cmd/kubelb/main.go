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

	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/config"
	"k8c.io/kubelb/internal/controllers/kubelb"
	"k8c.io/kubelb/internal/envoy"
	portlookup "k8c.io/kubelb/internal/port-lookup"

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
	kubeconfig           string
	enableDebugMode      bool
	namespace            string
}

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("init")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kubelbv1alpha1.AddToScheme(scheme))
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
	flag.StringVar(&opt.namespace, "namespace", "", "The namespace where the controller will run.")

	if flag.Lookup("kubeconfig") == nil {
		flag.StringVar(&opt.kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	}

	if len(opt.namespace) == 0 {
		// Retrieve controller namespace
		ns, _ := os.LookupEnv("NAMESPACE")
		if len(ns) == 0 {
			setupLog.Error(nil, "unable to determine controller namespace. Please set NAMESPACE environment variable or use --namespace flag.")
			os.Exit(1)
		}
		opt.namespace = ns
	}

	opt.kubeconfig = flag.Lookup("kubeconfig").Value.(flag.Getter).Get().(string)
	opts := zap.Options{
		Development: false,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

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

	// Load the Config for controller
	err = config.LoadConfig(ctx, mgr.GetAPIReader(), opt.namespace)
	if err != nil {
		setupLog.Error(err, "unable to load controller config")
		os.Exit(1)
	}
	// For Global topology, we need to ensure that the port lookup table exists. If it doesn't, we create it since it's managed by this controller.
	portAllocator := portlookup.NewPortAllocator()
	if err := portAllocator.LoadState(ctx, mgr.GetAPIReader()); err != nil {
		setupLog.Error(err, ("unable to load port lookup state"))
		os.Exit(1)
	}

	if err = (&kubelb.LoadBalancerReconciler{
		Client:             mgr.GetClient(),
		Cache:              mgr.GetCache(),
		Scheme:             mgr.GetScheme(),
		Namespace:          opt.namespace,
		EnvoyProxyTopology: kubelb.EnvoyProxyTopology(config.GetEnvoyProxyTopology()),
		PortAllocator:      portAllocator,
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
		EnvoyProxyTopology: kubelb.EnvoyProxyTopology(config.GetEnvoyProxyTopology()),
		PortAllocator:      portAllocator,
		Namespace:          opt.namespace,
		EnvoyBootstrap:     envoyServer.GenerateBootstrap(),
	}).SetupWithManager(ctx, envoyMgr); err != nil {
		setupLog.Error(err, "unable to create envoy control-plane controller", "controller", "LoadBalancer")
		os.Exit(1)
	}

	if err = (&kubelb.RouteReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Log:                ctrl.Log.WithName("controllers").WithName(kubelb.RouteControllerName),
		Recorder:           mgr.GetEventRecorderFor(kubelb.RouteControllerName),
		EnvoyProxyTopology: kubelb.EnvoyProxyTopology(config.GetEnvoyProxyTopology()),
		PortAllocator:      portAllocator,
		Namespace:          opt.namespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", kubelb.RouteControllerName)
		os.Exit(1)
	}

	if err = (&kubelb.TenantReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Config:   mgr.GetConfig(),
		Log:      ctrl.Log.WithName("controllers").WithName(kubelb.RouteControllerName),
		Recorder: mgr.GetEventRecorderFor(kubelb.RouteControllerName),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", kubelb.RouteControllerName)
		os.Exit(1)
	}

	if err = (&kubelb.TenantMigrationReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Log:      ctrl.Log.WithName("controllers").WithName(kubelb.TenantMigrationControllerName),
		Recorder: mgr.GetEventRecorderFor(kubelb.TenantMigrationControllerName),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", kubelb.TenantMigrationControllerName)
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
