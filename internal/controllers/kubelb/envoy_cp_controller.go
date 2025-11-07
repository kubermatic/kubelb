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

	envoycachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	envoyresource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	utils "k8c.io/kubelb/internal/controllers"
	envoycp "k8c.io/kubelb/internal/envoy"
	"k8c.io/kubelb/internal/kubelb"
	portlookup "k8c.io/kubelb/internal/port-lookup"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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
	EnvoyCache         envoycachev3.SnapshotCache
	EnvoyProxyTopology EnvoyProxyTopology
	PortAllocator      *portlookup.PortAllocator
	Namespace          string
	EnvoyBootstrap     string
	EnvoyDebugMode     bool
	DisableGatewayAPI  bool
	Config             *kubelbv1alpha1.Config
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=loadbalancers,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=loadbalancers/status,verbs=get
func (r *EnvoyCPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("reconciling LoadBalancer")

	// Retrieve updated config.
	config, err := GetConfig(ctx, r.Client, r.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to retrieve config: %w", err)
	}
	r.Config = config

	return ctrl.Result{}, r.reconcile(ctx, req)
}

func (r *EnvoyCPReconciler) reconcile(ctx context.Context, req ctrl.Request) error {
	snapshotName, appName := envoySnapshotAndAppName(r.EnvoyProxyTopology, req)

	lbs, routes, err := r.ListLoadBalancersAndRoutes(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to list LoadBalancers and Routes: %w", err)
	}

	namespace := req.Namespace
	if r.EnvoyProxyTopology.IsGlobalTopology() {
		namespace = r.Namespace
	}

	if len(lbs) == 0 && len(routes) == 0 {
		r.EnvoyCache.ClearSnapshot(snapshotName)
		return r.cleanupEnvoyProxy(ctx, appName, namespace)
	}

	if err := r.ensureEnvoyProxy(ctx, namespace, appName, snapshotName); err != nil {
		return fmt.Errorf("failed to update Envoy proxy: %w", err)
	}

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
	desiredSnapshot, err := envoycp.MapSnapshot(ctx, r.Client, lbs, routes, r.PortAllocator)
	if err != nil {
		return err
	}

	currentSnapshot, err := r.EnvoyCache.GetSnapshot(snapshotName)
	if err != nil {
		log.Info("init snapshot", "service-node", snapshotName, "version", desiredSnapshot.GetVersion(envoyresource.ClusterType))
		return r.EnvoyCache.SetSnapshot(ctx, snapshotName, desiredSnapshot)
	}

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

	return nil
}

func (r *EnvoyCPReconciler) ListLoadBalancersAndRoutes(ctx context.Context, req ctrl.Request) ([]kubelbv1alpha1.LoadBalancer, []kubelbv1alpha1.Route, error) {
	loadBalancers := kubelbv1alpha1.LoadBalancerList{}
	routes := kubelbv1alpha1.RouteList{}
	var err error

	switch r.EnvoyProxyTopology {
	case EnvoyProxyTopologyShared:
		err = r.List(ctx, &loadBalancers, ctrlruntimeclient.InNamespace(req.Namespace))
		if err != nil {
			return nil, nil, err
		}

		err = r.List(ctx, &routes, ctrlruntimeclient.InNamespace(req.Namespace))
		if err != nil {
			return nil, nil, err
		}
	case EnvoyProxyTopologyGlobal:
		err = r.List(ctx, &loadBalancers)
		if err != nil {
			return nil, nil, err
		}

		err = r.List(ctx, &routes)
		if err != nil {
			return nil, nil, err
		}
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
	return nil
}

func (r *EnvoyCPReconciler) ensureEnvoyProxy(ctx context.Context, namespace, appName, snapshotName string) error {
	log := ctrl.LoggerFrom(ctx).WithValues("reconcile", "envoy-proxy")
	log.V(2).Info("verify envoy-proxy")

	var envoyProxy ctrlruntimeclient.Object
	objMeta := metav1.ObjectMeta{
		Name:      fmt.Sprintf(envoyResourcePattern, appName),
		Namespace: namespace,
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

	original := envoyProxy.DeepCopyObject()
	if r.Config.Spec.EnvoyProxy.UseDaemonset {
		daemonset := envoyProxy.(*appsv1.DaemonSet)
		daemonset.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{kubelb.LabelAppKubernetesName: appName},
		}
		daemonset.Spec.Template = r.getEnvoyProxyPodSpec(namespace, appName, snapshotName)
		envoyProxy = daemonset
	} else {
		deployment := envoyProxy.(*appsv1.Deployment)
		var replicas = r.Config.Spec.EnvoyProxy.Replicas
		deployment.Spec.Replicas = &replicas
		deployment.Spec.Selector = &metav1.LabelSelector{
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
			envoyProxy.SetManagedFields([]metav1.ManagedFieldsEntry{})
			envoyProxy.SetResourceVersion("")
			if err := r.Patch(ctx, envoyProxy, ctrlruntimeclient.Apply, ctrlruntimeclient.ForceOwnership, ctrlruntimeclient.FieldOwner("kubelb")); err != nil {
				return err
			}
		}
	}
	log.V(5).Info("desired", "envoy-proxy", envoyProxy)

	return nil
}

func (r *EnvoyCPReconciler) getEnvoyProxyPodSpec(namespace, appName, snapshotName string) corev1.PodTemplateSpec {
	envoyProxy := r.Config.Spec.EnvoyProxy
	template := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:        appName,
			Namespace:   namespace,
			Labels:      map[string]string{kubelb.LabelAppKubernetesName: appName},
			Annotations: map[string]string{},
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

	if r.EnvoyDebugMode {
		// Admin interface is enabled, and exposes Prometheus metrics.
		template.Annotations["prometheus.io/scrape"] = "true"
		template.Annotations["prometheus.io/port"] = "9001"
		template.Annotations["prometheus.io/path"] = "/stats/prometheus"

		template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{
				Name:          "admin",
				ContainerPort: 9001,
			},
		}
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

func envoySnapshotAndAppName(topology EnvoyProxyTopology, req ctrl.Request) (string, string) {
	switch topology {
	case EnvoyProxyTopologyShared:
		return req.Namespace, req.Namespace
	case EnvoyProxyTopologyGlobal:
		return EnvoyGlobalCache, EnvoyGlobalCache
	}
	return "", ""
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

func (r *EnvoyCPReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// 1. Watch for changes in LoadBalancer resources.
	// 2. Resource must exist in a tenant namespace.
	// 3. Watch for changes in Route resources and enqueue LoadBalancer resources. TODO: we need to
	// find an alternative for this since it is more of a "hack".
	return ctrl.NewControllerManagedBy(mgr).
		Named(EnvoyCPControllerName).
		For(&kubelbv1alpha1.LoadBalancer{}).
		// Disable concurrency to ensure that only one snapshot is created at a time.
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		WithEventFilter(utils.ByLabelExistsOnNamespace(ctx, mgr.GetClient())).
		Watches(
			&kubelbv1alpha1.Route{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueLoadBalancers()),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&kubelbv1alpha1.Addresses{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueLoadBalancers()),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}
