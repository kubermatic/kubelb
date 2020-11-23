# Agent

Runs inside every user cluster and watches for Services and Nodes

## Development

Requires `$HOME/.kube/kubelb` kubernetes conf to access the load balancer cluster.

## Deployment

The agents requires the kubelb kubernetes config inside a configmap named: kubelb-cfg

`kubectl -n kubelb create configmap kubelb-cfg --from-file $HOME/.kube/kubelb`

## Annotations
Use annotations in the service object to control Kubelb.

`kubernetes.io/ingress.class: "kubelb"`  Enables kubelb for ingress 

`kubernetes.io/service.class: "kubelb"`  Enables kubelb for service 

This is only needed if `serve-default-lb` is set to false. 

For more information about Kubernetes annotations: [Concepts annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)


## Todo's 
 
