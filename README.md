<p align="center">
  <img src="docs/kubelb-logo.png#gh-light-mode-only" width="700px" />
  <img src="docs/kubelb-logo-dark.png#gh-dark-mode-only" width="700px" />
</p>

# KubeLB

## Overview

KubeLB is a project by Kubermatic, it is a Kubernetes native tool, responsible for centrally managing load balancers for Kubernetes clusters across multi-cloud and on-premise environments.

### Motivation and Background

Kubernetes does not offer any implementation for load balancers and in turn relies on the in-tree or out-of-tree cloud provider implementations to take care of provisioning and managing load balancers. This means that if you are not running on a supported cloud provider, your services of type `LoadBalancer` will never be allotted a load balancer IP address. This is an obstacle for bare-metal Kubernetes environments.

There are solutions available like [MetalLB][8], [Cilium][9], etc. that solve this issue. However, these solutions are focused on a single cluster where you have to deploy the application in the same cluster where you want the load balancers. This is not ideal for multi-cluster environments since you have to configure load balancing for each cluster separately, which makes IP address management not trivial.

KubeLB solves this problem by providing a centralized load balancer management solution for Kubernetes clusters across multi-cloud and on-premise environments.

## Architecture

Please see [docs/architecture.md](./docs/architecture.md) for an overview of the KubeLB architecture.

## Installation

### Manager

Please refer to the [KubeLB Manager README](./charts/kubelb-manager/README.md) for installation instructions.

### CCM

Please refer to the [KubeLB CCM README](./charts/kubelb-ccm/README.md) for installation instructions.

## Troubleshooting

If you encounter issues [file an issue][1] or talk to us on the [#kubermatic channel][6] on the [Kubermatic Slack][7].

## Contributing

Thanks for taking the time to join our community and start contributing!

Feedback and discussion are available on [the mailing list][5].

### Before you start

- Please familiarize yourself with the [Code of Conduct][4] before contributing.
- See [CONTRIBUTING.md][2] for instructions on the developer certificate of origin that we require.

### Pull requests

- We welcome pull requests. Feel free to dig through the [issues][1] and jump in.

## Changelog

See [the list of releases][3] to find out about feature changes.

[1]: https://github.com/kubermatic/kubelb/issues
[2]: https://github.com/kubermatic/kubelb/blob/main/CONTRIBUTING.md
[3]: https://github.com/kubermatic/kubelb/releases
[4]: https://github.com/kubermatic/kubelb/blob/main/CODE_OF_CONDUCT.md
[5]: https://groups.google.com/forum/#!forum/kubermatic-dev
[6]: https://kubermatic.slack.com/messages/kubermatic
[7]: http://slack.kubermatic.io/
[8]: https://metallb.universe.tf
[9]: https://cilium.io/use-cases/load-balancer
