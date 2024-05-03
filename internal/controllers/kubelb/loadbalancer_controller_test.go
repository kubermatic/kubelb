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
	"fmt"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/go-test/deep"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	envoycp "k8c.io/kubelb/internal/envoy"
	"k8c.io/kubelb/internal/kubelb"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

var testMatrix = []struct {
	topology                 EnvoyProxyTopology
	envoyProxyDeploymentName func(lb types.NamespacedName) types.NamespacedName
	envoyProxyServiceName    func(lb types.NamespacedName) types.NamespacedName
	envoySnapshotName        func(lb types.NamespacedName) string
}{
	{
		topology: EnvoyProxyTopologyShared,
		envoyProxyDeploymentName: func(lb types.NamespacedName) types.NamespacedName {
			return types.NamespacedName{Name: fmt.Sprintf(envoyResourcePattern, lb.Name), Namespace: LBNamespace}
		},
		envoyProxyServiceName: func(lb types.NamespacedName) types.NamespacedName {
			return types.NamespacedName{Name: fmt.Sprintf(envoyResourcePattern, lb.Name), Namespace: LBNamespace}
		},
		envoySnapshotName: func(lb types.NamespacedName) string { return lb.Namespace },
	},
	{
		topology: EnvoyProxyTopologyDedicated,
		envoyProxyDeploymentName: func(lb types.NamespacedName) types.NamespacedName {
			return types.NamespacedName{Name: fmt.Sprintf(envoyResourcePattern, lb.Name), Namespace: lb.Namespace}
		},
		envoyProxyServiceName: func(lb types.NamespacedName) types.NamespacedName {
			return types.NamespacedName{Name: fmt.Sprintf(envoyResourcePattern, lb.Name), Namespace: lb.Namespace}
		},
		envoySnapshotName: func(lb types.NamespacedName) string { return fmt.Sprintf("%s-%s", lb.Namespace, lb.Name) },
	},
	{
		topology: EnvoyProxyTopologyGlobal,
		envoyProxyDeploymentName: func(lb types.NamespacedName) types.NamespacedName {
			return types.NamespacedName{Name: "envoy-global", Namespace: LBNamespace}
		},
		envoyProxyServiceName: func(lb types.NamespacedName) types.NamespacedName {
			return types.NamespacedName{Name: fmt.Sprintf(envoyGlobalTopologyServicePattern, lb.Namespace, lb.Name), Namespace: LBNamespace}
		},
		envoySnapshotName: func(lb types.NamespacedName) string { return "global" },
	},
}

