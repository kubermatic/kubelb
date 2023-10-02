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

package ccm

import (
	"context"

	"github.com/go-logr/logr"

	kubelbiov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/pkg/kubelb"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// KubeLBNodeReconciler reconciles a Service object
type KubeLBNodeReconciler struct {
	ctrlclient.Client

	KubeLBClient ctrlclient.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	Endpoints    *kubelb.Endpoints
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=list;get;watch

func (r *KubeLBNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	// patch endpoints
	var lbList kubelbiov1alpha1.LoadBalancerList
	if err = r.KubeLBClient.List(ctx, &lbList); err != nil {
		log.Error(err, "unable to list LoadBalancer")
		return ctrl.Result{}, err
	}

	log.V(6).Info("patching", "LoadBalancers", lbList)

	var endpointAddresses []kubelbiov1alpha1.EndpointAddress
	for _, endpoint := range r.Endpoints.ClusterEndpoints {
		endpointAddresses = append(endpointAddresses, kubelbiov1alpha1.EndpointAddress{
			IP: endpoint,
		})
	}

	for _, lb := range lbList.Items {
		for _, endpoints := range lb.Spec.Endpoints {
			endpoints.Addresses = endpointAddresses
		}

		if err = r.KubeLBClient.Update(ctx, &lb); err != nil {
			log.Error(err, "unable to update", "LoadBalancer", lb.Name)
		}

		log.V(2).Info("updated", "LoadBalancer", lb.Name)
		log.V(7).Info("updated to", "LoadBalancer", lb)
	}

	return ctrl.Result{}, nil
}

func (r *KubeLBNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
