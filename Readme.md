# KubeLB

A control-plane for multicluster loadbalancing.

## Features

- Use Kubernetes to manage loadbalancer instances for sending traffic to ingress objects in multiple clusters
- The KubeLB agent creates `kind: LoadBalancer` in LB cluster
- The KubeLB manager creates Pods/Deployments of e.g. envoy proxies for every `kind: LoadBalancer` object
- The KubeLB controller creates Namespaces on the loadbalancer cluster per user-cluster/tenant
- Separate loadbalancing instances in the looadbalancing cluster in namespaces; one namespace per user cluster/tenant
- Be able to have the loadbalancing cluster exist seperately from the seed cluster or integrated into it
- Make the actual loadbalancer technology pluggable (easy implementation if e.g. someone wants to use nginx over envoy)

## Details

- Seed cluster = LB cluster / Seed cluster != LB cluster
- KubeLB Controller creates namespaces per tenant and/or user cluster
- KubeLB manager createsLB instances for LB objects
- KubeLB agent reads ingresses/services from user clusters and creates LB objects in LB cluster


## Kubebuilder 

Generate CRD: `make manifest`

#### Todos

Remove all biolerplate code and cleanup some files