# Manager

The manager is responsible for the load balancer resources and takes care of HTTPLoadBalancer and TCPLoadBalancer CRD 

## Requirements 

Type `LoadBalancer` service implementation

An Ingress Controller needs to be installed.

To install Traefik as an Ingress Controller with helm v3 run:

* `helm repo add traefik https://helm.traefik.io/traefik`

* `helm repo update`

* `kubectl create namespace traefik`

* `helm install traefik traefik/traefik --namespace=traefik` 

For development, you might want to run traefik in debug mode: 

`helm install --namespace=traefik  --set="additionalArguments={--log.level=DEBUG}"  traefik traefik/traefik`

## Kubebuilder 

Generate CRD: `make manifests`

Install CRD: `make install`

## Client generation 

There is a bug inside the golang.org/x/tools/. This bug got fixed, but the code-generator package uses an older version.

https://github.com/golang/go/issues/34027

Maybe there is a workaround, with the usage of GOPATH and not go modules.

To generate a new client for the CRD api run:

`go mod vendor && ./hack/update-codegen.sh`

## Todo's: 

* ValidatingAdmissionWebhook for CRD 
