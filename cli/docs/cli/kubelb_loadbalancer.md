+++
title = "kubelb loadbalancer"
date = 2026-07-08T00:00:00+00:00
weight = 30
+++

## kubelb loadbalancer

Manage KubeLB load balancers

### Synopsis

Manage KubeLB load balancer configurations

### Options

```
  -h, --help   help for loadbalancer
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
* [kubelb loadbalancer create](../kubelb_loadbalancer_create)	 - Create a load balancer
* [kubelb loadbalancer delete](../kubelb_loadbalancer_delete)	 - Delete a load balancer
* [kubelb loadbalancer get](../kubelb_loadbalancer_get)	 - Get a load balancer
* [kubelb loadbalancer list](../kubelb_loadbalancer_list)	 - List load balancers

