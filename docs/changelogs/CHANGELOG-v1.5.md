# KubeLB v1.5 Changelog

- [v1.5.0](#v150)
  - [Community Edition](#community-edition)
  - [Enterprise Edition](#enterprise-edition)

## v1.5.0

**GitHub release: [v1.5.0](https://github.com/kubermatic/kubelb/releases/tag/v1.5.0)**

### Community Edition


### Chore

- Bump Go to 1.26.3. ([#453](https://github.com/kubermatic/kubelb/pull/453))
- Bump addon charts: envoy-gateway 1.8.3, metallb 0.16.1, external-dns 1.21.1, agentgateway 1.4.0
  - metallb no longer deploys a kube-rbac-proxy sidecar ([#542](https://github.com/kubermatic/kubelb/pull/542))
- Bump addons to v0.5.0
  - Bump agentgateway and agentgateway-crds addon charts to 1.4.1 ([#564](https://github.com/kubermatic/kubelb/pull/564))
- KubeLB binaries and images are now built with Go 1.26.6. ([#577](https://github.com/kubermatic/kubelb/pull/577))

### API Change

- Add `deniedAnnotations` to Config and Tenant for excluding annotation keys from propagation. Keys in `propagatedAnnotations` and `deniedAnnotations` now support shell-style glob patterns (e.g. `nginx.ingress.kubernetes.io/*`). ([#454](https://github.com/kubermatic/kubelb/pull/454))

### Feature

- Add optional AI usage recording rules (aiRecordingRules, default off) and enable RateLimit response headers on the ratelimit addon. ([#527](https://github.com/kubermatic/kubelb/pull/527))
- Add optional SourceIP persistence for Layer 4 LoadBalancers. Services using `sessionAffinity: ClientIP` are propagated with observed-source persistence in KubeLB. ([#435](https://github.com/kubermatic/kubelb/pull/435))
- Add optional valkey and Envoy ratelimit addon subcharts (disabled by default) to the kubelb-addons chart. ([#526](https://github.com/kubermatic/kubelb/pull/526))
- Gateway API CRDs installed by the KubeLB CCM are now labeled with `kubelb.k8c.io/managed-by=kubelb`. ([#487](https://github.com/kubermatic/kubelb/pull/487))
- KubeLB's managed Envoy request/response header size and count limits are now configurable via `Config.spec.envoyProxy.headerLimits`, defaulting to Envoy's maximum so large client headers no longer cause `431 Request Header Fields Too Large`. ([#519](https://github.com/kubermatic/kubelb/pull/519))
- Upgrade to Go 1.26.4 ([#476](https://github.com/kubermatic/kubelb/pull/476))

### Bug or Regression

- Deleting a Tenant now clears its Envoy snapshot from the xDS cache instead of leaving it until the manager restarts. ([#534](https://github.com/kubermatic/kubelb/pull/534))
- Enable TCP keepalive on connections through the KubeLB-managed Envoy proxy so idle connections are not silently dropped by kube-proxy in IPVS mode. Existing connections are drained once when the updated listener config is pushed. ([#540](https://github.com/kubermatic/kubelb/pull/540))
- Envoy control plane now uses gRPC keepalive and bounded connection age, drains xDS streams on shutdown, and exports `kubelb_envoy_control_plane_xds_nacks_total` for configs rejected by Envoy. ([#532](https://github.com/kubermatic/kubelb/pull/532))
- Envoy snapshot versions no longer change when the underlying config is unchanged, which stopped needless xDS pushes to every connected proxy. Snapshot consistency is now validated on the first push as well as subsequent ones. ([#533](https://github.com/kubermatic/kubelb/pull/533))
- Fix CCM dropping a node reconcile when the tenant's Addresses object was updated concurrently, which could leave endpoints stale until the next node event. ([#553](https://github.com/kubermatic/kubelb/pull/553))
- Fix generated Ingress/Gateway/HTTPRoute/GRPCRoute not being removed when Ingress or Gateway API is disabled for a tenant or globally
  - Gateways naming a GatewayClass that KubeLB does not serve now get a Warning event instead of being silently ignored ([#568](https://github.com/kubermatic/kubelb/pull/568))
- Fix generated KubeLB Route service names to comply with Kubernetes DNS-1035 validation. ([#477](https://github.com/kubermatic/kubelb/pull/477))
- Fix global.imageRegistry rewriting for the cert-manager addon after the subchart bump to 1.21.0 ([#524](https://github.com/kubermatic/kubelb/pull/524))
- Fix manager and CCM ClusterRoles missing permission to create events, which silently dropped every event the controllers emitted. ([#531](https://github.com/kubermatic/kubelb/pull/531))
- Fix tenant reconciler failing with `no matches for kind "PodMonitor"` on clusters without the Prometheus Operator CRDs installed. ([#439](https://github.com/kubermatic/kubelb/pull/439))
- Fixed Gateway objects from different tenant cluster namespaces silently overwriting each other in the management cluster. The first namespace to claim a Gateway name keeps it and later claimants get an error on the Route. Errors from applying Ingress, Gateway, HTTPRoute and GRPCRoute objects are now reported as a condition and event on the Route and retried, instead of being discarded. ([#557](https://github.com/kubermatic/kubelb/pull/557))
- Fixed Routes staying broken after a transient backing-Service apply failure; these now requeue and recover automatically. ([#516](https://github.com/kubermatic/kubelb/pull/516))
- Fixed a potential manager crash (concurrent map read and map write) caused by unsynchronized reads of the port allocator. ([#515](https://github.com/kubermatic/kubelb/pull/515))
- Generated xDS cluster/listener names no longer include the Service UID; envoy proxy pods roll once on upgrade to pick up the new names.
  - Orphaned LoadBalancer/Route mirrors in the management cluster are now cleaned up when the tenant origin resource is deleted, recreated, or no longer qualifies.
  - Removing an annotation from a tenant Service now removes it from the generated LoadBalancer Service as well.
  - The managed Envoy proxy container now defaults to 200m CPU / 256Mi memory requests and 2 CPU / 1Gi memory limits; envoyProxy.resources in Config/Tenant overrides this. ([#570](https://github.com/kubermatic/kubelb/pull/570))
- HTTP idle-connection timeout default raised from 60s to 1h, and per-request timeout default changed from 15s (Envoy default) to disabled (streaming-friendly) ([#434](https://github.com/kubermatic/kubelb/pull/434))
- KubeLB binaries and images are now built with Go 1.26.5, addressing CVE-2026-42505 and CVE-2026-39822. ([#498](https://github.com/kubermatic/kubelb/pull/498))

### Other (Cleanup or Flake)

- The KubeLB manager ClusterRole no longer requests unused create/bind/escalate permissions on cluster-scoped clusterroles. ([#514](https://github.com/kubermatic/kubelb/pull/514))

### Uncategorized

- LoadBalancer and Route endpoints with hostnames (FQDNs) are now translated into Envoy STRICT_DNS clusters instead of EDS clusters, fixing "malformed IP address" rejections for tenants that use hostname-based
  endpoints. 
  - The `ip` field on EndpointAddress is now optional; at least one of `ip` or `hostname` must be set. ([#438](https://github.com/kubermatic/kubelb/pull/438))

### Enterprise Edition

**Enterprise Edition includes everything from Community Edition and more. The release notes below are for changes specific to just the Enterprise Edition.**


### EE Chore

- Bump Go to 1.26.3. (#400)
- Bump kubelb-addons chart to v0.5.0 (cert-manager 1.21.1, agentgateway 1.4.1)
  - Drop the XListenerSet CRD, removed upstream in Gateway API v1.5 (#551)
- Fixed `nginx.ingress.kubernetes.io/limit-rps` and `limit-rpm` annotation values above 4294967295 wrapping to a very low rate limit. (#519)
- KubeLB is now built with Go 1.26.5. (#431)

### EE API Change

- Add KubeLB Insights, an opt-in engine (`--enable-insights`) that evaluates the management cluster against a registry of checks and records findings as `Insight` resources, with triage (acknowledge, snooze, dismiss), per-check suppression via `Config.spec.insights.disabledChecks`, and Prometheus metrics. (#507)
- Added support for Gateway API ReferenceGrants. Cross-namespace backendRefs and Gateway TLS certificateRefs can now be restricted to references permitted by a ReferenceGrant, via `spec.gatewayAPI.enforceReferenceGrants` on Config/Tenant (default off). Also fixed CCM RBAC so events on tenant objects are no longer rejected. (#435)
- Cap admin WAFPolicy directives at 64 items / 1024 characters and default `Config.spec.waf.maxDirectivesPerPolicy` and `maxDirectiveLength` to 64/1024. Set either to 0 for unlimited. (#486)
- EE: Envoy active health checks (TCP/HTTP/gRPC) are now configurable on Config, Tenant, LoadBalancer, and Route, and via kubelb.k8c.io/health-check-* annotations on tenant Services, Ingresses, and Routes. (#471)
- LoadBalancer status now carries an `Accepted` condition, surfaced by the CCM as a Warning event on the tenant Service. (#556)
- Tenants can now map multiple Gateway API GatewayClasses to distinct management-cluster GatewayClasses via `Tenant.spec.gatewayAPI.classMappings` and `Config.spec.gatewayAPI.classMappings`. (#423)

### EE Feature

- AI gateway access logs now carry tenant_id/key_id attribution matching the metrics, and budget/rate-limit 429 responses include RateLimit-Limit, RateLimit-Remaining and RateLimit-Reset headers. (#481)
- Add Grafana dashboards and Prometheus alerts for WAF: blocked requests, filter failures, Coraza VM reloads, and xDS NACKs. Alerts are off by default (`prometheusRule.enabled`). (#484)
- Add Prometheus alert rules for KubeLB Insights: critical findings, degraded tenant posture, checks failing to evaluate, and a stalled sweep loop. (#512)
- Add fleet-wide insights checks (hostname collisions across tenants, silently stripped certificate annotations, unprotected HTTP routes, quota headroom, WAF failure-mode and network policy asymmetry, AI budget alert thresholds) and the `kubelb_manager_posture_score` metric, scored per tenant and category. (#511)
- Add mTLS backend transport mode for encrypted traffic between KubeLB management and tenants (#387)
- Add optional SourceIP persistence for Layer 4 LoadBalancers via `spec.persistence.type: SourceIP`
  - Services using `sessionAffinity: ClientIP` are propagated with observed-source persistence in KubeLB
  - Persistence takes precedence over `loadBalancerPolicy` when both are set (#550)
- Add optional `Config.spec.ai.prometheus`: when set, KubeLB writes each VirtualKey's token spend to its status so tenants see usage in their own cluster. (#480)
- Adds configurable Envoy timeouts at Config, Tenant, Route, and LoadBalancer levels. New `kubelb.k8c.io/timeout-*` annotations on Ingress/HTTPRoute/GRPCRoute/TCPRoute/TLSRoute propagate to `Route.Spec.Timeouts`. Six fields are configurable: `request`, `streamIdle`, `requestHeaders`, `idleConnection`, `tcpIdle`, `connect`.
  
  - HTTP idle-connection timeout default raised from 60s to 1h, and per-request timeout default changed from 15s (Envoy default) to disabled (streaming-friendly). To restore prior behavior, set `Config.Spec.Timeouts.IdleConnection: 60s` and `Config.Spec.Timeouts.Request: 15s`. (#386)
- Insights now reports mTLS backend transport certificates that entered their rotation window without being reissued. (#523)
- KubeLB Enterprise Edition users can group multiple LoadBalancer resources into one Gateway API backend pool with the `kubelb.k8c.io/backend-pool` annotation. (#440)
- Kubelb-addons can render Prometheus recording rules (aiRecordingRules.enabled) exposing stable kubelb:* series for AI token/request showback, as a PrometheusRule or a plain rule-file ConfigMap. (#483)
- Tenants can define namespaced TenantWAFPolicy resources to apply WAF rules to their own routes. The feature is opt-in per installation (Config.spec.waf.enableTenantPolicies) and per tenant (Tenant.spec.waf); tenant directives are validated against a strict allowlist and each policy is isolated to the tenant's own namespace. (#444)
- The KubeLB insights engine is now enabled by default. Set `kubelb.enableInsights: false` to turn it off. (#518)
- The mTLS tenant proxy exposure can now be overridden on the tenant side: kubelb.tenantProxy.serviceType on the kubelb-ccm chart overrides the Service type, and kubelb.tenantProxy.staticAddresses/staticPort publish static IPs or DNS names as the dial target for proxies behind an appliance, NAT, or user-managed DNS. (#528)

### EE Bug or Regression

- Air-gapped: published kubelb-manager-ee chart now ships airgapped/mirror-images.sh with the executable bit set; customers no longer need a manual chmod +x step. (#395)
- Bump cert-manager addon to 1.21.0 and fix global.imageRegistry rewriting for it in air-gapped setups (#476)
- CCM now emits a Warning event on a LoadBalancer Service when a non-default `sessionAffinityConfig.clientIP.timeoutSeconds` is set, since source-IP session affinity uses Maglev hashing and has no stickiness timeout to honour. (#561)
- Fix admin WAFPolicy changes triggering an Envoy snapshot rebuild for every tenant. Reconciles now fire only on meaningful changes and scope to the targeted namespace where possible. (#485)
- Fix tenant reconciler failing with `no matches for kind "PodMonitor"` on clusters without the Prometheus Operator CRDs installed. (#391)
- Fix two tenant RBAC gaps that stopped the CCM from propagating resources when running with the scoped tenant ServiceAccount, and left the mTLS tenant proxy uncreated. (#536)
- Fixed a leak where the Envoy xDS snapshot for a deleted tenant was retained until the manager restarted, if that tenant had any Tunnels. Tunnels are now deleted during tenant cleanup. (#500)
- Fixed intermittent 503s on L7 routes after a backend Service was replaced. Upgrading rolls the envoy proxy pods in each tenant namespace once. (#553)
- Fixed several WAF correctness and security issues: failureMode now governs runtime WASM failures, invalid fail-closed policies now block traffic instead of serving it unprotected, per-route policy resolution no longer leaks between routes sharing an origin name, Ingress routes can now be protected, and --enable-waf=false fully disables WAF filter injection. (#443)
- Fixed the Envoy xDS control plane republishing an unchanged configuration to every connected proxy on each reconcile, caused by an ordering-dependent snapshot version. The xDS gRPC server now sets keepalive and connection-age limits, and drains streams on shutdown instead of resetting them. (#497)
- Gateways named `kubelb` in more than one tenant cluster namespace resolved to a single object in the management cluster and silently overwrote each other. The first namespace to claim the name now keeps it and later claimants get an error on the Route. Gateways with any other name were already namespace-qualified and are unaffected. Routes whose sub-resources fail to apply are no longer marked Accepted; the failure is reported as a condition and event, and retried. (#529)
- Gateways naming a GatewayClass that KubeLB does not serve now get a GatewayClassNotAccepted warning event instead of being ignored silently (#538)
- KubeLB binaries and images are now built with Go 1.26.6. (#569)
- Mgmt-side mirrors are torn down when a Service stops being a LoadBalancer or its origin disappears
  - Annotation removals propagate to the generated Service
  - Backends that fail to render are no longer advertised in the tenant proxy allowlist
  - `TenantProxyConfigured` reports the rendered backend count
  - `TenantProxy*` conditions reset when a tenant leaves MTLS mode
  - The backend transport confirmation annotation is removed once consumed
  - The managed Envoy proxy container now has CPU and memory requests and limits, configurable via `kubelb.envoyProxy.resources` (#554)
- Route rejections now appear on the tenant cluster object's status instead of only in the management cluster (#537)
- Routes rejected because a hostname falls outside the tenant's `allowedDomains` now have their generated resources removed, instead of continuing to serve the previously accepted configuration. Disabling a resource type on a Tenant or Config also removes the mirrored resource, which was previously left behind. (#540)
- Setting a tunnel or loadBalancer limit on a Tenant no longer overrides disable; disabled features stay disabled in the projected TenantState. (#499)
- The KKP integration ClusterRole can now read Insight resources. (#513)
- The xDS NACK alert now fires only on sustained rejection storms, not on self-healing NACKs from routine route churn.
  - gRPC health checks now set HTTP/2 protocol options on the generated cluster instead of rendering config Envoy rejects.
  - Editing an accepted route to a disallowed hostname now tears down the previously mirrored configuration.
  - The CCM retries Gateway API CRD installation with backoff at startup instead of crashlooping on a slow apiserver.
  - Orphaned Gateway mirrors are now cleaned up after a tenant cluster rebuild, preventing a leaked Envoy Gateway proxy and cloud LoadBalancer.
  - The TenantWAFPolicy Accepted status is now shown in kubectl output. (#560)
- Tunnel status now reports a single `Ready` condition (phase as reason) instead of accumulating one condition per phase; stale phase-typed conditions are removed automatically. (#498)
- Tunnel token validation now uses a constant-time comparison. Successful tunnel authentication is logged at -v=2 and requests rejected for missing headers now log which headers were absent. (#525)
- WAFPolicy and TenantWAFPolicy no longer accept `targetRef.kind: Ingress`. WAF applies to Gateway API routes (HTTPRoute, GRPCRoute); it was never enforced on Ingress traffic. (#489)
- WAFPolicy directive validation now rejects directives that perform I/O at parse time (SecRemoteRules, filesystem Include, and audit/log/tmp/data path directives) and correctly validates multi-line directive entries. Such policies are now marked invalid instead of being accepted and failing at runtime. (#442)

### EE Other (Cleanup or Flake)

- AI spend metering no longer resets a VirtualKey's reported spend to 0 when Prometheus returns no data for a window; the prior value is retained. (#492)

### EE Uncategorized

- Tenants now receive a Kubernetes warning event on their Ingress / Gateway / *Route when KubeLB rejects it because the hostname is not in `tenant.spec.allowedDomains`. (#405)

### Release Artifacts

#### Community Edition

For Community Edition, the release artifacts are available on [GitHub Releases](https://github.com/kubermatic/kubelb/releases/tag/v1.5.0).

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

