# Manager

The manager is responsible for the load balancer resources and takes care of GlobalLoadBalancer CRD 

## Kubebuilder 

Generate CRD: `make manifests`

Install CRD: `make install`

## CRD GlobalLoadBalancer

Do not set more fields than necessary. Currently, the CRD makes use of the Kubernetes types **Endpoint.Subset** and **Service.Ports** which offer more configuration. 
However, this can break the load balancing very fast and will be changed in the future.

## Client generation 

There is a bug inside the golang.org/x/tools/. This bug got fixed, but the code-generator package use an old version.

https://github.com/golang/go/issues/34027

Maybe there is a workaround, with the usage of GOPATH and no go modules.

To generate a new client for the CRD api run:

`go mod vendor && ./hack/update-codegen.sh`

## Todo's: 

* Cleanup upon deletion (probably with Finalizers)

