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
	"k8c.io/kubelb/agent/pkg/l4"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// KubeLbIngressReconciler reconciles a Service object
type KubeLbServiceReconciler struct {
	client.Client
	KlbClient   *kubelb.Client
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

	var clusterEndpoints []string
	// Todo: use env to control which ip should be used
	for _, node := range nodes.Items {
		var internalIp string
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				internalIp = address.Address
			}
		}
		clusterEndpoints = append(clusterEndpoints, internalIp)
	}

	if service.Spec.Type != corev1.ServiceTypeNodePort {
		return ctrl.Result{}, nil
	}

	log.Info("reconciling service", "name", service.Name, "namespace", service.Namespace)

	desiredGlb := l4.MapGlobalLoadBalancer(&service, clusterEndpoints, r.ClusterName)

	//Todo: not possible because this resource lives in another cluster
	// Probably use finalizers here instead
	//err := ctrl.SetControllerReference(userService, desiredGlb, r.Scheme)
	//if err != nil {
	//	log.Error(err, "Unable to set controller reference")
	//	return err
	//}

	actualGlb, err := r.KlbClient.Get(service.Name, v1.GetOptions{})

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.Info("Creating glb", "namespace", desiredGlb.Namespace, "name", desiredGlb.Name)
		_, err = r.KlbClient.Create(desiredGlb)

		return ctrl.Result{}, err
	}

	if !l4.GlobalLoadBalancerIsDesiredState(actualGlb, desiredGlb) {
		log.Info("Updating glb", "namespace", desiredGlb.Namespace, "name", desiredGlb.Name)
		actualGlb.Spec = desiredGlb.Spec
		_, err = r.KlbClient.Update(actualGlb)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KubeLbServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		//WithEventFilter().
		Complete(r)
}

const (
	AnnotationServiceClass      = "kubernetes.io/service.class"
	AnnotationServiceClassValue = "kubelb"
)

var _ predicate.Predicate = &matchingServiceAnnotations{}

type matchingServiceAnnotations struct{}

// Create returns true if the Create event should be processed
func (r *matchingServiceAnnotations) Create(e event.CreateEvent) bool {
	return r.match(e.Meta.GetAnnotations())
}

// Delete returns true if the Delete event should be processed
func (r *matchingServiceAnnotations) Delete(e event.DeleteEvent) bool {
	return r.match(e.Meta.GetAnnotations())
}

// Update returns true if the Update event should be processed
func (r *matchingServiceAnnotations) Update(e event.UpdateEvent) bool {
	return r.match(e.MetaNew.GetAnnotations())
}

// Generic returns true if the Generic event should be processed
func (r *matchingServiceAnnotations) Generic(e event.GenericEvent) bool {
	return r.match(e.Meta.GetAnnotations())
}

func (r *matchingServiceAnnotations) match(annotations map[string]string) bool {
	val, ok := annotations[AnnotationServiceClass]
	return !(ok && val == "") && val == AnnotationServiceClassValue
}
