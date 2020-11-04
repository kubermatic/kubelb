package controllers

import (
	l4 "k8c.io/kubelb/agent/pkg/l4"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *KubeLbAgentReconciler) ReconcileGlobalLoadBalancer(userService *corev1.Service) error {

	log := r.Log.WithValues("globalloadbalancer", "glb")

	desiredGlb := l4.MapGlobalLoadBalancer(userService, r.ClusterEndpoints, r.ClusterName)

	//err := ctrl.SetControllerReference(userService, desiredGlb, r.Scheme)
	//if err != nil {
	//	log.Error(err, "Unable to set controller reference")
	//	return err
	//}

	actualGlb, err := r.KlbClient.Get(userService.Name, v1.GetOptions{})

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("Creating glb", "namespace", userService.Namespace, "name", userService.Name)
		_, err = r.KlbClient.Create(desiredGlb)
		return err
	}

	if !l4.GlobalLoadBalancerIsDesiredState(actualGlb, desiredGlb) {
		log.Info("Updating glb", "namespace", desiredGlb.Namespace, "name", desiredGlb.Name)
		actualGlb.Spec = desiredGlb.Spec
		_, err = r.KlbClient.Update(desiredGlb)
		return err
	}

	return nil
}
