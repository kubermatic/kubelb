# Manager

The manager is responsible for the loadbalancer resources and takes care of GlobalLoadBalancer CRD 

## Kubebuilder 

Generate CRD: `make manifests`

Install CRD: `make install`

## CRD GlobalLoadBalancer

Do not set more field's than necessary. Currently the CRD makes use of the Kubernetes types **Endpoint.Subset** and **Service.Ports** which offer more configuration. 
However this can break the load balancing very fast and will be changed in the future.