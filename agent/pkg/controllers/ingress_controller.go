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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// KubeLbIngressReconciler reconciles a Service object
type KubeLbIngressReconciler struct {
	client.Client
	ClusterCache cache.Cache
	KlbClient    *kubelb.Client
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

	log.Info("Ingress:" + ingress.Name)

	return ctrl.Result{}, nil
}

func (r *KubeLbIngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1beta1.Ingress{}).
		WithEventFilter(&matchingIngressAnnotations{}).
		Complete(r)
}

const (
	AnnotationIngressClass      = "kubernetes.io/ingress.class"
	AnnotationIngressClassValue = "kubelb"
)

var _ predicate.Predicate = &matchingIngressAnnotations{}

type matchingIngressAnnotations struct{}

// Create returns true if the Create event should be processed
func (r *matchingIngressAnnotations) Create(e event.CreateEvent) bool {
	return r.match(e.Meta.GetAnnotations())
}

// Delete returns true if the Delete event should be processed
func (r *matchingIngressAnnotations) Delete(e event.DeleteEvent) bool {
	return r.match(e.Meta.GetAnnotations())
}

// Update returns true if the Update event should be processed
func (r *matchingIngressAnnotations) Update(e event.UpdateEvent) bool {
	return r.match(e.MetaNew.GetAnnotations())
}

// Generic returns true if the Generic event should be processed
func (r *matchingIngressAnnotations) Generic(e event.GenericEvent) bool {
	return r.match(e.Meta.GetAnnotations())
}

func (r *matchingIngressAnnotations) match(annotations map[string]string) bool {
	val, ok := annotations[AnnotationIngressClass]
	return !(ok && val == "") && val == AnnotationIngressClassValue
}
