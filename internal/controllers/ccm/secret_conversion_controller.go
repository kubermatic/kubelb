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

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	predicateutil "k8c.io/kubelb/internal/util/predicate"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	SecretConversionControllerName = "secret-conversion-controller"
)

// SecretConversionReconciler reconciles an Ingress Object
type SecretConversionReconciler struct {
	ctrlclient.Client

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=syncsecrets,verbs=get;list;watch;create;update;patch;delete

func (r *SecretConversionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)

	log.Info("Reconciling Secrets")

	resource := &corev1.Secret{}
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
			log.Error(nil, "Failed to add finalizer for the Secret")
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.Update(ctx, resource); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	err := r.reconcile(ctx, log, resource)
	if err != nil {
		log.Error(err, "reconciling failed")
	}

	return reconcile.Result{}, err
}

func (r *SecretConversionReconciler) reconcile(ctx context.Context, _ logr.Logger, secret *corev1.Secret) error {
	syncSecret := &kubelbv1alpha1.SyncSecret{
		ObjectMeta: metav1.ObjectMeta{Name: secret.Name, Namespace: secret.Namespace},
	}
	syncSecret.Labels = secret.Labels
	if syncSecret.Labels == nil {
		syncSecret.Labels = make(map[string]string)
	}
	syncSecret.Labels[kubelb.LabelOriginNamespace] = secret.Namespace
	syncSecret.Labels[kubelb.LabelOriginName] = secret.Name
	syncSecret.Data = secret.Data
	syncSecret.StringData = secret.StringData
	syncSecret.Immutable = secret.Immutable
	syncSecret.Type = secret.Type
	return CreateOrUpdateSyncSecret(ctx, r.Client, syncSecret)
}

func (r *SecretConversionReconciler) cleanup(ctx context.Context, object *corev1.Secret) (ctrl.Result, error) {
	resource := &kubelbv1alpha1.SyncSecret{
		ObjectMeta: metav1.ObjectMeta{Name: object.Name, Namespace: object.Namespace},
	}

	err := r.Delete(ctx, resource)
	if err != nil && !kerrors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("failed to delete Sync Secret %s against %s: %w", resource.Name, object.Name, err)
	}

	controllerutil.RemoveFinalizer(object, CleanupFinalizer)
	if err := r.Update(ctx, object); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *SecretConversionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(SecretConversionControllerName).
		For(&corev1.Secret{}, builder.WithPredicates(predicateutil.ByLabel(kubelb.LabelManagedBy, kubelb.LabelControllerName))).
		Complete(r)
}
