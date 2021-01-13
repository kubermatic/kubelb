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

package agent

import (
	"context"
	"github.com/go-logr/logr"
	utils "k8c.io/kubelb/pkg/controllers"
	kubelbv1alpha1 "k8c.io/kubelb/pkg/generated/clientset/versioned/typed/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/pkg/kubelb"
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

const HttpLbFinalizerName = "kubelb.k8c.io/httplb-finalizer"

// KubeLbIngressReconciler reconciles a Service object
type KubeLbIngressReconciler struct {
	client.Client
	ClusterCache cache.Cache
	HttpLBClient kubelbv1alpha1.HTTPLoadBalancerInterface
	Log          logr.Logger
	Scheme       *runtime.Scheme

	ClusterName string
}

var IngressMatcher = &utils.MatchingAnnotationPredicate{
	AnnotationName:  "kubernetes.io/ingress.class",
	AnnotationValue: "kubelb",
}

// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;update;patch

func (r *KubeLbIngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.Name, "namespace", req.Namespace)
	log.V(2).Info("reconciling ingress")

	var ingress netv1beta1.Ingress
	err := r.Get(ctx, req.NamespacedName, &ingress)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch ingress")
		}
		log.V(3).Info("ingress not found")
		return ctrl.Result{}, nil
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if ingress.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !utils.ContainsString(ingress.ObjectMeta.Finalizers, HttpLbFinalizerName) {
			ingress.ObjectMeta.Finalizers = append(ingress.ObjectMeta.Finalizers, HttpLbFinalizerName)
			log.V(4).Info("setting finalizer")
			if err := r.Update(ctx, &ingress); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if utils.ContainsString(ingress.ObjectMeta.Finalizers, HttpLbFinalizerName) {

			log.V(1).Info("deleting HttpLoadBalancer", "name", kubelb.NamespacedName(&ingress.ObjectMeta))

			// our finalizer is present, so lets handle any external dependency
			err := r.HttpLBClient.Delete(ctx, kubelb.NamespacedName(&ingress.ObjectMeta), v1.DeleteOptions{})

			if client.IgnoreNotFound(err) != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			} else {
				log.V(3).Info("HttpLoadBalancer not found")
			}

			// remove our finalizer from the list and update it.
			ingress.ObjectMeta.Finalizers = utils.RemoveString(ingress.ObjectMeta.Finalizers, HttpLbFinalizerName)
			if err := r.Update(ctx, &ingress); err != nil {
				return ctrl.Result{}, err
			}
			log.V(4).Info("removed finalizer")
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	err = r.exposeBackendServices(ctx, &ingress)

	if err != nil {
		return ctrl.Result{}, err
	}

	desiredHttpLoadBalancer := kubelb.MapHttpLoadBalancer(&ingress, r.ClusterName)
	log.V(6).Info("desired", "HttpLoadBalancer", desiredHttpLoadBalancer)

	actualHttpLoadBalancer, err := r.HttpLBClient.Get(ctx, ingress.Name, v1.GetOptions{})
	log.V(6).Info("actual", "HttpLoadBalancer", actualHttpLoadBalancer)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.V(1).Info("creating HttpLoadBalancer", "name", desiredHttpLoadBalancer.Name, "namespace", desiredHttpLoadBalancer.Namespace)
		_, err = r.HttpLBClient.Create(ctx, desiredHttpLoadBalancer, v1.CreateOptions{})
		return ctrl.Result{}, err
	}

	log.V(1).Info("Updating HttpLoadBalancer", "name", desiredHttpLoadBalancer.Name, "namespace", desiredHttpLoadBalancer.Namespace)
	actualHttpLoadBalancer.Spec = desiredHttpLoadBalancer.Spec
	_, err = r.HttpLBClient.Update(ctx, actualHttpLoadBalancer, v1.UpdateOptions{})

	return ctrl.Result{}, err
}

func (r *KubeLbIngressReconciler) exposeBackendServices(ctx context.Context, ingress *netv1beta1.Ingress) error {
	log := r.Log

	for _, rule := range ingress.Spec.Rules {
		for _, path := range rule.HTTP.Paths {

			var service corev1.Service
			err := r.Get(ctx, types.NamespacedName{Name: path.Backend.ServiceName, Namespace: ingress.Namespace}, &service)
			if err != nil {
				log.Error(err, "unable to fetch backend service")
				return err
			}

			log.V(6).Info("exposing", "service", service)

			if AnnotationServiceClassMatcher.Match(service.GetAnnotations()) {
				log.V(3).Info("service already exposed by kubelb")
				continue
			}

			service.Annotations[AnnotationServiceClassMatcher.AnnotationName] = AnnotationServiceClassMatcher.AnnotationValue
			err = r.Update(ctx, &service)
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
