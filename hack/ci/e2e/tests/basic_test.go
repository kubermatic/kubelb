/*
Copyright 2023 The KubeLB Authors.

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

package main

import (
	"fmt"
	"os"
	"path"
	"sync"
	"testing"

	. "github.com/onsi/gomega"

	"k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	kubelbK8sClient  client.Client
	tenant1K8sClient client.Client
	tenant2K8sClient client.Client
)

func TestMain(m *testing.M) {
	err := kubelbv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}
	kubeconfigDir := os.Getenv("KUBECONFIGS_DIR")
	kubelbK8sClient = getK8sClient(path.Join(kubeconfigDir, "kubelb.kubeconfig"))
	tenant1K8sClient = getK8sClient(path.Join(kubeconfigDir, "tenant1.kubeconfig"))
	tenant2K8sClient = getK8sClient(path.Join(kubeconfigDir, "tenant2.kubeconfig"))
	code := m.Run()
	os.Exit(code)
}

func TestSimpleService(t *testing.T) {
	ctx := testInit(t)

	svc := sampleAppService("simple")
	Expect(tenant1K8sClient.Create(ctx, &svc)).To(Succeed())
	defer tenant1K8sClient.Delete(ctx, &svc)

	deployment := sampleAppDeployment("simple")
	Expect(tenant1K8sClient.Create(ctx, &deployment)).To(Succeed())
	defer tenant1K8sClient.Delete(ctx, &deployment)

	ip := expectServiceIP(ctx, tenant1K8sClient, client.ObjectKeyFromObject(&svc))
	testServiceURL := fmt.Sprintf("http://%v", ip)
	expectHTTPGet(testServiceURL, "nginx/1.24.0")

	lb := v1alpha1.LoadBalancer{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-primary", Name: string(svc.UID)}, &lb)).To(Succeed())
	Expect(len(lb.Spec.Endpoints)).To(Equal(1))
	Expect(len(lb.Spec.Endpoints[0].Ports)).To(Equal(1))

	Expect(lb.Spec.Endpoints[0].AddressesReference).ToNot(BeNil())
	// Retrieve the endpoint addresses and make sure they are correct
	addresses := v1alpha1.Addresses{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-primary", Name: string(*&lb.Spec.Endpoints[0].AddressesReference.Name)}, &addresses)).To(Succeed())
	Expect(len(addresses.Spec.Addresses)).To(Equal(1))
}

func TestMultiNodeService(t *testing.T) {
	ctx := testInit(t)

	svc := sampleAppService("multi-node")
	Expect(tenant2K8sClient.Create(ctx, &svc)).To(Succeed())
	defer tenant2K8sClient.Delete(ctx, &svc)

	deployment := sampleAppDeployment("multi-node")
	Expect(tenant2K8sClient.Create(ctx, &deployment)).To(Succeed())
	defer tenant2K8sClient.Delete(ctx, &deployment)

	ip := expectServiceIP(ctx, tenant2K8sClient, client.ObjectKeyFromObject(&svc))
	testServiceURL := fmt.Sprintf("http://%v", ip)
	expectHTTPGet(testServiceURL, "nginx/1.24.0")

	lb := v1alpha1.LoadBalancer{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-secondary", Name: string(svc.UID)}, &lb)).To(Succeed())
	Expect(len(lb.Spec.Endpoints)).To(Equal(1))
	Expect(len(lb.Spec.Endpoints[0].Ports)).To(Equal(1))

	Expect(lb.Spec.Endpoints[0].AddressesReference).ToNot(BeNil())
	// Retrieve the endpoint addresses and make sure they are correct
	addresses := v1alpha1.Addresses{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-secondary", Name: string(*&lb.Spec.Endpoints[0].AddressesReference.Name)}, &addresses)).To(Succeed())
	Expect(len(addresses.Spec.Addresses)).To(Equal(4))
}

func TestMultiPortService(t *testing.T) {
	ctx := testInit(t)

	svc := twoAppService("multi-port", 80, 9901)
	Expect(tenant1K8sClient.Create(ctx, &svc)).To(Succeed())
	defer tenant1K8sClient.Delete(ctx, &svc)

	deployment := twoAppDeployment("multi-port")
	Expect(tenant1K8sClient.Create(ctx, &deployment)).To(Succeed())
	defer tenant1K8sClient.Delete(ctx, &deployment)

	ip := expectServiceIP(ctx, tenant1K8sClient, client.ObjectKeyFromObject(&svc))
	testServiceURL := fmt.Sprintf("http://%v", ip)
	expectHTTPGet(testServiceURL, "nginx/1.24.0")
	expectHTTPGet(fmt.Sprintf("%v:9901", testServiceURL), "envoy")

	lb := v1alpha1.LoadBalancer{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-primary", Name: string(svc.UID)}, &lb)).To(Succeed())
	Expect(len(lb.Spec.Endpoints)).To(Equal(1))
	Expect(len(lb.Spec.Endpoints[0].Ports)).To(Equal(2))

	Expect(lb.Spec.Endpoints[0].AddressesReference).ToNot(BeNil())
	// Retrieve the endpoint addresses and make sure they are correct
	addresses := v1alpha1.Addresses{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-secondary", Name: string(*&lb.Spec.Endpoints[0].AddressesReference.Name)}, &addresses)).To(Succeed())
	Expect(len(addresses.Spec.Addresses)).To(Equal(4))
}

func TestMultiPortMultiNodeService(t *testing.T) {
	ctx := testInit(t)

	svc := twoAppService("multi-port-and-node", 80, 9901)
	Expect(tenant2K8sClient.Create(ctx, &svc)).To(Succeed())
	defer tenant2K8sClient.Delete(ctx, &svc)

	deployment := twoAppDeployment("multi-port-and-node")
	Expect(tenant2K8sClient.Create(ctx, &deployment)).To(Succeed())
	defer tenant2K8sClient.Delete(ctx, &deployment)

	ip := expectServiceIP(ctx, tenant2K8sClient, client.ObjectKeyFromObject(&svc))
	testServiceURL := fmt.Sprintf("http://%v", ip)
	expectHTTPGet(testServiceURL, "nginx/1.24.0")
	expectHTTPGet(fmt.Sprintf("%v:9901", testServiceURL), "envoy")

	lb := v1alpha1.LoadBalancer{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-secondary", Name: string(svc.UID)}, &lb)).To(Succeed())
	Expect(len(lb.Spec.Endpoints)).To(Equal(1))
	Expect(len(lb.Spec.Endpoints[0].Ports)).To(Equal(2))

	Expect(lb.Spec.Endpoints[0].AddressesReference).ToNot(BeNil())
	// Retrieve the endpoint addresses and make sure they are correct
	addresses := v1alpha1.Addresses{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-secondary", Name: string(*&lb.Spec.Endpoints[0].AddressesReference.Name)}, &addresses)).To(Succeed())
	Expect(len(addresses.Spec.Addresses)).To(Equal(4))
}

func TestDuplicateService(t *testing.T) {
	ctx := testInit(t)

	svc1 := twoAppService("duplicate", 8080, 9090)
	Expect(tenant1K8sClient.Create(ctx, &svc1)).To(Succeed())
	defer tenant1K8sClient.Delete(ctx, &svc1)
	svc2 := twoAppService("duplicate", 9090, 8080)
	Expect(tenant2K8sClient.Create(ctx, &svc2)).To(Succeed())
	defer tenant2K8sClient.Delete(ctx, &svc2)

	deployment1 := twoAppDeployment("duplicate")
	Expect(tenant1K8sClient.Create(ctx, &deployment1)).To(Succeed())
	defer tenant1K8sClient.Delete(ctx, &deployment1)
	deployment2 := twoAppDeployment("duplicate")
	Expect(tenant2K8sClient.Create(ctx, &deployment2)).To(Succeed())
	defer tenant2K8sClient.Delete(ctx, &deployment2)

	ip1 := expectServiceIP(ctx, tenant1K8sClient, client.ObjectKeyFromObject(&svc1))
	testServiceURL1 := fmt.Sprintf("http://%v", ip1)
	expectHTTPGet(fmt.Sprintf("%v:8080", testServiceURL1), "nginx/1.24.0")
	expectHTTPGet(fmt.Sprintf("%v:9090", testServiceURL1), "envoy")

	ip2 := expectServiceIP(ctx, tenant2K8sClient, client.ObjectKeyFromObject(&svc2))
	testServiceURL2 := fmt.Sprintf("http://%v", ip2)
	expectHTTPGet(fmt.Sprintf("%v:9090", testServiceURL2), "nginx/1.24.0")
	expectHTTPGet(fmt.Sprintf("%v:8080", testServiceURL2), "envoy")

	lb1 := v1alpha1.LoadBalancer{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-primary", Name: string(svc1.UID)}, &lb1)).To(Succeed())
	Expect(len(lb1.Spec.Endpoints)).To(Equal(1))
	Expect(len(lb1.Spec.Endpoints[0].Ports)).To(Equal(2))

	Expect(lb1.Spec.Endpoints[0].AddressesReference).ToNot(BeNil())
	// Retrieve the endpoint addresses and make sure they are correct
	addresses := v1alpha1.Addresses{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-primary", Name: string(*&lb1.Spec.Endpoints[0].AddressesReference.Name)}, &addresses)).To(Succeed())
	Expect(len(addresses.Spec.Addresses)).To(Equal(1))

	lb2 := v1alpha1.LoadBalancer{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-secondary", Name: string(svc2.UID)}, &lb2)).To(Succeed())
	Expect(len(lb2.Spec.Endpoints)).To(Equal(1))
	Expect(len(lb2.Spec.Endpoints[0].Ports)).To(Equal(2))

	Expect(lb2.Spec.Endpoints[0].AddressesReference).ToNot(BeNil())
	// Retrieve the endpoint addresses and make sure they are correct
	addresses = v1alpha1.Addresses{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-secondary", Name: string(*&lb2.Spec.Endpoints[0].AddressesReference.Name)}, &addresses)).To(Succeed())
	Expect(len(addresses.Spec.Addresses)).To(Equal(4))
}

func TestMultipleServices(t *testing.T) {
	ctx := testInit(t)

	svc1 := twoAppService("multi-port-1", 80, 9901)
	Expect(tenant2K8sClient.Create(ctx, &svc1)).To(Succeed())
	defer tenant2K8sClient.Delete(ctx, &svc1)

	deployment1 := twoAppDeployment("multi-port-1")
	Expect(tenant2K8sClient.Create(ctx, &deployment1)).To(Succeed())
	defer tenant2K8sClient.Delete(ctx, &deployment1)

	svc2 := twoAppService("multi-port-2", 80, 9901)
	Expect(tenant2K8sClient.Create(ctx, &svc2)).To(Succeed())
	defer tenant2K8sClient.Delete(ctx, &svc2)

	deployment2 := twoAppDeployment("multi-port-2")
	Expect(tenant2K8sClient.Create(ctx, &deployment2)).To(Succeed())
	defer tenant2K8sClient.Delete(ctx, &deployment2)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		ip1 := expectServiceIP(ctx, tenant2K8sClient, client.ObjectKeyFromObject(&svc1))
		testServiceURL1 := fmt.Sprintf("http://%v", ip1)
		expectHTTPGet(fmt.Sprintf("%v:80", testServiceURL1), "nginx/1.24.0")
		expectHTTPGet(fmt.Sprintf("%v:9901", testServiceURL1), "envoy")
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		ip2 := expectServiceIP(ctx, tenant2K8sClient, client.ObjectKeyFromObject(&svc2))
		testServiceURL2 := fmt.Sprintf("http://%v", ip2)
		expectHTTPGet(fmt.Sprintf("%v:80", testServiceURL2), "nginx/1.24.0")
		expectHTTPGet(fmt.Sprintf("%v:9901", testServiceURL2), "envoy")
		wg.Done()
	}()
	wg.Wait()

	lb1 := v1alpha1.LoadBalancer{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-secondary", Name: string(svc1.UID)}, &lb1)).To(Succeed())
	Expect(len(lb1.Spec.Endpoints)).To(Equal(1))
	Expect(len(lb1.Spec.Endpoints[0].Ports)).To(Equal(2))

	Expect(lb1.Spec.Endpoints[0].AddressesReference).ToNot(BeNil())
	// Retrieve the endpoint addresses and make sure they are correct
	addresses := v1alpha1.Addresses{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-secondary", Name: string(*&lb1.Spec.Endpoints[0].AddressesReference.Name)}, &addresses)).To(Succeed())
	Expect(len(addresses.Spec.Addresses)).To(Equal(4))

	lb2 := v1alpha1.LoadBalancer{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-secondary", Name: string(svc2.UID)}, &lb2)).To(Succeed())
	Expect(len(lb2.Spec.Endpoints)).To(Equal(1))
	Expect(len(lb2.Spec.Endpoints[0].Ports)).To(Equal(2))

	Expect(lb2.Spec.Endpoints[0].AddressesReference).ToNot(BeNil())
	// Retrieve the endpoint addresses and make sure they are correct
	addresses = v1alpha1.Addresses{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "tenant-secondary", Name: string(*&lb2.Spec.Endpoints[0].AddressesReference.Name)}, &addresses)).To(Succeed())
	Expect(len(addresses.Spec.Addresses)).To(Equal(4))
}
