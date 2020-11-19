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
	netv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KubeLbIngressReconciler reconciles a Service object
type KubeLbIngressReconciler struct {
	client.Client
	ClusterCache cache.Cache
	KlbClient    *kubelb.HttpLBClient
	Log          logr.Logger
	Scheme       *runtime.Scheme
	ctx          context.Context
	ClusterName  string
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

	/*	var service corev1.Service
		err = r.Get(r.ctx, types.NamespacedName{Name: ingress.Spec.Backend.ServiceName, Namespace: req.Namespace}, &service)

		var clusterEndpoints []string

		switch service.Spec.Type {

		case corev1.ServiceTypeNodePort:
			nodes := &corev1.NodeList{}
			err = r.List(context.Background(), nodes)

			if err != nil {
				log.Error(err, "unable to list nodes")
				return ctrl.Result{}, err
			}
			clusterEndpoints = kubelb.GetEndpoints(nodes, corev1.NodeInternalIP)

		case corev1.ServiceTypeLoadBalancer:

			for _, lbIngress := range service.Status.LoadBalancer.Ingress {

				if lbIngress.IP != "" {
					clusterEndpoints = append(clusterEndpoints, lbIngress.IP)
				} else {
					clusterEndpoints = append(clusterEndpoints, lbIngress.Hostname)
				}
			}

		case corev1.ServiceTypeClusterIP:
			//Todo: copy service and create one of type node port or update existing one
		default:
			log.Info("Unsupported service type")
		}

		desiredGlb := kubelb.MapTcpLoadBalancer(&service, clusterEndpoints, r.ClusterName, v1alpha1.Layer4)

		actualGlb, err := r.TcpLBClient.Get(service.Name, v1.GetOptions{})

		if err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			log.Info("Creating glb", "namespace", desiredGlb.Namespace, "name", desiredGlb.Name)
			_, err = r.TcpLBClient.Create(desiredGlb)

			return ctrl.Result{}, err
		}

		log.Info("Updating glb", "namespace", desiredGlb.Namespace, "name", desiredGlb.Name)
		actualGlb.Spec = desiredGlb.Spec
		_, err = r.TcpLBClient.Update(actualGlb)
	*/

	return ctrl.Result{}, err
}

func (r *KubeLbIngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1beta1.Ingress{}).
		WithEventFilter(&kubelb.MatchingAnnotationPredicate{
			AnnotationIngressClass:      "kubernetes.io/ingress.class",
			AnnotationIngressClassValue: "kubelb",
		}).
		Complete(r)
}
