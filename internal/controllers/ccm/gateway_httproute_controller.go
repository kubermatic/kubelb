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
	httprouteHelpers "k8c.io/kubelb/internal/resources/gatewayapi/httproute"

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
	GatewayHTTPRouteControllerName = "gateway-httproute-controller"
	HTTPRouteGVK                   = "HTTPRoute.gateway.networking.k8s.io"
)

// HTTPRouteReconciler reconciles an HTTPRoute Object
type HTTPRouteReconciler struct {
	ctrlclient.Client

	LBManager   ctrl.Manager
	ClusterName string

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder

	loop *SourceReconciler[*gwapiv1.HTTPRoute]
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/status,verbs=get;update;patch

func (r *HTTPRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.loop.Reconcile(ctx, req)
}

func (r *HTTPRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.loop = &SourceReconciler[*gwapiv1.HTTPRoute]{
		Client:      r.Client,
		LBClient:    r.LBManager.GetClient(),
		ClusterName: r.ClusterName,
		Log:         r.Log,
		Adapter:     &httpRouteAdapter{},
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named(GatewayHTTPRouteControllerName).
		For(&gwapiv1.HTTPRoute{}, builder.WithPredicates(r.loop.Predicate())).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.loop.EnqueueForService()),
		).
		WatchesRawSource(
			source.Kind(r.LBManager.GetCache(), &kubelbv1alpha1.Route{},
				handler.TypedEnqueueRequestsFromMapFunc[*kubelbv1alpha1.Route](enqueueRoutes(HTTPRouteGVK, r.ClusterName))),
		).
		Complete(r)
}

type httpRouteAdapter struct{}

var _ SourceAdapter[*gwapiv1.HTTPRoute] = (*httpRouteAdapter)(nil)

func (a *httpRouteAdapter) Kind() string                   { return "HTTPRoute" }
func (a *httpRouteAdapter) GVK() string                    { return HTTPRouteGVK }
func (a *httpRouteAdapter) NewObject() *gwapiv1.HTTPRoute  { return &gwapiv1.HTTPRoute{} }
func (a *httpRouteAdapter) NewList() ctrlclient.ObjectList { return &gwapiv1.HTTPRouteList{} }

func (a *httpRouteAdapter) Metrics() SourceMetrics {
	return SourceMetrics{
		ReconcileTotal:    ccmmetrics.HTTPRouteReconcileTotal,
		ReconcileDuration: ccmmetrics.HTTPRouteReconcileDuration,
		Managed:           ccmmetrics.ManagedHTTPRoutesTotal,
	}
}

func (a *httpRouteAdapter) ShouldReconcile(httpRoute *gwapiv1.HTTPRoute) bool {
	return gatewayhelper.ShouldReconcileResource(httpRoute, false)
}

func (a *httpRouteAdapter) ExtractServices(httpRoute *gwapiv1.HTTPRoute) []types.NamespacedName {
	return httprouteHelpers.GetServicesFromHTTPRoute(httpRoute)
}

func (a *httpRouteAdapter) ApplyRouteStatus(httpRoute *gwapiv1.HTTPRoute, raw []byte) (bool, error) {
	status := gwapiv1.HTTPRouteStatus{}
	if err := yaml.UnmarshalStrict(raw, &status); err != nil {
		return false, fmt.Errorf("failed to unmarshal HTTPRoute status: %w", err)
	}
	for i := range status.Parents {
		for j := range status.Parents[i].Conditions {
			status.Parents[i].Conditions[j].ObservedGeneration = httpRoute.Generation
		}
	}
	if reflect.DeepEqual(httpRoute.Status, status) {
		return false, nil
	}
	httpRoute.Status = status
	return true, nil
}
