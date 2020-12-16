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

package kubelb

import (
	"context"
	"github.com/Masterminds/semver/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	envoyresource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	envoycp "k8c.io/kubelb/pkg/envoy"
	"k8c.io/kubelb/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// TCPLoadBalancerReconciler reconciles a TCPLoadBalancer object
type TCPLoadBalancerReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	Ctx            context.Context
	Cache          cache.Cache
	EnvoyCache     cachev3.SnapshotCache
	EnvoyBootstrap string
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tcploadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tcploadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *TCPLoadBalancerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.Name, "namespace", req.Namespace)
	log.V(2).Info("reconciling TCPLoadBalancer")

	var tcpLoadBalancer kubelbk8ciov1alpha1.TCPLoadBalancer
	err := r.Get(r.Ctx, req.NamespacedName, &tcpLoadBalancer)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch TCPLoadBalancer")
		}
		log.V(3).Info("TCPLoadBalancer not found")
		return ctrl.Result{}, nil
	}

	log.V(6).Info("processing", "TCPLoadBalancer", tcpLoadBalancer)

	err = r.reconcileEnvoySnapshot(&tcpLoadBalancer)

	if err != nil {
		log.Error(err, "Unable to reconcile envoy snapshot")
		return ctrl.Result{}, err
	}

	err = r.reconcileDeployment(&tcpLoadBalancer)

	if err != nil {
		log.Error(err, "Unable to reconcile deployment")
		return ctrl.Result{}, err
	}

	err = r.reconcileService(&tcpLoadBalancer)

	if err != nil {
		log.Error(err, "Unable to reconcile service")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil

}

func (r *TCPLoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&kubelbk8ciov1alpha1.TCPLoadBalancer{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Build(r)

	if err != nil {
		return err
	}

	//Todo: same as owns? can be removed
	serviceInformer, err := r.Cache.GetInformer(&corev1.Service{})
	if err != nil {
		r.Log.Error(err, "error occurred while getting service informer")
		return err
	}

	err = c.Watch(
		&source.Informer{Informer: serviceInformer},
		&handler.EnqueueRequestForOwner{
			OwnerType:    &kubelbk8ciov1alpha1.TCPLoadBalancer{},
			IsController: true,
		},
	)

	return nil
}

func (r *TCPLoadBalancerReconciler) reconcileEnvoySnapshot(tcpLoadBalancer *kubelbk8ciov1alpha1.TCPLoadBalancer) error {

	log := r.Log.WithValues("reconcile", "envoy")

	// Get current snapshot
	currSnapshot, err := r.EnvoyCache.GetSnapshot(tcpLoadBalancer.Name)
	//Todo: check for not found error and return if not
	if err != nil {
		// Add the snapshot to the cache
		//Todo: check namespace and node-id uniqueness
		initSnapshot := envoycp.MapSnapshot(tcpLoadBalancer, "0.0.1")
		log.V(1).Info("init snapshot", "service-node", tcpLoadBalancer.Name, "version", "0.0.1")
		log.V(6).Info("serving", "snapshot", initSnapshot)

		if err := r.EnvoyCache.SetSnapshot(tcpLoadBalancer.Name, initSnapshot); err != nil {
			log.Error(err, "failed to set a new snapshot")
		}
		return nil
	}

	lastUsedVersion, err := semver.NewVersion(currSnapshot.GetVersion(envoyresource.ClusterType))
	if err != nil {
		return errors.Wrap(err, "failed to parse version from last snapshot")
	}

	// Generate a new snapshot using the old version to be able to do a DeepEqual comparison
	if reflect.DeepEqual(currSnapshot, envoycp.MapSnapshot(tcpLoadBalancer, lastUsedVersion.String())) {
		log.V(2).Info("snapshot is in desired state")
		return nil
	}

	newVersion := lastUsedVersion.IncMajor()
	log.Info("detected a change. Updating the Envoy config cache...", "version", newVersion.String())

	newSnapshot := envoycp.MapSnapshot(tcpLoadBalancer, newVersion.String())

	if err := newSnapshot.Consistent(); err != nil {
		return errors.Wrap(err, "new Envoy config snapshot is not consistent")
	}
	log.V(1).Info("updating snapshot", "service-node", tcpLoadBalancer.Name, "version", newVersion.String())
	log.V(6).Info("serving", "snapshot", newSnapshot)

	if err := r.EnvoyCache.SetSnapshot(tcpLoadBalancer.Name, newSnapshot); err != nil {
		return errors.Wrap(err, "failed to set a new Envoy cache snapshot")
	}

	return nil
}

func (r *TCPLoadBalancerReconciler) reconcileDeployment(tcpLoadBalancer *kubelbk8ciov1alpha1.TCPLoadBalancer) error {

	log := r.Log.WithValues("reconcile", "deployment")

	desiredDeployment := resources.MapDeployment(tcpLoadBalancer, r.EnvoyBootstrap)
	err := ctrl.SetControllerReference(tcpLoadBalancer, desiredDeployment, r.Scheme)
	if err != nil {
		log.Error(err, "Unable to set controller reference")
		return err
	}

	log.V(6).Info("desired", "deployment", desiredDeployment)

	actualDeployment := &appsv1.Deployment{}
	err = r.Get(r.Ctx, types.NamespacedName{
		Name:      tcpLoadBalancer.Name,
		Namespace: tcpLoadBalancer.Namespace,
	}, actualDeployment)

	log.V(6).Info("actual", "deployment", actualDeployment)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.V(1).Info("creating deployment", "name", tcpLoadBalancer.Name, "namespace", tcpLoadBalancer.Namespace)
		return r.Create(r.Ctx, desiredDeployment)
	}

	log.V(1).Info("updating deployment", "name", tcpLoadBalancer.Name, "namespace", tcpLoadBalancer.Namespace)
	actualDeployment.Spec = desiredDeployment.Spec

	log.V(7).Info("updated to", "deployment", actualDeployment)

	return r.Update(r.Ctx, actualDeployment)

}

