/*
Copyright 2026 The KubeLB Authors.

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
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	"k8c.io/kubelb/internal/metricsutil"
	serviceHelpers "k8c.io/kubelb/internal/resources/service"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SourceMetrics struct {
	ReconcileTotal    *prometheus.CounterVec
	ReconcileDuration *prometheus.HistogramVec
	Managed           *prometheus.GaugeVec
}

// SourceAdapter encapsulates the per-Source-kind variation in the
// Source-to-Route reconcile loop. One adapter per Source kind.
type SourceAdapter[T ctrlclient.Object] interface {
	Kind() string
	// GVK is the "Kind.group" string the LB-cluster Route carries in its
	// kubelb.LabelOriginResourceKind label; used by enqueueRoutes.
	GVK() string
	NewObject() T
	NewList() ctrlclient.ObjectList
	ShouldReconcile(T) bool
	ExtractServices(T) []types.NamespacedName
	// ApplyRouteStatus mutates src in-place with the Route's projected
	// status and reports whether the result differs from src's existing
	// status. The loop owns the GET-mutate-PATCH retry.
	ApplyRouteStatus(src T, raw []byte) (changed bool, err error)
	Metrics() SourceMetrics
}

type SourceReconciler[T ctrlclient.Object] struct {
	Client      ctrlclient.Client
	LBClient    ctrlclient.Client
	ClusterName string
	Log         logr.Logger
	Adapter     SourceAdapter[T]
}

func (r *SourceReconciler[T]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)
	startTime := time.Now()
	m := r.Adapter.Metrics()

	defer func() {
		m.ReconcileDuration.WithLabelValues(req.Namespace).Observe(time.Since(startTime).Seconds())
	}()

	log.Info("Reconciling " + r.Adapter.Kind())

	resource := r.Adapter.NewObject()
	if err := r.Client.Get(ctx, req.NamespacedName, resource); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		m.ReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return reconcile.Result{}, err
	}

	if resource.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(resource, CleanupFinalizer) {
			return r.cleanup(ctx, resource)
		}
		return reconcile.Result{}, nil
	}

	if !r.Adapter.ShouldReconcile(resource) {
		m.ReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSkipped).Inc()
		return reconcile.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(resource, CleanupFinalizer) {
		controllerutil.AddFinalizer(resource, CleanupFinalizer)
		if err := r.Client.Update(ctx, resource); err != nil {
			m.ReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.reconcile(ctx, log, resource); err != nil {
		log.Error(err, "reconciling failed")
		m.ReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return reconcile.Result{}, err
	}

	if count, err := r.countManaged(ctx, req.Namespace); err == nil {
		m.Managed.WithLabelValues(req.Namespace).Set(float64(count))
	}

	m.ReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSuccess).Inc()
	return reconcile.Result{}, nil
}

func (r *SourceReconciler[T]) reconcile(ctx context.Context, log logr.Logger, resource T) error {
	originalServices := r.Adapter.ExtractServices(resource)
	if err := reconcileSourceForRoute(ctx, log, r.Client, r.LBClient, resource, originalServices, r.ClusterName); err != nil {
		return fmt.Errorf("failed to reconcile source for route: %w", err)
	}

	route := kubelbv1alpha1.Route{}
	if err := r.LBClient.Get(ctx, types.NamespacedName{Name: string(resource.GetUID()), Namespace: r.ClusterName}, &route); err != nil {
		return fmt.Errorf("failed to get Route from LB cluster: %w", err)
	}

	if len(route.Status.Resources.Route.GeneratedName) == 0 {
		return nil
	}

	resourceStatus := route.Status.Resources.Route.Status
	jsonData, err := json.Marshal(resourceStatus.Raw)
	if err != nil || string(jsonData) == kubelb.DefaultRouteStatus {
		return nil
	}

	log.V(3).Info("updating "+r.Adapter.Kind()+" status", "name", resource.GetName(), "namespace", resource.GetNamespace())
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Client.Get(ctx, ctrlclient.ObjectKeyFromObject(resource), resource); err != nil {
			return err
		}
		original, ok := resource.DeepCopyObject().(ctrlclient.Object)
		if !ok {
			return fmt.Errorf("failed to deep copy %s as ctrlclient.Object", r.Adapter.Kind())
		}
		changed, err := r.Adapter.ApplyRouteStatus(resource, resourceStatus.Raw)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		return r.Client.Status().Patch(ctx, resource, ctrlclient.MergeFrom(original))
	})
}

func (r *SourceReconciler[T]) cleanup(ctx context.Context, resource T) (ctrl.Result, error) {
	impactedServices := r.Adapter.ExtractServices(resource)
	services := corev1.ServiceList{}
	if err := r.Client.List(ctx, &services, ctrlclient.InNamespace(resource.GetNamespace()), ctrlclient.MatchingLabels{kubelb.LabelManagedBy: kubelb.LabelControllerName}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list services: %w", err)
	}

	for _, service := range services.Items {
		originalName := service.Name
		if service.Labels[kubelb.LabelOriginName] != "" {
			originalName = service.Labels[kubelb.LabelOriginName]
		}
		for _, serviceRef := range impactedServices {
			if serviceRef.Name == originalName && serviceRef.Namespace == service.Namespace {
				if err := r.Client.Delete(ctx, &service); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to delete service: %w", err)
				}
			}
		}
	}

	if err := cleanupRoute(ctx, r.LBClient, string(resource.GetUID()), r.ClusterName); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to cleanup route: %w", err)
	}

	controllerutil.RemoveFinalizer(resource, CleanupFinalizer)
	if err := r.Client.Update(ctx, resource); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}
	return reconcile.Result{}, nil
}

func (r *SourceReconciler[T]) countManaged(ctx context.Context, namespace string) (int, error) {
	list := r.Adapter.NewList()
	if err := r.Client.List(ctx, list, ctrlclient.InNamespace(namespace)); err != nil {
		return 0, err
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, item := range items {
		obj, ok := item.(T)
		if !ok {
			continue
		}
		if r.Adapter.ShouldReconcile(obj) && obj.GetDeletionTimestamp() == nil {
			count++
		}
	}
	return count, nil
}

func (r *SourceReconciler[T]) Predicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(o ctrlclient.Object) bool {
		obj, ok := o.(T)
		if !ok {
			return false
		}
		return r.Adapter.ShouldReconcile(obj)
	})
}

func (r *SourceReconciler[T]) EnqueueForService() handler.MapFunc {
	return func(ctx context.Context, o ctrlclient.Object) []ctrl.Request {
		list := r.Adapter.NewList()
		if err := r.Client.List(ctx, list, ctrlclient.InNamespace(o.GetNamespace())); err != nil {
			return nil
		}
		items, err := meta.ExtractList(list)
		if err != nil {
			return nil
		}
		result := []reconcile.Request{}
		for _, item := range items {
			obj, ok := item.(T)
			if !ok {
				continue
			}
			if !r.Adapter.ShouldReconcile(obj) {
				continue
			}
			for _, serviceRef := range r.Adapter.ExtractServices(obj) {
				if serviceRef.Namespace != o.GetNamespace() {
					continue
				}
				if serviceRef.Name != o.GetName() && fmt.Sprintf(serviceHelpers.NodePortServicePattern, serviceRef.Name) != o.GetName() {
					continue
				}
				result = append(result, reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      obj.GetName(),
					Namespace: obj.GetNamespace(),
				}})
				break
			}
		}
		return result
	}
}
