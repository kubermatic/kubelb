# Manager

The manager is responsible for the load balancer resources and takes care of GlobalLoadBalancer CRD 

## Kubebuilder 

Generate CRD: `make manifests`

Install CRD: `make install`

## CRD GlobalLoadBalancer

## Client generation 

There is a bug inside the golang.org/x/tools/. This bug got fixed, but the code-generator package use an old version.

https://github.com/golang/go/issues/34027

Maybe there is a workaround, with the usage of GOPATH and no go modules.

To generate a new client for the CRD api run:

`go mod vendor && ./hack/update-codegen.sh`

## Todo's: 

* ValidatingAdmissionWebhook for CRD

* Cleanup upon deletion (probably with Finalizers)

