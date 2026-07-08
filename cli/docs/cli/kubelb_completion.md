+++
title = "kubelb completion"
date = 2026-07-08T00:00:00+00:00
weight = 30
+++

## kubelb completion

Generate the autocompletion script for the specified shell

### Synopsis

Generate the autocompletion script for kubelb for the specified shell.
See each sub-command's help for details on how to use the generated script.


### Options

```
  -h, --help   help for completion
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
* [kubelb completion bash](../kubelb_completion_bash)	 - Generate the autocompletion script for bash
* [kubelb completion fish](../kubelb_completion_fish)	 - Generate the autocompletion script for fish
* [kubelb completion powershell](../kubelb_completion_powershell)	 - Generate the autocompletion script for powershell
* [kubelb completion zsh](../kubelb_completion_zsh)	 - Generate the autocompletion script for zsh

