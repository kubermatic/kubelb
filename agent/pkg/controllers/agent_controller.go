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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KubeLbAgentReconciler reconciles a Service object
type KubeLbAgentReconciler struct {
	client.Client
	KlbClient        *kubelb.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	ctx              context.Context
	ClusterName      string
	ClusterEndpoints []string
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=node,verbs=get;list;watch

func (r *KubeLbAgentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	r.ctx = context.Background()
	log := r.Log.WithValues("kubelb_agent", req.NamespacedName)

	var service corev1.Service
	err := r.Get(r.ctx, req.NamespacedName, &service)
	if err != nil {
		log.Error(err, "unable to fetch service")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	nodes := &corev1.NodeList{}
	err = r.List(context.Background(), nodes)

	if err != nil {
		log.Error(err, "unable to list nodes")
		return ctrl.Result{}, err

	}

	var clusterEndpoints []string
	//Todo: use env to control which ip should be used
	for _, node := range nodes.Items {
		var internalIp string
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				internalIp = address.Address
			}
		}

		clusterEndpoints = append(clusterEndpoints, internalIp)
	}

	r.ClusterEndpoints = clusterEndpoints

	if service.Spec.Type != corev1.ServiceTypeNodePort {
		return ctrl.Result{}, nil
	}

	//Todo: use well defined annotations
	//check for kubelb.enable: "true" annotation on service object
	if enable, ok := service.ObjectMeta.Annotations["kubelb.enable"]; !ok || enable != "true" {
		return ctrl.Result{}, nil
	}

	log.Info("reconciling service", "name", service.Name, "namespace", service.Namespace)

	err = r.ReconcileGlobalLoadBalancer(&service)

	if err != nil {
		log.Error(err, "Unable to create L4 Load Balancer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KubeLbAgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}
