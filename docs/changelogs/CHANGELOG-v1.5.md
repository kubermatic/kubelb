# Kubermatic KubeLB v1.5

- [v1.5.0](#v150)
  - [Community Edition](#community-edition)
  - [Enterprise Edition](#enterprise-edition)

## v1.5.0

**GitHub release: [v1.5.0](https://github.com/kubermatic/kubelb/releases/tag/v1.5.0)**

### Highlights

#### KubeLB Insights (EE)

KubeLB v1.5 ships **[KubeLB Insights](https://docs.kubermatic.com/kubelb/v1.5/insights/)** — a deterministic advisor for the platform operator. The management cluster already knows every tenant's effective configuration, every route and every WAF policy; the insights engine periodically evaluates that state against a registry of 15 checks and records what it finds as `Insight` resources.

- No sampling, no scoring model, no LLM. Every check is a pure function of cluster state, so a finding is reproducible and its absence means the same thing every time.
- Findings land in the namespace of what they are about, and can be triaged in place — acknowledge, snooze, dismiss. Individual checks can be suppressed via `Config.spec.insights.disabledChecks`.
- Fleet-wide checks cover hostname collisions across tenants, silently stripped certificate annotations, unprotected HTTP routes, quota headroom, WAF failure-mode and network policy asymmetry, and mTLS certificates that missed their rotation window.
- A `kubelb_manager_posture_score` metric is scored per tenant and category, with a bundled Grafana dashboard and Prometheus alert rules (findings, degraded posture, checks failing to evaluate, stalled sweep loop).
- The engine is on by default and quiet by design — checks declare the features they need and skip themselves when those features are absent. Turn it off with `kubelb.enableInsights: false`.

#### mTLS Backend Transport (EE, Beta / Technical Preview)

Management-to-tenant backend traffic can now be encrypted end to end. Enabled from the management cluster with `Config.spec.backendTransport.mode: MTLS`, traffic is routed through a KubeLB-managed Envoy proxy in the tenant cluster instead of straight to workload NodePorts. TCP backends use raw mTLS/SNI; UDPRoute uses a CONNECT-UDP tunnel over the same port.

- `Config.spec.backendTransport.udp.mode` switches UDP between `Tunnel` (default, encrypted CONNECT-UDP) and `Direct` (classic per-service NodePort, unencrypted) for workloads sensitive to the tunnel's MTU overhead. TCP stays on mTLS either way.
- The management cluster owns the root CA and issues a name-constrained intermediate per tenant. Intermediate rotation is two-phase and root rotation goes through a pre-generated successor, so leaves never outrun the trust bundle they are validated against.
- The effective mode is projected into each tenant's `TenantState`, so tenant-side controllers cannot drift from what the manager has configured.
- Flipping `Direct <-> MTLS` re-plumbs the tenant dataplane, so it is gated behind an explicit `kubelb.k8c.io/confirm-backend-transport-change` annotation on the `Config`.
- Tenant-side exposure is overridable: `kubelb.tenantProxy.serviceType` and `kubelb.tenantProxy.staticAddresses`/`staticPort` on the kubelb-ccm chart cover proxies behind an appliance, NAT, or user-managed DNS.

#### Tenant Self-Service WAF (EE)

Tenants can now define namespaced `TenantWAFPolicy` resources against their own routes, instead of every rule going through the platform operator. The feature is opt-in per installation (`Config.spec.waf.enableTenantPolicies`) and per tenant (`Tenant.spec.waf`); tenant directives are validated against a strict allowlist and each policy is isolated to the tenant's own namespace.

WAF also gained dataplane observability this release — Grafana dashboards and Prometheus alerts for blocked requests, filter failures, Coraza VM reloads and xDS NACKs (alerts off by default via `prometheusRule.enabled`).

#### KubeLB CLI

The **[KubeLB CLI](https://docs.kubermatic.com/kubelb/v1.5/cli/)** — until now released from `kubermatic/kubelb-cli` on its own version line — has been merged into this repository under `cli/`. It is developed, tested and released on the same tag and the same cycle as the manager and CCM, with full supply-chain parity: keyless cosign signatures and provenance attestations on every artifact.

- Version jumps `v0.2.0` → `v1.5.0` and tracks the KubeLB release from here on.
- Download `kubelb-cli_<version>_<os>_<arch>` from the [GitHub release](https://github.com/kubermatic/kubelb/releases/tag/v1.5.0); the binary inside the archive is named `kubelb`.
- Commands: `expose`, `loadbalancer`, `ingress` (including `convert` and `preview` for the Ingress → Gateway API migration), `status`, `serve` and `tunnel`. `tunnel` requires Enterprise Edition.
- The CLI auto-detects Community vs Enterprise Edition against the target cluster at startup.

```bash
VERSION=1.5.0
curl -LO https://github.com/kubermatic/kubelb/releases/download/v${VERSION}/kubelb-cli_${VERSION}_linux_amd64.tar.gz
tar -xzf kubelb-cli_${VERSION}_linux_amd64.tar.gz kubelb
sudo install kubelb /usr/local/bin/
```

Verify the download with keyless cosign:

```bash
gh attestation verify kubelb-cli_${VERSION}_linux_amd64.tar.gz --repo kubermatic/kubelb
```

Existing `kubelb-cli` users have two behaviour changes to pick up — see [Urgent Upgrade Notes](#urgent-upgrade-notes).

#### Configurable Envoy Timeouts and Streaming-Friendly Defaults

Stock Envoy timeouts were cutting off streaming and large-file workloads — object-storage downloads, model artifacts, websockets, SSE, long-running gRPC streams — because the KubeLB Envoy layer was double-bounding requests that the edge proxy already bounds.

| Timeout | Was | Now |
| --- | --- | --- |
| Per-request (`RouteAction.Timeout`) | 15s (Envoy default) | disabled |
| HTTP idle connection | 60s | 1h |
| HCM stream idle | 5m (Envoy default) | 1h |

Enterprise Edition additionally makes six timeouts configurable at Config, Tenant, Route and LoadBalancer level, with `kubelb.k8c.io/timeout-*` annotations on Ingress/HTTPRoute/GRPCRoute/TCPRoute/TLSRoute: `request`, `streamIdle`, `requestHeaders`, `idleConnection`, `tcpIdle` and `connect`.

#### Gateway API

- **Gateway API CRDs upgraded to v1.6.1** (CE). CRDs installed by the CCM are labeled `kubelb.k8c.io/managed-by=kubelb`, and CRDs that disappear upstream between versions are now pruned instead of being left pinned at a stale bundle version.
- **Gateway name collisions are no longer silent** (CE). Gateways with the same name in different tenant cluster namespaces resolved to a single object in the management cluster and overwrote each other. The first namespace to claim a name keeps it; later claimants get an error on the Route instead of a silent takeover.
- **Unserved GatewayClasses get a signal** (CE). A Gateway naming a GatewayClass that KubeLB does not serve now gets a `GatewayClassNotAccepted` warning event instead of being ignored.
- **ReferenceGrant enforcement** (EE). Cross-namespace `backendRefs` and Gateway TLS `certificateRefs` can be restricted to references permitted by a ReferenceGrant, via `spec.gatewayAPI.enforceReferenceGrants` on Config/Tenant (default off).
- **Multiple GatewayClasses per tenant** (EE). `Tenant.spec.gatewayAPI.classMappings` and `Config.spec.gatewayAPI.classMappings` map tenant-cluster GatewayClasses to distinct management-cluster GatewayClasses.
- **Backend pools** (EE). Group multiple LoadBalancer resources into one Gateway API backend pool with the `kubelb.k8c.io/backend-pool` annotation.

#### xDS Control Plane Hardening (CE)

A focused pass on the Envoy control plane, driven by what showed up at scale:

- Snapshot versions are now stable for unchanged config, so an unchanged reconcile no longer republishes to every connected proxy. Snapshot consistency is validated on the first push as well as subsequent ones.
- The xDS gRPC server sets keepalive and bounded connection age, and drains streams on shutdown instead of resetting them.
- `kubelb_envoy_control_plane_xds_nacks_total` exports configs rejected by Envoy.
- Deleting a Tenant clears its snapshot from the cache instead of leaking it until the next manager restart.
- Generated cluster and listener names no longer include the Service UID, so replacing an origin Service is seen by Envoy as an update rather than a new cluster plus listener — the cause of NC 503s on pooled connections after a Service replace.
- TCP keepalive is enabled on proxied connections so idle connections are not silently dropped by kube-proxy in IPVS mode.

#### KubeLB Dashboard v1.1

[KubeLB Dashboard](https://docs.kubermatic.com/kubelb/v1.5/dashboard/) v1.1.0 is the companion release for KubeLB v1.5. Install via `oci://quay.io/kubermatic/helm-charts/kubelb-dashboard`; source at [kubermatic/kubelb-dashboard](https://github.com/kubermatic/kubelb-dashboard).

#### Also in This Release

**Community Edition (CE)**

- **Annotation deny list and glob patterns** — `deniedAnnotations` on Config and Tenant, and shell-style globs (`nginx.ingress.kubernetes.io/*`) in both `propagatedAnnotations` and `deniedAnnotations`.
- **Configurable client header limits** — `Config.spec.envoyProxy.headerLimits` defaults to Envoy's maximum, so large client headers no longer produce `431 Request Header Fields Too Large`.
- **Source IP persistence for Layer 4** — `LoadBalancer.spec.persistence.type: SourceIP`; tenant Services using `sessionAffinity: ClientIP` are propagated with observed-source persistence.
- **Hostname (FQDN) endpoints** — translated into Envoy `STRICT_DNS` clusters instead of EDS, fixing "malformed IP address" rejections. `EndpointAddress.ip` is now optional.
- **Default resource requests and limits on the managed Envoy proxy**, so it no longer runs BestEffort.
- Addons chart v0.5.0: envoy-gateway 1.8.3, cert-manager 1.21.1, external-dns 1.21.1, metallb 0.16.1, agentgateway/agentgateway-crds 1.4.1, ingress-nginx 4.15.1.
- Built with Go 1.26.6.

**Enterprise Edition (EE)**

- **Configurable active health checks** — TCP, HTTP and gRPC, on Config, Tenant, LoadBalancer and Route, and via `kubelb.k8c.io/health-check-*` annotations on tenant Services, Ingresses and Routes.
- **Rejections are visible where tenants look** — `LoadBalancerStatus` carries an `Accepted` condition surfaced by the CCM as a Warning event on the tenant Service, route rejections appear on the tenant cluster object's status, and a Route rejected for a hostname outside `allowedDomains` gets a warning event on the tenant's Ingress/Gateway/Route.
- **Source IP persistence for Layer 4** via `spec.persistence.type: SourceIP`, taking precedence over `loadBalancerPolicy` when both are set.

#### Stability & Reliability

Beyond the control plane work above, v1.5 lands a broad batch of correctness fixes:

- Orphaned management-cluster mirrors are reaped when the origin Service/Ingress/Route is deleted, recreated, downgraded, or the tenant cluster is rebuilt.
- Removing an annotation from a tenant Service now removes it from the generated LoadBalancer Service, tracked via `kubelb.k8c.io/managed-annotations` so third-party annotations are not clobbered.
- Generated resources are torn down when Ingress or Gateway API is disabled for a tenant or globally, and when an accepted Route is edited to a disallowed hostname.
- Routes that hit a transient backing-Service apply failure requeue and recover instead of staying broken.
- The manager and CCM ClusterRoles can create events again — the missing permission was silently dropping every event the controllers emitted.
- A potential manager crash from unsynchronized `PortAllocator` reads is fixed.
- Generated Route service names comply with DNS-1035.
- The tenant reconciler tolerates clusters without the Prometheus Operator CRDs installed.

### Community Edition

#### Urgent Upgrade Notes

**(No, really, you MUST read this before you upgrade)**

- **[action required] The KubeLB CLI now ships from `kubermatic/kubelb` and shares the KubeLB version number** (`kubelb-cli` v0.2.0 → v1.5.0). Download `kubelb-cli_<version>_<os>_<arch>` archives from the `kubermatic/kubelb` release assets; the binary inside is named `kubelb`.
  - CLI artifacts are now signed with keyless cosign and carry provenance attestations. Verification against the old `kubelb-cli` static cosign key no longer applies to new releases.
  - `kubelb serve` now listens on `127.0.0.1:8080` by default instead of on all interfaces. ([#497](https://github.com/kubermatic/kubelb/pull/497))
- **Envoy proxy pods roll exactly once on upgrade.** Generated xDS cluster and listener names no longer include the Service UID; the roll is driven by a naming-version pod annotation so the renamed resource set only reaches fresh proxies. ([#570](https://github.com/kubermatic/kubelb/pull/570))
- **The managed Envoy proxy container now has default resource requests and limits** — 200m CPU / 256Mi memory requests, 2 CPU / 1Gi memory limits. Previously it ran BestEffort. Check namespace quota and node capacity before upgrading; `envoyProxy.resources` in Config/Tenant still overrides. ([#570](https://github.com/kubermatic/kubelb/pull/570))
- **HTTP timeout defaults changed.** Idle-connection timeout is raised from 60s to 1h and the per-request timeout changes from 15s (Envoy default) to disabled. Enterprise Edition users can restore the previous behaviour with `Config.spec.timeouts.idleConnection: 60s` and `Config.spec.timeouts.request: 15s`. ([#434](https://github.com/kubermatic/kubelb/pull/434))
- **TCP keepalive is enabled on proxied connections.** Existing connections are drained once when the updated listener config is pushed. ([#540](https://github.com/kubermatic/kubelb/pull/540))
- **Gateway API CRDs are upgraded to v1.6.1**, and the experimental `XListenerSet` CRD — removed upstream in Gateway API v1.5 and still pinned at bundle v1.4.1 — is dropped from the bundle. CRDs that disappear upstream are now pruned on download instead of lingering in tenant clusters. ([#541](https://github.com/kubermatic/kubelb/pull/541), [#565](https://github.com/kubermatic/kubelb/pull/565), [#566](https://github.com/kubermatic/kubelb/pull/566))

#### API Change

- Add `deniedAnnotations` to Config and Tenant for excluding annotation keys from propagation. Keys in `propagatedAnnotations` and `deniedAnnotations` now support shell-style glob patterns (e.g. `nginx.ingress.kubernetes.io/*`). ([#454](https://github.com/kubermatic/kubelb/pull/454))
- The `ip` field on `EndpointAddress` is now optional; at least one of `ip` or `hostname` must be set. ([#438](https://github.com/kubermatic/kubelb/pull/438))

#### Features

- The KubeLB CLI is merged into this repository under `cli/` and released on the same tag and cycle as the manager and CCM, with keyless cosign signatures and provenance attestations. ([#497](https://github.com/kubermatic/kubelb/pull/497))
- KubeLB's managed Envoy request/response header size and count limits are now configurable via `Config.spec.envoyProxy.headerLimits`, defaulting to Envoy's maximum so large client headers no longer cause `431 Request Header Fields Too Large`. ([#519](https://github.com/kubermatic/kubelb/pull/519))
- Add optional SourceIP persistence for Layer 4 LoadBalancers. Services using `sessionAffinity: ClientIP` are propagated with observed-source persistence in KubeLB. ([#435](https://github.com/kubermatic/kubelb/pull/435))
- Gateway API CRDs installed by the KubeLB CCM are now labeled with `kubelb.k8c.io/managed-by=kubelb`. ([#487](https://github.com/kubermatic/kubelb/pull/487))
- Gateway API CRDs have been upgraded to v1.6.1. ([#541](https://github.com/kubermatic/kubelb/pull/541))
- KubeLB is now built using Go 1.26.6. ([#453](https://github.com/kubermatic/kubelb/pull/453), [#476](https://github.com/kubermatic/kubelb/pull/476), [#498](https://github.com/kubermatic/kubelb/pull/498), [#577](https://github.com/kubermatic/kubelb/pull/577))

#### Bug or Regression

- Generated xDS cluster/listener names no longer include the Service UID; envoy proxy pods roll once on upgrade to pick up the new names.
  - Orphaned LoadBalancer/Route mirrors in the management cluster are now cleaned up when the tenant origin resource is deleted, recreated, or no longer qualifies.
  - Removing an annotation from a tenant Service now removes it from the generated LoadBalancer Service as well.
  - The managed Envoy proxy container now defaults to 200m CPU / 256Mi memory requests and 2 CPU / 1Gi memory limits; `envoyProxy.resources` in Config/Tenant overrides this. ([#570](https://github.com/kubermatic/kubelb/pull/570))
- Fixed Gateway objects from different tenant cluster namespaces silently overwriting each other in the management cluster. The first namespace to claim a Gateway name keeps it and later claimants get an error on the Route. Errors from applying Ingress, Gateway, HTTPRoute and GRPCRoute objects are now reported as a condition and event on the Route and retried, instead of being discarded. ([#557](https://github.com/kubermatic/kubelb/pull/557))
- Fix generated Ingress/Gateway/HTTPRoute/GRPCRoute not being removed when Ingress or Gateway API is disabled for a tenant or globally.
  - Gateways naming a GatewayClass that KubeLB does not serve now get a Warning event instead of being silently ignored. ([#568](https://github.com/kubermatic/kubelb/pull/568))
- Envoy snapshot versions no longer change when the underlying config is unchanged, which stopped needless xDS pushes to every connected proxy. Snapshot consistency is now validated on the first push as well as subsequent ones. ([#533](https://github.com/kubermatic/kubelb/pull/533))
- Envoy control plane now uses gRPC keepalive and bounded connection age, drains xDS streams on shutdown, and exports `kubelb_envoy_control_plane_xds_nacks_total` for configs rejected by Envoy. ([#532](https://github.com/kubermatic/kubelb/pull/532))
- Deleting a Tenant now clears its Envoy snapshot from the xDS cache instead of leaving it until the manager restarts. ([#534](https://github.com/kubermatic/kubelb/pull/534))
- Enable TCP keepalive on connections through the KubeLB-managed Envoy proxy so idle connections are not silently dropped by kube-proxy in IPVS mode. Existing connections are drained once when the updated listener config is pushed. ([#540](https://github.com/kubermatic/kubelb/pull/540))
- LoadBalancer and Route endpoints with hostnames (FQDNs) are now translated into Envoy `STRICT_DNS` clusters instead of EDS clusters, fixing "malformed IP address" rejections for tenants that use hostname-based endpoints. ([#438](https://github.com/kubermatic/kubelb/pull/438))
- HTTP idle-connection timeout default raised from 60s to 1h, and per-request timeout default changed from 15s (Envoy default) to disabled (streaming-friendly). ([#434](https://github.com/kubermatic/kubelb/pull/434))
- Fixed Routes staying broken after a transient backing-Service apply failure; these now requeue and recover automatically. ([#516](https://github.com/kubermatic/kubelb/pull/516))
- Fixed a potential manager crash (concurrent map read and map write) caused by unsynchronized reads of the port allocator. ([#515](https://github.com/kubermatic/kubelb/pull/515))
- Fix manager and CCM ClusterRoles missing permission to create events, which silently dropped every event the controllers emitted. ([#531](https://github.com/kubermatic/kubelb/pull/531))
- Fix CCM dropping a node reconcile when the tenant's Addresses object was updated concurrently, which could leave endpoints stale until the next node event. ([#553](https://github.com/kubermatic/kubelb/pull/553))
- Fix generated KubeLB Route service names to comply with Kubernetes DNS-1035 validation. ([#477](https://github.com/kubermatic/kubelb/pull/477))
- Fix tenant reconciler failing with `no matches for kind "PodMonitor"` on clusters without the Prometheus Operator CRDs installed. ([#439](https://github.com/kubermatic/kubelb/pull/439))
- Fix `global.imageRegistry` rewriting for the cert-manager addon after the subchart bump to 1.21.0. ([#524](https://github.com/kubermatic/kubelb/pull/524))
- KubeLB binaries and images are now built with Go 1.26.5, addressing CVE-2026-42505 and CVE-2026-39822. ([#498](https://github.com/kubermatic/kubelb/pull/498))

#### Other (Cleanup, Flake, or Chore)

- The KubeLB manager ClusterRole no longer requests unused create/bind/escalate permissions on cluster-scoped clusterroles. ([#514](https://github.com/kubermatic/kubelb/pull/514))
- Bump `kubelb-addons` to v0.5.0 and pin it in the `kubelb-manager` chart, with dependency bumps:
  - envoy-gateway: 1.7.2 → 1.8.3
  - cert-manager: v1.20.2 → 1.21.1
  - external-dns: 1.20.0 → 1.21.1
  - metallb: 0.15.3 → 0.16.1 (no longer deploys a kube-rbac-proxy sidecar)
  - agentgateway / agentgateway-crds: v1.1.0 → 1.4.1
  - ingress-nginx stays at 4.15.1
  ([#542](https://github.com/kubermatic/kubelb/pull/542), [#564](https://github.com/kubermatic/kubelb/pull/564), [#567](https://github.com/kubermatic/kubelb/pull/567))
- Drop the stale `XListenerSet` CRD, removed upstream in Gateway API v1.5, and prune Gateway API CRDs that disappear upstream on download. ([#565](https://github.com/kubermatic/kubelb/pull/565), [#566](https://github.com/kubermatic/kubelb/pull/566))

**Full Changelog**: <https://github.com/kubermatic/kubelb/compare/v1.4.0...v1.5.0>

### Enterprise Edition

**Enterprise Edition includes everything from Community Edition and more. The release notes below are for changes specific to just the Enterprise Edition.**

#### EE Urgent Upgrade Notes

**(No, really, you MUST read this before you upgrade)**

- **The Insights engine is enabled by default.** Set `kubelb.enableInsights: false` to turn it off. (#518)
- **`WAFPolicy` and `TenantWAFPolicy` no longer accept `targetRef.kind: Ingress`.** WAF applies to Gateway API routes (HTTPRoute, GRPCRoute); it was never enforced on Ingress traffic. Existing policies targeting Ingress must be retargeted. (#489)
- **Admin `WAFPolicy` directives are now capped** at 64 items / 1024 characters, with `Config.spec.waf.maxDirectivesPerPolicy` and `maxDirectiveLength` defaulting to those values. Set either to 0 for unlimited. (#486)
- **Envoy proxy pods roll once per tenant namespace on upgrade**, picking up the Service-UID-free xDS resource names and the new default proxy resource requests/limits (configurable via `kubelb.envoyProxy.resources`). (#553, #554)
- **HTTP timeout defaults changed** — see the CE note. Restore the previous behaviour with `Config.spec.timeouts.idleConnection: 60s` and `Config.spec.timeouts.request: 15s`. (#386)
- **Changing `Config.spec.backendTransport.mode` requires explicit confirmation.** A `Direct <-> MTLS` flip re-plumbs the tenant dataplane, so it is not applied until the `Config` carries `kubelb.k8c.io/confirm-backend-transport-change` set to the target mode. Until then each `TenantState` reports `BackendTransportChangePending`. (#430)
- **`Config.spec.prometheus` replaces `Config.spec.ai.prometheus`.** The setting was generalized out of the AI tree and is now owned by Insights. (#502)

#### EE API Change

- Add KubeLB Insights, an opt-in engine (`--enable-insights`) that evaluates the management cluster against a registry of checks and records findings as `Insight` resources, with triage (acknowledge, snooze, dismiss), per-check suppression via `Config.spec.insights.disabledChecks`, and Prometheus metrics. (#507)
- Adds configurable Envoy timeouts at Config, Tenant, Route and LoadBalancer level. New `kubelb.k8c.io/timeout-*` annotations on Ingress/HTTPRoute/GRPCRoute/TCPRoute/TLSRoute propagate to `Route.spec.timeouts`. Six fields are configurable: `request`, `streamIdle`, `requestHeaders`, `idleConnection`, `tcpIdle`, `connect`. (#386)
- Envoy active health checks (TCP/HTTP/gRPC) are now configurable on Config, Tenant, LoadBalancer and Route, and via `kubelb.k8c.io/health-check-*` annotations on tenant Services, Ingresses and Routes. (#471)
- Added support for Gateway API ReferenceGrants. Cross-namespace `backendRefs` and Gateway TLS `certificateRefs` can now be restricted to references permitted by a ReferenceGrant, via `spec.gatewayAPI.enforceReferenceGrants` on Config/Tenant (default off). Also fixed CCM RBAC so events on tenant objects are no longer rejected. (#435)
- Tenants can now map multiple Gateway API GatewayClasses to distinct management-cluster GatewayClasses via `Tenant.spec.gatewayAPI.classMappings` and `Config.spec.gatewayAPI.classMappings`. (#423)
- LoadBalancer status now carries an `Accepted` condition, surfaced by the CCM as a Warning event on the tenant Service. (#556)
- Cap admin WAFPolicy directives at 64 items / 1024 characters and default `Config.spec.waf.maxDirectivesPerPolicy` and `maxDirectiveLength` to 64/1024. Set either to 0 for unlimited. (#486)

#### EE Features

- **mTLS backend transport.** Encrypted management-to-tenant backend traffic via a KubeLB-managed Envoy proxy in the tenant cluster, enabled with `Config.spec.backendTransport.mode: MTLS`. TCP backends use raw mTLS/SNI; UDPRoute uses a CONNECT-UDP tunnel over the same port. `Config.spec.backendTransport.udp.mode` switches UDP between `Tunnel` (default) and `Direct`. Ships as **Beta / Technical Preview**, with per-tenant name-constrained intermediates, two-phase certificate rotation and a mode-flip confirmation gate. (#387, #428, #429, #430, #433, #434, #436, #437, #557, #562)
- **Tenant self-service WAF.** Tenants can define namespaced `TenantWAFPolicy` resources to apply WAF rules to their own routes. Opt-in per installation (`Config.spec.waf.enableTenantPolicies`) and per tenant (`Tenant.spec.waf`); tenant directives are validated against a strict allowlist and each policy is isolated to the tenant's own namespace. (#444)
- **Insights fleet-wide checks and posture score.** Hostname collisions across tenants, silently stripped certificate annotations, unprotected HTTP routes, quota headroom, WAF failure-mode and network policy asymmetry, plus the `kubelb_manager_posture_score` metric scored per tenant and category. (#511)
- Insights now reports mTLS backend transport certificates that entered their rotation window without being reissued. (#523)
- Add Prometheus alert rules for KubeLB Insights: critical findings, degraded tenant posture, checks failing to evaluate, and a stalled sweep loop. (#512)
- Add Grafana dashboards and Prometheus alerts for WAF: blocked requests, filter failures, Coraza VM reloads, and xDS NACKs. Alerts are off by default (`prometheusRule.enabled`). (#484)
- **Resilient backend pools.** Group multiple LoadBalancer resources into one Gateway API backend pool with the `kubelb.k8c.io/backend-pool` annotation. (#440)
- Add optional SourceIP persistence for Layer 4 LoadBalancers via `spec.persistence.type: SourceIP`. Services using `sessionAffinity: ClientIP` are propagated with observed-source persistence; persistence takes precedence over `loadBalancerPolicy` when both are set. (#550)
- The mTLS tenant proxy exposure can now be overridden on the tenant side: `kubelb.tenantProxy.serviceType` on the kubelb-ccm chart overrides the Service type, and `kubelb.tenantProxy.staticAddresses`/`staticPort` publish static IPs or DNS names as the dial target for proxies behind an appliance, NAT, or user-managed DNS. (#528)
- Tenants now receive a Kubernetes warning event on their Ingress / Gateway / *Route when KubeLB rejects it because the hostname is not in `tenant.spec.allowedDomains`. (#405)

#### EE Bug or Regression

- Gateways named `kubelb` in more than one tenant cluster namespace resolved to a single object in the management cluster and silently overwrote each other. The first namespace to claim the name now keeps it and later claimants get an error on the Route. Gateways with any other name were already namespace-qualified and are unaffected. Routes whose sub-resources fail to apply are no longer marked Accepted; the failure is reported as a condition and event, and retried. (#529)
- Mgmt-side mirrors are torn down when a Service stops being a LoadBalancer or its origin disappears.
  - Annotation removals propagate to the generated Service.
  - Backends that fail to render are no longer advertised in the tenant proxy allowlist.
  - `TenantProxyConfigured` reports the rendered backend count.
  - `TenantProxy*` conditions reset when a tenant leaves MTLS mode.
  - The backend transport confirmation annotation is removed once consumed.
  - The managed Envoy proxy container now has CPU and memory requests and limits, configurable via `kubelb.envoyProxy.resources`. (#554)
- Fixed intermittent 503s on L7 routes after a backend Service was replaced. Upgrading rolls the envoy proxy pods in each tenant namespace once. (#553)
- Fixed the Envoy xDS control plane republishing an unchanged configuration to every connected proxy on each reconcile, caused by an ordering-dependent snapshot version. The xDS gRPC server now sets keepalive and connection-age limits, and drains streams on shutdown instead of resetting them. (#497)
- The xDS NACK alert now fires only on sustained rejection storms, not on self-healing NACKs from routine route churn.
  - gRPC health checks now set HTTP/2 protocol options on the generated cluster instead of rendering config Envoy rejects.
  - Editing an accepted route to a disallowed hostname now tears down the previously mirrored configuration.
  - The CCM retries Gateway API CRD installation with backoff at startup instead of crashlooping on a slow apiserver.
  - Orphaned Gateway mirrors are now cleaned up after a tenant cluster rebuild, preventing a leaked Envoy Gateway proxy and cloud LoadBalancer.
  - The `TenantWAFPolicy` Accepted status is now shown in kubectl output. (#560)
- Routes rejected because a hostname falls outside the tenant's `allowedDomains` now have their generated resources removed, instead of continuing to serve the previously accepted configuration. Disabling a resource type on a Tenant or Config also removes the mirrored resource, which was previously left behind. (#540)
- Route rejections now appear on the tenant cluster object's status instead of only in the management cluster. (#537)
- Gateways naming a GatewayClass that KubeLB does not serve now get a `GatewayClassNotAccepted` warning event instead of being ignored silently. (#538)
- Fixed several WAF correctness and security issues: `failureMode` now governs runtime WASM failures, invalid fail-closed policies now block traffic instead of serving it unprotected, per-route policy resolution no longer leaks between routes sharing an origin name, Ingress routes can now be protected, and `--enable-waf=false` fully disables WAF filter injection. (#443)
- WAFPolicy directive validation now rejects directives that perform I/O at parse time (`SecRemoteRules`, filesystem `Include`, and audit/log/tmp/data path directives) and correctly validates multi-line directive entries. Such policies are now marked invalid instead of being accepted and failing at runtime. (#442)
- Fix admin WAFPolicy changes triggering an Envoy snapshot rebuild for every tenant. Reconciles now fire only on meaningful changes and scope to the targeted namespace where possible. (#485)
- Fixed a leak where the Envoy xDS snapshot for a deleted tenant was retained until the manager restarted, if that tenant had any Tunnels. Tunnels are now deleted during tenant cleanup. (#500)
- Tunnel status now reports a single `Ready` condition (phase as reason) instead of accumulating one condition per phase; stale phase-typed conditions are removed automatically. (#498)
- Tunnel token validation now uses a constant-time comparison. Successful tunnel authentication is logged at `-v=2` and requests rejected for missing headers now log which headers were absent. (#525)
- Setting a tunnel or loadBalancer limit on a Tenant no longer overrides disable; disabled features stay disabled in the projected TenantState. (#499)
- Fix two tenant RBAC gaps that stopped the CCM from propagating resources when running with the scoped tenant ServiceAccount, and left the mTLS tenant proxy uncreated. (#536)
- Upstream TLS CA bundles (`kubelb.k8c.io/backend-tls-ca-secret`) are now carried as SyncSecrets instead of native Secrets. Under the scoped tenant ServiceAccount that write was forbidden and hung rather than erroring, wedging every subsequent Service reconcile behind the first `Verify`-policy Service. Bundles are also keyed on the origin UID, so two Services in different tenant-cluster namespaces referencing the same Secret name no longer overwrite each other. (#544)
- The KKP integration ClusterRole can now read Insight resources. (#513)
- CCM now emits a Warning event on a LoadBalancer Service when a non-default `sessionAffinityConfig.clientIP.timeoutSeconds` is set, since source-IP session affinity uses Maglev hashing and has no stickiness timeout to honour. (#561)
- Fixed `nginx.ingress.kubernetes.io/limit-rps` and `limit-rpm` annotation values above 4294967295 wrapping to a very low rate limit. (#519)
- Fix tenant reconciler failing with `no matches for kind "PodMonitor"` on clusters without the Prometheus Operator CRDs installed. (#391)
- Bump cert-manager addon to 1.21.0 and fix `global.imageRegistry` rewriting for it in air-gapped setups. (#476)
- Air-gapped: published `kubelb-manager-ee` chart now ships `airgapped/mirror-images.sh` with the executable bit set; customers no longer need a manual `chmod +x` step. (#395)

#### EE Other (Cleanup, Flake, or Chore)

- Bump `kubelb-addons` chart to v0.5.0 (cert-manager 1.21.1, agentgateway 1.4.1) and drop the `XListenerSet` CRD removed upstream in Gateway API v1.5. (#551)
- KubeLB is now built with Go 1.26.5. (#400, #431)

### Release Artifacts

#### Community Edition

For Community Edition, the release artifacts are available on [GitHub Releases](https://github.com/kubermatic/kubelb/releases/tag/v1.5.0).

The `kubelb-cli_<version>_<os>_<arch>` archives on that page ship the KubeLB CLI and are used by both editions.

#### Enterprise Edition

<details>
<summary><b>Docker Images</b></summary>

```bash
# Login to registry
docker login quay.io -u <username> -p <password>

# kubelb manager
docker pull quay.io/kubermatic/kubelb-manager-ee:v1.5.0

# ccm
docker pull quay.io/kubermatic/kubelb-ccm-ee:v1.5.0

# connection-manager
docker pull quay.io/kubermatic/kubelb-connection-manager-ee:v1.5.0
```

</details>

<details>
<summary><b>Helm Charts</b></summary>

```bash
# kubelb-manager
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-manager-ee --version v1.5.0

# kubelb-ccm
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-ccm-ee --version v1.5.0

# kubelb-addons
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-addons --version v0.5.0

# kubelb-dashboard
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-dashboard --version v1.1.0
```

</details>

<details>
<summary><b>SBOMs</b></summary>

Container image SBOMs are attached as OCI artifacts and attested with cosign.

**Pull SBOM:**

```bash
# Login to registry
oras login quay.io -u <username> -p <password>

## kubelb-manager
SBOM_DIGEST=$(oras discover --format json --artifact-type application/spdx+json \
  quay.io/kubermatic/kubelb-manager-ee:v1.5.0 | jq -r '.referrers[0].digest')
oras pull quay.io/kubermatic/kubelb-manager-ee@${SBOM_DIGEST} --output sbom/

## kubelb-ccm
SBOM_DIGEST=$(oras discover --format json --artifact-type application/spdx+json \
  quay.io/kubermatic/kubelb-ccm-ee:v1.5.0 | jq -r '.referrers[0].digest')
oras pull quay.io/kubermatic/kubelb-ccm-ee@${SBOM_DIGEST} --output sbom/

## kubelb-connection-manager
SBOM_DIGEST=$(oras discover --format json --artifact-type application/spdx+json \
  quay.io/kubermatic/kubelb-connection-manager-ee:v1.5.0 | jq -r '.referrers[0].digest')
oras pull quay.io/kubermatic/kubelb-connection-manager-ee@${SBOM_DIGEST} --output sbom/
```

**Verify SBOM attestation:**

```bash
cosign verify-attestation quay.io/kubermatic/kubelb-manager-ee:v1.5.0 \
  --type spdxjson \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify-attestation quay.io/kubermatic/kubelb-ccm-ee:v1.5.0 \
  --type spdxjson \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify-attestation quay.io/kubermatic/kubelb-connection-manager-ee:v1.5.0 \
  --type spdxjson \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

</details>

<details>
<summary><b>Verify Signatures</b></summary>

**Docker images:**

```bash
cosign verify quay.io/kubermatic/kubelb-manager-ee:v1.5.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/kubelb-ccm-ee:v1.5.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/kubelb-connection-manager-ee:v1.5.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

**Helm charts:**

```bash
cosign verify quay.io/kubermatic/helm-charts/kubelb-manager-ee:v1.5.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/helm-charts/kubelb-ccm-ee:v1.5.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/helm-charts/kubelb-addons:v0.5.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb/.github/workflows/release.yml@refs/tags/addons-v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

**Release checksums (requires repository access):**

```bash
cosign verify-blob --bundle checksums.txt.sigstore.json checksums.txt \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

</details>

<details>
<summary><b>Tools</b></summary>

- [Cosign](https://github.com/sigstore/cosign) - Container signing
- [ORAS](https://oras.land) - OCI Registry As Storage

</details>
