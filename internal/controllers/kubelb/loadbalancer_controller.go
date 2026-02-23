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
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"time"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	utils "k8c.io/kubelb/internal/controllers"
	"k8c.io/kubelb/internal/kubelb"
	"k8c.io/kubelb/internal/metricsutil"
	managermetrics "k8c.io/kubelb/internal/metricsutil/manager"
	portlookup "k8c.io/kubelb/internal/port-lookup"
	"k8c.io/kubelb/internal/resources/gatewayapi/httproute"
	ingress "k8c.io/kubelb/internal/resources/ingress"
	k8sutils "k8c.io/kubelb/internal/util/kubernetes"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	envoyImage                   = "envoyproxy/envoy:distroless-v1.36.4"
	envoyProxyContainerName      = "envoy-proxy"
	shutdownManagerContainerName = "shutdown-manager"
	envoyResourcePattern         = "envoy-%s"
	envoyProxyCleanupFinalizer   = "kubelb.k8c.io/cleanup-envoy-proxy"
	hostnameCleanupFinalizer     = "kubelb.k8c.io/cleanup-hostname"
	LoadBalancerControllerName   = "loadbalancer-controller"
)

// LoadBalancerReconciler reconciles a LoadBalancer object
type LoadBalancerReconciler struct {
	ctrlruntimeclient.Client
	Scheme    *runtime.Scheme
	Cache     cache.Cache
	Namespace string

	PortAllocator *portlookup.PortAllocator
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
	startTime := time.Now()

	// Track reconciliation duration
	defer func() {
		managermetrics.LoadBalancerReconcileDuration.WithLabelValues(req.Namespace).Observe(time.Since(startTime).Seconds())
	}()

	log.V(2).Info("reconciling LoadBalancer")

	var loadBalancer kubelbv1alpha1.LoadBalancer
	err := r.Get(ctx, req.NamespacedName, &loadBalancer)
	if err != nil {
		if ctrlruntimeclient.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch LoadBalancer")
			managermetrics.LoadBalancerReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		}
		log.V(3).Info("LoadBalancer not found")
		return ctrl.Result{}, nil
	}

	log.V(5).Info("processing", "LoadBalancer", loadBalancer)

	// Todo: check validation webhook - ports must equal endpoint ports as well
	if len(loadBalancer.Spec.Endpoints) == 0 {
		log.Error(fmt.Errorf("invalid Spec"), "No Endpoints set")
		managermetrics.LoadBalancerReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return ctrl.Result{}, nil
	}

	// Fetch all load balancers in the namespace for shared topology.
	var loadBalancers kubelbv1alpha1.LoadBalancerList
	err = r.List(ctx, &loadBalancers, ctrlruntimeclient.InNamespace(req.Namespace))
	if err != nil {
		log.Error(err, "unable to fetch LoadBalancer list")
		managermetrics.LoadBalancerReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return ctrl.Result{}, err
	}
	resourceNamespace := req.Namespace
	// Resource is marked for deletion.
	if loadBalancer.DeletionTimestamp != nil {
		if controllerutil.ContainsFinalizer(&loadBalancer, envoyProxyCleanupFinalizer) ||
			controllerutil.ContainsFinalizer(&loadBalancer, CleanupFinalizer) ||
			controllerutil.ContainsFinalizer(&loadBalancer, hostnameCleanupFinalizer) {
			return reconcile.Result{}, r.cleanup(ctx, loadBalancer, resourceNamespace)
		}
		// Finalizer doesn't exist so clean up is already done
		return reconcile.Result{}, nil
	}

	// Before proceeding further we need to make sure that the resource is reconcilable.
	tenant, config, err := GetTenantAndConfig(ctx, r.Client, r.Namespace, RemoveTenantPrefix(loadBalancer.Namespace))
	if err != nil {
		log.Error(err, "unable to fetch Tenant and Config, cannot proceed")
		managermetrics.LoadBalancerReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return reconcile.Result{}, err
	}

	shouldReconcile, disabled, err := r.shouldReconcile(ctx, &loadBalancer, tenant, config)
	if err != nil {
		log.Error(err, "unable to determine if the LoadBalancer should be reconciled")
		managermetrics.LoadBalancerReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return reconcile.Result{}, err
	}

	// If the resource is disabled, we need to clean up the resources
	if controllerutil.ContainsFinalizer(&loadBalancer, CleanupFinalizer) && disabled {
		log.V(3).Info("Removing load balancer as load balancing is disabled")
		return reconcile.Result{}, r.cleanup(ctx, loadBalancer, resourceNamespace)
	}

	if !shouldReconcile {
		managermetrics.LoadBalancerReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSkipped).Inc()
		return reconcile.Result{}, nil
	}

	var className *string
	if tenant.Spec.LoadBalancer.Class != nil {
		className = tenant.Spec.LoadBalancer.Class
	} else if config.Spec.LoadBalancer.Class != nil {
		className = config.Spec.LoadBalancer.Class
	}

	annotations := GetAnnotations(tenant, config)

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&loadBalancer, CleanupFinalizer) {
		if ok := controllerutil.AddFinalizer(&loadBalancer, CleanupFinalizer); !ok {
			log.Error(nil, "Failed to add finalizer for the LoadBalancer")
			return ctrl.Result{Requeue: true}, nil
		}

		// Remove old finalizer since it is not used anymore.
		controllerutil.RemoveFinalizer(&loadBalancer, envoyProxyCleanupFinalizer)

		if err := r.Update(ctx, &loadBalancer); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.PortAllocator.AllocatePortsForLoadBalancers(loadBalancers); err != nil {
		managermetrics.LoadBalancerReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return ctrl.Result{}, err
	}

	// Generate the service name and app name.
	appName := req.Namespace
	svcName := fmt.Sprintf(envoyResourcePattern, loadBalancer.Name)

	err = r.reconcileService(ctx, &loadBalancer, svcName, appName, resourceNamespace, r.PortAllocator, className, annotations)
	if err != nil {
		log.Error(err, "Unable to reconcile service")
		managermetrics.LoadBalancerReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return ctrl.Result{}, err
	}

	// At this point, we have a service that is ready to be used.
	// Get the service to get its current status
	existingService := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      svcName,
		Namespace: resourceNamespace,
	}, existingService)
	if err != nil {
		log.Error(err, "Unable to get service for status update")
		managermetrics.LoadBalancerReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return ctrl.Result{}, err
	}

	// Check if we need to configure the hostname.
	var hostname string
	if kubelb.ShouldConfigureHostname(log, loadBalancer.Annotations, loadBalancer.Name, loadBalancer.Spec.Hostname, tenant, config) {
		var err error
		hostname, err = r.configureHostname(ctx, &loadBalancer, svcName, tenant, config, annotations)
		if err != nil {
			log.Error(err, "Unable to configure hostname")
			managermetrics.LoadBalancerReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
			return ctrl.Result{}, err
		}
	}

	// Update LoadBalancer status (including hostname if configured)
	if err := r.updateLoadBalancerStatus(ctx, &loadBalancer, existingService, hostname); err != nil {
		log.Error(err, "Unable to update LoadBalancer status")
		managermetrics.LoadBalancerReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultError).Inc()
		return ctrl.Result{}, err
	}

	// Update LB count gauge per namespace
	managermetrics.LoadBalancersTotal.WithLabelValues(req.Namespace, RemoveTenantPrefix(req.Namespace), "shared").Set(float64(len(loadBalancers.Items)))

	managermetrics.LoadBalancerReconcileTotal.WithLabelValues(req.Namespace, metricsutil.ResultSuccess).Inc()
	return ctrl.Result{}, nil
}

