+++
title = "kubelb loadbalancer delete"
date = 2026-07-08T00:00:00+00:00
weight = 50
+++

## kubelb loadbalancer delete

Delete a load balancer

### Synopsis

Delete a load balancer by ID.

This command will:
- Check if the load balancer was created by the CLI
- Display a warning if it wasn't created by the CLI
- Ask for confirmation before deletion (unless --force is used)
- Delete the load balancer resource


```
kubelb loadbalancer delete ID [flags]
```

### Examples

```
kubelb loadbalancer delete nginx-loadbalancer --tenant=mytenant --kubeconfig=./kubeconfig
```

### Options

```
  -f, --force   Force deletion without confirmation
  -h, --help    help for delete
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

* [kubelb loadbalancer](../kubelb_loadbalancer)	 - Manage KubeLB load balancers

