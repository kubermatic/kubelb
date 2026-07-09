+++
title = "kubelb completion bash"
date = 2026-07-08T00:00:00+00:00
weight = 50
+++

## kubelb completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(kubelb completion bash)

To load completions for every new session, execute once:

#### Linux:

	kubelb completion bash > /etc/bash_completion.d/kubelb

#### macOS:

	kubelb completion bash > $(brew --prefix)/etc/bash_completion.d/kubelb

You will need to start a new shell for this setup to take effect.


```
kubelb completion bash
```

### Options

```
  -h, --help              help for bash
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

