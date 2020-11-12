package controllers

import (
	kubelbiov1alpha1 "k8c.io/kubelb/manager/pkg/api/globalloadbalancer/v1alpha1"
	"k8c.io/kubelb/manager/pkg/l4"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *GlobalLoadBalancerReconciler) reconcileService(glb *kubelbiov1alpha1.GlobalLoadBalancer) error {
	log := r.Log.WithValues("globalloadbalancer", "l4-svc")

	desiredService := l4.MapService(glb)

	err := ctrl.SetControllerReference(glb, desiredService, r.Scheme)
	if err != nil {
		log.Error(err, "Unable to set controller reference")
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

	if !l4.ServiceIsDesiredState(actualService, desiredService) {
		log.Info("Updating service", "namespace", glb.Namespace, "name", glb.Name)
		actualService.Spec.Ports = desiredService.Spec.Ports

		return r.Update(r.ctx, actualService)
	}

	return nil
}

func (r *GlobalLoadBalancerReconciler) reconcileConfigMap(glb *kubelbiov1alpha1.GlobalLoadBalancer) error {

	log := r.Log.WithValues("globalloadbalancer", "l4-cfg")

	desiredConfigMap := l4.MapConfigmap(glb, r.ClusterName)

	err := ctrl.SetControllerReference(glb, desiredConfigMap, r.Scheme)
	if err != nil {
		log.Error(err, "Unable to set controller reference")
		return err
	}

	actualConfigMap := &corev1.ConfigMap{}
	err = r.Get(r.ctx, types.NamespacedName{
		Name:      glb.Name,
		Namespace: glb.Namespace,
	}, actualConfigMap)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("Creating configmap", "namespace", glb.Namespace, "name", glb.Name)
		return r.Create(r.ctx, desiredConfigMap)
	}

	log.Info("Updating configmap", "namespace", glb.Namespace, "name", glb.Name)
	actualConfigMap.Data = desiredConfigMap.Data

	return r.Update(r.ctx, actualConfigMap)

}

func (r *GlobalLoadBalancerReconciler) reconcileDeployment(glb *kubelbiov1alpha1.GlobalLoadBalancer) error {

	log := r.Log.WithValues("globalloadbalancer", "l4-deployment")

	desiredDeployment := l4.MapDeployment(glb)

	err := ctrl.SetControllerReference(glb, desiredDeployment, r.Scheme)
	if err != nil {
		log.Error(err, "Unable to set controller reference")
		return err
	}

	actualDeployment := &appsv1.Deployment{}
	err = r.Get(r.ctx, types.NamespacedName{
		Name:      glb.Name,
		Namespace: glb.Namespace,
	}, actualDeployment)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("Creating endpoints", "namespace", glb.Namespace, "name", glb.Name)
		return r.Create(r.ctx, desiredDeployment)
	}

	log.Info("Updating endpoints", "namespace", glb.Namespace, "name", glb.Name)
	actualDeployment.Spec = desiredDeployment.Spec

	return r.Update(r.ctx, actualDeployment)

}

func (r *GlobalLoadBalancerReconciler) handleL4(glb *kubelbiov1alpha1.GlobalLoadBalancer) error {

	log := r.Log.WithValues("globalloadbalancer", "l4")

	err := r.reconcileConfigMap(glb)

	if err != nil {
		log.Error(err, "Unable to reconcile service")
		return err
	}

	err = r.reconcileDeployment(glb)

	if err != nil {
		log.Error(err, "Unable to reconcile deployment")
		return err
	}

	err = r.reconcileService(glb)

	if err != nil {
		log.Error(err, "Unable to reconcile service")
		return err
	}

	return nil

}
