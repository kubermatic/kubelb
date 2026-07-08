+++
title = "kubelb"
date = 2026-07-08T00:00:00+00:00
weight = 10
+++

## kubelb

KubeLB CLI - Manage load balancers and create secure tunnels

### Synopsis

KubeLB CLI provides tools to manage KubeLB load balancers and create secure tunnels
to expose local services through the KubeLB infrastructure.

### Options

```
  -h, --help                help for kubelb
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

* [kubelb completion](../kubelb_completion)	 - Generate the autocompletion script for the specified shell
* [kubelb docs](../kubelb_docs)	 - Generate markdown documentation for all commands
* [kubelb expose](../kubelb_expose)	 - Expose a local port via tunnel
* [kubelb ingress](../kubelb_ingress)	 - Migrate Ingress resources to Gateway API
* [kubelb loadbalancer](../kubelb_loadbalancer)	 - Manage KubeLB load balancers
* [kubelb serve](../kubelb_serve)	 - Start web dashboard for interactive Ingress migration
* [kubelb status](../kubelb_status)	 - Display current status of KubeLB
* [kubelb tunnel](../kubelb_tunnel)	 - Manage secure tunnels to expose local services
* [kubelb version](../kubelb_version)	 - Print the version information

