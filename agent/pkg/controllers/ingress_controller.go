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
	"k8c.io/kubelb/agent/pkg/kubelb"
	kubelbv1alpha1 "k8c.io/kubelb/manager/pkg/generated/clientset/versioned/typed/kubelb.k8c.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	netv1beta1 "k8s.io/api/networking/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KubeLbIngressReconciler reconciles a Service object
type KubeLbIngressReconciler struct {
	client.Client
	ClusterCache cache.Cache
	HttpLBClient kubelbv1alpha1.HTTPLoadBalancerInterface
	Log          logr.Logger
	Scheme       *runtime.Scheme
	ctx          context.Context
	ClusterName  string
}

var IngressMatcher = &kubelb.MatchingAnnotationPredicate{
	AnnotationName:  "kubernetes.io/ingress.class",
	AnnotationValue: "kubelb",
}

// +kubebuilder:rbac:groups="",resources=ingress,verbs=get;list;watch;update;patch
func (r *KubeLbIngressReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	r.ctx = context.Background()
	log := r.Log.WithValues("kubelb_ingress_agent", req.NamespacedName)

	var ingress netv1beta1.Ingress
	err := r.Get(r.ctx, req.NamespacedName, &ingress)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch ingress")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling ingress", "name", ingress.Name, "namespace", ingress.Namespace)

	err = r.exposeIngressBackendServices(&ingress)

	if err != nil {
		return ctrl.Result{}, err
	}

	desiredHttpLoadBalancer := kubelb.MapHttpLoadBalancer(&ingress, r.ClusterName)

	actualHttpLoadBalancer, err := r.HttpLBClient.Get(ingress.Name, v1.GetOptions{})

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.Info("Creating HttpLoadBalancer", "namespace", desiredHttpLoadBalancer.Namespace, "name", desiredHttpLoadBalancer.Name)
		_, err = r.HttpLBClient.Create(desiredHttpLoadBalancer)

		return ctrl.Result{}, err
	}

	log.Info("Updating HttpLoadBalancer", "namespace", desiredHttpLoadBalancer.Namespace, "name", desiredHttpLoadBalancer.Name)
	actualHttpLoadBalancer.Spec = desiredHttpLoadBalancer.Spec
	_, err = r.HttpLBClient.Update(actualHttpLoadBalancer)

	return ctrl.Result{}, err
}

func (r *KubeLbIngressReconciler) exposeIngressBackendServices(ingress *netv1beta1.Ingress) error {

	log := r.Log.WithValues("kubelb_ingress_agent", "exposing_backends")

	for _, rule := range ingress.Spec.Rules {
		for _, path := range rule.HTTP.Paths {

			var service corev1.Service
			err := r.Get(r.ctx, types.NamespacedName{Name: path.Backend.ServiceName, Namespace: ingress.Namespace}, &service)
			if err != nil {
				log.Error(err, "unable to fetch backend service")
				return err
			}

			if ServiceMatcher.Match(service.GetAnnotations()) {
				log.Info("Service already exposed by kubelb")
				continue
			}

			service.Annotations[ServiceMatcher.AnnotationName] = ServiceMatcher.AnnotationValue
			err = r.Update(r.ctx, &service)
			if err != nil {
				log.Error(err, "unable to set backend service annotation")
				return err
			}
		}
	}
	return nil
}

func (r *KubeLbIngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1beta1.Ingress{}).
		WithEventFilter(IngressMatcher).
		Complete(r)
}