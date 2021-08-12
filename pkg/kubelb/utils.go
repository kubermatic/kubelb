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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const LabelOriginNamespace = "kubelb.k8c.io/origin-ns"
const LabelOriginName = "kubelb.k8c.io/origin-name"

const LabelAppKubernetesName = "app.kubernetes.io/name"            //mysql
const LabelAppKubernetesInstance = "app.kubernetes.io/instance"    //mysql-abcxzy"
const LabelAppKubernetesVersion = "app.kubernetes.io/version"      //5.7.21
const LabelAppKubernetesComponent = "app.kubernetes.io/component"  // database
const LabelAppKubernetesPartOf = "app.kubernetes.io/part-of"       //wordpress
const LabelAppKubernetesManagedBy = "app.kubernetes.io/managed-by" //helm

func NamespacedName(obj *metav1.ObjectMeta) string {
	return strings.Join([]string{obj.Namespace, obj.Name}, "-")
}
