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

package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	kubelbiov1alpha1 "manager/pkg/api/globalloadbalancer/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GlobalLoadBalancerReconciler reconciles a GlobalLoadBalancer object
type GlobalLoadBalancerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	ctx    context.Context
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=globalloadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=globalloadbalancers/status,verbs=get;update;patch

func (r *GlobalLoadBalancerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	r.ctx = context.Background()
	log := r.Log.WithValues("globalloadbalancer", req.NamespacedName)

	var globalLoadBalancer kubelbiov1alpha1.GlobalLoadBalancer
	if err := r.Get(r.ctx, req.NamespacedName, &globalLoadBalancer); err != nil {
		log.Error(err, "unable to fetch GlobalLoadBalancer")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling LB", "name", globalLoadBalancer.Name, "namespace", globalLoadBalancer.Namespace)

	if globalLoadBalancer.Spec.Type == kubelbiov1alpha1.Layer4 {
		log.Info("handling layer 4 laod balancer")
		err := r.handleL4(&globalLoadBalancer)

		if err != nil {
			log.Error(err, "Unable to handle L4 load balancer!")
			return ctrl.Result{}, err
		}

	}

	return ctrl.Result{}, nil
}

func (r *GlobalLoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubelbiov1alpha1.GlobalLoadBalancer{}).
		Complete(r)
}
