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
	kubelbiov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/pkg/generated/clientset/versioned/typed/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/pkg/kubelb"
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
	Endpoints *kubelb.Endpoints
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=list;get;watch

func (r *KubeLbNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.Name)

	log.V(2).Info("reconciling node")

	nodeList := &corev1.NodeList{}
	err := r.List(ctx, nodeList)

	if err != nil {
		log.Error(err, "unable to list nodeList")
		return ctrl.Result{}, err
	}

	log.V(6).Info("processing", "nodes", nodeList, "endpoints", r.Endpoints)

	if r.Endpoints.EndpointIsDesiredState(nodeList) {
		log.V(2).Info("endpoints are in desired state")
		return ctrl.Result{}, err
	}

	log.V(6).Info("actual", "endpoints", r.Endpoints.ClusterEndpoints)
	log.V(6).Info("desired", "endpoints", r.Endpoints.GetEndpoints(nodeList))

	r.Endpoints.ClusterEndpoints = r.Endpoints.GetEndpoints(nodeList)
	log.V(5).Info("proceeding with", "endpoints", r.Endpoints.ClusterEndpoints)

	//patch endpoints
	tcpLbList, err := r.KlbClient.List(ctx, v1.ListOptions{})
	if err != nil {
		log.Error(err, "unable to list TcpLoadBalancer")
		return ctrl.Result{}, err
	}

	log.V(6).Info("patching", "TcpLoadBalancers", tcpLbList)

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

		_, err = r.KlbClient.Update(ctx, &tcpLb, v1.UpdateOptions{})
		if err != nil {
			log.Error(err, "unable to update", "TcpLoadBalancer", tcpLb.Name)
		}
		log.V(2).Info("updated", "TcpLoadBalancer", tcpLb.Name)
		log.V(7).Info("updated to", "TcpLoadBalancer", tcpLb)

	}

	return ctrl.Result{}, nil
}

func (r *KubeLbNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
