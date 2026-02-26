/*
Copyright 2023 The KubeLB Authors.

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
	"time"

	envoycachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	envoyresource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	utils "k8c.io/kubelb/internal/controllers"
	envoycp "k8c.io/kubelb/internal/envoy"
	"k8c.io/kubelb/internal/kubelb"
	"k8c.io/kubelb/internal/metricsutil"
	envoycpmetrics "k8c.io/kubelb/internal/metricsutil/envoycp"
	managermetrics "k8c.io/kubelb/internal/metricsutil/manager"
	portlookup "k8c.io/kubelb/internal/port-lookup"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	RequeueAllResources   = "requeue-all-for-route"
	EnvoyCPControllerName = "envoy-cp-controller"
)

type EnvoyCPReconciler struct {
	ctrlruntimeclient.Client
	EnvoyCache        envoycachev3.SnapshotCache
	PortAllocator     *portlookup.PortAllocator
	Namespace         string
	EnvoyServer       *envoycp.Server
	DisableGatewayAPI bool
	Config            *kubelbv1alpha1.Config
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=loadbalancers,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=loadbalancers/status,verbs=get
func (r *EnvoyCPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	startTime := time.Now()

	// Track reconciliation duration
	defer func() {
		managermetrics.EnvoyCPReconcileDuration.WithLabelValues().Observe(time.Since(startTime).Seconds())
	}()

	log.V(2).Info("reconciling LoadBalancer")

	// Retrieve updated config.
	config, err := GetConfig(ctx, r.Client, r.Namespace)
	if err != nil {
		managermetrics.EnvoyCPReconcileTotal.WithLabelValues(metricsutil.ResultError).Inc()
		return ctrl.Result{}, fmt.Errorf("failed to retrieve config: %w", err)
	}
	r.Config = config
	// Update EnvoyServer config so GenerateBootstrap() uses the latest config
	r.EnvoyServer.UpdateConfig(config)

	if err := r.reconcile(ctx, req); err != nil {
		managermetrics.EnvoyCPReconcileTotal.WithLabelValues(metricsutil.ResultError).Inc()
		return ctrl.Result{}, err
	}

	managermetrics.EnvoyCPReconcileTotal.WithLabelValues(metricsutil.ResultSuccess).Inc()
	return ctrl.Result{}, nil
}

func (r *EnvoyCPReconciler) reconcile(ctx context.Context, req ctrl.Request) error {
	snapshotName, appName := req.Namespace, req.Namespace

	lbs, routes, err := r.ListLoadBalancersAndRoutes(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to list LoadBalancers and Routes: %w", err)
	}

	namespace := req.Namespace

	if len(lbs) == 0 && len(routes) == 0 {
		r.EnvoyCache.ClearSnapshot(snapshotName)
		envoycpmetrics.CacheClearsTotal.WithLabelValues(snapshotName).Inc()
		return r.cleanupEnvoyProxy(ctx, appName, namespace)
	}

	if err := r.ensureEnvoyProxy(ctx, namespace, appName, snapshotName); err != nil {
		return fmt.Errorf("failed to update Envoy proxy: %w", err)
	}
	managermetrics.EnvoyProxiesTotal.WithLabelValues(namespace, "shared").Set(1)
	envoycpmetrics.EnvoyProxiesTotal.WithLabelValues(namespace, "shared").Set(1)

	lbList := kubelbv1alpha1.LoadBalancerList{
		Items: lbs,
	}
	if err := r.PortAllocator.AllocatePortsForLoadBalancers(lbList); err != nil {
		return err
	}

	if err := r.PortAllocator.AllocatePortsForRoutes(routes); err != nil {
		return err
	}

	return r.updateCache(ctx, snapshotName, lbs, routes)
}

func (r *EnvoyCPReconciler) updateCache(ctx context.Context, snapshotName string, lbs []kubelbv1alpha1.LoadBalancer, routes []kubelbv1alpha1.Route) error {
	log := ctrl.LoggerFrom(ctx)
	snapshotStart := time.Now()
	desiredSnapshot, err := envoycp.MapSnapshot(ctx, r.Client, lbs, routes, r.PortAllocator)
	envoycpmetrics.SnapshotGenerationDuration.WithLabelValues(snapshotName).Observe(time.Since(snapshotStart).Seconds())
	if err != nil {
		return err
	}

	currentSnapshot, err := r.EnvoyCache.GetSnapshot(snapshotName)
	if err != nil {
		envoycpmetrics.CacheMissesTotal.WithLabelValues(snapshotName).Inc()
		log.Info("init snapshot", "service-node", snapshotName, "version", desiredSnapshot.GetVersion(envoyresource.ClusterType))
		if err := r.EnvoyCache.SetSnapshot(ctx, snapshotName, desiredSnapshot); err != nil {
			return err
		}
		r.recordSnapshotMetrics(snapshotName, desiredSnapshot)
		managermetrics.EnvoyCPSnapshotUpdatesTotal.WithLabelValues(snapshotName).Inc()
		envoycpmetrics.SnapshotUpdatesTotal.WithLabelValues(snapshotName).Inc()
		return nil
	}
	envoycpmetrics.CacheHitsTotal.WithLabelValues(snapshotName).Inc()

	lastUsedVersion := currentSnapshot.GetVersion(envoyresource.ClusterType)
	desiredVersion := desiredSnapshot.GetVersion(envoyresource.ClusterType)
	if lastUsedVersion == desiredVersion {
		log.V(2).Info("snapshot is in desired state")
		return nil
	}

	if err := desiredSnapshot.Consistent(); err != nil {
		return fmt.Errorf("new Envoy config snapshot is not consistent: %w", err)
	}

	log.Info("updating snapshot", "service-node", snapshotName, "version", desiredSnapshot.GetVersion(envoyresource.ClusterType))

	if err := r.EnvoyCache.SetSnapshot(ctx, snapshotName, desiredSnapshot); err != nil {
		return fmt.Errorf("failed to set a new Envoy cache snapshot: %w", err)
	}

	r.recordSnapshotMetrics(snapshotName, desiredSnapshot)
	managermetrics.EnvoyCPSnapshotUpdatesTotal.WithLabelValues(snapshotName).Inc()
	envoycpmetrics.SnapshotUpdatesTotal.WithLabelValues(snapshotName).Inc()

	return nil
}

// recordSnapshotMetrics records the resource counts for a snapshot.
func (r *EnvoyCPReconciler) recordSnapshotMetrics(snapshotName string, snapshot envoycachev3.ResourceSnapshot) {
	clusters := snapshot.GetResources(envoyresource.ClusterType)
	listeners := snapshot.GetResources(envoyresource.ListenerType)
	endpoints := snapshot.GetResources(envoyresource.EndpointType)
	routes := snapshot.GetResources(envoyresource.RouteType)
	secrets := snapshot.GetResources(envoyresource.SecretType)

	managermetrics.EnvoyCPClusters.WithLabelValues(snapshotName).Set(float64(len(clusters)))
	managermetrics.EnvoyCPListeners.WithLabelValues(snapshotName).Set(float64(len(listeners)))
	managermetrics.EnvoyCPEndpoints.WithLabelValues(snapshotName).Set(float64(len(endpoints)))

	envoycpmetrics.ClustersTotal.WithLabelValues(snapshotName).Set(float64(len(clusters)))
	envoycpmetrics.ListenersTotal.WithLabelValues(snapshotName).Set(float64(len(listeners)))
	envoycpmetrics.EndpointsTotal.WithLabelValues(snapshotName).Set(float64(len(endpoints)))
	envoycpmetrics.RoutesTotal.WithLabelValues(snapshotName).Set(float64(len(routes)))
	envoycpmetrics.SecretsTotal.WithLabelValues(snapshotName).Set(float64(len(secrets)))
}

func (r *EnvoyCPReconciler) ListLoadBalancersAndRoutes(ctx context.Context, req ctrl.Request) ([]kubelbv1alpha1.LoadBalancer, []kubelbv1alpha1.Route, error) {
	loadBalancers := kubelbv1alpha1.LoadBalancerList{}
	routes := kubelbv1alpha1.RouteList{}
	var err error

	err = r.List(ctx, &loadBalancers, ctrlruntimeclient.InNamespace(req.Namespace))
	if err != nil {
		return nil, nil, err
	}

	err = r.List(ctx, &routes, ctrlruntimeclient.InNamespace(req.Namespace))
	if err != nil {
		return nil, nil, err
	}

	lbs := make([]kubelbv1alpha1.LoadBalancer, 0, len(loadBalancers.Items))
	for _, lb := range loadBalancers.Items {
		if lb.DeletionTimestamp.IsZero() {
			lbs = append(lbs, lb)
		}
	}

	routeList := make([]kubelbv1alpha1.Route, 0, len(routes.Items))
	for _, route := range routes.Items {
		if route.DeletionTimestamp.IsZero() {
			routeList = append(routeList, route)
		}
	}

	return lbs, routeList, nil
}

func (r *EnvoyCPReconciler) cleanupEnvoyProxy(ctx context.Context, appName string, namespace string) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "envoy-proxy")
	log.V(2).Info("cleanup envoy-proxy")

	objMeta := metav1.ObjectMeta{
		Name:      fmt.Sprintf(envoyResourcePattern, appName),
		Namespace: namespace,
	}
	var envoyProxy ctrlruntimeclient.Object
	if r.Config.Spec.EnvoyProxy.UseDaemonset {
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
		return fmt.Errorf("failed to delete envoy proxy %s: %w", envoyProxy.GetName(), err)
	}
	envoycpmetrics.EnvoyProxyDeleteTotal.WithLabelValues(namespace).Inc()
	return nil
}

func (r *EnvoyCPReconciler) ensureEnvoyProxy(ctx context.Context, namespace, appName, snapshotName string) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "envoy-proxy")
	log.V(2).Info("verify envoy-proxy")

	var envoyProxy ctrlruntimeclient.Object
	objMeta := metav1.ObjectMeta{
		Name:      fmt.Sprintf(envoyResourcePattern, appName),
		Namespace: namespace,
		Labels: map[string]string{
			kubelb.LabelAppKubernetesName:      "kubelb-envoy-proxy",
			kubelb.LabelAppKubernetesInstance:  appName,
			kubelb.LabelAppKubernetesManagedBy: "kubelb",
		},
	}
	if r.Config.Spec.EnvoyProxy.UseDaemonset {
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

	desiredTemplate := r.getEnvoyProxyPodSpec(namespace, appName, snapshotName)
	var originalTemplate corev1.PodTemplateSpec
	if r.Config.Spec.EnvoyProxy.UseDaemonset {
		daemonset := envoyProxy.(*appsv1.DaemonSet)
		originalTemplate = daemonset.Spec.Template
		daemonset.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{kubelb.LabelAppKubernetesName: appName},
		}
		daemonset.Spec.Template = desiredTemplate
		envoyProxy = daemonset
	} else {
		deployment := envoyProxy.(*appsv1.Deployment)
		originalTemplate = deployment.Spec.Template
		var replicas = r.Config.Spec.EnvoyProxy.Replicas
		deployment.Spec.Replicas = &replicas
		deployment.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{kubelb.LabelAppKubernetesName: appName},
		}
		deployment.Spec.Template = desiredTemplate
		envoyProxy = deployment
	}

	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, envoyProxy); err != nil {
			return err
		}
		envoycpmetrics.EnvoyProxyCreateTotal.WithLabelValues(namespace).Inc()
	} else if podTemplateSpecNeedsUpdate(originalTemplate, desiredTemplate) {
		if err := r.Update(ctx, envoyProxy); err != nil {
			return fmt.Errorf("failed to update envoy proxy: %w", err)
		}
	}
	return nil
}

func (r *EnvoyCPReconciler) getEnvoyProxyPodSpec(namespace, appName, snapshotName string) corev1.PodTemplateSpec {
	envoyProxy := r.Config.Spec.EnvoyProxy

	// Use configured image or fall back to default
	image := envoyProxy.Image
	if image == "" {
		image = envoyImage
	}

	// Extract graceful shutdown configuration
	gracefulShutdownEnabled := true
	drainTimeout := int64(envoycp.DefaultEnvoyDrainTimeout)
	terminationGracePeriod := int64(envoycp.DefaultEnvoyTerminationGracePeriod)
	shutdownManagerImage := envoycp.DefaultShutdownManagerImage

	if envoyProxy.GracefulShutdown != nil {
		if envoyProxy.GracefulShutdown.Disabled {
			gracefulShutdownEnabled = false
		}
		if envoyProxy.GracefulShutdown.DrainTimeout != nil {
			drainTimeout = int64(envoyProxy.GracefulShutdown.DrainTimeout.Seconds())
		}
		if envoyProxy.GracefulShutdown.TerminationGracePeriodSeconds != nil {
			terminationGracePeriod = *envoyProxy.GracefulShutdown.TerminationGracePeriodSeconds
		}
		if envoyProxy.GracefulShutdown.ShutdownManagerImage != "" {
			shutdownManagerImage = envoyProxy.GracefulShutdown.ShutdownManagerImage
		}
	}

	// Build envoy proxy container
	envoyContainer := corev1.Container{
		Name:  envoyProxyContainerName,
		Image: image,
		Args: []string{
			"--config-yaml", r.EnvoyServer.GenerateBootstrap(),
			"--service-node", snapshotName,
			"--service-cluster", namespace,
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "readiness",
				ContainerPort: envoycp.EnvoyReadinessPort,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          "metrics",
				ContainerPort: envoycp.EnvoyStatsPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   envoycp.EnvoyReadinessPath,
					Port:   intstr.IntOrString{Type: intstr.Int, IntVal: envoycp.EnvoyReadinessPort},
					Scheme: corev1.URISchemeHTTP,
				},
			},
			TimeoutSeconds:   5,
			PeriodSeconds:    10,
			SuccessThreshold: 1,
			FailureThreshold: 30,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   envoycp.EnvoyReadinessPath,
					Port:   intstr.IntOrString{Type: intstr.Int, IntVal: envoycp.EnvoyReadinessPort},
					Scheme: corev1.URISchemeHTTP,
				},
			},
			TimeoutSeconds:   5,
			PeriodSeconds:    10,
			SuccessThreshold: 1,
			FailureThreshold: 2,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   envoycp.EnvoyReadinessPath,
					Port:   intstr.IntOrString{Type: intstr.Int, IntVal: envoycp.EnvoyReadinessPort},
					Scheme: corev1.URISchemeHTTP,
				},
			},
			TimeoutSeconds:   5,
			PeriodSeconds:    10,
			SuccessThreshold: 1,
			FailureThreshold: 3,
		},
	}

	// Add lifecycle hook if graceful shutdown is enabled
	if gracefulShutdownEnabled {
		envoyContainer.Lifecycle = &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   envoycp.ShutdownManagerReadyPath,
					Port:   intstr.FromInt(envoycp.ShutdownManagerPort),
					Scheme: corev1.URISchemeHTTP,
				},
			},
		}
	}

	containers := []corev1.Container{envoyContainer}
	if gracefulShutdownEnabled {
		shutdownManagerContainer := corev1.Container{
			Name:    shutdownManagerContainerName,
			Image:   shutdownManagerImage,
			Command: []string{"envoy-gateway"},
			Args: []string{
				"envoy",
				"shutdown-manager",
				fmt.Sprintf("--ready-timeout=%ds", drainTimeout+10),
			},
			Ports: []corev1.ContainerPort{
				{
					Name:          "shutdown",
					ContainerPort: envoycp.ShutdownManagerPort,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10m"),
					corev1.ResourceMemory: resource.MustParse("32Mi"),
				},
			},
			StartupProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   envoycp.ShutdownManagerHealthCheckPath,
						Port:   intstr.FromInt(envoycp.ShutdownManagerPort),
						Scheme: corev1.URISchemeHTTP,
					},
				},
				TimeoutSeconds:   1,
				PeriodSeconds:    10,
				SuccessThreshold: 1,
				FailureThreshold: 30,
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   envoycp.ShutdownManagerHealthCheckPath,
						Port:   intstr.FromInt(envoycp.ShutdownManagerPort),
						Scheme: corev1.URISchemeHTTP,
					},
				},
				TimeoutSeconds:   1,
				PeriodSeconds:    10,
				SuccessThreshold: 1,
				FailureThreshold: 3,
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   envoycp.ShutdownManagerHealthCheckPath,
						Port:   intstr.FromInt(envoycp.ShutdownManagerPort),
						Scheme: corev1.URISchemeHTTP,
					},
				},
				TimeoutSeconds:   1,
				PeriodSeconds:    10,
				SuccessThreshold: 1,
				FailureThreshold: 3,
			},
		}
		containers = append(containers, shutdownManagerContainer)
	}

	template := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
			Labels: map[string]string{
				kubelb.LabelAppKubernetesName:      appName,
				kubelb.LabelAppKubernetesManagedBy: "kubelb",
			},
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/port":   fmt.Sprintf("%d", envoycp.EnvoyStatsPort),
				"prometheus.io/path":   "/stats/prometheus",
			},
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: ptr.To(terminationGracePeriod),
			Containers:                    containers,
		},
	}

	if envoyProxy.Resources != nil {
		template.Spec.Containers[0].Resources = *envoyProxy.Resources
	}

	if envoyProxy.Affinity != nil {
		template.Spec.Affinity = envoyProxy.Affinity
	}

	if len(envoyProxy.Tolerations) > 0 {
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
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{kubelb.LabelAppKubernetesName: appName},
				},
			},
		}
	}
	return template
}

// podTemplateSpecNeedsUpdate compares the relevant fields of two PodTemplateSpecs
func podTemplateSpecNeedsUpdate(original, desired corev1.PodTemplateSpec) bool {
	if !reflect.DeepEqual(original.Annotations, desired.Annotations) {
		return true
	}
	if !reflect.DeepEqual(original.Labels, desired.Labels) {
		return true
	}

	// Compare only the fields we actually configure in getEnvoyProxyPodSpec
	origSpec := original.Spec
	desiredSpec := desired.Spec

	if !reflect.DeepEqual(origSpec.TerminationGracePeriodSeconds, desiredSpec.TerminationGracePeriodSeconds) {
		return true
	}

	if !reflect.DeepEqual(origSpec.Containers, desiredSpec.Containers) {
		return true
	}

	if !reflect.DeepEqual(origSpec.Affinity, desiredSpec.Affinity) {
		return true
	}

	if !reflect.DeepEqual(origSpec.Tolerations, desiredSpec.Tolerations) {
		return true
	}

	if !reflect.DeepEqual(origSpec.NodeSelector, desiredSpec.NodeSelector) {
		return true
	}

	if !reflect.DeepEqual(origSpec.TopologySpreadConstraints, desiredSpec.TopologySpreadConstraints) {
		return true
	}

	return false
}

// enqueueLoadBalancers is a handler.MapFunc to be used to enqueue requests for reconciliation
// for LoadBalancers.
func (r *EnvoyCPReconciler) enqueueLoadBalancers() handler.MapFunc {
	return func(_ context.Context, o ctrlruntimeclient.Object) []ctrl.Request {
		result := []reconcile.Request{}

		result = append(result, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      RequeueAllResources,
				Namespace: o.GetNamespace(),
			},
		})

		return result
	}
}

// enqueueAllLoadBalancers enqueues reconciliation for all tenant namespaces.
// Used when Config changes affect all Envoy deployments.
func (r *EnvoyCPReconciler) enqueueAllLoadBalancers() handler.MapFunc {
	return func(ctx context.Context, _ ctrlruntimeclient.Object) []ctrl.Request {
		tenants := &kubelbv1alpha1.TenantList{}
		if err := r.List(ctx, tenants); err != nil {
			return nil
		}

		result := make([]reconcile.Request, 0, len(tenants.Items))
		for _, tenant := range tenants.Items {
			result = append(result, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      RequeueAllResources,
					Namespace: fmt.Sprintf(tenantNamespacePattern, tenant.Name),
				},
			})
		}
		return result
	}
}

// enqueueTenantLoadBalancers enqueues reconciliation for a specific tenant's namespace.
// Used when Tenant changes affect that tenant's Envoy configuration.
func (r *EnvoyCPReconciler) enqueueTenantLoadBalancers() handler.MapFunc {
	return func(_ context.Context, o ctrlruntimeclient.Object) []ctrl.Request {
		tenant := o.(*kubelbv1alpha1.Tenant)
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name:      RequeueAllResources,
				Namespace: fmt.Sprintf(tenantNamespacePattern, tenant.Name),
			},
		}}
	}
}

// configSpecChangedPredicate triggers when Config spec fields affecting Envoy change.
func configSpecChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldConfig := e.ObjectOld.(*kubelbv1alpha1.Config)
			newConfig := e.ObjectNew.(*kubelbv1alpha1.Config)
			return !reflect.DeepEqual(oldConfig.Spec.EnvoyProxy, newConfig.Spec.EnvoyProxy)
		},
		CreateFunc:  func(_ event.CreateEvent) bool { return true },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

// tenantSpecChangedPredicate triggers when Tenant spec fields affecting Envoy change.
func tenantSpecChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldTenant := e.ObjectOld.(*kubelbv1alpha1.Tenant)
			newTenant := e.ObjectNew.(*kubelbv1alpha1.Tenant)
			return !reflect.DeepEqual(oldTenant.Spec.Ingress.Class, newTenant.Spec.Ingress.Class)
		},
		CreateFunc:  func(_ event.CreateEvent) bool { return false },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

func (r *EnvoyCPReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// 1. Watch for changes in LoadBalancer resources.
	// 2. Resource must exist in a tenant namespace.
	// 3. Watch for changes in Route resources and enqueue LoadBalancer resources. TODO: we need to
	// find an alternative for this since it is more of a "hack".
	// 4. Watch Config and Tenant for changes that affect Envoy proxy configuration.
	//
	// Note: namespace filtering is applied per-watch rather than globally via WithEventFilter
	// because Config (in manager namespace) and Tenant (cluster-scoped) don't reside in tenant
	// namespaces but still need to trigger reconciliation.

	namespaceFilter := utils.ByLabelExistsOnNamespace(ctx, mgr.GetClient())
	return ctrl.NewControllerManagedBy(mgr).
		Named(EnvoyCPControllerName).
		For(&kubelbv1alpha1.LoadBalancer{}, builder.WithPredicates(namespaceFilter)).
		// Disable concurrency to ensure that only one snapshot is created at a time.
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Watches(
			&kubelbv1alpha1.Route{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueLoadBalancers()),
			builder.WithPredicates(namespaceFilter, predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&kubelbv1alpha1.Addresses{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueLoadBalancers()),
			builder.WithPredicates(namespaceFilter, predicate.GenerationChangedPredicate{}),
		).
		// Config and Tenant watches don't use namespace filter since they're not in tenant namespaces
		Watches(
			&kubelbv1alpha1.Config{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueAllLoadBalancers()),
			builder.WithPredicates(configSpecChangedPredicate()),
		).
		Watches(
			&kubelbv1alpha1.Tenant{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueTenantLoadBalancers()),
			builder.WithPredicates(tenantSpecChangedPredicate()),
		).
		Complete(r)
}
