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

package kubelb

import (
	"context"
	"reflect"
	"strings"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	envoycp "k8c.io/kubelb/pkg/envoy"
	"k8c.io/kubelb/pkg/kubelb"

	"github.com/Masterminds/semver/v3"
	envoycachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
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
)

const EnvoyImage = "envoyproxy/envoy:distroless-v1.26.2"

// LoadBalancerReconciler reconciles a LoadBalancer object
type LoadBalancerReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Cache          cache.Cache
	EnvoyCache     envoycachev3.SnapshotCache
	EnvoyBootstrap string
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tcploadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tcploadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *LoadBalancerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("reconciling LoadBalancer")

	var LoadBalancer kubelbk8ciov1alpha1.TCPLoadBalancer
	err := r.Get(ctx, req.NamespacedName, &LoadBalancer)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch LoadBalancer")
		}
		log.V(3).Info("LoadBalancer not found")
		r.EnvoyCache.ClearSnapshot(req.Name)
		return ctrl.Result{}, nil
	}

	log.V(5).Info("processing", "LoadBalancer", LoadBalancer)

	// Todo: check validation webhook - ports must equal endpoint ports as well
	if len(LoadBalancer.Spec.Endpoints) == 0 {
		log.Error(errors.New("Invalid Spec"), "No Endpoints set")
		return ctrl.Result{}, nil
	}

	err = r.reconcileEnvoySnapshot(ctx, &LoadBalancer)

	if err != nil {
		log.Error(err, "Unable to reconcile envoy snapshot")
		return ctrl.Result{}, err
	}

	err = r.reconcileDeployment(ctx, &LoadBalancer)

	if err != nil {
		log.Error(err, "Unable to reconcile deployment")
		return ctrl.Result{}, err
	}

	err = r.reconcileService(ctx, &LoadBalancer)

	if err != nil {
		log.Error(err, "Unable to reconcile service")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *LoadBalancerReconciler) reconcileEnvoySnapshot(ctx context.Context, LoadBalancer *kubelbk8ciov1alpha1.TCPLoadBalancer) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "envoy")
	log.V(2).Info("verify envoy snapshot")

	// Get current snapshot
	actualSnapshot, err := r.EnvoyCache.GetSnapshot(LoadBalancer.Name)
	if err != nil {
		// Add the snapshot to the cache
		// Todo: check namespace and node-id uniqueness
		initSnapshot, errSnapshor := envoycp.MapSnapshot(LoadBalancer, "0.0.1")
		if errSnapshor != nil {
			return errSnapshor
		}

		log.Info("init snapshot", "service-node", LoadBalancer.Name, "version", "0.0.1")
		log.V(5).Info("serving", "snapshot", initSnapshot)

		return r.EnvoyCache.SetSnapshot(ctx, LoadBalancer.Name, initSnapshot)
	}

	log.V(5).Info("actual", "snapshot", actualSnapshot)

	lastUsedVersion, err := semver.NewVersion(actualSnapshot.GetVersion(envoyresource.ClusterType))
	if err != nil {
		return errors.Wrap(err, "failed to parse version from last snapshot")
	}

	desiredSnapshot, err := envoycp.MapSnapshot(LoadBalancer, lastUsedVersion.String())
	if err != nil {
		return err
	}

	log.V(5).Info("desired", "snapshot", desiredSnapshot)

	// Generate a new snapshot using the old version to be able to do a DeepEqual comparison
	if reflect.DeepEqual(actualSnapshot, desiredSnapshot) {
		log.V(2).Info("snapshot is in desired state")
		return nil
	}

	newVersion := lastUsedVersion.IncMajor()
	newSnapshot, err := envoycp.MapSnapshot(LoadBalancer, newVersion.String())
	if err != nil {
		return err
	}

	if err := newSnapshot.Consistent(); err != nil {
		return errors.Wrap(err, "new Envoy config snapshot is not consistent")
	}

	log.Info("updating snapshot", "service-node", LoadBalancer.Name, "version", newVersion.String())

	if err := r.EnvoyCache.SetSnapshot(ctx, LoadBalancer.Name, newSnapshot); err != nil {
		return errors.Wrap(err, "failed to set a new Envoy cache snapshot")
	}

	return nil
}

