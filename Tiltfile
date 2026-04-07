# -*- mode: Python -*-
#
# Tiltfile for local KubeLB development.
#
# Watches source files and rebuilds+redeploys the manager and CCM into their
# respective kind clusters. Processes run INSIDE the clusters (so
# envoycp.kubelb.svc xDS keeps working). Under the hood just calls
# `make dev-reload`, which uses hash-tracked builds to skip unchanged
# components.
#
# Usage (via Makefile): `make dev-tilt`
#
# Prerequisites: `make dev-setup` must have been run first.

config.define_string('kubeconfigs-dir')
cfg = config.parse()
KUBECONFIGS_DIR = cfg.get('kubeconfigs-dir', os.environ.get('KUBECONFIGS_DIR', os.getcwd() + '/.e2e-kubeconfigs'))

# Tilt connects to exactly one k8s context; we point at the manager cluster.
# CCM lives in kind-tenant1 and is managed via local_resource below
# (kubectl --kubeconfig), so Tilt never switches contexts.
allow_k8s_contexts('kind-kubelb')

# Fail fast if dev-setup wasn't run.
if not os.path.exists(KUBECONFIGS_DIR + '/kubelb.kubeconfig'):
    fail('kubeconfigs not found at %s — run `make dev-setup` first' % KUBECONFIGS_DIR)

# ---------------------------------------------------------------------------
# Reload: rebuilds kubelb + ccm binaries, does hash-skipped docker build +
# kind load + rollout restart across kubelb and tenant1 clusters. Single
# resource so builds stay serialized and there are no concurrent-make races.
# reload.sh internally decides which components actually changed and skips
# untouched ones.
# ---------------------------------------------------------------------------
local_resource(
    'kubelb-reload',
    cmd='make -s DEV_MODE=true KUBECONFIGS_DIR=%s dev-reload' % KUBECONFIGS_DIR,
    deps=[
        'cmd/kubelb',
        'cmd/ccm',
        'internal',
        'api',
        'pkg',
        'go.mod',
        'go.sum',
    ],
    ignore=[
        '**/*_test.go',
        '**/zz_generated_*.go',
    ],
    labels=['kubelb'],
)

# Stream manager logs into the Tilt UI. (CCM lives in tenant1; tail its logs
# separately: `kubectl --kubeconfig=.e2e-kubeconfigs/tenant1.kubeconfig
# -n kubelb logs -f deploy/kubelb-ccm`.)
local_resource(
    'kubelb-manager-logs',
    serve_cmd='kubectl --kubeconfig="%s/kubelb.kubeconfig" -n kubelb logs -f deploy/kubelb --tail=50' % KUBECONFIGS_DIR,
    resource_deps=['kubelb-reload'],
    labels=['logs'],
)

local_resource(
    'kubelb-ccm-logs',
    serve_cmd='kubectl --kubeconfig="%s/tenant1.kubeconfig" -n kubelb logs -f deploy/kubelb-ccm --tail=50' % KUBECONFIGS_DIR,
    resource_deps=['kubelb-reload'],
    labels=['logs'],
)
