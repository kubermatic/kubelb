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
	"strings"
	"syscall"

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/controllers/ccm"
	ingressconversion "k8c.io/kubelb/internal/ingress-to-gateway"
	ccmmetrics "k8c.io/kubelb/internal/metricsutil/ccm"
	"k8c.io/kubelb/pkg/conversion"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	// Register CCM metrics
	ccmmetrics.Register()

	// +kubebuilder:scaffold:scheme
}

type options struct {
	metricsAddr                string
	probeAddr                  string
	enableCloudController      bool
	useLoadbalancerClass       bool
	enableLeaderElection       bool
	leaderElectionNamespace    string
	endpointAddressTypeString  string
	clusterName                string
	kubeLbKubeconf             string
	kubeconfig                 string
	useIngressClass            bool
	useGatewayClass            bool
	disableIngressController   bool
	disableGatewayController   bool
	disableHTTPRouteController bool
	disableGRPCRouteController bool
	enableSecretSynchronizer   bool
	enableGatewayAPI           bool
	installGatewayAPICRDs      bool
	gatewayAPICRDsChannel      string

	// Ingress-to-Gateway conversion
	conversionOpts conversion.Options
}

func main() {
	opt := &options{}

	if flag.Lookup("kubeconfig") == nil {
		flag.StringVar(&opt.kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	}

	flag.StringVar(&opt.metricsAddr, "metrics-addr", ":9445", "The address the metric endpoint binds to.")
	flag.StringVar(&opt.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&opt.enableLeaderElection, "enable-leader-election", true, "Enable leader election for controller ccm. Enabling this will ensure there is only one active controller ccm.")
	flag.StringVar(&opt.leaderElectionNamespace, "leader-election-namespace", "", "Optionally configure leader election namespace.")

	flag.StringVar(&opt.endpointAddressTypeString, "node-address-type", string(corev1.NodeExternalIP), "The default address type used as an endpoint address for the LoadBalancer service. Valid values are ExternalIP, InternalIP, or Hostname, default is ExternalIP.")
	flag.StringVar(&opt.clusterName, "cluster-name", "", "Cluster name where the ccm is running. Resources inside the KubeLb cluster will get deployed to the namespace named by cluster name, must be unique.")
	flag.StringVar(&opt.kubeLbKubeconf, "kubelb-kubeconfig", defaultKubeLbConf, "The path to the kubelb cluster kubeconfig.")
	flag.BoolVar(&opt.enableCloudController, "enable-cloud-provider", true, "Enables cloud controller like behavior. This will set the status of LoadBalancer")
	flag.BoolVar(&opt.useIngressClass, "use-ingress-class", true, "Use IngressClass `kubelb` to filter Ingress objects. Ingresses should have `ingressClassName: kubelb` set in the spec.")
	flag.BoolVar(&opt.useLoadbalancerClass, "use-loadbalancer-class", false, "Use LoadBalancerClass `kubelb` to filter services. If false, all load balancer services will be managed by KubeLB.")
	flag.BoolVar(&opt.useGatewayClass, "use-gateway-class", true, "Use Gateway Class `kubelb` to filter Gateway objects. Gateway should have `gatewayClassName: kubelb` set in the spec.")

	flag.BoolVar(&opt.disableIngressController, "disable-ingress-controller", false, "Disable the Ingress controller.")
	flag.BoolVar(&opt.disableGatewayController, "disable-gateway-controller", false, "Disable the Gateway controller.")
	flag.BoolVar(&opt.disableHTTPRouteController, "disable-httproute-controller", false, "Disable the HTTPRoute controller.")
	flag.BoolVar(&opt.disableGRPCRouteController, "disable-grpcroute-controller", false, "Disable the GRPCRoute controller.")

	flag.BoolVar(&opt.enableSecretSynchronizer, "enable-secret-synchronizer", false, "Enable to automatically convert Secrets labelled with `kubelb.k8c.io/managed-by: kubelb` to Sync Secrets.  This is used to sync secrets from tenants to the LB cluster in a controlled and secure way.")
	flag.BoolVar(&opt.enableGatewayAPI, "enable-gateway-api", false, "Enable the Gateway APIs and controllers. By default Gateway API is disabled since without Gateway API CRDs installed the controller cannot start.")
	flag.BoolVar(&opt.installGatewayAPICRDs, "install-gateway-api-crds", false, "Installs and manages the Gateway API CRDs using gateway crd controller.")
	flag.StringVar(&opt.gatewayAPICRDsChannel, "gateway-api-crds-channel", "standard", "Gateway API CRDs channel: 'standard' or 'experimental'.")

	opt.conversionOpts.BindFlags(flag.CommandLine)

	opts := zap.Options{
		Development: false,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	// Set up logger before validation checks
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// setup signal handler
	ctx := ctrl.SetupSignalHandler()

	// Standalone conversion mode - skip LB cluster setup
	if opt.conversionOpts.StandaloneMode {
		// Standalone mode needs Gateway API for HTTPRoute creation
		utilruntime.Must(gwapiv1.Install(scheme))

		if !opt.conversionOpts.DisableEnvoyGatewayFeatures {
			utilruntime.Must(egv1alpha1.AddToScheme(scheme))
		}

		os.Exit(ingressconversion.RunStandalone(ctx, ingressconversion.StandaloneConfig{
			Scheme:                  scheme,
			MetricsAddr:             opt.metricsAddr,
			ProbeAddr:               opt.probeAddr,
			EnableLeaderElection:    opt.enableLeaderElection,
			LeaderElectionNamespace: opt.leaderElectionNamespace,
		}, opt.conversionOpts))
	}

	clusterName := opt.clusterName

	if clusterName == "" {
		setupLog.Error(errors.New("cluster/tenant name is required. Please provide a valid cluster/tenant name using the --cluster-name flag"), "cluster/tenant name is required")
		os.Exit(1)
	}

	// If clusterName is not prefixed with "tenant-" then prefix it
	if !strings.HasPrefix(opt.clusterName, "tenant-") {
		clusterName = "tenant-" + clusterName
	}

	if opt.enableGatewayAPI {
		utilruntime.Must(gwapiv1.Install(scheme))
	}

	opt.kubeconfig = flag.Lookup("kubeconfig").Value.(flag.Getter).Get().(string)

	setupLog.V(1).Info("cluster", "name", clusterName)

	// Validate gatewayAPICRDsChannel
	if !ccm.IsValidGatewayAPICRDsChannel(opt.gatewayAPICRDsChannel) {
		setupLog.Error(nil, "Invalid value for --gateway-api-crds-channel. Must be 'standard' or 'experimental'.", "current value", opt.gatewayAPICRDsChannel)
		os.Exit(1)
	}

	var endpointAddressType corev1.NodeAddressType
	switch opt.endpointAddressTypeString {
	case string(corev1.NodeInternalIP):
		endpointAddressType = corev1.NodeInternalIP
	case string(corev1.NodeExternalIP):
		endpointAddressType = corev1.NodeExternalIP
	case string(corev1.NodeHostName):
		endpointAddressType = corev1.NodeHostName
	default:
		setupLog.Error(errors.New("invalid node address type"), fmt.Sprintf("Expected: %s, %s, or %s, got: %s", corev1.NodeInternalIP, corev1.NodeExternalIP, corev1.NodeHostName, opt.endpointAddressTypeString))
		os.Exit(1)
	}

	setupLog.V(1).Info("using endpoint address", "type", endpointAddressType)

	kubeLBClientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{
			ExplicitPath: opt.kubeLbKubeconf,
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

	restConfig := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsserver.Options{BindAddress: opt.metricsAddr},
		HealthProbeBindAddress:  opt.probeAddr,
		LeaderElection:          opt.enableLeaderElection,
		LeaderElectionID:        "19f32e7b.ccm.kubelb.k8c.io",
		LeaderElectionNamespace: opt.leaderElectionNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start ccm manager")
		os.Exit(1)
	}

	if err := mgr.Add(kubeLBMgr); err != nil {
		setupLog.Error(err, "unable to start kubelb manager")
		os.Exit(1)
	}

	if err := setupControllers(mgr, kubeLBMgr, setupLog, opt, clusterName, endpointAddressType); err != nil {
		setupLog.Error(err, "unable to setup controllers")
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

	// Install CRDs synchronously before starting manager to avoid race conditions
	// with controllers that watch these CRDs. Uses direct client since manager cache isn't started yet.
	if opt.installGatewayAPICRDs {
		directClient, err := client.New(restConfig, client.Options{Scheme: scheme})
		if err != nil {
			setupLog.Error(err, "unable to create direct client for CRD installation")
			os.Exit(1)
		}
		if err := ccm.InstallCRDs(ctx, directClient, setupLog, ccm.GatewayAPIChannel(opt.gatewayAPICRDsChannel)); err != nil {
			setupLog.Error(err, "unable to install Gateway API CRDs")
			os.Exit(1)
		}
	}

	// Mark KubeLB cluster as connected since manager setup succeeded
	ccmmetrics.KubeLBClusterConnected.Set(1)

	setupLog.Info("starting kubelb CCM")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running kubelb")
		os.Exit(1)
	}
}

// setupControllers sets up all the controllers with the given configuration
func setupControllers(mgr, kubeLBMgr ctrl.Manager, setupLog logr.Logger, opt *options, clusterName string, endpointAddressType corev1.NodeAddressType) error {
	if err := (&ccm.KubeLBNodeReconciler{
		Client:              mgr.GetClient(),
		Log:                 ctrl.Log.WithName("kubelb.node.reconciler"),
		Scheme:              mgr.GetScheme(),
		KubeLBClient:        kubeLBMgr.GetClient(),
		EndpointAddressType: endpointAddressType,
		ClusterName:         clusterName,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "reconciler", "kubelb.node.reconciler")
		return err
	}

	if err := (&ccm.KubeLBServiceReconciler{
		Client:               mgr.GetClient(),
		KubeLBManager:        kubeLBMgr,
		Log:                  ctrl.Log.WithName("kubelb.service.reconciler"),
		Scheme:               mgr.GetScheme(),
		CloudController:      opt.enableCloudController,
		UseLoadbalancerClass: opt.useLoadbalancerClass,
		ClusterName:          clusterName,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "reconciler", "kubelb.service.reconciler")
		return err
	}

	if !opt.disableIngressController {
		if err := (&ccm.IngressReconciler{
			Client:          mgr.GetClient(),
			LBManager:       kubeLBMgr,
			ClusterName:     clusterName,
			Log:             ctrl.Log.WithName("controllers").WithName(ccm.IngressControllerName),
			Scheme:          mgr.GetScheme(),
			Recorder:        mgr.GetEventRecorder(ccm.IngressControllerName),
			UseIngressClass: opt.useIngressClass,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", ccm.IngressControllerName)
			return err
		}
	}

	if opt.installGatewayAPICRDs {
		if err := (&ccm.GatewayCRDReconciler{
			Client:  mgr.GetClient(),
			Log:     ctrl.Log.WithName("controllers").WithName(ccm.GatewayCRDControllerName),
			Channel: ccm.GatewayAPIChannel(opt.gatewayAPICRDsChannel),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", ccm.GatewayCRDControllerName)
			return err
		}
	}

	if !opt.disableGatewayController && opt.enableGatewayAPI {
		if err := (&ccm.GatewayReconciler{
			Client:          mgr.GetClient(),
			LBManager:       kubeLBMgr,
			ClusterName:     clusterName,
			Log:             ctrl.Log.WithName("controllers").WithName(ccm.GatewayControllerName),
			Scheme:          mgr.GetScheme(),
			Recorder:        mgr.GetEventRecorder(ccm.GatewayControllerName),
			UseGatewayClass: opt.useGatewayClass,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", ccm.GatewayControllerName)
			return err
		}
	}

	if !opt.disableHTTPRouteController && opt.enableGatewayAPI {
		if err := (&ccm.HTTPRouteReconciler{
			Client:      mgr.GetClient(),
			LBManager:   kubeLBMgr,
			ClusterName: clusterName,
			Log:         ctrl.Log.WithName("controllers").WithName(ccm.GatewayHTTPRouteControllerName),
			Scheme:      mgr.GetScheme(),
			Recorder:    mgr.GetEventRecorder(ccm.GatewayHTTPRouteControllerName),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", ccm.GatewayHTTPRouteControllerName)
			return err
		}
	}

	if !opt.disableGRPCRouteController && opt.enableGatewayAPI {
		if err := (&ccm.GRPCRouteReconciler{
			Client:      mgr.GetClient(),
			LBManager:   kubeLBMgr,
			ClusterName: clusterName,
			Log:         ctrl.Log.WithName("controllers").WithName(ccm.GatewayGRPCRouteControllerName),
			Scheme:      mgr.GetScheme(),
			Recorder:    mgr.GetEventRecorder(ccm.GatewayGRPCRouteControllerName),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", ccm.GatewayGRPCRouteControllerName)
			return err
		}
	}

	if opt.enableSecretSynchronizer {
		if err := (&ccm.SecretConversionReconciler{
			Client:   mgr.GetClient(),
			Log:      ctrl.Log.WithName("controllers").WithName(ccm.SecretConversionControllerName),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorder(ccm.SecretConversionControllerName),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", ccm.SecretConversionControllerName)
			return err
		}
	}

	if err := (&ccm.SyncSecretReconciler{
		Client:      mgr.GetClient(),
		LBClient:    kubeLBMgr.GetClient(),
		Log:         ctrl.Log.WithName("controllers").WithName(ccm.SyncSecretControllerName),
		Scheme:      mgr.GetScheme(),
		ClusterName: clusterName,
		Recorder:    mgr.GetEventRecorder(ccm.SyncSecretControllerName),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", ccm.SyncSecretControllerName)
		return err
	}

	if opt.conversionOpts.Enabled {
		if err := opt.conversionOpts.Validate(); err != nil {
			return err
		}
		if err := ingressconversion.SetupReconciler(mgr, opt.conversionOpts); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", conversion.ControllerName)
			return err
		}
	}

	return nil
}
