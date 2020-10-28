package controllers

import (
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	kubelbiov1alpha1 "manager/pkg/api/v1alpha1"
	"manager/pkg/l4"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *GlobalLoadBalancerReconciler) reconcileService(glb *kubelbiov1alpha1.GlobalLoadBalancer) error {
	log := r.Log.WithValues("globalloadbalancer", "l4-svc")

	desiredService := l4.MapService(glb)

	err := ctrl.SetControllerReference(glb, desiredService, r.Scheme)
	if err != nil {
		return err
	}

	actualService := &corev1.Service{}
	err = r.Get(r.ctx, types.NamespacedName{
		Name:      glb.Name,
		Namespace: glb.Namespace,
	}, actualService)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("Creating service", "namespace", glb.Namespace, "name", glb.Name)
		return r.Create(r.ctx, desiredService)
	}

	if err := ctrl.SetControllerReference(glb, desiredService, r.Scheme); err != nil {
		log.Error(err, "Unable to set controller reference")
		return err
	}

	if !l4.ServiceIsDesiredState(actualService, desiredService) {
		log.Info("Updating service", "namespace", desiredService.Namespace, "name", desiredService.Name)
		actualService.Spec.Ports = desiredService.Spec.Ports

		return r.Update(r.ctx, actualService)
	}

	return nil
}

func (r *GlobalLoadBalancerReconciler) reconcileEndpoints(glb *kubelbiov1alpha1.GlobalLoadBalancer) error {

	log := r.Log.WithValues("globalloadbalancer", "l4-endpoints")

	desiredEndpoints := l4.MapEndpoints(glb)

	err := ctrl.SetControllerReference(glb, desiredEndpoints, r.Scheme)
	if err != nil {
		return err
	}

	actualEndpoints := &corev1.Endpoints{}
	err = r.Get(r.ctx, types.NamespacedName{
		Name:      glb.Name,
		Namespace: glb.Namespace,
	}, actualEndpoints)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("Creating endpoints", "namespace", glb.Namespace, "name", glb.Name)
		return r.Create(r.ctx, l4.MapEndpoints(glb))
	}

	if !l4.EndpointsIsDesiredState(actualEndpoints, desiredEndpoints) {
		log.Info("Updating endpoints", "namespace", glb.Namespace, "name", glb.Name)
		actualEndpoints.Subsets = desiredEndpoints.Subsets

		return r.Update(r.ctx, actualEndpoints)
	}

	return nil
}

func (r *GlobalLoadBalancerReconciler) handleL4(glb *kubelbiov1alpha1.GlobalLoadBalancer) error {

	log := r.Log.WithValues("globalloadbalancer", "l4")

	err := r.reconcileService(glb)

	if err != nil {
		log.Error(err, "Unable to reconcile service")
		return err
	}

	err = r.reconcileEndpoints(glb)

	if err != nil {
		log.Error(err, "Unable to reconcile endpoints")
		return err
	}

	return nil

}
