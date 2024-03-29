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
	"k8c.io/kubelb/pkg/config"
	utils "k8c.io/kubelb/pkg/controllers"
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
	"k8s.io/client-go/util/retry"
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
	envoyImage                        = "envoyproxy/envoy:distroless-v1.27.0"
	envoyProxyContainerName           = "envoy-proxy"
	envoyResourcePattern              = "envoy-%s"
	envoyGlobalTopologyServicePattern = "envoy-%s-%s"
	envoyProxyCleanupFinalizer        = "kubelb.k8c.io/cleanup-envoy-proxy"
	envoyGlobalCache                  = "global"
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
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=loadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=loadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=configs,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=configs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=daemonsets,verbs=get;list;watch;create;update;patch;delete

func (r *LoadBalancerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("reconciling LoadBalancer")

	var loadBalancer kubelbk8ciov1alpha1.LoadBalancer
	err := r.Get(ctx, req.NamespacedName, &loadBalancer)
	if err != nil {
		if ctrlruntimeclient.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch LoadBalancer")
		}
		log.V(3).Info("LoadBalancer not found")
		return ctrl.Result{}, nil
	}

	log.V(5).Info("processing", "LoadBalancer", loadBalancer)

	// Todo: check validation webhook - ports must equal endpoint ports as well
	if len(loadBalancer.Spec.Endpoints) == 0 {
		log.Error(fmt.Errorf("Invalid Spec"), "No Endpoints set")
		return ctrl.Result{}, nil
	}

	// In case of shared envoy proxy topology, we need to fetch all load balancers. Otherwise, we only need to fetch the current one.
	// To keep things generic, we always propagate a list of load balancers here.
	var (
		loadBalancers     kubelbk8ciov1alpha1.LoadBalancerList
		appName           string
		resourceNamespace string
	)

	switch r.EnvoyProxyTopology {
	case EnvoyProxyTopologyShared:
		err = r.List(ctx, &loadBalancers, ctrlruntimeclient.InNamespace(req.Namespace))
		if err != nil {
			log.Error(err, "unable to fetch LoadBalancer list")
			return ctrl.Result{}, err
		}
		appName = req.Namespace
		resourceNamespace = req.Namespace
	case EnvoyProxyTopologyGlobal:
		// List all loadbalancers. We don't care about the namespace here.
		// TODO: ideally we should only process the load balancer that is being reconciled.
		err = r.List(ctx, &loadBalancers)
		if err != nil {
			log.Error(err, "unable to fetch LoadBalancer list")
			return ctrl.Result{}, err
		}
		appName = envoyGlobalCache
		resourceNamespace = r.Namespace
	case EnvoyProxyTopologyDedicated:
		loadBalancers.Items = []kubelbk8ciov1alpha1.LoadBalancer{loadBalancer}
		appName = loadBalancer.Name
		resourceNamespace = req.Namespace
	}

	// Resource is marked for deletion.
	if loadBalancer.DeletionTimestamp != nil {
		if kuberneteshelper.HasFinalizer(&loadBalancer, envoyProxyCleanupFinalizer) {
			return reconcile.Result{}, r.handleEnvoyProxyCleanup(ctx, loadBalancer, len(loadBalancers.Items), appName, resourceNamespace)
		}
		// Finalizer doesn't exist so clean up is already done
		return reconcile.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !kuberneteshelper.HasFinalizer(&loadBalancer, envoyProxyCleanupFinalizer) {
		kuberneteshelper.AddFinalizer(&loadBalancer, envoyProxyCleanupFinalizer)
		if err := r.Update(ctx, &loadBalancer); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if r.EnvoyProxyTopology == EnvoyProxyTopologyGlobal {
		// For Global topology, we need to ensure that an arbitrary port has been assigned to the endpoint ports of the LoadBalancer.
		if err := r.PortAllocator.AllocatePortsForLoadBalancers(loadBalancers); err != nil {
			return ctrl.Result{}, err
		}
	}

	snapshotName := envoySnapshotName(r.EnvoyProxyTopology, req)
	err = r.reconcileEnvoyProxy(ctx, resourceNamespace, appName, snapshotName)
	if err != nil {
		log.Error(err, "Unable to reconcile envoy proxy")
		return ctrl.Result{}, err
	}

	err = r.reconcileService(ctx, &loadBalancer, appName, resourceNamespace, r.PortAllocator)
	if err != nil {
		log.Error(err, "Unable to reconcile service")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *LoadBalancerReconciler) reconcileEnvoyProxy(ctx context.Context, namespace, appName, snapshotName string) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "envoy-proxy")
	log.V(2).Info("verify envoy-proxy")

	var envoyProxy ctrlruntimeclient.Object
	objMeta := v1.ObjectMeta{
		Name:      fmt.Sprintf(envoyResourcePattern, appName),
		Namespace: namespace,
	}
	if config.GetConfig().Spec.EnvoyProxy.UseDaemonset {
		envoyProxy = &appsv1.DaemonSet{
			ObjectMeta: objMeta,
		}
	} else {
		envoyProxy = &appsv1.Deployment{
			ObjectMeta: objMeta,
		}
	}

	err := r.Get(ctx, types.NamespacedName{
		Name:      fmt.Sprintf(envoyResourcePattern, appName),
		Namespace: namespace,
	}, envoyProxy)

	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	original := envoyProxy.DeepCopyObject()
	if config.GetConfig().Spec.EnvoyProxy.UseDaemonset {
		daemonset := envoyProxy.(*appsv1.DaemonSet)
		daemonset.Spec.Selector = &v1.LabelSelector{
			MatchLabels: map[string]string{kubelb.LabelAppKubernetesName: appName},
		}
		daemonset.Spec.Template = r.getEnvoyProxyPodSpec(namespace, appName, snapshotName)
		envoyProxy = daemonset
	} else {
		deployment := envoyProxy.(*appsv1.Deployment)
		var replicas = config.GetConfig().Spec.EnvoyProxy.Replicas
		deployment.Spec.Replicas = &replicas
		deployment.Spec.Selector = &v1.LabelSelector{
			MatchLabels: map[string]string{kubelb.LabelAppKubernetesName: appName},
		}
		deployment.Spec.Template = r.getEnvoyProxyPodSpec(namespace, appName, snapshotName)
		envoyProxy = deployment
	}

	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, envoyProxy); err != nil {
			return err
		}
	} else {
		if !reflect.DeepEqual(original, envoyProxy) {
			envoyProxy.SetManagedFields([]v1.ManagedFieldsEntry{})
			envoyProxy.SetResourceVersion("")
			if err := r.Patch(ctx, envoyProxy, ctrlruntimeclient.Apply, ctrlruntimeclient.ForceOwnership, ctrlruntimeclient.FieldOwner("kubelb")); err != nil {
				return err
			}
		}
	}
	log.V(5).Info("desired", "envoy-proxy", envoyProxy)

	return nil
}

