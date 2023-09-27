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
	"fmt"
	"reflect"
	"strings"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/pkg/kubelb"
	kuberneteshelper "k8c.io/kubelb/pkg/kubernetes"
	portlookup "k8c.io/kubelb/pkg/port-lookup"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	envoyImage                           = "envoyproxy/envoy:distroless-v1.27.0"
	envoyProxyContainerName              = "envoy-proxy"
	envoyResourcePattern                 = "envoy-%s"
	envoyProxyDeploymentCleanupFinalizer = "kubelb.k8c.io/cleanup-envoy-proxy-deployment"
	envoyGlobalCache                     = "global"
)

type EnvoyProxyTopology string

const (
	EnvoyProxyTopologyShared    EnvoyProxyTopology = "shared"
	EnvoyProxyTopologyDedicated EnvoyProxyTopology = "dedicated"
	EnvoyProxyTopologyGlobal    EnvoyProxyTopology = "global"
)

// LoadBalancerReconciler reconciles a LoadBalancer object
type LoadBalancerReconciler struct {
	ctrlruntimeclient.Client
	Scheme    *runtime.Scheme
	Cache     cache.Cache
	Namespace string

	PortAllocator *portlookup.PortAllocator

	EnvoyBootstrap     string
	EnvoyProxyTopology EnvoyProxyTopology
	EnvoyProxyReplicas int
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tcploadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tcploadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *LoadBalancerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("reconciling LoadBalancer")

	var LoadBalancer kubelbk8ciov1alpha1.TCPLoadBalancer
	err := r.Get(ctx, req.NamespacedName, &LoadBalancer)
	if err != nil {
		if ctrlruntimeclient.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch LoadBalancer")
		}
		log.V(3).Info("LoadBalancer not found")
		return ctrl.Result{}, nil
	}

	log.V(5).Info("processing", "LoadBalancer", LoadBalancer)

	// Todo: check validation webhook - ports must equal endpoint ports as well
	if len(LoadBalancer.Spec.Endpoints) == 0 {
		log.Error(fmt.Errorf("Invalid Spec"), "No Endpoints set")
		return ctrl.Result{}, nil
	}

	// In case of shared envoy proxy topology, we need to fetch all load balancers. Otherwise, we only need to fetch the current one.
	// To keep things generic, we always propagate a list of load balancers here.
	var (
		loadBalancers kubelbk8ciov1alpha1.TCPLoadBalancerList
		appName       string
	)

	switch r.EnvoyProxyTopology {
	case EnvoyProxyTopologyShared:
		err = r.List(ctx, &loadBalancers, ctrlruntimeclient.InNamespace(req.Namespace))
		if err != nil {
			log.Error(err, "unable to fetch LoadBalancer list")
			return ctrl.Result{}, err
		}
		appName = req.Namespace
	case EnvoyProxyTopologyGlobal:
		// List all loadbalancers. We don't care about the namespace here.
		// TODO: ideally we should only process the load balancer that is being reconciled.
		err = r.List(ctx, &loadBalancers)
		if err != nil {
			log.Error(err, "unable to fetch LoadBalancer list")
			return ctrl.Result{}, err
		}
		appName = envoyGlobalCache
	case EnvoyProxyTopologyDedicated:
		loadBalancers.Items = []kubelbk8ciov1alpha1.TCPLoadBalancer{LoadBalancer}
		appName = LoadBalancer.Name
	}

	// Resource is marked for deletion.
	if LoadBalancer.DeletionTimestamp != nil {
		if kuberneteshelper.HasFinalizer(&LoadBalancer, envoyProxyDeploymentCleanupFinalizer) {
			return reconcile.Result{}, r.handleEnvoyProxyCleanup(ctx, LoadBalancer, len(loadBalancers.Items), appName)
		}
		// Finalizer doesn't exist so clean up is already done
		return reconcile.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !kuberneteshelper.HasFinalizer(&LoadBalancer, envoyProxyDeploymentCleanupFinalizer) {
		kuberneteshelper.AddFinalizer(&LoadBalancer, envoyProxyDeploymentCleanupFinalizer)
		if err := r.Update(ctx, &LoadBalancer); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if r.EnvoyProxyTopology == EnvoyProxyTopologyGlobal {
		// For Global topology, we need to ensure that an arbitrary port has been assigned to the endpoint ports of the LoadBalancer.
		if err := r.PortAllocator.AllocatePortsForLoadBalancers(ctx, loadBalancers); err != nil {
			return ctrl.Result{}, err
		}
	}

	snapshotName := envoySnapshotName(r.EnvoyProxyTopology, req)
	err = r.reconcileDeployment(ctx, req.Namespace, appName, snapshotName)
	if err != nil {
		log.Error(err, "Unable to reconcile deployment")
		return ctrl.Result{}, err
	}

	err = r.reconcileService(ctx, &LoadBalancer, appName, r.PortAllocator)
	if err != nil {
		log.Error(err, "Unable to reconcile service")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *LoadBalancerReconciler) reconcileDeployment(ctx context.Context, namespace, appName, snapshotName string) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "deployment")
	log.V(2).Info("verify deployment")

	ns := namespace
	if r.EnvoyProxyTopology == EnvoyProxyTopologyGlobal {
		ns = r.Namespace
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf(envoyResourcePattern, appName),
			Namespace: ns,
		},
	}
	err := r.Get(ctx, types.NamespacedName{
		Name:      fmt.Sprintf(envoyResourcePattern, appName),
		Namespace: ns,
	}, deployment)

	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	log.V(5).Info("actual", "deployment", deployment)

	result, err := ctrl.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		var replicas = int32(r.EnvoyProxyReplicas)
		updateContainer := func(cnt corev1.Container) corev1.Container {
			cnt.Name = envoyProxyContainerName
			cnt.Image = envoyImage
			cnt.Args = []string{
				"--config-yaml", r.EnvoyBootstrap,
				"--service-node", snapshotName,
				"--service-cluster", ns,
			}
			return cnt
		}

		deployment.Spec.Replicas = &replicas
		deployment.Spec.Selector = &v1.LabelSelector{
			MatchLabels: map[string]string{kubelb.LabelAppKubernetesName: appName},
		}

		deployment.Spec.Template.ObjectMeta = v1.ObjectMeta{
			Name:      appName,
			Namespace: ns,
			Labels:    map[string]string{kubelb.LabelAppKubernetesName: appName},
		}

		envoyContainer := updateContainer(corev1.Container{})

		if len(deployment.Spec.Template.Spec.Containers) == 1 {
			currentContainer := deployment.Spec.Template.Spec.Containers[0]
			envoyContainer = updateContainer(currentContainer)
		}

		deployment.Spec.Template.Spec.Containers = []corev1.Container{envoyContainer}
		return nil
	})

	log.V(5).Info("desired", "deployment", deployment)

	log.V(2).Info("operation fulfilled", "status", result)

	return err
}

