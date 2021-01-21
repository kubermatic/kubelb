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
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	envoycp "k8c.io/kubelb/pkg/envoy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"time"
)

var _ = Describe("TcpLb deployment and service creation", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		tcpLbName      = "app1"
		tcpLbNamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating a TcpLoadBalancer", func() {

		var tcpLb = &kubelbk8ciov1alpha1.TCPLoadBalancer{
			TypeMeta: v1.TypeMeta{
				APIVersion: APIVersion,
				Kind:       Kind,
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      tcpLbName,
				Namespace: tcpLbNamespace,
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
								Port: 9090,
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

		It("Should create an envoy deployment with a matching service", func() {

			ctx := context.Background()
			Expect(k8sClient.Create(ctx, tcpLb)).Should(Succeed())

			By("creating a new deployment")

			deploymentLookupKey := types.NamespacedName{Name: tcpLbName, Namespace: tcpLbNamespace}
			createdDeployment := &appsv1.Deployment{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deploymentLookupKey, createdDeployment)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeployment.Spec.Template.Spec.Containers[0].Args[1]).Should(Equal(envoyServer.GenerateBootstrap()))
			Expect(createdDeployment.OwnerReferences[0].Name).Should(Equal(tcpLbName))

			By("creating a corresponding service")

			serviceLookupKey := types.NamespacedName{Name: tcpLbName, Namespace: tcpLbNamespace}
			createdService := &corev1.Service{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, serviceLookupKey, createdService)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeployment.Spec.Template.Labels["app"]).Should(Equal(createdService.Spec.Selector["app"]))
			Expect(createdService.OwnerReferences[0].Name).Should(Equal(tcpLbName))

			By("creating an envoy snapshot")

			snapshot, err := envoyServer.Cache.GetSnapshot(tcpLbName)
			Expect(err).ToNot(HaveOccurred())

			Expect(reflect.DeepEqual(snapshot, envoycp.MapSnapshot(tcpLb, "0.0.1"))).To(BeTrue())

		})
	})
})
