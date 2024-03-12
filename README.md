<p align="center">
  <img src="docs/kubelb-logo.png#gh-light-mode-only" width="700px" />
  <img src="docs/kubelb-logo-dark.png#gh-dark-mode-only" width="700px" />
</p>

# KubeLB

## Overview

KubeLB is a project to centrally manage load balancers across multicloud and on-prem.

## Architecture

Please see [docs/architecture.md](./docs/architecture.md) for an overview of the KubeLB architecture.

## Installation

We strongly recommend that you use an [official release][3] of KubeLB. The tarballs for each release contain the
version-specific sample YAML files for deploying KubeLB to your cluster.

_The code and sample YAML files in the main branch of the KubeLB repository are under active development and are not
guaranteed to be stable. Use them at your own risk!_

Make sure your current cluster configuration for kubectl points to the correct cluster.

You probably want to change the default configuration of the agent or manager.

To do so, you can edit the deployment with your parameters in the [ccm](./config/ccm/) or [kubelb](./config/kubelb/).
directory.

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