func (r *LoadBalancerReconciler) reconcileService(ctx context.Context, loadBalancer *kubelbk8ciov1alpha1.LoadBalancer, appName, namespace string, portAllocator *portlookup.PortAllocator) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "service")

	log.V(2).Info("verify service")

	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: loadBalancer.Namespace}, ns); err != nil {
		return err
	}

	labels := map[string]string{
		kubelb.LabelAppKubernetesName: appName,
		// This helps us to identify the LoadBalancer that this service belongs to.
		kubelb.LabelLoadBalancerName:      loadBalancer.Name,
		kubelb.LabelLoadBalancerNamespace: loadBalancer.Namespace,
		// This helps us to identify the origin of the service.
		kubelb.LabelOriginNamespace: loadBalancer.Labels[kubelb.LabelOriginNamespace],
		kubelb.LabelOriginName:      loadBalancer.Labels[kubelb.LabelOriginName],
	}

	svcName := fmt.Sprintf(envoyResourcePattern, loadBalancer.Name)
	if r.EnvoyProxyTopology == EnvoyProxyTopologyGlobal {
		svcName = fmt.Sprintf(envoyGlobalTopologyServicePattern, loadBalancer.Namespace, loadBalancer.Name)
	}

	service := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:        svcName,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: propagateAnnotations(ns.Annotations, loadBalancer.Annotations),
		},
	}
	err := r.Get(ctx, types.NamespacedName{
		Name:      svcName,
		Namespace: namespace,
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

			if r.EnvoyProxyTopology == EnvoyProxyTopologyGlobal {
				endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointPattern, loadBalancer.Namespace, loadBalancer.Name, 0)
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

	updatedPorts := []kubelbk8ciov1alpha1.ServicePort{}
	for i, port := range service.Spec.Ports {
		targetPort := loadBalancer.Spec.Endpoints[0].Ports[i].Port
		updatedPorts = append(updatedPorts, kubelbk8ciov1alpha1.ServicePort{
			ServicePort: port,
			// In case of global topology, this will be different from the targetPort. Otherwise it will be the same.
			UpstreamTargetPort: targetPort,
		})
	}

	// Update status if needed
	updateStatus := false
	updatedLoadBalanacerStatus := kubelbk8ciov1alpha1.LoadBalancerStatus{
		Service: kubelbk8ciov1alpha1.ServiceStatus{
			Ports: updatedPorts,
		},
		LoadBalancer: service.Status.LoadBalancer,
	}

	if !reflect.DeepEqual(loadBalancer.Status.Service.Ports, updatedPorts) {
		updateStatus = true
	}

	if loadBalancer.Spec.Type == corev1.ServiceTypeLoadBalancer {
		if !reflect.DeepEqual(loadBalancer.Status.LoadBalancer.Ingress, service.Status.LoadBalancer.Ingress) {
			updateStatus = true
		}
	}

	if !updateStatus {
		log.V(3).Info("LoadBalancer status is in desired state")
		return nil
	}

	log.V(3).Info("updating LoadBalancer status", "name", loadBalancer.Name, "namespace", loadBalancer.Namespace)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		lb := &kubelbk8ciov1alpha1.LoadBalancer{}
		if err := r.Get(ctx, types.NamespacedName{Name: loadBalancer.Name, Namespace: loadBalancer.Namespace}, lb); err != nil {
			return err
		}
		original := lb.DeepCopy()
		lb.Status = updatedLoadBalanacerStatus
		if reflect.DeepEqual(original.Status, lb.Status) {
			return nil
		}
		// update the status
		return r.Status().Patch(ctx, lb, ctrlruntimeclient.MergeFrom(original))
	})
}

