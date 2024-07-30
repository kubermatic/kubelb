/*
Copyright 2024 The KubeLB Authors.

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

package service

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	"k8c.io/kubelb/internal/kubelb"
	portlookup "k8c.io/kubelb/internal/port-lookup"
	"k8c.io/reconciler/pkg/equality"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	NodePortServicePattern = "%s-nodeport"
)

func NormalizeAndReplicateServices(ctx context.Context, log logr.Logger, client ctrlclient.Client, references []types.NamespacedName) ([]corev1.Service, error) {
	var services []corev1.Service
	var servicesToCreateUpdate []corev1.Service
	for _, serviceRef := range references {
		log.Info("Normalizing service", "name", serviceRef.Name, "namespace", serviceRef.Namespace)

		service := &corev1.Service{}
		if err := client.Get(ctx, ctrlclient.ObjectKey{Namespace: serviceRef.Namespace, Name: serviceRef.Name}, service); err != nil {
			if kerrors.IsNotFound(err) {
				log.V(4).Info("service not found", "service", serviceRef)
				continue
			}
			return nil, fmt.Errorf("failed to get service %s/%s: %w", serviceRef.Namespace, serviceRef.Name, err)
		}

		if service.Spec.Type != corev1.ServiceTypeClusterIP {
			// We can use the service as is
			services = append(services, *cleanseService(*service, false, true))
		} else {
			// We need to create a new service with NodePort type
			uid := service.UID
			service = cleanseService(*service, true, false)
			service.Spec.Type = corev1.ServiceTypeNodePort

			// Add owner reference for the original service so that this service gets deleted when the original service is deleted
			service.SetOwnerReferences([]metav1.OwnerReference{
				{
					APIVersion: service.APIVersion,
					Kind:       service.Kind,
					Name:       service.Name,
					UID:        uid,
				},
			})

			if service.Labels == nil {
				service.Labels = make(map[string]string)
			}
			service.Labels[kubelb.LabelOriginNamespace] = service.Namespace
			service.Labels[kubelb.LabelOriginName] = service.Name
			service.Labels[kubelb.LabelManagedBy] = kubelb.LabelControllerName

			service.Name = fmt.Sprintf(NodePortServicePattern, service.Name)
			servicesToCreateUpdate = append(servicesToCreateUpdate, *service)
		}
	}

	for _, svc := range servicesToCreateUpdate {
		log.V(4).Info("Creating/Updating service", "name", svc.Name, "namespace", svc.Namespace)
		if err := CreateOrUpdateService(ctx, client, &svc); err != nil {
			return nil, fmt.Errorf("failed to create or update Service: %w", err)
		}
	}

	// We need to fetch the newly created services to get the NodePort
	for _, svc := range servicesToCreateUpdate {
		service := &corev1.Service{}
		if err := client.Get(ctx, ctrlclient.ObjectKey{Namespace: svc.Namespace, Name: svc.Name}, service); err != nil {
			if kerrors.IsNotFound(err) {
				log.V(4).Info("service not found", "service", svc)
				continue
			}
			return nil, fmt.Errorf("failed to get service %s/%s: %w", svc.Namespace, svc.Name, err)
		}
		services = append(services, *cleanseService(*service, false, true))
	}

	return services, nil
}

func GenerateServiceForLBCluster(service corev1.Service, appName, namespace, resourceNamespace string, portAllocator *portlookup.PortAllocator) corev1.Service {
	endpointKey := fmt.Sprintf(kubelb.EnvoyEndpointRoutePattern, namespace, service.Namespace, service.Name)

	service.Name = kubelb.GenerateName(false, string(service.UID), GetServiceName(service), service.Namespace)
	service.Namespace = resourceNamespace
	service.UID = ""
	if service.Spec.Type == corev1.ServiceTypeNodePort {
		service.Spec.Type = corev1.ServiceTypeClusterIP
	}

	// Use the nodePort(s) assigned as the targetPort for the new service. This is required to route traffic back to the actual service.
	for i, port := range service.Spec.Ports {
		portKey := fmt.Sprintf(kubelb.EnvoyListenerPattern, port.Port, port.Protocol)
		targetPort := port.NodePort
		if value, exists := portAllocator.Lookup(endpointKey, portKey); exists {
			targetPort = int32(value)
		}

		port.TargetPort = intstr.FromInt(int(targetPort))
		port.NodePort = 0
		service.Spec.Ports[i] = port
	}

	// Replace the selector with the envoy proxy selector.
	service.Spec.Selector = map[string]string{
		kubelb.LabelAppKubernetesName: appName,
	}

	return service
}

func CreateOrUpdateService(ctx context.Context, client ctrlclient.Client, obj *corev1.Service) error {
	key := ctrlclient.ObjectKey{Namespace: obj.Namespace, Name: obj.Name}
	existingObj := &corev1.Service{}
	if err := client.Get(ctx, key, existingObj); err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get Service: %w", err)
		}
		err := client.Create(ctx, obj)
		if err != nil {
			return fmt.Errorf("failed to create Service: %w", err)
		}
		return nil
	}

	// Update the Service object if it is different from the existing one.
	if equality.Semantic.DeepEqual(existingObj.Spec, obj.Spec) &&
		equality.Semantic.DeepEqual(existingObj.Labels, obj.Labels) &&
		equality.Semantic.DeepEqual(existingObj.Annotations, obj.Annotations) {
		return nil
	}

	if err := client.Update(ctx, obj); err != nil {
		return fmt.Errorf("failed to update Service: %w", err)
	}
	return nil
}

func GetServiceName(service corev1.Service) string {
	name := service.Name
	if labels := service.Labels; labels != nil {
		if _, ok := labels[kubelb.LabelOriginName]; ok {
			name = service.Labels[kubelb.LabelOriginName]
		}
	}
	return name
}

func GetServiceNamespace(service corev1.Service) string {
	namespace := service.Namespace
	if labels := service.Labels; labels != nil {
		if _, ok := labels[kubelb.LabelOriginNamespace]; ok {
			namespace = service.Labels[kubelb.LabelOriginNamespace]
		}
	}
	return namespace
}

func cleanseService(svc corev1.Service, removeUID, removeClusterSpecificFields bool) *corev1.Service {
	obj := &corev1.Service{
		TypeMeta: svc.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:        svc.Name,
			Namespace:   svc.Namespace,
			Labels:      svc.Labels,
			Annotations: svc.Annotations,
			UID:         svc.UID,
		},
		Spec: svc.Spec,
	}

	annotations := obj.GetAnnotations()
	delete(annotations, corev1.LastAppliedConfigAnnotation)
	obj.SetAnnotations(annotations)

	if obj.Spec.ClusterIP != corev1.ClusterIPNone {
		obj.Spec.ClusterIP = ""
		obj.Spec.ClusterIPs = nil
	}

	if removeUID {
		obj.SetUID("")
	}

	if removeClusterSpecificFields {
		obj.Spec.IPFamilies = nil
		obj.Spec.IPFamilyPolicy = nil
		obj.Spec.SessionAffinity = ""
		obj.Spec.ExternalTrafficPolicy = ""
		obj.Spec.InternalTrafficPolicy = nil
	}
	return obj
}