func (r *LoadBalancerReconciler) reconcileService(ctx context.Context, loadBalancer *kubelbv1alpha1.LoadBalancer, svcName, appName, namespace string, portAllocator *portlookup.PortAllocator, className *string,
	annotations kubelbv1alpha1.AnnotationSettings) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "service")

	log.V(2).Info("verify service")

	labels := map[string]string{
		kubelb.LabelAppKubernetesName:     appName,
		kubelb.LabelLoadBalancerName:      loadBalancer.Name,
		kubelb.LabelLoadBalancerNamespace: loadBalancer.Namespace,
		kubelb.LabelOriginNamespace:       loadBalancer.Labels[kubelb.LabelOriginNamespace],
		kubelb.LabelOriginName:            loadBalancer.Labels[kubelb.LabelOriginName],
	}

	desiredService := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:        svcName,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: kubelb.PropagateAnnotations(loadBalancer.Annotations, annotations, kubelbv1alpha1.AnnotatedResourceService),
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{kubelb.LabelAppKubernetesName: appName},
			Type:     loadBalancer.Spec.Type,
		},
	}

	if className != nil {
		desiredService.Spec.LoadBalancerClass = className
	}

	// Get existing service.
	existingService := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      svcName,
		Namespace: namespace,
	}, existingService)

	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	// Handle service ports
	desiredService.Spec.Ports = kubelb.CreateServicePorts(loadBalancer, existingService, portAllocator)

	// If service doesn't exist, create it
	if apierrors.IsNotFound(err) {
		log.V(2).Info("creating service", "name", svcName)
		if err := r.Create(ctx, desiredService); err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
	} else {
		// Merge the annotations with the existing annotations to allow annotations that are configured by third party controllers on the existing service to be preserved.
		desiredService.Annotations = k8sutils.MergeAnnotations(existingService.Annotations, desiredService.Annotations)

		// Service already exists, we need to check if it needs to be updated.
		if !equality.Semantic.DeepEqual(existingService.Spec.Ports, desiredService.Spec.Ports) ||
			!equality.Semantic.DeepEqual(existingService.Spec.Selector, desiredService.Spec.Selector) ||
			!equality.Semantic.DeepEqual(existingService.Spec.Type, desiredService.Spec.Type) ||
			!equality.Semantic.DeepEqual(existingService.Spec.LoadBalancerClass, desiredService.Spec.LoadBalancerClass) ||
			!equality.Semantic.DeepEqual(existingService.Labels, desiredService.Labels) ||
			!k8sutils.CompareAnnotations(existingService.Annotations, desiredService.Annotations) {
			log.V(2).Info("updating service", "name", svcName)
			existingService.Spec = desiredService.Spec
			existingService.Labels = desiredService.Labels
			existingService.Annotations = desiredService.Annotations
			if err := r.Update(ctx, existingService); err != nil {
				return fmt.Errorf("failed to update service: %w", err)
			}
		}
	}
	return nil
}

