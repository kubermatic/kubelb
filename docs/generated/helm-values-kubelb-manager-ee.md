
| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| autoscaling.enabled | bool | `false` |  |
| autoscaling.maxReplicas | int | `10` |  |
| autoscaling.minReplicas | int | `1` |  |
| autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| autoscaling.targetMemoryUtilizationPercentage | int | `80` |  |
| cert-manager.enabled | bool | `false` | Enable cert-manager. |
| external-dns.enabled | bool | `false` | Enable External-DNS. |
| fullnameOverride | string | `""` |  |
| grafana.dashboards.annotations | object | `{}` | Additional annotations for dashboard ConfigMaps |
| grafana.dashboards.enabled | bool | `false` | Requires grafana to be deployed with `sidecar.dashboards.enabled=true`. For more info: https://github.com/grafana/helm-charts/tree/grafana-10.5.13/charts/grafana#:~:text=%5B%5D-,sidecar.dashboards.enabled,-Enables%20the%20cluster |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"quay.io/kubermatic/kubelb-manager-ee"` |  |
| image.tag | string | `"v99.0.1"` |  |
| imagePullSecrets[0].name | string | `"kubermatic-quay.io"` |  |
| kkpintegration.rbac | bool | `false` | Create RBAC for KKP integration. |
| kubeRbacProxy.image.pullPolicy | string | `"IfNotPresent"` |  |
| kubeRbacProxy.image.repository | string | `"quay.io/brancz/kube-rbac-proxy"` |  |
| kubeRbacProxy.image.tag | string | `"v0.20.1"` |  |
| kubelb.debug | bool | `true` |  |
| kubelb.disableEnvoyGatewayFeatures | bool | `false` | disableEnvoyGatewayFeatures disables Envoy Gateway support for BackendTrafficPolicy and ClientTrafficPolicy. Use this if you're using a Gateway API implementation other than Envoy Gateway. |
| kubelb.enableGatewayAPI | bool | `false` | enableGatewayAPI specifies whether to enable the Gateway API and Gateway Controllers. By default Gateway API is disabled since without Gateway APIs installed the controller cannot start. |
| kubelb.enableLeaderElection | bool | `true` |  |
| kubelb.enableWAF | bool | `false` | [Alpha Feature] enableWAF enables the WAF controller for Web Application Firewall policy validation. WAF is an alpha feature and is disabled by default. |
| kubelb.envoyProxy.affinity | object | `{}` |  |
| kubelb.envoyProxy.gracefulShutdown.disabled | bool | `false` | Disable graceful shutdown (default: false) |
| kubelb.envoyProxy.imagePullSecrets | list | `[]` | imagePullSecrets for Envoy Proxy pods. If not set, auto-detected from manager pod. |
| kubelb.envoyProxy.nodeSelector | object | `{}` |  |
| kubelb.envoyProxy.replicas | int | `2` | The number of replicas for the Envoy Proxy deployment. |
| kubelb.envoyProxy.resources | object | `{}` |  |
| kubelb.envoyProxy.singlePodPerNode | bool | `true` | Deploy single pod per node. |
| kubelb.envoyProxy.tolerations | list | `[]` |  |
| kubelb.envoyProxy.topology | string | `"shared"` | Topology defines the deployment topology for Envoy Proxy. Only "shared" is supported. "dedicated" and "global" are deprecated and will default to shared. |
| kubelb.envoyProxy.useDaemonset | bool | `false` | Use DaemonSet for Envoy Proxy deployment instead of Deployment. |
| kubelb.logLevel | string | `"info"` | To configure the verbosity of logging. Can be one of 'debug', 'info', 'error', 'panic' or any integer value > 0 which corresponds to custom debug levels of increasing verbosity. |
| kubelb.propagateAllAnnotations | bool | `false` | Propagate all annotations from the LB resource to the LB service. |
| kubelb.propagatedAnnotations | object | `{}` | Allowed annotations that will be propagated from the LB resource to the LB service. |
| kubelb.skipConfigGeneration | bool | `false` | Set to true to skip the generation of the Config CR. Useful when the config CR needs to be managed manually. |
| kubelb.tunnel.connectionManager.affinity | object | `{}` |  |
| kubelb.tunnel.connectionManager.healthCheck.enabled | bool | `true` |  |
| kubelb.tunnel.connectionManager.healthCheck.livenessInitialDelay | int | `30` |  |
| kubelb.tunnel.connectionManager.healthCheck.readinessInitialDelay | int | `10` |  |
| kubelb.tunnel.connectionManager.httpAddr | string | `":8080"` | Server addresses |
| kubelb.tunnel.connectionManager.httpRoute.annotations | object | `{"cert-manager.io/cluster-issuer":"letsencrypt-prod","external-dns.alpha.kubernetes.io/hostname":"connection-manager.${DOMAIN}"}` | Annotations for HTTPRoute |
| kubelb.tunnel.connectionManager.httpRoute.domain | string | `"connection-manager.${DOMAIN}"` | Domain for the HTTPRoute NOTE: Replace ${DOMAIN} with your domain name. |
| kubelb.tunnel.connectionManager.httpRoute.enabled | bool | `false` |  |
| kubelb.tunnel.connectionManager.httpRoute.gatewayName | string | `"gateway"` | Gateway name to attach to |
| kubelb.tunnel.connectionManager.httpRoute.gatewayNamespace | string | `""` | Gateway namespace |
| kubelb.tunnel.connectionManager.image | object | `{"pullPolicy":"IfNotPresent","repository":"quay.io/kubermatic/kubelb-connection-manager-ee","tag":""}` | Connection manager image configuration |
| kubelb.tunnel.connectionManager.ingress | object | `{"annotations":{"cert-manager.io/cluster-issuer":"letsencrypt-prod","external-dns.alpha.kubernetes.io/hostname":"connection-manager.${DOMAIN}","nginx.ingress.kubernetes.io/backend-protocol":"HTTP","nginx.ingress.kubernetes.io/proxy-read-timeout":"3600","nginx.ingress.kubernetes.io/proxy-send-timeout":"3600"},"className":"nginx","enabled":false,"hosts":[{"host":"connection-manager.${DOMAIN}","paths":[{"path":"/tunnel","pathType":"Prefix"},{"path":"/health","pathType":"Prefix"}]}],"tls":[{"hosts":["connection-manager.${DOMAIN}"],"secretName":"connection-manager-tls"}]}` | Ingress configuration for external HTTP/2 access |
| kubelb.tunnel.connectionManager.nodeSelector | object | `{}` |  |
| kubelb.tunnel.connectionManager.podAnnotations | object | `{}` | Pod configuration |
| kubelb.tunnel.connectionManager.podLabels | object | `{}` |  |
| kubelb.tunnel.connectionManager.podSecurityContext.fsGroup | int | `65534` |  |
| kubelb.tunnel.connectionManager.podSecurityContext.runAsNonRoot | bool | `true` |  |
| kubelb.tunnel.connectionManager.podSecurityContext.runAsUser | int | `65534` |  |
| kubelb.tunnel.connectionManager.replicaCount | int | `1` | Number of connection manager replicas |
| kubelb.tunnel.connectionManager.requestTimeout | string | `"30s"` |  |
| kubelb.tunnel.connectionManager.resources | object | `{"limits":{"cpu":"500m","memory":"256Mi"},"requests":{"cpu":"250m","memory":"128Mi"}}` | Resource limits |
| kubelb.tunnel.connectionManager.securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true,"runAsNonRoot":true,"runAsUser":65534}` | Security context |
| kubelb.tunnel.connectionManager.service | object | `{"httpPort":8080,"type":"ClusterIP"}` | Service configuration |
| kubelb.tunnel.connectionManager.tolerations | list | `[]` |  |
| kubelb.tunnel.enabled | bool | `false` | Enable tunnel functionality |
| metrics.port | int | `9443` | Port where the manager exposes metrics (includes both manager and envoycp metrics) |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` |  |
| podAnnotations | object | `{}` |  |
| podLabels | object | `{}` |  |
| podSecurityContext.runAsNonRoot | bool | `true` |  |
| podSecurityContext.seccompProfile.type | string | `"RuntimeDefault"` |  |
| priorityClassName | string | `""` | PriorityClassName for the manager pod (e.g., "system-cluster-critical") |
| rbac.allowLeaderElectionRole | bool | `true` |  |
| rbac.allowMetricsReaderRole | bool | `true` |  |
| rbac.allowProxyRole | bool | `true` |  |
| rbac.enabled | bool | `true` |  |
| replicaCount | int | `1` |  |
| resources.limits.cpu | string | `"500m"` |  |
| resources.limits.memory | string | `"512Mi"` |  |
| resources.requests.cpu | string | `"100m"` |  |
| resources.requests.memory | string | `"128Mi"` |  |
| securityContext.allowPrivilegeEscalation | bool | `false` |  |
| securityContext.capabilities.drop[0] | string | `"ALL"` |  |
| securityContext.runAsUser | int | `65532` |  |
| service.port | int | `8001` |  |
| service.protocol | string | `"TCP"` |  |
| service.type | string | `"ClusterIP"` |  |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |
| serviceMonitor.enabled | bool | `false` |  |
| tolerations | list | `[]` |  |
