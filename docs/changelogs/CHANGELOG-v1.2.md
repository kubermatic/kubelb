## Kubermatic KubeLB v1.2

- [v1.2.0](#v120)
  - [Community Edition](#community-edition)
  - [Enterprise Edition](#enterprise-edition)
- [v1.2.1](#v121)
- [v1.2.2](#v122)

## v1.2.2

**GitHub release: [v1.2.2](https://github.com/kubermatic/kubelb/releases/tag/v1.2.2)**

### Security

Updated kubelb-addons chart to v0.2.1 with dependency bumps and security fixes([#259](https://github.com/kubermatic/kubelb/pull/259)):

#### ingress-nginx 4.13.0 → 4.13.3

- [CVE-2026-1580](https://github.com/kubernetes/kubernetes/issues/136677) - auth-method nginx configuration injection
- [CVE-2026-24512](https://github.com/kubernetes/kubernetes/issues/136678) - rules.http.paths.path nginx configuration injection
- [CVE-2026-24513](https://github.com/kubernetes/kubernetes/issues/136679) - auth-url protection bypass
- [CVE-2026-24514](https://github.com/kubernetes/kubernetes/issues/136680) - Admission Controller denial of service

Reference: [[Security Advisory] Multiple issues in ingress-nginx](https://groups.google.com/a/kubernetes.io/g/dev/c/9RYJrB8e8ts/m/SCatUN2AAQAJ)

#### envoy-gateway 1.5.4 → 1.5.8

- [CVE-2025-64527](https://github.com/envoyproxy/envoy/releases/tag/v1.35.7), [CVE-2025-66220](https://github.com/envoyproxy/envoy/releases/tag/v1.35.7), [CVE-2025-64763](https://github.com/envoyproxy/envoy/releases/tag/v1.35.7) - EnvoyProxy vulnerabilities fixed in v1.35.7
- [CVE-2026-22771](https://gateway.envoyproxy.io/news/releases/notes/v1.5.7/) - Arbitrary code execution through EnvoyExtensionPolicy Lua scripts

#### cert-manager v1.18.2 → v1.18.5

- [CVE-2025-61727](https://github.com/cert-manager/cert-manager/releases/tag/v1.18.4), [CVE-2025-61729](https://github.com/cert-manager/cert-manager/releases/tag/v1.18.4) - Go runtime vulnerabilities
- [GHSA-gx3x-vq4p-mhhv](https://github.com/cert-manager/cert-manager/security/advisories/GHSA-gx3x-vq4p-mhhv) - DoS via malformed DNS response
- 9 additional Go CVEs fixed in v1.18.3

#### References

- [Envoy Gateway v1.5.8](https://gateway.envoyproxy.io/news/releases/notes/v1.5.8/)
- [cert-manager v1.18.5](https://github.com/cert-manager/cert-manager/releases/tag/v1.18.5)
- [ingress-nginx v1.13.3](https://github.com/kubernetes/ingress-nginx/releases/tag/controller-v1.13.3)

## v1.2.1

**GitHub release: [v1.2.1](https://github.com/kubermatic/kubelb/releases/tag/v1.2.1)**

### Features

- HTTP2 and TCP keep-alive have been configured for Envoy Proxy. This circumvents an issue where a drop in connection from the envoy proxy -> envoy XDS left the envoy proxy in a dead/blocked state. ([#164](https://github.com/kubermatic/kubelb/pull/164))
- Configure health checks for Envoy Proxy upstream endpoints. ([#153](https://github.com/kubermatic/kubelb/pull/153))
  - Add startup-, liveness, and readiness probes to tenant envoy-proxy
  - Enable Prometheus metrics for tenant envoy-proxy
- Upgrade to Envoy Gateway v1.5.4. ([#155](https://github.com/kubermatic/kubelb/pull/155))
- KubeLB addons v0.2.0 has been released. ([#155](https://github.com/kubermatic/kubelb/pull/155))
- Log level for CCM and KubeLB can now be configured using helm charts. ([#152](https://github.com/kubermatic/kubelb/pull/152))

### Bug or Regression

- Skip nodes that are not ready from upstream address pool. ([#166](https://github.com/kubermatic/kubelb/pull/166))
- Fix a bug where resource names, specially for services, were not being truncated properly to avoid 63 character limit from Kubernetes. ([#159](https://github.com/kubermatic/kubelb/pull/159))
- KubeLB will retain annotations on generated/propagated resources like service, Ingress, Gateway API objects if they were added either manually or by external controllers. ([#170](https://github.com/kubermatic/kubelb/pull/170))
- [CE Only] Remove namespace from parentRefs for Gateway API resources. ([#157](https://github.com/kubermatic/kubelb/pull/157))
- [EE Only] Fix a bug where the gateway would not be created if the namespace was not found.

#### Other (Cleanup, Flake, or Chore)

- Annotate KubeLB created envoy-proxies with well-known labels. ([#173](https://github.com/kubermatic/kubelb/pull/173))

**Full Changelog**: <https://github.com/kubermatic/kubelb/compare/v1.2.0...v1.2.1>

## v1.2.0

**GitHub release: [v1.2.0](https://github.com/kubermatic/kubelb/releases/tag/v1.2.0)**

### Highlights

#### Community Edition(CE)

- Support for Load Balancer Hostname has been introduced. This allows users to specify a hostname for the load balancer.
- Default Annotations can now be configured for services, Ingress, and Gateway API resources in the management cluster.
- KubeLB Addons chart has been introduced to simplify the installation of the required components for the management cluster.
  - Tools such as ingress-nginx, external-dns, cert-manager, etc. can be installed through a single KubeLB management chart through this change.
  - KubeLB Addons chart will ship versions of components that we are actively testing and supporting.
- TenantState API has been introduced to share tenant status with the KubeLB consumers i.e. through CCM or CLI. This simplifies sharing details such as load balancer limit, allowed domains, wildcard domain, etc. with the consumers.
- KubeLB CCM can now install Gateway API CRDs by itself. Hence, removing the need to install them manually.
- KubeLB now maintains the required RBAC attached to the kubeconfig for KKP integration. `kkpintegration.rbac: true` can be used to manage the RBAC using KubeLB helm chart.

#### Enterprise Edition(EE)

- Tunneling support has been introduced in the Management Cluster. The server side and control plane components for tunneling are shipped with Enterprise Edition of KubeLB.
- AI and MCP Gateway Integration has been introduced. As running your AI, MCP, and Agent2Agent toolings alongisde your data plane is a common use case, we are now leveraging [kgateway](https://kgateway.dev/) to solidify the integration with AI, MCP, and Agent2Agent toolings.

### Community Edition

#### API Changes

- Enterprise Edition APIs for KubeLB are now available at k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1 ([#101](https://github.com/kubermatic/kubelb/pull/101))

#### Features

- Support for adding default annotations to the load balancing resources ([#78](https://github.com/kubermatic/kubelb/pull/78))
- KubeLB now maintains the required RBAC attached to the kubeconfig for KKP integration. `kkpintegration.rbac: true` can be used to manage the RBAC using KubeLB helm chart ([#79](https://github.com/kubermatic/kubelb/pull/79))
- Envoy: no_traffic_interval for upstream endpoints health check has been reduced to 5s from the default of 60s. Envoy will start sending health checks to a new cluster after 5s now ([#106](https://github.com/kubermatic/kubelb/pull/106))
- KubeLB CCM will now automatically install Kubernetes Gateway API CRDs using the following flags:
  - --install-gateway-api-crds: That installs and manages the Gateway API CRDs using gateway crd controller.
  - --gateway-api-crds-channel: That specifies the channel for Gateway API CRDs, with possible values of 'standard' or 'experimental'. ([#110](https://github.com/kubermatic/kubelb/pull/110))
- Improve validations for cluster-name in CCM ([#111](https://github.com/kubermatic/kubelb/pull/111))
- Gracefully handle nodes that don't have an IP address assigned while computing Addresses ([#111](https://github.com/kubermatic/kubelb/pull/111))
- LoadBalancer resources can now be directly assigned a hostname/URL ([#113](https://github.com/kubermatic/kubelb/pull/113))
- TenantState API has been introduced to share tenant status with the KubeLB consumers i.e. through CCM or CLI ([#117](https://github.com/kubermatic/kubelb/pull/117))
- Dedicated addons chart has been introduced for KubeLB at `oci://quay.io/kubermatic/helm-charts/kubelb-addons`. ([#122](https://github.com/kubermatic/kubelb/pull/122))
- KubeLB is now built using Go 1.25 ([#126](https://github.com/kubermatic/kubelb/pull/126))
- Update kube-rbac-proxy to v0.19.1 ([#128](https://github.com/kubermatic/kubelb/pull/128))
- Add metallb to kubelb-addons ([#130](https://github.com/kubermatic/kubelb/pull/130))

#### Design

- Restructure repository and make Enterprise Edition APIs available at k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1 ([#101](https://github.com/kubermatic/kubelb/pull/101))

#### Bug or Regression

- Fix annotation handling for services ([#82](https://github.com/kubermatic/kubelb/pull/82))
- Don't modify IngressClassName if it's not set in the configuration ([#88](https://github.com/kubermatic/kubelb/pull/88))
- Fix an issue with KubeLB not respecting the already allocated NodePort in the management cluster for load balancers with large amount of open Nodeports ([#91](https://github.com/kubermatic/kubelb/pull/91))
- Before removing RBAC for tenant, ensure that all routes, load balancers, and syncsecrets are cleaned up ([#92](https://github.com/kubermatic/kubelb/pull/92))
- Update health checks for envoy upstream endpoint:
  - UDP health checking has been removed due to limited supported from Envoy
  - TCP health checking has been updated to perform a connect-only health check ([#103](https://github.com/kubermatic/kubelb/pull/103))
- Use arbitrary ports as target port for load balancer services ([#119](https://github.com/kubermatic/kubelb/pull/119))

#### Other (Cleanup, Flake, or Chore)

- Upgrade to Go 1.24.1 ([#87](https://github.com/kubermatic/kubelb/pull/87))
- Upgrade to EnvoyProxy v1.33.1 ([#87](https://github.com/kubermatic/kubelb/pull/87))
- Sort IPs in `addresses` Endpoint to reduce updates ([#93](https://github.com/kubermatic/kubelb/pull/93))
- KubeLB is now built using Go 1.24.6 ([#118](https://github.com/kubermatic/kubelb/pull/118))
- Add additional columns for TenantState and Tunnel CRDs ([#124](https://github.com/kubermatic/kubelb/pull/124))

**Full Changelog**: <https://github.com/kubermatic/kubelb/compare/v1.1.0...v1.2.0>

### Enterprise Edition

**Enterprise Edition includes everything from Community Edition and more. The release notes below are for changes specific to just the Enterprise Edition.**

#### EE Features

- Default annotations support for Alpha/Beta Gateway API resources like TLSRoute, TCPRoute, and UDPRoute.
- More fine-grained load balancer hostname support.
- Tunneling support has been introduced in the Management Cluster. With the newly introduced KubeLB CLI, users can now expose workloads/applications running in their local workstations or VMs in closed networks to the outside world. Since all the traffic is routed through the KubeLB management cluster, security, observability, and other features are available and applied by default based on your configuration.
- AI and MCP Gateway Integration has been introduced. As running your AI, MCP, and Agent2Agent toolings alongisde your data plane is a common use case, we are now leveraging [kgateway](https://kgateway.dev/) to solidify the integration with AI, MCP, and Agent2Agent toolings.