func (r *LoadBalancerReconciler) configureHostname(ctx context.Context, loadBalancer *kubelbv1alpha1.LoadBalancer, svcName string, tenant *kubelbv1alpha1.Tenant, config *kubelbv1alpha1.Config, annotations kubelbv1alpha1.AnnotationSettings) (string, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "hostname")

	log.V(2).Info("configure hostname", "name", loadBalancer.Name, "namespace", loadBalancer.Namespace)

	// Use existing hostname from status if available, otherwise generate/use spec hostname
	var hostname string
	if loadBalancer.Status.Hostname != nil && loadBalancer.Status.Hostname.Hostname != "" {
		hostname = loadBalancer.Status.Hostname.Hostname
	} else {
		// Generate new hostname or use spec hostname
		if loadBalancer.Spec.Hostname != "" {
			hostname = loadBalancer.Spec.Hostname
		} else {
			hostname = kubelb.GenerateHostname(tenant.Spec.DNS, config.Spec.DNS)
		}

		if hostname == "" {
			// No need for an error here since we can still manage the LB and skip the hostname configuration.
			log.V(2).Info("no hostname configurable, skipping")
			return "", nil
		}

		needsUpdate := loadBalancer.Status.Hostname == nil || loadBalancer.Status.Hostname.Hostname != hostname
		loadBalancer.Status.Hostname = &kubelbv1alpha1.HostnameStatus{
			Hostname: hostname,
		}
		if needsUpdate {
			if err := r.Status().Update(ctx, loadBalancer); err != nil {
				return "", fmt.Errorf("failed to update LoadBalancer status: %w", err)
			}
			log.V(2).Info("persisted hostname to status", "hostname", hostname)
		}
	}

	// Add hostname finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(loadBalancer, hostnameCleanupFinalizer) {
		if ok := controllerutil.AddFinalizer(loadBalancer, hostnameCleanupFinalizer); !ok {
			log.Error(nil, "Failed to add hostname finalizer for the LoadBalancer")
			return "", fmt.Errorf("failed to add hostname finalizer")
		}
		if err := r.Update(ctx, loadBalancer); err != nil {
			return "", fmt.Errorf("failed to add hostname finalizer: %w", err)
		}
		log.V(2).Info("added hostname finalizer")
	}

	// Determine whether to use Ingress or Gateway API based on configuration
	// Use Gateway API only if it's not disabled and a class is specified
	useGatewayAPI := !tenant.Spec.GatewayAPI.Disable && !config.Spec.GatewayAPI.Disable &&
		(tenant.Spec.GatewayAPI.Class != nil || config.Spec.GatewayAPI.Class != nil)

	if useGatewayAPI {
		// Create HTTPRoute for Gateway API
		return hostname, httproute.CreateHTTPRouteForHostname(ctx, r.Client, loadBalancer, svcName, hostname, tenant, config, annotations)
	}
	// Create Ingress resource
	return hostname, ingress.CreateIngressForHostname(ctx, r.Client, loadBalancer, svcName, hostname, tenant, config, annotations)
}

