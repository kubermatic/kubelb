package lb

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kubelbiov1alpha1 "manager/pkg/api/v1alpha1"
	"os"
	"path/filepath"
)

type Manager struct {
	kubernetes.Clientset
}

func NewManager() (*Manager, error) {

	kubeconfig := filepath.Join(
		os.Getenv("HOME"), ".kube", "kubelb",
	)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Manager{
		Clientset: *clientset,
	}, nil

}

func (m *Manager) CreateL4LbService(glb *kubelbiov1alpha1.GlobalLoadBalancer) error {

	_, err := m.Clientset.CoreV1().Services(glb.Namespace).Create(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      glb.Name,
			Namespace: glb.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: glb.Spec.Ports,
			Type:  corev1.ServiceTypeLoadBalancer,
		},
	})

	if err != nil {
		return err
	}

	_, err = m.Clientset.CoreV1().Endpoints(glb.Namespace).Create(&corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      glb.Name,
			Namespace: glb.Namespace,
		},
		Subsets: glb.Spec.Subsets,
	})

	if err != nil {
		return err
	}

	return nil
}
