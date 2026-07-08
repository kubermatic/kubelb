+++
title = "kubelb tunnel create"
date = 2026-07-08T00:00:00+00:00
weight = 50
+++

## kubelb tunnel create

Create a tunnel

### Synopsis

Create a new secure tunnel to expose a local service.

The tunnel provides secure access to your local service through the KubeLB infrastructure.

Examples:
  # Create tunnel for local app on port 8080
  kubelb tunnel create my-app --port 8080

  # Create tunnel with custom hostname
  kubelb tunnel create my-app --port 8080 --hostname app.example.com

  # Create tunnel and connect immediately
  kubelb tunnel create my-app --port 8080 --connect


```
kubelb tunnel create NAME [flags]
```

### Examples

```
kubelb tunnel create my-app --port 8080 --tenant=mytenant
```

### Options

```
      --connect           Connect to tunnel after creation
  -h, --help              help for create
      --hostname string   Custom hostname for the tunnel (default: auto-assigned wildcard domain)
  -o, --output string     Output format (summary, yaml, json) (default "summary")
  -p, --port int          Local port to tunnel (required)
      --wait              Wait for tunnel to be ready (default true)
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