func (r *LoadBalancerReconciler) reconcileService(ctx context.Context, loadBalancer *kubelbk8ciov1alpha1.TCPLoadBalancer, appName string, portAllocator *portlookup.PortAllocator) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "service")

	log.V(2).Info("verify service")

	svcNamespace := loadBalancer.Namespace
	if r.EnvoyProxyTopology == EnvoyProxyTopologyGlobal {
		svcNamespace = r.Namespace
	}

	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: loadBalancer.Namespace}, ns); err != nil {
		return err
	}

	labels := map[string]string{
		kubelb.LabelAppKubernetesName:     appName,
		kubelb.LabelLoadBalancerName:      loadBalancer.Name,
		kubelb.LabelLoadBalancerNamespace: loadBalancer.Namespace,
	}

	service := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:        fmt.Sprintf(envoyResourcePattern, loadBalancer.Name),
			Namespace:   svcNamespace,
			Labels:      labels,
			Annotations: propagateAnnotations(ns.Annotations, loadBalancer.Annotations),
		},
	}
	err := r.Get(ctx, types.NamespacedName{
		Name:      fmt.Sprintf(envoyResourcePattern, loadBalancer.Name),
		Namespace: svcNamespace,
	}, service)

	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	log.V(5).Info("actual", "service", service)

	allocatedServicePorts := len(service.Spec.Ports)

	result, err := ctrl.CreateOrUpdate(ctx, r.Client, service, func() error {
		var ports []corev1.ServicePort
		for currentLbPort, lbServicePort := range loadBalancer.Spec.Ports {
			var allocatedPort corev1.ServicePort
			targetPort := loadBalancer.Spec.Endpoints[0].Ports[currentLbPort].Port

			// if portAllocator is not nil, it means that envoy topology is set to global.
			if portAllocator != nil {
				endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointPattern, loadBalancer.Namespace, loadBalancer.Name, currentLbPort)
				portKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, targetPort, lbServicePort.Protocol)
				if value, exists := portAllocator.Lookup(endpointKey, portKey); exists {
					targetPort = int32(value)
				}
			}

			// Edit existing port
			if currentLbPort < allocatedServicePorts {
				allocatedPort = service.Spec.Ports[currentLbPort]

				allocatedPort.Name = lbServicePort.Name
				allocatedPort.Port = lbServicePort.Port
				allocatedPort.TargetPort = intstr.FromInt(int(targetPort))
				allocatedPort.Protocol = lbServicePort.Protocol
			} else {
				allocatedPort = corev1.ServicePort{
					Name:       lbServicePort.Name,
					Port:       lbServicePort.Port,
					TargetPort: intstr.FromInt(int(targetPort)),
					Protocol:   lbServicePort.Protocol,
				}
			}
			ports = append(ports, allocatedPort)
		}

		for k, v := range propagateAnnotations(ns.Annotations, loadBalancer.Annotations) {
			service.Annotations[k] = v
		}
		service.Spec.Ports = ports

		service.Spec.Selector = map[string]string{kubelb.LabelAppKubernetesName: appName}
		service.Spec.Type = loadBalancer.Spec.Type
		return nil
	})

	log.V(5).Info("desired", "service", service)

	log.V(2).Info("operation fulfilled", "status", result)

	if err != nil {
		return err
	}

	// Status changes
	log.V(5).Info("load balancer status", "LoadBalancer", loadBalancer.Status.LoadBalancer.Ingress, "service", service.Status.LoadBalancer.Ingress)

	if loadBalancer.Spec.Type != corev1.ServiceTypeLoadBalancer || len(loadBalancer.Status.LoadBalancer.Ingress) == len(service.Status.LoadBalancer.Ingress) {
		log.V(2).Info("LoadBalancer status is in desired state")
		return nil
	}

	log.V(3).Info("updating LoadBalancer status", "name", loadBalancer.Name, "namespace", loadBalancer.Namespace)

	loadBalancer.Status.LoadBalancer = service.Status.LoadBalancer
	log.V(4).Info("updating to", "LoadBalancer status", loadBalancer.Status.LoadBalancer)

	return r.Status().Update(ctx, loadBalancer)
}

