# Agent

Runs inside every user cluster and watches for Services and Nodes

## Development

Requires `$HOME/.kube/kubelb` kubernetes conf to access the load balancer cluster.

## Deployment

The agents requires the kubelb kubernetes config inside a configmap named: kubelb-cfg

`kubectl -n kubelb create configmap kubelb-cfg --from-file $HOME/.kube/kubelb`

## Annotations
Use annotations in the service object to control Kubelb.

`kubelb.enable: "true"` | Enables Kubelb | 
--- | --- | 

For more information about Kubernetes annotations: [Concepts annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)

## L4

For now, only L4 load balancing is available. 

## Todo's 

