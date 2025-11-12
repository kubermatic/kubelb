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

package kubelb

import (
	"context"

	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	configpkg "k8c.io/kubelb/internal/config"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func GetTenantAndConfig(ctx context.Context, client ctrlclient.Client, configNamespace, tenantName string) (*kubelbv1alpha1.Tenant, *kubelbv1alpha1.Config, error) {
	tenant, err := GetTenant(ctx, client, tenantName)
	if err != nil {
		return nil, nil, err
	}
	config, err := GetConfig(ctx, client, configNamespace)
	if err != nil {
		return nil, nil, err
	}

	return tenant, config, nil
}

func GetTenant(ctx context.Context, client ctrlclient.Client, tenantName string) (*kubelbv1alpha1.Tenant, error) {
	tenant := &kubelbv1alpha1.Tenant{}
	if err := client.Get(ctx, ctrlclient.ObjectKey{
		Name: tenantName,
	}, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

func GetConfig(ctx context.Context, client ctrlclient.Client, configNamespace string) (*kubelbv1alpha1.Config, error) {
	config := &kubelbv1alpha1.Config{}
	if err := client.Get(ctx, ctrlclient.ObjectKey{
		Namespace: configNamespace,
		Name:      configpkg.DefaultConfigResourceName,
	}, config); err != nil {
		return nil, err
	}
	return config, nil
}

func GetAnnotations(tenant *kubelbv1alpha1.Tenant, config *kubelbv1alpha1.Config) kubelbv1alpha1.AnnotationSettings {
	var annotations kubelbv1alpha1.AnnotationSettings
	if config.Spec.PropagateAllAnnotations != nil && *config.Spec.PropagateAllAnnotations {
		annotations.PropagateAllAnnotations = config.Spec.PropagateAllAnnotations
	} else if config.Spec.PropagatedAnnotations != nil {
		annotations.PropagatedAnnotations = config.Spec.PropagatedAnnotations
	}

	// Tenant configuration has higher precedence than the annotations specified at the Config level.
	if tenant.Spec.PropagateAllAnnotations != nil && *tenant.Spec.PropagateAllAnnotations {
		annotations.PropagateAllAnnotations = tenant.Spec.PropagateAllAnnotations
	} else if tenant.Spec.PropagatedAnnotations != nil {
		annotations.PropagatedAnnotations = tenant.Spec.PropagatedAnnotations
	}

	return annotations
}
