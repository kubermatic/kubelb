package kubelb

import (
	kubelbClient "k8c.io/kubelb/manager/pkg/generated/clientset/versioned"
	kubelbv1alpha1 "k8c.io/kubelb/manager/pkg/generated/clientset/versioned/typed/kubelb.k8c.io/v1alpha1"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
)

type Client struct {
	TcpLbClient  kubelbv1alpha1.TCPLoadBalancerInterface
	HttpLbClient kubelbv1alpha1.HTTPLoadBalancerInterface
	Clientset    *kubelbClient.Clientset
	Namespace    string
}

//Client for the KubeLb kubernetes Custer.
func NewClient(clusterName string, kubeConfPath string) (*Client, error) {

	var kubeconfig string
	if kubeConfPath != "" {
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

	var kubeLbAlpha1Clientset kubelbv1alpha1.KubelbV1alpha1Interface
	kubeLbAlpha1Clientset = clientset.KubelbV1alpha1()

	tcpLBClient := kubeLbAlpha1Clientset.TCPLoadBalancers(clusterName)
	httpLBClient := kubeLbAlpha1Clientset.HTTPLoadBalancers(clusterName)

	return &Client{
		TcpLbClient:  tcpLBClient,
		HttpLbClient: httpLBClient,
		Clientset:    clientset,
		Namespace:    clusterName,
	}, nil

}
