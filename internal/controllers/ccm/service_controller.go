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
	"reflect"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	utils "k8c.io/kubelb/internal/controllers"
	"k8c.io/kubelb/internal/kubelb"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	LBFinalizerName       = "kubelb.k8c.io/lb-finalizer"
	LoadBalancerClassName = "kubelb"
)

// KubeLBServiceReconciler reconciles a Service object
type KubeLBServiceReconciler struct {
	ctrlclient.Client

	KubeLBManager        ctrl.Manager
	Log                  logr.Logger
	Scheme               *runtime.Scheme
	ClusterName          string
	CloudController      bool
	UseLoadbalancerClass bool
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

	if !r.shouldReconcile(service) {
		return ctrl.Result{}, nil
	}

	clusterEndpoints, useAddressesReference := r.getEndpoints(&service)

	// examine DeletionTimestamp to determine if object is under deletion
	if !service.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is being deleted
		// our finalizer is present, so lets handle any external dependency
		// if fail to delete the external dependency here, return with error
		// so that it can be retried
		// remove our finalizer from the list and update it.
		// Stop reconciliation as the item is being deleted
		return r.cleanupService(ctx, log, &service)
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

	desiredLB := kubelb.MapLoadBalancer(&service, clusterEndpoints, useAddressesReference, r.ClusterName)
	log.V(6).Info("desired", "LoadBalancer", desiredLB)

	kubelbClient := r.KubeLBManager.GetClient()

	var actualLB kubelbv1alpha1.LoadBalancer

	err = kubelbClient.Get(ctx, ctrlclient.ObjectKeyFromObject(desiredLB), &actualLB)
	log.V(6).Info("actual", "LoadBalancer", actualLB)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.V(1).Info("creating LoadBalancer", "name", desiredLB.Name, "namespace", desiredLB.Namespace)

		return ctrl.Result{}, kubelbClient.Create(ctx, desiredLB)
	}

	log.V(6).Info("load balancer status", "LoadBalancer", actualLB.Status.LoadBalancer.Ingress, "service", service.Status.LoadBalancer.Ingress)

	if service.Spec.Type == corev1.ServiceTypeLoadBalancer && !reflect.DeepEqual(actualLB.Status.LoadBalancer.Ingress, service.Status.LoadBalancer.Ingress) {
		log.V(1).Info("updating service status", "name", desiredLB.Name, "namespace", desiredLB.Namespace)

		key := ctrlclient.ObjectKeyFromObject(&service)
		retErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// fetch the current state of the service
			if err := r.Client.Get(ctx, key, &service); err != nil {
				return err
			}

			service.Status.LoadBalancer = actualLB.Status.LoadBalancer

			// update the status
			return r.Client.Status().Update(ctx, &service)
		})

		if retErr != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update status %s: %w", service.Name, retErr)
		}
	}

	if kubelb.LoadBalancerIsDesiredState(&actualLB, desiredLB) {
		log.V(2).Info("LoadBalancer is in desired state")
		return ctrl.Result{}, nil
	}

	log.V(1).Info("updating LoadBalancer spec", "name", desiredLB.Name, "namespace", desiredLB.Namespace)
	actualLB.Spec = desiredLB.Spec
	actualLB.Annotations = desiredLB.Annotations

	err = kubelbClient.Update(ctx, &actualLB)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, err
}

func (r *KubeLBServiceReconciler) cleanupService(ctx context.Context, log logr.Logger, service *corev1.Service) (reconcile.Result, error) {
	if !utils.ContainsString(service.ObjectMeta.Finalizers, LBFinalizerName) {
		return ctrl.Result{}, nil
	}

	lb := &kubelbv1alpha1.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(service.UID),
			Namespace: r.ClusterName,
		},
	}
	err := r.KubeLBManager.GetClient().Delete(ctx, lb)
	switch {
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	case err != nil:
		return ctrl.Result{}, fmt.Errorf("deleting LoadBalancer: %w", err)
	default:
		// proceed
	}

	log.V(1).Info("deleting Service LoadBalancer finalizer", "name", lb.Name)

	service.ObjectMeta.Finalizers = utils.RemoveString(service.ObjectMeta.Finalizers, LBFinalizerName)
	if err := r.Update(ctx, service); err != nil {
		return ctrl.Result{}, err
	}

	log.V(4).Info("removed finalizer")

	return ctrl.Result{}, nil
}

func (r *KubeLBServiceReconciler) enqueueLoadBalancer() handler.TypedMapFunc[*kubelbv1alpha1.LoadBalancer] {
	return handler.TypedMapFunc[*kubelbv1alpha1.LoadBalancer](func(_ context.Context, lb *kubelbv1alpha1.LoadBalancer) []reconcile.Request {
		if lb.GetNamespace() != r.ClusterName {
			return []reconcile.Request{}
		}

		if lb.Spec.Type != corev1.ServiceTypeLoadBalancer {
			return []reconcile.Request{}
		}

		originalNamespace, ok := lb.GetLabels()[kubelb.LabelOriginNamespace]
		if !ok || originalNamespace == "" {
			r.Log.Error(fmt.Errorf("required label \"%s\" not found", kubelb.LabelOriginNamespace), fmt.Sprintf("failed to queue service for LoadBalancer: %s, could not determine origin namespace", lb.GetName()))

			return []reconcile.Request{}
		}

		originalName, ok := lb.GetLabels()[kubelb.LabelOriginName]
		if !ok || originalName == "" {
			r.Log.Error(fmt.Errorf("required label \"%s\" not found", kubelb.LabelOriginName), fmt.Sprintf("failed to queue service for LoadBalancer: %s, could not determine origin name", lb.GetName()))

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
	})
}

func (r *KubeLBServiceReconciler) getEndpoints(service *corev1.Service) ([]string, bool) {
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
		return nil, true
	}

	return clusterEndpoints, false
}

func (r *KubeLBServiceReconciler) shouldReconcile(svc corev1.Service) bool {
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return false
	}

	if r.UseLoadbalancerClass {
		return svc.Spec.LoadBalancerClass != nil && *svc.Spec.LoadBalancerClass == LoadBalancerClassName
	}
	return true
}

func (r *KubeLBServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		WatchesRawSource(
			source.Kind(r.KubeLBManager.GetCache(), &kubelbv1alpha1.LoadBalancer{},
				handler.TypedEnqueueRequestsFromMapFunc[*kubelbv1alpha1.LoadBalancer](r.enqueueLoadBalancer())),
		).
		Complete(r)
}
