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
	informers "k8c.io/kubelb/manager/pkg/generated/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"
	// +kubebuilder:scaffold:imports
)

var (
	scheme            = runtime.NewScheme()
	setupLog          = ctrl.Log.WithName("setup")
	defaultKubeLbConf = filepath.Join(
		os.Getenv("HOME"), ".kube", "kubelb",
	)
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	//_ = kubelbk8ciov1alpha1.SchemeBuilder.AddToScheme(scheme)

	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableCloudController bool
	var enableLeaderElection bool
	var endpointAddressTypeString string
	var clusterName string
	var kubeLbKubeconf string

	flag.StringVar(&metricsAddr, "metrics-addr", ":0", "The address the metric endpoint binds to.")
	flag.StringVar(&endpointAddressTypeString, "node-address-type", ":ExternalIP", "The default address type used as an endpoint address.")
	flag.StringVar(&clusterName, "cluster-name", "default", "Cluster name where the agent is running. Resources inside the KubeLb cluster will get deployed to the namespace named by cluster name, must be unique.")
	flag.StringVar(&kubeLbKubeconf, "kubelb-kubeconfig", defaultKubeLbConf, "The path to the kubelb cluster kubeconfig.")
	flag.BoolVar(&enableCloudController, "enable-cloud-provider", true, "Enables cloud controller like behavior. This will set the status of TCP LoadBalancer")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller agent. Enabling this will ensure there is only one active controller agent.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	//Todo: is there something better i could do=?
	var endpointAddressType corev1.NodeAddressType
	if endpointAddressTypeString == string(corev1.NodeInternalIP) {
		endpointAddressType = corev1.NodeInternalIP
	} else if endpointAddressTypeString == string(corev1.NodeExternalIP) {
		endpointAddressType = corev1.NodeExternalIP
	}

	var sharedEndpoints = kubelb.Endpoints{
		ClusterEndpoints:    []string{},
		EndpointAddressType: endpointAddressType,
	}

	kubeLbClient, err := kubelb.NewClient(clusterName, kubeLbKubeconf)

	if err != nil {
		setupLog.Error(err, "unable to create client for kubelb cluster")
		os.Exit(1)
	}

	tcpLoadBalancerInformerFactory := informers.NewSharedInformerFactory(kubeLbClient.Clientset, time.Second*10)

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

	if err = (&controllers.KubeLbNodeReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("kubelb-node-agent.controller"),
		Scheme:    mgr.GetScheme(),
		KlbClient: kubeLbClient.TcpLbClient,
		Endpoints: &sharedEndpoints,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "kubelb-node-agent.controller")
		os.Exit(1)
	}

	if err = (&controllers.KubeLbServiceReconciler{
		Client:                  mgr.GetClient(),
		Log:                     ctrl.Log.WithName("kubelb-service-agent.controller"),
		Scheme:                  mgr.GetScheme(),
		TcpLBClient:             kubeLbClient.TcpLbClient,
		CloudController:         enableCloudController,
		Endpoints:               &sharedEndpoints,
		ClusterName:             clusterName,
		TcpLoadBalancerInformer: tcpLoadBalancerInformerFactory.Kubelb().V1alpha1().TCPLoadBalancers(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "kubelb-service-agent.controller")
		os.Exit(1)
	}

	if err = (&controllers.KubeLbIngressReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("kubelb-ingress-agent.controller"),
		Scheme:       mgr.GetScheme(),
		HttpLBClient: kubeLbClient.HttpLbClient,
		ClusterName:  clusterName,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "kubelb-ingress-agent.controller")
		os.Exit(1)
	}

	sigHandler := ctrl.SetupSignalHandler()
	tcpLoadBalancerInformerFactory.Start(sigHandler)

	// +kubebuilder:scaffold:builder
	setupLog.Info("starting kubelb agent")
	if err := mgr.Start(sigHandler); err != nil {
		setupLog.Error(err, "problem running agent")
		os.Exit(1)
	}
}
