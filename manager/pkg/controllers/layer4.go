package controllers

import (
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubelbiov1alpha1 "manager/pkg/api/v1alpha1"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *GlobalLoadBalancerReconciler) handleL4(glb *kubelbiov1alpha1.GlobalLoadBalancer) error {

	log := r.Log.WithValues("globalloadbalancer", "layer4")

	// Reconcile
	// Service

	desiredService := r.mapService(glb)

	if err := ctrl.SetControllerReference(glb, desiredService, r.Scheme); err != nil {
		log.Error(err, "Unable to SetControllerReference")
		return err
	}

	actualService := &corev1.Service{}
	err := r.Get(r.ctx, types.NamespacedName{
		Name:      desiredService.Name,
		Namespace: desiredService.Namespace,
	}, actualService)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("Creating Service", "namespace", desiredService.Namespace, "name", desiredService.Name)
		err = r.Create(r.ctx, desiredService)
		if err != nil {
			log.Error(err, "Unable to Create Service")
			return err
		}
	} else {
		if !reflect.DeepEqual(desiredService.Spec.Ports, actualService.Spec.Ports) {
			log.Info("Updating Service", "namespace", desiredService.Namespace, "name", desiredService.Name)
			actualService.Spec.Ports = desiredService.Spec.Ports

			if err := r.Update(r.ctx, actualService); err != nil {
				log.Error(err, "Unable to Update Service")
				return err
			}
		}
	}

	// Reconcile
	// Endpoints

	desiredEndpoints := r.mapEndpoints(glb)

	if err := ctrl.SetControllerReference(glb, desiredEndpoints, r.Scheme); err != nil {
		return err
	}

	actualEndpoints := &corev1.Endpoints{}
	err = r.Get(r.ctx, types.NamespacedName{
		Name:      desiredEndpoints.Name,
		Namespace: desiredEndpoints.Namespace,
	}, actualEndpoints)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("Creating Endpoints", "namespace", desiredEndpoints.Namespace, "name", desiredEndpoints.Name)
		err = r.Create(r.ctx, desiredEndpoints)
		if err != nil {
			log.Error(err, "Unable to create Endpoint")
			return err
		}
	} else {
		if !reflect.DeepEqual(desiredEndpoints.Subsets, actualEndpoints.Subsets) {
			log.Info("Updating Endpoints", "namespace", desiredEndpoints.Namespace, "name", desiredEndpoints.Name)
			actualEndpoints.Subsets = desiredEndpoints.Subsets

			if err := r.Update(r.ctx, desiredEndpoints); err != nil {
				log.Error(err, "Unable to update Endpoint")
				return err
			}
		}
	}
	return nil

}

func (r *GlobalLoadBalancerReconciler) mapService(glb *kubelbiov1alpha1.GlobalLoadBalancer) *corev1.Service {

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      glb.Name,
			Namespace: glb.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: glb.Spec.Ports,
			Type:  corev1.ServiceTypeLoadBalancer,
		},
	}
}

func (r *GlobalLoadBalancerReconciler) mapEndpoints(glb *kubelbiov1alpha1.GlobalLoadBalancer) *corev1.Endpoints {

	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      glb.Name,
			Namespace: glb.Namespace,
		},
		Subsets: glb.Spec.Subsets,
	}

}
