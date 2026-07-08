+++
title = "kubelb serve"
date = 2026-07-08T00:00:00+00:00
weight = 30
+++

## kubelb serve

Start web dashboard for interactive Ingress migration

### Synopsis

Start a local web server with an interactive dashboard for Ingress-to-Gateway
API migration. Provides a visual interface for the same operations available
through the CLI subcommands.

The dashboard has two views:

Ingresses: Browse all Ingress resources with conversion status, preview
generated Gateway API YAML, and convert individually or in batch.

Gateway API: Inspect all created resources (Gateways, HTTPRoutes, GRPCRoutes,
Envoy Gateway policies) with their acceptance status.


```
kubelb serve [flags]
```

### Examples

```
kubelb serve --addr 127.0.0.1:8080
```

### Options

```
      --addr string                      Address to listen on (default "127.0.0.1:8080")
      --copy-tls-secrets                 Copy TLS secrets to Gateway namespace [$KUBELB_COPY_TLS_SECRETS] (default true)
      --disable-envoy-gateway-features   Disable Envoy Gateway policies [$KUBELB_DISABLE_ENVOY_GATEWAY_FEATURES]
      --domain-replace string            Source domain to strip [$KUBELB_DOMAIN_REPLACE]
      --domain-suffix string             Target domain suffix [$KUBELB_DOMAIN_SUFFIX]
      --gateway-annotations stringMap    Gateway annotations (key=value,...) [$KUBELB_GATEWAY_ANNOTATIONS]
      --gateway-class string             GatewayClass name [$KUBELB_GATEWAY_CLASS] (default "kubelb")
      --gateway-name string              Gateway name [$KUBELB_GATEWAY_NAME] (default "kubelb")
      --gateway-namespace string         Gateway namespace [$KUBELB_GATEWAY_NAMESPACE] (default "kubelb")
  -h, --help                             help for serve
      --ingress-class string             Only convert Ingresses with this class [$KUBELB_INGRESS_CLASS]
      --propagate-external-dns           Propagate external-dns annotations [$KUBELB_PROPAGATE_EXTERNAL_DNS] (default true)
```

### Options inherited from parent commands

```
      --kubeconfig string   Path to the kubeconfig for the tenant
      --log-file string     Log to file instead of stderr
      --log-format string   Log format (cli, json, text) - defaults to cli
      --log-level string    Log level (error, warn, info, debug, trace) - overrides verbosity
  -q, --quiet               Suppress non-essential output (equivalent to --v=0)
  -t, --tenant string       Name of the tenant
      --timeout duration    Timeout for the command (e.g., 30s, 5m) (default 4m0s)
  -v, --v int               Verbosity level (0-4): 0=errors only, 1=basic info, 2=detailed status, 3=debug info, 4=trace (default 1)
```

### SEE ALSO

* [kubelb](../kubelb)	 - KubeLB CLI - Manage load balancers and create secure tunnels

