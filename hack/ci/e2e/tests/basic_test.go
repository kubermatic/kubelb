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
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	. "github.com/onsi/gomega"

	"k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	kubelbv1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"

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
	RegisterTestingT(t)
	ctx := context.Background()

	svc := sampleAppService()
	Expect(tenant1K8sClient.Create(ctx, &svc)).To(Succeed())
	defer tenant1K8sClient.Delete(ctx, &svc)

	deployment := sampleAppDeployment()
	Expect(tenant1K8sClient.Create(ctx, &deployment)).To(Succeed())
	defer tenant1K8sClient.Delete(ctx, &deployment)

	ip := expectServiceIP(ctx, tenant1K8sClient, client.ObjectKeyFromObject(&svc))
	testServiceURL := fmt.Sprintf("http://%v", ip)
	expectHTTPGet(testServiceURL)

	lb := v1alpha1.LoadBalancer{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "cluster-tenant1", Name: string(svc.UID)}, &lb)).To(Succeed())
	Expect(len(lb.Spec.Endpoints)).To(Equal(1))
	Expect(len(lb.Spec.Endpoints[0].Addresses)).To(Equal(1))
	Expect(len(lb.Spec.Endpoints[0].Ports)).To(Equal(1))
}

func TestMultiNodeService(t *testing.T) {
	RegisterTestingT(t)
	ctx := context.Background()

	svc := sampleAppService()
	Expect(tenant2K8sClient.Create(ctx, &svc)).To(Succeed())
	defer tenant2K8sClient.Delete(ctx, &svc)

	deployment := sampleAppDeployment()
	Expect(tenant2K8sClient.Create(ctx, &deployment)).To(Succeed())
	defer tenant2K8sClient.Delete(ctx, &deployment)

	ip := expectServiceIP(ctx, tenant2K8sClient, client.ObjectKeyFromObject(&svc))
	testServiceURL := fmt.Sprintf("http://%v", ip)
	expectHTTPGet(testServiceURL)

	lb := v1alpha1.LoadBalancer{}
	Expect(kubelbK8sClient.Get(ctx, types.NamespacedName{Namespace: "cluster-tenant2", Name: string(svc.UID)}, &lb)).To(Succeed())
	Expect(len(lb.Spec.Endpoints)).To(Equal(1))
	Expect(len(lb.Spec.Endpoints[0].Addresses)).To(Equal(4))
	Expect(len(lb.Spec.Endpoints[0].Ports)).To(Equal(1))
}
