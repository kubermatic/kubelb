/*
Copyright 2025 The KubeLB Authors.

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

package tunnel

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	kubelb_internal "k8c.io/kubelb/internal/kubelb"
	"k8c.io/kubelb/internal/resources"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	TunnelServiceName   = "tunnel-connection-manager"
	TunnelServicePort   = 8080
	TunnelOwnerLabel    = "kubelb.k8c.io/tunnel-owner"
	TunnelHostnameLabel = "kubelb.k8c.io/tunnel-hostname"
	TunnelSecretLabel   = "kubelb.k8c.io/tunnel-secret"
)

type Reconciler struct {
	client            ctrlruntimeclient.Client
	scheme            *runtime.Scheme
	recorder          record.EventRecorder
	disableGatewayAPI bool
}

func NewReconciler(client ctrlruntimeclient.Client, scheme *runtime.Scheme, recorder record.EventRecorder, disableGatewayAPI bool) *Reconciler {
	return &Reconciler{
		client:            client,
		scheme:            scheme,
		recorder:          recorder,
		disableGatewayAPI: disableGatewayAPI,
	}
}

// Reconcile reconciles a tunnel object and returns the hostname and route reference
func (r *Reconciler) Reconcile(ctx context.Context, log logr.Logger, tunnel *kubelbv1alpha1.Tunnel, config *kubelbv1alpha1.Config, tenant *kubelbv1alpha1.Tenant, annotations kubelbv1alpha1.AnnotationSettings) (*corev1.ObjectReference, error) {
	// Check if hostname should be configured using the same logic as LoadBalancer
	if !kubelb_internal.ShouldConfigureHostname(log, tunnel.Annotations, tunnel.Name, tunnel.Spec.Hostname, tenant, config) {
		return nil, fmt.Errorf("no hostname configurable")
	}

	// Assign a wildcard hostname if the annotation is set and the hostname is empty.
	hostname := tunnel.Status.Hostname
	if hostname == "" {
		if tunnel.Spec.Hostname != "" {
			hostname = tunnel.Spec.Hostname
		} else {
			hostname = kubelb_internal.GenerateHostname(tenant.Spec.DNS, config.Spec.DNS)
		}
	}

	if hostname == "" {
		// No need for an error here since we can still manage the tunnel and skip the hostname configuration.
		log.V(2).Info("no hostname configurable, skipping")
		return nil, fmt.Errorf("no hostname configurable for the tunnel")
	}

	oldStatus := tunnel.Status.DeepCopy()
	if tunnel.Status.Hostname == "" {
		tunnel.Status.Hostname = hostname
	}

	tunnel.Status.URL = fmt.Sprintf("https://%s", tunnel.Status.Hostname)
	tunnel.Status.ConnectionManagerURL = config.Spec.Tunnel.ConnectionManagerURL
	tunnel.Status.Resources.ServiceName = fmt.Sprintf("tunnel-envoy-%s", tunnel.Namespace)

	statusChanged := oldStatus.Hostname != tunnel.Status.Hostname ||
		oldStatus.URL != tunnel.Status.URL ||
		oldStatus.ConnectionManagerURL != tunnel.Status.ConnectionManagerURL ||
		oldStatus.Resources.ServiceName != tunnel.Status.Resources.ServiceName

	if statusChanged {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			latestTunnel := &kubelbv1alpha1.Tunnel{}
			if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Name: tunnel.Name, Namespace: tunnel.Namespace}, latestTunnel); err != nil {
				return err
			}
			latestTunnel.Status.Hostname = tunnel.Status.Hostname
			latestTunnel.Status.URL = tunnel.Status.URL
			latestTunnel.Status.ConnectionManagerURL = tunnel.Status.ConnectionManagerURL
			latestTunnel.Status.Resources.ServiceName = tunnel.Status.Resources.ServiceName
			return r.client.Status().Update(ctx, latestTunnel)
		})
		if err != nil {
			return nil, fmt.Errorf("failed to update tunnel status: %w", err)
		}
	}

	svcName := fmt.Sprintf("tunnel-envoy-%s", tunnel.Namespace)
	// NOTE: We only support shared topology for tunneling and global topology is not supported yet.
	appName := tunnel.Namespace
	if err := r.ensureTunnelService(ctx, log, tunnel, svcName, appName, annotations); err != nil {
		return nil, fmt.Errorf("failed to ensure tunnel service: %w", err)
	}

	// Create ingress or gateway based on configuration
	var (
		routeRef *corev1.ObjectReference
		err      error
	)
	useGatewayAPI := !tenant.Spec.GatewayAPI.Disable && !config.Spec.GatewayAPI.Disable &&
		(tenant.Spec.GatewayAPI.Class != nil || config.Spec.GatewayAPI.Class != nil)

	if !useGatewayAPI {
		routeRef, err = r.createOrUpdateIngress(ctx, tunnel, hostname, svcName, config, tenant, annotations)
	} else {
		routeRef, err = r.createOrUpdateHTTPRoute(ctx, tunnel, hostname, svcName, tenant, config, annotations)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create/update route for tunnel %s: %w", tunnel.Name, err)
	}

	return routeRef, nil
}

// Cleanup removes all resources created for the tunnel
func (r *Reconciler) Cleanup(ctx context.Context, tunnel *kubelbv1alpha1.Tunnel) error {
	// Delete ingress or httproute
	if tunnel.Status.Resources.RouteRef != nil {
		switch tunnel.Status.Resources.RouteRef.Kind {
		case "Ingress":
			ingress := &networkingv1.Ingress{}
			if err := r.client.Get(ctx, types.NamespacedName{
				Name:      tunnel.Status.Resources.RouteRef.Name,
				Namespace: tunnel.Status.Resources.RouteRef.Namespace,
			}, ingress); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to get ingress: %w", err)
				}
			} else {
				if err := r.client.Delete(ctx, ingress); err != nil && !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to delete ingress: %w", err)
				}
			}
		case "HTTPRoute":
			httpRoute := &gwapiv1.HTTPRoute{}
			if err := r.client.Get(ctx, types.NamespacedName{
				Name:      tunnel.Status.Resources.RouteRef.Name,
				Namespace: tunnel.Status.Resources.RouteRef.Namespace,
			}, httpRoute); err != nil {
				if !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to get httproute: %w", err)
				}
			} else {
				if err := r.client.Delete(ctx, httpRoute); err != nil && !kerrors.IsNotFound(err) {
					return fmt.Errorf("failed to delete httproute: %w", err)
				}
			}
		}
	}

	// Check if this is the last tunnel in the tenant before deleting shared service
	tunnelList := &kubelbv1alpha1.TunnelList{}
	if err := r.client.List(ctx, tunnelList, ctrlruntimeclient.InNamespace(tunnel.Namespace)); err != nil {
		return fmt.Errorf("failed to list tunnels in namespace: %w", err)
	}

	remainingTunnels := 0
	for _, t := range tunnelList.Items {
		if t.Name != tunnel.Name {
			remainingTunnels++
		}
	}

	// Only delete service if this is the last tunnel in the tenant
	if remainingTunnels == 0 {
		svcName := fmt.Sprintf("tunnel-envoy-%s", tunnel.Namespace)
		service := &corev1.Service{}
		if err := r.client.Get(ctx, types.NamespacedName{
			Name:      svcName,
			Namespace: tunnel.Namespace,
		}, service); err != nil {
			if !kerrors.IsNotFound(err) {
				return fmt.Errorf("failed to get tunnel service: %w", err)
			}
		} else {
			if err := r.client.Delete(ctx, service); err != nil && !kerrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete tunnel service: %w", err)
			}
		}
	}
	return nil
}

func (r *Reconciler) ensureTunnelService(ctx context.Context, log logr.Logger, tunnel *kubelbv1alpha1.Tunnel, svcName, appName string, annotations kubelbv1alpha1.AnnotationSettings) error {
	log.V(2).Info("ensuring tunnel service", "service", svcName)

	// Create labels for tenant-wide tunnel service
	labels := map[string]string{
		kubelb_internal.LabelAppKubernetesName: appName,
		"kubelb.k8c.io/service-type":           "tunnel-envoy",
		"kubelb.k8c.io/tenant":                 tunnel.Namespace,
	}

	// Create service that points to Envoy proxy
	desiredService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   tunnel.Namespace,
			Labels:      labels,
			Annotations: kubelb_internal.PropagateAnnotations(tunnel.Annotations, annotations, kubelbv1alpha1.AnnotatedResourceService),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{kubelb_internal.LabelAppKubernetesName: appName},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "https",
					Port:       443,
					TargetPort: intstr.FromInt(8443),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	// Get existing service
	existingService := &corev1.Service{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      svcName,
		Namespace: tunnel.Namespace,
	}, existingService)

	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}

	// If service doesn't exist, create it
	if kerrors.IsNotFound(err) {
		log.V(2).Info("creating tunnel service", "name", svcName)
		if err := r.client.Create(ctx, desiredService); err != nil {
			return fmt.Errorf("failed to create tunnel service: %w", err)
		}
	} else if !equality.Semantic.DeepEqual(existingService.Spec.Ports, desiredService.Spec.Ports) ||
		!equality.Semantic.DeepEqual(existingService.Spec.Selector, desiredService.Spec.Selector) ||
		!equality.Semantic.DeepEqual(existingService.Labels, desiredService.Labels) {
		// Update service if needed
		log.V(2).Info("updating tunnel service", "name", svcName)
		existingService.Spec = desiredService.Spec
		existingService.Labels = desiredService.Labels
		existingService.Annotations = desiredService.Annotations
		if err := r.client.Update(ctx, existingService); err != nil {
			return fmt.Errorf("failed to update tunnel service: %w", err)
		}
	}

	return nil
}

func (r *Reconciler) createOrUpdateIngress(ctx context.Context, tunnel *kubelbv1alpha1.Tunnel, hostname, serviceName string, config *kubelbv1alpha1.Config, tenant *kubelbv1alpha1.Tenant, annotations kubelbv1alpha1.AnnotationSettings) (*corev1.ObjectReference, error) {
	ingressName := fmt.Sprintf("tunnel-%s", tunnel.Name)

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: tunnel.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, ingress, func() error {
		if ingress.Labels == nil {
			ingress.Labels = make(map[string]string)
		}
		ingress.Labels[TunnelOwnerLabel] = tunnel.Name
		ingress.Labels[TunnelHostnameLabel] = hostname
		ingress.Annotations = kubelb_internal.PropagateAnnotations(tunnel.Annotations, annotations, kubelbv1alpha1.AnnotatedResourceIngress)

		if tenant.Spec.Certificates.DefaultClusterIssuer != nil {
			ingress.Annotations[resources.CertManagerClusterIssuerAnnotation] = *tenant.Spec.Certificates.DefaultClusterIssuer
		} else if config.Spec.Certificates.DefaultClusterIssuer != nil {
			ingress.Annotations[resources.CertManagerClusterIssuerAnnotation] = *config.Spec.Certificates.DefaultClusterIssuer
		}
		ingress.Annotations[resources.ExternalDNSHostnameAnnotation] = hostname
		ingress.Annotations[resources.ExternalDNSTTLAnnotation] = resources.ExternalDNSTTLDefault

		// Set ingress class if specified
		if config.Spec.Ingress.Class != nil {
			ingress.Spec.IngressClassName = ptr.To(*config.Spec.Ingress.Class)
		} else if tenant.Spec.Ingress.Class != nil {
			ingress.Spec.IngressClassName = ptr.To(*tenant.Spec.Ingress.Class)
		}
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{hostname},
				SecretName: fmt.Sprintf("tunnel-%s-tls", tunnel.Name),
			},
		}
		pathType := networkingv1.PathTypePrefix
		ingress.Spec.Rules = []networkingv1.IngressRule{
			{
				Host: hostname,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: &pathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: serviceName,
										Port: networkingv1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
						},
					},
				},
			},
		}
		// Set owner reference
		return controllerutil.SetControllerReference(tunnel, ingress, r.scheme)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create/update ingress: %w", err)
	}

	return &corev1.ObjectReference{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "Ingress",
		Name:       ingress.Name,
		Namespace:  ingress.Namespace,
		UID:        ingress.UID,
	}, nil
}

func (r *Reconciler) createOrUpdateHTTPRoute(ctx context.Context, tunnel *kubelbv1alpha1.Tunnel, hostname, serviceName string, tenant *kubelbv1alpha1.Tenant, config *kubelbv1alpha1.Config, annotations kubelbv1alpha1.AnnotationSettings) (*corev1.ObjectReference, error) {
	httpRouteName := fmt.Sprintf("tunnel-%s", tunnel.Name)

	httpRoute := &gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      httpRouteName,
			Namespace: tunnel.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, httpRoute, func() error {
		if httpRoute.Labels == nil {
			httpRoute.Labels = make(map[string]string)
		}
		httpRoute.Labels[TunnelOwnerLabel] = tunnel.Name
		httpRoute.Labels[TunnelHostnameLabel] = hostname
		httpRoute.Annotations = kubelb_internal.PropagateAnnotations(tunnel.Annotations, annotations, kubelbv1alpha1.AnnotatedResourceHTTPRoute)

		// Add cert-manager and external-dns annotations for automated DNS and TLS
		if tenant.Spec.Certificates.DefaultClusterIssuer != nil {
			httpRoute.Annotations[resources.CertManagerClusterIssuerAnnotation] = *tenant.Spec.Certificates.DefaultClusterIssuer
		} else if config.Spec.Certificates.DefaultClusterIssuer != nil {
			httpRoute.Annotations[resources.CertManagerClusterIssuerAnnotation] = *config.Spec.Certificates.DefaultClusterIssuer
		}
		httpRoute.Annotations[resources.ExternalDNSHostnameAnnotation] = hostname
		httpRoute.Annotations[resources.ExternalDNSTTLAnnotation] = resources.ExternalDNSTTLDefault

		var parentGateway gwapiv1.ParentReference
		if tenant.Spec.GatewayAPI.DefaultGateway != nil {
			parentGateway = gwapiv1.ParentReference{
				Name: gwapiv1.ObjectName(tenant.Spec.GatewayAPI.DefaultGateway.Name),
			}
			if tenant.Spec.GatewayAPI.DefaultGateway.Namespace != "" {
				parentGateway.Namespace = ptr.To(gwapiv1.Namespace(tenant.Spec.GatewayAPI.DefaultGateway.Namespace))
			}
		} else if config.Spec.GatewayAPI.DefaultGateway != nil {
			parentGateway = gwapiv1.ParentReference{
				Name: gwapiv1.ObjectName(config.Spec.GatewayAPI.DefaultGateway.Name),
			}
			if config.Spec.GatewayAPI.DefaultGateway.Namespace != "" {
				parentGateway.Namespace = ptr.To(gwapiv1.Namespace(config.Spec.GatewayAPI.DefaultGateway.Namespace))
			}
		}
		httpRoute.Spec.ParentRefs = []gwapiv1.ParentReference{
			parentGateway,
		}

		httpRoute.Spec.Hostnames = []gwapiv1.Hostname{
			gwapiv1.Hostname(hostname),
		}

		pathType := gwapiv1.PathMatchPathPrefix
		httpRoute.Spec.Rules = []gwapiv1.HTTPRouteRule{
			{
				Matches: []gwapiv1.HTTPRouteMatch{
					{
						Path: &gwapiv1.HTTPPathMatch{
							Type:  &pathType,
							Value: ptr.To("/"),
						},
					},
				},
				BackendRefs: []gwapiv1.HTTPBackendRef{
					{
						BackendRef: gwapiv1.BackendRef{
							BackendObjectReference: gwapiv1.BackendObjectReference{
								Name: gwapiv1.ObjectName(serviceName),
								Port: ptr.To(gwapiv1.PortNumber(80)),
							},
						},
					},
				},
			},
		}
		// Set owner reference
		return controllerutil.SetControllerReference(tunnel, httpRoute, r.scheme)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create/update httproute: %w", err)
	}

	return &corev1.ObjectReference{
		APIVersion: "gateway.networking.k8s.io/v1",
		Kind:       "HTTPRoute",
		Name:       httpRoute.Name,
		Namespace:  httpRoute.Namespace,
		UID:        httpRoute.UID,
	}, nil
}
