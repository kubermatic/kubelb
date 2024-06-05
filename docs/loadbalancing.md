# Layer 4 and Layer 7 Load Balancing with KubeLB

## Layer 4

KubeLB supports Layer 4 load balancing by leveraging the Kubernetes Service abstraction. When a Service of type `LoadBalancer` is created in the tenant cluster, KubeLB CCM will create a new `LoadBalancer` resource in the LB cluster. This resource will be responsible for creating the necessary network resources to expose the Service to the internet.

The `LoadBalancer` resource created by KubeLB CCM is a Kubernetes Custom Resource Definition (CRD) that is managed by the KubeLB controller. The controller watches for changes to the `LoadBalancer` resources and performs the following actions:

- Creates a corresponding service of type loadbalancer in the LB cluster.
- Propagates the load balancing configuration to the envoy proxy to route traffic to the backend pods.

Example of a Service of type `LoadBalancer`:

```yaml
apiVersion: kubelb.k8c.io/v1alpha1
kind: LoadBalancer
metadata:
  name: loadbalancer
spec:
  endpoints:
    - addresses:
        - ip: 168.119.189.211
        - ip: 168.119.185.115
      ports:
        - name: a-port
          port: 32019
          protocol: TCP
        - name: b-port
          port: 32529
          protocol: TCP
  ports:
    - name: a-port
      port: 8080
      protocol: TCP
    - name: b-port
      port: 8081
      protocol: TCP
```

## Layer 7

While Layer 4 load balancing is sufficient for many use cases, Layer 7 load balancing provides additional features such as path-based routing, header-based routing, and SSL termination.

Since Layer 7 load balancing is not natively supported by Kubernetes, KubeLB uses the Ingress and Gateway API resources to provide Layer 7 load balancing capabilities. For l4, we could simply rely on a single resource i.e. service of type LoadBalancer. However, for l7, we need to rely on multiple resources to achieve the desired functionality. This list of resources is also going to be extended in the future as we add more features to KubeLB.

Eventually the workflow for Layer 7 load balancing will look like this:

- Resources for Application load balancing are created in the tenant cluster.
- KubeLB CCM watches for changes to the Ingress and Gateway resources in the tenant cluster.
- KubeLB CCM creates corresponding resources in the LB cluster to configure the load balancer.

For transmitting these resources in the LB cluster, we had the following options:

1. Use the original Ingress and Gateway resources.
2. KubeLB manager uses kubeconfigs to access the tenant cluster and create the resources in the LB cluster.
3. Use the same CRD `LoadBalancer` as Layer 4 and extend it to support Layer 7.
4. Create a new CRD called `Route` to cover routing for both L4 and L7 flows.

### Use the original Ingress and Gateway resources

The reason we didn't go with this option is that it would require us to give CRUD permissions to the tenants on the LB cluster for the actual resources. Which would give them access to the final transformed form of the resources that they can also use to manipulate the load balancer. This is not something we want to expose to the tenants. We will be making transformations that should be hidden from the tenants, a prime example for that would be how we deal with ReferenceGrants that is used to allow cross-namespace resource access in Gateway API; tenant resources are created in a single namespace and we'll have to perform the authorizations in-code instead of via the ReferenceGrant.

### KubeLB manager uses kubeconfigs to access the tenant cluster and create the resources in the LB cluster

The reason we didn't go with this option is that it would require us to store the kubeconfigs of the tenant clusters in the LB cluster. This is not something we want to do as it would expose the kubeconfigs to the LB cluster. We want to keep the tenant clusters isolated from the LB cluster.

Another notable point is that we can't simply run the ingress controller, let's say the NGINX ingress controller, in the LB cluster and have it watch the tenant clusters. This is because the ingress controller needs to be able to route traffic to the backend pods, which are in the tenant clusters. This is not possible without without pod-level network access to ALL the tenant clusters, in the LB cluster.

### Use the same CRD `LoadBalancer` as Layer 4 and extend it to support Layer 7

This would overcomplicate the CRD and make it harder to manage. We would have to add a lot of fields to the CRD to support all the features of Layer 7 load balancing. This would make the CRD harder to understand and use.

#### Decision

We decided to go with the fourth option and create a new CRD called `Route` to cover routing for both L4 and L7 flows. This will make it easier to manage the resources and keep the Layer 4 and Layer 7 load balancing configurations separate. So we will have two CRDs, `LoadBalancer` for provisioning actual load balancers, and then `Route` for configuring the L4/L7 routing rules.

Example of an Ingress resource:

```yaml
apiVersion: kubelb.k8c.io/v1alpha1
kind: Route
metadata:
  name: sample
spec:
  endpoints:
    - addresses:
        - ip: 168.119.189.211
        - ip: 168.119.185.115
      ports:
        - name: a-port
          port: 32019
          protocol: TCP
  source:
    kubernetes:
      ingresses:
        - apiVersion: extensions/v1beta1
          kind: Ingress
          metadata:
            name: name-virtual-host-ingress
            namespace: tenant-xr8bmnsvch
          spec:
            ingressClassName: kubelb
            rules:
              - host: foo.bar.com
                http:
                  paths:
                    - backend:
                        serviceName: service1
                        servicePort: 80
      services:
        - apiVersion: v1
          kind: Service
          metadata:
            name: service1
            namespace: default
          spec:
            ports:
              - port: 80
                targetPort: 8080
```
