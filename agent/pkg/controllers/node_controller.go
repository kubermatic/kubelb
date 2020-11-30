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
	kubelbiov1alpha1 "k8c.io/kubelb/manager/pkg/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/manager/pkg/generated/clientset/versioned/typed/kubelb.k8c.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KubeLbIngressReconciler reconciles a Service object
type KubeLbNodeReconciler struct {
	client.Client
	KlbClient v1alpha1.TCPLoadBalancerInterface
	Log       logr.Logger
	Scheme    *runtime.Scheme
	ctx       context.Context
	Endpoints *kubelb.Endpoints
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=list;get;watch

func (r *KubeLbNodeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	r.ctx = context.Background()
	log := r.Log.WithValues("cluster", req.NamespacedName)

	nodeList := &corev1.NodeList{}
	err := r.List(context.Background(), nodeList)

	if err != nil {
		log.Error(err, "unable to list nodeList")
		return ctrl.Result{}, err
	}

	if r.Endpoints.EndpointIsDesiredState(nodeList) {
		return ctrl.Result{}, err
	}

	r.Endpoints.ClusterEndpoints = r.Endpoints.GetEndpoints(nodeList)

	//patch endpoints
	tcpLbList, err := r.KlbClient.List(v1.ListOptions{})

	if err != nil {
		log.Error(err, "unable to list TcpLoadBalancer")
		return ctrl.Result{}, err
	}

	var endpointAddresses []kubelbiov1alpha1.EndpointAddress
	for _, endpoint := range r.Endpoints.ClusterEndpoints {
		endpointAddresses = append(endpointAddresses, kubelbiov1alpha1.EndpointAddress{
			IP: endpoint,
		})
	}

	for _, tcpLb := range tcpLbList.Items {
		for _, endpoints := range tcpLb.Spec.Endpoints {
			endpoints.Addresses = endpointAddresses
		}

		_, err = r.KlbClient.Update(&tcpLb)

		if err != nil {
			log.Error(err, "unable to update TcpLoadBalancer")
		}
	}

	return ctrl.Result{}, err
}

func (r *KubeLbNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
