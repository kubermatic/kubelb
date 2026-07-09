+++
title = "kubelb tunnel"
date = 2026-07-08T00:00:00+00:00
weight = 30
+++

## kubelb tunnel

Manage secure tunnels to expose local services

### Synopsis

Create and manage secure tunnels to expose local services through the KubeLB infrastructure

### Options

```
  -h, --help   help for tunnel
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
* [kubelb tunnel connect](../kubelb_tunnel_connect)	 - Connect to an existing tunnel
* [kubelb tunnel create](../kubelb_tunnel_create)	 - Create a tunnel
* [kubelb tunnel delete](../kubelb_tunnel_delete)	 - Delete a tunnel
* [kubelb tunnel get](../kubelb_tunnel_get)	 - Get a tunnel
* [kubelb tunnel list](../kubelb_tunnel_list)	 - List tunnels

