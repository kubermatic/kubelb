# kubelb-manager

Helm chart for KubeLB Manager. This is used to deploy the KubeLB CCM to a Kubernetes cluster. The CCM is responsible for propagating the load balancer configurations to the management cluster.

![Version: v1.0.0-beta.0](https://img.shields.io/badge/Version-v1.0.0--beta.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v1.0.0-beta.0](https://img.shields.io/badge/AppVersion-v1.0.0--beta.0-informational?style=flat-square)

## Installing the chart

### Pre-requisites

* Create a namespace `kubelb` for the CCM to be deployed in.
* Create imagePullSecrets for the chart to pull the image from the registry.

At this point a minimal values.yaml should look like this:

```yaml
imagePullSecrets:
  - name: <imagePullSecretName>
```

### Install helm chart

Now, we can install the helm chart:

```sh
helm registry login quay.io --username ${REGISTRY_USER} --password ${REGISTRY_PASSWORD}
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-manager --version=v1.0.0-beta.0 --untardir "kubelb-manager" --untar
## Create and update values.yaml with the required values.
helm install kubelb-manager kubelb-manager --namespace kubelb -f values.yaml
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
| fullnameOverride | string | `""` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"quay.io/kubermatic/kubelb-manager-ee"` |  |
| image.tag | string | `"v1.0.0-beta.0"` |  |
| imagePullSecrets | list | `[]` |  |
| kubelb.debug | bool | `false` |  |
| kubelb.enableLeaderElection | bool | `true` |  |
| kubelb.envoyProxyReplicas | int | `3` |  |
| kubelb.envoyProxySinglePodPerNode | bool | `true` |  |
| kubelb.envoyProxyTopology | string | `"shared"` |  |
| kubelb.envoyProxyUseDaemonset | bool | `false` |  |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` |  |
| podAnnotations | object | `{}` |  |
| podLabels | object | `{}` |  |
| podSecurityContext.runAsNonRoot | bool | `true` |  |
| podSecurityContext.seccompProfile.type | string | `"RuntimeDefault"` |  |
| rbac.allowLeaderElectionRole | bool | `true` |  |
| rbac.allowMetricsReaderRole | bool | `true` |  |
| rbac.allowProxyRole | bool | `true` |  |
| rbac.enabled | bool | `true` |  |
| replicaCount | int | `1` |  |
| resources.limits.cpu | string | `"100m"` |  |
| resources.limits.memory | string | `"128Mi"` |  |
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