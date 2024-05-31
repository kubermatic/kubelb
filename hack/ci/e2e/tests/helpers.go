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
	"log"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func testInit(t *testing.T) context.Context {
	RegisterTestingT(t)
	ctx := context.Background()
	t.Parallel()
	return ctx
}

func getK8sClient(path string) client.Client {
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{
			ExplicitPath: path,
		},
		&clientcmd.ConfigOverrides{},
	)
	cli, err := cfg.ClientConfig()
	if err != nil {
		panic(err)
	}
	kcfg, err := client.New(cli, client.Options{})
	if err != nil {
		panic(err)
	}
	return kcfg
}

func sampleAppService(testID string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      testID,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 80, TargetPort: intstr.IntOrString{IntVal: 80}},
			},
			Selector: map[string]string{
				"sample-app": testID,
			},
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}
}

func twoAppService(testID string, port1, port2 int32) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      testID,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "nginx", Port: port1, TargetPort: intstr.IntOrString{IntVal: 80}},
				{Name: "envoy", Port: port2, TargetPort: intstr.IntOrString{IntVal: 9901}},
			},
			Selector: map[string]string{
				"two-app": testID,
			},
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}
}

func sampleAppDeployment(testID string) appsv1.Deployment {
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      testID,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"sample-app": testID,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"sample-app": testID,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.24.0",
						},
					},
				},
			},
		},
	}
}

func twoAppDeployment(testID string) appsv1.Deployment {
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      testID,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"two-app": testID,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"two-app": testID,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.24.0",
						},
						{
							Name:  "envoy",
							Image: "envoyproxy/envoy:distroless-v1.30.1",
						},
					},
				},
			},
		},
	}
}

func expectServiceIP(ctx context.Context, cl client.Client, n types.NamespacedName) string {
	svc := corev1.Service{}
	Eventually(func() error {
		err := cl.Get(ctx, n, &svc)
		if err != nil {
			return err
		}
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			return fmt.Errorf("missing loadbalancer ingress")
		}
		if svc.Status.LoadBalancer.Ingress[0].IP == "" {
			return fmt.Errorf("missing loadbalancer ingress IP")
		}
		return nil
	}).WithTimeout(10 * time.Second).Should(Succeed())
	return svc.Status.LoadBalancer.Ingress[0].IP
}

func expectHTTPGet(url, serverHeader string) {
	var respServerHeader string
	Eventually(func() error {
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("failed %v: %v", url, err)
			return err
		}
		if resp.StatusCode != http.StatusOK {
			log.Printf("wrong status %v: %v", url, resp.StatusCode)
			return fmt.Errorf("expected status %v but got %v", http.StatusOK, resp.StatusCode)
		}
		log.Printf("SUCCESS %v", url)
		respServerHeader = resp.Header.Get("server")
		return nil
	}).WithTimeout(30 * time.Minute).WithPolling(1 * time.Second).Should(Succeed())
	Expect(respServerHeader).To(Equal(serverHeader))
}
