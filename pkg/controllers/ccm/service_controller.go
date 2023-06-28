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
	"fmt"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	utils "k8c.io/kubelb/pkg/controllers"
	"k8c.io/kubelb/pkg/kubelb"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const LBFinalizerName = "kubelb.k8c.io/tcplb-finalizer"

// KubeLBServiceReconciler reconciles a Service object
type KubeLBServiceReconciler struct {
	ctrlclient.Client

	KubeLBMananger  ctrl.Manager
	Log             logr.Logger
	Scheme          *runtime.Scheme
	ClusterName     string
	CloudController bool
	Endpoints       *kubelb.Endpoints
}

var AnnotationServiceClassMatcher = &utils.MatchingAnnotationPredicate{
	AnnotationName:  "kubernetes.io/service.class",
	AnnotationValue: "kubelb",
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get;update;patch

func (r *KubeLBServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.Name, "namespace", req.Namespace)
	log.V(2).Info("reconciling service")

	var service corev1.Service

	err := r.Get(ctx, req.NamespacedName, &service)
	if err != nil {
		if ctrlclient.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch service")
		}
		log.V(3).Info("service not found")

		return ctrl.Result{}, nil
	}

	if service.Spec.Type != corev1.ServiceTypeNodePort && service.Spec.Type != corev1.ServiceTypeLoadBalancer ||
		(!r.CloudController || service.Spec.Type == corev1.ServiceTypeNodePort) && !AnnotationServiceClassMatcher.Match(service.GetAnnotations()) {

		return ctrl.Result{}, nil
	}

	clusterEndpoints := r.getEndpoints(&service)

	log.V(6).Info("processing", "service", service)
	log.V(5).Info("proceeding with", "endpoints", clusterEndpoints)

	// examine DeletionTimestamp to determine if object is under deletion
	if !service.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is being deleted
		// our finalizer is present, so lets handle any external dependency
		// if fail to delete the external dependency here, return with error
		// so that it can be retried
		// remove our finalizer from the list and update it.
		// Stop reconciliation as the item is being deleted
		return r.cleanupService(ctx, log, clusterEndpoints, &service)
	}

	// If it does not have our finalizer, then lets add the finalizer and update the object. This is equivalent
	// registering our finalizer.
	if !utils.ContainsString(service.ObjectMeta.Finalizers, LBFinalizerName) {
		service.ObjectMeta.Finalizers = append(service.ObjectMeta.Finalizers, LBFinalizerName)
		log.V(4).Info("setting finalizer")

		if err := r.Update(ctx, &service); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.V(5).Info("proceeding with", "endpoints", clusterEndpoints)

	desiredTcpLB := kubelb.MapLoadBalancer(&service, clusterEndpoints, r.ClusterName)
	log.V(6).Info("desired", "LoadBalancer", desiredTcpLB)

	kubelbClient := r.KubeLBMananger.GetClient()

	var actualTcpLB kubelbk8ciov1alpha1.TCPLoadBalancer

	err = kubelbClient.Get(ctx, ctrlclient.ObjectKeyFromObject(desiredTcpLB), &actualTcpLB)
	log.V(6).Info("actual", "LoadBalancer", actualTcpLB)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.V(1).Info("creating LoadBalancer", "name", desiredTcpLB.Name, "namespace", desiredTcpLB.Namespace)

		return ctrl.Result{}, kubelbClient.Create(ctx, desiredTcpLB)
	}

	log.V(6).Info("load balancer status", "LoadBalancer", actualTcpLB.Status.LoadBalancer.Ingress, "service", service.Status.LoadBalancer.Ingress)

	if service.Spec.Type != corev1.ServiceTypeLoadBalancer || len(actualTcpLB.Status.LoadBalancer.Ingress) == len(service.Status.LoadBalancer.Ingress) {
		log.V(2).Info("service status is in desired state")
	} else {
		log.V(1).Info("updating service status", "name", desiredTcpLB.Name, "namespace", desiredTcpLB.Namespace)
		service.Status.LoadBalancer = actualTcpLB.Status.LoadBalancer
		log.V(7).Info("updating to", "service status", service.Status.LoadBalancer)

		err = r.Client.Status().Update(ctx, &service)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if kubelb.LoadBalancerIsDesiredState(&actualTcpLB, desiredTcpLB) {
		log.V(2).Info("LoadBalancer is in desired state")
		return ctrl.Result{}, nil
	}

	log.V(1).Info("updating LoadBalancer spec", "name", desiredTcpLB.Name, "namespace", desiredTcpLB.Namespace)
	actualTcpLB.Spec = desiredTcpLB.Spec

	err = kubelbClient.Update(ctx, &actualTcpLB)
	if err != nil {
		return ctrl.Result{}, err
	}
	log.V(7).Info("updated to", "LoadBalancer", actualTcpLB)

	return ctrl.Result{}, err
}

