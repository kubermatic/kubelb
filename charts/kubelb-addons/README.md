# KubeLB Addons

Helm chart for deploying optional addons to enhance KubeLB functionality.

## Overview

This chart packages commonly used networking and certificate management tools that integrate well with KubeLB:

- **ingress-nginx**: NGINX Ingress Controller for Kubernetes
- **envoy-gateway**: Modern cloud native API gateway
- **cert-manager**: Automatic TLS certificate management
- **external-dns**: Synchronize exposed services with DNS providers
- **kgateway**: Kubernetes Gateway API implementation

## Installation

```bash
helm install kubelb-addons ./charts/kubelb-addons
```

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
