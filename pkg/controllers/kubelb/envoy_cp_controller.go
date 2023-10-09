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

	envoycachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	envoyresource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	utils "k8c.io/kubelb/pkg/controllers"
	envoycp "k8c.io/kubelb/pkg/envoy"
	portlookup "k8c.io/kubelb/pkg/port-lookup"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EnvoyCPReconciler struct {
	client.Client
	EnvoyCache         envoycachev3.SnapshotCache
	EnvoyProxyTopology EnvoyProxyTopology
	PortAllocator      *portlookup.PortAllocator
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=loadbalancers,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=loadbalancers/status,verbs=get
func (r *EnvoyCPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("reconciling LoadBalancer")

	return ctrl.Result{}, r.reconcile(ctx, req)
}

func (r *EnvoyCPReconciler) reconcile(ctx context.Context, req ctrl.Request) error {
	var lb kubelbk8ciov1alpha1.LoadBalancer
	err := r.Get(ctx, req.NamespacedName, &lb)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	snapshotName := envoySnapshotName(r.EnvoyProxyTopology, req)

	switch r.EnvoyProxyTopology {
	case EnvoyProxyTopologyDedicated:
		if apierrors.IsNotFound(err) || !lb.DeletionTimestamp.IsZero() {
			r.EnvoyCache.ClearSnapshot(snapshotName)
			return nil
		}
		return r.updateCache(ctx, snapshotName, []kubelbk8ciov1alpha1.LoadBalancer{lb})
	case EnvoyProxyTopologyShared:
		lbs, err := r.listLBs(ctx, client.InNamespace(lb.Namespace))
		if err != nil {
			return err
		}
		if len(lbs) == 0 {
			r.EnvoyCache.ClearSnapshot(snapshotName)
			return nil
		}
		return r.updateCache(ctx, snapshotName, lbs)
	case EnvoyProxyTopologyGlobal:
		lbs, err := r.listLBs(ctx)
		if err != nil {
			return err
		}
		if len(lbs) == 0 {
			r.EnvoyCache.ClearSnapshot(snapshotName)
			return nil
		}

		// For Global topology, we need to ensure that an arbitrary port has been assigned to the endpoint ports of the LoadBalancer.
		lbList := kubelbk8ciov1alpha1.LoadBalancerList{
			Items: lbs,
		}
		if err := r.PortAllocator.AllocatePortsForLoadBalancers(lbList); err != nil {
			return err
		}

		return r.updateCache(ctx, snapshotName, lbs)
	}
	return fmt.Errorf("unknown envoy proxy topology: %v", r.EnvoyProxyTopology)
}

func (r *EnvoyCPReconciler) updateCache(ctx context.Context, snapshotName string, lbs []kubelbk8ciov1alpha1.LoadBalancer) error {
	log := ctrl.LoggerFrom(ctx)
	currentSnapshot, err := r.EnvoyCache.GetSnapshot(snapshotName)
	if err != nil {
		initSnapshot, err := envoycp.MapSnapshot(lbs, r.PortAllocator, r.EnvoyProxyTopology == EnvoyProxyTopologyGlobal)
		if err != nil {
			return fmt.Errorf("failed to init snapshot %q: %w", snapshotName, err)
		}
		log.Info("init snapshot", "service-node", snapshotName, "version", initSnapshot.GetVersion(envoyresource.ClusterType))
		return r.EnvoyCache.SetSnapshot(ctx, snapshotName, initSnapshot)
	}

	desiredSnapshot, err := envoycp.MapSnapshot(lbs, r.PortAllocator, r.EnvoyProxyTopology == EnvoyProxyTopologyGlobal)
	if err != nil {
		return err
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

func (r *EnvoyCPReconciler) listLBs(ctx context.Context, options ...client.ListOption) (lbs []kubelbk8ciov1alpha1.LoadBalancer, err error) {
	var list kubelbk8ciov1alpha1.LoadBalancerList
	err = r.List(ctx, &list, options...)
	if err != nil {
		return
	}
	for _, lb := range list.Items {
		if lb.DeletionTimestamp.IsZero() {
			lbs = append(lbs, lb)
		}
	}
	return
}

func envoySnapshotName(topology EnvoyProxyTopology, req ctrl.Request) string {
	switch topology {
	case EnvoyProxyTopologyShared:
		return req.Namespace
	case EnvoyProxyTopologyDedicated:
		return fmt.Sprintf("%s-%s", req.Namespace, req.Name)
	case EnvoyProxyTopologyGlobal:
		return envoyGlobalCache
	}
	return ""
}

func (r *EnvoyCPReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubelbk8ciov1alpha1.LoadBalancer{}).
		WithEventFilter(utils.ByLabelExistsOnNamespace(ctx, mgr.GetClient())).
		Complete(r)
}
