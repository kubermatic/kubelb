# E2E Tests

## Local (Kind)

```bash
# Full setup + deploy + test
make e2e-kind

# Or step by step:
make e2e-setup-kind    # Create kind clusters
make e2e-deploy        # Build and deploy KubeLB
make e2e              # Run tests
make e2e-cleanup-kind  # Cleanup
```

### macOS Docker Networking

On macOS, Docker containers run inside a Linux VM, so container IPs (including MetalLB LoadBalancer IPs) are not directly reachable from the host. This breaks e2e tests that curl LoadBalancer endpoints.

The setup script automatically installs and configures [docker-mac-net-connect](https://github.com/chipmk/docker-mac-net-connect) to create a tunnel between macOS and the Docker VM, making container IPs routable.

**Requirements:**

- Docker Desktop (not Colima) - run `docker context use desktop-linux` if needed
- Homebrew for installing docker-mac-net-connect
- sudo access for creating network routes

**Manual setup (if automatic setup fails):**

```bash
brew install chipmk/tap/docker-mac-net-connect
sudo docker-mac-net-connect
```

**Known issues:**

- Docker Desktop 4.52+ requires `DOCKER_API_VERSION=1.44` workaround ([issue #62](https://github.com/chipmk/docker-mac-net-connect/issues/62))

For more details, see: <https://waddles.org/2024/06/04/kind-with-metallb-on-macos/>

## Cloud Clusters

Requires 3 clusters with kubeconfigs at `$KUBECONFIGS_DIR/{kubelb,tenant1,tenant2}.kubeconfig`.

```bash
export KUBECONFIGS_DIR=/path/to/kubeconfigs
export USE_KIND=false
export IMAGE_REGISTRY=gcr.io/myproject  # Images pushed here

make e2e-deploy
make e2e
```

### Cloud Environment Variables

| Variable | Description |
|----------|-------------|
| `USE_KIND=false` | Disable kind-specific behavior |
| `IMAGE_REGISTRY` | Registry to push images (required) |
| `METALLB_IP_RANGE` | MetalLB IP pool; if empty, skips MetalLB (uses cloud LB) |
| `SKIP_BUILD=true` | Skip building, use pre-built images |
| `SKIP_IMAGE_LOAD=true` | Skip kind image loading |

## Running Specific Tests

Use `make e2e-select` with label selectors:

```bash
make e2e-select select=layer=layer4      # All L4 tests
make e2e-select select=layer=layer7      # All L7 tests
make e2e-select select=resource=service  # Service tests only
make e2e-select select=resource=ingress  # Ingress tests only
make e2e-select select=test=basic        # Basic tests only
```

Available labels (defined in each `chainsaw-test.yaml`):

- `layer` - `layer4`, `layer7`
- `resource` - `service`, `ingress`, `gateway`, `syncsecret`
- `test` - test name (e.g., `basic`, `multi-port`)
