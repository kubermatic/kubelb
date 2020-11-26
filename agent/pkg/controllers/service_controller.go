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
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	"k8c.io/kubelb/agent/pkg/kubelb"
	kubelbk8ciov1alpha1 "k8c.io/kubelb/manager/pkg/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/manager/pkg/generated/clientset/versioned/typed/kubelb.k8c.io/v1alpha1"
	kubelbk8ciov1alpha1informers "k8c.io/kubelb/manager/pkg/generated/informers/externalversions/kubelb.k8c.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// KubeLbIngressReconciler reconciles a Service object
type KubeLbServiceReconciler struct {
	client.Client
	TcpLBClient             v1alpha1.TCPLoadBalancerInterface
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	ctx                     context.Context
	ClusterName             string
	CloudController         bool
	Endpoints               *kubelb.Endpoints
	TcpLoadBalancerInformer kubelbk8ciov1alpha1informers.TCPLoadBalancerInformer
}

var ServiceMatcher = &kubelb.MatchingAnnotationPredicate{
	AnnotationName:  "kubernetes.io/service.class",
	AnnotationValue: "kubelb",
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get
func (r *KubeLbServiceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	r.ctx = context.Background()
	log := r.Log.WithValues("cluster", r.ClusterName)

	log.Info("reconciling service", "name", req.Name, "namespace", req.Namespace)

	var service corev1.Service
	err := r.Get(r.ctx, req.NamespacedName, &service)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch service")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var clusterEndpoints []string
	//Use node ports as backend for nodePort service and load balancer if the agent server as cloud controller and should provision the load balacner
	if service.Spec.Type == corev1.ServiceTypeNodePort || (service.Spec.Type == corev1.ServiceTypeLoadBalancer && r.CloudController) {
		clusterEndpoints = r.Endpoints.ClusterEndpoints
	} else if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
		for _, lbIngress := range service.Status.LoadBalancer.Ingress {

			if lbIngress.IP != "" {
				clusterEndpoints = append(clusterEndpoints, lbIngress.IP)
			} else {
				clusterEndpoints = append(clusterEndpoints, lbIngress.Hostname)
			}
		}
	} else {
		err = errors.New("service type not supported")
		log.Error(err, "requires services to be either NodePort or LoadBalancer")
		return ctrl.Result{}, err
	}

	desiredTcpLB := kubelb.MapTcpLoadBalancer(&service, clusterEndpoints, r.ClusterName)

	actualTcpLB, err := r.TcpLBClient.Get(desiredTcpLB.Name, v1.GetOptions{})

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.Info("creating TcpLoadBalancer", "namespace", desiredTcpLB.Namespace, "name", desiredTcpLB.Name)
		_, err = r.TcpLBClient.Create(desiredTcpLB)
		return ctrl.Result{}, err
	}

	if !kubelb.TcpLoadBalancerIsDesiredState(actualTcpLB, desiredTcpLB) {
		log.Info("updating TcpLoadBalancer", "namespace", desiredTcpLB.Namespace, "name", desiredTcpLB.Name)
		actualTcpLB.Spec = desiredTcpLB.Spec
		_, err = r.TcpLBClient.Update(actualTcpLB)

		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if service.Spec.Type == corev1.ServiceTypeLoadBalancer && len(service.Status.LoadBalancer.Ingress) != len(actualTcpLB.Status.LoadBalancer.Ingress) {
		service.Status.LoadBalancer = actualTcpLB.Status.LoadBalancer
		log.Info("updating service status", "namespace", desiredTcpLB.Namespace, "name", desiredTcpLB.Name)
		return ctrl.Result{}, r.Client.Status().Update(r.ctx, &service)
	}

	return ctrl.Result{}, nil
}

func (r *KubeLbServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {

	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		WithEventFilter(ServiceMatcher).
		Build(r)

	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Informer{Informer: r.TcpLoadBalancerInformer.Informer()},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: &TcpLbMapper{ClusterName: r.ClusterName, Log: r.Log}},
	)

	return err

}

var _ handler.Mapper = &TcpLbMapper{}

type TcpLbMapper struct {
	ClusterName string
	Log         logr.Logger
}

// Map enqueues a request per each iems at each node event.
func (sm *TcpLbMapper) Map(m handler.MapObject) []ctrl.Request {

	if m.Meta.GetNamespace() != sm.ClusterName {
		return []ctrl.Request{}
	}

	tcpLb := m.Object.(*kubelbk8ciov1alpha1.TCPLoadBalancer)

	if tcpLb.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return []ctrl.Request{}
	}

	originalNamespace, ok := m.Meta.GetLabels()[kubelb.LabelOriginNamespace]
	if !ok || originalNamespace == "" {
		sm.Log.Error(fmt.Errorf("required label \"%s\" not found", kubelb.LabelOriginNamespace), fmt.Sprintf("failed to queue service for TcpLoadBalacner: %s, could not determine origin namespace", m.Meta.GetName()))
		return []ctrl.Request{}
	}

	originalName, ok := m.Meta.GetLabels()[kubelb.LabelOriginName]
	if !ok || originalName == "" {
		sm.Log.Error(fmt.Errorf("required label \"%s\" not found", kubelb.LabelOriginName), fmt.Sprintf("failed to queue service for TcpLoadBalacner: %s, could not determine origin name", m.Meta.GetName()))
		return []ctrl.Request{}
	}

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      originalName,
				Namespace: originalNamespace,
			},
		},
	}
}