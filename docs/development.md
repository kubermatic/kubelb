# Local Development

How to run KubeLB on your machine and iterate fast.

## Prerequisites

- Go 1.26+
- Docker (Desktop on macOS, daemon on Linux)
- [kind](https://kind.sigs.k8s.io/), `kubectl`, `helm`
- [Tilt](https://docs.tilt.dev/install.html) (optional, for `make dev-tilt`)

## Quick start

Two kind clusters (`kubelb` + `tenant1`) with full addons, one command:

```bash
make dev-setup
```

Iterate on code:

```bash
# … edit a controller …
make dev-reload          # rebuild + redeploy changed binaries
```

Tear down:

```bash
make dev-cleanup
```

`dev-setup` is idempotent — re-run after a chart edit and it does `helm
upgrade` instead of failing.

> **Note:** dev clusters share `.e2e-kubeconfigs/` with `make e2e-*` (kind
> cluster names are global on the docker daemon, so a separate dir would not
> isolate anything). After `make dev-setup` you can promote to a full e2e
> environment with `make e2e-setup-kind && make e2e-deploy` (adds tenant2).

## Two development modes

KubeLB supports two local-dev modes. Pick based on what you're doing.

### Mode A — Attach (processes run in-cluster)

Binaries run inside the kind clusters in their normal pods. You edit code,
the binary is rebuilt and swapped into the running deployment.
**xDS and the Envoy dataplane keep working** because the manager pod stays in
place at its original Service address.

```bash
make dev-reload          # one-shot reload after a code change
# or
make dev-tilt            # watcher: auto-reload on file save
```

`dev-tilt` watches `cmd/`, `internal/`, `api/`, `pkg/` and calls `dev-reload`
automatically. Single Tilt process handles both manager (in `kubelb` cluster)
and CCM (in `tenant1` cluster). Open the Tilt UI at <http://localhost:10350>
for build logs and streaming pod logs.

Use this mode for:

- normal iteration on controller logic
- anything that requires real cluster networking (xDS, Envoy Gateway, MetalLB)
- verifying behavior against the production image path

### Mode B — Local (processes run on your laptop)

The binary runs on your machine via `go run`. The `make dev-run-*` targets
**scale the in-cluster deployment to 0 for the session** and restore it on
exit, so there's no double-reconcile.

```bash
make dev-run-ccm         # CCM on your laptop
make dev-run-manager     # manager on your laptop
```

Use this mode for:

- attaching a debugger (Delve, VS Code, GoLand)
- trying changes without paying the docker-build + kind-load cost
- running against a real cluster via BYO kubeconfig (see below)

**Limitation — manager + xDS:** in local mode the in-cluster `envoycp.kubelb.svc`
Service has no backing pod, so tenant Envoy proxies can't reach the local
manager for xDS updates. API-level reconciliation works; the Envoy dataplane
goes stale. If you need xDS locally, use Mode A (`dev-tilt` / `dev-reload`) or
[ktunnel](https://github.com/omrikiei/ktunnel) to reverse-port-forward port
8001 as the `envoycp` Service.

## Bring your own clusters

All `dev-run-*` targets honor env var overrides — point them at any kubeconfig.

```bash
# CCM locally against a real tenant cluster
TENANT_KUBECONFIG=~/.kube/prod-tenant \
  KUBELB_INTERNAL_KUBECONFIG=~/.kube/prod-hub \
  CLUSTER_NAME=my-tenant \
  make dev-run-ccm

# Manager locally against a real management cluster
KUBELB_KUBECONFIG=~/.kube/prod-hub make dev-run-manager

# Hybrid: in-cluster manager from dev-setup, local CCM against a real tenant
make dev-setup
TENANT_KUBECONFIG=~/.kube/my-tenant make dev-run-ccm
```

`KUBELB_INTERNAL_KUBECONFIG` is the kubeconfig the CCM uses to reach the
manager cluster's API. For kind it must point at `kubelb-internal.kubeconfig`
(docker-bridge IP); for real clusters any kubeconfig that resolves the manager
API server works.

## Make targets

| Target | What it does |
| --- | --- |
| `make dev-setup` | Create kind clusters + deploy KubeLB |
| `make dev-deploy` | Re-run helm upgrade against existing clusters |
| `make dev-reload` | Rebuild + reload manager/CCM images (skips unchanged) |
| `make dev-tilt` | Watch source, auto-reload in-cluster binaries (mode A) |
| `make dev-run-manager` | Run manager on laptop, scale in-cluster to 0 (mode B) |
| `make dev-run-ccm` | Run CCM on laptop, scale in-cluster to 0 (mode B) |
| `make dev-cleanup` | Delete clusters and kubeconfigs |

## Troubleshooting

**`make dev-reload` keeps redeploying unchanged binaries**: delete
`.e2e-kubeconfigs/.reload-hashes/` and re-run.

**`make dev-tilt` fails with "kubeconfigs not found"**: run `make dev-setup`
first.

**`make dev-run-*` left the deployment scaled to 0 after a crash**: run
`kubectl -n kubelb scale deploy/kubelb --replicas=1` (or `kubelb-ccm` in the
tenant cluster), or just `make dev-deploy` to re-run helm upgrade.

**Force-recreate dev clusters**: `./hack/e2e/setup-kind.sh --force`.

**Mac Docker network unreachable**: enable Docker Desktop host networking
(Settings → Resources → Network), or let `setup-kind.sh` install
`docker-mac-net-connect` automatically.
