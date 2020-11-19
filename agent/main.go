/*
Copyright 2020 Kubermatic GmbH.

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
	"k8c.io/kubelb/agent/pkg/controllers"
	"k8c.io/kubelb/agent/pkg/kubelb"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller agent. "+
			"Enabling this will ensure there is only one active controller agent.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "k8c.io.kubelb.agent",
	})
	if err != nil {
		setupLog.Error(err, "unable to start agent")
		os.Exit(1)
	}

	//Todo: set via env
	//Todo: namespace needs to be created inside load balancing cluster
	var clusterName = "default"

	tcpLBClient, err := kubelb.NewTcpLBClient(clusterName)

	if err != nil {
		setupLog.Error(err, "unable to create kubelb client")
		os.Exit(1)
	}

	if err = (&controllers.KubeLbServiceReconciler{
		Client:      mgr.GetClient(),
		Log:         ctrl.Log.WithName("service_agent_controllers"),
		Scheme:      mgr.GetScheme(),
		TcpLBClient: tcpLBClient,
		ClusterName: clusterName,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "service_agent_controllers")
		os.Exit(1)
	}

	httpLBClient, err := kubelb.NewHttpLBClient(clusterName)

	if err != nil {
		setupLog.Error(err, "unable to create kubelb client")
		os.Exit(1)
	}

	if err = (&controllers.KubeLbIngressReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("ingress_agent_controllers"),
		Scheme:       mgr.GetScheme(),
		HttpLBClient: httpLBClient,
		ClusterName:  clusterName,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ingress_agent_controllers")
		os.Exit(1)
	}

	if err = (&controllers.KubeLbNodeReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("node_agent_controllers"),
		Scheme:    mgr.GetScheme(),
		KlbClient: tcpLBClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "node_agent_controllers")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder
	setupLog.Info("starting kubelb agent")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running agent")
		os.Exit(1)
	}
}
