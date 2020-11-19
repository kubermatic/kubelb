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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KubeLbIngressReconciler reconciles a Service object
type KubeLbServiceReconciler struct {
	client.Client
	TcpLBClient *kubelb.TcpLBClient
	Log         logr.Logger
	Scheme      *runtime.Scheme
	ctx         context.Context
	ClusterName string
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get
func (r *KubeLbServiceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	r.ctx = context.Background()
	log := r.Log.WithValues("cluster", req.NamespacedName)

	var service corev1.Service
	err := r.Get(r.ctx, req.NamespacedName, &service)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch service")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	nodes := &corev1.NodeList{}
	err = r.List(context.Background(), nodes)

	if err != nil {
		log.Error(err, "unable to list nodes")
		return ctrl.Result{}, err
	}

	clusterEndpoints := kubelb.GetEndpoints(nodes, corev1.NodeInternalIP)

	if service.Spec.Type != corev1.ServiceTypeNodePort {
		return ctrl.Result{}, nil
	}

	log.Info("reconciling service", "name", service.Name, "namespace", service.Namespace)

	desiredTcpLB := kubelb.MapTcpLoadBalancer(&service, clusterEndpoints, r.ClusterName)

	//Todo: not possible because this resource lives in another cluster
	// Probably use finalizers here instead
	//err := ctrl.SetControllerReference(userService, desiredTcpLB, r.Scheme)
	//if err != nil {
	//	log.Error(err, "Unable to set controller reference")
	//	return err
	//}

	actualTcpLB, err := r.TcpLBClient.Get(service.Name, v1.GetOptions{})

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.Info("Creating TcpLoadBalancer", "namespace", desiredTcpLB.Namespace, "name", desiredTcpLB.Name)
		_, err = r.TcpLBClient.Create(desiredTcpLB)

		return ctrl.Result{}, err
	}

	if !kubelb.TcpLoadBalancerIsDesiredState(actualTcpLB, desiredTcpLB) {
		log.Info("Updating TcpLoadBalancer", "namespace", desiredTcpLB.Namespace, "name", desiredTcpLB.Name)
		actualTcpLB.Spec = desiredTcpLB.Spec
		_, err = r.TcpLBClient.Update(actualTcpLB)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KubeLbServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		WithEventFilter(&kubelb.MatchingAnnotationPredicate{
			AnnotationIngressClass:      "kubernetes.io/service.class",
			AnnotationIngressClassValue: "kubelb",
		}).
		Complete(r)
}
