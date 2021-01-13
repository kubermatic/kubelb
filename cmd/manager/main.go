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
	"github.com/spf13/pflag"
	envoycp "k8c.io/kubelb/pkg/envoy"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/pkg/controllers/kubelb"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = kubelbk8ciov1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var envoyListenAddress string
	var enableLeaderElection bool
	var enableDebugMode bool

	// Add klog flags
	klogFlags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(klogFlags)

	flagset := pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	flagset.AddGoFlagSet(klogFlags)

	flagset.StringVar(&envoyListenAddress, "listen-address", ":8001", "Address to serve envoy control-plane on")
	flagset.StringVar(&metricsAddr, "metrics-addr", ":0", "The address the metric endpoint binds to.")
	flagset.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flagset.BoolVar(&enableDebugMode, "debug", false, "Enables debug mode")

	err := flagset.Parse(os.Args[1:])
	if err != nil {
		os.Exit(1)
	}

	log := klogr.New()
	ctrl.SetLogger(log)

	setupLog := log.WithName("init")

	// setup signal handler
	ctx := ctrl.SetupSignalHandler()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "manager.kubelb.k8c.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	envoyServer, err := envoycp.NewServer(envoyListenAddress, enableDebugMode)

	if err != nil {
		setupLog.Error(err, "unable to create envoy server")
		os.Exit(1)
	}

	if err := mgr.Add(envoyServer); err != nil {
		setupLog.Error(err, "failed to register envoy config server with controller-runtime manager")
		os.Exit(1)
	}

	if err = (&kubelb.TCPLoadBalancerReconciler{
		Client:         mgr.GetClient(),
		Log:            ctrl.Log.WithName("kubelb.tcploadbalancer.reconciler"),
		Cache:          mgr.GetCache(),
		Scheme:         mgr.GetScheme(),
		EnvoyCache:     envoyServer.Cache,
		EnvoyBootstrap: envoyServer.GenerateBootstrap(),
	}).SetupWithManager(mgr, ctx); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TCPLoadBalancer")
		os.Exit(1)
	}

	if err = (&kubelb.HTTPLoadBalancerReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("kubelb.httploadbalancer.reconciler"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HTTPLoadBalancer")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
