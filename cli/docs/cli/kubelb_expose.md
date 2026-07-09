+++
title = "kubelb expose"
date = 2026-07-08T00:00:00+00:00
weight = 30
+++

## kubelb expose

Expose a local port via tunnel

### Synopsis

Expose a local port via secure tunnel with auto-generated name.

This is a convenience command that creates a tunnel with an auto-generated
name and immediately connects to it.

Examples:
  # Expose port 8080 with auto-generated tunnel name
  kubelb expose 8080

  # Expose port 3000 with custom hostname
  kubelb expose 3000 --hostname api.example.com


```
kubelb expose PORT [flags]
```

### Examples

```
kubelb expose 8080 --tenant=mytenant
```

### Options

```
  -h, --help              help for expose
      --hostname string   Custom hostname for the tunnel (default: auto-assigned wildcard domain)
  -o, --output string     Output format (summary, yaml, json) (default "summary")
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

* [kubelb](../kubelb)	 - KubeLB CLI - Manage load balancers and create secure tunnels

