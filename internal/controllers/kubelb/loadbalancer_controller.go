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

	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/config"
	utils "k8c.io/kubelb/internal/controllers"
	"k8c.io/kubelb/internal/kubelb"
	kuberneteshelper "k8c.io/kubelb/internal/kubernetes"
	portlookup "k8c.io/kubelb/internal/port-lookup"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	envoyImage                        = "envoyproxy/envoy:distroless-v1.30.1"
	envoyProxyContainerName           = "envoy-proxy"
	envoyResourcePattern              = "envoy-%s"
	envoyGlobalTopologyServicePattern = "envoy-%s-%s"
	envoyProxyCleanupFinalizer        = "kubelb.k8c.io/cleanup-envoy-proxy"
	EnvoyGlobalCache                  = "global"
)

type EnvoyProxyTopology string

const (
	EnvoyProxyTopologyShared    EnvoyProxyTopology = "shared"
	EnvoyProxyTopologyDedicated EnvoyProxyTopology = "dedicated"
	EnvoyProxyTopologyGlobal    EnvoyProxyTopology = "global"
)

func (e EnvoyProxyTopology) IsGlobalTopology() bool {
	return e == EnvoyProxyTopologyGlobal
}

// LoadBalancerReconciler reconciles a LoadBalancer object
type LoadBalancerReconciler struct {
	ctrlruntimeclient.Client
	Scheme    *runtime.Scheme
	Cache     cache.Cache
	Namespace string

	PortAllocator      *portlookup.PortAllocator
	EnvoyProxyTopology EnvoyProxyTopology
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=loadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=loadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=configs,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=configs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=addresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=addresses/status,verbs=get
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=daemonsets,verbs=get;list;watch;create;update;patch;delete

func (r *LoadBalancerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("reconciling LoadBalancer")

	var loadBalancer kubelbv1alpha1.LoadBalancer
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
		loadBalancers     kubelbv1alpha1.LoadBalancerList
		resourceNamespace string
	)

	switch r.EnvoyProxyTopology {
	case EnvoyProxyTopologyShared:
		err = r.List(ctx, &loadBalancers, ctrlruntimeclient.InNamespace(req.Namespace))
		if err != nil {
			log.Error(err, "unable to fetch LoadBalancer list")
			return ctrl.Result{}, err
		}
		resourceNamespace = req.Namespace
	case EnvoyProxyTopologyGlobal:
		// List all loadbalancers. We don't care about the namespace here.
		// TODO: ideally we should only process the load balancer that is being reconciled.
		err = r.List(ctx, &loadBalancers)
		if err != nil {
			log.Error(err, "unable to fetch LoadBalancer list")
			return ctrl.Result{}, err
		}
		resourceNamespace = r.Namespace
	case EnvoyProxyTopologyDedicated:
		loadBalancers.Items = []kubelbv1alpha1.LoadBalancer{loadBalancer}
		resourceNamespace = req.Namespace
	}
	// Resource is marked for deletion.
	if loadBalancer.DeletionTimestamp != nil {
		if kuberneteshelper.HasFinalizer(&loadBalancer, envoyProxyCleanupFinalizer) {
			return reconcile.Result{}, r.handleEnvoyProxyCleanup(ctx, loadBalancer, resourceNamespace)
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

	_, appName := envoySnapshotAndAppName(r.EnvoyProxyTopology, req)
	err = r.reconcileService(ctx, &loadBalancer, appName, resourceNamespace, r.PortAllocator)
	if err != nil {
		log.Error(err, "Unable to reconcile service")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *LoadBalancerReconciler) reconcileService(ctx context.Context, loadBalancer *kubelbv1alpha1.LoadBalancer, appName, namespace string, portAllocator *portlookup.PortAllocator) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "service")

	log.V(2).Info("verify service")

	tenant, err := getTenantForNamespace(ctx, r.Client, namespace)
	if err != nil {
		// This should never happen as the namespace should always have a tenant owner. We simply log this and continue.
		log.V(5).Info("Tenant not found for namespace", "namespace", namespace)
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
			Annotations: propagateAnnotations(tenant, loadBalancer.Annotations),
		},
	}
	err = r.Get(ctx, types.NamespacedName{
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

		for k, v := range propagateAnnotations(tenant, loadBalancer.Annotations) {
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

	updatedPorts := []kubelbv1alpha1.ServicePort{}
	for i, port := range service.Spec.Ports {
		targetPort := loadBalancer.Spec.Endpoints[0].Ports[i].Port
		updatedPorts = append(updatedPorts, kubelbv1alpha1.ServicePort{
			ServicePort: port,
			// In case of global topology, this will be different from the targetPort. Otherwise it will be the same.
			UpstreamTargetPort: targetPort,
		})
	}

	// Update status if needed
	updateStatus := false
	updatedLoadBalanacerStatus := kubelbv1alpha1.LoadBalancerStatus{
		Service: kubelbv1alpha1.ServiceStatus{
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
		lb := &kubelbv1alpha1.LoadBalancer{}
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

func (r *LoadBalancerReconciler) handleEnvoyProxyCleanup(ctx context.Context, lb kubelbv1alpha1.LoadBalancer, resourceNamespace string) error {
	log := ctrl.LoggerFrom(ctx).WithValues("cleanup", "LoadBalancer")
	log.V(2).Info("Cleaning up LoadBalancer", "name", lb.Name, "namespace", lb.Namespace)

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
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubelbv1alpha1.LoadBalancer{}).
		WithEventFilter(utils.ByLabelExistsOnNamespace(ctx, mgr.GetClient())).
		Watches(
			&kubelbv1alpha1.Config{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueLoadBalancersForConfig()),
			builder.WithPredicates(filterServicesPredicate()),
		).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueLoadBalancers()),
		).
		Complete(r)
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
func filterServicesPredicate() predicate.TypedPredicate[client.Object] {
	return predicate.TypedFuncs[client.Object]{
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

func propagateAnnotations(tenant *kubelbv1alpha1.Tenant, loadbalancer map[string]string) map[string]string {
	permitted := make(map[string]string)
	if tenant != nil {
		if tenant.Spec.LoadBalancer.PropagateAllAnnotations != nil && *tenant.Spec.LoadBalancer.PropagateAllAnnotations {
			return loadbalancer
		}
		if tenant.Spec.LoadBalancer.PropagatedAnnotations != nil {
			permitted = *tenant.Spec.LoadBalancer.PropagatedAnnotations
		}
	}

	if permitted == nil {
		if config.GetConfig().Spec.PropagateAllAnnotations {
			return loadbalancer
		}
		permitted = config.GetConfig().Spec.PropagatedAnnotations
	}

	a := make(map[string]string)
	permittedMap := make(map[string][]string)
	for k, v := range permitted {
		if _, found := permittedMap[k]; !found {
			permittedMap[k] = []string{v}
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
		conf := &kubelbv1alpha1.Config{}
		err := r.Get(ctx, types.NamespacedName{Name: config.DefaultConfigResourceName, Namespace: r.Namespace}, conf)
		if err != nil {
			return result
		}
		config.SetConfig(*conf)

		// List all loadbalancers. We don't care about the namespace here.
		loadBalancers := &kubelbv1alpha1.LoadBalancerList{}
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

func getTenantForNamespace(ctx context.Context, client ctrlruntimeclient.Client, namespace string) (*kubelbv1alpha1.Tenant, error) {
	ns := &corev1.Namespace{}
	err := client.Get(ctx, types.NamespacedName{Name: namespace}, ns)
	if err != nil {
		return nil, err
	}

	for _, owner := range ns.GetOwnerReferences() {
		if owner.Kind == "Tenant" {
			tenant := &kubelbv1alpha1.Tenant{}
			err := client.Get(ctx, types.NamespacedName{Name: owner.Name}, tenant)
			if err != nil {
				return nil, err
			}
			return tenant, nil
		}
	}
	return nil, fmt.Errorf("no tenant found for namespace %s", namespace)
}
