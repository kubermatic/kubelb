# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in KubeLB, please report it privately:

**Email**: <security@kubermatic.com>

**Please include:**

- Description and potential impact
- Steps to reproduce
- Affected versions
- Suggested remediation (if any)

**Response timeline:**

- Acknowledgment within 48 hours
- Initial assessment within 7 days
- Regular updates on remediation progress

We follow coordinated disclosure practices. Please do not disclose vulnerabilities publicly until we have released a fix and coordinated disclosure timing.

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest stable | Yes |
| Previous minor (n-1) | 3 months after new release |
| Older versions | No |

## Security Measures

### Supply Chain Security

- **Artifact Signing**: All container images, binaries and Helm charts are signed with [Sigstore Cosign](https://github.com/sigstore/cosign) using keyless signing
- **SBOMs**: Software Bill of Materials in SPDX format for all binaries and container images (see [SBOM.md](SBOM.md))
- **Dependency Management**: Dependabot monitors for vulnerabilities with automated updates
- **Immutable Releases**: GitHub releases are immutable and the assets cannot be modified after release
- **Vulnerability Scanning**: All PRs are scanned for vulnerabilities before merge. Releases are scanned for vulnerabilities at release time.

### Automated Scanning

- All PRs scanned for vulnerabilities before merge
- Container images scanned with Trivy at release time
- HIGH/CRITICAL vulnerabilities block releases

### SBOM

View the [SBOM.md](SBOM.md) file for more details.

### Verification

Verify artifact signatures before deployment:

```bash
# Verify image
cosign verify quay.io/kubermatic/kubelb-manager:v1.3.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com

# Verify Helm chart
cosign verify quay.io/kubermatic/helm-charts/kubelb-manager:v1.3.0 \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

## Embargo Policy

Security vulnerabilities are handled under embargo until:

- A fix is available and tested
- Affected users have been notified (if applicable)
- A coordinated disclosure date is agreed upon

Embargo violations may result in exclusion from future security communications.
