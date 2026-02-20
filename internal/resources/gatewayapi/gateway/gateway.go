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

package gateway

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/kubelb"
	k8sutils "k8c.io/kubelb/internal/util/kubernetes"

	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func CreateOrUpdateGateway(ctx context.Context, log logr.Logger, client ctrlclient.Client, object *gwapiv1.Gateway, namespace string, config *kubelbv1alpha1.Config, tenant *kubelbv1alpha1.Tenant,
	annotations kubelbv1alpha1.AnnotationSettings) error {
	// Transformations to make it compliant with the LB cluster.
	// Update the GatewayClass name to match the GatewayClass object in the LB cluster.
	var gatewayClassName *string
	if tenant.Spec.GatewayAPI.Class != nil {
		gatewayClassName = tenant.Spec.GatewayAPI.Class
	} else if config.Spec.GatewayAPI.Class != nil {
		gatewayClassName = config.Spec.GatewayAPI.Class
	}

	// Check if Gateway with the same name but different namespace already exists. If it does, log an error as we don't support
	// multiple Gateway objects.
	gateways := &gwapiv1.GatewayList{}
	if err := client.List(ctx, gateways, ctrlclient.InNamespace(namespace)); err != nil {
		return fmt.Errorf("failed to list Gateways: %w", err)
	}

	found := false
	for _, existingGateway := range gateways.Items {
		if existingGateway.Name == object.Name {
			found = true
			break
		}
	}

	if !found && len(gateways.Items) >= MaxGatewaysPerTenant {
		return fmt.Errorf("maximum of %d Gateway object(s) per tenant is supported", MaxGatewaysPerTenant)
	}

	// Process annotations.
	object.Annotations = kubelb.PropagateAnnotations(object.Annotations, annotations, kubelbv1alpha1.AnnotatedResourceGateway)

	// Process secrets.
	for i, listener := range object.Spec.Listeners {
		if listener.TLS != nil {
			for j, reference := range listener.TLS.CertificateRefs {
				secretName := k8sutils.GetSecretNameIfExists(ctx, client, string(reference.Name), object.Namespace, namespace)
				if secretName != "" {
					object.Spec.Listeners[i].TLS.CertificateRefs[j].Name = gwapiv1.ObjectName(secretName)
				}
				// cross namespace references are not allowed
				object.Spec.Listeners[i].TLS.CertificateRefs[j].Namespace = nil
			}
		}
	}

	// Process labels
	object.Labels = kubelb.AddKubeLBLabels(object.Labels, object.Name, object.Namespace, "")

	object.Namespace = namespace
	object.SetUID("") // Reset UID to generate a new UID for the Gateway object

	// Set the GatewayClassName if it is specified in the configuration.
	if gatewayClassName != nil {
		object.Spec.GatewayClassName = gwapiv1.ObjectName(*gatewayClassName)
	}

	log.V(4).Info("Creating/Updating Gateway", "name", object.Name, "namespace", object.Namespace)
	// Check if it already exists.
	gatewayKey := ctrlclient.ObjectKey{Namespace: object.Namespace, Name: object.Name}
	existingGateway := &gwapiv1.Gateway{}
	if err := client.Get(ctx, gatewayKey, existingGateway); err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get Gateway: %w", err)
		}
		err := client.Create(ctx, object)
		if err != nil {
			return fmt.Errorf("failed to create Gateway: %w", err)
		}
		return nil
	}

	// Merge the annotations with the existing annotations to allow annotations that are configured by third party controllers on the existing service to be preserved.
	object.Annotations = k8sutils.MergeAnnotations(existingGateway.Annotations, object.Annotations)

	// Update the Gateway object if it is different from the existing one.
	if equality.Semantic.DeepEqual(existingGateway.Spec, object.Spec) &&
		equality.Semantic.DeepEqual(existingGateway.Labels, object.Labels) &&
		k8sutils.CompareAnnotations(existingGateway.Annotations, object.Annotations) {
		return nil
	}

	// Required to update the object.
	object.ResourceVersion = existingGateway.ResourceVersion
	object.UID = existingGateway.UID

	if err := client.Update(ctx, object); err != nil {
		return fmt.Errorf("failed to update Gateway: %w", err)
	}
	return nil
}
