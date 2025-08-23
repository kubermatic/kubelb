# KubeLB Addons

Helm chart for deploying optional addons to enhance KubeLB functionality.

![Version: v0.0.1](https://img.shields.io/badge/Version-v0.0.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.0.1](https://img.shields.io/badge/AppVersion-v0.0.1-informational?style=flat-square)

## Installing the chart

```sh
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-addons --version=v0.0.1 --untardir "kubelb-addons" --untar
## Create and update values.yaml with the required values.
helm install kubelb-addons kubelb-addons --namespace kubelb -f values.yaml --create-namespace
```

## Overview

This chart packages commonly used networking and certificate management tools that integrate well with KubeLB:

- **ingress-nginx**: NGINX Ingress Controller for Kubernetes
- **envoy-gateway**: Modern cloud native API gateway
- **cert-manager**: Automatic TLS certificate management
- **external-dns**: Synchronize exposed services with DNS providers
- **kgateway**: Kubernetes Gateway API implementation

## Configuration

All addons are disabled by default. Enable specific addons by setting their `enabled` flag:

```yaml
ingress-nginx:
  enabled: true

cert-manager:
  enabled: true

external-dns:
  enabled: true
```

### External DNS Configuration

External DNS requires provider-specific configuration. Example is for AWS Route53 for more providers see [External DNS](https://kubernetes-sigs.github.io/external-dns/latest/charts/external-dns/):

1. Create AWS credentials secret:

```bash
kubectl create secret generic route53-credentials \
  --from-file=credentials=/path/to/aws/credentials
```

2. Configure domain filters in values:

```yaml
external-dns:
  enabled: true
  domainFilters:
    - example.com
  provider: aws
```

## Values

### values.yaml

These are the default values to use when Gateway API is enabled for KubeLB.

### values-ingress.yaml

These are the default values to use when Gateway API is disabled for KubeLB in favor of using Ingress.

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://charts.jetstack.io | cert-manager | 1.18.2 |
| https://kubernetes-sigs.github.io/external-dns | external-dns | 1.18.0 |
| https://kubernetes.github.io/ingress-nginx | ingress-nginx | 4.13.0 |
| oci://cr.kgateway.dev/kgateway-dev/charts | kgateway | v2.0.4 |
| oci://cr.kgateway.dev/kgateway-dev/charts | kgateway-crds | v2.0.4 |
| oci://docker.io/envoyproxy | gateway-helm | 1.3.0 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| cert-manager.config.apiVersion | string | `"controller.config.cert-manager.io/v1alpha1"` |  |
| cert-manager.config.enableGatewayAPI | bool | `true` |  |
| cert-manager.config.kind | string | `"ControllerConfiguration"` |  |
| cert-manager.crds.enabled | bool | `true` |  |
| cert-manager.enabled | bool | `false` |  |
| envoy-gateway.enabled | bool | `false` |  |
| external-dns.domainFilters[0] | string | `"${DOMAIN}"` |  |
| external-dns.enabled | bool | `false` |  |
| external-dns.env[0].name | string | `"AWS_SHARED_CREDENTIALS_FILE"` |  |
| external-dns.env[0].value | string | `"/.aws/credentials"` |  |
| external-dns.extraVolumeMounts[0].mountPath | string | `"/.aws"` |  |
| external-dns.extraVolumeMounts[0].name | string | `"credentials"` |  |
| external-dns.extraVolumeMounts[0].readOnly | bool | `true` |  |
| external-dns.extraVolumes[0].name | string | `"credentials"` |  |
| external-dns.extraVolumes[0].secret.secretName | string | `"route53-credentials"` |  |
| external-dns.policy | string | `"sync"` |  |
| external-dns.provider | string | `"aws"` |  |
| external-dns.registry | string | `"txt"` |  |
| external-dns.sources[0] | string | `"service"` |  |
| external-dns.sources[1] | string | `"ingress"` |  |
| external-dns.sources[2] | string | `"gateway-httproute"` |  |
| external-dns.sources[3] | string | `"gateway-grpcroute"` |  |
| external-dns.sources[4] | string | `"gateway-tlsroute"` |  |
| external-dns.sources[5] | string | `"gateway-tcproute"` |  |
| external-dns.sources[6] | string | `"gateway-udproute"` |  |
| external-dns.txtOwnerId | string | `"kubelb-management"` |  |
| ingress-nginx.enabled | bool | `false` |  |
| kgateway-crds.enabled | bool | `false` |  |
| kgateway.agentgateway.enabled | bool | `false` |  |
| kgateway.enabled | bool | `false` |  |
| kgateway.gateway.aiExtension.enabled | bool | `true` |  |

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Kubermatic | <support@kubermatic.com> | <https://kubermatic.com> |