package kubelb

import (
	"github.com/go-logr/logr"
	"k8c.io/kubelb/manager/pkg/api/globalloadbalancer/v1alpha1"
	kubelbClient "k8c.io/kubelb/manager/pkg/generated/clientset/versioned"
	kubelbv1alpha1 "k8c.io/kubelb/manager/pkg/generated/clientset/versioned/typed/globalloadbalancer/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
)

type GlobalLoadBalancerClient struct {
	kubelbv1alpha1.GlobalLoadBalancerInterface
	Log              logr.Logger
	clusterName      string
	clusterEndpoints []string
}

func NewConnector(nodeList *corev1.NodeList, namespace string) (*GlobalLoadBalancerClient, error) {

	kubeconfig := filepath.Join(
		os.Getenv("HOME"), ".kube", "kubelb",
	)
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubelbClient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	//Todo: why is explicit typing needed
	var kubeLbAlpha1Clientset kubelbv1alpha1.KubelbV1alpha1Interface
	kubeLbAlpha1Clientset = clientset.KubelbV1alpha1()

	client := kubeLbAlpha1Clientset.GlobalLoadBalancers(namespace)

	var clusterEndpoints []string

	//Todo: This is not reliable probably use env and set manual
	for _, node := range nodeList.Items {
		var internalIp string
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				internalIp = address.Address
			}
		}

		clusterEndpoints = append(clusterEndpoints, internalIp)
	}

	return &GlobalLoadBalancerClient{
		GlobalLoadBalancerInterface: client,
		clusterName:                 namespace,
		clusterEndpoints:            clusterEndpoints,
	}, nil

}

func (k *GlobalLoadBalancerClient) CreateL4GlobalLoadBalancer(userService corev1.Service) error {

	var lbServicePorts []v1alpha1.LoadBalancerPort
	var lbEndpointSubsets []v1alpha1.LoadBalancerEndpointSubset
	var lbEndpointPorts []v1alpha1.LoadBalancerEndpointPort

	//mapping into load balancing service and endpoint subset ports
	for _, port := range userService.Spec.Ports {
		lbServicePorts = append(lbServicePorts, v1alpha1.LoadBalancerPort{
			//Todo: use annotation to set the lb port
			Port:       80,
			Protocol:   port.Protocol,
			TargetPort: port.NodePort,
		})

		lbEndpointPorts = append(lbEndpointPorts, v1alpha1.LoadBalancerEndpointPort{
			Port:     port.NodePort,
			Protocol: port.Protocol,
		})

	}

	var endpointAddresses []v1alpha1.LoadBalancerEndpointAddress
	for _, endpoint := range k.clusterEndpoints {
		endpointAddresses = append(endpointAddresses, v1alpha1.LoadBalancerEndpointAddress{
			IP: endpoint,
		})
	}

	lbEndpointSubsets = append(lbEndpointSubsets, v1alpha1.LoadBalancerEndpointSubset{
		Addresses: endpointAddresses,
		Ports:     lbEndpointPorts,
	})

	_, err := k.Create(&v1alpha1.GlobalLoadBalancer{
		ObjectMeta: v1.ObjectMeta{
			Name:      userService.Name,
			Namespace: k.clusterName,
		},
		Spec: v1alpha1.GlobalLoadBalancerSpec{
			Type:    v1alpha1.Layer4,
			Ports:   lbServicePorts,
			Subsets: lbEndpointSubsets,
		},
	})

	return err
}
