+++
title = "kubelb loadbalancer create"
date = 2026-07-08T00:00:00+00:00
weight = 50
+++

## kubelb loadbalancer create

Create a load balancer

### Synopsis

Create a new HTTP load balancer with the specified endpoints.

The load balancer supports HTTP routing and hostname-based access.

Examples:
  # Create HTTP load balancer with random hostname
  kubelb lb create my-app --endpoints 10.0.1.1:8080

  # Create HTTP load balancer with custom hostname
  kubelb lb create my-app --endpoints 10.0.1.1:8080 --hostname app.example.com

  # Create HTTP load balancer without a route
  kubelb lb create my-app --endpoints 10.0.1.1:8080 --route=false


```
kubelb loadbalancer create NAME [flags]
```

### Examples

```
kubelb loadbalancer create my-app --endpoints 10.0.1.1:8080,10.0.1.2:8080 --tenant=mytenant
```

### Options

```
  -e, --endpoints string   Comma-separated list of IP:port pairs (required)
  -h, --help               help for create
      --hostname string    Custom hostname for the route
  -o, --output string      Output format (summary, yaml, json) (default "summary")
  -p, --protocol string    Protocol (http only) (default "http")
      --route              Create a route for HTTP traffic (default true)
      --type string        LoadBalancer type (ClusterIP, LoadBalancer), defaults to ClusterIP (default "ClusterIP")
      --wait               Wait for load balancer to be ready (default true)
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

