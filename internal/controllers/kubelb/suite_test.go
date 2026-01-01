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
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/envoy"
	"k8c.io/kubelb/internal/kubelb"
	portlookup "k8c.io/kubelb/internal/port-lookup"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg         *rest.Config
	k8sClient   client.Client
	ctx         context.Context
	cancel      context.CancelFunc
	testEnv     *envtest.Environment
	envoyServer *envoy.Server
	lbr         *LoadBalancerReconciler
	ecpr        *EnvoyCPReconciler
)

const (
	APIVersion  = "kubelb.k8c.io/v1alpha1"
	Kind        = "LoadBalancer"
	LBNamespace = "tenant-uno"
	Tenant      = "uno"
)

func TestLoadBalancerCustomResource(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "LoadBalancer controller Suite")
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

	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))
	sigCtx := ctrl.SetupSignalHandler()
	ctx, cancel = context.WithCancel(sigCtx)

	envoyServer, err = envoy.NewServer(&v1alpha1.Config{}, ":8001", true)

	Expect(err).ToNot(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	portAllocator := portlookup.NewPortAllocator()
	err = portAllocator.LoadState(ctx, k8sManager.GetAPIReader())
	Expect(err).ToNot(HaveOccurred())

	ns := &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: LBNamespace,
			Labels: map[string]string{
				kubelb.LabelManagedBy: kubelb.LabelControllerName,
			},
		},
	}

	tenant := &v1alpha1.Tenant{
		ObjectMeta: v1.ObjectMeta{
			Name: Tenant,
		},
	}

	config := &v1alpha1.Config{
		ObjectMeta: v1.ObjectMeta{
			Name:      "default",
			Namespace: LBNamespace,
		},
	}

	err = k8sManager.GetClient().Create(ctx, tenant)
	Expect(err).ToNot(HaveOccurred())

	err = k8sManager.GetClient().Create(ctx, ns)
	Expect(err).ToNot(HaveOccurred())
	err = k8sManager.GetClient().Create(ctx, config)
	Expect(err).ToNot(HaveOccurred())

	lbr = &LoadBalancerReconciler{
		Client:             k8sManager.GetClient(),
		Cache:              k8sManager.GetCache(),
		Scheme:             k8sManager.GetScheme(),
		EnvoyProxyTopology: EnvoyProxyTopologyShared,
		Namespace:          LBNamespace,
		PortAllocator:      portAllocator,
	}
	err = lbr.SetupWithManager(ctx, k8sManager)
	Expect(err).ToNot(HaveOccurred())

	ecpr = &EnvoyCPReconciler{
		Client:             k8sManager.GetClient(),
		EnvoyCache:         envoyServer.Cache,
		EnvoyProxyTopology: EnvoyProxyTopologyShared,
		EnvoyServer:        envoyServer,
		Namespace:          LBNamespace,
		PortAllocator:      portAllocator,
	}
	err = ecpr.SetupWithManager(ctx, k8sManager)
	Expect(err).ToNot(HaveOccurred())

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()
	Expect(k8sManager.GetCache().WaitForCacheSync(ctx)).To(BeTrue())
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func GetDefaultLoadBalancer(name string, namespace string) *v1alpha1.LoadBalancer {
	return &v1alpha1.LoadBalancer{
		TypeMeta: v1.TypeMeta{
			APIVersion: APIVersion,
			Kind:       Kind,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.LoadBalancerSpec{
			Endpoints: []v1alpha1.LoadBalancerEndpoints{
				{
					Addresses: []v1alpha1.EndpointAddress{
						{
							IP: "123.123.123.123",
						},
						{
							IP: "123.123.123.124",
						},
					},
					Ports: []v1alpha1.EndpointPort{
						{
							Port:     8080,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
			Ports: []v1alpha1.LoadBalancerPort{
				{
					Port:     80,
					Protocol: corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}
}
