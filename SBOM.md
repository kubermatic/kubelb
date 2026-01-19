# Software Bill of Materials (SBOM)

KubeLB generates Software Bill of Materials (SBOMs) for all release artifacts, enabling supply chain transparency and vulnerability management.

## SBOM Format

We generate SBOMs in the following format:

| Format | Standard | Description |
|--------|----------|-------------|
| **SPDX** | ISO/IEC 5962:2021 | Primary format for all artifacts |

## Where to Find SBOMs

### 1. GitHub Release Assets

Each release includes SBOM files as downloadable assets:

```
# Binary SBOMs
kubelb_<version>_<os>_<arch>.sbom.spdx.json
ccm_<version>_<os>_<arch>.sbom.spdx.json

# Archive SBOMs
kubelb_<version>_<os>_<arch>.tar.gz.sbom.spdx.json

# Signed checksums (covers all release artifacts including SBOMs)
checksums.txt
checksums.txt.sigstore.json
```

Download example:

```bash
VERSION=1.3.0

# Download binary SBOM
curl -LO https://github.com/kubermatic/kubelb/releases/download/v${VERSION}/kubelb_${VERSION}_linux_amd64.sbom.spdx.json

# Download signed checksums for verification
curl -LO https://github.com/kubermatic/kubelb/releases/download/v${VERSION}/checksums.txt
curl -LO https://github.com/kubermatic/kubelb/releases/download/v${VERSION}/checksums.txt.sigstore.json
```

Download from: https://github.com/kubermatic/kubelb/releases

### 2. Docker Image Attestations

SBOMs are attached to Docker images as OCI artifacts:

```bash
# Discover attached SBOMs
oras discover -o tree quay.io/kubermatic/kubelb-manager:v1.3.0

# Pull SBOM by digest
SBOM_DIGEST=$(oras discover --format json --artifact-type application/spdx+json \
  quay.io/kubermatic/kubelb-manager:v1.3.0 | jq -r '.referrers[0].digest')
oras pull quay.io/kubermatic/kubelb-manager@${SBOM_DIGEST} --output sbom/

# Or use cosign to extract the SBOM attestation
cosign download attestation quay.io/kubermatic/kubelb-manager:v1.3.0 \
  | jq -r '.payload' | base64 -d | jq '.predicate' > sbom.spdx.json
```

Available images:

- `quay.io/kubermatic/kubelb-manager`
- `quay.io/kubermatic/kubelb-ccm`

### 3. Helm Charts

Helm charts are signed with cosign for supply chain integrity (no separate SBOM):

```bash
# Verify chart signature
cosign verify quay.io/kubermatic/helm-charts/kubelb-manager:v1.3.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

Available charts:

- `quay.io/kubermatic/helm-charts/kubelb-manager`
- `quay.io/kubermatic/helm-charts/kubelb-ccm`
- `quay.io/kubermatic/helm-charts/kubelb-addons`

## Verifying SBOMs

### Binary SBOM Verification

Binary SBOMs are verified via signed checksums:

```bash
VERSION=1.3.0

# Download checksums and signature bundle
curl -LO https://github.com/kubermatic/kubelb/releases/download/v${VERSION}/checksums.txt
curl -LO https://github.com/kubermatic/kubelb/releases/download/v${VERSION}/checksums.txt.sigstore.json

# Verify checksums signature (keyless)
cosign verify-blob --bundle checksums.txt.sigstore.json checksums.txt \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

# Verify SBOM integrity against checksums
sha256sum -c checksums.txt --ignore-missing
```

### Docker Image SBOM Attestations

SBOMs attached to images are cryptographically signed:

```bash
# Verify the SBOM attestation (keyless)
cosign verify-attestation \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  --type spdxjson \
  quay.io/kubermatic/kubelb-manager:v1.3.0
```

### Validate SBOM Format

Use SPDX tools to validate the SBOM format:

```bash
# Install SPDX tools
pip install spdx-tools

# Validate SBOM
pyspdxtools -i sbom.spdx.json
```

## Analyzing SBOMs

### Trivy

```bash
# Scan SBOM for vulnerabilities
trivy sbom sbom.spdx.json

# Scan with specific severity threshold
trivy sbom sbom.spdx.json --severity HIGH,CRITICAL
```

### Grype

```bash
# Install Grype
brew install grype  # macOS
# or curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin

# Scan SBOM for vulnerabilities
grype sbom:sbom.spdx.json

# Output with severity filter
grype sbom:sbom.spdx.json --fail-on high
```

## SBOM Contents

Our SBOMs include:

- **Package Information**: Name, version, supplier, download location
- **License Information**: SPDX license identifiers
- **Relationship Information**: Dependencies, build tools
- **External References**: Package URLs (purl), CPE identifiers

### Example SBOM Entry

```json
{
  "SPDXID": "SPDXRef-Package-go-module-k8s.io/client-go",
  "name": "k8s.io/client-go",
  "versionInfo": "v0.31.0",
  "supplier": "Organization: Kubernetes",
  "downloadLocation": "https://proxy.golang.org/k8s.io/client-go/@v/v0.31.0.zip",
  "filesAnalyzed": false,
  "licenseConcluded": "Apache-2.0",
  "licenseDeclared": "Apache-2.0",
  "externalRefs": [
    {
      "referenceCategory": "PACKAGE-MANAGER",
      "referenceType": "purl",
      "referenceLocator": "pkg:golang/k8s.io/client-go@v0.31.0"
    }
  ]
}
```

## Compliance

Our SBOM generation follows:

- **NTIA Minimum Elements**: All required fields populated
- **Executive Order 14028**: SBOM requirements for software supply chain
- **SLSA**: Supply-chain Levels for Software Artifacts

## Resources

- [SPDX Specification](https://spdx.github.io/spdx-spec/)
- [NTIA SBOM Minimum Elements](https://www.ntia.gov/page/software-bill-materials)
- [Trivy](https://github.com/aquasecurity/trivy)
- [Grype Documentation](https://github.com/anchore/grype)
- [Cosign](https://github.com/sigstore/cosign)
