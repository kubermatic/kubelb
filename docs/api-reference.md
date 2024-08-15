# API Reference

## Packages

- [kubelb.k8c.io/v1alpha1](#kubelbk8ciov1alpha1)

## kubelb.k8c.io/v1alpha1

Package v1alpha1 contains API Schema definitions for the kubelb.k8c.io v1alpha1 API group

### Resource Types

- [Addresses](#addresses)
- [AddressesList](#addresseslist)
- [Config](#config)
- [ConfigList](#configlist)
- [LoadBalancer](#loadbalancer)
- [LoadBalancerList](#loadbalancerlist)
- [Route](#route)
- [RouteList](#routelist)
- [SyncSecret](#syncsecret)
- [SyncSecretList](#syncsecretlist)
- [Tenant](#tenant)
- [TenantList](#tenantlist)

#### Addresses

Addresses is the Schema for the addresses API

_Appears in:_

- [AddressesList](#addresseslist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `Addresses` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[AddressesSpec](#addressesspec)_ |  |  |  |
| `status` _[AddressesStatus](#addressesstatus)_ |  |  |  |

#### AddressesList

AddressesList contains a list of Addresses

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `AddressesList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Addresses](#addresses) array_ |  |  |  |

#### AddressesSpec

AddressesSpec defines the desired state of Addresses

_Appears in:_

- [Addresses](#addresses)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `addresses` _[EndpointAddress](#endpointaddress) array_ | Addresses contains a list of addresses. |  | MinItems: 1 <br /> |

#### AddressesStatus

AddressesStatus defines the observed state of Addresses

_Appears in:_

- [Addresses](#addresses)

#### AnnotationSettings

_Appears in:_

- [ConfigSpec](#configspec)
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `propagatedAnnotations` _map[string]string_ | PropagatedAnnotations defines the list of annotations(key-value pairs) that will be propagated to the LoadBalancer service. Keep the `value` field empty in the key-value pair to allow any value.<br />This will have a higher precedence than the annotations specified at the Config level. |  |  |
| `propagateAllAnnotations` _boolean_ | PropagateAllAnnotations defines whether all annotations will be propagated to the LoadBalancer service. If set to true, PropagatedAnnotations will be ignored.<br />This will have a higher precedence than the value specified at the Config level. |  |  |

#### Config

Config is the object that represents the Config for the KubeLB management controller.

_Appears in:_

- [ConfigList](#configlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `Config` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ConfigSpec](#configspec)_ |  |  |  |

#### ConfigList

ConfigList contains a list of Config

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `ConfigList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Config](#config) array_ |  |  |  |

#### ConfigSpec

ConfigSpec defines the desired state of the Config

_Appears in:_

- [Config](#config)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `propagatedAnnotations` _map[string]string_ | PropagatedAnnotations defines the list of annotations(key-value pairs) that will be propagated to the LoadBalancer service. Keep the `value` field empty in the key-value pair to allow any value.<br />This will have a higher precedence than the annotations specified at the Config level. |  |  |
| `propagateAllAnnotations` _boolean_ | PropagateAllAnnotations defines whether all annotations will be propagated to the LoadBalancer service. If set to true, PropagatedAnnotations will be ignored.<br />This will have a higher precedence than the value specified at the Config level. |  |  |
| `envoyProxy` _[EnvoyProxy](#envoyproxy)_ | EnvoyProxy defines the desired state of the Envoy Proxy |  |  |
| `loadBalancer` _[LoadBalancerSettings](#loadbalancersettings)_ |  |  |  |
| `ingress` _[IngressSettings](#ingresssettings)_ |  |  |  |
| `gatewayAPI` _[GatewayAPISettings](#gatewayapisettings)_ |  |  |  |

#### EndpointAddress

EndpointAddress is a tuple that describes single IP address.

_Appears in:_

- [AddressesSpec](#addressesspec)
- [LoadBalancerEndpoints](#loadbalancerendpoints)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ip` _string_ | The IP of this endpoint.<br />May not be loopback (127.0.0.0/8), link-local (169.254.0.0/16),<br />or link-local multicast ((224.0.0.0/24). |  | MinLength: 7 <br /> |
| `hostname` _string_ | The Hostname of this endpoint |  |  |

#### EndpointPort

EndpointPort is a tuple that describes a single port.

_Appears in:_

- [LoadBalancerEndpoints](#loadbalancerendpoints)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The name of this port.  This must match the 'name' field in the<br />corresponding ServicePort.<br />Must be a DNS_LABEL.<br />Optional only if one port is defined. |  |  |
| `port` _integer_ | The port number of the endpoint. |  |  |
| `protocol` _[Protocol](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#protocol-v1-core)_ | The IP protocol for this port. Defaults to "TCP". |  | Enum: [TCP UDP] <br /> |

#### EnvoyProxy

EnvoyProxy defines the desired state of the EnvoyProxy

_Appears in:_

- [ConfigSpec](#configspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `topology` _[EnvoyProxyTopology](#envoyproxytopology)_ | Topology defines the deployment topology for Envoy Proxy. Valid values are: shared and global.<br />DEPRECATION NOTICE: The value "dedicated" is deprecated and will be removed in a future release. Dedicated topology will now default to shared topology. | shared | Enum: [shared dedicated global] <br /> |
| `useDaemonset` _boolean_ | UseDaemonset defines whether Envoy Proxy will run as daemonset. By default, Envoy Proxy will run as deployment.<br />If set to true, Replicas will be ignored. |  |  |
| `replicas` _integer_ | Replicas defines the number of replicas for Envoy Proxy. This field is ignored if UseDaemonset is set to true. | 3 | Minimum: 1 <br /> |
| `singlePodPerNode` _boolean_ | SinglePodPerNode defines whether Envoy Proxy pods will be spread across nodes. This ensures that multiple replicas are not running on the same node. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector is used to select nodes to run Envoy Proxy. If specified, the node must have all the indicated labels. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#toleration-v1-core) array_ | Tolerations is used to schedule Envoy Proxy pods on nodes with matching taints. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#resourcerequirements-v1-core)_ | Resources defines the resource requirements for Envoy Proxy. |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#affinity-v1-core)_ | Affinity is used to schedule Envoy Proxy pods on nodes with matching affinity. |  |  |

#### EnvoyProxyTopology

_Underlying type:_ _string_

_Appears in:_

- [EnvoyProxy](#envoyproxy)

| Field | Description |
| --- | --- |
| `shared` |  |
| `dedicated` |  |
| `global` |  |

#### GatewayAPISettings

GatewayAPISettings defines the settings for the gateway API.

_Appears in:_

- [ConfigSpec](#configspec)
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `class` _string_ | Class is the class of the gateway API to use. This can be used to specify a specific gateway API implementation.<br />This has higher precedence than the value specified in the Config. |  |  |
| `disable` _boolean_ | Disable is a flag that can be used to disable Gateway API for a tenant. |  |  |

#### IngressSettings

IngressSettings defines the settings for the ingress.

_Appears in:_

- [ConfigSpec](#configspec)
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `class` _string_ | Class is the class of the ingress to use.<br />This has higher precedence than the value specified in the Config. |  |  |
| `disable` _boolean_ | Disable is a flag that can be used to disable Ingress for a tenant. |  |  |

#### KubernetesSource

_Appears in:_

- [RouteSource](#routesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resource` _[Unstructured](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#unstructured-unstructured-v1)_ |  |  | EmbeddedResource: \{\} <br /> |
| `services` _[UpstreamService](#upstreamservice) array_ | Services contains the list of services that are used as the source for the Route. |  |  |

#### LoadBalancer

LoadBalancer is the Schema for the loadbalancers API

_Appears in:_

- [LoadBalancerList](#loadbalancerlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `LoadBalancer` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LoadBalancerSpec](#loadbalancerspec)_ |  |  |  |
| `status` _[LoadBalancerStatus](#loadbalancerstatus)_ |  |  |  |

#### LoadBalancerEndpoints

LoadBalancerEndpoints is a group of addresses with a common set of ports. The
expanded set of endpoints is the Cartesian product of Addresses x Ports.
For example, given:

 {
   Addresses: [{"ip": "10.10.1.1"}, {"ip": "10.10.2.2"}],
   Ports:     [{"name": "a", "port": 8675}, {"name": "b", "port": 309}]
 }

The resulting set of endpoints can be viewed as:

 a: [ 10.10.1.1:8675, 10.10.2.2:8675 ],
 b: [ 10.10.1.1:309, 10.10.2.2:309 ]

_Appears in:_

- [LoadBalancerSpec](#loadbalancerspec)
- [RouteSpec](#routespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the endpoints. |  |  |
| `addresses` _[EndpointAddress](#endpointaddress) array_ | IP addresses which offer the related ports that are marked as ready. These endpoints<br />should be considered safe for load balancers and clients to utilize. |  | MinItems: 1 <br /> |
| `addressesReference` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectreference-v1-core)_ | AddressesReference is a reference to the Addresses object that contains the IP addresses.<br />If this field is set, the Addresses field will be ignored. |  |  |
| `ports` _[EndpointPort](#endpointport) array_ | Port numbers available on the related IP addresses.<br />This field is ignored for routes that are using kubernetes resources as the source. |  | MinItems: 1 <br /> |

#### LoadBalancerList

LoadBalancerList contains a list of LoadBalancer

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `LoadBalancerList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LoadBalancer](#loadbalancer) array_ |  |  |  |

#### LoadBalancerPort

LoadBalancerPort contains information on service's port.

_Appears in:_

- [LoadBalancerSpec](#loadbalancerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The name of this port within the service. This must be a DNS_LABEL.<br />All ports within a Spec must have unique names. When considering<br />the endpoints for a Service, this must match the 'name' field in the<br />EndpointPort.<br />Optional if only one ServicePort is defined on this service. |  |  |
| `protocol` _[Protocol](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#protocol-v1-core)_ | The IP protocol for this port. Defaults to "TCP". |  | Enum: [TCP UDP] <br /> |
| `port` _integer_ | The port that will be exposed by the LoadBalancer. |  |  |

#### LoadBalancerSettings

LoadBalancerSettings defines the settings for the load balancers.

_Appears in:_

- [ConfigSpec](#configspec)
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `class` _string_ | Class is the class of the load balancer to use.<br />This has higher precedence than the value specified in the Config. |  |  |
| `disable` _boolean_ | Disable is a flag that can be used to disable L4 load balancing for a tenant. |  |  |

#### LoadBalancerSpec

LoadBalancerSpec defines the desired state of LoadBalancer

_Appears in:_

- [LoadBalancer](#loadbalancer)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoints` _[LoadBalancerEndpoints](#loadbalancerendpoints) array_ | Sets of addresses and ports that comprise an exposed user service on a cluster. |  | MinItems: 1 <br /> |
| `ports` _[LoadBalancerPort](#loadbalancerport) array_ | The list of ports that are exposed by the load balancer service.<br />only needed for layer 4 |  |  |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#servicetype-v1-core)_ | type determines how the Service is exposed. Defaults to ClusterIP. Valid<br />options are ExternalName, ClusterIP, NodePort, and LoadBalancer.<br />"ExternalName" maps to the specified externalName.<br />"ClusterIP" allocates a cluster-internal IP address for load-balancing to<br />endpoints. Endpoints are determined by the selector or if that is not<br />specified, by manual construction of an Endpoints object. If clusterIP is<br />"None", no virtual IP is allocated and the endpoints are published as a<br />set of endpoints rather than a stable IP.<br />"NodePort" builds on ClusterIP and allocates a port on every node which<br />routes to the clusterIP.<br />"LoadBalancer" builds on NodePort and creates an<br />external load-balancer (if supported in the current cloud) which routes<br />to the clusterIP.<br />More info: <https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types> | ClusterIP |  |

#### LoadBalancerStatus

LoadBalancerStatus defines the observed state of LoadBalancer

_Appears in:_

- [LoadBalancer](#loadbalancer)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `loadBalancer` _[LoadBalancerStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#loadbalancerstatus-v1-core)_ | LoadBalancer contains the current status of the load-balancer,<br />if one is present. |  |  |
| `service` _[ServiceStatus](#servicestatus)_ | Service contains the current status of the LB service. |  |  |

#### ResourceState

_Appears in:_

- [RouteResourcesStatus](#routeresourcesstatus)
- [RouteServiceStatus](#routeservicestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | APIVersion is the API version of the resource. |  |  |
| `name` _string_ | Name is the name of the resource. |  |  |
| `namespace` _string_ | Namespace is the namespace of the resource. |  |  |
| `generatedName` _string_ | GeneratedName is the generated name of the resource. |  |  |
| `status` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#rawextension-runtime-pkg)_ | Status is the actual status of the resource. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta) array_ |  |  |  |

#### Route

Route is the object that represents a route in the cluster.

_Appears in:_

- [RouteList](#routelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `Route` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RouteSpec](#routespec)_ |  |  |  |
| `status` _[RouteStatus](#routestatus)_ |  |  |  |

#### RouteList

RouteList contains a list of Routes

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `RouteList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Route](#route) array_ |  |  |  |

#### RouteResourcesStatus

_Appears in:_

- [RouteStatus](#routestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `source` _string_ |  |  |  |
| `services` _object (keys:string, values:[RouteServiceStatus](#routeservicestatus))_ |  |  |  |
| `route` _[ResourceState](#resourcestate)_ |  |  |  |

#### RouteServiceStatus

_Appears in:_

- [RouteResourcesStatus](#routeresourcesstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | APIVersion is the API version of the resource. |  |  |
| `name` _string_ | Name is the name of the resource. |  |  |
| `namespace` _string_ | Namespace is the namespace of the resource. |  |  |
| `generatedName` _string_ | GeneratedName is the generated name of the resource. |  |  |
| `status` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#rawextension-runtime-pkg)_ | Status is the actual status of the resource. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta) array_ |  |  |  |
| `ports` _[ServicePort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#serviceport-v1-core) array_ |  |  |  |

#### RouteSource

_Appears in:_

- [RouteSpec](#routespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kubernetes` _[KubernetesSource](#kubernetessource)_ | Kubernetes contains the information about the Kubernetes source.<br />This field is automatically populated by the KubeLB CCM and in most cases, users should not set this field manually. |  |  |

#### RouteSpec

RouteSpec defines the desired state of the Route.

_Appears in:_

- [Route](#route)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoints` _[LoadBalancerEndpoints](#loadbalancerendpoints) array_ | Sets of addresses and ports that comprise an exposed user service on a cluster. |  | MinItems: 1 <br /> |
| `source` _[RouteSource](#routesource)_ | Source contains the information about the source of the route. This is used when the route is created from external sources. |  |  |

#### RouteStatus

RouteStatus defines the observed state of the Route.

_Appears in:_

- [Route](#route)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[RouteResourcesStatus](#routeresourcesstatus)_ | Resources contains the list of resources that are created/processed as a result of the Route. |  |  |

#### ServicePort

ServicePort contains information on service's port.

_Appears in:_

- [ServiceStatus](#servicestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The name of this port within the service. This must be a DNS_LABEL.<br />All ports within a ServiceSpec must have unique names. When considering<br />the endpoints for a Service, this must match the 'name' field in the<br />EndpointPort.<br />Optional if only one ServicePort is defined on this service. |  |  |
| `protocol` _[Protocol](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#protocol-v1-core)_ | The IP protocol for this port. Supports "TCP", "UDP", and "SCTP".<br />Default is TCP. |  |  |
| `appProtocol` _string_ | The application protocol for this port.<br />This is used as a hint for implementations to offer richer behavior for protocols that they understand.<br />This field follows standard Kubernetes label syntax.<br />Valid values are either:<br /><br />*Un-prefixed protocol names - reserved for IANA standard service names (as per<br />RFC-6335 and <https://www.iana.org/assignments/service-names>).<br /><br />* Kubernetes-defined prefixed names:<br />  *'kubernetes.io/h2c' - HTTP/2 prior knowledge over cleartext as described in <https://www.rfc-editor.org/rfc/rfc9113.html#name-starting-http-2-with-prior-><br />* 'kubernetes.io/ws'  - WebSocket over cleartext as described in <https://www.rfc-editor.org/rfc/rfc6455><br />  *'kubernetes.io/wss' - WebSocket over TLS as described in <https://www.rfc-editor.org/rfc/rfc6455><br /><br />* Other protocols should use implementation-defined prefixed names such as<br />mycompany.com/my-custom-protocol. |  |  |
| `port` _integer_ | The port that will be exposed by this service. |  |  |
| `targetPort` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#intorstring-intstr-util)_ | Number or name of the port to access on the pods targeted by the service.<br />Number must be in the range 1 to 65535. Name must be an IANA_SVC_NAME.<br />If this is a string, it will be looked up as a named port in the<br />target Pod's container ports. If this is not specified, the value<br />of the 'port' field is used (an identity map).<br />This field is ignored for services with clusterIP=None, and should be<br />omitted or set equal to the 'port' field.<br />More info: <https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service> |  |  |
| `nodePort` _integer_ | The port on each node on which this service is exposed when type is<br />NodePort or LoadBalancer.  Usually assigned by the system. If a value is<br />specified, in-range, and not in use it will be used, otherwise the<br />operation will fail.  If not specified, a port will be allocated if this<br />Service requires one.  If this field is specified when creating a<br />Service which does not need it, creation will fail. This field will be<br />wiped when updating a Service to no longer need it (e.g. changing type<br />from NodePort to ClusterIP).<br />More info: <https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport> |  |  |
| `upstreamTargetPort` _integer_ |  |  |  |

#### ServiceStatus

_Appears in:_

- [LoadBalancerStatus](#loadbalancerstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ports` _[ServicePort](#serviceport) array_ |  |  |  |

#### SyncSecret

SyncSecret is a wrapper over Kubernetes Secret object. This is used to sync secrets from tenants to the LB cluster in a controlled and secure way.

_Appears in:_

- [SyncSecretList](#syncsecretlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `SyncSecret` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `data` _object (keys:string, values:integer array)_ |  |  |  |
| `stringData` _object (keys:string, values:string)_ |  |  |  |
| `type` _[SecretType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#secrettype-v1-core)_ |  |  |  |

#### SyncSecretList

SyncSecretList contains a list of SyncSecrets

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `SyncSecretList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[SyncSecret](#syncsecret) array_ |  |  |  |

#### Tenant

Tenant is the Schema for the tenants API

_Appears in:_

- [TenantList](#tenantlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `Tenant` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[TenantSpec](#tenantspec)_ |  |  |  |
| `status` _[TenantStatus](#tenantstatus)_ |  |  |  |

#### TenantList

TenantList contains a list of Tenant

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `TenantList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Tenant](#tenant) array_ |  |  |  |

#### TenantSpec

TenantSpec defines the desired state of Tenant

_Appears in:_

- [Tenant](#tenant)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `propagatedAnnotations` _map[string]string_ | PropagatedAnnotations defines the list of annotations(key-value pairs) that will be propagated to the LoadBalancer service. Keep the `value` field empty in the key-value pair to allow any value.<br />This will have a higher precedence than the annotations specified at the Config level. |  |  |
| `propagateAllAnnotations` _boolean_ | PropagateAllAnnotations defines whether all annotations will be propagated to the LoadBalancer service. If set to true, PropagatedAnnotations will be ignored.<br />This will have a higher precedence than the value specified at the Config level. |  |  |
| `loadBalancer` _[LoadBalancerSettings](#loadbalancersettings)_ |  |  |  |
| `ingress` _[IngressSettings](#ingresssettings)_ |  |  |  |
| `gatewayAPI` _[GatewayAPISettings](#gatewayapisettings)_ |  |  |  |

#### TenantStatus

TenantStatus defines the observed state of Tenant

_Appears in:_

- [Tenant](#tenant)

#### UpstreamService

UpstreamService is a wrapper over the corev1.Service object.
This is required as kubebuilder:validation:EmbeddedResource marker adds the x-kubernetes-embedded-resource to the array instead of
the elements within it. Which results in a broken CRD; validation error. Without this marker, the embedded resource is not properly
serialized to the CRD.

_Appears in:_

- [KubernetesSource](#kubernetessource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ServiceSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#servicespec-v1-core)_ | Spec defines the behavior of a service.<br /><https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status> |  |  |
| `status` _[ServiceStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#servicestatus-v1-core)_ | Most recently observed status of the service.<br />Populated by the system.<br />Read-only.<br />More info: <https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status> |  |  |
