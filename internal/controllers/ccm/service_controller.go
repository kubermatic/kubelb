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
	"time"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	"k8c.io/kubelb/internal/metricsutil"
	ccmmetrics "k8c.io/kubelb/internal/metricsutil/ccm"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	LBFinalizerName       = "kubelb.k8c.io/lb-finalizer"
	LoadBalancerClassName = "kubelb"
	ServiceControllerName = "service-controller"
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
	startTime := time.Now()

	// Track reconciliation duration
	defer func() {
		ccmmetrics.ServiceReconcileDuration.WithLabelValues(req.Namespace).Observe(time.Since(startTime).Seconds())
	}()

	log.V(2).Info("reconciling service")

	var service corev1.Service

	err := r.Get(ctx, req.NamespacedName, &service)
	if err != nil {
		if ctrlclient.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch service")
			ccmmetrics.ServiceReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		}
		log.V(3).Info("service not found")

		return ctrl.Result{}, nil
	}

	// Resource is marked for deletion
	if service.DeletionTimestamp != nil {
		if controllerutil.ContainsFinalizer(&service, CleanupFinalizer) || controllerutil.ContainsFinalizer(&service, LBFinalizerName) {
			return r.cleanupService(ctx, log, &service)
		}
		// Finalizer doesn't exist so clean up is already done
		return reconcile.Result{}, nil
	}

	if !r.shouldReconcile(service) {
		ccmmetrics.ServiceReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSkipped).Inc()
		return ctrl.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&service, CleanupFinalizer) {
		if ok := controllerutil.AddFinalizer(&service, CleanupFinalizer); !ok {
			log.Error(nil, "Failed to add finalizer for the LB Service")
			return ctrl.Result{Requeue: true}, nil
		}

		// Remove old finalizer since it is not used anymore.
		controllerutil.RemoveFinalizer(&service, LBFinalizerName)

		if err := r.Update(ctx, &service); err != nil {
			ccmmetrics.ServiceReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	clusterEndpoints, useAddressesReference := r.getEndpoints(&service)
	log.V(5).Info("proceeding with", "endpoints", clusterEndpoints)

	desiredLB := kubelb.MapLoadBalancer(&service, clusterEndpoints, useAddressesReference, r.ClusterName)
	log.V(6).Info("desired", "LoadBalancer", desiredLB)

	kubelbClient := r.KubeLBManager.GetClient()

	var actualLB kubelbv1alpha1.LoadBalancer

	err = recordKubeLBOperation("get", func() error {
		return kubelbClient.Get(ctx, ctrlclient.ObjectKeyFromObject(desiredLB), &actualLB)
	})
	log.V(6).Info("actual", "LoadBalancer", actualLB)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			ccmmetrics.ServiceReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
			return ctrl.Result{}, err
		}
		log.V(1).Info("creating LoadBalancer", "name", desiredLB.Name, "namespace", desiredLB.Namespace)

		if err := recordKubeLBOperation("create", func() error {
			return kubelbClient.Create(ctx, desiredLB)
		}); err != nil {
			if kerrors.IsAlreadyExists(err) {
				if getErr := kubelbClient.Get(ctx, ctrlclient.ObjectKeyFromObject(desiredLB), &actualLB); getErr != nil {
					ccmmetrics.ServiceReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
					return ctrl.Result{}, fmt.Errorf("failed to get LoadBalancer after conflict: %w", getErr)
				}
			} else {
				ccmmetrics.ServiceReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
				return ctrl.Result{}, err
			}
		} else {
			r.updateManagedServicesGauge(ctx, req.Namespace)
			ccmmetrics.ServiceReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSuccess).Inc()
			return ctrl.Result{}, nil
		}
	}

	log.V(6).Info("load balancer status", "LoadBalancer", actualLB.Status.LoadBalancer.Ingress, "service", service.Status.LoadBalancer.Ingress)

	if service.Spec.Type == corev1.ServiceTypeLoadBalancer && !reflect.DeepEqual(actualLB.Status.LoadBalancer.Ingress, service.Status.LoadBalancer.Ingress) {
		log.V(1).Info("updating service status", "name", desiredLB.Name, "namespace", desiredLB.Namespace)

		key := ctrlclient.ObjectKeyFromObject(&service)
		retErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// fetch the current state of the service
			if err := r.Get(ctx, key, &service); err != nil {
				return err
			}

			service.Status.LoadBalancer = actualLB.Status.LoadBalancer

			// update the status
			return r.Client.Status().Update(ctx, &service)
		})

		if retErr != nil {
			ccmmetrics.ServiceReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
			return ctrl.Result{}, fmt.Errorf("failed to update status %s: %w", service.Name, retErr)
		}
	}

	if kubelb.LoadBalancerIsDesiredState(&actualLB, desiredLB) {
		log.V(2).Info("LoadBalancer is in desired state")
		r.updateManagedServicesGauge(ctx, req.Namespace)
		ccmmetrics.ServiceReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSuccess).Inc()
		return ctrl.Result{}, nil
	}

	log.V(1).Info("updating LoadBalancer spec", "name", desiredLB.Name, "namespace", desiredLB.Namespace)

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Re-fetch the latest version before updating
		if getErr := kubelbClient.Get(ctx, ctrlclient.ObjectKeyFromObject(desiredLB), &actualLB); getErr != nil {
			return getErr
		}
		actualLB.Spec = desiredLB.Spec
		actualLB.Annotations = desiredLB.Annotations
		return kubelbClient.Update(ctx, &actualLB)
	})
	if err != nil {
		ccmmetrics.ServiceReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return ctrl.Result{}, err
	}

	r.updateManagedServicesGauge(ctx, req.Namespace)
	ccmmetrics.ServiceReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSuccess).Inc()
	return ctrl.Result{}, nil
}

