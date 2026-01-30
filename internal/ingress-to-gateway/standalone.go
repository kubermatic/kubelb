/*
Copyright 2026 The KubeLB Authors.

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

package ingressconversion

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// StandaloneConfig holds configuration for running the standalone converter
type StandaloneConfig struct {
	Scheme                  *runtime.Scheme
	MetricsAddr             string
	ProbeAddr               string
	EnableLeaderElection    bool
	LeaderElectionNamespace string
}

// RunStandalone starts the converter as a standalone controller without LB cluster
func RunStandalone(ctx context.Context, cfg StandaloneConfig, opts Options) int {
	setupLog := ctrl.Log.WithName("setup")
	setupLog.Info("starting standalone Ingress-to-Gateway converter")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  cfg.Scheme,
		Metrics:                 metricsserver.Options{BindAddress: cfg.MetricsAddr},
		HealthProbeBindAddress:  cfg.ProbeAddr,
		LeaderElection:          cfg.EnableLeaderElection,
		LeaderElectionID:        "ingress-conversion.ccm.kubelb.k8c.io",
		LeaderElectionNamespace: cfg.LeaderElectionNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		return 1
	}

	if err := SetupReconciler(mgr, opts); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", ControllerName)
		return 1
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return 1
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return 1
	}

	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running standalone converter")
		return 1
	}
	return 0
}

// SetupReconciler creates and registers the Reconciler with the manager
func SetupReconciler(mgr ctrl.Manager, opts Options) error {
	return (&Reconciler{
		Client:               mgr.GetClient(),
		Log:                  ctrl.Log.WithName("controllers").WithName(ControllerName),
		Scheme:               mgr.GetScheme(),
		Recorder:             mgr.GetEventRecorder(ControllerName),
		GatewayName:          opts.GatewayName,
		GatewayNamespace:     opts.GatewayNamespace,
		GatewayClassName:     opts.GatewayClassName,
		DomainReplace:        opts.DomainReplace,
		DomainSuffix:         opts.DomainSuffix,
		PropagateCertManager: opts.PropagateCertManager,
		PropagateExternalDNS: opts.PropagateExternalDNS,
	}).SetupWithManager(mgr)
}
