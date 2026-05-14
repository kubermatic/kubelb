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
	ingressHelpers "k8c.io/kubelb/internal/resources/ingress"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"
)

const (
	IngressControllerName = "ingress-controller"
	IngressClassName      = "kubelb"
	IngressGVK            = "Ingress.networking.k8s.io"
)

// IngressReconciler reconciles an Ingress Object
type IngressReconciler struct {
	ctrlclient.Client

	LBManager       ctrl.Manager
	ClusterName     string
	UseIngressClass bool

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder

	loop *SourceReconciler[*networkingv1.Ingress]
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/status,verbs=get;update;patch

func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.loop.Reconcile(ctx, req)
}

func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.loop = &SourceReconciler[*networkingv1.Ingress]{
		Client:      r.Client,
		LBClient:    r.LBManager.GetClient(),
		ClusterName: r.ClusterName,
		Log:         r.Log,
		Adapter:     &ingressAdapter{useClass: r.UseIngressClass},
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named(IngressControllerName).
		For(&networkingv1.Ingress{}, builder.WithPredicates(r.loop.Predicate())).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.loop.EnqueueForService()),
		).
		WatchesRawSource(
			source.Kind(r.LBManager.GetCache(), &kubelbv1alpha1.Route{},
				handler.TypedEnqueueRequestsFromMapFunc[*kubelbv1alpha1.Route](enqueueRoutes(IngressGVK, r.ClusterName))),
		).
		Complete(r)
}

type ingressAdapter struct {
	useClass bool
}

var _ SourceAdapter[*networkingv1.Ingress] = (*ingressAdapter)(nil)

func (a *ingressAdapter) Kind() string                     { return "Ingress" }
func (a *ingressAdapter) GVK() string                      { return IngressGVK }
func (a *ingressAdapter) NewObject() *networkingv1.Ingress { return &networkingv1.Ingress{} }
func (a *ingressAdapter) NewList() ctrlclient.ObjectList   { return &networkingv1.IngressList{} }

func (a *ingressAdapter) Metrics() SourceMetrics {
	return SourceMetrics{
		ReconcileTotal:    ccmmetrics.IngressReconcileTotal,
		ReconcileDuration: ccmmetrics.IngressReconcileDuration,
		Managed:           ccmmetrics.ManagedIngressesTotal,
	}
}

func (a *ingressAdapter) ShouldReconcile(ingress *networkingv1.Ingress) bool {
	if a.useClass {
		return ingress.Spec.IngressClassName != nil && *ingress.Spec.IngressClassName == IngressClassName
	}
	return true
}

func (a *ingressAdapter) ExtractServices(ingress *networkingv1.Ingress) []types.NamespacedName {
	return ingressHelpers.GetServicesFromIngress(*ingress)
}

func (a *ingressAdapter) ApplyRouteStatus(ingress *networkingv1.Ingress, raw []byte) (bool, error) {
	status := networkingv1.IngressStatus{}
	if err := yaml.UnmarshalStrict(raw, &status); err != nil {
		return false, fmt.Errorf("failed to unmarshal Ingress status: %w", err)
	}
	if reflect.DeepEqual(ingress.Status, status) {
		return false, nil
	}
	ingress.Status = status
	return true, nil
}