func (r *TCPLoadBalancerReconciler) reconcileService(tcpLoadBalancer *kubelbk8ciov1alpha1.TCPLoadBalancer) error {
	log := r.Log.WithValues("reconcile", "service")

	desiredService := resources.MapService(tcpLoadBalancer)
	err := ctrl.SetControllerReference(tcpLoadBalancer, desiredService, r.Scheme)
	if err != nil {
		log.Error(err, "Unable to set controller reference")
		return err
	}
	log.V(6).Info("desired", "service", desiredService)

	actualService := &corev1.Service{}
	err = r.Get(r.Ctx, types.NamespacedName{
		Name:      tcpLoadBalancer.Name,
		Namespace: tcpLoadBalancer.Namespace,
	}, actualService)

	log.V(6).Info("actual", "service", actualService)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.V(1).Info("creating service", "name", tcpLoadBalancer.Name, "namespace", tcpLoadBalancer.Namespace)
		return r.Create(r.Ctx, desiredService)
	}

	if resources.ServiceIsDesiredState(actualService, desiredService) {
		log.V(2).Info("service is in desired state")

	} else {
		log.V(1).Info("updating service", "name", tcpLoadBalancer.Name, "namespace", tcpLoadBalancer.Namespace)
		actualService.Spec.Ports = desiredService.Spec.Ports
		actualService.Spec.Type = desiredService.Spec.Type
		log.V(7).Info("updated to", "service", actualService)

		err = r.Update(r.Ctx, actualService)

		if err != nil {
			return err
		}
	}

	log.V(6).Info("load balancer status", "TcpLoadBalancer", tcpLoadBalancer.Status.LoadBalancer.Ingress, "service", actualService.Status.LoadBalancer.Ingress)

	if tcpLoadBalancer.Spec.Type != corev1.ServiceTypeLoadBalancer || len(tcpLoadBalancer.Status.LoadBalancer.Ingress) == len(actualService.Status.LoadBalancer.Ingress) {
		log.V(2).Info("TcpLoadBalancer status is in desired state")
	}

	log.V(1).Info("updating TcpLoadBalancer status", "name", tcpLoadBalancer.Name, "namespace", tcpLoadBalancer.Namespace)

	tcpLoadBalancer.Status.LoadBalancer = actualService.Status.LoadBalancer
	log.V(7).Info("updating to", "TcpLoadBalancer status", tcpLoadBalancer.Status.LoadBalancer)

	return r.Status().Update(r.Ctx, tcpLoadBalancer)

}
