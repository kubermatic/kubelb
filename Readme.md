# KubeLB

**Work in progress**

Control-plane for multicluster loadbalancing.

## Agent

Watches for Services, Ingress and node Changes inside the user cluster and creates a CRD accordingly inside the LB cluster

## Manager

Watches for it's CRD and configures the load balancer inside the LB cluster accordingly.

## Controller

Deploys the manager for each user cluster inside the LB Cluster

## Todo's

Remove all biolerplate code and cleanup some autgenerated files

Linting -> golangci-lint

