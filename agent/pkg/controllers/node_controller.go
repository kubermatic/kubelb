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

// KubeLbIngressReconciler reconciles a Service object
type KubeLbNodeReconciler struct {
	client.Client
	KlbClient *kubelb.TcpLBClient
	Log       logr.Logger
	Scheme    *runtime.Scheme
	ctx       context.Context
}

// +kubebuilder:rbac:groups="",resources=node,verbs=get;list;watch
func (r *KubeLbNodeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	r.ctx = context.Background()
	log := r.Log.WithValues("cluster", req.NamespacedName)

	var node corev1.Node
	err := r.Get(r.ctx, req.NamespacedName, &node)
	if err != nil {
		log.Error(err, "unable to fetch node")
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
	for _, node := range nodes.Items {
		var internalIp string
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				internalIp = address.Address
			}
		}

		clusterEndpoints = append(clusterEndpoints, internalIp)
	}

	// Todo: update all TcpLB endpoints

	return ctrl.Result{}, nil
}

func (r *KubeLbNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
