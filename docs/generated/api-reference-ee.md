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
- [TenantState](#tenantstate)
- [TenantStateList](#tenantstatelist)
- [Tunnel](#tunnel)
- [TunnelList](#tunnellist)
- [WAFPolicy](#wafpolicy)
- [WAFPolicyList](#wafpolicylist)



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



#### AnnotatedResource

_Underlying type:_ _string_



_Validation:_
- Enum: [all service ingress gateway httproute grpcroute tcproute udproute tlsroute]

_Appears in:_
- [AnnotationSettings](#annotationsettings)
- [ConfigSpec](#configspec)
- [TenantSpec](#tenantspec)

| Field | Description |
| --- | --- |
| `all` |  |
| `service` |  |
| `ingress` |  |
| `gateway` |  |
| `httproute` |  |
| `grpcroute` |  |
| `tcproute` |  |
| `udproute` |  |
| `tlsroute` |  |


#### AnnotationSettings







_Appears in:_
- [ConfigSpec](#configspec)
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `propagatedAnnotations` _map[string]string_ | PropagatedAnnotations defines the list of annotations(key-value pairs) that will be propagated to the LoadBalancer service. Keep the `value` field empty in the key-value pair to allow any value.<br />Tenant configuration has higher precedence than the annotations specified at the Config level. |  |  |
| `propagateAllAnnotations` _boolean_ | PropagateAllAnnotations defines whether all annotations will be propagated to the LoadBalancer service. If set to true, PropagatedAnnotations will be ignored.<br />Tenant configuration has higher precedence than the value specified at the Config level. |  |  |
| `defaultAnnotations` _object (keys:[AnnotatedResource](#annotatedresource), values:[Annotations](#annotations))_ | DefaultAnnotations defines the list of annotations(key-value pairs) that will be set on the load balancing resources if not already present. A special key `all` can be used to apply the same<br />set of annotations to all resources.<br />Tenant configuration has higher precedence than the annotations specified at the Config level. |  |  |


#### Annotations

_Underlying type:_ _object_





_Appears in:_
- [AnnotationSettings](#annotationsettings)
- [ConfigSpec](#configspec)
- [TenantSpec](#tenantspec)



#### CertificatesSettings



CertificatesSettings defines the settings for the certificates.



_Appears in:_
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disable` _boolean_ | Disable is a flag that can be used to disable certificate automation for a tenant. |  |  |
| `defaultClusterIssuer` _string_ | DefaultClusterIssuer is the Cluster Issuer to use for the certificates by default. This is applied when the cluster issuer is not specified in the annotations on the resource itself. |  |  |
| `allowedDomains` _string array_ | AllowedDomains is a list of allowed domains for automated Certificate management. Has a higher precedence than the value specified in the Config.<br />If empty, the value specified in `tenant.spec.allowedDomains` will be used.<br />Examples:<br />- ["*.example.com"] -> this allows subdomains at the root level such as example.com and test.example.com but won't allow domains at one level above like test.test.example.com<br />- ["**.example.com"] -> this allows all subdomains of example.com such as test.dns.example.com and dns.example.com<br />- ["example.com"] -> this allows only example.com<br />- ["**"] or ["*"] -> this allows all domains<br />Note: "**" was added as a special case to allow any levels of subdomains that come before it. "*" works for only 1 level. |  |  |


#### CircuitBreaker



CircuitBreaker defines the Circuit Breaker configuration for Envoy clusters.
Circuit breakers prevent cascading failures by limiting connections/requests to upstream clusters. For more info: https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/circuit_breaking



_Appears in:_
- [ConfigSpec](#configspec)
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxConnections` _integer_ | MaxConnections is the maximum number of connections that Envoy will establish to all endpoints in the cluster.<br />If not specified, the default is 1024. |  | Maximum: 4.294967295e+09 <br />Minimum: 0 <br /> |
| `maxPendingRequests` _integer_ | MaxPendingRequests is the maximum number of pending requests that Envoy will queue to the cluster.<br />If not specified, the default is 1024. |  | Maximum: 4.294967295e+09 <br />Minimum: 0 <br /> |
| `maxParallelRequests` _integer_ | MaxParallelRequests is the maximum number of parallel requests that Envoy will make to the cluster.<br />This is applicable to HTTP/2 and gRPC connections.<br />If not specified, the default is 1024. |  | Maximum: 4.294967295e+09 <br />Minimum: 0 <br /> |
| `maxParallelRetries` _integer_ | MaxParallelRetries is the maximum number of parallel retries that Envoy will make to the cluster.<br />If not specified, the default is 3. |  | Maximum: 4.294967295e+09 <br />Minimum: 0 <br /> |
| `maxRequestsPerConnection` _integer_ | MaxRequestsPerConnection is the maximum number of requests that Envoy will make over a single connection<br />to the cluster. If not specified, there is no limit. |  | Maximum: 4.294967295e+09 <br />Minimum: 0 <br /> |
| `perEndpoint` _[PerEndpointCircuitBreaker](#perendpointcircuitbreaker)_ | PerEndpoint configures circuit breaker thresholds that apply to individual endpoints rather than the whole cluster. |  |  |




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
| `status` _[ConfigStatus](#configstatus)_ |  |  |  |


#### ConfigCertificatesSettings



ConfigCertificatesSettings defines the global settings for the certificates.



_Appears in:_
- [ConfigSpec](#configspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disable` _boolean_ | Disable is a flag that can be used to disable certificate automation globally for all the tenants. |  |  |
| `defaultClusterIssuer` _string_ | DefaultClusterIssuer is the Cluster Issuer to use for the certificates by default. This is applied when the cluster issuer is not specified in the annotations on the resource itself. |  |  |


#### ConfigDNSSettings



ConfigDNSSettings defines the global settings for DNS management and automation.



_Appears in:_
- [ConfigSpec](#configspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disable` _boolean_ | Disable is a flag that can be used to disable DNS automation globally for all the tenants. |  |  |
| `wildcardDomain` _string_ | WildcardDomain is the domain that will be used as the base domain to create wildcard DNS records for DNS resources.<br />This is only used for determining the hostname for LoadBalancer and Tunnel resources. |  |  |
| `allowExplicitHostnames` _boolean_ | AllowExplicitHostnames is a flag that can be used to allow explicit hostnames to be used for DNS resources.<br />This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set. |  |  |
| `useDNSAnnotations` _boolean_ | UseDNSAnnotations is a flag that can be used to add DNS annotations to DNS resources.<br />This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set. |  |  |
| `useCertificateAnnotations` _boolean_ | UseCertificateAnnotations is a flag that can be used to add Certificate annotations to Certificate resources.<br />This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set. |  |  |


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
| `propagatedAnnotations` _map[string]string_ | PropagatedAnnotations defines the list of annotations(key-value pairs) that will be propagated to the LoadBalancer service. Keep the `value` field empty in the key-value pair to allow any value.<br />Tenant configuration has higher precedence than the annotations specified at the Config level. |  |  |
| `propagateAllAnnotations` _boolean_ | PropagateAllAnnotations defines whether all annotations will be propagated to the LoadBalancer service. If set to true, PropagatedAnnotations will be ignored.<br />Tenant configuration has higher precedence than the value specified at the Config level. |  |  |
| `defaultAnnotations` _object (keys:[AnnotatedResource](#annotatedresource), values:[Annotations](#annotations))_ | DefaultAnnotations defines the list of annotations(key-value pairs) that will be set on the load balancing resources if not already present. A special key `all` can be used to apply the same<br />set of annotations to all resources.<br />Tenant configuration has higher precedence than the annotations specified at the Config level. |  |  |
| `envoyProxy` _[EnvoyProxy](#envoyproxy)_ | EnvoyProxy defines the desired state of the Envoy Proxy |  |  |
| `loadBalancer` _[LoadBalancerSettings](#loadbalancersettings)_ |  |  |  |
| `ingress` _[IngressSettings](#ingresssettings)_ |  |  |  |
| `gatewayAPI` _[GatewayAPISettings](#gatewayapisettings)_ |  |  |  |
| `dns` _[ConfigDNSSettings](#configdnssettings)_ |  |  |  |
| `certificates` _[ConfigCertificatesSettings](#configcertificatessettings)_ |  |  |  |
| `tunnel` _[TunnelSettings](#tunnelsettings)_ |  |  |  |
| `circuitBreaker` _[CircuitBreaker](#circuitbreaker)_ | CircuitBreaker defines the default circuit breaker configuration for all Envoy clusters.<br />These settings can be overridden at the Tenant level. |  |  |
| `waf` _[WAFSettings](#wafsettings)_ | WAF defines WAF-related settings. |  |  |


#### ConfigStatus



ConfigStatus defines the observed state of the Config.



_Appears in:_
- [Config](#config)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _[Version](#version)_ |  |  |  |


#### DNSSettings



DNSSettings defines the tenant specific settings for DNS management and automation.



_Appears in:_
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disable` _boolean_ | Disable is a flag that can be used to disable DNS automation for a tenant. |  |  |
| `allowedDomains` _string array_ | AllowedDomains is a list of allowed domains for automated DNS management. Has a higher precedence than the value specified in the Config.<br />If empty, the value specified in `tenant.spec.allowedDomains` will be used.<br />Examples:<br />- ["*.example.com"] -> this allows subdomains at the root level such as example.com and test.example.com but won't allow domains at one level above like test.test.example.com<br />- ["**.example.com"] -> this allows all subdomains of example.com such as test.dns.example.com and dns.example.com<br />- ["example.com"] -> this allows only example.com<br />- ["**"] or ["*"] -> this allows all domains<br />Note: "**" was added as a special case to allow any levels of subdomains that come before it. "*" works for only 1 level. |  |  |
| `wildcardDomain` _string_ | WildcardDomain is the domain that will be used as the base domain to create wildcard DNS records for DNS resources.<br />This is only used for determining the hostname for LoadBalancer and Tunnel resources. |  |  |
| `allowExplicitHostnames` _boolean_ | AllowExplicitHostnames is a flag that can be used to allow explicit hostnames to be used for DNS resources.<br />This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set. |  |  |
| `useDNSAnnotations` _boolean_ | UseDNSAnnotations is a flag that can be used to add DNS annotations to DNS resources.<br />This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set. |  |  |
| `useCertificateAnnotations` _boolean_ | UseCertificateAnnotations is a flag that can be used to add Certificate annotations to Certificate resources.<br />This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set. |  |  |


#### EndpointAddress



EndpointAddress is a tuple that describes single IP address.



_Appears in:_
- [AddressesSpec](#addressesspec)
- [LoadBalancerEndpoints](#loadbalancerendpoints)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ip` _string_ | The IP of the endpoint. This can be an IPv4 or IPv6 address.<br />The IP address must not be IP CIDR, Loopback (127.0.0.0/8), link-local (169.254.0.0/16), or link-local multicast ((224.0.0.0/24) addresses. |  |  |
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
| `topology` _[EnvoyProxyTopology](#envoyproxytopology)_ | Topology defines the deployment topology for Envoy Proxy. The only supported value is: shared.<br />DEPRECATION NOTICE: The values "dedicated" and "global" are deprecated and will be removed in a future release. Both will now default to shared topology. | shared | Enum: [shared dedicated global] <br /> |
| `useDaemonset` _boolean_ | UseDaemonset defines whether Envoy Proxy will run as daemonset. By default, Envoy Proxy will run as deployment.<br />If set to true, Replicas will be ignored. |  |  |
| `replicas` _integer_ | Replicas defines the number of replicas for Envoy Proxy. This field is ignored if UseDaemonset is set to true. | 3 | Minimum: 1 <br /> |
| `singlePodPerNode` _boolean_ | SinglePodPerNode defines whether Envoy Proxy pods will be spread across nodes. This ensures that multiple replicas are not running on the same node. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector is used to select nodes to run Envoy Proxy. If specified, the node must have all the indicated labels. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#toleration-v1-core) array_ | Tolerations is used to schedule Envoy Proxy pods on nodes with matching taints. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#resourcerequirements-v1-core)_ | Resources defines the resource requirements for Envoy Proxy. |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#affinity-v1-core)_ | Affinity is used to schedule Envoy Proxy pods on nodes with matching affinity. |  |  |
| `image` _string_ | Image defines the Envoy Proxy image to use. |  |  |
| `gracefulShutdown` _[EnvoyProxyGracefulShutdown](#envoyproxygracefulshutdown)_ | GracefulShutdown defines the graceful shutdown configuration for Envoy Proxy. |  |  |
| `overloadManager` _[EnvoyProxyOverloadManager](#envoyproxyoverloadmanager)_ | OverloadManager defines the overload manager configuration for Envoy XDS bootstrap. |  |  |
| `podMonitor` _[EnvoyProxyPodMonitor](#envoyproxypodmonitor)_ | PodMonitor enables creation of PodMonitor resources for Envoy Proxy pods<br />to enable metrics scraping by Prometheus Operator. |  |  |


#### EnvoyProxyGracefulShutdown



EnvoyProxyGracefulShutdown defines the graceful shutdown configuration for Envoy Proxy



_Appears in:_
- [EnvoyProxy](#envoyproxy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disabled` _boolean_ | Disabled controls whether graceful shutdown is disabled |  |  |
| `drainTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#duration-v1-meta)_ | DrainTimeout is the maximum time to wait for connections to drain.<br />Defaults to 60s. Must be less than TerminationGracePeriodSeconds. | 60s |  |
| `minDrainDuration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#duration-v1-meta)_ | MinDrainDuration is the minimum time to wait before checking connection count.<br />This prevents premature termination. Defaults to 5s. | 5s |  |
| `terminationGracePeriodSeconds` _integer_ | TerminationGracePeriodSeconds is the grace period for pod termination.<br />Must be greater than DrainTimeout. Defaults to 300s. | 300 | Minimum: 30 <br /> |
| `shutdownManagerImage` _string_ | ShutdownManagerImage is the Docker image for the shutdown-manager sidecar.<br />Defaults to "docker.io/envoyproxy/gateway:v1.3.0" | docker.io/envoyproxy/gateway:v1.3.0 |  |


#### EnvoyProxyOverloadManager



EnvoyProxyOverloadManager defines the overload manager configuration for Envoy XDS



_Appears in:_
- [EnvoyProxy](#envoyproxy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether overload manager is enabled |  |  |
| `maxActiveDownstreamConnections` _integer_ | MaxActiveDownstreamConnections is the maximum number of active downstream connections for the Envoy. |  |  |
| `maxHeapSizeBytes` _integer_ | MaxHeapSizeBytes is the maximum heap size for the Envoy in bytes. On reaching the limit, the Envoy will start to reject new connections. |  |  |


#### EnvoyProxyPodMonitor



EnvoyProxyPodMonitor defines the PodMonitor configuration for Envoy Proxy



_Appears in:_
- [EnvoyProxy](#envoyproxy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether a PodMonitor is created for Envoy Proxy pods. |  |  |


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
| `defaultGateway` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectreference-v1-core)_ | DefaultGateway is the default gateway reference to use for the tenant. This is only used for load balancer hostname and tunneling. |  |  |
| `gateway` _[GatewaySettings](#gatewaysettings)_ |  |  |  |
| `disableHTTPRoute` _boolean_ |  |  |  |
| `disableGRPCRoute` _boolean_ |  |  |  |
| `disableTCPRoute` _boolean_ |  |  |  |
| `disableUDPRoute` _boolean_ |  |  |  |
| `disableTLSRoute` _boolean_ |  |  |  |
| `disableBackendTrafficPolicy` _boolean_ |  |  |  |
| `disableClientTrafficPolicy` _boolean_ |  |  |  |


#### GatewayAPIsSettings







_Appears in:_
- [GatewayAPISettings](#gatewayapisettings)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disableHTTPRoute` _boolean_ |  |  |  |
| `disableGRPCRoute` _boolean_ |  |  |  |
| `disableTCPRoute` _boolean_ |  |  |  |
| `disableUDPRoute` _boolean_ |  |  |  |
| `disableTLSRoute` _boolean_ |  |  |  |
| `disableBackendTrafficPolicy` _boolean_ |  |  |  |
| `disableClientTrafficPolicy` _boolean_ |  |  |  |


#### GatewaySettings



GatewaySettings defines the settings for the gateway resource.



_Appears in:_
- [GatewayAPISettings](#gatewayapisettings)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `limit` _integer_ | Limit is the maximum number of gateways to create.<br />If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this<br />is that it is not possible for KubeLB to know which resources are safe to remove. |  |  |


#### HostnameStatus







_Appears in:_
- [LoadBalancerStatus](#loadbalancerstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `hostname` _string_ | Hostname contains the hostname of the load-balancer. |  |  |
| `tlsEnabled` _boolean_ | TLSEnabled is true if certificate is created for the hostname. |  |  |
| `dnsRecordCreated` _boolean_ | DNSRecordCreated is true if DNS record is created for the hostname. |  |  |


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


#### LoadBalancerPersistence



LoadBalancerPersistence configures backend persistence for a LoadBalancer.



_Appears in:_
- [LoadBalancerSpec](#loadbalancerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[LoadBalancerPersistenceType](#loadbalancerpersistencetype)_ | Type selects the persistence strategy.<br />SourceIP uses the downstream source IP as observed by KubeLB Envoy. |  | Enum: [SourceIP] <br /> |


#### LoadBalancerPersistenceType

_Underlying type:_ _string_

LoadBalancerPersistenceType defines the supported backend persistence modes.

_Validation:_
- Enum: [SourceIP]

_Appears in:_
- [LoadBalancerPersistence](#loadbalancerpersistence)

| Field | Description |
| --- | --- |
| `SourceIP` | LoadBalancerPersistenceTypeSourceIP routes connections from the same<br />observed source IP to the same healthy backend endpoint when possible.<br /> |


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
| `limit` _integer_ | Limit is the maximum number of load balancers to create.<br />If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this<br />is that it is not possible for KubeLB to know which resources are safe to remove. |  |  |
| `disable` _boolean_ | Disable is a flag that can be used to disable L4 load balancing for a tenant. |  |  |


#### LoadBalancerSpec



LoadBalancerSpec defines the desired state of LoadBalancer



_Appears in:_
- [LoadBalancer](#loadbalancer)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoints` _[LoadBalancerEndpoints](#loadbalancerendpoints) array_ | Sets of addresses and ports that comprise an exposed user service on a cluster. |  | MinItems: 1 <br /> |
| `ports` _[LoadBalancerPort](#loadbalancerport) array_ | The list of ports that are exposed by the load balancer service.<br />only needed for layer 4 |  |  |
| `hostname` _string_ | Hostname is the domain name at which the load balancer service will be accessible.<br />When hostname is set, KubeLB will create a route(ingress or httproute) for the service, and expose it with TLS on the given hostname. |  |  |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#servicetype-v1-core)_ | type determines how the Service is exposed. Defaults to ClusterIP. Valid<br />options are ExternalName, ClusterIP, NodePort, and LoadBalancer.<br />"ExternalName" maps to the specified externalName.<br />"ClusterIP" allocates a cluster-internal IP address for load-balancing to<br />endpoints. Endpoints are determined by the selector or if that is not<br />specified, by manual construction of an Endpoints object. If clusterIP is<br />"None", no virtual IP is allocated and the endpoints are published as a<br />set of endpoints rather than a stable IP.<br />"NodePort" builds on ClusterIP and allocates a port on every node which<br />routes to the clusterIP.<br />"LoadBalancer" builds on NodePort and creates an<br />external load-balancer (if supported in the current cloud) which routes<br />to the clusterIP.<br />More info: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types | ClusterIP |  |
| `externalTrafficPolicy` _[ServiceExternalTrafficPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#serviceexternaltrafficpolicy-v1-core)_ | externalTrafficPolicy denotes if this Service desires to route external traffic to<br />node-local or cluster-wide endpoints. "Local" preserves the client source IP and avoids<br />a second hop for LoadBalancer and Nodeport type services, but risks potentially imbalanced<br />traffic spreading. "Cluster" obscures the client source IP and may cause a second hop to<br />another node, but should have good overall load-spreading. |  |  |
| `persistence` _[LoadBalancerPersistence](#loadbalancerpersistence)_ | Persistence configures backend endpoint persistence. When omitted,<br />KubeLB keeps the default non-sticky load balancing behavior.<br />SourceIP persistence is based on the source IP observed by KubeLB Envoy<br />for TCP and UDP traffic, which may be a gateway, node, or NAT address in<br />proxied topologies. |  |  |


#### LoadBalancerState







_Appears in:_
- [TenantStateStatus](#tenantstatestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disable` _boolean_ |  |  |  |
| `limit` _integer_ |  |  |  |


#### LoadBalancerStatus



LoadBalancerStatus defines the observed state of LoadBalancer



_Appears in:_
- [LoadBalancer](#loadbalancer)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `loadBalancer` _[LoadBalancerStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#loadbalancerstatus-v1-core)_ | LoadBalancer contains the current status of the load-balancer,<br />if one is present. |  |  |
| `service` _[ServiceStatus](#servicestatus)_ | Service contains the current status of the LB service. |  |  |
| `hostname` _[HostnameStatus](#hostnamestatus)_ | Hostname contains the status for hostname resources. |  |  |


#### PerEndpointCircuitBreaker



PerEndpointCircuitBreaker defines circuit breaker thresholds that apply to individual endpoints.



_Appears in:_
- [CircuitBreaker](#circuitbreaker)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxConnections` _integer_ | MaxConnections is the maximum number of connections that Envoy will establish to a single endpoint.<br />If not specified, the default is 1024. |  | Maximum: 4.294967295e+09 <br />Minimum: 0 <br /> |


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
| `endpoints` _[LoadBalancerEndpoints](#loadbalancerendpoints) array_ | Sets of addresses and ports that comprise an exposed user service on a cluster.<br />This field is required for Routes that represent traffic-forwarding resources (Ingress, Gateway routes).<br />It is optional for policy resources like BackendTrafficPolicy. |  |  |
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
| `appProtocol` _string_ | The application protocol for this port.<br />This is used as a hint for implementations to offer richer behavior for protocols that they understand.<br />This field follows standard Kubernetes label syntax.<br />Valid values are either:<br />* Un-prefixed protocol names - reserved for IANA standard service names (as per<br />RFC-6335 and https://www.iana.org/assignments/service-names).<br />* Kubernetes-defined prefixed names:<br />  * 'kubernetes.io/h2c' - HTTP/2 prior knowledge over cleartext as described in https://www.rfc-editor.org/rfc/rfc9113.html#name-starting-http-2-with-prior-<br />  * 'kubernetes.io/ws'  - WebSocket over cleartext as described in https://www.rfc-editor.org/rfc/rfc6455<br />  * 'kubernetes.io/wss' - WebSocket over TLS as described in https://www.rfc-editor.org/rfc/rfc6455<br />* Other protocols should use implementation-defined prefixed names such as<br />mycompany.com/my-custom-protocol. |  |  |
| `port` _integer_ | The port that will be exposed by this service. |  |  |
| `targetPort` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#intorstring-intstr-util)_ | Number or name of the port to access on the pods targeted by the service.<br />Number must be in the range 1 to 65535. Name must be an IANA_SVC_NAME.<br />If this is a string, it will be looked up as a named port in the<br />target Pod's container ports. If this is not specified, the value<br />of the 'port' field is used (an identity map).<br />This field is ignored for services with clusterIP=None, and should be<br />omitted or set equal to the 'port' field.<br />More info: https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service |  |  |
| `nodePort` _integer_ | The port on each node on which this service is exposed when type is<br />NodePort or LoadBalancer.  Usually assigned by the system. If a value is<br />specified, in-range, and not in use it will be used, otherwise the<br />operation will fail.  If not specified, a port will be allocated if this<br />Service requires one.  If this field is specified when creating a<br />Service which does not need it, creation will fail. This field will be<br />wiped when updating a Service to no longer need it (e.g. changing type<br />from NodePort to ClusterIP).<br />More info: https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport |  |  |
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
| `immutable` _boolean_ |  |  |  |
| `data` _object (keys:string, values:integer array)_ |  |  |  |
| `stringData` _object (keys:string, values:string)_ |  |  |  |
| `type` _[SecretType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#secrettype-v1-core)_ |  |  |  |
| `status` _[SyncSecretStatus](#syncsecretstatus)_ |  |  |  |


#### SyncSecretList



SyncSecretList contains a list of SyncSecrets





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `SyncSecretList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[SyncSecret](#syncsecret) array_ |  |  |  |


#### SyncSecretPhase

_Underlying type:_ _string_

SyncSecretPhase represents the lifecycle phase of a SyncSecret.



_Appears in:_
- [SyncSecretStatus](#syncsecretstatus)

| Field | Description |
| --- | --- |
| `Pending` | SyncSecretPhasePending means the SyncSecret has not yet been synced.<br /> |
| `Synced` | SyncSecretPhaseSynced means the SyncSecret has been successfully synced to a Secret.<br /> |
| `Failed` | SyncSecretPhaseFailed means the SyncSecret sync failed.<br /> |
| `Terminating` | SyncSecretPhaseTerminating means the SyncSecret is being deleted.<br /> |


#### SyncSecretStatus



SyncSecretStatus defines the observed state of SyncSecret.



_Appears in:_
- [SyncSecret](#syncsecret)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed for this SyncSecret by the controller. |  |  |
| `phase` _[SyncSecretPhase](#syncsecretphase)_ | Phase is the current lifecycle phase of the SyncSecret. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta) array_ | Conditions represents the latest available observations of the SyncSecret's state. |  |  |


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


#### TenantEnvoyProxy



TenantEnvoyProxy defines tenant-level overrides for Envoy Proxy configuration.



_Appears in:_
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Replicas is the number of Envoy Proxy replicas for this tenant.<br />This field is ignored if Config.Spec.EnvoyProxy.UseDaemonset is true. |  | Minimum: 1 <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#resourcerequirements-v1-core)_ | Resources defines the resource requirements for the Envoy Proxy container. |  |  |


#### TenantList



TenantList contains a list of Tenant





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `TenantList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Tenant](#tenant) array_ |  |  |  |


#### TenantPhase

_Underlying type:_ _string_

TenantPhase represents the lifecycle phase of a Tenant.



_Appears in:_
- [TenantStatus](#tenantstatus)

| Field | Description |
| --- | --- |
| `Pending` | TenantPhasePending means the Tenant is being provisioned.<br /> |
| `Ready` | TenantPhaseReady means the Tenant has been successfully reconciled.<br /> |
| `Failed` | TenantPhaseFailed means the Tenant reconciliation failed.<br /> |
| `Terminating` | TenantPhaseTerminating means the Tenant is being deleted.<br /> |


#### TenantSpec



TenantSpec defines the desired state of Tenant



_Appears in:_
- [Tenant](#tenant)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `propagatedAnnotations` _map[string]string_ | PropagatedAnnotations defines the list of annotations(key-value pairs) that will be propagated to the LoadBalancer service. Keep the `value` field empty in the key-value pair to allow any value.<br />Tenant configuration has higher precedence than the annotations specified at the Config level. |  |  |
| `propagateAllAnnotations` _boolean_ | PropagateAllAnnotations defines whether all annotations will be propagated to the LoadBalancer service. If set to true, PropagatedAnnotations will be ignored.<br />Tenant configuration has higher precedence than the value specified at the Config level. |  |  |
| `defaultAnnotations` _object (keys:[AnnotatedResource](#annotatedresource), values:[Annotations](#annotations))_ | DefaultAnnotations defines the list of annotations(key-value pairs) that will be set on the load balancing resources if not already present. A special key `all` can be used to apply the same<br />set of annotations to all resources.<br />Tenant configuration has higher precedence than the annotations specified at the Config level. |  |  |
| `loadBalancer` _[LoadBalancerSettings](#loadbalancersettings)_ |  |  |  |
| `ingress` _[IngressSettings](#ingresssettings)_ |  |  |  |
| `gatewayAPI` _[GatewayAPISettings](#gatewayapisettings)_ |  |  |  |
| `dns` _[DNSSettings](#dnssettings)_ |  |  |  |
| `certificates` _[CertificatesSettings](#certificatessettings)_ |  |  |  |
| `tunnel` _[TenantTunnelSettings](#tenanttunnelsettings)_ |  |  |  |
| `envoyProxy` _[TenantEnvoyProxy](#tenantenvoyproxy)_ | EnvoyProxy defines tenant-level overrides for Envoy Proxy configuration.<br />Fields set here take precedence over Config.Spec.EnvoyProxy. |  |  |
| `circuitBreaker` _[CircuitBreaker](#circuitbreaker)_ | CircuitBreaker defines the circuit breaker configuration for this tenant's Envoy clusters.<br />Overrides Config-level settings. |  |  |
| `allowedDomains` _string array_ | List of allowed domains for the tenant. This is used to restrict the domains that can be used<br />for the tenant. If specified, applies on all the components such as Ingress, GatewayAPI, DNS, certificates, etc.<br />Examples:<br />- ["*.example.com"] -> this allows subdomains at the root level such as example.com and test.example.com but won't allow domains at one level above like test.test.example.com<br />- ["**.example.com"] -> this allows all subdomains of example.com such as test.dns.example.com and dns.example.com<br />- ["example.com"] -> this allows only example.com<br />- ["**"] or ["*"] -> this allows all domains<br />Note: "**" was added as a special case to allow any levels of subdomains that come before it. "*" works for only 1 level.<br />Default: value is ["**"] and all domains are allowed. | [**] |  |


#### TenantState



TenantState is the Schema for the tenants API



_Appears in:_
- [TenantStateList](#tenantstatelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `TenantState` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[TenantStateSpec](#tenantstatespec)_ |  |  |  |
| `status` _[TenantStateStatus](#tenantstatestatus)_ |  |  |  |


#### TenantStateList



TenantStateList contains a list of TenantState





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `TenantStateList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[TenantState](#tenantstate) array_ |  |  |  |


#### TenantStateSpec



TenantStateSpec defines the desired state of TenantState.



_Appears in:_
- [TenantState](#tenantstate)



#### TenantStateStatus



TenantStateStatus defines the observed state of TenantState



_Appears in:_
- [TenantState](#tenantstate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _[Version](#version)_ |  |  |  |
| `lastUpdated` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#time-v1-meta)_ |  |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta) array_ |  |  |  |
| `tunnel` _[TunnelState](#tunnelstate)_ |  |  |  |
| `loadBalancer` _[LoadBalancerState](#loadbalancerstate)_ |  |  |  |
| `allowedDomains` _string array_ |  |  |  |


#### TenantStatus



TenantStatus defines the observed state of Tenant



_Appears in:_
- [Tenant](#tenant)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed for this Tenant by the controller. |  |  |
| `phase` _[TenantPhase](#tenantphase)_ | Phase is the current lifecycle phase of the Tenant. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta) array_ | Conditions represents the latest available observations of the Tenant's state. |  |  |


#### TenantTunnelSettings



TenantTunnelSettings defines the settings for the tunnel.



_Appears in:_
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `limit` _integer_ | Limit is the maximum number of tunnels to create.<br />If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this<br />is that it is not possible for KubeLB to know which resources are safe to remove. |  |  |
| `disable` _boolean_ | Disable is a flag that can be used to disable tunneling for a tenant. |  |  |


#### Tunnel



Tunnel is the Schema for the tunnels API



_Appears in:_
- [TunnelList](#tunnellist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `Tunnel` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[TunnelSpec](#tunnelspec)_ |  |  |  |
| `status` _[TunnelStatus](#tunnelstatus)_ |  |  |  |


#### TunnelList



TunnelList contains a list of Tunnel





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `TunnelList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Tunnel](#tunnel) array_ |  |  |  |


#### TunnelPhase

_Underlying type:_ _string_

TunnelPhase represents the phase of tunnel



_Appears in:_
- [TunnelStatus](#tunnelstatus)

| Field | Description |
| --- | --- |
| `Pending` | TunnelPhasePending means the tunnel is being provisioned<br /> |
| `Ready` | TunnelPhaseReady means the tunnel is ready to accept connections<br /> |
| `Failed` | TunnelPhaseFailed means the tunnel provisioning failed<br /> |
| `Terminating` | TunnelPhaseTerminating means the tunnel is being terminated<br /> |


#### TunnelResources



TunnelResources contains references to resources created for the tunnel



_Appears in:_
- [TunnelStatus](#tunnelstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceName` _string_ | ServiceName is the name of the service created for this tunnel |  |  |
| `routeRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectreference-v1-core)_ | RouteRef is a reference to the route (HTTPRoute or Ingress) created for this tunnel |  |  |


#### TunnelSettings



TunnelSettings defines the global settings for Tunnel resources.



_Appears in:_
- [ConfigSpec](#configspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `limit` _integer_ | Limit is the maximum number of tunnels to create.<br />If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this<br />is that it is not possible for KubeLB to know which resources are safe to remove. |  |  |
| `connectionManagerURL` _string_ | ConnectionManagerURL is the URL of the connection manager service that handles tunnel connections.<br />This is required if tunneling is enabled.<br />For example: "https://con.example.com" |  |  |
| `disable` _boolean_ | Disable indicates whether tunneling feature should be disabled. |  |  |


#### TunnelSpec



TunnelSpec defines the desired state of Tunnel



_Appears in:_
- [Tunnel](#tunnel)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `hostname` _string_ | Hostname is the hostname of the tunnel. If not specified, the hostname will be generated by KubeLB. |  |  |


#### TunnelState







_Appears in:_
- [TenantStateStatus](#tenantstatestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disable` _boolean_ |  |  |  |
| `limit` _integer_ |  |  |  |
| `connectionManagerURL` _string_ |  |  |  |


#### TunnelStatus



TunnelStatus defines the observed state of Tunnel



_Appears in:_
- [Tunnel](#tunnel)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `hostname` _string_ | Hostname contains the actual hostname assigned to the tunnel |  |  |
| `url` _string_ | URL contains the full URL to access the tunnel |  |  |
| `connectionManagerURL` _string_ | ConnectionManagerURL contains the URL that clients should use to establish tunnel connections |  |  |
| `phase` _[TunnelPhase](#tunnelphase)_ | Phase represents the current phase of the tunnel |  |  |
| `resources` _[TunnelResources](#tunnelresources)_ | Resources contains references to the resources created for this tunnel |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta) array_ | Conditions represents the current conditions of the tunnel |  |  |


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
| `spec` _[ServiceSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#servicespec-v1-core)_ | Spec defines the behavior of a service.<br />https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  |  |
| `status` _[ServiceStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#servicestatus-v1-core)_ | Most recently observed status of the service.<br />Populated by the system.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  |  |


#### Version







_Appears in:_
- [ConfigStatus](#configstatus)
- [TenantStateStatus](#tenantstatestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `gitVersion` _string_ |  |  |  |
| `gitCommit` _string_ |  |  |  |
| `buildDate` _string_ |  |  |  |
| `edition` _string_ |  |  |  |


#### WAFFailureMode

_Underlying type:_ _string_

WAFFailureMode defines how routes behave when WAF filter creation fails.

_Validation:_
- Enum: [Open Closed]

_Appears in:_
- [WAFPolicySpec](#wafpolicyspec)

| Field | Description |
| --- | --- |
| `Open` | WAFFailureModeOpen allows traffic through without WAF protection if filter fails.<br /> |
| `Closed` | WAFFailureModeClosed blocks traffic if WAF filter cannot be applied.<br /> |


#### WAFPolicy



WAFPolicy defines Web Application Firewall policy for L7 routes.
Applies to HTTPRoute, GRPCRoute, and Ingress resources.



_Appears in:_
- [WAFPolicyList](#wafpolicylist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `WAFPolicy` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[WAFPolicySpec](#wafpolicyspec)_ |  |  |  |
| `status` _[WAFPolicyStatus](#wafpolicystatus)_ |  |  |  |


#### WAFPolicyList



WAFPolicyList contains a list of WAFPolicy.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `WAFPolicyList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[WAFPolicy](#wafpolicy) array_ |  |  |  |


#### WAFPolicySpec



WAFPolicySpec defines the desired state of WAFPolicy.
Exactly one targeting method must be used: targetRef, targetSelector, or global.
Setting multiple targeting methods is invalid. Policies without any targeting are ignored.
Feature stage: Alpha



_Appears in:_
- [WAFPolicy](#wafpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `global` _boolean_ | Global when set to true applies this policy to all routes for all tenants within a KubeLB installation.<br />Mutually exclusive with TargetRef and TargetSelector.<br />Policies without global, targetRef, or targetSelector are ignored. |  |  |
| `targetRef` _[WAFTargetRef](#waftargetref)_ | TargetRef identifies a specific route by name and optionally namespace.<br />Mutually exclusive with Global and TargetSelector. |  |  |
| `targetSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#labelselector-v1-meta)_ | TargetSelector selects routes or HTTPRoute/GRPCRoute resources by label.<br />It checks whether the route has the labels or the labels of the HTTPRoute/GRPCRoute resource. In case of a<br />conflict, the labels of the Route resource takes precedence.<br />Mutually exclusive with Global and TargetRef. |  |  |
| `directives` _string array_ | Directives contains SecLang/ModSecurity directives passed to Coraza.<br />Reference: https://coraza.io/docs/seclang/directives/<br />If empty, the following OWASP CRS defaults are applied:<br />  - SecRuleEngine On<br />  - SecRequestBodyAccess On<br />  - SecRequestBodyLimit 13107200<br />  - Include @crs-setup-conf<br />  - Include @owasp_crs/*.conf |  |  |
| `failureMode` _[WAFFailureMode](#waffailuremode)_ | FailureMode defines behavior when WAF filter creation fails.<br />- Closed: Block traffic if WAF cannot be applied (default)<br />- Open: Allow traffic without WAF protection | Closed | Enum: [Open Closed] <br /> |


#### WAFPolicyStatus



WAFPolicyStatus defines the observed state of WAFPolicy.



_Appears in:_
- [WAFPolicy](#wafpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta) array_ | Conditions describe the current state of the WAFPolicy. |  |  |


#### WAFSettings



WAFSettings defines settings for the WAF (Web Application Firewall).



_Appears in:_
- [ConfigSpec](#configspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `wasmInitContainerImage` _string_ | WASMInitContainerImage overrides the image used for the WASM init container.<br />If empty, defaults to the kubelb-manager image detected at runtime. |  |  |
| `skipValidation` _boolean_ | SkipValidation skips directive validation for WAFPolicies.<br />When true, all WAFPolicies are marked as valid without parsing. |  |  |


#### WAFTargetRef



WAFTargetRef identifies a route by name.



_Appears in:_
- [WAFPolicySpec](#wafpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `group` _string_ | Group is the API group of the target resource. | gateway.networking.k8s.io |  |
| `namespace` _string_ | Namespace is the management cluster namespace (e.g., tenant-primary).<br />If omitted, matches across all namespaces. |  |  |
| `name` _string_ | Name is the name of the target resource which could either be the name of the resource in management cluster<br />that is generated by KubeLB or the `kubelb.k8c.io/origin-name` that is the original name of the resource in the tenant cluster. |  | MinLength: 1 <br /> |


