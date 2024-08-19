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

	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/controllers/ccm"

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
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
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
	utilruntime.Must(kubelbv1alpha1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var probeAddr string
	var enableCloudController bool
	var useLoadbalancerClass bool
	var enableLeaderElection bool
	var leaderElectionNamespace string
	var endpointAddressTypeString string
	var clusterName string
	var kubeLbKubeconf string
	var kubeconfig string
	var useIngressClass bool
	var useGatewayClass bool
	var disableIngressController bool
	var disableGatewayController bool
	var disableHTTPRouteController bool
	var disableGRPCRouteController bool
	var disableGatewayAPI bool
	var enableSecretSynchronizer bool

	if flag.Lookup("kubeconfig") == nil {
		flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	}

	flag.StringVar(&metricsAddr, "metrics-addr", ":0", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", true,
		"Enable leader election for controller ccm. Enabling this will ensure there is only one active controller ccm.")
	flag.StringVar(&leaderElectionNamespace, "leader-election-namespace", "", "Optionally configure leader election namespace.")

	flag.StringVar(&endpointAddressTypeString, "node-address-type", string(corev1.NodeExternalIP), "The default address type used as an endpoint address for the LoadBalancer service. Valid values are ExternalIP or InternalIP, default is ExternalIP.")
	flag.StringVar(&clusterName, "cluster-name", "", "Cluster name where the ccm is running. Resources inside the KubeLb cluster will get deployed to the namespace named by cluster name, must be unique.")
	flag.StringVar(&kubeLbKubeconf, "kubelb-kubeconfig", defaultKubeLbConf, "The path to the kubelb cluster kubeconfig.")
	flag.BoolVar(&enableCloudController, "enable-cloud-provider", true, "Enables cloud controller like behavior. This will set the status of LoadBalancer")
	flag.BoolVar(&useIngressClass, "use-ingress-class", true, "Use IngressClass `kubelb` to filter Ingress objects. Ingresses should have `ingressClassName: kubelb` set in the spec.")
	flag.BoolVar(&useLoadbalancerClass, "use-loadbalancer-class", false, "Use LoadBalancerClass `kubelb` to filter services. If false, all load balancer services will be managed by KubeLB.")
	flag.BoolVar(&useGatewayClass, "use-gateway-class", true, "Use Gateway Class `kubelb` to filter Gateway objects. Gateway should have `gatewayClassName: kubelb` set in the spec.")

	flag.BoolVar(&disableIngressController, "disable-ingress-controller", false, "Disable the Ingress controller.")
	flag.BoolVar(&disableGatewayAPI, "disable-gateway-api", false, "Disable the Gateway APIs and controllers.")
	flag.BoolVar(&disableGatewayController, "disable-gateway-controller", false, "Disable the Gateway controller.")
	flag.BoolVar(&disableHTTPRouteController, "disable-httproute-controller", false, "Disable the HTTPRoute controller.")
	flag.BoolVar(&disableGRPCRouteController, "disable-grpcroute-controller", false, "Disable the GRPCRoute controller.")

	flag.BoolVar(&enableSecretSynchronizer, "enable-secret-synchronizer", false, "Enable to automatically convert Secrets labelled with `kubelb.k8c.io/managed-by: kubelb` to Sync Secrets.  This is used to sync secrets from tenants to the LB cluster in a controlled and secure way.")

	if !disableGatewayAPI {
		utilruntime.Must(gwapiv1alpha2.Install(scheme))
		utilruntime.Must(gwapiv1.Install(scheme))
	}

	opts := zap.Options{
		Development: false,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	kubeconfig = flag.Lookup("kubeconfig").Value.(flag.Getter).Get().(string)

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
				&kubelbv1alpha1.LoadBalancer{}: {
					Namespaces: map[string]cache.Config{
						clusterName: {},
					},
				},
				&kubelbv1alpha1.Route{}: {
					Namespaces: map[string]cache.Config{
						clusterName: {},
					},
				},
				&kubelbv1alpha1.Addresses{}: {
					Namespaces: map[string]cache.Config{
						clusterName: {},
					},
				},
				&kubelbv1alpha1.SyncSecret{}: {
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
		Client:              mgr.GetClient(),
		Log:                 ctrl.Log.WithName("kubelb.node.reconciler"),
		Scheme:              mgr.GetScheme(),
		KubeLBClient:        kubeLBMgr.GetClient(),
		EndpointAddressType: endpointAddressType,
		ClusterName:         clusterName,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "reconciler", "kubelb.node.reconciler")
		os.Exit(1)
	}

	if err = (&ccm.KubeLBServiceReconciler{
		Client:               mgr.GetClient(),
		KubeLBManager:        kubeLBMgr,
		Log:                  ctrl.Log.WithName("kubelb.service.reconciler"),
		Scheme:               mgr.GetScheme(),
		CloudController:      enableCloudController,
		UseLoadbalancerClass: useLoadbalancerClass,
		ClusterName:          clusterName,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "reconciler", "kubelb.service.reconciler")
		os.Exit(1)
	}

	if !disableIngressController {
		if err = (&ccm.IngressReconciler{
			Client:          mgr.GetClient(),
			LBManager:       kubeLBMgr,
			ClusterName:     clusterName,
			Log:             ctrl.Log.WithName("controllers").WithName(ccm.IngressControllerName),
			Scheme:          mgr.GetScheme(),
			Recorder:        mgr.GetEventRecorderFor(ccm.IngressControllerName),
			UseIngressClass: useIngressClass,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", ccm.IngressControllerName)
			os.Exit(1)
		}
	}

	if !disableGatewayController && !disableGatewayAPI {
		if err = (&ccm.GatewayReconciler{
			Client:          mgr.GetClient(),
			LBManager:       kubeLBMgr,
			ClusterName:     clusterName,
			Log:             ctrl.Log.WithName("controllers").WithName(ccm.GatewayControllerName),
			Scheme:          mgr.GetScheme(),
			Recorder:        mgr.GetEventRecorderFor(ccm.GatewayControllerName),
			UseGatewayClass: useGatewayClass,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", ccm.GatewayControllerName)
			os.Exit(1)
		}
	}

	if !disableHTTPRouteController && !disableGatewayAPI {
		if err = (&ccm.HTTPRouteReconciler{
			Client:      mgr.GetClient(),
			LBManager:   kubeLBMgr,
			ClusterName: clusterName,
			Log:         ctrl.Log.WithName("controllers").WithName(ccm.GatewayHTTPRouteControllerName),
			Scheme:      mgr.GetScheme(),
			Recorder:    mgr.GetEventRecorderFor(ccm.GatewayHTTPRouteControllerName),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", ccm.GatewayHTTPRouteControllerName)
			os.Exit(1)
		}
	}

	if !disableGRPCRouteController && !disableGatewayAPI {
		if err = (&ccm.GRPCRouteReconciler{
			Client:      mgr.GetClient(),
			LBManager:   kubeLBMgr,
			ClusterName: clusterName,
			Log:         ctrl.Log.WithName("controllers").WithName(ccm.GatewayGRPCRouteControllerName),
			Scheme:      mgr.GetScheme(),
			Recorder:    mgr.GetEventRecorderFor(ccm.GatewayGRPCRouteControllerName),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", ccm.GatewayGRPCRouteControllerName)
			os.Exit(1)
		}
	}

	if enableSecretSynchronizer {
		if err = (&ccm.SecretConversionReconciler{
			Client:   mgr.GetClient(),
			Log:      ctrl.Log.WithName("controllers").WithName(ccm.SecretConversionControllerName),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor(ccm.SecretConversionControllerName),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", ccm.SecretConversionControllerName)
			os.Exit(1)
		}
	}

	if err = (&ccm.SyncSecretReconciler{
		Client:      mgr.GetClient(),
		LBClient:    kubeLBMgr.GetClient(),
		Log:         ctrl.Log.WithName("controllers").WithName(ccm.SyncSecretControllerName),
		Scheme:      mgr.GetScheme(),
		ClusterName: clusterName,
		Recorder:    mgr.GetEventRecorderFor(ccm.SyncSecretControllerName),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", ccm.SyncSecretControllerName)
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

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting kubelb CCM")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running kubelb")
		os.Exit(1)
	}
}
