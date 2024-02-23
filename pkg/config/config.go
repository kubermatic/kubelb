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

package config

import (
	"context"

	"k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultConfigResourceName represents name of default config CR
	DefaultConfigResourceName string = "default"
)

var config v1alpha1.Config

func LoadConfig(ctx context.Context, apiReader client.Reader, namespace string) error {
	conf := v1alpha1.Config{}
	if err := apiReader.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      DefaultConfigResourceName,
	}, &conf); err != nil {
		return err
	}
	config = conf
	return nil
}

func GetConfig() v1alpha1.Config {
	return config
}

func SetConfig(conf v1alpha1.Config) {
	config = conf
}

func GetEnvoyProxyTopology() v1alpha1.EnvoyProxyTopology {
	return config.Spec.EnvoyProxy.Topology
}
