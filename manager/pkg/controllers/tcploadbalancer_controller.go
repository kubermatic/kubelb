/*


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

package controllers

import (
	"context"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/go-logr/logr"
	kubelbk8ciov1alpha1 "k8c.io/kubelb/manager/pkg/api/kubelb.k8c.io/v1alpha1"
	envoycp "k8c.io/kubelb/manager/pkg/envoy"
	"k8c.io/kubelb/manager/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// TCPLoadBalancerReconciler reconciles a TCPLoadBalancer object
type TCPLoadBalancerReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	Ctx        context.Context
	Cache      cache.Cache
	EnvoyCache cachev3.SnapshotCache
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tcploadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tcploadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *TCPLoadBalancerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("TCPLoadBalancer", req.NamespacedName)

	var tcpLoadBalancer kubelbk8ciov1alpha1.TCPLoadBalancer
	if err := r.Get(r.Ctx, req.NamespacedName, &tcpLoadBalancer); err != nil {
		log.Error(err, "unable to fetch TCPLoadBalancer")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling", "name", tcpLoadBalancer.Name, "namespace", tcpLoadBalancer.Namespace)

	err := r.reconcileEnvoySnapshot(&tcpLoadBalancer)

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
	c, err := ctrl.NewControllerManagedBy(mgr).For(&kubelbk8ciov1alpha1.TCPLoadBalancer{}).Build(r)

	if err != nil {
		return err
	}

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

func (r *TCPLoadBalancerReconciler) reconcileService(tcpLoadBalancer *kubelbk8ciov1alpha1.TCPLoadBalancer) error {
	log := r.Log.WithValues("TCPLoadBalancer", "svc")

	desiredService := resources.MapService(tcpLoadBalancer)

	err := ctrl.SetControllerReference(tcpLoadBalancer, desiredService, r.Scheme)
	if err != nil {
		log.Error(err, "Unable to set controller reference")
		return err
	}

	actualService := &corev1.Service{}
	err = r.Get(r.Ctx, types.NamespacedName{
		Name:      tcpLoadBalancer.Name,
		Namespace: tcpLoadBalancer.Namespace,
	}, actualService)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("Creating service", "namespace", tcpLoadBalancer.Namespace, "name", tcpLoadBalancer.Name)
		return r.Create(r.Ctx, desiredService)
	}

	if !resources.ServiceIsDesiredState(actualService, desiredService) {
		log.Info("Updating service", "namespace", tcpLoadBalancer.Namespace, "name", tcpLoadBalancer.Name)
		actualService.Spec.Ports = desiredService.Spec.Ports
		actualService.Spec.Type = desiredService.Spec.Type
		err = r.Update(r.Ctx, actualService)

		if err != nil {
			return err
		}
	}

	if tcpLoadBalancer.Spec.Type == corev1.ServiceTypeLoadBalancer && len(tcpLoadBalancer.Status.LoadBalancer.Ingress) == 0 && len(actualService.Status.LoadBalancer.Ingress) != 0 {
		tcpLoadBalancer.Status.LoadBalancer = actualService.Status.LoadBalancer
		return r.Status().Update(r.Ctx, tcpLoadBalancer)
	}

	return nil
}

func (r *TCPLoadBalancerReconciler) reconcileEnvoySnapshot(tcpLoadBalancer *kubelbk8ciov1alpha1.TCPLoadBalancer) error {

	log := r.Log.WithValues("TCPLoadBalancer", "snapshot")

	desiredSnapshot := envoycp.MapSnapshot(tcpLoadBalancer)

	// Create the snapshot that we'll serve to Envoy
	if err := desiredSnapshot.Consistent(); err != nil {
		log.Error(err, "snapshot inconsistency", desiredSnapshot)
	}
	log.Info("will serve snapshot", "Snapshot", desiredSnapshot, "NodeId", tcpLoadBalancer.Name)

	// Add the snapshot to the cache
	//Todo: check namespace and node-id uniqueness
	if err := r.EnvoyCache.SetSnapshot(tcpLoadBalancer.Name, desiredSnapshot); err != nil {
		log.Error(err, "snapshot error for", desiredSnapshot)
	}

	return nil
}

func (r *TCPLoadBalancerReconciler) reconcileDeployment(tcpLoadBalancer *kubelbk8ciov1alpha1.TCPLoadBalancer) error {

	log := r.Log.WithValues("TCPLoadBalancer", "deployment")

	desiredDeployment := resources.MapDeployment(tcpLoadBalancer)

	err := ctrl.SetControllerReference(tcpLoadBalancer, desiredDeployment, r.Scheme)
	if err != nil {
		log.Error(err, "Unable to set controller reference")
		return err
	}

	actualDeployment := &appsv1.Deployment{}
	err = r.Get(r.Ctx, types.NamespacedName{
		Name:      tcpLoadBalancer.Name,
		Namespace: tcpLoadBalancer.Namespace,
	}, actualDeployment)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("Creating deployment", "namespace", tcpLoadBalancer.Namespace, "name", tcpLoadBalancer.Name)
		return r.Create(r.Ctx, desiredDeployment)
	}

	log.Info("Updating deployment", "namespace", tcpLoadBalancer.Namespace, "name", tcpLoadBalancer.Name)
	actualDeployment.Spec = desiredDeployment.Spec

	return r.Update(r.Ctx, actualDeployment)

}
