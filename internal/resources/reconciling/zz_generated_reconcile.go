/*
Copyright 2026 The KubeLB Authors.

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
package reconciling

import (
	"context"
	"fmt"

	"k8c.io/reconciler/pkg/reconciling"
	"k8s.io/apimachinery/pkg/types"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// CustomResourceDefinitionReconciler defines an interface to create/update CustomResourceDefinitions.
type CustomResourceDefinitionReconciler = func(existing *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error)

// NamedCustomResourceDefinitionReconcilerFactory returns the name of the resource and the corresponding Reconciler function.
type NamedCustomResourceDefinitionReconcilerFactory = func() (name string, reconciler CustomResourceDefinitionReconciler)

// CustomResourceDefinitionObjectWrapper adds a wrapper so the CustomResourceDefinitionReconciler matches ObjectReconciler.
// This is needed as Go does not support function interface matching.
func CustomResourceDefinitionObjectWrapper(reconciler CustomResourceDefinitionReconciler) reconciling.ObjectReconciler {
	return func(existing ctrlruntimeclient.Object) (ctrlruntimeclient.Object, error) {
		if existing != nil {
			return reconciler(existing.(*apiextensionsv1.CustomResourceDefinition))
		}
		return reconciler(&apiextensionsv1.CustomResourceDefinition{})
	}
}

// ReconcileCustomResourceDefinitions will create and update the CustomResourceDefinitions coming from the passed CustomResourceDefinitionReconciler slice.
func ReconcileCustomResourceDefinitions(ctx context.Context, namedFactories []NamedCustomResourceDefinitionReconcilerFactory, namespace string, client ctrlruntimeclient.Client, objectModifiers ...reconciling.ObjectModifier) error {
	for _, factory := range namedFactories {
		name, reconciler := factory()
		reconcileObject := CustomResourceDefinitionObjectWrapper(reconciler)
		reconcileObject = reconciling.CreateWithNamespace(reconcileObject, namespace)
		reconcileObject = reconciling.CreateWithName(reconcileObject, name)

		for _, objectModifier := range objectModifiers {
			reconcileObject = objectModifier(reconcileObject)
		}

		if err := reconciling.EnsureNamedObject(ctx, types.NamespacedName{Namespace: namespace, Name: name}, reconcileObject, client, &apiextensionsv1.CustomResourceDefinition{}, false); err != nil {
			return fmt.Errorf("failed to ensure CustomResourceDefinition %s/%s: %w", namespace, name, err)
		}
	}

	return nil
}
