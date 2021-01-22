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

package manager

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8c.io/kubelb/pkg/envoy"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2/klogr"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var envoyServer *envoy.Server

const APIVersion = "kubelb.k8c.io/v1alpha1"
const Kind = "TcpLoadBalancer"

func TestTcpLoadBalancerCustomResource(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"TcpLoadBalancer controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.UseDevMode(false)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = kubelbk8ciov1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	ctrl.SetLogger(klogr.New())
	ctx := ctrl.SetupSignalHandler()

	envoyServer, err = envoy.NewServer(":8001", true)

	Expect(err).ToNot(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&TCPLoadBalancerReconciler{
		Client:         k8sManager.GetClient(),
		Cache:          k8sManager.GetCache(),
		Scheme:         k8sManager.GetScheme(),
		EnvoyCache:     envoyServer.Cache,
		EnvoyBootstrap: envoyServer.GenerateBootstrap(),
	}).SetupWithManager(k8sManager, ctx)
	Expect(err).ToNot(HaveOccurred())

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func GetDefaultTcpLoadBalancer(name string, namespace string) *kubelbk8ciov1alpha1.TCPLoadBalancer {
	return &kubelbk8ciov1alpha1.TCPLoadBalancer{
		TypeMeta: v1.TypeMeta{
			APIVersion: APIVersion,
			Kind:       Kind,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: kubelbk8ciov1alpha1.TCPLoadBalancerSpec{
			Endpoints: []kubelbk8ciov1alpha1.LoadBalancerEndpoints{
				{
					Addresses: []kubelbk8ciov1alpha1.EndpointAddress{
						{
							IP: "123.123.123.123",
						},
					},
					Ports: []kubelbk8ciov1alpha1.EndpointPort{
						{
							Port: 8080,
						},
					}},
			},
			Ports: []kubelbk8ciov1alpha1.LoadBalancerPort{
				{
					Port: 80,
				},
			},
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}
}
