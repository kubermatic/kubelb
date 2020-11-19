package kubelb

import (
	"github.com/go-logr/logr"
	kubelbClient "k8c.io/kubelb/manager/pkg/generated/clientset/versioned"
	kubelbv1alpha1 "k8c.io/kubelb/manager/pkg/generated/clientset/versioned/typed/kubelb.k8c.io/v1alpha1"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
)

type TcpLBClient struct {
	kubelbv1alpha1.TCPLoadBalancerInterface
	Log         logr.Logger
	clusterName string
}

func NewTcpLBClient(clusterName string) (*TcpLBClient, error) {

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

	var kubeLbAlpha1Clientset kubelbv1alpha1.KubelbV1alpha1Interface
	kubeLbAlpha1Clientset = clientset.KubelbV1alpha1()

	client := kubeLbAlpha1Clientset.TCPLoadBalancers(clusterName)

	return &TcpLBClient{
		TCPLoadBalancerInterface: client,
		clusterName:              clusterName,
	}, nil

}
