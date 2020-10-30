# Agent

Runs inside every user cluster and watches for Services and Nodes

## Configuration

Requires `$HOME/.kube/kubelb` kubernetes conf to access the load balancer cluster.

## Annotations
Use annotations in the service object to control Kubelb.

`kubelb.enable: "true"` | Enables Kubelb | 
--- | --- | 

For more information about Kubernetes annotations: [Concepts annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)

## L4

For now, only L4 load balancing is available. 

## Todo's 