var _ = Describe("Lb deployment and service creation", func() {
	for _, t := range testMatrix {
		// Define utility constants for object names and testing timeouts/durations and intervals.
		const (
			lbName      = "serious-application"
			lbNamespace = LBNamespace

			timeout  = time.Second * 10
			interval = time.Millisecond * 250
		)
		var (
			deploymentLookupKey types.NamespacedName
			serviceLookupKey    types.NamespacedName
			snapshotName        string
		)

		lbLookupKey := types.NamespacedName{Name: lbName, Namespace: lbNamespace}
		ctx := context.Background()

		Context(fmt.Sprintf("When creating a LoadBalancer with %v topology", t.topology), func() {
			lb := GetDefaultLoadBalancer(lbName, lbNamespace)
			It("Configures the reconcilers", func() {
				lbr.EnvoyProxyTopology = t.topology
				ecpr.EnvoyProxyTopology = t.topology
				deploymentLookupKey = t.envoyProxyDeploymentName(lbLookupKey)
				serviceLookupKey = t.envoyProxyServiceName(lbLookupKey)
				snapshotName = t.envoySnapshotName(lbLookupKey)
			})

			It("Should create an envoy deployment", func() {
				Expect(k8sClient.Create(ctx, lb)).Should(Succeed())

				By("creating a new deployment")

				createdDeployment := &appsv1.Deployment{}

				Eventually(func() error {
					return k8sClient.Get(ctx, deploymentLookupKey, createdDeployment)
				}, timeout, interval).Should(Succeed())

				Expect(createdDeployment.Spec.Template.Spec.Containers[0].Args[1]).Should(Equal(envoyServer.GenerateBootstrap()))

				By("creating a corresponding service")

				createdService := &corev1.Service{}

				Eventually(func() error {
					return k8sClient.Get(ctx, serviceLookupKey, createdService)
				}, timeout, interval).Should(Succeed())

				Expect(createdDeployment.Spec.Template.Labels[kubelb.LabelAppKubernetesName]).Should(Equal(createdService.Spec.Selector[kubelb.LabelAppKubernetesName]))

				By("creating an envoy snapshot")

				snapshot, err := envoyServer.Cache.GetSnapshot(snapshotName)
				Expect(err).ToNot(HaveOccurred())

				testSnapshot, err := envoycp.MapSnapshot(getLoadBalancerList(*lb), ecpr.PortAllocator, t.topology == EnvoyProxyTopologyGlobal)
				Expect(err).ToNot(HaveOccurred())
				diff := deep.Equal(snapshot, testSnapshot)
				if len(diff) > 0 {
					fmt.Printf("expected snapshot didn't match generated snapshot, diff: %+v", diff)
				}
				Expect(len(diff)).To(Equal(0))
			})
		})

		Context(fmt.Sprintf("When updating an existing LoadBalancers Ports with %v topology", t.topology), func() {
			It("Should update the load balancer service and envoy snapshot", func() {
				existingLb := &kubelbk8ciov1alpha1.LoadBalancer{}

				Eventually(func() error {
					return k8sClient.Get(ctx, lbLookupKey, existingLb)
				}, timeout, interval).Should(Succeed())

				// Make sure we have only 1 port in the existing LoadBalancer
				Expect(len(existingLb.Spec.Ports)).To(BeEquivalentTo(1))

				existingLb.Spec.Ports[0].Name = "port-a"
				existingLb.Spec.Endpoints[0].Ports[0].Name = "port-a"

				existingLb.Spec.Ports = append(existingLb.Spec.Ports, kubelbk8ciov1alpha1.LoadBalancerPort{
					Name: "port-b",
					Port: 81,
				})

				existingLb.Spec.Endpoints[0].Ports = append(existingLb.Spec.Endpoints[0].Ports, kubelbk8ciov1alpha1.EndpointPort{
					Name: "port-b",
					Port: 8081,
				})

				// Todo: this should actually fail on update if there is no corresponding endpoint port set,
				// as well as a name to map those. Go ahead with admission webhooks
				Expect(k8sClient.Update(ctx, existingLb)).Should(Succeed())

				By("updating the service ports")

				updatedService := &corev1.Service{}

				Eventually(func() bool {
					err := k8sClient.Get(ctx, serviceLookupKey, updatedService)
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

				snapshot, err := envoyServer.Cache.GetSnapshot(snapshotName)
				Expect(err).ToNot(HaveOccurred())

				testSnapshot, err := envoycp.MapSnapshot(getLoadBalancerList(*existingLb), ecpr.PortAllocator, t.topology == EnvoyProxyTopologyGlobal)
				Expect(err).ToNot(HaveOccurred())
				diff := deep.Equal(snapshot, testSnapshot)
				if len(diff) > 0 {
					fmt.Printf("expected snapshot didn't match generated snapshot, diff: %+v", diff)
				}
				Expect(len(diff)).To(Equal(0))

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

		Context(fmt.Sprintf("When all loadbalancers get deleted with %v topology", t.topology), func() {
			lb := GetDefaultLoadBalancer(lbName, lbNamespace)
			It("Should garbage collect all managed resources", func() {
				Expect(k8sClient.Delete(ctx, lb)).Should(Succeed())

				Eventually(func() bool {
					deployment := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, deploymentLookupKey, deployment)
					return kerrors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())

				Eventually(func() bool {
					service := &corev1.Service{}
					err := k8sClient.Get(ctx, serviceLookupKey, service)
					return kerrors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			})
		})
	}
})

func getLoadBalancerList(lb kubelbk8ciov1alpha1.LoadBalancer) []kubelbk8ciov1alpha1.LoadBalancer {
	return []kubelbk8ciov1alpha1.LoadBalancer{lb}
}
