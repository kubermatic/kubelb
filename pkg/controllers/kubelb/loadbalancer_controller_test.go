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
	"reflect"
	"time"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	envoycp "k8c.io/kubelb/pkg/envoy"
	"k8c.io/kubelb/pkg/kubelb"

	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("TcpLb deployment and service creation", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		tcpLbName      = "serious-application"
		tcpLbNamespace = "default"

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	var lookupKey = types.NamespacedName{Name: tcpLbName, Namespace: tcpLbNamespace}
	var ctx = context.Background()

	Context("When creating a LoadBalancer", func() {
		var tcpLb = GetDefaultLoadBalancer(tcpLbName, tcpLbNamespace)
		It("Should create an envoy deployment", func() {

			Expect(k8sClient.Create(ctx, tcpLb)).Should(Succeed())

			By("creating a new deployment")

			createdDeployment := &appsv1.Deployment{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookupKey, createdDeployment)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeployment.Spec.Template.Spec.Containers[0].Args[1]).Should(Equal(envoyServer.GenerateBootstrap()))
			Expect(createdDeployment.OwnerReferences[0].Name).Should(Equal(tcpLbName))

			By("creating a corresponding service")

			createdService := &corev1.Service{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookupKey, createdService)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeployment.Spec.Template.Labels[kubelb.LabelAppKubernetesName]).Should(Equal(createdService.Spec.Selector[kubelb.LabelAppKubernetesName]))
			Expect(createdService.OwnerReferences[0].Name).Should(Equal(tcpLbName))

			By("creating an envoy snapshot")

			snapshot, err := envoyServer.Cache.GetSnapshot(tcpLbName)
			Expect(err).ToNot(HaveOccurred())

			Expect(reflect.DeepEqual(snapshot, envoycp.MapSnapshot(tcpLb, "0.0.1"))).To(BeTrue())

		})
	})

	Context("When updating an existing LoadBalancers Ports", func() {

		It("Should update the load balancer service and envoy snapshot", func() {

			existingTcpLb := &kubelbk8ciov1alpha1.LoadBalancer{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookupKey, existingTcpLb)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			//Make sure we have only 1 port in the existing LoadBalancer
			Expect(len(existingTcpLb.Spec.Ports)).To(BeEquivalentTo(1))

			existingTcpLb.Spec.Ports[0].Name = "port-a"
			existingTcpLb.Spec.Endpoints[0].Ports[0].Name = "port-a"

			existingTcpLb.Spec.Ports = append(existingTcpLb.Spec.Ports, kubelbk8ciov1alpha1.LoadBalancerPort{
				Name: "port-b",
				Port: 81,
			})

			existingTcpLb.Spec.Endpoints[0].Ports = append(existingTcpLb.Spec.Endpoints[0].Ports, kubelbk8ciov1alpha1.EndpointPort{
				Name: "port-b",
				Port: 8081,
			})

			//Todo: this should actually fail on update if there is no corresponding endpoint port set,
			//as well as a name to map those. Go ahead with admission webhooks
			Expect(k8sClient.Update(ctx, existingTcpLb)).Should(Succeed())

			By("updating the service ports")

			updatedService := &corev1.Service{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookupKey, updatedService)
				if err != nil {
					return false
				}

				for _, servicePort := range updatedService.Spec.Ports {
					if servicePort.Port == 81 {
						return true
					}
				}

				return false
			}, timeout, interval).Should(BeTrue())

			Expect(updatedService.Spec.Ports[0].Name).To(Equal("port-a"))
			Expect(updatedService.Spec.Ports[1].Name).To(Equal("port-b"))
			Expect(updatedService.Spec.Ports[1].Port).To(Equal(int32(81)))

			By("updating the envoy listener")

			snapshot, err := envoyServer.Cache.GetSnapshot(tcpLbName)
			Expect(err).ToNot(HaveOccurred())
			Expect(reflect.DeepEqual(snapshot, envoycp.MapSnapshot(existingTcpLb, "1.0.0"))).To(BeTrue())

			listener := snapshot.GetResources(resource.ListenerType)

			Expect(len(listener)).To(BeEquivalentTo(2))
			/*
				aListenerAny, err := ptypes.MarshalAny(listener["port-a"])
				Expect(err).ToNot(HaveOccurred())
				aListener := &listenerv3.Listener{}
				err = ptypes.UnmarshalAny(aListenerAny, aListener)
				Expect(err).ToNot(HaveOccurred())

				Expect(aListener.Name).To(Equal("port-a"))

				socketAddress := aListener.Address.Address.(*envoyCore.Address_SocketAddress)
				socketPortValue := socketAddress.SocketAddress.PortSpecifier.(*envoyCore.SocketAddress_PortValue)
				Expect(socketPortValue.PortValue).To(Equal(uint32(80)))

				bListenerAny, err := ptypes.MarshalAny(listener["port-b"])
				Expect(err).ToNot(HaveOccurred())
				bListener := &listenerv3.Listener{}
				err = ptypes.UnmarshalAny(bListenerAny, bListener)
				Expect(err).ToNot(HaveOccurred())

				Expect(bListener.Name).To(Equal("port-b"))

				socketAddress = bListener.Address.Address.(*envoyCore.Address_SocketAddress)
				socketPortValue = socketAddress.SocketAddress.PortSpecifier.(*envoyCore.SocketAddress_PortValue)
				Expect(socketPortValue.PortValue).To(Equal(uint32(81)))

				By("updating the envoy cluster")

				cluster := snapshot.GetResources(resource.ClusterType)

				ClusterAny, err := ptypes.MarshalAny(cluster["default-port-a"])
				Expect(err).ToNot(HaveOccurred())
				envoyCluster := &clusterv3.Cluster{}
				err = ptypes.UnmarshalAny(ClusterAny, envoyCluster)
				Expect(err).ToNot(HaveOccurred())
				clusterLbEndpoint := envoyCluster.LoadAssignment.Endpoints[0].LbEndpoints[0].HostIdentifier.(*endpointv3.LbEndpoint_Endpoint)
				socketAddress = clusterLbEndpoint.Endpoint.Address.Address.(*envoyCore.Address_SocketAddress)
				socketPortValue = socketAddress.SocketAddress.PortSpecifier.(*envoyCore.SocketAddress_PortValue)

				Expect(socketPortValue.PortValue).To(Equal(uint32(8080)))

				ClusterAny, err = ptypes.MarshalAny(cluster["default-port-b"])
				Expect(err).ToNot(HaveOccurred())
				envoyCluster = &clusterv3.Cluster{}
				err = ptypes.UnmarshalAny(ClusterAny, envoyCluster)
				Expect(err).ToNot(HaveOccurred())
				clusterLbEndpoint = envoyCluster.LoadAssignment.Endpoints[0].LbEndpoints[0].HostIdentifier.(*endpointv3.LbEndpoint_Endpoint)
				socketAddress = clusterLbEndpoint.Endpoint.Address.Address.(*envoyCore.Address_SocketAddress)
				socketPortValue = socketAddress.SocketAddress.PortSpecifier.(*envoyCore.SocketAddress_PortValue)

				Expect(socketPortValue.PortValue).To(Equal(uint32(8081)))
			*/
		})
	})
})