func (r *LoadBalancerReconciler) handleEnvoyProxyCleanup(ctx context.Context, lb kubelbk8ciov1alpha1.LoadBalancer, lbCount int, appName, resourceNamespace string) error {
	log := ctrl.LoggerFrom(ctx).WithValues("cleanup", "LoadBalancer")

	log.V(2).Info("Cleaning up LoadBalancer", "name", lb.Name, "namespace", lb.Namespace)

	// We can delete the envoy proxy deployment if there are no other load balancers.
	if lbCount == 1 {
		objMeta := v1.ObjectMeta{
			Name:      fmt.Sprintf(envoyResourcePattern, appName),
			Namespace: resourceNamespace,
		}
		var envoyProxy ctrlruntimeclient.Object
		if config.GetConfig().Spec.EnvoyProxy.UseDaemonset {
			envoyProxy = &appsv1.DaemonSet{
				ObjectMeta: objMeta,
			}
		} else {
			envoyProxy = &appsv1.Deployment{
				ObjectMeta: objMeta,
			}
		}

		log.V(2).Info("Deleting envoy proxy", "name", envoyProxy.GetName(), "namespace", envoyProxy.GetNamespace())
		if err := r.Delete(ctx, envoyProxy); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete envoy proxy %s: %v against LoadBalancer %w", envoyProxy.GetName(), fmt.Sprintf("%s/%s", lb.Name, lb.Namespace), err)
		}
	}

	// Deallocate ports if we are using global envoy proxy topology.
	if r.EnvoyProxyTopology == EnvoyProxyTopologyGlobal {
		if err := r.PortAllocator.DeallocatePortsForLoadBalancer(lb); err != nil {
			return err
		}
	}

	// Remove corresponding service.
	svcName := fmt.Sprintf(envoyResourcePattern, lb.Name)
	if r.EnvoyProxyTopology == EnvoyProxyTopologyGlobal {
		svcName = fmt.Sprintf(envoyGlobalTopologyServicePattern, lb.Namespace, lb.Name)
	}

	svc := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:      svcName,
			Namespace: resourceNamespace,
		},
	}
	log.V(2).Info("Deleting service", "name", svc.Name, "namespace", svc.Namespace)
	if err := r.Delete(ctx, svc); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete service %s: %v against LoadBalancer %w", svc.Name, fmt.Sprintf("%s/%s", lb.Name, lb.Namespace), err)
	}

	// Remove finalizer
	kuberneteshelper.RemoveFinalizer(&lb, envoyProxyCleanupFinalizer)

	// Update instance
	err := r.Update(ctx, &lb)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return nil
}

func (r *LoadBalancerReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&kubelbk8ciov1alpha1.LoadBalancer{}).
		WithEventFilter(utils.ByLabelExistsOnNamespace(ctx, mgr.GetClient())).
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

	// Watch for changes to Config in the KubeLB management cluster.
	if err := c.Watch(source.Kind(mgr.GetCache(), &kubelbk8ciov1alpha1.Config{}), handler.EnqueueRequestsFromMapFunc(r.enqueueLoadBalancersForConfig())); err != nil {
		return fmt.Errorf("failed to watch Config: %w", err)
	}

	return err
}

