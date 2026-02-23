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

package ccm

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	"k8c.io/kubelb/internal/metricsutil"
	ccmmetrics "k8c.io/kubelb/internal/metricsutil/ccm"

	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	SyncSecretControllerName = "sync-secret-controller"
)

// SyncSecretReconciler reconciles an Ingress Object
type SyncSecretReconciler struct {
	ctrlclient.Client

	LBClient    ctrlclient.Client
	ClusterName string

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=syncsecrets,verbs=get;list;watch;create;update;patch;delete

func (r *SyncSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)

	log.Info("Reconciling SyncSecret")

	resource := &kubelbv1alpha1.SyncSecret{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Resource is marked for deletion
	if resource.DeletionTimestamp != nil {
		if controllerutil.ContainsFinalizer(resource, CleanupFinalizer) {
			return r.cleanup(ctx, resource)
		}
		// Finalizer doesn't exist so clean up is already done
		return reconcile.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(resource, CleanupFinalizer) {
		if ok := controllerutil.AddFinalizer(resource, CleanupFinalizer); !ok {
			log.Error(nil, "Failed to add finalizer for the SyncSecret")
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.Update(ctx, resource); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	startTime := time.Now()
	err := r.reconcile(ctx, log, resource)

	// Track reconciliation duration
	ccmmetrics.SyncSecretReconcileDuration.WithLabelValues(req.Namespace).Observe(time.Since(startTime).Seconds())

	if err != nil {
		log.Error(err, "reconciling failed")
		ccmmetrics.SyncSecretReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return reconcile.Result{}, err
	}

	ccmmetrics.SyncSecretReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSuccess).Inc()
	return reconcile.Result{}, nil
}

func (r *SyncSecretReconciler) reconcile(ctx context.Context, _ logr.Logger, object *kubelbv1alpha1.SyncSecret) error {
	if object.Labels == nil {
		object.Labels = make(map[string]string)
	}
	object.Labels[kubelb.LabelOriginNamespace] = object.Namespace
	object.Labels[kubelb.LabelOriginName] = object.Name

	object.Namespace = r.ClusterName
	object.Finalizers = []string{}
	object.Name = string(object.UID)
	object.SetUID("") // Reset UID to generate a new UID for the object
	object.SetResourceVersion("")

	return CreateOrUpdateSyncSecret(ctx, r.LBClient, object)
}

func CreateOrUpdateSyncSecret(ctx context.Context, client ctrlclient.Client, obj *kubelbv1alpha1.SyncSecret) error {
	key := ctrlclient.ObjectKey{Namespace: obj.Namespace, Name: obj.Name}
	existingObj := &kubelbv1alpha1.SyncSecret{}
	if err := client.Get(ctx, key, existingObj); err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get SyncSecret: %w", err)
		}
		err := client.Create(ctx, obj)
		if err != nil {
			if kerrors.IsAlreadyExists(err) {
				if getErr := client.Get(ctx, key, existingObj); getErr != nil {
					return fmt.Errorf("failed to get SyncSecret after conflict: %w", getErr)
				}
			} else {
				return fmt.Errorf("failed to create SyncSecret: %w", err)
			}
		} else {
			return nil
		}
	}

	// Update the object if it is different from the existing one.
	if equality.Semantic.DeepEqual(existingObj.Data, obj.Data) &&
		equality.Semantic.DeepEqual(existingObj.StringData, obj.StringData) &&
		equality.Semantic.DeepEqual(existingObj.Type, obj.Type) &&
		equality.Semantic.DeepEqual(existingObj.Labels, obj.Labels) &&
		equality.Semantic.DeepEqual(existingObj.Annotations, obj.Annotations) {
		return nil
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Re-fetch the latest version before updating
		if getErr := client.Get(ctx, key, existingObj); getErr != nil {
			return getErr
		}
		existingObj.Data = obj.Data
		existingObj.StringData = obj.StringData
		existingObj.Type = obj.Type
		existingObj.Labels = obj.Labels
		existingObj.Annotations = obj.Annotations
		return client.Update(ctx, existingObj)
	})
	if err != nil {
		return fmt.Errorf("failed to update SyncSecret: %w", err)
	}
	return nil
}

func (r *SyncSecretReconciler) cleanup(ctx context.Context, object *kubelbv1alpha1.SyncSecret) (ctrl.Result, error) {
	resource := &kubelbv1alpha1.SyncSecret{
		ObjectMeta: metav1.ObjectMeta{Name: string(object.UID), Namespace: r.ClusterName},
	}

	err := r.LBClient.Delete(ctx, resource)
	if err != nil && !kerrors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("failed to delete object %s from LB cluster: %w", object.Name, err)
	}

	controllerutil.RemoveFinalizer(object, CleanupFinalizer)
	if err := r.Update(ctx, object); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *SyncSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(SyncSecretControllerName).
		For(&kubelbv1alpha1.SyncSecret{}).
		Complete(r)
}
