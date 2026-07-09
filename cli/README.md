<p align="center">
  <img src="docs/kubelb-logo.png#gh-light-mode-only" width="200px" />
  <img src="docs/kubelb-logo-dark.png#gh-dark-mode-only" width="200px" />
</p>

# KubeLB CLI

> [!NOTE]
> **🚧 Beta Software** - This CLI is currently in beta stage and is not yet ready for production use. Features may change as we continue development and gather feedback. Please report any issues you encounter!

The KubeLB CLI provides tools to manage KubeLB load balancers and create secure tunnels to expose local services through the KubeLB infrastructure(Requires KubeLB Enterprise Edition).

For more information, please refer to the [KubeLB CLI documentation](https://docs.kubermatic.com/kubelb/latest/cli).

## Installation

### Download a release

Starting with KubeLB v1.5.0, the CLI is released together with KubeLB from
[kubermatic/kubelb releases](https://github.com/kubermatic/kubelb/releases)
and shares the KubeLB version number. Download the `kubelb-cli` archive for
your platform; the binary inside is named `kubelb`.

```bash
VERSION=1.5.0
curl -LO https://github.com/kubermatic/kubelb/releases/download/v${VERSION}/kubelb-cli_${VERSION}_linux_amd64.tar.gz
tar -xzf kubelb-cli_${VERSION}_linux_amd64.tar.gz kubelb
sudo install kubelb /usr/local/bin/
```

Verify the download (keyless cosign — releases before v1.5.0 were published
from kubermatic/kubelb-cli and signed with a static key instead):

```bash
gh attestation verify kubelb-cli_${VERSION}_linux_amd64.tar.gz --repo kubermatic/kubelb

# or via the signed checksums
curl -LO https://github.com/kubermatic/kubelb/releases/download/v${VERSION}/checksums.txt
curl -LO https://github.com/kubermatic/kubelb/releases/download/v${VERSION}/checksums.txt.sigstore.json
cosign verify-blob --bundle checksums.txt.sigstore.json checksums.txt \
  --certificate-identity-regexp="^https://github.com/kubermatic/kubelb/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
sha256sum -c checksums.txt --ignore-missing
```

### Build from source

```bash
make build
```

This will create the binary in `./bin/kubelb`.

### Install to system

```bash
make install
```

This will install the binary to your Go bin directory.

## Configuration

The KubeLB CLI supports multiple ways to configure kubeconfig and tenant settings, with the following precedence (highest to lowest):

### Kubeconfig Configuration

1. **`--kubeconfig` flag**: Explicitly specify kubeconfig path
2. **`KUBECONFIG` environment variable**: Standard Kubernetes environment variable

### Tenant Configuration

1. **`--tenant, -t` flag**: Explicitly specify tenant name
2. **`TENANT_NAME` environment variable**: Set tenant via environment variable
3. **Error if neither provided**: Tenant is required for most commands (error message includes both flag and environment variable options)

### Examples

```bash
# Using flags
kubelb loadbalancer list --tenant mycompany --kubeconfig ./kubeconfig

# Using environment variables
export TENANT_NAME=mycompany
export KUBECONFIG=./kubeconfig
kubelb loadbalancer list

# Mixed (flags override environment variables)
export TENANT_NAME=from-env
kubelb loadbalancer list --tenant from-flag  # Uses "from-flag"

# Tenant namespace examples
kubelb loadbalancer list --tenant mycompany          # Uses namespace: tenant-mycompany
kubelb loadbalancer list --tenant tenant-mycompany  # Uses namespace: tenant-mycompany (no double prefix)
```

## Global Flags

- `--kubeconfig`: Path to kubeconfig file
- `--tenant, -t`: Tenant name (creates namespace `tenant-{name}`, or uses as-is if already prefixed)
- `--timeout`: Timeout for operations (e.g., 30s, 5m)

## CLI Reference

For more information, please refer to the [CLI Reference](docs/cli/kubelb.md).