func (r *LoadBalancerReconciler) getEnvoyProxyPodSpec(namespace, appName, snapshotName string) corev1.PodTemplateSpec {
	envoyProxy := config.GetConfig().Spec.EnvoyProxy
	template := corev1.PodTemplateSpec{
		ObjectMeta: v1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
			Labels:    map[string]string{kubelb.LabelAppKubernetesName: appName},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  envoyProxyContainerName,
					Image: envoyImage,
					Args: []string{
						"--config-yaml", r.EnvoyBootstrap,
						"--service-node", snapshotName,
						"--service-cluster", namespace,
					},
				},
			},
		},
	}

	if envoyProxy.Resources != nil {
		template.Spec.Containers[0].Resources = *envoyProxy.Resources
	}

	if envoyProxy.Affinity != nil {
		template.Spec.Affinity = envoyProxy.Affinity
	}

	if envoyProxy.Tolerations != nil && len(envoyProxy.Tolerations) > 0 {
		template.Spec.Tolerations = envoyProxy.Tolerations
	}

	if envoyProxy.NodeSelector != nil {
		template.Spec.NodeSelector = envoyProxy.NodeSelector
	}

	if envoyProxy.SinglePodPerNode {
		template.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
			{
				MaxSkew:           1,
				TopologyKey:       "kubernetes.io/hostname",
				WhenUnsatisfiable: corev1.ScheduleAnyway,
				LabelSelector: &v1.LabelSelector{
					MatchLabels: map[string]string{kubelb.LabelAppKubernetesName: appName},
				},
			},
		}
	}
	return template
}

// enqueueLoadBalancers is a handler.MapFunc to be used to enqeue requests for reconciliation
// for LoadBalancers against the corresponding service.
func (r *LoadBalancerReconciler) enqueueLoadBalancers() handler.MapFunc {
	return func(_ context.Context, o ctrlruntimeclient.Object) []ctrl.Request {
		result := []reconcile.Request{}

		// Find the LoadBalancer that corresponds to this service.
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
// We only want to handle services that are managed by kubelb.
func filterServicesPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetLabels()[kubelb.LabelLoadBalancerName] != ""
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetLabels()[kubelb.LabelLoadBalancerName] != ""
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			newSvc := e.ObjectNew.(*corev1.Service)
			oldSvc := e.ObjectOld.(*corev1.Service)

			// Ensure that this service is managed by kubelb
			if e.ObjectNew.GetLabels()[kubelb.LabelLoadBalancerName] == "" {
				return false
			}

			return !reflect.DeepEqual(newSvc.Status, oldSvc.Status)
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}
}

func propagateAnnotations(permitted map[string]string, loadbalancer map[string]string) map[string]string {
	if config.GetConfig().Spec.PropagateAllAnnotations {
		return loadbalancer
	}

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

	if config.GetConfig().Spec.PropagatedAnnotations != nil {
		globallyPermitted := config.GetConfig().Spec.PropagatedAnnotations
		for k, v := range globallyPermitted {
			// Annotations specified at namespace level have a higher precedence.
			if _, found := permittedMap[k]; !found {
				permittedMap[k] = []string{v}
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

// enqueueLoadBalancersForConfig is a handler.MapFunc to be used to enqeue requests for reconciliation
// for LoadBalancers if some change is made to the controller config.
func (r *LoadBalancerReconciler) enqueueLoadBalancersForConfig() handler.MapFunc {
	return func(ctx context.Context, _ ctrlruntimeclient.Object) []ctrl.Request {
		result := []reconcile.Request{}

		// Reload the Config for the controller.
		conf := &kubelbk8ciov1alpha1.Config{}
		err := r.Get(ctx, types.NamespacedName{Name: config.DefaultConfigResourceName, Namespace: r.Namespace}, conf)
		if err != nil {
			return result
		}
		config.SetConfig(*conf)

		// List all loadbalancers. We don't care about the namespace here.
		loadBalancers := &kubelbk8ciov1alpha1.LoadBalancerList{}
		err = r.List(ctx, loadBalancers)
		if err != nil {
			return result
		}

		for _, lb := range loadBalancers.Items {
			result = append(result, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      lb.Name,
					Namespace: lb.Namespace,
				},
			})
		}

		return result
	}
}
