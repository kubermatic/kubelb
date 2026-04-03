# KubeLB v99.0 Changelog

## v99.0.0

**GitHub release: [v99.0.0](https://github.com/kubermatic/kubelb/releases/tag/v99.0.0)**

### Community Edition


### Chore

- Bump kubelb-addons to v0.3.1 for kubelb-manager helm chart ([#258](https://github.com/kubermatic/kubelb/pull/258))

### API Change

- Preserve `externalTrafficPolicy` field for LoadBalancer services
  - Support for Proxy Protocol v2 for preserving client IP for TCP services ([#276](https://github.com/kubermatic/kubelb/pull/276))

### Feature

- Add support for PodMonitor creation for Envoy Proxy pods. Enable via `spec.envoyProxy.podMonitor.enabled: true` in the Config CR. ([#313](https://github.com/kubermatic/kubelb/pull/313))
- Fix empty IP in envoy access logs for TCP/UDP listeners ([#269](https://github.com/kubermatic/kubelb/pull/269))
- Improvements for dual-stack and IPv6 only support ([#265](https://github.com/kubermatic/kubelb/pull/265))
- KubeLB is now built using 1.25.7 ([#270](https://github.com/kubermatic/kubelb/pull/270))
- KubeLB is now built using Go 1.26.1 ([#314](https://github.com/kubermatic/kubelb/pull/314))
- Separate EDS from CDS in xDS snapshots to reduce payload size on endpoint changes ([#304](https://github.com/kubermatic/kubelb/pull/304))
- Upgrade ingress-nginx to v1.15.0 to mitigate CVE-2026-3288, for more details: https://github.com/kubernetes/kubernetes/issues/137560 ([#315](https://github.com/kubermatic/kubelb/pull/315))

### Bug or Regression

- Add explicit namespace to all namespaced scoped resources in helm charts ([#292](https://github.com/kubermatic/kubelb/pull/292))
- Fix WebSocket connections failing through KubeLB Layer 7 proxy by adding UpgradeConfigs to Envoy HttpConnectionManager ([#328](https://github.com/kubermatic/kubelb/pull/328))
- Fix envoy proxy readiness probe failures under stats scrape contention at scale ([#303](https://github.com/kubermatic/kubelb/pull/303))
- Fix lengthy reconciliation loop when creating HTTPRoutes with ClusterIP backend services ([#330](https://github.com/kubermatic/kubelb/pull/330))
- Increased TimeoutSeconds and FailureThresholds on envoy-proxy probes ([#287](https://github.com/kubermatic/kubelb/pull/287))
- Use default value for per_connection_buffer_limit_bytes instead of a lowered 32KB buffer limit. ([#345](https://github.com/kubermatic/kubelb/pull/345))

### Other (Cleanup or Flake)

- The "Global" Envoy Proxy Topology has been removed in favor of the default "Shared" ([#273](https://github.com/kubermatic/kubelb/pull/273))
- The manager now operates with sensible defaults (shared topology, 3 replicas) when the Config CR is absent. ([#318](https://github.com/kubermatic/kubelb/pull/318))

### Uncategorized

- Move ingress conversion logic to pkg/conversion ([#261](https://github.com/kubermatic/kubelb/pull/261))
- Updated kubelb-addons chart to v0.3.1 with dependency bumps: 
  
  #### ingress-nginx 4.14.1 → 4.14.3
  - [CVE-2026-1580](https://github.com/kubernetes/kubernetes/issues/136677) - auth-method nginx configuration injection
  - [CVE-2026-24512](https://github.com/kubernetes/kubernetes/issues/136678) - rules.http.paths.path nginx configuration injection
  - [CVE-2026-24513](https://github.com/kubernetes/kubernetes/issues/136679) - auth-url protection bypass
  - [CVE-2026-24514](https://github.com/kubernetes/kubernetes/issues/136680) - Admission Controller denial of service
  
  #### envoy-gateway 1.6.2 → 1.6.3
  - [CVE-2025-0913](https://nvd.nist.gov/vuln/detail/CVE-2025-0913) - Use-after-free in c-ares DNS resolver
  
  #### cert-manager v1.19.2 → v1.19.3
  - [GHSA-gx3x-vq4p-mhhv](https://github.com/cert-manager/cert-manager/security/advisories/GHSA-gx3x-vq4p-mhhv) - DoS via malformed DNS response
  
  ### References
  - [Envoy Gateway v1.6.3 Release](https://gateway.envoyproxy.io/news/releases/notes/v1.6.3/)
  - [cert-manager v1.19.3 Release](https://github.com/cert-manager/cert-manager/releases/tag/v1.19.3)
  - [ingress-nginx v1.14.3 Release](https://github.com/kubernetes/ingress-nginx/releases/tag/controller-v1.14.3) ([#257](https://github.com/kubermatic/kubelb/pull/257))

### Enterprise Edition

**Enterprise Edition includes everything from Community Edition and more. The release notes below are for changes specific to just the Enterprise Edition.**


### EE Feature

- Add imagePullSecrets support for envoy proxy pods. Secrets are auto-detected from the manager pod or can be explicitly configured via `spec.envoyProxy.imagePullSecrets` in the Config CR. (#308)
- Add`MaxEndpointsPerCluster` to Config CR to cap upstream endpoints in Envoy xDS snapshot. 
  - `--max-node-address-count` and `--node-address-label-selector` flags to CCM to filter/limit node addresses forwarded to the LB cluster. When `--max-node-address-count` is enabled, addresses are selected with topology-aware round-robin spread across zones (topology.kubernetes.io/zone). (#295)
- KubeLB creates network policies for tenant isolation. Policies can be enabled/disabled, additional policies can be added, etc. at Global/Config and Tenant level. (#316)
- Users can now configure the Envoy load balancing policy per LoadBalancer/Route, per tenant, or globally. Supported policies: RoundRobin, LeastRequest, and Random. Use the `kubelb.k8c.io/lb-policy` annotation on Services or Ingresses in tenant clusters. (#317)

### EE Bug or Regression

- Fix lengthy reconciliation loop when creating HTTPRoutes with ClusterIP backend services (#332)

### Release Artifacts

#### Community Edition

For Community Edition, the release artifacts are available on [GitHub Releases](https://github.com/kubermatic/kubelb/releases/tag/v99.0.0).

#### Enterprise Edition

<details>
<summary><b>Docker Images</b></summary>

```bash
# Login to registry
docker login quay.io -u <username> -p <password>

# kubelb manager
docker pull quay.io/kubermatic/kubelb-manager-ee:v99.0.0

# ccm
docker pull quay.io/kubermatic/kubelb-ccm-ee:v99.0.0

# connection-manager
docker pull quay.io/kubermatic/kubelb-connection-manager-ee:v99.0.0
```

</details>

<details>
<summary><b>Helm Charts</b></summary>

```bash
# kubelb-manager
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-manager-ee --version v99.0.0

# kubelb-ccm
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-ccm-ee --version v99.0.0

# kubelb-addons
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-addons --version v0.3.2
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
  quay.io/kubermatic/kubelb-manager-ee:v99.0.0 | jq -r '.referrers[0].digest')
oras pull quay.io/kubermatic/kubelb-manager-ee@${SBOM_DIGEST} --output sbom/

## kubelb-ccm
SBOM_DIGEST=$(oras discover --format json --artifact-type application/spdx+json \
  quay.io/kubermatic/kubelb-ccm-ee:v99.0.0 | jq -r '.referrers[0].digest')
oras pull quay.io/kubermatic/kubelb-ccm-ee@${SBOM_DIGEST} --output sbom/

## kubelb-connection-manager
SBOM_DIGEST=$(oras discover --format json --artifact-type application/spdx+json \
  quay.io/kubermatic/kubelb-connection-manager-ee:v99.0.0 | jq -r '.referrers[0].digest')
oras pull quay.io/kubermatic/kubelb-connection-manager-ee@${SBOM_DIGEST} --output sbom/
```

**Verify SBOM attestation:**

```bash
cosign verify-attestation quay.io/kubermatic/kubelb-manager-ee:v99.0.0 \
  --type spdxjson \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify-attestation quay.io/kubermatic/kubelb-ccm-ee:v99.0.0 \
  --type spdxjson \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify-attestation quay.io/kubermatic/kubelb-connection-manager-ee:v99.0.0 \
  --type spdxjson \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

</details>

<details>
<summary><b>Verify Signatures</b></summary>

**Docker images:**

```bash
cosign verify quay.io/kubermatic/kubelb-manager-ee:v99.0.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/kubelb-ccm-ee:v99.0.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/kubelb-connection-manager-ee:v99.0.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

**Helm charts:**

```bash
cosign verify quay.io/kubermatic/helm-charts/kubelb-manager-ee:v99.0.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/helm-charts/kubelb-ccm-ee:v99.0.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/helm-charts/kubelb-addons:v0.3.2 \
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

## v99.0.0

**GitHub release: [v99.0.0](https://github.com/kubermatic/kubelb/releases/tag/v99.0.0)**

### Community Edition


### Chore

- Bump kubelb-addons to v0.3.1 for kubelb-manager helm chart ([#258](https://github.com/kubermatic/kubelb/pull/258))

### API Change

- Preserve `externalTrafficPolicy` field for LoadBalancer services
  - Support for Proxy Protocol v2 for preserving client IP for TCP services ([#276](https://github.com/kubermatic/kubelb/pull/276))

### Feature

- Add support for PodMonitor creation for Envoy Proxy pods. Enable via `spec.envoyProxy.podMonitor.enabled: true` in the Config CR. ([#313](https://github.com/kubermatic/kubelb/pull/313))
- Fix empty IP in envoy access logs for TCP/UDP listeners ([#269](https://github.com/kubermatic/kubelb/pull/269))
- Improvements for dual-stack and IPv6 only support ([#265](https://github.com/kubermatic/kubelb/pull/265))
- KubeLB is now built using 1.25.7 ([#270](https://github.com/kubermatic/kubelb/pull/270))
- KubeLB is now built using Go 1.26.1 ([#314](https://github.com/kubermatic/kubelb/pull/314))
- Separate EDS from CDS in xDS snapshots to reduce payload size on endpoint changes ([#304](https://github.com/kubermatic/kubelb/pull/304))
- Upgrade ingress-nginx to v1.15.0 to mitigate CVE-2026-3288, for more details: https://github.com/kubernetes/kubernetes/issues/137560 ([#315](https://github.com/kubermatic/kubelb/pull/315))

### Bug or Regression

- Add explicit namespace to all namespaced scoped resources in helm charts ([#292](https://github.com/kubermatic/kubelb/pull/292))
- Fix WebSocket connections failing through KubeLB Layer 7 proxy by adding UpgradeConfigs to Envoy HttpConnectionManager ([#328](https://github.com/kubermatic/kubelb/pull/328))
- Fix envoy proxy readiness probe failures under stats scrape contention at scale ([#303](https://github.com/kubermatic/kubelb/pull/303))
- Fix lengthy reconciliation loop when creating HTTPRoutes with ClusterIP backend services ([#330](https://github.com/kubermatic/kubelb/pull/330))
- Increased TimeoutSeconds and FailureThresholds on envoy-proxy probes ([#287](https://github.com/kubermatic/kubelb/pull/287))
- Use default value for per_connection_buffer_limit_bytes instead of a lowered 32KB buffer limit. ([#345](https://github.com/kubermatic/kubelb/pull/345))

### Other (Cleanup or Flake)

- The "Global" Envoy Proxy Topology has been removed in favor of the default "Shared" ([#273](https://github.com/kubermatic/kubelb/pull/273))
- The manager now operates with sensible defaults (shared topology, 3 replicas) when the Config CR is absent. ([#318](https://github.com/kubermatic/kubelb/pull/318))

### Uncategorized

- Move ingress conversion logic to pkg/conversion ([#261](https://github.com/kubermatic/kubelb/pull/261))
- Updated kubelb-addons chart to v0.3.1 with dependency bumps: 
  
  #### ingress-nginx 4.14.1 → 4.14.3
  - [CVE-2026-1580](https://github.com/kubernetes/kubernetes/issues/136677) - auth-method nginx configuration injection
  - [CVE-2026-24512](https://github.com/kubernetes/kubernetes/issues/136678) - rules.http.paths.path nginx configuration injection
  - [CVE-2026-24513](https://github.com/kubernetes/kubernetes/issues/136679) - auth-url protection bypass
  - [CVE-2026-24514](https://github.com/kubernetes/kubernetes/issues/136680) - Admission Controller denial of service
  
  #### envoy-gateway 1.6.2 → 1.6.3
  - [CVE-2025-0913](https://nvd.nist.gov/vuln/detail/CVE-2025-0913) - Use-after-free in c-ares DNS resolver
  
  #### cert-manager v1.19.2 → v1.19.3
  - [GHSA-gx3x-vq4p-mhhv](https://github.com/cert-manager/cert-manager/security/advisories/GHSA-gx3x-vq4p-mhhv) - DoS via malformed DNS response
  
  ### References
  - [Envoy Gateway v1.6.3 Release](https://gateway.envoyproxy.io/news/releases/notes/v1.6.3/)
  - [cert-manager v1.19.3 Release](https://github.com/cert-manager/cert-manager/releases/tag/v1.19.3)
  - [ingress-nginx v1.14.3 Release](https://github.com/kubernetes/ingress-nginx/releases/tag/controller-v1.14.3) ([#257](https://github.com/kubermatic/kubelb/pull/257))

### Enterprise Edition

**Enterprise Edition includes everything from Community Edition and more. The release notes below are for changes specific to just the Enterprise Edition.**


### EE Feature

- Add imagePullSecrets support for envoy proxy pods. Secrets are auto-detected from the manager pod or can be explicitly configured via `spec.envoyProxy.imagePullSecrets` in the Config CR. (#308)
- Add`MaxEndpointsPerCluster` to Config CR to cap upstream endpoints in Envoy xDS snapshot. 
  - `--max-node-address-count` and `--node-address-label-selector` flags to CCM to filter/limit node addresses forwarded to the LB cluster. When `--max-node-address-count` is enabled, addresses are selected with topology-aware round-robin spread across zones (topology.kubernetes.io/zone). (#295)
- KubeLB creates network policies for tenant isolation. Policies can be enabled/disabled, additional policies can be added, etc. at Global/Config and Tenant level. (#316)
- Users can now configure the Envoy load balancing policy per LoadBalancer/Route, per tenant, or globally. Supported policies: RoundRobin, LeastRequest, and Random. Use the `kubelb.k8c.io/lb-policy` annotation on Services or Ingresses in tenant clusters. (#317)

### EE Bug or Regression

- Fix lengthy reconciliation loop when creating HTTPRoutes with ClusterIP backend services (#332)

### Release Artifacts

#### Community Edition

For Community Edition, the release artifacts are available on [GitHub Releases](https://github.com/kubermatic/kubelb/releases/tag/v99.0.0).

#### Enterprise Edition

<details>
<summary><b>Docker Images</b></summary>

```bash
# Login to registry
docker login quay.io -u <username> -p <password>

# kubelb manager
docker pull quay.io/kubermatic/kubelb-manager-ee:v99.0.0

# ccm
docker pull quay.io/kubermatic/kubelb-ccm-ee:v99.0.0

# connection-manager
docker pull quay.io/kubermatic/kubelb-connection-manager-ee:v99.0.0
```

</details>

<details>
<summary><b>Helm Charts</b></summary>

```bash
# kubelb-manager
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-manager-ee --version v99.0.0

# kubelb-ccm
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-ccm-ee --version v99.0.0

# kubelb-addons
helm pull oci://quay.io/kubermatic/helm-charts/kubelb-addons --version v0.3.2
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
  quay.io/kubermatic/kubelb-manager-ee:v99.0.0 | jq -r '.referrers[0].digest')
oras pull quay.io/kubermatic/kubelb-manager-ee@${SBOM_DIGEST} --output sbom/

## kubelb-ccm
SBOM_DIGEST=$(oras discover --format json --artifact-type application/spdx+json \
  quay.io/kubermatic/kubelb-ccm-ee:v99.0.0 | jq -r '.referrers[0].digest')
oras pull quay.io/kubermatic/kubelb-ccm-ee@${SBOM_DIGEST} --output sbom/

## kubelb-connection-manager
SBOM_DIGEST=$(oras discover --format json --artifact-type application/spdx+json \
  quay.io/kubermatic/kubelb-connection-manager-ee:v99.0.0 | jq -r '.referrers[0].digest')
oras pull quay.io/kubermatic/kubelb-connection-manager-ee@${SBOM_DIGEST} --output sbom/
```

**Verify SBOM attestation:**

```bash
cosign verify-attestation quay.io/kubermatic/kubelb-manager-ee:v99.0.0 \
  --type spdxjson \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify-attestation quay.io/kubermatic/kubelb-ccm-ee:v99.0.0 \
  --type spdxjson \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify-attestation quay.io/kubermatic/kubelb-connection-manager-ee:v99.0.0 \
  --type spdxjson \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

</details>

<details>
<summary><b>Verify Signatures</b></summary>

**Docker images:**

```bash
cosign verify quay.io/kubermatic/kubelb-manager-ee:v99.0.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/kubelb-ccm-ee:v99.0.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/kubelb-connection-manager-ee:v99.0.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

**Helm charts:**

```bash
cosign verify quay.io/kubermatic/helm-charts/kubelb-manager-ee:v99.0.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/helm-charts/kubelb-ccm-ee:v99.0.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb-ee/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

cosign verify quay.io/kubermatic/helm-charts/kubelb-addons:v0.3.2 \
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

