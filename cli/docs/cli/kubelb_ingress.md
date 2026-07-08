+++
title = "kubelb ingress"
date = 2026-07-08T00:00:00+00:00
weight = 30
+++

## kubelb ingress

Migrate Ingress resources to Gateway API

### Synopsis

Tools for migrating NGINX Ingress resources to Gateway API (HTTPRoute).

This command provides a complete workflow for converting Ingresses:
- List ingresses with their conversion status
- Get clean YAML for individual ingresses
- Preview the generated HTTPRoute YAML before applying
- Convert individual or multiple ingresses
- Run a web dashboard for interactive migration

All subcommands respect the same conversion options (gateway name, namespace, etc).


### Options

```
      --copy-tls-secrets                 Copy TLS secrets to Gateway namespace [$KUBELB_COPY_TLS_SECRETS] (default true)
      --disable-envoy-gateway-features   Disable Envoy Gateway policies [$KUBELB_DISABLE_ENVOY_GATEWAY_FEATURES]
      --domain-replace string            Source domain to strip [$KUBELB_DOMAIN_REPLACE]
      --domain-suffix string             Target domain suffix [$KUBELB_DOMAIN_SUFFIX]
      --gateway-annotations stringMap    Gateway annotations (key=value,...) [$KUBELB_GATEWAY_ANNOTATIONS]
      --gateway-class string             GatewayClass name [$KUBELB_GATEWAY_CLASS] (default "kubelb")
      --gateway-name string              Gateway name [$KUBELB_GATEWAY_NAME] (default "kubelb")
      --gateway-namespace string         Gateway namespace [$KUBELB_GATEWAY_NAMESPACE] (default "kubelb")
  -h, --help                             help for ingress
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
* [kubelb ingress convert](../kubelb_ingress_convert)	 - Convert ingresses to Gateway API
* [kubelb ingress get](../kubelb_ingress_get)	 - Display an Ingress resource as clean YAML
* [kubelb ingress list](../kubelb_ingress_list)	 - List ingresses with conversion status
* [kubelb ingress preview](../kubelb_ingress_preview)	 - Preview HTTPRoute YAML for an ingress

