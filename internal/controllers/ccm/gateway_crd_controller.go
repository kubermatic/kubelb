/*
Copyright 2025 The KubeLB Authors.

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
	"path/filepath"

	"github.com/go-logr/logr"

	crds "k8c.io/kubelb/internal/resources/crds"
	"k8c.io/kubelb/internal/resources/reconciling"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	GatewayCRDControllerName = "gateway-crd-controller"
)

type CRDMap map[string]*apiextensionsv1.CustomResourceDefinition

type GatewayCRDReconciler struct {
	client.Client
	Log            logr.Logger
	Channel        GatewayAPIChannel
	GatewayAPICRDs CRDMap
}

// GatewayAPIChannel represents the channel of Gateway API CRDs
type GatewayAPIChannel string

const (
	ChannelStandard     GatewayAPIChannel = "standard"
	ChannelExperimental GatewayAPIChannel = "experimental"
)

func IsValidGatewayAPICRDsChannel(channel string) bool {
	switch GatewayAPIChannel(channel) {
	case ChannelStandard, ChannelExperimental:
		return true
	default:
		return false
	}
}

// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update
func (r *GatewayCRDReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if err := r.reconcileCRD(ctx, req.Name); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *GatewayCRDReconciler) reconcileCRD(ctx context.Context, name string) error {
	var crd *apiextensionsv1.CustomResourceDefinition
	var source string

	if c, exists := r.GatewayAPICRDs[name]; exists {
		crd = c
		source = "Gateway API"
	} else {
		return fmt.Errorf("CRD %s not found in Gateway API or Envoy Gateway CRDs", name)
	}

	r.Log.Info("Reconciling CRD", "name", name, "source", source)

	reconcilers := []reconciling.NamedCustomResourceDefinitionReconcilerFactory{}
	reconcilers = append(reconcilers, r.crdReconciler(name, crd))

	if err := reconciling.ReconcileCustomResourceDefinitions(ctx, reconcilers, "", r.Client); err != nil {
		return fmt.Errorf("failed to reconcile CRD %s: %w", name, err)
	}

	return nil
}

func (r *GatewayCRDReconciler) crdReconciler(name string, source *apiextensionsv1.CustomResourceDefinition) reconciling.NamedCustomResourceDefinitionReconcilerFactory {
	return func() (string, reconciling.CustomResourceDefinitionReconciler) {
		return name, func(crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error) {
			crd.Labels = source.Labels
			crd.Annotations = source.Annotations
			crd.Spec = source.Spec

			// reconcile fails if conversion is not set as it's set by default to None
			crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{Strategy: apiextensionsv1.NoneConverter}

			return crd, nil
		}
	}
}

func loadGatewayAPICRDs(channel GatewayAPIChannel) (CRDMap, error) {
	crdDir, err := getGatewayAPICRDsPath(channel)
	if err != nil {
		return nil, fmt.Errorf("failed to get Gateway API CRDs directory: %w", err)
	}

	crdFiles, err := crds.FS.ReadDir(crdDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read CRDs directory: %w", err)
	}

	var gatewayCRDs = make(CRDMap, len(crdFiles))
	for _, crdFile := range crdFiles {
		filename := crdFile.Name()

		crd, err := parseCRDFile(crdDir, filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read CRD %s: %w", filename, err)
		}

		// Use CRD name as the key
		gatewayCRDs[crd.Name] = crd
	}

	return gatewayCRDs, nil
}

func getGatewayAPICRDsPath(channel GatewayAPIChannel) (string, error) {
	switch channel {
	case ChannelStandard:
		return crds.StandardGatewayAPICRDsDir, nil
	case ChannelExperimental:
		return crds.ExperimentalGatewayAPICRDsDir, nil
	default:
		return "", fmt.Errorf("unsupported Gateway API channel: %s", channel)
	}
}

func parseCRDFile(dir, filename string) (*apiextensionsv1.CustomResourceDefinition, error) {
	content, err := crds.FS.Open(filepath.Join(dir, filename))
	if err != nil {
		return nil, err
	}
	defer content.Close()

	crd := &apiextensionsv1.CustomResourceDefinition{}
	dec := yaml.NewYAMLOrJSONDecoder(content, 1024)
	if err := dec.Decode(crd); err != nil {
		return nil, err
	}

	return crd, nil
}

func (r *GatewayCRDReconciler) resourceFilter() predicate.Predicate {
	isRelevantGroup := func(group string) bool {
		return group == gwapiv1.GroupName
	}

	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return false // Only reconcile on update and delete
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCRD, okOld := e.ObjectOld.(*apiextensionsv1.CustomResourceDefinition)
			newCRD, okNew := e.ObjectNew.(*apiextensionsv1.CustomResourceDefinition)
			return okOld && okNew &&
				isRelevantGroup(oldCRD.Spec.Group) &&
				isRelevantGroup(newCRD.Spec.Group)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			crd, ok := e.Object.(*apiextensionsv1.CustomResourceDefinition)
			return ok && isRelevantGroup(crd.Spec.Group)
		},
	}
}

func (r *GatewayCRDReconciler) SetupWithManager(mgr ctrl.Manager) error {
	crdMap, err := loadGatewayAPICRDs(r.Channel)
	if err != nil {
		return fmt.Errorf("failed to load Gateway API CRDs for channel %s: %w", r.Channel, err)
	}
	r.GatewayAPICRDs = crdMap

	bootstrapping := make(chan event.GenericEvent, len(r.GatewayAPICRDs))
	for name := range r.GatewayAPICRDs {
		bootstrapping <- event.GenericEvent{
			Object: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			},
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(GatewayCRDControllerName).
		For(&apiextensionsv1.CustomResourceDefinition{}, builder.WithPredicates(predicate.And(predicate.GenerationChangedPredicate{},
			r.resourceFilter()))).
		WatchesRawSource(
			source.Channel(bootstrapping, &handler.EnqueueRequestForObject{}),
		).Complete(r)
}

// InstallCRDs installs Gateway API and Envoy Gateway CRDs synchronously.
// Call this before mgr.Start() to ensure CRDs exist before controllers watch them.
func InstallCRDs(ctx context.Context, c client.Client, log logr.Logger, channel GatewayAPIChannel) error {
	gatewayAPICRDs, err := loadGatewayAPICRDs(channel)
	if err != nil {
		return fmt.Errorf("failed to load Gateway API CRDs: %w", err)
	}

	// Install all CRDs
	allCRDs := make([]reconciling.NamedCustomResourceDefinitionReconcilerFactory, 0, len(gatewayAPICRDs))
	for name, crd := range gatewayAPICRDs {
		allCRDs = append(allCRDs, crdReconcilerFactory(name, crd))
	}

	log.Info("Installing CRDs", "gatewayAPI", len(gatewayAPICRDs))
	if err := reconciling.ReconcileCustomResourceDefinitions(ctx, allCRDs, "", c); err != nil {
		return fmt.Errorf("failed to install CRDs: %w", err)
	}

	log.Info("CRDs installed successfully")
	return nil
}

func crdReconcilerFactory(name string, source *apiextensionsv1.CustomResourceDefinition) reconciling.NamedCustomResourceDefinitionReconcilerFactory {
	return func() (string, reconciling.CustomResourceDefinitionReconciler) {
		return name, func(crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error) {
			crd.Labels = source.Labels
			crd.Annotations = source.Annotations
			crd.Spec = source.Spec
			crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{Strategy: apiextensionsv1.NoneConverter}
			return crd, nil
		}
	}
}