func (r *LoadBalancerReconciler) updateLoadBalancerStatus(ctx context.Context, loadBalancer *kubelbv1alpha1.LoadBalancer, service *corev1.Service, hostname string) error {
	log := ctrl.LoggerFrom(ctx)
	log.V(5).Info("updating load balancer status", "LoadBalancer", loadBalancer.Status.LoadBalancer.Ingress, "service", service.Status.LoadBalancer.Ingress, "hostname", hostname)

	// Validate that endpoints exist
	if len(loadBalancer.Spec.Endpoints) == 0 {
		log.V(3).Info("No endpoints found, skipping status update")
		return nil
	}

	updatedPorts := []kubelbv1alpha1.ServicePort{}
	for i, port := range service.Spec.Ports {
		targetPort := loadBalancer.Spec.Endpoints[0].Ports[i].Port
		updatedPorts = append(updatedPorts, kubelbv1alpha1.ServicePort{
			ServicePort: port,
			// In case of global topology, this will be different from the targetPort. Otherwise it will be the same.
			UpstreamTargetPort: targetPort,
		})
	}

	// Create the updated status
	updatedLoadBalancerStatus := kubelbv1alpha1.LoadBalancerStatus{
		Service: kubelbv1alpha1.ServiceStatus{
			Ports: updatedPorts,
		},
		LoadBalancer: service.Status.LoadBalancer,
	}

	// Add hostname status if hostname is configured and not already set
	if hostname != "" {
		// Only assign hostname if it's not already set in status
		var hostnameToUse string
		if loadBalancer.Status.Hostname != nil && loadBalancer.Status.Hostname.Hostname != "" {
			hostnameToUse = loadBalancer.Status.Hostname.Hostname
		} else {
			hostnameToUse = hostname
		}

		// Perform actual health checks for DNS and TLS
		dnsReady := r.checkDNSResolution(hostnameToUse)
		tlsReady := r.checkTLSHealth(fmt.Sprintf("https://%s", hostnameToUse))

		updatedLoadBalancerStatus.Hostname = &kubelbv1alpha1.HostnameStatus{
			Hostname:         hostnameToUse,
			TLSEnabled:       tlsReady, // Actually check if TLS endpoint is working
			DNSRecordCreated: dnsReady, // Actually check if DNS resolves
		}
	}

	// Check if status update is needed
	updateStatus := !reflect.DeepEqual(loadBalancer.Status.Service.Ports, updatedPorts)

	if loadBalancer.Spec.Type == corev1.ServiceTypeLoadBalancer {
		if !reflect.DeepEqual(loadBalancer.Status.LoadBalancer.Ingress, service.Status.LoadBalancer.Ingress) {
			updateStatus = true
		}
	}

	if !reflect.DeepEqual(loadBalancer.Status.Hostname, updatedLoadBalancerStatus.Hostname) {
		updateStatus = true
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
		lb.Status = updatedLoadBalancerStatus
		if reflect.DeepEqual(original.Status, lb.Status) {
			return nil
		}
		// update the status
		return r.Status().Patch(ctx, lb, ctrlruntimeclient.MergeFrom(original))
	})
}

