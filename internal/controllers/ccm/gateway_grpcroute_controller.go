/*
Copyright 2024 The KubeLB Authors.

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

package ccm

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	ccmmetrics "k8c.io/kubelb/internal/metricsutil/ccm"
	gatewayhelper "k8c.io/kubelb/internal/resources/gatewayapi/gateway"
	grpcrouteHelpers "k8c.io/kubelb/internal/resources/gatewayapi/grpcroute"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/yaml"
)

const (
	GatewayGRPCRouteControllerName = "gateway-grpcroute-controller"
	GRPCRouteGVK                   = "GRPCRoute.gateway.networking.k8s.io"
)

// GRPCRouteReconciler reconciles a GRPCRoute Object
type GRPCRouteReconciler struct {
	ctrlclient.Client

	LBManager   ctrl.Manager
	ClusterName string

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder

	loop *SourceReconciler[*gwapiv1.GRPCRoute]
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=grpcroutes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=grpcroutes/status,verbs=get;update;patch

func (r *GRPCRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.loop.Reconcile(ctx, req)
}

func (r *GRPCRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.loop = &SourceReconciler[*gwapiv1.GRPCRoute]{
		Client:      r.Client,
		LBClient:    r.LBManager.GetClient(),
		ClusterName: r.ClusterName,
		Log:         r.Log,
		Adapter:     &grpcRouteAdapter{},
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named(GatewayGRPCRouteControllerName).
		For(&gwapiv1.GRPCRoute{}, builder.WithPredicates(r.loop.Predicate())).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.loop.EnqueueForService()),
		).
		WatchesRawSource(
			source.Kind(r.LBManager.GetCache(), &kubelbv1alpha1.Route{},
				handler.TypedEnqueueRequestsFromMapFunc[*kubelbv1alpha1.Route](enqueueRoutes(GRPCRouteGVK, r.ClusterName))),
		).
		Complete(r)
}

type grpcRouteAdapter struct{}

var _ SourceAdapter[*gwapiv1.GRPCRoute] = (*grpcRouteAdapter)(nil)

func (a *grpcRouteAdapter) Kind() string                   { return "GRPCRoute" }
func (a *grpcRouteAdapter) GVK() string                    { return GRPCRouteGVK }
func (a *grpcRouteAdapter) NewObject() *gwapiv1.GRPCRoute  { return &gwapiv1.GRPCRoute{} }
func (a *grpcRouteAdapter) NewList() ctrlclient.ObjectList { return &gwapiv1.GRPCRouteList{} }

func (a *grpcRouteAdapter) Metrics() SourceMetrics {
	return SourceMetrics{
		ReconcileTotal:    ccmmetrics.GRPCRouteReconcileTotal,
		ReconcileDuration: ccmmetrics.GRPCRouteReconcileDuration,
		Managed:           ccmmetrics.ManagedGRPCRoutesTotal,
	}
}

func (a *grpcRouteAdapter) ShouldReconcile(grpcRoute *gwapiv1.GRPCRoute) bool {
	return gatewayhelper.ShouldReconcileResource(grpcRoute, false)
}

func (a *grpcRouteAdapter) ExtractServices(grpcRoute *gwapiv1.GRPCRoute) []types.NamespacedName {
	return grpcrouteHelpers.GetServicesFromGRPCRoute(grpcRoute)
}

func (a *grpcRouteAdapter) ApplyRouteStatus(grpcRoute *gwapiv1.GRPCRoute, raw []byte) (bool, error) {
	status := gwapiv1.GRPCRouteStatus{}
	if err := yaml.UnmarshalStrict(raw, &status); err != nil {
		return false, fmt.Errorf("failed to unmarshal GRPCRoute status: %w", err)
	}
	for i := range status.Parents {
		for j := range status.Parents[i].Conditions {
			status.Parents[i].Conditions[j].ObservedGeneration = grpcRoute.Generation
		}
	}
	if reflect.DeepEqual(grpcRoute.Status, status) {
		return false, nil
	}
	grpcRoute.Status = status
	return true, nil
}
