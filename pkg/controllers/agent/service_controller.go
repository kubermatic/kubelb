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
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	utils "k8c.io/kubelb/pkg/controllers"
	"k8c.io/kubelb/pkg/generated/clientset/versioned/typed/kubelb.k8c.io/v1alpha1"
	kubelbk8ciov1alpha1informers "k8c.io/kubelb/pkg/generated/informers/externalversions/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/pkg/kubelb"
)

const TcpLbFinalizerName = "kubelb.k8c.io/tcplb-finalizer"

// KubeLbServiceReconciler reconciles a Service object
type KubeLbServiceReconciler struct {
	client.Client
	TcpLBClient             v1alpha1.TCPLoadBalancerInterface
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	ClusterName             string
	CloudController         bool
	Endpoints               *kubelb.Endpoints
	TcpLoadBalancerInformer kubelbk8ciov1alpha1informers.TCPLoadBalancerInformer
}

var AnnotationServiceClassMatcher = &utils.MatchingAnnotationPredicate{
	AnnotationName:  "kubernetes.io/service.class",
	AnnotationValue: "kubelb",
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get;update;patch

func (r *KubeLbServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log := r.Log.WithValues("name", req.Name, "namespace", req.Namespace)
	log.V(2).Info("reconciling service")

	var service corev1.Service
	err := r.Get(ctx, req.NamespacedName, &service)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch service")
		}
		log.V(3).Info("service not found")
		return ctrl.Result{}, nil
	}

	if service.Spec.Type != corev1.ServiceTypeNodePort && service.Spec.Type != corev1.ServiceTypeLoadBalancer ||
		(!r.CloudController || service.Spec.Type == corev1.ServiceTypeNodePort) && !AnnotationServiceClassMatcher.Match(service.GetAnnotations()) {
		return ctrl.Result{}, nil
	}

	clusterEndpoints := r.getEndpoints(&service)

	log.V(6).Info("processing", "service", service)
	log.V(5).Info("proceeding with", "endpoints", clusterEndpoints)

	// examine DeletionTimestamp to determine if object is under deletion
	if service.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !utils.ContainsString(service.ObjectMeta.Finalizers, TcpLbFinalizerName) {
			service.ObjectMeta.Finalizers = append(service.ObjectMeta.Finalizers, TcpLbFinalizerName)
			log.V(4).Info("setting finalizer")
			if err := r.Update(ctx, &service); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if utils.ContainsString(service.ObjectMeta.Finalizers, TcpLbFinalizerName) {

			log.V(1).Info("deleting TCPLoadBalancer", "name", kubelb.NamespacedName(&service.ObjectMeta))

			// our finalizer is present, so lets handle any external dependency
			err := r.TcpLBClient.Delete(ctx, kubelb.NamespacedName(&service.ObjectMeta), v1.DeleteOptions{})

			if client.IgnoreNotFound(err) != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			} else {
				log.V(3).Info("TCPLoadBalancer not found")
			}

			// remove our finalizer from the list and update it.
			service.ObjectMeta.Finalizers = utils.RemoveString(service.ObjectMeta.Finalizers, TcpLbFinalizerName)
			if err := r.Update(ctx, &service); err != nil {
				return ctrl.Result{}, err
			}
			log.V(4).Info("removed finalizer")
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	log.V(5).Info("proceeding with", "endpoints", clusterEndpoints)

	desiredTcpLB := kubelb.MapTcpLoadBalancer(&service, clusterEndpoints, r.ClusterName)
	log.V(6).Info("desired", "TcpLoadBalancer", desiredTcpLB)

	actualTcpLB, err := r.TcpLBClient.Get(ctx, desiredTcpLB.Name, v1.GetOptions{})
	log.V(6).Info("actual", "TcpLoadBalancer", actualTcpLB)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.V(1).Info("creating TcpLoadBalancer", "name", desiredTcpLB.Name, "namespace", desiredTcpLB.Namespace)
		_, err = r.TcpLBClient.Create(ctx, desiredTcpLB, v1.CreateOptions{})
		return ctrl.Result{}, err
	}

	log.V(6).Info("load balancer status", "TcpLoadBalancer", actualTcpLB.Status.LoadBalancer.Ingress, "service", service.Status.LoadBalancer.Ingress)

	if service.Spec.Type != corev1.ServiceTypeLoadBalancer || len(actualTcpLB.Status.LoadBalancer.Ingress) == len(service.Status.LoadBalancer.Ingress) {
		log.V(2).Info("service status is in desired state")
	} else {
		log.V(1).Info("updating service status", "name", desiredTcpLB.Name, "namespace", desiredTcpLB.Namespace)
		service.Status.LoadBalancer = actualTcpLB.Status.LoadBalancer
		log.V(7).Info("updating to", "service status", service.Status.LoadBalancer)

		err = r.Client.Status().Update(ctx, &service)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if kubelb.TcpLoadBalancerIsDesiredState(actualTcpLB, desiredTcpLB) {
		log.V(2).Info("TcpLoadBalancer is in desired state")
		return ctrl.Result{}, nil
	}

	log.V(1).Info("updating TcpLoadBalancer spec", "name", desiredTcpLB.Name, "namespace", desiredTcpLB.Namespace)
	actualTcpLB.Spec = desiredTcpLB.Spec
	_, err = r.TcpLBClient.Update(ctx, actualTcpLB, v1.UpdateOptions{})

	if err != nil {
		return ctrl.Result{}, err
	}
	log.V(7).Info("updated to", "TcpLoadBalancer", actualTcpLB)

	return ctrl.Result{}, err
}

func (r *KubeLbServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {

	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).Build(r)

	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Informer{Informer: r.TcpLoadBalancerInformer.Informer()},
		handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {

			if a.GetNamespace() != r.ClusterName {
				return []reconcile.Request{}
			}

			tcpLb := a.(*kubelbk8ciov1alpha1.TCPLoadBalancer)

			if tcpLb.Spec.Type != corev1.ServiceTypeLoadBalancer {
				return []reconcile.Request{}
			}

			originalNamespace, ok := a.GetLabels()[kubelb.LabelOriginNamespace]
			if !ok || originalNamespace == "" {
				r.Log.Error(fmt.Errorf("required label \"%s\" not found", kubelb.LabelOriginNamespace), fmt.Sprintf("failed to queue service for TcpLoadBalacner: %s, could not determine origin namespace", a.GetName()))
				return []reconcile.Request{}
			}

			originalName, ok := a.GetLabels()[kubelb.LabelOriginName]
			if !ok || originalName == "" {
				r.Log.Error(fmt.Errorf("required label \"%s\" not found", kubelb.LabelOriginName), fmt.Sprintf("failed to queue service for TcpLoadBalacner: %s, could not determine origin name", a.GetName()))
				return []reconcile.Request{}
			}

			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      originalName,
						Namespace: originalNamespace,
					},
				},
			}
		}),
	)

	return err

}

func (r *KubeLbServiceReconciler) getEndpoints(service *corev1.Service) []string {

	var clusterEndpoints []string

	//Use LB Endpoint if there is any non KubeLb load balancer implementation
	if service.Spec.Type == corev1.ServiceTypeLoadBalancer && !r.CloudController {
		for _, lbIngress := range service.Status.LoadBalancer.Ingress {
			if lbIngress.IP != "" {
				clusterEndpoints = append(clusterEndpoints, lbIngress.IP)
			} else {
				clusterEndpoints = append(clusterEndpoints, lbIngress.Hostname)
			}
		}
	} else {
		clusterEndpoints = r.Endpoints.ClusterEndpoints
	}

	return clusterEndpoints

}