func (r *LoadBalancerReconciler) cleanup(ctx context.Context, lb kubelbv1alpha1.LoadBalancer, resourceNamespace string) error {
	log := ctrl.LoggerFrom(ctx).WithValues("cleanup", "LoadBalancer")
	log.V(2).Info("Cleaning up LoadBalancer", "name", lb.Name, "namespace", lb.Namespace)

	// Deallocate ports for the load balancers.
	if err := r.PortAllocator.DeallocatePortsForLoadBalancer(lb); err != nil {
		return err
	}

	// Clean up hostname resources (Ingress/HTTPRoute) if hostname finalizer exists
	if controllerutil.ContainsFinalizer(&lb, hostnameCleanupFinalizer) {
		if err := r.cleanupHostnameResources(ctx, &lb, resourceNamespace); err != nil {
			return err
		}
	}

	// Remove corresponding service.
	svcName := fmt.Sprintf(envoyResourcePattern, lb.Name)

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

	// Remove finalizers
	controllerutil.RemoveFinalizer(&lb, CleanupFinalizer)
	controllerutil.RemoveFinalizer(&lb, envoyProxyCleanupFinalizer)
	controllerutil.RemoveFinalizer(&lb, hostnameCleanupFinalizer)

	// Update instance
	err := r.Update(ctx, &lb)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return nil
}

func (r *LoadBalancerReconciler) shouldReconcile(ctx context.Context, _ *kubelbv1alpha1.LoadBalancer, tenant *kubelbv1alpha1.Tenant, config *kubelbv1alpha1.Config) (bool, bool, error) {
	log := ctrl.LoggerFrom(ctx)

	// 1. Ensure that L4 loadbalancing is enabled.
	if config.Spec.LoadBalancer.Disable {
		log.Error(fmt.Errorf("L4 loadbalancing is disabled at the global level"), "cannot proceed")
		return false, true, nil
	} else if tenant.Spec.LoadBalancer.Disable {
		log.Error(fmt.Errorf("L4 loadbalancing is disabled at the tenant level"), "cannot proceed")
		return false, true, nil
	}
	return true, false, nil
}

func (r *LoadBalancerReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// Note: namespace filtering is applied per-watch rather than globally via WithEventFilter
	// because Config (in manager namespace) and Tenant (cluster-scoped) don't reside in tenant
	// namespaces but still need to trigger reconciliation.
	namespaceFilter := utils.ByLabelExistsOnNamespace(ctx, mgr.GetClient())

	return ctrl.NewControllerManagedBy(mgr).
		Named(LoadBalancerControllerName).
		For(&kubelbv1alpha1.LoadBalancer{}, builder.WithPredicates(namespaceFilter)).
		// Config and Tenant watches don't use namespace filter since they're not in tenant namespaces
		Watches(
			&kubelbv1alpha1.Config{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueLoadBalancersForConfig()),
		).
		Watches(
			&kubelbv1alpha1.Tenant{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueLoadBalancersForTenant()),
		).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueLoadBalancers()),
			builder.WithPredicates(namespaceFilter, filterServicesPredicate()),
		).
		Complete(r)
}

// enqueueLoadBalancers is a handler.MapFunc to be used to enqueue requests for reconciliation
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

