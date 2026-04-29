# KubeLB v1.4 Changelog

- [v1.4.0](#v140)
  - [Community Edition](#community-edition)
  - [Enterprise Edition](#enterprise-edition)
## v1.4.0

**GitHub release: [v1.4.0](https://github.com/kubermatic/kubelb/releases/tag/v1.4.0)**

### Community Edition

No notable changes.

### Enterprise Edition

**Enterprise Edition includes everything from Community Edition and more. The release notes below are for changes specific to just the Enterprise Edition.**


### EE Feature

- Add air-gap deployment support. List of all Images and charts shipped by KubeLB and used by KubeLb through addons is now published with each release as part of release artifacts. (#354)
- Add imagePullSecrets support for envoy proxy pods. Secrets are auto-detected from the manager pod or can be explicitly configured via `spec.envoyProxy.imagePullSecrets` in the Config CR. (#308)
- Add upstream TLS support for backend connections. New `spec.upstreamTLS` field on LoadBalancer CRD with two modes: `Insecure` (skip cert verification) and `Verify` (validate against CA certificate from Secret). Configurable via `kubelb.k8c.io/backend-tls-policy` and `kubelb.k8c.io/backend-tls-ca-secret` annotations on tenant Services. (#351)
- Add`MaxEndpointsPerCluster` to Config CR to cap upstream endpoints in Envoy xDS snapshot. 
  - `--max-node-address-count` and `--node-address-label-selector` flags to CCM to filter/limit node addresses forwarded to the LB cluster. When `--max-node-address-count` is enabled, addresses are selected with topology-aware round-robin spread across zones (topology.kubernetes.io/zone). (#295)
- KubeLB creates network policies for tenant isolation. Policies can be enabled/disabled, additional policies can be added, etc. at Global/Config and Tenant level. (#316)
- Promote Web Application Firewall (WAF) feature from Alpha to Beta. (#362)
- Users can now configure the Envoy load balancing policy per LoadBalancer/Route, per tenant, or globally. Supported policies: RoundRobin, LeastRequest, and Random. Use the `kubelb.k8c.io/lb-policy` annotation on Services or Ingresses in tenant clusters. (#317)

### EE Bug or Regression

- Fix lengthy reconciliation loop when creating HTTPRoutes with ClusterIP backend services (#332)

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

- [Cosign](https://github.com/sigstore/cosign) - Container signing
- [ORAS](https://oras.land) - OCI Registry As Storage

</details>

