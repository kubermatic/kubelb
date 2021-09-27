/*
Copyright 2020 The KubeLB Authors.

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

package manager

import (
	"context"
	"reflect"

	"github.com/Masterminds/semver/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	envoyresource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	envoycp "k8c.io/kubelb/pkg/envoy"
	"k8c.io/kubelb/pkg/kubelb"
)

// TCPLoadBalancerReconciler reconciles a TCPLoadBalancer object
type TCPLoadBalancerReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Cache          cache.Cache
	EnvoyCache     cachev3.SnapshotCache
	EnvoyBootstrap string
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tcploadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tcploadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=list;watch;delete

func (r *TCPLoadBalancerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("reconciling TCPLoadBalancer")

	var tcpLoadBalancer kubelbk8ciov1alpha1.TCPLoadBalancer
	err := r.Get(ctx, req.NamespacedName, &tcpLoadBalancer)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch TCPLoadBalancer")
		}
		log.V(3).Info("TCPLoadBalancer not found")
		r.EnvoyCache.ClearSnapshot(kubelb.NamespacedName(&v1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		}))
		return ctrl.Result{}, nil
	}

	log.V(5).Info("processing", "TCPLoadBalancer", tcpLoadBalancer)

	//Todo: check validation webhook - ports must equal endpoint ports as well
	if len(tcpLoadBalancer.Spec.Endpoints) == 0 {
		log.Error(errors.New("Invalid Spec"), "No Endpoints set")
		return ctrl.Result{}, nil
	}

	err = r.reconcileEnvoySnapshot(ctx, &tcpLoadBalancer)

	if err != nil {
		log.Error(err, "Unable to reconcile envoy snapshot")
		return ctrl.Result{}, err
	}

	err = r.reconcileDeployment(ctx, &tcpLoadBalancer)

	if err != nil {
		log.Error(err, "Unable to reconcile deployment")
		return ctrl.Result{}, err
	}

	err = r.reconcileService(ctx, &tcpLoadBalancer)

	if err != nil {
		log.Error(err, "Unable to reconcile service")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil

}

func (r *TCPLoadBalancerReconciler) reconcileEnvoySnapshot(ctx context.Context, tcpLoadBalancer *kubelbk8ciov1alpha1.TCPLoadBalancer) error {

	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "envoy")
	log.V(2).Info("verify envoy snapshot")

	// Get current snapshot
	actualSnapshot, err := r.EnvoyCache.GetSnapshot(kubelb.NamespacedName(&tcpLoadBalancer.ObjectMeta))
	if err != nil {
		// Add the snapshot to the cache
		initSnapshot := envoycp.MapSnapshot(tcpLoadBalancer, "0.0.1")
		log.Info("init snapshot", "service-node", tcpLoadBalancer.Name, "version", "0.0.1")
		log.V(5).Info("serving", "snapshot", initSnapshot)

		return r.EnvoyCache.SetSnapshot(kubelb.NamespacedName(&tcpLoadBalancer.ObjectMeta), initSnapshot)
	}

	log.V(5).Info("actual", "snapshot", actualSnapshot)

	lastUsedVersion, err := semver.NewVersion(actualSnapshot.GetVersion(envoyresource.ClusterType))
	if err != nil {
		return errors.Wrap(err, "failed to parse version from last snapshot")
	}

	desiredSnapshot := envoycp.MapSnapshot(tcpLoadBalancer, lastUsedVersion.String())
	log.V(5).Info("desired", "snapshot", desiredSnapshot)

	// Generate a new snapshot using the old version to be able to do a DeepEqual comparison
	if reflect.DeepEqual(actualSnapshot, desiredSnapshot) {
		log.V(2).Info("snapshot is in desired state")
		return nil
	}

	newVersion := lastUsedVersion.IncMajor()
	newSnapshot := envoycp.MapSnapshot(tcpLoadBalancer, newVersion.String())

	if err := newSnapshot.Consistent(); err != nil {
		return errors.Wrap(err, "new Envoy config snapshot is not consistent")
	}
	log.Info("updating snapshot", "service-node", tcpLoadBalancer.Name, "version", newVersion.String())

	if err := r.EnvoyCache.SetSnapshot(kubelb.NamespacedName(&tcpLoadBalancer.ObjectMeta), newSnapshot); err != nil {
		return errors.Wrap(err, "failed to set a new Envoy cache snapshot")
	}

	return nil
}

func (r *TCPLoadBalancerReconciler) reconcileDeployment(ctx context.Context, tcpLoadBalancer *kubelbk8ciov1alpha1.TCPLoadBalancer) error {

	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "deployment")
	log.V(2).Info("verify deployment")

	deployment := &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      tcpLoadBalancer.Name,
			Namespace: tcpLoadBalancer.Namespace,
		},
	}
	err := r.Get(ctx, types.NamespacedName{
		Name:      tcpLoadBalancer.Name,
		Namespace: tcpLoadBalancer.Namespace,
	}, deployment)

	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	log.V(5).Info("actual", "deployment", deployment)

	result, err := ctrl.CreateOrUpdate(ctx, r.Client, deployment, func() error {

		var replicas int32 = 1
		var envoyListenerPorts []corev1.ContainerPort

		//use endpoint node port as internal envoy port
		for _, epPort := range tcpLoadBalancer.Spec.Endpoints[0].Ports {
			envoyListenerPorts = append(envoyListenerPorts, corev1.ContainerPort{
				//Name:          lbServicePort.Name,
				ContainerPort: epPort.Port,
			})
		}

		deployment.Labels = map[string]string{kubelb.LabelAppKubernetesName: tcpLoadBalancer.Name, kubelb.LabelAppKubernetesManagedBy: kubelb.LabelControllerName}

		deployment.Spec.Replicas = &replicas
		deployment.Spec.Selector = &v1.LabelSelector{
			MatchLabels: map[string]string{kubelb.LabelAppKubernetesName: tcpLoadBalancer.Name},
		}

		deployment.Spec.Template.ObjectMeta = v1.ObjectMeta{
			Name:      tcpLoadBalancer.Name,
			Namespace: tcpLoadBalancer.Namespace,
			Labels:    map[string]string{kubelb.LabelAppKubernetesName: tcpLoadBalancer.Name, kubelb.LabelAppKubernetesManagedBy: kubelb.LabelControllerName},
		}

		var containers []corev1.Container

		if len(deployment.Spec.Template.Spec.Containers) == 1 {
			currentContainer := deployment.Spec.Template.Spec.Containers[0]

			currentContainer.Name = tcpLoadBalancer.Name
			currentContainer.Image = "envoyproxy/envoy-alpine:v1.16-latest"
			currentContainer.Args = []string{
				"--config-yaml", r.EnvoyBootstrap,
				"--service-node", kubelb.NamespacedName(&tcpLoadBalancer.ObjectMeta),
				"--service-cluster", tcpLoadBalancer.Namespace,
			}
			currentContainer.Ports = envoyListenerPorts

			containers = append(containers, currentContainer)
		} else {
			containers = []corev1.Container{
				{
					Name:  tcpLoadBalancer.Name,
					Image: "envoyproxy/envoy:v1.19.1",
					Args: []string{
						"--config-yaml", r.EnvoyBootstrap,
						"--service-node", kubelb.NamespacedName(&tcpLoadBalancer.ObjectMeta),
						"--service-cluster", tcpLoadBalancer.Namespace,
					},
					Ports: envoyListenerPorts,
				},
			}
		}

		deployment.Spec.Template.Spec.Containers = containers

		return ctrl.SetControllerReference(tcpLoadBalancer, deployment, r.Scheme)

	})

	log.V(5).Info("desired", "deployment", deployment)

	log.V(2).Info("operation fulfilled", "status", result)

	return err

}

func (r *TCPLoadBalancerReconciler) reconcileService(ctx context.Context, tcpLoadBalancer *kubelbk8ciov1alpha1.TCPLoadBalancer) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "service")

	log.V(2).Info("verify service")

	service := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:      tcpLoadBalancer.Name,
			Namespace: tcpLoadBalancer.Namespace,
			Labels:    map[string]string{kubelb.LabelAppKubernetesName: tcpLoadBalancer.Name},
		},
	}
	err := r.Get(ctx, types.NamespacedName{
		Name:      tcpLoadBalancer.Name,
		Namespace: tcpLoadBalancer.Namespace,
	}, service)

	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	log.V(5).Info("actual", "service", service)

	allocatedServicePorts := len(service.Spec.Ports)

	result, err := ctrl.CreateOrUpdate(ctx, r.Client, service, func() error {

		var ports []corev1.ServicePort
		for currentLbPort, lbServicePort := range tcpLoadBalancer.Spec.Ports {

			var allocatedPort corev1.ServicePort

			//Edit existing port
			if currentLbPort < allocatedServicePorts {
				allocatedPort = service.Spec.Ports[currentLbPort]

				allocatedPort.Name = lbServicePort.Name
				allocatedPort.Port = lbServicePort.Port
				allocatedPort.TargetPort = intstr.FromInt(int(tcpLoadBalancer.Spec.Endpoints[0].Ports[currentLbPort].Port))
				allocatedPort.Protocol = lbServicePort.Protocol

			} else {
				allocatedPort = corev1.ServicePort{
					Name:       lbServicePort.Name,
					Port:       lbServicePort.Port,
					TargetPort: intstr.FromInt(int(tcpLoadBalancer.Spec.Endpoints[0].Ports[currentLbPort].Port)),
					Protocol:   lbServicePort.Protocol,
				}
			}
			ports = append(ports, allocatedPort)
		}

		service.Spec.Ports = ports

		service.Spec.Selector = map[string]string{kubelb.LabelAppKubernetesName: tcpLoadBalancer.Name}
		service.Spec.Type = tcpLoadBalancer.Spec.Type

		return ctrl.SetControllerReference(tcpLoadBalancer, service, r.Scheme)

	})

	log.V(5).Info("desired", "service", service)

	log.V(2).Info("operation fulfilled", "status", result)

	if err != nil {
		return err
	}

	//Status changes
	log.V(5).Info("load balancer status", "TcpLoadBalancer", tcpLoadBalancer.Status.LoadBalancer.Ingress, "service", service.Status.LoadBalancer.Ingress)

	if tcpLoadBalancer.Spec.Type != corev1.ServiceTypeLoadBalancer || len(tcpLoadBalancer.Status.LoadBalancer.Ingress) == len(service.Status.LoadBalancer.Ingress) {
		log.V(2).Info("TcpLoadBalancer status is in desired state")
		return nil
	}

	log.V(3).Info("updating TcpLoadBalancer status", "name", tcpLoadBalancer.Name, "namespace", tcpLoadBalancer.Namespace)

	tcpLoadBalancer.Status.LoadBalancer = service.Status.LoadBalancer
	log.V(4).Info("updating to", "TcpLoadBalancer status", tcpLoadBalancer.Status.LoadBalancer)

	return r.Status().Update(ctx, tcpLoadBalancer)

}

func (r *TCPLoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager, ctx context.Context) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&kubelbk8ciov1alpha1.TCPLoadBalancer{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Build(r)

	if err != nil {
		return err
	}

	//Todo: same as owns? can be removed : leave it
	serviceInformer, err := r.Cache.GetInformer(ctx, &corev1.Service{})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Informer{Informer: serviceInformer},
		&handler.EnqueueRequestForOwner{
			OwnerType:    &kubelbk8ciov1alpha1.TCPLoadBalancer{},
			IsController: true,
		},
	)

	return err
}
