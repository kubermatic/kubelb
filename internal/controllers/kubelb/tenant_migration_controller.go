/*
Copyright 2024 The KubeLB Authors.

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

package kubelb

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	TenantMigrationControllerName = "tenant-migration-controller"
)

// TenantMigrationReconciler is responsible for migrating namespace to a 1:1 tenant mapping.
type TenantMigrationReconciler struct {
	ctrlclient.Client

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

func (r *TenantMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)

	log.Info("Reconciling Namespace")

	resource := &corev1.Namespace{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Resource is marked for deletion
	if resource.DeletionTimestamp != nil {
		// The resource is marked for deletion and migration is not required
		return reconcile.Result{}, nil
	}

	if !r.shouldReconcile(resource) {
		return reconcile.Result{}, nil
	}

	err := r.reconcile(ctx, log, resource)
	if err != nil {
		log.Error(err, "reconciling failed")
	}

	return reconcile.Result{}, err
}

func (r *TenantMigrationReconciler) reconcile(ctx context.Context, _ logr.Logger, namespace *corev1.Namespace) error {
	// We need to create tenant resource corresponding to the namespace. All the other aspects are handled by the tenant controller
	// Remove `tenant-` prefix from namespace name if it exists
	tenantName := RemoveTenantPrefix(namespace.Name)

	// Copy `kubelb.k8c.io/propagate-annotation` from namespace to the tenant resource
	permittedMap := make(map[string]string)
	for k, v := range namespace.GetAnnotations() {
		if strings.HasPrefix(k, kubelbv1alpha1.PropagateAnnotation) {
			filter := strings.SplitN(k, "=", 2)
			if len(filter) <= 1 {
				permittedMap[v] = ""
			} else {
				permittedMap[filter[0]] = filter[1]
			}
		}
	}

	tenant := &kubelbv1alpha1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name: tenantName,
		},
		Spec: kubelbv1alpha1.TenantSpec{
			AnnotationSettings: kubelbv1alpha1.AnnotationSettings{
				PropagatedAnnotations: &permittedMap,
			},
		},
	}

	if err := r.Create(ctx, tenant); err != nil {
		return fmt.Errorf("failed to create tenant: %w", err)
	}

	return nil
}

func (r *TenantMigrationReconciler) resourceFilter() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if obj, ok := e.Object.(*corev1.Namespace); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if obj, ok := e.ObjectNew.(*corev1.Namespace); ok {
				if !r.shouldReconcile(obj) {
					return false
				}
				return e.ObjectOld.GetResourceVersion() != e.ObjectNew.GetResourceVersion()
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if obj, ok := e.Object.(*corev1.Namespace); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			if obj, ok := e.Object.(*corev1.Namespace); ok {
				return r.shouldReconcile(obj)
			}
			return false
		},
	}
}

func (r *TenantMigrationReconciler) shouldReconcile(ns *corev1.Namespace) bool {
	reconcile := true
	if ns.Labels == nil || ns.Labels[kubelb.LabelManagedBy] != kubelb.LabelControllerName {
		reconcile = false
	}

	if ns.OwnerReferences != nil {
		for _, owner := range ns.OwnerReferences {
			if owner.Kind == "Tenant" {
				reconcile = false
				break
			}
		}
	}
	return reconcile
}

func RemoveTenantPrefix(namespace string) string {
	prefix := "tenant-"
	if strings.HasPrefix(namespace, prefix) {
		return strings.TrimPrefix(namespace, prefix)
	}
	return namespace
}

func (r *TenantMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}, builder.WithPredicates(r.resourceFilter())).
		Complete(r)
}
