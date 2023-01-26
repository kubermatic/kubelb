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
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	informers "k8c.io/kubelb/pkg/generated/informers/externalversions"

	"k8c.io/kubelb/pkg/controllers/ccm"
	"k8c.io/kubelb/pkg/kubelb"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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
	var probeAddr string
	var enableCloudController bool
	var enableLeaderElection bool
	var endpointAddressTypeString string
	var clusterName string
	var kubeLbKubeconf string

	flag.StringVar(&metricsAddr, "metrics-addr", ":0", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&endpointAddressTypeString, "node-address-type", "ExternalIP", "The default address type used as an endpoint address.")
	flag.StringVar(&clusterName, "cluster-name", "default", "Cluster name where the ccm is running. Resources inside the KubeLb cluster will get deployed to the namespace named by cluster name, must be unique.")
	flag.StringVar(&kubeLbKubeconf, "kubelb-kubeconfig", defaultKubeLbConf, "The path to the kubelb cluster kubeconfig.")
	flag.BoolVar(&enableCloudController, "enable-cloud-provider", true, "Enables cloud controller like behavior. This will set the status of TCP LoadBalancer")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller ccm. Enabling this will ensure there is only one active controller ccm.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.V(1).Info("cluster", "name", clusterName)

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
	loadBalancerInformerFactory := informers.NewSharedInformerFactoryWithOptions(kubeLbClient.Clientset, time.Second*120, informers.WithNamespace(clusterName))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "19f32e7b.ccm.kubelb.k8c.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start ccm")
		os.Exit(1)
	}

	if err = (&ccm.KubeLbNodeReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("kubelb.node.reconciler"),
		Scheme:    mgr.GetScheme(),
		KlbClient: kubeLbClient.TcpLbClient,
		Endpoints: &sharedEndpoints,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "reconciler", "kubelb.node.reconciler")
		os.Exit(1)
	}

	if err = (&ccm.KubeLbServiceReconciler{
		Client:               mgr.GetClient(),
		Log:                  ctrl.Log.WithName("kubelb.service.reconciler"),
		Scheme:               mgr.GetScheme(),
		KubeLbClient:         kubeLbClient.TcpLbClient,
		CloudController:      enableCloudController,
		Endpoints:            &sharedEndpoints,
		ClusterName:          clusterName,
		LoadBalancerInformer: loadBalancerInformerFactory.Kubelb().V1alpha1().LoadBalancers(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "reconciler", "kubelb.service.reconciler")
		os.Exit(1)
	}

	//this is a copy and paste of SetupSignalHandler which only returns a context
	tcplbInformerChannel := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, []os.Signal{os.Interrupt, syscall.SIGTERM}...)
	go func() {
		<-c
		close(tcplbInformerChannel)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	loadBalancerInformerFactory.Start(tcplbInformerChannel)

	//+kubebuilder:scaffold:builder

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
