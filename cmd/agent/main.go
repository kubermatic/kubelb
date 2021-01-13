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
	"errors"
	"flag"
	"fmt"
	"github.com/spf13/pflag"
	"k8c.io/kubelb/pkg/controllers/agent"
	informers "k8c.io/kubelb/pkg/generated/informers/externalversions"
	"k8c.io/kubelb/pkg/kubelb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"os"
	"os/signal"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"syscall"
	"time"
	// +kubebuilder:scaffold:imports
)

var (
	scheme            = runtime.NewScheme()
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

	// Add klog flags
	klogFlags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(klogFlags)

	flagset := pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	flagset.AddGoFlagSet(klogFlags)

	flagset.StringVar(&metricsAddr, "metrics-addr", ":0", "The address the metric endpoint binds to.")
	flagset.StringVar(&endpointAddressTypeString, "node-address-type", "ExternalIP", "The default address type used as an endpoint address.")
	flagset.StringVar(&clusterName, "cluster-name", "default", "Cluster name where the agent is running. Resources inside the KubeLb cluster will get deployed to the namespace named by cluster name, must be unique.")
	flagset.StringVar(&kubeLbKubeconf, "kubelb-kubeconfig", defaultKubeLbConf, "The path to the kubelb cluster kubeconfig.")
	flagset.BoolVar(&enableCloudController, "enable-cloud-provider", true, "Enables cloud controller like behavior. This will set the status of TCP LoadBalancer")
	flagset.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller agent. Enabling this will ensure there is only one active controller agent.")

	err := flagset.Parse(os.Args[1:])
	if err != nil {
		os.Exit(1)
	}

	log := klogr.New()
	ctrl.SetLogger(log)

	setupLog := log.WithName("init")

	setupLog.V(1).Info("cluster", "name", clusterName)

	//is there something better i could do=?
	var endpointAddressType corev1.NodeAddressType
	if endpointAddressTypeString == string(corev1.NodeInternalIP) {
		endpointAddressType = corev1.NodeInternalIP
	} else if endpointAddressTypeString == string(corev1.NodeExternalIP) {
		endpointAddressType = corev1.NodeExternalIP
	} else {
		setupLog.Error(errors.New("invalid node address type"), fmt.Sprintf("Expected: %s or %s, got: %s", corev1.NodeInternalIP, corev1.NodeExternalIP, endpointAddressTypeString))
		os.Exit(1)
	}

	setupLog.V(1).Info("using endpoint address", "type", endpointAddressType)

	var sharedEndpoints = kubelb.Endpoints{
		ClusterEndpoints:    []string{},
		EndpointAddressType: endpointAddressType,
	}

	// setup signal handler
	ctx := ctrl.SetupSignalHandler()

	kubeLbClient, err := kubelb.NewClient(clusterName, kubeLbKubeconf)
	if err != nil {
		setupLog.Error(err, "unable to create client for kubelb cluster")
		os.Exit(1)
	}

	//verify update status in user cluster
	tcpLoadBalancerInformerFactory := informers.NewSharedInformerFactory(kubeLbClient.Clientset, time.Second*120)

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

	if err = (&agent.KubeLbNodeReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("kubelb.node.reconciler"),
		Scheme:    mgr.GetScheme(),
		KlbClient: kubeLbClient.TcpLbClient,
		Endpoints: &sharedEndpoints,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "reconciler", "kubelb.node.reconciler")
		os.Exit(1)
	}

	if err = (&agent.KubeLbServiceReconciler{
		Client:                  mgr.GetClient(),
		Log:                     ctrl.Log.WithName("kubelb.service.reconciler"),
		Scheme:                  mgr.GetScheme(),
		TcpLBClient:             kubeLbClient.TcpLbClient,
		CloudController:         enableCloudController,
		Endpoints:               &sharedEndpoints,
		ClusterName:             clusterName,
		TcpLoadBalancerInformer: tcpLoadBalancerInformerFactory.Kubelb().V1alpha1().TCPLoadBalancers(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "reconciler", "kubelb.service.reconciler")
		os.Exit(1)
	}

	if err = (&agent.KubeLbIngressReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("kubelb.ingress.reconciler"),
		Scheme:       mgr.GetScheme(),
		HttpLBClient: kubeLbClient.HttpLbClient,
		ClusterName:  clusterName,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "reconciler", "kubelb.ingress.reconciler")
		os.Exit(1)
	}

	//this is a copy paste of SetupSignalHandler which only returns a context
	tcplbInformerChannel := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, []os.Signal{os.Interrupt, syscall.SIGTERM}...)
	go func() {
		<-c
		close(tcplbInformerChannel)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	tcpLoadBalancerInformerFactory.Start(tcplbInformerChannel)

	// +kubebuilder:scaffold:builder
	setupLog.V(1).Info("starting kubelb agent")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running agent")
		os.Exit(1)
	}
}
