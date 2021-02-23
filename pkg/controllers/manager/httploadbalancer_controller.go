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

package manager

import (
	"context"
	netv1beta1 "k8s.io/api/networking/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=httploadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=httploadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *HTTPLoadBalancerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("reconciling HTTPLoadBalancer")

	var httpLoadBalancer kubelbk8ciov1alpha1.HTTPLoadBalancer
	err := r.Get(ctx, req.NamespacedName, &httpLoadBalancer)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch HTTPLoadBalancer")
		}
		log.V(3).Info("HTTPLoadBalancer not found")
		return ctrl.Result{}, nil
	}

	log.V(5).Info("processing", "HTTPLoadBalancer", httpLoadBalancer)

	err = r.reconcileIngress(ctx, &httpLoadBalancer)

	if err != nil {
		log.Error(err, "Unable to reconcile ingress")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *HTTPLoadBalancerReconciler) reconcileIngress(ctx context.Context, httpLoadBalancer *kubelbk8ciov1alpha1.HTTPLoadBalancer) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "ingress")

	ingress := &netv1beta1.Ingress{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      httpLoadBalancer.Name,
		Namespace: httpLoadBalancer.Namespace,
	}, ingress)

	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	log.V(5).Info("actual", "ingress", ingress)

	result, err := ctrl.CreateOrUpdate(ctx, r.Client, ingress, func() error {

		var rules []netv1beta1.IngressRule

		for _, rule := range httpLoadBalancer.Spec.Rules {

			var paths []netv1beta1.HTTPIngressPath

			for _, path := range rule.HTTP.Paths {
				paths = append(paths, netv1beta1.HTTPIngressPath{
					Path: path.Path,
					Backend: netv1beta1.IngressBackend{
						ServiceName: path.Backend.ServiceName,
						ServicePort: path.Backend.ServicePort,
					},
				})
			}

			rules = append(rules, netv1beta1.IngressRule{
				IngressRuleValue: netv1beta1.IngressRuleValue{
					HTTP: &netv1beta1.HTTPIngressRuleValue{
						Paths: paths,
					},
				},
			})
		}

		ingress.ObjectMeta = v1.ObjectMeta{
			Name:      httpLoadBalancer.Name,
			Namespace: httpLoadBalancer.Namespace,
			Labels:    map[string]string{"app": httpLoadBalancer.Name},
		}

		ingress.Spec.Rules = rules

		return ctrl.SetControllerReference(httpLoadBalancer, ingress, r.Scheme)

	})

	log.V(5).Info("desired", "ingress", ingress)

	log.V(1).Info("status", result)

	return err

}

func (r *HTTPLoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubelbk8ciov1alpha1.HTTPLoadBalancer{}).
		Complete(r)
}
