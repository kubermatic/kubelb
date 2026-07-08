+++
title = "kubelb ingress get"
date = 2026-07-08T00:00:00+00:00
weight = 50
+++

## kubelb ingress get

Display an Ingress resource as clean YAML

### Synopsis

Fetch an Ingress and display its YAML with cluster noise removed.

Strips managed fields, last-applied-configuration annotation, and sets
the proper API version and kind. Useful for inspecting an Ingress before
deciding whether to convert it.


```
kubelb ingress get NAME [flags]
```

### Examples

```
# Get ingress YAML
kubelb ingress get my-app -n default

# Using namespace/name format
kubelb ingress get default/my-app

```

### Options

```
  -h, --help               help for get
  -n, --namespace string   Namespace of the ingress (default: default)
```

### Options inherited from parent commands

```
      --copy-tls-secrets                 Copy TLS secrets to Gateway namespace [$KUBELB_COPY_TLS_SECRETS] (default true)
      --disable-envoy-gateway-features   Disable Envoy Gateway policies [$KUBELB_DISABLE_ENVOY_GATEWAY_FEATURES]
      --domain-replace string            Source domain to strip [$KUBELB_DOMAIN_REPLACE]
      --domain-suffix string             Target domain suffix [$KUBELB_DOMAIN_SUFFIX]
      --gateway-annotations stringMap    Gateway annotations (key=value,...) [$KUBELB_GATEWAY_ANNOTATIONS]
      --gateway-class string             GatewayClass name [$KUBELB_GATEWAY_CLASS] (default "kubelb")
      --gateway-name string              Gateway name [$KUBELB_GATEWAY_NAME] (default "kubelb")
      --gateway-namespace string         Gateway namespace [$KUBELB_GATEWAY_NAMESPACE] (default "kubelb")
      --ingress-class string             Only convert Ingresses with this class [$KUBELB_INGRESS_CLASS]
      --kubeconfig string                Path to the kubeconfig for the tenant
      --log-file string                  Log to file instead of stderr
      --log-format string                Log format (cli, json, text) - defaults to cli
      --log-level string                 Log level (error, warn, info, debug, trace) - overrides verbosity
      --propagate-external-dns           Propagate external-dns annotations [$KUBELB_PROPAGATE_EXTERNAL_DNS] (default true)
  -q, --quiet                            Suppress non-essential output (equivalent to --v=0)
  -t, --tenant string                    Name of the tenant
      --timeout duration                 Timeout for the command (e.g., 30s, 5m) (default 4m0s)
  -v, --v int                            Verbosity level (0-4): 0=errors only, 1=basic info, 2=detailed status, 3=debug info, 4=trace (default 1)
```

### SEE ALSO

* [kubelb ingress](../kubelb_ingress)	 - Migrate Ingress resources to Gateway API

