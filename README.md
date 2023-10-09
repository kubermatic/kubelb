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

Deploy the manager to the load balancer cluster

Install the LoadBalancers CRD: `make install`

Deploy to load balancer cluster: `make deploy-kubelb`

### CCM

Deploy the agent to every user cluster where you want to use KubeLB

Inside the deployment you need to change the ClusterName parameter to its actual name. Also make sure that a namespace
is created inside the kubelb cluster, so the agent can create the CRDs inside its cluster namespace.

The agent expects a `Secret` with a kubeconf file named `kubelb` to access the load balancer cluster.

To create such run: `kubectl --namespace kubelb create secret generic kubelb-cluster --from-file=<path to kubelb kubeconf file>`

Deploy to user cluster: `make deploy-ccm`

## Troubleshooting

If you encounter issues [file an issue][1] or talk to us on the [#KubeLB channel][12] on the [Kubermatic Slack][15].

## Contributing

Thanks for taking the time to join our community and start contributing!

Feedback and discussion are available on [the mailing list][11].

### Before you start

* Please familiarize yourself with the [Code of Conduct][4] before contributing.
* See [CONTRIBUTING.md][2] for instructions on the developer certificate of origin that we require.

### Pull requests

* We welcome pull requests. Feel free to dig through the [issues][1] and jump in.

## Changelog

See [the list of releases][3] to find out about feature changes.

[1]: https://github.com/kubermatic/KubeLB/issues
[2]: https://github.com/kubermatic/KubeLB/blob/main/CONTRIBUTING.md
[3]: https://github.com/kubermatic/KubeLB/releases
[4]: https://github.com/kubermatic/KubeLB/blob/main/CODE_OF_CONDUCT.md
[11]: https://groups.google.com/forum/#!forum/kubelb-dev
[12]: https://kubermatic.slack.com/messages/kubelb
[15]: http://slack.kubermatic.io/
