# Ingress to Gateway API Conversion

**What this PR does / why we need it**:

Adds a migration assistant that automatically converts Kubernetes Ingress resources to Gateway API HTTPRoutes and GRPCRoutes. This helps users transition from ingress-nginx to Gateway API without manual rewriting of routing configuration.

The converter can run in two modes:

- **Standalone mode** (`--ingress-conversion-only`): Pure migration tool, no KubeLB management cluster required
- **Integrated mode** (`--enable-ingress-conversion`): Works alongside existing CCM controllers

### Key Features

**Core Conversion**

- Converts Ingress → HTTPRoute (one route per hostname for clean separation)
- Detects `backend-protocol: GRPC/GRPCS` → generates GRPCRoute instead
- Handles path types (Exact, Prefix, ImplementationSpecific)
- Resolves named ports via Service lookup
- Transforms hostnames with configurable domain replacement

**Gateway Management**

- Auto-creates Gateway resource with appropriate listeners
- Adds HTTPS listeners when Ingress has TLS configuration
- Merges listeners from multiple Ingresses intelligently
- Supports user-specified gateway annotations via `--conversion-gateway-annotations` flag
- Propagates external-dns target annotation from Ingress to Gateway

**NGINX Annotation Support**

- Redirects → `RequestRedirect` filter (ssl-redirect, permanent-redirect, temporal-redirect)
- Rewrites → `URLRewrite` filter (rewrite-target, app-root)
- Headers → `RequestHeaderModifier` filter (proxy-set-headers, custom-headers)
- Policies → warnings with suggested Gateway API equivalents (timeouts, CORS, rate-limiting, auth, affinity)
- Unsupported → clear warnings (snippets, modsecurity, canary)

**Lifecycle**

- Skip annotation (`kubelb.k8c.io/skip-conversion`) for opt-out
- Status tracking via annotations (converted/partial/pending/failed/skipped)
- Stale route cleanup when hosts are removed from Ingress
- Once converted, Ingress is not re-reconciled (users own the generated routes)

**Which issue(s) this PR fixes**:
Fixes #

**What type of PR is this?**
/kind feature

**Special notes for your reviewer**:

The converter is designed as a migration tool, not a runtime sync. Once an Ingress is successfully converted (`conversion-status: converted` or `partial`), the controller stops watching it. Users then own the generated Gateway API resources and can modify them freely.

To trigger re-conversion, remove the `kubelb.k8c.io/conversion-status` annotation from the Ingress.

### New Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--ingress-conversion-only` | Standalone mode | false |
| `--enable-ingress-conversion` | Integrated mode | false |
| `--conversion-gateway-name` | Gateway name | kubelb |
| `--conversion-gateway-namespace` | Gateway namespace | (same as Ingress) |
| `--conversion-gateway-class` | GatewayClass | kubelb |
| `--conversion-ingress-class` | Filter by IngressClass | (all) |
| `--conversion-domain-replace` | Domain to strip | |
| `--conversion-domain-suffix` | Replacement suffix | |
| `--conversion-gateway-annotations` | Annotations to add to Gateway (comma-separated key=value) | |
| `--conversion-propagate-external-dns-annotations` | Propagate external-dns annotations | true |
| `--conversion-cleanup-stale` | Delete orphaned routes | true |

**Does this PR introduce a user-facing change? Then add your Release Note here**:

```release-note
New Ingress to Gateway API conversion feature. Automatically converts Ingress resources to HTTPRoutes/GRPCRoutes with support for common ingress-nginx annotations. Can run standalone or integrated with KubeLB CCM.
```

**Documentation**:

```documentation
TBD
```
