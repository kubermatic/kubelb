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

	"github.com/go-logr/logr"

	"k8c.io/kubelb/internal/kubelb"
	"k8c.io/kubelb/internal/util/predicate"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	BridgeServiceControllerName = "bridge-service-controller"
)

// BridgeServiceReconciler reconciles the "bridge" service. This service is used to forward traffic from tenant namespace to the controller namespace when global
// topology is used.
type BridgeServiceReconciler struct {
	ctrlclient.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="discovery.k8s.io",resources=endpointslices,verbs=get;list;watch;create;update;patch;delete

func (r *BridgeServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)

	log.Info("Reconciling Service")

	resource := &corev1.Service{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Resource is marked for deletion
	if resource.DeletionTimestamp != nil {
		// Ignore since no cleanup is required. Clean up is done automatically using owner references.
		return reconcile.Result{}, nil
	}

	err := r.reconcile(ctx, log, resource)
	if err != nil {
		log.Error(err, "reconciling failed")
	}

	return reconcile.Result{}, err
}

func (r *BridgeServiceReconciler) reconcile(ctx context.Context, _ logr.Logger, object *corev1.Service) error {
	// Find original service against the bridge service
	name := object.Labels[kubelb.LabelOriginName]
	namespace := object.Labels[kubelb.LabelOriginNamespace]

	originalService := &corev1.Service{}
	err := r.Get(ctx, ctrlclient.ObjectKey{Name: name, Namespace: namespace}, originalService)
	if err != nil {
		return fmt.Errorf("failed to get original service: %w", err)
	}

	endpointSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
	endpointSlice.Labels = make(map[string]string)
	endpointSlice.Labels = kubelb.AddKubeLBLabels(endpointSlice.Labels, name, namespace, "Service")
	endpointSlice.Labels[discoveryv1.LabelServiceName] = name
	endpointSlice.AddressType = discoveryv1.AddressTypeIPv4
	for _, port := range originalService.Spec.Ports {
		endpointSlice.Ports = append(endpointSlice.Ports, discoveryv1.EndpointPort{
			Name:     ptr.To(port.Name),
			Port:     ptr.To(port.TargetPort.IntVal),
			Protocol: ptr.To(port.Protocol),
		})
	}
	endpointSlice.Endpoints = []discoveryv1.Endpoint{
		{
			Addresses: object.Spec.ClusterIPs,
			Conditions: discoveryv1.EndpointConditions{
				Ready: ptr.To(true),
			},
		},
	}

	ownerReference := metav1.OwnerReference{
		APIVersion: originalService.APIVersion,
		Kind:       originalService.Kind,
		Name:       originalService.Name,
		UID:        originalService.UID,
		Controller: ptr.To(true),
	}

	// Set owner reference for the resource.
	endpointSlice.SetOwnerReferences([]metav1.OwnerReference{ownerReference})

	return CreateOrUpdateEndpointSlice(ctx, r.Client, endpointSlice)
}

func CreateOrUpdateEndpointSlice(ctx context.Context, client ctrlclient.Client, obj *discoveryv1.EndpointSlice) error {
	key := ctrlclient.ObjectKey{Namespace: obj.Namespace, Name: obj.Name}
	existing := &discoveryv1.EndpointSlice{}
	if err := client.Get(ctx, key, existing); err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get EndpointSlice: %w", err)
		}
		err := client.Create(ctx, obj)
		if err != nil {
			return fmt.Errorf("failed to create EndpointSlice: %w", err)
		}
		return nil
	}

	// Update the object if it is different from the existing one.
	if equality.Semantic.DeepEqual(existing.Ports, obj.Ports) &&
		equality.Semantic.DeepEqual(existing.Endpoints, obj.Endpoints) &&
		equality.Semantic.DeepEqual(existing.Labels, obj.Labels) {
		return nil
	}

	// Required to update the object.
	obj.ResourceVersion = existing.ResourceVersion
	obj.UID = existing.UID

	if err := client.Update(ctx, obj); err != nil {
		return fmt.Errorf("failed to update EndpointSlice: %w", err)
	}
	return nil
}

func (r *BridgeServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Watch services with label app.kubernetes.io/type=bridge-service
		For(&corev1.Service{}, builder.WithPredicates(predicate.ByLabel(kubelb.LabelAppKubernetesType, kubelb.LabelBridgeService))).
		Complete(r)
}
