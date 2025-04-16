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
	"reflect"
	"sort"

	"github.com/go-logr/logr"

	kubelbiov1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// KubeLBNodeReconciler reconciles a Service object
type KubeLBNodeReconciler struct {
	ctrlclient.Client

	KubeLBClient        ctrlclient.Client
	ClusterName         string
	Log                 logr.Logger
	Scheme              *runtime.Scheme
	EndpointAddressType corev1.NodeAddressType
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

	// Compute current state
	currentAddresses := r.GenerateAddresses(nodeList)

	// Retrieve current state from the LB cluster
	var addresses kubelbiov1alpha1.Addresses
	if err = r.KubeLBClient.Get(ctx, types.NamespacedName{Name: kubelbiov1alpha1.DefaultAddressName, Namespace: r.ClusterName}, &addresses); err != nil {
		if kerrors.IsNotFound(err) {
			// Create the default address object
			if err = r.KubeLBClient.Create(ctx, currentAddresses); err != nil {
				log.Error(err, "unable to create addresses")
				return ctrl.Result{}, err
			}
		}
		return reconcile.Result{}, err
	}

	// Compare the current state with the desired state
	if reflect.DeepEqual(addresses.Spec.Addresses, currentAddresses.Spec.Addresses) {
		log.V(2).Info("addresses are in desired state")
		return ctrl.Result{}, nil
	}

	// Update the addresses
	addresses.Spec.Addresses = currentAddresses.Spec.Addresses
	if err = r.KubeLBClient.Update(ctx, &addresses); err != nil {
		log.Error(err, "unable to update addresses")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KubeLBNodeReconciler) GenerateAddresses(nodes *corev1.NodeList) *kubelbiov1alpha1.Addresses {
	endpoints := r.getEndpoints(nodes)
	var addresses []kubelbiov1alpha1.EndpointAddress
	for _, endpoint := range endpoints {
		addresses = append(addresses, kubelbiov1alpha1.EndpointAddress{
			IP: endpoint,
		})
	}

	sort.Slice(addresses, func(i, j int) bool {
		return addresses[i].IP < addresses[j].IP
	})

	return &kubelbiov1alpha1.Addresses{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubelbiov1alpha1.DefaultAddressName,
			Namespace: r.ClusterName,
		},
		Spec: kubelbiov1alpha1.AddressesSpec{
			Addresses: addresses,
		},
	}
}

func (r *KubeLBNodeReconciler) getEndpoints(nodes *corev1.NodeList) []string {
	var clusterEndpoints []string
	for _, node := range nodes.Items {
		var internalIP string
		for _, address := range node.Status.Addresses {
			if address.Type == r.EndpointAddressType {
				internalIP = address.Address
			}
		}
		clusterEndpoints = append(clusterEndpoints, internalIP)
	}
	return clusterEndpoints
}

func (r *KubeLBNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