func (r *KubeLBServiceReconciler) updateManagedServicesGauge(ctx context.Context, namespace string) {
	serviceList := &corev1.ServiceList{}
	if err := r.List(ctx, serviceList, ctrlclient.InNamespace(namespace)); err == nil {
		count := 0
		for _, svc := range serviceList.Items {
			if r.shouldReconcile(svc) && svc.DeletionTimestamp == nil {
				count++
			}
		}
		ccmmetrics.ManagedServicesTotal.WithLabelValues(namespace).Set(float64(count))
	}
}

func (r *KubeLBServiceReconciler) cleanupService(ctx context.Context, log logr.Logger, service *corev1.Service) (reconcile.Result, error) {
	lb := &kubelbv1alpha1.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(service.UID),
			Namespace: r.ClusterName,
		},
	}
	err := recordKubeLBOperation("delete", func() error {
		return r.KubeLBManager.GetClient().Delete(ctx, lb)
	})
	if err != nil && !kerrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("deleting LoadBalancer: %w", err)
	}

	log.V(1).Info("deleting Service LoadBalancer finalizer", "name", lb.Name)

	controllerutil.RemoveFinalizer(service, LBFinalizerName)
	controllerutil.RemoveFinalizer(service, CleanupFinalizer)
	if err := r.Update(ctx, service); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}
	log.V(4).Info("removed finalizer")

	return ctrl.Result{}, nil
}

func (r *KubeLBServiceReconciler) enqueueLoadBalancer() handler.TypedMapFunc[*kubelbv1alpha1.LoadBalancer, reconcile.Request] {
	return handler.TypedMapFunc[*kubelbv1alpha1.LoadBalancer, reconcile.Request](func(_ context.Context, lb *kubelbv1alpha1.LoadBalancer) []reconcile.Request {
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

func (r *KubeLBServiceReconciler) getEndpoints(service *corev1.Service) ([]kubelbv1alpha1.EndpointAddress, bool) {
	var clusterEndpoints []kubelbv1alpha1.EndpointAddress

	// Use LB Endpoint if there is any non KubeLb load balancer implementation
	if service.Spec.Type == corev1.ServiceTypeLoadBalancer && !r.CloudController {
		for _, lbIngress := range service.Status.LoadBalancer.Ingress {
			addr := kubelbv1alpha1.EndpointAddress{}
			if lbIngress.IP != "" {
				addr.IP = lbIngress.IP
			} else {
				addr.Hostname = lbIngress.Hostname
			}
			clusterEndpoints = append(clusterEndpoints, addr)
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
		Named(ServiceControllerName).
		For(&corev1.Service{}).
		WatchesRawSource(
			source.Kind(r.KubeLBManager.GetCache(), &kubelbv1alpha1.LoadBalancer{},
				handler.TypedEnqueueRequestsFromMapFunc[*kubelbv1alpha1.LoadBalancer](r.enqueueLoadBalancer())),
		).
		Complete(r)
}
