# KubeLB

KubeLB is a Kubernetes native tool, responsible for centrally managing load balancers for Kubernetes clusters across multi-cloud and on-premise environments.

## Architecture

KubeLB comprises two main components:

### CCM

The `KubeLB CCM` is deployed in the consumer clusters that require load balancer services. Its main responsibility is to propagate the load balancer configurations to the manager.

It watches for changes in Kubernetes services, and nodes, and then generates the load balancer configuration for the manager. It then sends the configuration to the manager in the form of the `LoadBalancer` CRD.

### Manager

The `KubeLB manager` is responsible for deploying and configuring the actual load balancers. The manager registers the consumer clusters as tenants, and then it receives the load balancer configurations from the CCM(s) in the form of the `LoadBalancer` CRD. It then deploys the load balancer and configures it according to the configuration.

At its core, the KubeLB manager hosts the envoy xDS server and implements the [envoy-control-plane][1] APIs to configure the xDS services. Based on the envoy proxy deployment topology, it then installs the [envoy proxy][2] and configures it to use the xDS services to load balance the traffic.

### Envoy Proxy Deployment Topology

KubeLB manager supports three different deployment topologies for envoy proxy:

#### Dedicated (Deprecated on 3rd August 2024 - release v1.1.0)

In this topology, the envoy proxy is deployed per load balancer service.

#### Shared

In this topology, a single envoy proxy is deployed per tenant cluster. All the load balancer services in the tenant cluster are configured to use this envoy proxy. This is the default topology.

#### Global

In this topology, a single envoy proxy is deployed per KubeLB manager. All the load balancer services in all the tenant clusters are configured to use this envoy proxy.

## Requirements

### Consumer cluster

* Registered as a tenant in the KubeLB manager cluster.
* KubeLB manager cluster API access.

### Load balancer cluster

* Service type "LoadBalancer" implementation. This can be a cloud solution or a self-managed implementation like [MetalLB][3].
* Network access to the consumer cluster nodes with node port range (default: 30000-32767). This is required for the envoy proxy to be able to connect to the consumer cluster nodes.

[1]: https://github.com/envoyproxy/go-control-plane
[2]: https://github.com/envoyproxy/envoy
[3]: https://metallb.universe.tf/