func (r *LoadBalancerReconciler) handleEnvoyProxyCleanup(ctx context.Context, lb kubelbk8ciov1alpha1.TCPLoadBalancer, lbCount int, appName string) error {
	log := ctrl.LoggerFrom(ctx).WithValues("cleanup", "LoadBalancer")

	log.V(2).Info("Cleaning up TCP LoadBalancer", "name", lb.Name, "namespace", lb.Namespace)

	// We can delete the envoy proxy deployment if there are no other load balancers.
	if lbCount == 1 {
		deployment := &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:      fmt.Sprintf(envoyResourcePattern, appName),
				Namespace: lb.Namespace,
			},
		}
		log.V(2).Info("Deleting deployment", "name", deployment.Name, "namespace", deployment.Namespace)
		if err := r.Delete(ctx, deployment); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete envoy proxy deployment %s: %v against LoadBalancer %w", deployment.Name, fmt.Sprintf("%s/%s", lb.Name, lb.Namespace), err)
		}
	}

	// Deallocate ports if we are using global envoy proxy topology.
	if r.EnvoyProxyTopology == EnvoyProxyTopologyGlobal {
		if err := r.PortAllocator.DeallocatePortsForLoadBalancer(ctx, lb); err != nil {
			return err
		}
	}

	// Remove corresponding service.
	svcNamespace := lb.Namespace
	if r.EnvoyProxyTopology == EnvoyProxyTopologyGlobal {
		svcNamespace = r.Namespace
	}
	svc := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf(envoyResourcePattern, lb.Name),
			Namespace: svcNamespace,
		},
	}
	log.V(2).Info("Deleting service", "name", svc.Name, "namespace", svc.Namespace)
	if err := r.Delete(ctx, svc); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete service %s: %v against LoadBalancer %w", svc.Name, fmt.Sprintf("%s/%s", lb.Name, lb.Namespace), err)
	}

	// Remove finalizer
	kuberneteshelper.RemoveFinalizer(&lb, envoyProxyDeploymentCleanupFinalizer)

	// Update instance
	err := r.Update(ctx, &lb)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return nil
}

func (r *LoadBalancerReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
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
		handler.EnqueueRequestsFromMapFunc(r.enqueueLoadBalancers()),
		filterServicesPredicate(),
	)
	return err
}

// enqueueLoadBalancers is a handler.MapFunc to be used to enqeue requests for reconciliation
// for TCPLoadBalancers against the corresponding service.
func (r *LoadBalancerReconciler) enqueueLoadBalancers() handler.MapFunc {
	return func(ctx context.Context, o ctrlruntimeclient.Object) []ctrl.Request {
		result := []reconcile.Request{}

		// Find the TCPLoadBalancer that corresponds to this service.
		labels := o.GetLabels()
		if labels == nil {
			return result
		}

		name, ok := labels[kubelb.LabelLoadBalancerName]
		if !ok {
			return result
		}
		namespace, ok := labels[kubelb.LabelLoadBalancerNamespace]
		if !ok {
			return result
		}

		result = append(result, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		})

		return result
	}
}

// filterServicesPredicate filters out services that need to be propagated to the event handlers.
// We only want to handle services that are managed by kubelb and queued against updation events.
func filterServicesPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ensure that this service is managed by kubelb
			newSvc := e.ObjectNew.(*corev1.Service)
			oldSvc := e.ObjectOld.(*corev1.Service)
			labels := newSvc.GetLabels()

			if labels != nil {
				if _, ok := labels[kubelb.LabelLoadBalancerName]; ok {
					// Service is managed by KubeLB.
					// Save reconciliation cost here and only queue up LBs when the status has changed.
					if reflect.DeepEqual(newSvc.Status, oldSvc.Status) {
						return true
					}
				}
			}
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
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