func (r *LoadBalancerReconciler) cleanupHostnameResources(ctx context.Context, lb *kubelbv1alpha1.LoadBalancer, namespace string) error {
	log := ctrl.LoggerFrom(ctx).WithValues("cleanup", "hostname")
	log.V(2).Info("Cleaning up hostname resources", "name", lb.Name, "namespace", lb.Namespace)

	// Cleanup Ingress resource
	ingressName := fmt.Sprintf("%s-ingress", lb.Name)
	ingress := &networkingv1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:      ingressName,
			Namespace: namespace,
		},
	}
	log.V(2).Info("Deleting ingress", "name", ingress.Name, "namespace", ingress.Namespace)
	if err := r.Delete(ctx, ingress); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ingress %s: %w", ingress.Name, err)
	}

	// Cleanup HTTPRoute resource
	httpRouteName := fmt.Sprintf("%s-httproute", lb.Name)
	httpRoute := &gwapiv1.HTTPRoute{
		ObjectMeta: v1.ObjectMeta{
			Name:      httpRouteName,
			Namespace: namespace,
		},
	}
	log.V(2).Info("Deleting httproute", "name", httpRoute.Name, "namespace", httpRoute.Namespace)
	if err := r.Delete(ctx, httpRoute); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete httproute %s: %w", httpRoute.Name, err)
	}

	log.V(2).Info("Successfully cleaned up hostname resources")
	return nil
}

// filterServicesPredicate filters out services that need to be propagated to the event handlers.
// We only want to handle services that are managed by kubelb.
func filterServicesPredicate() predicate.TypedPredicate[ctrlruntimeclient.Object] {
	return predicate.TypedFuncs[ctrlruntimeclient.Object]{
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

// enqueueLoadBalancersForConfig is a handler.MapFunc to be used to enqueue requests for reconciliation
// for LoadBalancers if some change is made to the controller config.
func (r *LoadBalancerReconciler) enqueueLoadBalancersForConfig() handler.MapFunc {
	return func(ctx context.Context, _ ctrlruntimeclient.Object) []ctrl.Request {
		result := []reconcile.Request{}

		// List all loadbalancers. We don't care about the namespace here.
		loadBalancers := &kubelbv1alpha1.LoadBalancerList{}
		err := r.List(ctx, loadBalancers)
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

// enqueueLoadBalancersForTenant is a handler.MapFunc to be used to enqueue requests for reconciliation
// for e changLoadBalancers if some is made to the tenant config.
func (r *LoadBalancerReconciler) enqueueLoadBalancersForTenant() handler.MapFunc {
	return func(ctx context.Context, o ctrlruntimeclient.Object) []ctrl.Request {
		result := []reconcile.Request{}

		namespace := fmt.Sprintf(tenantNamespacePattern, o.GetName())

		// List all loadbalancers in tenant namespace
		loadBalancers := &kubelbv1alpha1.LoadBalancerList{}
		err := r.List(ctx, loadBalancers, ctrlruntimeclient.InNamespace(namespace))
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

// checkDNSResolution checks if the hostname resolves to an IP address
func (r *LoadBalancerReconciler) checkDNSResolution(hostname string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, hostname)
	if err != nil {
		ctrl.LoggerFrom(ctx).V(2).Info("DNS resolution failed", "hostname", hostname, "error", err)
		return false
	}
	ctrl.LoggerFrom(ctx).V(3).Info("DNS resolution successful", "hostname", hostname, "ips", ips)
	return true
}

// checkTLSHealth checks if the TLS endpoint has a working TLS connection
func (r *LoadBalancerReconciler) checkTLSHealth(url string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create HTTP client with timeout - skip cert verification to handle self-signed certs
	// We're checking if TLS handshake works, not certificate validity
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Skip cert verification to handle self-signed certificates
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		ctrl.LoggerFrom(ctx).V(2).Info("TLS health check failed - request creation", "url", url, "error", err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		// TLS handshake or connection errors will cause this to fail
		ctrl.LoggerFrom(ctx).V(2).Info("TLS health check failed - request execution", "url", url, "error", err)
		return false
	}
	defer resp.Body.Close()

	// If we get here, TLS handshake succeeded (even with self-signed cert)
	// Any HTTP response code means TLS is working
	ctrl.LoggerFrom(ctx).V(3).Info("TLS health check successful", "url", url, "statusCode", resp.StatusCode)
	return true
}
