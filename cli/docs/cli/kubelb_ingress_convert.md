+++
title = "kubelb ingress convert"
date = 2026-07-08T00:00:00+00:00
weight = 50
+++

## kubelb ingress convert

Convert ingresses to Gateway API

### Synopsis

Convert one or more Ingress resources to Gateway API HTTPRoutes.

By default, applies the resources directly to the cluster.
Use --output-dir to export YAML files for GitOps workflows.
Use --dry-run to preview changes without applying.

Must specify NAME(s), --all (with -n), or -A. No implicit bulk conversion.


```
kubelb ingress convert [NAME...] [flags]
```

### Examples

```
# Convert single ingress
kubelb ingress convert my-app -n default

# Convert all ingresses in a namespace
kubelb ingress convert --all -n default

# Convert all ingresses across all namespaces
kubelb ingress convert -A

# Export to files for GitOps
kubelb ingress convert --all -n default --output-dir ./manifests

# Preview changes without applying
kubelb ingress convert my-app --dry-run

```

### Options

```
      --all                 Convert all ingresses in namespace (requires -n)
  -A, --all-namespaces      Convert all ingresses across all namespaces
      --dry-run             Preview changes without applying
  -h, --help                help for convert
  -n, --namespace string    Namespace to filter ingresses
  -o, --output-dir string   Export YAML files to directory instead of applying
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

