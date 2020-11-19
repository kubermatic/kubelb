package kubelb

import (
	"github.com/go-logr/logr"
	kubelbClient "k8c.io/kubelb/manager/pkg/generated/clientset/versioned"
	kubelbv1alpha1 "k8c.io/kubelb/manager/pkg/generated/clientset/versioned/typed/kubelb.k8c.io/v1alpha1"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
)

type HttpLBClient struct {
	kubelbv1alpha1.HTTPLoadBalancerInterface
	Log         logr.Logger
	clusterName string
}

func NewHttpLBClient(clusterName string) (*HttpLBClient, error) {

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

	client := kubeLbAlpha1Clientset.HTTPLoadBalancers(clusterName)

	return &HttpLBClient{
		HTTPLoadBalancerInterface: client,
		clusterName:               clusterName,
	}, nil

}
