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

	"go.uber.org/zap/zapcore"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/pkg/controllers/ccm"
	"k8c.io/kubelb/pkg/kubelb"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	scheme            = runtime.NewScheme()
	setupLog          = ctrl.Log.WithName("setup")
	defaultKubeLbConf = filepath.Join(
		os.Getenv("HOME"), ".kube", "kubelb",
	)
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kubelbk8ciov1alpha1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var probeAddr string
	var enableCloudController bool
	var enableLeaderElection bool
	var leaderElectionNamespace string
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
	flag.StringVar(&leaderElectionNamespace, "leader-election-namespace", "", "Optionally configure leader election namespace.")

	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.V(1).Info("cluster", "name", clusterName)

	var endpointAddressType corev1.NodeAddressType
	switch endpointAddressTypeString {
	case string(corev1.NodeInternalIP):
		endpointAddressType = corev1.NodeInternalIP
	case string(corev1.NodeExternalIP):
		endpointAddressType = corev1.NodeExternalIP
	default:
		setupLog.Error(errors.New("invalid node address type"), fmt.Sprintf("Expected: %s or %s, got: %s", corev1.NodeInternalIP, corev1.NodeExternalIP, endpointAddressTypeString))
		os.Exit(1)
	}

	setupLog.V(1).Info("using endpoint address", "type", endpointAddressType)

	sharedEndpoints := kubelb.Endpoints{
		ClusterEndpoints:    []string{},
		EndpointAddressType: endpointAddressType,
	}

	// setup signal handler
	ctx := ctrl.SetupSignalHandler()

	kubeLBClientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{
			ExplicitPath: kubeLbKubeconf,
		},
		&clientcmd.ConfigOverrides{},
	)

	kubeLBRestConfig, err := kubeLBClientConfig.ClientConfig()
	if err != nil {
		setupLog.Error(err, "unable to create rest config for kubelb cluster")
		os.Exit(1)
	}

	kubeLBMgr, err := ctrl.NewManager(kubeLBRestConfig, ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&kubelbk8ciov1alpha1.TCPLoadBalancer{}: {
					Namespaces: map[string]cache.Config{
						clusterName: {},
					},
				},
			},
		},
	})
	if err != nil {
		setupLog.Error(err, "failed to create manager for kubelb cluster")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "19f32e7b.ccm.kubelb.k8c.io",
		LeaderElectionNamespace: leaderElectionNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start ccm manager")
		os.Exit(1)
	}

	if err := mgr.Add(kubeLBMgr); err != nil {
		setupLog.Error(err, "unable to start kubelb manager")
		os.Exit(1)
	}

	if err = (&ccm.KubeLBNodeReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("kubelb.node.reconciler"),
		Scheme:       mgr.GetScheme(),
		KubeLBClient: kubeLBMgr.GetClient(),
		Endpoints:    &sharedEndpoints,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "reconciler", "kubelb.node.reconciler")
		os.Exit(1)
	}

	if err = (&ccm.KubeLBServiceReconciler{
		Client:          mgr.GetClient(),
		KubeLBManager:   kubeLBMgr,
		Log:             ctrl.Log.WithName("kubelb.service.reconciler"),
		Scheme:          mgr.GetScheme(),
		CloudController: enableCloudController,
		Endpoints:       &sharedEndpoints,
		ClusterName:     clusterName,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "reconciler", "kubelb.service.reconciler")
		os.Exit(1)
	}

	// this is a copy and paste of SetupSignalHandler which only returns a context
	signals := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, []os.Signal{os.Interrupt, syscall.SIGTERM}...)
	go func() {
		<-c
		close(signals)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

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
