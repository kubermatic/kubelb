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
	"k8c.io/kubelb/pkg/resources"
	netv1beta1 "k8s.io/api/networking/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
)

// HTTPLoadBalancerReconciler reconciles a HTTPLoadBalancer object
type HTTPLoadBalancerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Ctx    context.Context
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=httploadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=httploadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *HTTPLoadBalancerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("HTTPLoadBalancer", req.NamespacedName)

	var httpLoadBalancer kubelbk8ciov1alpha1.HTTPLoadBalancer
	if err := r.Get(r.Ctx, req.NamespacedName, &httpLoadBalancer); err != nil {
		log.Error(err, "unable to fetch HTTPLoadBalancer")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling", "name", httpLoadBalancer.Name, "namespace", httpLoadBalancer.Namespace)

	//Todo: replace this with envoy control-plane stuff
	err := r.reconcileIngress(&httpLoadBalancer)

	if err != nil {
		log.Error(err, "Unable to reconcile ingress")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *HTTPLoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubelbk8ciov1alpha1.HTTPLoadBalancer{}).
		Complete(r)
}

func (r *HTTPLoadBalancerReconciler) reconcileIngress(httpLoadBalancer *kubelbk8ciov1alpha1.HTTPLoadBalancer) error {
	log := r.Log.WithValues("TCPLoadBalancer", "resources-svc")

	desiredIngress := resources.MapIngress(httpLoadBalancer)

	err := ctrl.SetControllerReference(httpLoadBalancer, desiredIngress, r.Scheme)
	if err != nil {
		log.Error(err, "Unable to set controller reference")
		return err
	}

	actualIngress := &netv1beta1.Ingress{}
	err = r.Get(r.Ctx, types.NamespacedName{
		Name:      httpLoadBalancer.Name,
		Namespace: httpLoadBalancer.Namespace,
	}, actualIngress)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("Creating ingress", "namespace", httpLoadBalancer.Namespace, "name", httpLoadBalancer.Name)
		return r.Create(r.Ctx, desiredIngress)
	}

	actualIngress.Spec = desiredIngress.Spec

	return r.Update(r.Ctx, actualIngress)

}
