+++
title = "kubelb completion fish"
date = 2026-07-08T00:00:00+00:00
weight = 50
+++

## kubelb completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	kubelb completion fish | source

To load completions for every new session, execute once:

	kubelb completion fish > ~/.config/fish/completions/kubelb.fish

You will need to start a new shell for this setup to take effect.


```
kubelb completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
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

* [kubelb completion](../kubelb_completion)	 - Generate the autocompletion script for the specified shell