func (r *LoadBalancerReconciler) reconcileDeployment(ctx context.Context, LoadBalancer *kubelbk8ciov1alpha1.TCPLoadBalancer) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "deployment")
	log.V(2).Info("verify deployment")

	deployment := &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      LoadBalancer.Name,
			Namespace: LoadBalancer.Namespace,
		},
	}
	err := r.Get(ctx, types.NamespacedName{
		Name:      LoadBalancer.Name,
		Namespace: LoadBalancer.Namespace,
	}, deployment)

	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	log.V(5).Info("actual", "deployment", deployment)

	result, err := ctrl.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		var replicas int32 = 1
		var envoyListenerPorts []corev1.ContainerPort

		// use endpoint node port as internal envoy port
		for _, epPort := range LoadBalancer.Spec.Endpoints[0].Ports {
			envoyListenerPorts = append(envoyListenerPorts, corev1.ContainerPort{
				// Name:          lbServicePort.Name,
				ContainerPort: epPort.Port,
			})
		}

		updateContainer := func(cnt corev1.Container) corev1.Container {
			cnt.Name = LoadBalancer.Name
			cnt.Image = EnvoyImage
			cnt.Args = []string{
				"--config-yaml", r.EnvoyBootstrap,
				"--service-node", LoadBalancer.Name,
				"--service-cluster", LoadBalancer.Namespace,
			}
			cnt.Ports = envoyListenerPorts

			return cnt
		}

		deployment.Spec.Replicas = &replicas
		deployment.Spec.Selector = &v1.LabelSelector{
			MatchLabels: map[string]string{kubelb.LabelAppKubernetesName: LoadBalancer.Name},
		}

		deployment.Spec.Template.ObjectMeta = v1.ObjectMeta{
			Name:      LoadBalancer.Name,
			Namespace: LoadBalancer.Namespace,
			Labels:    map[string]string{kubelb.LabelAppKubernetesName: LoadBalancer.Name},
		}

		envoyContainer := updateContainer(corev1.Container{})

		if len(deployment.Spec.Template.Spec.Containers) == 1 {
			currentContainer := deployment.Spec.Template.Spec.Containers[0]
			envoyContainer = updateContainer(currentContainer)
		}

		deployment.Spec.Template.Spec.Containers = []corev1.Container{envoyContainer}

		return ctrl.SetControllerReference(LoadBalancer, deployment, r.Scheme)
	})

	log.V(5).Info("desired", "deployment", deployment)

	log.V(2).Info("operation fulfilled", "status", result)

	return err
}