func (r *KubeLBServiceReconciler) cleanupService(ctx context.Context, log logr.Logger, clusterEndpoints []string, service *corev1.Service) (reconcile.Result, error) {
	if !utils.ContainsString(service.ObjectMeta.Finalizers, LBFinalizerName) {
		return ctrl.Result{}, nil
	}

	kubelbClient := r.KubeLBMananger.GetClient()
	desiredTcpLB := kubelb.MapLoadBalancer(service, clusterEndpoints, r.ClusterName)
	log.V(1).Info("deleting TCPLoadBalancer", "name", desiredTcpLB)

	err := kubelbClient.Delete(ctx, desiredTcpLB)
	switch {
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	case err != nil:
		return ctrl.Result{}, fmt.Errorf("deleting TCPLoadBalancer: %w", err)
	default:
		// proceed
	}

	log.V(1).Info("deleting Service LoadBalancer finalizer", "name", kubelb.NamespacedName(&service.ObjectMeta))

	service.ObjectMeta.Finalizers = utils.RemoveString(service.ObjectMeta.Finalizers, LBFinalizerName)
	if err := r.Update(ctx, service); err != nil {
		return ctrl.Result{}, err
	}

	log.V(4).Info("removed finalizer")

	return ctrl.Result{}, nil
}

func (r *KubeLBServiceReconciler) enqueueTCPLoadBalancer(a ctrlclient.Object) []reconcile.Request {
	if a.GetNamespace() != r.ClusterName {
		return []reconcile.Request{}
	}

	tcpLb := a.(*kubelbk8ciov1alpha1.TCPLoadBalancer)
	if tcpLb.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return []reconcile.Request{}
	}

	originalNamespace, ok := a.GetLabels()[kubelb.LabelOriginNamespace]
	if !ok || originalNamespace == "" {
		r.Log.Error(fmt.Errorf("required label \"%s\" not found", kubelb.LabelOriginNamespace), fmt.Sprintf("failed to queue service for LoadBalacner: %s, could not determine origin namespace", a.GetName()))

		return []reconcile.Request{}
	}

	originalName, ok := a.GetLabels()[kubelb.LabelOriginName]
	if !ok || originalName == "" {
		r.Log.Error(fmt.Errorf("required label \"%s\" not found", kubelb.LabelOriginName), fmt.Sprintf("failed to queue service for LoadBalacner: %s, could not determine origin name", a.GetName()))

		return []reconcile.Request{}
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      originalName,
				Namespace: originalNamespace,
			},
		},
	}
}

func (r *KubeLBServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctr, err := controller.New("kubelb-ccm", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return fmt.Errorf("failed to create new kubelb-ccm controller: %w", err)
	}

	err = ctr.Watch(
		&source.Kind{Type: &corev1.Service{}},
		&handler.EnqueueRequestForObject{},
	)
	if err != nil {
		return fmt.Errorf("failed to watch for service: %w", err)
	}

	kubeLBWatch := &source.Kind{Type: &kubelbk8ciov1alpha1.TCPLoadBalancer{}}
	if err = kubeLBWatch.InjectCache(r.KubeLBMananger.GetCache()); err != nil {
		return fmt.Errorf("failed to inject cache: %w", err)
	}

	if err = ctr.Watch(kubeLBWatch, handler.EnqueueRequestsFromMapFunc(r.enqueueTCPLoadBalancer)); err != nil {
		return fmt.Errorf("failed to watch for TCPLoadBalancer %w", err)
	}

	return nil
}

func (r *KubeLBServiceReconciler) getEndpoints(service *corev1.Service) []string {
	var clusterEndpoints []string

	// Use LB Endpoint if there is any non KubeLb load balancer implementation
	if service.Spec.Type == corev1.ServiceTypeLoadBalancer && !r.CloudController {
		for _, lbIngress := range service.Status.LoadBalancer.Ingress {
			if lbIngress.IP != "" {
				clusterEndpoints = append(clusterEndpoints, lbIngress.IP)
			} else {
				clusterEndpoints = append(clusterEndpoints, lbIngress.Hostname)
			}
		}
	} else {
		clusterEndpoints = r.Endpoints.ClusterEndpoints
	}

	return clusterEndpoints
}
