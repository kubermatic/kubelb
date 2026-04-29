# Kubermatic KubeLB v1.4

- [v1.4.0](#v140)
  - [Community Edition](#community-edition)
  - [Enterprise Edition](#enterprise-edition)

## v1.4.0

**GitHub release: [v1.4.0](https://github.com/kubermatic/kubelb/releases/tag/v1.4.0)**

### Highlights

#### KubeLB Dashboard

KubeLB v1.4 ships with a brand-new **[KubeLB Dashboard]({{< relref "../dashboard" >}})** — a web UI for browsing tenants, LoadBalancers, Routes,  WAF policies etc. across the fleet. A single chart and binary cover both Community and Enterprise editions; the edition is detected internally at runtime.

- Install via OCI Helm chart: `oci://quay.io/kubermatic/helm-charts/kubelb-dashboard`
- OIDC authentication opt-in, optional out-of-cluster kubeconfig
- Source: [kubermatic/kubelb-dashboard](https://github.com/kubermatic/kubelb-dashboard)

#### Air-Gap / Offline Deployment Support

KubeLB v1.4 Enterprise Edition is fully supported in air-gapped environments. Every release now publishes the complete list of container images and Helm charts shipped by KubeLB (including addons) as a release artifact, so customers can mirror them into an internal registry and pin-scan them from their own supply chain.

- `global.imageRegistry` is honoured end-to-end, including by bundled addon sub-charts, so a single Helm flag rewrites every image reference.
- CI nightly runs the full e2e suite with registry egress blocked via iptables to catch regressions.

#### Network Policies for Tenant Isolation (EE)

KubeLB now ships first-class Kubernetes **NetworkPolicies for tenant isolation** out of the box. Policies can be enabled or disabled, and additional policies can be added, at both Global/Config and Tenant level — so platform operators get a safe default while still being able to layer in per-tenant overrides.

#### Web Application Firewall promoted to Beta (EE)

#### Upstream TLS for Backend Connections (EE)

LoadBalancer now has a `spec.upstreamTLS` field that controls how KubeLB's Envoy proxy speaks to tenant backends. Two modes are supported:

- `Insecure` — skip upstream certificate verification (useful for self-signed or short-lived backend certs).
- `Verify` — validate the backend certificate against a CA bundle sourced from a Secret.

Tenants configure this with two annotations on the backing Service in the tenant cluster:

- `kubelb.k8c.io/backend-tls-policy` — selects the mode.
- `kubelb.k8c.io/backend-tls-ca-secret` — points at the Secret holding the CA.

#### Community Edition (CE)

- **Per-tenant Envoy Proxy sizing**: tenants can override Envoy Proxy `replicas` and `resources` via `tenant.spec.envoyProxy`, enabling per-tenant scaling without affecting other tenants.
- **Gateway API CRDs upgraded to v1.5.1** and `ingress-nginx` bumped to v1.15.0 (addresses [CVE-2026-3288](https://github.com/kubernetes/kubernetes/issues/137560)).
- **Proxy Protocol v2** and `externalTrafficPolicy` preservation for LoadBalancer services, for client-IP preservation on TCP services.
- **PodMonitor support for Envoy Proxy** pods via `spec.envoyProxy.podMonitor.enabled`.
- **Status for Tenant and SyncSecret** CRs, so operators can see health at a glance.
- **EDS/CDS split in xDS snapshots**, reducing payload size on endpoint changes.
- **Dual-stack and IPv6-only improvements**.
- Built on Go 1.26.2.

#### Enterprise Edition (EE)

- **Configurable load balancing policy** — pick `RoundRobin`, `LeastRequest`, or `Random` per LoadBalancer/Route, per tenant, or globally via the `kubelb.k8c.io/lb-policy` annotation.
- **Web Application Firewall (WAF) promoted to Beta**.
- **Endpoint cap and node-address filtering** — new `MaxEndpointsPerCluster` on the Config CR, plus `--max-node-address-count` and `--node-address-label-selector` CCM flags with topology-aware zone spread.
- **`imagePullSecrets` for Envoy Proxy** — auto-detected from the manager pod or set explicitly on `spec.envoyProxy.imagePullSecrets`.

#### Stability & Reliability

A broad set of bug fixes land in v1.4, several already back-ported to the v1.3.x patch series:

- Force TCP listener for Ingresses annotated with `nginx.ingress.kubernetes.io/ssl-passthrough: "true"`, fixing TLS handshake failures.
- Restore raw TCP passthrough for Ingresses with `backend-protocol: HTTPS` / `GRPCS`, fixing 502 Bad Gateway on TLS backends.
- Enable WebSocket `UpgradeConfigs` on the Envoy `HttpConnectionManager`.
- Fix `observedGeneration` drift on HTTPRoute / GRPCRoute / Gateway status sync (ArgoCD "perpetually progressing" bug).
- Fix long reconciliation loop when creating HTTPRoutes with ClusterIP backends.
- Restore the Envoy default `per_connection_buffer_limit_bytes` (removes the lowered 32 KiB cap).
- Increased `timeoutSeconds` and `failureThreshold` on Envoy proxy probes, plus readiness-probe stability under stats-scrape contention.

### Community Edition

#### Urgent Upgrade Notes

**(No, really, you MUST read this before you upgrade)**

- `ShutdownManagerImage` and `Image` fields have been dropped from Community Edition. ([#388](https://github.com/kubermatic/kubelb/pull/388))

#### API Change

- Preserve `externalTrafficPolicy` field for LoadBalancer services. Support for Proxy Protocol v2 for preserving client IP for TCP services. ([#276](https://github.com/kubermatic/kubelb/pull/276))

#### Features

- Add support for PodMonitor creation for Envoy Proxy pods. Enable via `spec.envoyProxy.podMonitor.enabled: true` in the Config CR. ([#313](https://github.com/kubermatic/kubelb/pull/313))
- Tenant and SyncSecret now expose status to represent their health. ([#375](https://github.com/kubermatic/kubelb/pull/375))
- Tenants can now override Envoy Proxy `replicas` and `resources` via `tenant.spec.envoyProxy`. This allows per-tenant scaling of Envoy proxies without affecting other tenants. ([#377](https://github.com/kubermatic/kubelb/pull/377))
- Separate EDS from CDS in xDS snapshots to reduce payload size on endpoint changes. ([#304](https://github.com/kubermatic/kubelb/pull/304))
- Improvements for dual-stack and IPv6-only support. ([#265](https://github.com/kubermatic/kubelb/pull/265))
- Fix empty IP in envoy access logs for TCP/UDP listeners. ([#269](https://github.com/kubermatic/kubelb/pull/269))
- Gateway API CRDs have been upgraded to v1.5.1. ([#402](https://github.com/kubermatic/kubelb/pull/402))
- Upgrade ingress-nginx to v1.15.0 to mitigate [CVE-2026-3288](https://github.com/kubernetes/kubernetes/issues/137560). ([#315](https://github.com/kubermatic/kubelb/pull/315))
- Upgrade kubelb-addons sub charts to latest versions:
  - ingress-nginx: 4.15.0 → 4.15.1
  - cert-manager: 1.20.0 → v1.20.2
  - kgateway: v2.1.2 → v2.2.3
  - kgateway-crds: v2.1.2 → v2.2.3 ([#387](https://github.com/kubermatic/kubelb/pull/387))
- KubeLB is now built using Go 1.25.7. ([#270](https://github.com/kubermatic/kubelb/pull/270))
- KubeLB is now built using Go 1.26.1. ([#314](https://github.com/kubermatic/kubelb/pull/314))
- KubeLB is now built using Go 1.26.2. ([#389](https://github.com/kubermatic/kubelb/pull/389))

#### Bug or Regression

- Fix a bug where HTTP listeners were being used for Ingresses annotated with `nginx.ingress.kubernetes.io/ssl-passthrough: "true"` and causing TLS handshake failures. ([#403](https://github.com/kubermatic/kubelb/pull/403))
- Fix 502 Bad Gateway for Ingress resources annotated with `nginx.ingress.kubernetes.io/backend-protocol: HTTPS` (or `GRPCS`) when the backend speaks TLS. Restores 1.2.x raw TCP passthrough for this specific case so the upstream nginx ingress controller can complete its TLS handshake against the backend through kubelb. ([#376](https://github.com/kubermatic/kubelb/pull/376))
- Fix WebSocket connections failing through KubeLB Layer 7 proxy by adding `UpgradeConfigs` to Envoy `HttpConnectionManager`. ([#328](https://github.com/kubermatic/kubelb/pull/328))
- Fix lengthy reconciliation loop when creating HTTPRoutes with ClusterIP backend services. ([#330](https://github.com/kubermatic/kubelb/pull/330))
- Fix `observedGeneration` mismatch in HTTPRoute / GRPCRoute / Gateway status sync that caused ArgoCD to show resources as perpetually progressing. ([#373](https://github.com/kubermatic/kubelb/pull/373))
- Fix envoy proxy readiness probe failures under stats-scrape contention at scale. ([#303](https://github.com/kubermatic/kubelb/pull/303))
- Increased `timeoutSeconds` and `failureThreshold` on envoy-proxy probes. ([#287](https://github.com/kubermatic/kubelb/pull/287))
- Use the default value for `per_connection_buffer_limit_bytes` instead of a lowered 32 KiB buffer limit. ([#345](https://github.com/kubermatic/kubelb/pull/345))
- Add explicit namespace to all namespaced-scoped resources in helm charts. ([#292](https://github.com/kubermatic/kubelb/pull/292))
- Fix duplicate port allocations for Routes that shared a backend Service before upgrade, and fix multi-port LoadBalancer target-port corruption that could cause traffic drop after a manager restart while the API server was write-unavailable. ([#409](https://github.com/kubermatic/kubelb/pull/409))

#### Other (Cleanup, Flake, or Chore)

- Kgateway has been replaced with agentgateway. Upstream Kgateway moved all the AI-extension functionality out of kgateway into agentgateway. ([#395](https://github.com/kubermatic/kubelb/pull/395))
- The "Global" Envoy Proxy Topology has been removed in favor of the default "Shared". ([#273](https://github.com/kubermatic/kubelb/pull/273))
- The manager now operates with sensible defaults (shared topology, 3 replicas) when the Config CR is absent. ([#318](https://github.com/kubermatic/kubelb/pull/318))
- Move ingress conversion logic to `pkg/conversion`. ([#261](https://github.com/kubermatic/kubelb/pull/261))
- Bump `kubelb-addons` to v0.3.1 for `kubelb-manager` Helm chart. ([#258](https://github.com/kubermatic/kubelb/pull/258))
- Update `kubelb-addons` chart to v0.3.1 with dependency bumps:
  - **ingress-nginx 4.14.1 → 4.14.3**:
    [CVE-2026-1580](https://github.com/kubernetes/kubernetes/issues/136677),
    [CVE-2026-24512](https://github.com/kubernetes/kubernetes/issues/136678),
    [CVE-2026-24513](https://github.com/kubernetes/kubernetes/issues/136679),
    [CVE-2026-24514](https://github.com/kubernetes/kubernetes/issues/136680).
  - **envoy-gateway 1.6.2 → 1.6.3**:
    [CVE-2025-0913](https://nvd.nist.gov/vuln/detail/CVE-2025-0913).
  - **cert-manager v1.19.2 → v1.19.3**:
    [GHSA-gx3x-vq4p-mhhv](https://github.com/cert-manager/cert-manager/security/advisories/GHSA-gx3x-vq4p-mhhv).
  ([#257](https://github.com/kubermatic/kubelb/pull/257))

**Full Changelog**: <https://github.com/kubermatic/kubelb/compare/v1.3.0...v1.4.0>

### Enterprise Edition

**Enterprise Edition includes everything from Community Edition and more. The release notes below are for changes specific to just the Enterprise Edition.**

#### EE Features

- **Air-gap deployment support.** The list of all images and charts shipped by KubeLB (including addons) is now published with each release as part of the release artifacts. (#354)
- **Web Application Firewall (WAF) promoted from Alpha to Beta.** (#362)
- **Network policies for tenant isolation.** KubeLB creates Kubernetes NetworkPolicies for tenant isolation out of the box. Policies can be enabled / disabled and additional policies can be layered in at Global/Config and Tenant level. (#316)
- **Upstream TLS for backend connections.** New `spec.upstreamTLS` field on the LoadBalancer CRD with two modes: `Insecure` (skip cert verification) and `Verify` (validate against CA certificate from Secret). Configurable via `kubelb.k8c.io/backend-tls-policy` and `kubelb.k8c.io/backend-tls-ca-secret` annotations on tenant Services. (#351)
- **Configurable load balancing policy.** Pick `RoundRobin`, `LeastRequest`, or `Random` per LoadBalancer/Route, per tenant, or globally via the `kubelb.k8c.io/lb-policy` annotation on tenant Services or Ingresses. (#317)
- **Endpoint cap + node-address filtering.** Add `MaxEndpointsPerCluster` to the Config CR to cap upstream endpoints in the Envoy xDS snapshot. New `--max-node-address-count` and `--node-address-label-selector` flags on the CCM filter / limit node addresses forwarded to the LB cluster. When `--max-node-address-count` is enabled, addresses are selected with topology-aware round-robin spread across zones (`topology.kubernetes.io/zone`). (#295)
- **`imagePullSecrets` for Envoy Proxy pods.** Secrets are auto-detected from the manager pod or can be explicitly configured via `spec.envoyProxy.imagePullSecrets` in the Config CR. (#308)

#### EE Bug or Regression

- Fix lengthy reconciliation loop when creating HTTPRoutes with ClusterIP backend services. (#332)

### Release Artifacts

#### Community Edition

For Community Edition, the release artifacts are available on [GitHub Releases](https://github.com/kubermatic/kubelb/releases/tag/v1.4.0).

#### Enterprise Edition

<details>
<summary><b>Docker Images</b></summary>

```bash
# Login to registry
docker login quay.io -u <username> -p <password>

# kubelb manager
docker pull quay.io/kubermatic/kubelb-manager-ee:v1.4.0

# ccm
docker pull quay.io/kubermatic/kubelb-ccm-ee:v1.4.0

# connection-manager
docker pull quay.io/kubermatic/kubelb-connection-manager-ee:v1.4.0
```

</details>

<details>
<summary><b>Helm Charts</b></summary>

```bash
# kubelb-manager
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-manager-ee --version v1.4.0

# kubelb-ccm
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-ccm-ee --version v1.4.0

# kubelb-addons
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-addons --version v0.4.0

# kubelb-dashboard
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-dashboard --version v1.0.0
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
  quay.io/kubermatic/kubelb-manager-ee:v1.4.0 | jq -r '.referrers[0].digest')
oras pull quay.io/kubermatic/kubelb-manager-ee@${SBOM_DIGEST} --output sbom/

## kubelb-ccm
SBOM_DIGEST=$(oras discover --format json --artifact-type application/spdx+json \
  quay.io/kubermatic/kubelb-ccm-ee:v1.4.0 | jq -r '.referrers[0].digest')
oras pull quay.io/kubermatic/kubelb-ccm-ee@${SBOM_DIGEST} --output sbom/

## kubelb-connection-manager
SBOM_DIGEST=$(oras discover --format json --artifact-type application/spdx+json \
  quay.io/kubermatic/kubelb-connection-manager-ee:v1.4.0 | jq -r '.referrers[0].digest')
oras pull quay.io/kubermatic/kubelb-connection-manager-ee@${SBOM_DIGEST} --output sbom/
```

**Verify SBOM attestation:**

```bash
cosign verify-attestation quay.io/kubermatic/kubelb-manager-ee:v1.4.0 \
  --type spdxjson \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify-attestation quay.io/kubermatic/kubelb-ccm-ee:v1.4.0 \
  --type spdxjson \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify-attestation quay.io/kubermatic/kubelb-connection-manager-ee:v1.4.0 \
  --type spdxjson \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

</details>

<details>
<summary><b>Verify Signatures</b></summary>

**Docker images:**

```bash
cosign verify quay.io/kubermatic/kubelb-manager-ee:v1.4.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/kubelb-ccm-ee:v1.4.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/kubelb-connection-manager-ee:v1.4.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

**Helm charts:**

```bash
cosign verify quay.io/kubermatic/helm-charts/kubelb-manager-ee:v1.4.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/helm-charts/kubelb-ccm-ee:v1.4.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/helm-charts/kubelb-addons:v0.4.0 \
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

- [Cosign](https://github.com/sigstore/cosign) — Container signing
- [ORAS](https://oras.land) — OCI Registry As Storage

</details>
