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
	"reflect"

	"github.com/go-logr/logr"

	"k8c.io/kubelb/internal/kubelb"
	kuberneteshelper "k8c.io/kubelb/internal/kubernetes"
	ingressHelpers "k8c.io/kubelb/internal/resources/ingress"
	serviceHelpers "k8c.io/kubelb/internal/resources/service"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	IngressControllerName = "ingress-controller"
	IngressClassName      = "kubelb"
)

// IngressReconciler reconciles an Ingress Object
type IngressReconciler struct {
	ctrlclient.Client

	LBClient        ctrlclient.Client
	ClusterName     string
	UseIngressClass bool

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch

func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)

	log.Info("Reconciling Ingress")

	resource := &networkingv1.Ingress{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Resource is marked for deletion
	if resource.DeletionTimestamp != nil {
		if kuberneteshelper.HasFinalizer(resource, CleanupFinalizer) {
			return r.cleanup(ctx, resource)
		}
		// Finalizer doesn't exist so clean up is already done
		return reconcile.Result{}, nil
	}

	if !r.shouldReconcile(resource) {
		return reconcile.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !kuberneteshelper.HasFinalizer(resource, CleanupFinalizer) {
		kuberneteshelper.AddFinalizer(resource, CleanupFinalizer)
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

func (r *IngressReconciler) reconcile(ctx context.Context, log logr.Logger, ingress *networkingv1.Ingress) error {
	// We need to traverse the Ingress, find all the services associated with it, create/update the corresponding Route in LB cluster.
	originalServices := ingressHelpers.GetServicesFromIngress(*ingress)
	return reconcileSourceForRoute(ctx, log, r.Client, r.LBClient, ingress, originalServices, nil, r.ClusterName)
}

func (r *IngressReconciler) cleanup(ctx context.Context, ingress *networkingv1.Ingress) (ctrl.Result, error) {
	impactedServices := ingressHelpers.GetServicesFromIngress(*ingress)
	services := corev1.ServiceList{}
	err := r.List(ctx, &services, ctrlclient.InNamespace(ingress.Namespace), ctrlclient.MatchingLabels{kubelb.LabelAppKubernetesManagedBy: kubelb.LabelControllerName})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list services: %w", err)
	}

	// Delete services created by the controller.
	for _, service := range services.Items {
		originalName := service.Name
		if service.Labels[kubelb.LabelOriginName] != "" {
			originalName = service.Labels[kubelb.LabelOriginName]
		}

		for _, serviceRef := range impactedServices {
			if serviceRef.Name == originalName && serviceRef.Namespace == service.Namespace {
				err := r.Delete(ctx, &service)
				if err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to delete service: %w", err)
				}
			}
		}
	}

	// Find the Route in LB cluster and delete it
	err = cleanupRoute(ctx, r.LBClient, string(ingress.UID), r.ClusterName)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to cleanup route: %w", err)
	}

	kuberneteshelper.RemoveFinalizer(ingress, CleanupFinalizer)
	if err := r.Update(ctx, ingress); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return reconcile.Result{}, nil
}

// enqueueIngresses is a handler.MapFunc to be used to enqeue requests for reconciliation
// for Ingresses against the corresponding service.
func (r *IngressReconciler) enqueueIngresses() handler.MapFunc {
	return func(_ context.Context, o ctrlclient.Object) []ctrl.Request {
		result := []reconcile.Request{}
		ingressList := &networkingv1.IngressList{}
		if err := r.List(context.Background(), ingressList, ctrlclient.InNamespace(o.GetNamespace())); err != nil {
			return nil
		}

		for _, ingress := range ingressList.Items {
			if !r.shouldReconcile(&ingress) {
				continue
			}

			services := ingressHelpers.GetServicesFromIngress(ingress)
			for _, serviceRef := range services {
				if (serviceRef.Name == o.GetName() || fmt.Sprintf(serviceHelpers.NodePortServicePattern, serviceRef.Name) == o.GetName()) && serviceRef.Namespace == o.GetNamespace() {
					result = append(result, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      ingress.Name,
							Namespace: ingress.Namespace,
						},
					})
				}
			}
		}
		return result
	}
}

func (r *IngressReconciler) ingressFilter() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if ingress, ok := e.Object.(*networkingv1.Ingress); ok {
				return r.shouldReconcile(ingress)
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if ingress, ok := e.ObjectNew.(*networkingv1.Ingress); ok {
				if !r.shouldReconcile(ingress) {
					return false
				}
				oldIngress, _ := e.ObjectOld.(*networkingv1.Ingress)
				return !reflect.DeepEqual(ingress.Spec, oldIngress.Spec) || !reflect.DeepEqual(ingress.Labels, oldIngress.Labels) ||
					!reflect.DeepEqual(ingress.Annotations, oldIngress.Annotations)
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if ingress, ok := e.Object.(*networkingv1.Ingress); ok {
				return r.shouldReconcile(ingress)
			}
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			if ingress, ok := e.Object.(*networkingv1.Ingress); ok {
				return r.shouldReconcile(ingress)
			}
			return false
		},
	}
}

func (r *IngressReconciler) shouldReconcile(ingress *networkingv1.Ingress) bool {
	if r.UseIngressClass {
		return ingress.Spec.IngressClassName != nil && *ingress.Spec.IngressClassName == IngressClassName
	}
	return true
}

func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}, builder.WithPredicates(r.ingressFilter())).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueIngresses()),
		).
		Complete(r)
}
