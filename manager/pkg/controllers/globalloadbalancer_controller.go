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
	glb "manager/pkg/api/v1alpha1"
	kubelbiov1alpha1 "manager/pkg/api/v1alpha1"
	"manager/pkg/lb"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GlobalLoadBalancerReconciler reconciles a GlobalLoadBalancer object
type GlobalLoadBalancerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=globalloadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=globalloadbalancers/status,verbs=get;update;patch

func (r *GlobalLoadBalancerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("globalloadbalancer", req.NamespacedName)

	var globalLoadBalancer kubelbiov1alpha1.GlobalLoadBalancer
	if err := r.Get(ctx, req.NamespacedName, &globalLoadBalancer); err != nil {
		log.Error(err, "unable to fetch GlobalLoadBalancer")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling LB", "name", globalLoadBalancer.Name, "namespace", globalLoadBalancer.Namespace)

	l4Manager, err := lb.NewManager()

	if err != nil {
		log.Error(err, "Unable to create L4 Manager")
		return ctrl.Result{}, err
	}

	if globalLoadBalancer.Spec.Type == glb.Layer4 {
		log.Info("Creating layer 4 laod balancer")
		err = l4Manager.CreateL4LbService(&globalLoadBalancer)

		if err != nil {
			log.Error(err, "Unable to create L4 Manager")
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
