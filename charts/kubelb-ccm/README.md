# kubelb-ccm

Helm chart for KubeLB CCM. This is used to deploy the KubeLB CCM to a Kubernetes cluster. The CCM is responsible for propagating the load balancer configurations to the management cluster.

![Version: v1.3.0](https://img.shields.io/badge/Version-v1.3.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v1.3.0](https://img.shields.io/badge/AppVersion-v1.3.0-informational?style=flat-square)

## Installing the chart

### Pre-requisites

* Create a namespace `kubelb` for the CCM to be deployed in.
* The agent expects a `Secret` with a kubeconf file named `kubelb` to access the load balancer cluster. To create such run: `kubectl --namespace kubelb create secret generic kubelb-cluster --from-file=<path to kubelb kubeconf file>`. The name of secret cant be overridden using `.Values.kubelb.clusterSecretName`
* Update the `tenantName` in the values.yaml to a unique identifier for the tenant. This is used to identify the tenant in the manager cluster. This can be any unique string that follows [lower case RFC 1123](https://www.rfc-editor.org/rfc/rfc1123).

At this point a minimal values.yaml should look like this:

```yaml
kubelb:
    clusterSecretName: kubelb-cluster
    tenantName: <unique-identifier-for-tenant>
```

### Install helm chart

Now, we can install the helm chart:

```sh
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-ccm --version=v1.3.0 --untardir "." --untar
## Apply CRDs
kubectl apply -f kubelb-ccm/crds/
## Create and update values.yaml with the required values.
helm install kubelb-ccm kubelb-ccm/ --namespace kubelb -f values.yaml
```

## Security

### Chart Signing

All Helm charts are cryptographically signed using [Sigstore Cosign](https://github.com/sigstore/cosign) with keyless signing.

### Verify Chart Signature

```bash
cosign verify quay.io/kubermatic/helm-charts/kubelb-ccm:v1.3.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

### Verify Image Signature

```bash
cosign verify quay.io/kubermatic/kubelb-ccm:v1.3.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| autoscaling.enabled | bool | `false` |  |
| autoscaling.maxReplicas | int | `10` |  |
| autoscaling.minReplicas | int | `1` |  |
| autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| autoscaling.targetMemoryUtilizationPercentage | int | `80` |  |
| extraVolumeMounts | list | `[]` |  |
| extraVolumes | list | `[]` |  |
| fullnameOverride | string | `""` |  |
| grafana.dashboards.annotations | object | `{}` | Additional annotations for dashboard ConfigMaps |
| grafana.dashboards.enabled | bool | `false` | Requires grafana to be deployed with `sidecar.dashboards.enabled=true`. For more info: https://github.com/grafana/helm-charts/tree/grafana-10.5.13/charts/grafana#:~:text=%5B%5D-,sidecar.dashboards.enabled,-Enables%20the%20cluster |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"quay.io/kubermatic/kubelb-ccm"` |  |
| image.tag | string | `"v1.3.0"` |  |
| imagePullSecrets | list | `[]` |  |
| kubeRbacProxy.image.pullPolicy | string | `"IfNotPresent"` |  |
| kubeRbacProxy.image.repository | string | `"quay.io/brancz/kube-rbac-proxy"` |  |
| kubeRbacProxy.image.tag | string | `"v0.20.1"` |  |
| kubelb.clusterSecretName | string | `"kubelb-cluster"` | Name of the secret that contains kubeconfig for the loadbalancer cluster |
| kubelb.disableGRPCRouteController | bool | `false` | disableGRPCRouteController specifies whether to disable the GRPCRoute Controller. |
| kubelb.disableGatewayController | bool | `false` | disableGatewayController specifies whether to disable the Gateway Controller. |
| kubelb.disableHTTPRouteController | bool | `false` | disableHTTPRouteController specifies whether to disable the HTTPRoute Controller. |
| kubelb.disableIngressController | bool | `false` | disableIngressController specifies whether to disable the Ingress Controller. |
| kubelb.enableGatewayAPI | bool | `false` | enableGatewayAPI specifies whether to enable the Gateway API and Gateway Controllers. By default Gateway API is disabled since without Gateway APIs installed the controller cannot start. |
| kubelb.enableLeaderElection | bool | `true` | Enable the leader election. |
| kubelb.enableSecretSynchronizer | bool | `false` | Enable to automatically convert Secrets labelled with `kubelb.k8c.io/managed-by: kubelb` to Sync Secrets. This is used to sync secrets from tenants to the LB cluster in a controlled and secure way. |
| kubelb.gatewayAPICRDsChannel | string | `"standard"` | gatewayAPICRDsChannel specifies the channel for the Gateway API CRDs. Options are `standard` and `experimental`. |
| kubelb.ingressConversion.copyTLSSecrets | bool | `true` | copyTLSSecrets copies TLS secrets from Ingress namespace to Gateway namespace for cross-namespace certificate references |
| kubelb.ingressConversion.disableEnvoyGatewayFeatures | bool | `false` | disableEnvoyGatewayFeatures disables creation of Envoy Gateway policies (SecurityPolicy, BackendTrafficPolicy) |
| kubelb.ingressConversion.domainReplace | string | `""` | domainReplace is the domain suffix to replace in hostnames |
| kubelb.ingressConversion.domainSuffix | string | `""` | domainSuffix is the replacement domain suffix for hostnames |
| kubelb.ingressConversion.enabled | bool | `false` | enabled enables automatic Ingress to HTTPRoute conversion |
| kubelb.ingressConversion.gatewayAnnotations | string | `""` | gatewayAnnotations are annotations to add to created Gateway (comma-separated key=value pairs) Example: "cert-manager.io/cluster-issuer=letsencrypt,external-dns.alpha.kubernetes.io/target=lb.example.com" |
| kubelb.ingressConversion.gatewayClass | string | `"kubelb"` | gatewayClass is the GatewayClass name for created Gateway |
| kubelb.ingressConversion.gatewayName | string | `"kubelb"` | gatewayName is the name of the Gateway for converted HTTPRoutes |
| kubelb.ingressConversion.gatewayNamespace | string | `"kubelb"` | gatewayNamespace is the namespace for the shared Gateway (required) |
| kubelb.ingressConversion.ingressClass | string | `""` | ingressClass filters Ingresses to convert (empty = convert all) |
| kubelb.ingressConversion.propagateExternalDnsAnnotations | bool | `true` | propagateExternalDnsAnnotations propagates external-dns annotations to Gateway/HTTPRoute |
| kubelb.ingressConversion.standaloneMode | bool | `false` | standaloneMode runs as standalone converter, disabling all other controllers |
| kubelb.installGatewayAPICRDs | bool | `false` | installGatewayAPICRDs Installs and manages the Gateway API CRDs using gateway crd controller. |
| kubelb.logLevel | string | `"info"` | To configure the verbosity of logging. Can be one of 'debug', 'info', 'error', 'panic' or any integer value > 0 which corresponds to custom debug levels of increasing verbosity. |
| kubelb.nodeAddressType | string | `"ExternalIP"` | Address type to use for routing traffic to node ports. Values are ExternalIP, InternalIP. |
| kubelb.tenantName | string | `nil` | Name of the tenant, must be unique against a load balancer cluster. |
| kubelb.useGatewayClass | bool | `true` | useGatewayClass specifies whether to target resources with `kubelb` gateway class or all resources. |
| kubelb.useIngressClass | bool | `true` | useIngressClass specifies whether to target resources with `kubelb` ingress class or all resources. |
| kubelb.useLoadBalancerClass | bool | `false` | useLoadBalancerClass specifies whether to target services of type LoadBalancer with `kubelb` load balancer class or all services of type LoadBalancer. |
| metrics.port | int | `9445` | Port where the CCM exposes metrics |
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
| service.port | int | `8443` |  |
| service.protocol | string | `"TCP"` |  |
| service.type | string | `"ClusterIP"` |  |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |
| serviceMonitor.enabled | bool | `false` |  |
| tolerations | list | `[]` |  |

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Kubermatic | <support@kubermatic.com> | <https://kubermatic.com> |