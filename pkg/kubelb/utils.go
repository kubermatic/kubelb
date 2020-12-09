package kubelb

import (
	"fmt"
	"github.com/go-logr/logr"
	kubelbk8ciov1alpha1 "k8c.io/kubelb/pkg/api/kubelb.k8c.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"strings"
)

const LabelOriginNamespace = "kubelb.k8c.io/origin-ns"
const LabelOriginName = "kubelb.k8c.io/origin-name"

var _ handler.Mapper = &TcpLbMapper{}

type TcpLbMapper struct {
	ClusterName string
	Log         logr.Logger
}

// Map enqueues a request per each iems at each node event.
func (sm *TcpLbMapper) Map(m handler.MapObject) []ctrl.Request {

	if m.Meta.GetNamespace() != sm.ClusterName {
		return []ctrl.Request{}
	}

	tcpLb := m.Object.(*kubelbk8ciov1alpha1.TCPLoadBalancer)

	if tcpLb.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return []ctrl.Request{}
	}

	originalNamespace, ok := m.Meta.GetLabels()[LabelOriginNamespace]
	if !ok || originalNamespace == "" {
		sm.Log.Error(fmt.Errorf("required label \"%s\" not found", LabelOriginNamespace), fmt.Sprintf("failed to queue service for TcpLoadBalacner: %s, could not determine origin namespace", m.Meta.GetName()))
		return []ctrl.Request{}
	}

	originalName, ok := m.Meta.GetLabels()[LabelOriginName]
	if !ok || originalName == "" {
		sm.Log.Error(fmt.Errorf("required label \"%s\" not found", LabelOriginName), fmt.Sprintf("failed to queue service for TcpLoadBalacner: %s, could not determine origin name", m.Meta.GetName()))
		return []ctrl.Request{}
	}

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      originalName,
				Namespace: originalNamespace,
			},
		},
	}
}

func NamespacedName(obj *metav1.ObjectMeta) string {
	return strings.Join([]string{obj.Namespace, obj.Name}, "-")
}
