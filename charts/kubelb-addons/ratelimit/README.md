# ratelimit

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1e50889b](https://img.shields.io/badge/AppVersion-1e50889b-informational?style=flat-square)

Envoy global rate limit service (RLS) backing agentgateway global rate-limit policies

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Kubermatic | <support@kubermatic.com> | <https://kubermatic.com> |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| config.configMapName | string | `"ratelimit-config"` |  |
| config.create | bool | `false` |  |
| config.inline | object | `{}` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.registry | string | `"docker.io"` |  |
| image.repository | string | `"envoyproxy/ratelimit"` |  |
| image.tag | string | `"1e50889b"` |  |
| imagePullSecrets | list | `[]` |  |
| logLevel | string | `"info"` |  |
| nodeSelector | object | `{}` |  |
| redis.authSecretName | string | `""` |  |
| redis.url | string | `""` |  |
| replicaCount | int | `2` |  |
| resources | object | `{}` |  |
| service.grpcPort | int | `8081` |  |
| service.httpPort | int | `8080` |  |
| service.metricsPort | int | `9090` |  |
| tolerations | list | `[]` |  |