func (r *LoadBalancerReconciler) reconcileService(ctx context.Context, LoadBalancer *kubelbk8ciov1alpha1.TCPLoadBalancer) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "service")

	log.V(2).Info("verify service")

	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: LoadBalancer.Namespace}, ns); err != nil {
		return err
	}

	service := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:        LoadBalancer.Name,
			Namespace:   LoadBalancer.Namespace,
			Labels:      map[string]string{kubelb.LabelAppKubernetesName: LoadBalancer.Name},
			Annotations: propagateAnnotations(ns.Annotations, LoadBalancer.Annotations),
		},
	}
	err := r.Get(ctx, types.NamespacedName{
		Name:      LoadBalancer.Name,
		Namespace: LoadBalancer.Namespace,
	}, service)

	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	log.V(5).Info("actual", "service", service)

	allocatedServicePorts := len(service.Spec.Ports)

	result, err := ctrl.CreateOrUpdate(ctx, r.Client, service, func() error {
		var ports []corev1.ServicePort
		for currentLbPort, lbServicePort := range LoadBalancer.Spec.Ports {

			var allocatedPort corev1.ServicePort

			// Edit existing port
			if currentLbPort < allocatedServicePorts {
				allocatedPort = service.Spec.Ports[currentLbPort]

				allocatedPort.Name = lbServicePort.Name
				allocatedPort.Port = lbServicePort.Port
				allocatedPort.TargetPort = intstr.FromInt(int(LoadBalancer.Spec.Endpoints[0].Ports[currentLbPort].Port))
				allocatedPort.Protocol = lbServicePort.Protocol

			} else {
				allocatedPort = corev1.ServicePort{
					Name:       lbServicePort.Name,
					Port:       lbServicePort.Port,
					TargetPort: intstr.FromInt(int(LoadBalancer.Spec.Endpoints[0].Ports[currentLbPort].Port)),
					Protocol:   lbServicePort.Protocol,
				}
			}
			ports = append(ports, allocatedPort)
		}

		for k, v := range propagateAnnotations(ns.Annotations, LoadBalancer.Annotations) {
			service.Annotations[k] = v
		}
		service.Spec.Ports = ports

		service.Spec.Selector = map[string]string{kubelb.LabelAppKubernetesName: LoadBalancer.Name}
		service.Spec.Type = LoadBalancer.Spec.Type

		return ctrl.SetControllerReference(LoadBalancer, service, r.Scheme)
	})

	log.V(5).Info("desired", "service", service)

	log.V(2).Info("operation fulfilled", "status", result)

	if err != nil {
		return err
	}

	// Status changes
	log.V(5).Info("load balancer status", "LoadBalancer", LoadBalancer.Status.LoadBalancer.Ingress, "service", service.Status.LoadBalancer.Ingress)

	if LoadBalancer.Spec.Type != corev1.ServiceTypeLoadBalancer || len(LoadBalancer.Status.LoadBalancer.Ingress) == len(service.Status.LoadBalancer.Ingress) {
		log.V(2).Info("LoadBalancer status is in desired state")
		return nil
	}

	log.V(3).Info("updating LoadBalancer status", "name", LoadBalancer.Name, "namespace", LoadBalancer.Namespace)

	LoadBalancer.Status.LoadBalancer = service.Status.LoadBalancer
	log.V(4).Info("updating to", "LoadBalancer status", LoadBalancer.Status.LoadBalancer)

	return r.Status().Update(ctx, LoadBalancer)
}

func (r *LoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager, ctx context.Context) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&kubelbk8ciov1alpha1.TCPLoadBalancer{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Build(r)
	if err != nil {
		return err
	}

	// Todo: same as owns? can be removed : leave it
	serviceInformer, err := r.Cache.GetInformer(ctx, &corev1.Service{})
	if err != nil {
		return err
	}
	_, err = r.Cache.GetInformer(ctx, &corev1.Namespace{})
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

func propagateAnnotations(permitted map[string]string, loadbalancer map[string]string) map[string]string {
	a := make(map[string]string)
	permittedMap := make(map[string][]string)
	for k, v := range permitted {
		if strings.HasPrefix(k, kubelbk8ciov1alpha1.PropagateAnnotation) {
			filter := strings.SplitN(k, "=", 2)
			if len(filter) <= 1 {
				permittedMap[v] = []string{}
			} else {
				// optional value filter provided
				filterValues := strings.Split(filter[1], ",")
				for i, v := range filterValues {
					filterValues[i] = strings.TrimSpace(v)
				}
				permittedMap[filter[0]] = filterValues
			}
		}
	}
	for k, v := range loadbalancer {
		if valuesFilter, ok := permittedMap[k]; ok {
			if len(valuesFilter) == 0 {
				a[k] = v
			} else {
				for _, vf := range valuesFilter {
					if v == vf {
						a[k] = v
						break
					}
				}
			}
		}
	}
	return a
}
