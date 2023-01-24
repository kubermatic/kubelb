/*
Copyright 2020 The KubeLB Authors.

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
	"os"
	"path/filepath"

	kubelbClient "k8c.io/kubelb/pkg/generated/clientset/versioned"
	kubelbv1alpha1 "k8c.io/kubelb/pkg/generated/clientset/versioned/typed/kubelb.k8c.io/v1alpha1"

	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	TcpLbClient kubelbv1alpha1.LoadBalancerInterface
	Clientset   *kubelbClient.Clientset
	Namespace   string
}

// Client for the KubeLb kubernetes Custer.
func NewClient(clusterName string, kubeConfPath string) (*Client, error) {

	var kubeconfig string
	if kubeConfPath == "" {
		kubeconfig = filepath.Join(
			os.Getenv("HOME"), ".kube", "kubelb",
		)
	} else {
		kubeconfig = kubeConfPath
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubelbClient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	kubeLbAlpha1Clientset := clientset.KubelbV1alpha1()

	tcpLBClient := kubeLbAlpha1Clientset.LoadBalancers(clusterName)

	return &Client{
		TcpLbClient: tcpLBClient,
		Clientset:   clientset,
		Namespace:   clusterName,
	}, nil

}
