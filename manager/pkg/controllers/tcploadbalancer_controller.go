/*


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

package controllers

import (
	"context"
	"k8c.io/kubelb/manager/pkg/l4"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/manager/pkg/api/kubelb.k8c.io/v1alpha1"
)

// TCPLoadBalancerReconciler reconciles a TCPLoadBalancer object
type TCPLoadBalancerReconciler struct {
	client.Client
	Log         logr.Logger
	Scheme      *runtime.Scheme
	ctx         context.Context
	ClusterName string
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tcploadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tcploadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *TCPLoadBalancerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("kubelb.k8c.io", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *TCPLoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubelbk8ciov1alpha1.TCPLoadBalancer{}).
		Complete(r)
}

func (r *TCPLoadBalancerReconciler) reconcileService(glb *kubelbk8ciov1alpha1.TCPLoadBalancer) error {
	log := r.Log.WithValues("TCPLoadBalancer", "l4-svc")

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

func (r *TCPLoadBalancerReconciler) reconcileConfigMap(glb *kubelbk8ciov1alpha1.TCPLoadBalancer) error {

	log := r.Log.WithValues("TCPLoadBalancer", "l4-cfg")

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

func (r *TCPLoadBalancerReconciler) reconcileDeployment(glb *kubelbk8ciov1alpha1.TCPLoadBalancer) error {

	log := r.Log.WithValues("TCPLoadBalancer", "l4-deployment")

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

func (r *TCPLoadBalancerReconciler) handleL4(glb *kubelbk8ciov1alpha1.TCPLoadBalancer) error {

	log := r.Log.WithValues("TCPLoadBalancer", "l4")

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
