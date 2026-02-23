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
	"bytes"
	"context"
	"fmt"
	"net"
	"reflect"
	"sort"
	"time"

	"github.com/go-logr/logr"

	kubelbiov1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/metricsutil"
	ccmmetrics "k8c.io/kubelb/internal/metricsutil/ccm"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	NodeControllerName = "node-controller"
	requeueAfter       = 10 * time.Second
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
	startTime := time.Now()

	// Track reconciliation duration
	defer func() {
		ccmmetrics.NodeReconcileDuration.WithLabelValues().Observe(time.Since(startTime).Seconds())
	}()

	log.V(2).Info("reconciling node")

	nodeList := &corev1.NodeList{}
	err := r.List(ctx, nodeList)
	if err != nil {
		log.Error(err, "unable to list nodeList")
		ccmmetrics.NodeReconcileTotal.WithLabelValues(metricsutil.ResultError).Inc()
		return ctrl.Result{}, err
	}

	ccmmetrics.NodesTotal.Set(float64(len(nodeList.Items)))

	// Compute current state
	currentAddresses, err := r.GenerateAddresses(nodeList)
	if err != nil {
		log.Error(err, "unable to find a node with an IP address")
		// This is a transient error and happens when the nodes are coming up.
		// We will requeue after a short period of time to give the nodes a chance to be ready and have an IP address.
		ccmmetrics.NodeReconcileTotal.WithLabelValues(metricsutil.ResultError).Inc()
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}

	// Retrieve current state from the LB cluster
	var addresses kubelbiov1alpha1.Addresses
	if err = recordKubeLBOperation("get", func() error {
		return r.KubeLBClient.Get(ctx, types.NamespacedName{Name: kubelbiov1alpha1.DefaultAddressName, Namespace: r.ClusterName}, &addresses)
	}); err != nil {
		if kerrors.IsNotFound(err) {
			// Create the default address object
			if err = recordKubeLBOperation("create", func() error {
				return r.KubeLBClient.Create(ctx, currentAddresses)
			}); err != nil {
				log.Error(err, "unable to create addresses")
				ccmmetrics.NodeReconcileTotal.WithLabelValues(metricsutil.ResultError).Inc()
				return ctrl.Result{}, err
			}
			ccmmetrics.NodeReconcileTotal.WithLabelValues(metricsutil.ResultSuccess).Inc()
		} else {
			ccmmetrics.NodeReconcileTotal.WithLabelValues(metricsutil.ResultError).Inc()
		}
		return reconcile.Result{}, err
	}

	// Compare the current state with the desired state
	if reflect.DeepEqual(addresses.Spec.Addresses, currentAddresses.Spec.Addresses) {
		log.V(2).Info("addresses are in desired state")
		ccmmetrics.NodeReconcileTotal.WithLabelValues(metricsutil.ResultSuccess).Inc()
		return ctrl.Result{}, nil
	}

	// Update the addresses
	addresses.Spec.Addresses = currentAddresses.Spec.Addresses
	if err = recordKubeLBOperation("update", func() error {
		return r.KubeLBClient.Update(ctx, &addresses)
	}); err != nil {
		log.Error(err, "unable to update addresses")
		ccmmetrics.NodeReconcileTotal.WithLabelValues(metricsutil.ResultError).Inc()
		return ctrl.Result{}, err
	}

	ccmmetrics.NodeReconcileTotal.WithLabelValues(metricsutil.ResultSuccess).Inc()
	return ctrl.Result{}, nil
}

func (r *KubeLBNodeReconciler) GenerateAddresses(nodes *corev1.NodeList) (*kubelbiov1alpha1.Addresses, error) {
	endpoints := r.getEndpoints(nodes)
	var addresses []kubelbiov1alpha1.EndpointAddress
	for _, endpoint := range endpoints {
		if net.ParseIP(endpoint) != nil {
			addresses = append(addresses, kubelbiov1alpha1.EndpointAddress{IP: endpoint})
			continue
		}
		// When using NodeHostName, non-IP values are valid hostnames.
		if r.EndpointAddressType == corev1.NodeHostName {
			addresses = append(addresses, kubelbiov1alpha1.EndpointAddress{Hostname: endpoint})
			continue
		}
		r.Log.V(6).Info("skipping invalid IP", "address", endpoint)
	}

	if len(addresses) == 0 {
		return nil, fmt.Errorf("no valid addresses found")
	}

	sort.Slice(addresses, func(i, j int) bool {
		ipI := net.ParseIP(addresses[i].IP)
		ipJ := net.ParseIP(addresses[j].IP)
		if ipI != nil && ipJ != nil {
			return bytes.Compare(ipI, ipJ) < 0
		}
		// IPs before hostnames, then lexicographic for hostnames
		if addresses[i].IP != addresses[j].IP {
			return addresses[i].IP > addresses[j].IP // non-empty IP sorts first
		}
		return addresses[i].Hostname < addresses[j].Hostname
	})

	return &kubelbiov1alpha1.Addresses{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubelbiov1alpha1.DefaultAddressName,
			Namespace: r.ClusterName,
		},
		Spec: kubelbiov1alpha1.AddressesSpec{
			Addresses: addresses,
		},
	}, nil
}

func (r *KubeLBNodeReconciler) getEndpoints(nodes *corev1.NodeList) []string {
	seen := make(map[string]struct{})
	var clusterEndpoints []string
	for _, node := range nodes.Items {
		// Only process nodes that are ready.
		isReady := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				isReady = true
				break
			}
		}
		if !isReady {
			continue
		}

		for _, address := range node.Status.Addresses {
			if address.Type == r.EndpointAddressType {
				if _, ok := seen[address.Address]; !ok {
					seen[address.Address] = struct{}{}
					clusterEndpoints = append(clusterEndpoints, address.Address)
				}
			}
		}
	}
	return clusterEndpoints
}

func (r *KubeLBNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(NodeControllerName).
		For(&corev1.Node{}).
		Complete(r)
}
