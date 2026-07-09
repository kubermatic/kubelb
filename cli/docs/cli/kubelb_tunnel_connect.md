+++
title = "kubelb tunnel connect"
date = 2026-07-08T00:00:00+00:00
weight = 50
+++

## kubelb tunnel connect

Connect to an existing tunnel

### Synopsis

Connect to an existing tunnel to start forwarding traffic.

This command establishes a secure connection to the tunnel and forwards
traffic from the tunnel to your local service.


```
kubelb tunnel connect NAME [flags]
```

### Examples

```
kubelb tunnel connect my-app --port 8080 --tenant=mytenant
```

### Options

```
  -h, --help       help for connect
  -p, --port int   Local port to forward to (required)
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

* [kubelb tunnel](../kubelb_tunnel)	 - Manage secure tunnels to expose local services

