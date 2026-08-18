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
- [Insight](#insight)
- [InsightList](#insightlist)
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
- [TenantWAFPolicy](#tenantwafpolicy)
- [TenantWAFPolicyList](#tenantwafpolicylist)
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
| `propagatedAnnotations` _map[string]string_ | PropagatedAnnotations defines the set of annotation key patterns that will be propagated to load balancing resources.<br />Keys support shell-style glob patterns (e.g. "nginx.ingress.kubernetes.io/*"). Keep the value empty to allow any value;<br />otherwise the value is a comma-separated list of permitted values for exact match.<br />Tenant configuration has higher precedence than the annotations specified at the Config level. |  | Optional: \{\} <br /> |
| `propagateAllAnnotations` _boolean_ | PropagateAllAnnotations defines whether all annotations will be propagated to load balancing resources.<br />If set to true, PropagatedAnnotations is ignored. DeniedAnnotations still applies on top of this flag.<br />Tenant configuration has higher precedence than the value specified at the Config level. |  | Optional: \{\} <br /> |
| `deniedAnnotations` _string array_ | DeniedAnnotations is a list of annotation key patterns that are excluded from propagation, regardless of<br />PropagateAllAnnotations or PropagatedAnnotations. Patterns support shell-style globbing (e.g. "nginx.ingress.kubernetes.io/*").<br />Tenant configuration has higher precedence than the value specified at the Config level. |  | Optional: \{\} <br /> |
| `defaultAnnotations` _object (keys:[AnnotatedResource](#annotatedresource), values:[Annotations](#annotations))_ | DefaultAnnotations defines the list of annotations(key-value pairs) that will be set on the load balancing resources if not already present. A special key `all` can be used to apply the same<br />set of annotations to all resources.<br />Tenant configuration has higher precedence than the annotations specified at the Config level. |  | Optional: \{\} <br /> |


#### Annotations

_Underlying type:_ _object_





_Appears in:_
- [AnnotationSettings](#annotationsettings)
- [ConfigSpec](#configspec)
- [TenantSpec](#tenantspec)



#### BackendTransport







_Appears in:_
- [ConfigSpec](#configspec)
- [TenantStateStatus](#tenantstatestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `mode` _[BackendTransportMode](#backendtransportmode)_ | Mode controls how management Envoy connects to tenant backends.<br />Direct preserves the existing node-address plus workload NodePort topology.<br />MTLS routes L7 and L4 TCP traffic through a KubeLB-managed tenant Envoy proxy.<br />MTLS is a Beta / Technical Preview feature: safe to enable and supported,<br />but its configuration surface may still change between releases with<br />migration instructions. See https://docs.kubermatic.com/kubermatic/main/architecture/feature-stages/ | Direct | Enum: [Direct MTLS] <br />Optional: \{\} <br /> |
| `udp` _[BackendTransportUDP](#backendtransportudp)_ | UDP configures how UDP traffic reaches tenant backends when Mode is MTLS.<br />It has no effect in Direct mode. |  | Optional: \{\} <br /> |
| `tenantProxy` _[TenantProxy](#tenantproxy)_ | TenantProxy tunes the KubeLB-managed tenant Envoy proxy used in the<br />MTLS topology. It has no effect in Direct mode. |  | Optional: \{\} <br /> |


#### BackendTransportMode

_Underlying type:_ _string_





_Appears in:_
- [BackendTransport](#backendtransport)

| Field | Description |
| --- | --- |
| `Direct` |  |
| `MTLS` |  |


#### BackendTransportUDP







_Appears in:_
- [BackendTransport](#backendtransport)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `mode` _[BackendTransportUDPMode](#backendtransportudpmode)_ | Mode selects the UDP transport in the MTLS topology.<br />Tunnel wraps each UDP session in CONNECT-UDP over the encrypted mTLS<br />tenant proxy port. Direct is an escape hatch that keeps UDP on plain<br />per-service NodePorts (unencrypted) for workloads sensitive to the<br />tunnel's MTU overhead or Envoy's upstream CONNECT-UDP maturity. | Tunnel | Enum: [Tunnel Direct] <br />Optional: \{\} <br /> |


#### BackendTransportUDPMode

_Underlying type:_ _string_





_Appears in:_
- [BackendTransportUDP](#backendtransportudp)

| Field | Description |
| --- | --- |
| `Tunnel` |  |
| `Direct` |  |


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
| `maxConnections` _integer_ | MaxConnections is the maximum number of connections that Envoy will establish to all endpoints in the cluster.<br />If not specified, the default is 1024. |  | Maximum: 4.294967295e+09 <br />Minimum: 0 <br />Optional: \{\} <br /> |
| `maxPendingRequests` _integer_ | MaxPendingRequests is the maximum number of pending requests that Envoy will queue to the cluster.<br />If not specified, the default is 1024. |  | Maximum: 4.294967295e+09 <br />Minimum: 0 <br />Optional: \{\} <br /> |
| `maxParallelRequests` _integer_ | MaxParallelRequests is the maximum number of parallel requests that Envoy will make to the cluster.<br />This is applicable to HTTP/2 and gRPC connections.<br />If not specified, the default is 1024. |  | Maximum: 4.294967295e+09 <br />Minimum: 0 <br />Optional: \{\} <br /> |
| `maxParallelRetries` _integer_ | MaxParallelRetries is the maximum number of parallel retries that Envoy will make to the cluster.<br />If not specified, the default is 3. |  | Maximum: 4.294967295e+09 <br />Minimum: 0 <br />Optional: \{\} <br /> |
| `maxRequestsPerConnection` _integer_ | MaxRequestsPerConnection is the maximum number of requests that Envoy will make over a single connection<br />to the cluster. If not specified, there is no limit. |  | Maximum: 4.294967295e+09 <br />Minimum: 0 <br />Optional: \{\} <br /> |
| `perEndpoint` _[PerEndpointCircuitBreaker](#perendpointcircuitbreaker)_ | PerEndpoint configures circuit breaker thresholds that apply to individual endpoints rather than the whole cluster. |  | Optional: \{\} <br /> |




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
| `wildcardDomain` _string_ | WildcardDomain is the domain that will be used as the base domain to create wildcard DNS records for DNS resources.<br />This is only used for determining the hostname for LoadBalancer and Tunnel resources. |  | Optional: \{\} <br /> |
| `allowExplicitHostnames` _boolean_ | AllowExplicitHostnames is a flag that can be used to allow explicit hostnames to be used for DNS resources.<br />This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set. |  | Optional: \{\} <br /> |
| `useDNSAnnotations` _boolean_ | UseDNSAnnotations is a flag that can be used to add DNS annotations to DNS resources.<br />This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set. |  | Optional: \{\} <br /> |
| `useCertificateAnnotations` _boolean_ | UseCertificateAnnotations is a flag that can be used to add Certificate annotations to Certificate resources.<br />This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set. |  | Optional: \{\} <br /> |


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
| `propagatedAnnotations` _map[string]string_ | PropagatedAnnotations defines the set of annotation key patterns that will be propagated to load balancing resources.<br />Keys support shell-style glob patterns (e.g. "nginx.ingress.kubernetes.io/*"). Keep the value empty to allow any value;<br />otherwise the value is a comma-separated list of permitted values for exact match.<br />Tenant configuration has higher precedence than the annotations specified at the Config level. |  | Optional: \{\} <br /> |
| `propagateAllAnnotations` _boolean_ | PropagateAllAnnotations defines whether all annotations will be propagated to load balancing resources.<br />If set to true, PropagatedAnnotations is ignored. DeniedAnnotations still applies on top of this flag.<br />Tenant configuration has higher precedence than the value specified at the Config level. |  | Optional: \{\} <br /> |
| `deniedAnnotations` _string array_ | DeniedAnnotations is a list of annotation key patterns that are excluded from propagation, regardless of<br />PropagateAllAnnotations or PropagatedAnnotations. Patterns support shell-style globbing (e.g. "nginx.ingress.kubernetes.io/*").<br />Tenant configuration has higher precedence than the value specified at the Config level. |  | Optional: \{\} <br /> |
| `defaultAnnotations` _object (keys:[AnnotatedResource](#annotatedresource), values:[Annotations](#annotations))_ | DefaultAnnotations defines the list of annotations(key-value pairs) that will be set on the load balancing resources if not already present. A special key `all` can be used to apply the same<br />set of annotations to all resources.<br />Tenant configuration has higher precedence than the annotations specified at the Config level. |  | Optional: \{\} <br /> |
| `envoyProxy` _[EnvoyProxy](#envoyproxy)_ | EnvoyProxy defines the desired state of the Envoy Proxy |  |  |
| `backendTransport` _[BackendTransport](#backendtransport)_ | BackendTransport controls how management Envoy connects to tenant backends.<br />Defaults to Direct for backward compatibility. |  | Optional: \{\} <br /> |
| `loadBalancer` _[LoadBalancerSettings](#loadbalancersettings)_ |  |  |  |
| `ingress` _[IngressSettings](#ingresssettings)_ |  |  |  |
| `gatewayAPI` _[GatewayAPISettings](#gatewayapisettings)_ |  |  |  |
| `dns` _[ConfigDNSSettings](#configdnssettings)_ |  |  |  |
| `certificates` _[ConfigCertificatesSettings](#configcertificatessettings)_ |  |  |  |
| `tunnel` _[TunnelSettings](#tunnelsettings)_ |  |  |  |
| `circuitBreaker` _[CircuitBreaker](#circuitbreaker)_ | CircuitBreaker defines the default circuit breaker configuration for all Envoy clusters.<br />These settings can be overridden at the Tenant level. |  | Optional: \{\} <br /> |
| `timeouts` _[EnvoyTimeouts](#envoytimeouts)_ | Timeouts defines default Envoy timeouts applied to all routes and<br />load balancers in this cluster. Tenant and Route/LoadBalancer<br />settings override these defaults per-field. |  | Optional: \{\} <br /> |
| `loadBalancerPolicy` _[LoadBalancerPolicy](#loadbalancerpolicy)_ | LoadBalancerPolicy defines the default load balancing policy for all Envoy clusters.<br />These settings can be overridden at the Tenant and LoadBalancer/Route level. |  | Enum: [RoundRobin LeastRequest Random] <br />Optional: \{\} <br /> |
| `healthCheck` _[HealthCheck](#healthcheck)_ | HealthCheck defines the default active health check for all Envoy clusters.<br />Whole-struct override: Tenant and LoadBalancer/Route settings replace this<br />entirely rather than merging per-field. |  | Optional: \{\} <br /> |
| `waf` _[WAFSettings](#wafsettings)_ | WAF defines WAF-related settings. |  | Optional: \{\} <br /> |
| `prometheus` _[PrometheusSettings](#prometheussettings)_ | Prometheus, when set, gives the manager a Prometheus query endpoint to<br />read metrics from. Optional and bring-your-own: KubeLB does not run a<br />Prometheus. |  | Optional: \{\} <br /> |
| `networkPolicy` _[NetworkPolicySettings](#networkpolicysettings)_ | NetworkPolicy defines the default network policy settings for all tenant namespaces.<br />Tenant has higher precedence than the settings specified at the Config level. |  | Optional: \{\} <br /> |
| `insights` _[InsightsSettings](#insightssettings)_ | Insights defines settings for the KubeLB insights engine. It only takes<br />effect when the manager runs with --enable-insights. |  | Optional: \{\} <br /> |


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
| `wildcardDomain` _string_ | WildcardDomain is the domain that will be used as the base domain to create wildcard DNS records for DNS resources.<br />This is only used for determining the hostname for LoadBalancer and Tunnel resources. |  | Optional: \{\} <br /> |
| `allowExplicitHostnames` _boolean_ | AllowExplicitHostnames is a flag that can be used to allow explicit hostnames to be used for DNS resources.<br />This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set. |  | Optional: \{\} <br /> |
| `useDNSAnnotations` _boolean_ | UseDNSAnnotations is a flag that can be used to add DNS annotations to DNS resources.<br />This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set. |  | Optional: \{\} <br /> |
| `useCertificateAnnotations` _boolean_ | UseCertificateAnnotations is a flag that can be used to add Certificate annotations to Certificate resources.<br />This is only used when LoadBalancer.Spec.Hostname or Tunnel.Spec.Hostname is set. |  | Optional: \{\} <br /> |


#### EndpointAddress



EndpointAddress is a tuple that describes a single endpoint address. At least
one of IP or Hostname must be set.



_Appears in:_
- [AddressesSpec](#addressesspec)
- [LoadBalancerEndpoints](#loadbalancerendpoints)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ip` _string_ | The IP of the endpoint. This can be an IPv4 or IPv6 address.<br />The IP address must not be IP CIDR, Loopback (127.0.0.0/8), link-local (169.254.0.0/16), or link-local multicast ((224.0.0.0/24) addresses. |  | Optional: \{\} <br /> |
| `hostname` _string_ | The Hostname of this endpoint. Used when the backend has no stable IP and<br />must be resolved by DNS. If both ip and hostname are set, ip wins. |  | Optional: \{\} <br /> |


#### EndpointPort



EndpointPort is a tuple that describes a single port.



_Appears in:_
- [LoadBalancerEndpoints](#loadbalancerendpoints)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The name of this port.  This must match the 'name' field in the<br />corresponding ServicePort.<br />Must be a DNS_LABEL.<br />Optional only if one port is defined. |  | Optional: \{\} <br /> |
| `port` _integer_ | The port number of the endpoint. |  |  |
| `protocol` _[Protocol](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#protocol-v1-core)_ | The IP protocol for this port. Defaults to "TCP". |  | Enum: [TCP UDP] <br /> |


#### EnvoyProxy



EnvoyProxy defines the desired state of the EnvoyProxy



_Appears in:_
- [ConfigSpec](#configspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `topology` _[EnvoyProxyTopology](#envoyproxytopology)_ | Topology defines the deployment topology for Envoy Proxy. The only supported value is: shared.<br />DEPRECATION NOTICE: The values "dedicated" and "global" are deprecated and will be removed in a future release. They will now default to shared topology. | shared | Enum: [shared dedicated global] <br />Optional: \{\} <br /> |
| `useDaemonset` _boolean_ | UseDaemonset defines whether Envoy Proxy will run as daemonset. By default, Envoy Proxy will run as deployment.<br />If set to true, Replicas will be ignored. |  | Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas defines the number of replicas for Envoy Proxy. This field is ignored if UseDaemonset is set to true. | 3 | Minimum: 1 <br />Optional: \{\} <br /> |
| `singlePodPerNode` _boolean_ | SinglePodPerNode defines whether Envoy Proxy pods will be spread across nodes. This ensures that multiple replicas are not running on the same node. |  | Optional: \{\} <br /> |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector is used to select nodes to run Envoy Proxy. If specified, the node must have all the indicated labels. |  | Optional: \{\} <br /> |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#toleration-v1-core) array_ | Tolerations is used to schedule Envoy Proxy pods on nodes with matching taints. |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#resourcerequirements-v1-core)_ | Resources defines the resource requirements for Envoy Proxy. |  | Optional: \{\} <br /> |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#affinity-v1-core)_ | Affinity is used to schedule Envoy Proxy pods on nodes with matching affinity. |  | Optional: \{\} <br /> |
| `image` _string_ | Image defines the Envoy Proxy image to use. |  | Optional: \{\} <br /> |
| `gracefulShutdown` _[EnvoyProxyGracefulShutdown](#envoyproxygracefulshutdown)_ | GracefulShutdown defines the graceful shutdown configuration for Envoy Proxy. |  | Optional: \{\} <br /> |
| `overloadManager` _[EnvoyProxyOverloadManager](#envoyproxyoverloadmanager)_ | OverloadManager defines the overload manager configuration for Envoy XDS bootstrap. |  | Optional: \{\} <br /> |
| `maxEndpointsPerCluster` _integer_ | MaxEndpointsPerCluster limits the number of upstream endpoint addresses per Envoy cluster.<br />When set to a positive value, only the first N endpoints are included in the xDS as upstream addresses.<br />Defaults to 0, which means no limit. |  | Optional: \{\} <br /> |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#localobjectreference-v1-core) array_ | ImagePullSecrets is a list of references to secrets in the same namespace to use for pulling the Envoy Proxy image.<br />If not set, imagePullSecrets are auto-detected from the manager pod. |  | Optional: \{\} <br /> |
| `podMonitor` _[EnvoyProxyPodMonitor](#envoyproxypodmonitor)_ | PodMonitor enables creation of PodMonitor resources for Envoy Proxy pods<br />to enable metrics scraping by Prometheus Operator. |  | Optional: \{\} <br /> |
| `headerLimits` _[EnvoyProxyHeaderLimits](#envoyproxyheaderlimits)_ | HeaderLimits configures the client header size and count limits for the<br />KubeLB-managed Envoy Proxy. Unset fields default to Envoy's maximum so the<br />managed proxy never rejects headers that the edge proxy already accepted. |  | Optional: \{\} <br /> |


#### EnvoyProxyGracefulShutdown



EnvoyProxyGracefulShutdown defines the graceful shutdown configuration for Envoy Proxy



_Appears in:_
- [EnvoyProxy](#envoyproxy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disabled` _boolean_ | Disabled controls whether graceful shutdown is disabled |  | Optional: \{\} <br /> |
| `drainTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#duration-v1-meta)_ | DrainTimeout is the maximum time to wait for connections to drain.<br />Defaults to 60s. Must be less than TerminationGracePeriodSeconds. | 60s | Optional: \{\} <br /> |
| `minDrainDuration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#duration-v1-meta)_ | MinDrainDuration is the minimum time to wait before checking connection count.<br />This prevents premature termination. Defaults to 5s. | 5s | Optional: \{\} <br /> |
| `terminationGracePeriodSeconds` _integer_ | TerminationGracePeriodSeconds is the grace period for pod termination.<br />Must be greater than DrainTimeout. Defaults to 300s. | 300 | Minimum: 30 <br />Optional: \{\} <br /> |
| `shutdownManagerImage` _string_ | ShutdownManagerImage is the Docker image for the shutdown-manager sidecar.<br />Defaults to "docker.io/envoyproxy/gateway:v1.8.3" |  | Optional: \{\} <br /> |


#### EnvoyProxyHeaderLimits



EnvoyProxyHeaderLimits configures the client header size and count limits for
the KubeLB-managed Envoy Proxy. Envoy rejects requests whose headers exceed
its 60 KiB default with HTTP 431; these fields raise that ceiling.



_Appears in:_
- [EnvoyProxy](#envoyproxy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxRequestHeadersKb` _integer_ | MaxRequestHeadersKb is the maximum request header block size in KiB.<br />Envoy's default is 60; defaults to 8192 (Envoy's maximum) when unset. |  | Maximum: 8192 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `maxRequestHeadersCount` _integer_ | MaxRequestHeadersCount is the maximum number of request headers.<br />Envoy's default is 100; defaults to 4096 when unset. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `maxResponseHeadersKb` _integer_ | MaxResponseHeadersKb is the maximum upstream response header block size in KiB.<br />Envoy's default is 60; defaults to 8192 (Envoy's maximum) when unset. |  | Maximum: 8192 <br />Minimum: 1 <br />Optional: \{\} <br /> |


#### EnvoyProxyOverloadManager



EnvoyProxyOverloadManager defines the overload manager configuration for Envoy XDS



_Appears in:_
- [EnvoyProxy](#envoyproxy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether overload manager is enabled |  | Optional: \{\} <br /> |
| `maxActiveDownstreamConnections` _integer_ | MaxActiveDownstreamConnections is the maximum number of active downstream connections for the Envoy. |  | Optional: \{\} <br /> |
| `maxHeapSizeBytes` _integer_ | MaxHeapSizeBytes is the maximum heap size for the Envoy in bytes. On reaching the limit, the Envoy will start to reject new connections. |  | Optional: \{\} <br /> |


#### EnvoyProxyPodMonitor



EnvoyProxyPodMonitor defines the PodMonitor configuration for Envoy Proxy



_Appears in:_
- [EnvoyProxy](#envoyproxy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether a PodMonitor is created for Envoy Proxy pods. |  | Optional: \{\} <br /> |


#### EnvoyProxyTopology

_Underlying type:_ _string_





_Appears in:_
- [EnvoyProxy](#envoyproxy)

| Field | Description |
| --- | --- |
| `shared` |  |
| `dedicated` |  |
| `global` |  |


#### EnvoyTimeouts



EnvoyTimeouts configures upstream and connection timeouts on the
KubeLB-managed Envoy proxy. Nil duration fields inherit from the
next tier (Route/LB → Tenant → Config → built-in default). A value
of 0s explicitly disables that timeout (Envoy semantics).



_Appears in:_
- [ConfigSpec](#configspec)
- [LoadBalancerSpec](#loadbalancerspec)
- [RouteSpec](#routespec)
- [TenantSpec](#tenantspec)
- [TenantStateStatus](#tenantstatestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `request` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#duration-v1-meta)_ | Request is the total upstream request timeout for HTTP routes<br />(Envoy route.timeout). Built-in default: 0 (disabled).<br />Applies to: Ingress, HTTPRoute, GRPCRoute. |  | Optional: \{\} <br /> |
| `streamIdle` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#duration-v1-meta)_ | StreamIdle is the maximum time an HTTP stream can be idle without<br />any bytes flowing in either direction (Envoy stream_idle_timeout).<br />Built-in default: 1h.<br />Applies to: Ingress, HTTPRoute, GRPCRoute. |  | Optional: \{\} <br /> |
| `requestHeaders` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#duration-v1-meta)_ | RequestHeaders is the maximum time to receive complete request<br />headers (Envoy request_headers_timeout). Built-in default: 0<br />(disabled). Applies to: Ingress, HTTPRoute, GRPCRoute. |  | Optional: \{\} <br /> |
| `idleConnection` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#duration-v1-meta)_ | IdleConnection is the maximum HTTP connection idle time<br />(Envoy common_http_protocol_options.idle_timeout). Built-in<br />default: 1h. Applies to: Ingress, HTTPRoute, GRPCRoute. |  | Optional: \{\} <br /> |
| `tcpIdle` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#duration-v1-meta)_ | TCPIdle is the TCP proxy idle timeout (Envoy<br />tcp_proxy.idle_timeout). Built-in default: 1h.<br />Applies to: TCPRoute, TLSRoute, L4 LoadBalancer. |  | Optional: \{\} <br /> |
| `connect` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#duration-v1-meta)_ | Connect is the upstream cluster TCP connect timeout<br />(Envoy cluster.connect_timeout). Built-in default: 5s.<br />Applies to: all routes and L4 LoadBalancer. |  | Optional: \{\} <br /> |
| `udpIdle` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#duration-v1-meta)_ | UDPIdle is the UDP session idle timeout. When set, it applies to the<br />management Envoy UDP proxy sessions (Envoy udp_proxy idle_timeout)<br />and, in the MTLS topology, to the CONNECT-UDP tunnel streams on both<br />hops. When unset, the per-hop Envoy defaults apply (60s udp_proxy<br />session idle, 5m tunnel stream idle).<br />Applies to: UDPRoute and L4 LoadBalancer UDP ports. |  | Optional: \{\} <br /> |


#### GRPCHealthCheck



GRPCHealthCheck configures a gRPC active health check (grpc.health.v1.Health).



_Appears in:_
- [HealthCheck](#healthcheck)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceName` _string_ | ServiceName is the value passed as the service name in the gRPC health check<br />request. Empty checks overall server health. Optional. |  | Optional: \{\} <br /> |
| `authority` _string_ | Authority is the value of the :authority header on the gRPC health check<br />request. Defaults to the cluster name (Envoy default) when unset. Optional. |  | Optional: \{\} <br /> |


#### GatewayAPISettings



GatewayAPISettings defines the settings for the gateway API.



_Appears in:_
- [ConfigSpec](#configspec)
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `class` _string_ | Class is the class of the gateway API to use. This can be used to specify a specific gateway API implementation.<br />This has higher precedence than the value specified in the Config. |  | Optional: \{\} <br /> |
| `classMappings` _[GatewayClassMapping](#gatewayclassmapping) array_ | ClassMappings defines gateway class name mappings from tenant clusters to the management cluster.<br />Config mappings are defaults. Tenant mappings override Config mappings with the same source class. |  | MaxItems: 32 <br />Optional: \{\} <br /> |
| `disable` _boolean_ | Disable is a flag that can be used to disable Gateway API for a tenant. |  |  |
| `enforceReferenceGrants` _boolean_ | EnforceReferenceGrants requires a ReferenceGrant in the target namespace<br />for any cross-namespace backendRef (route -> Service) or Gateway TLS<br />certificateRef (Gateway -> Secret) in the tenant cluster. References<br />without a matching grant are dropped and reported via the<br />ResolvedRefs=False/RefNotPermitted condition. The Tenant value overrides<br />the Config value; unset means inherit (Tenant) or disabled (Config). |  | Optional: \{\} <br /> |
| `defaultGateway` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectreference-v1-core)_ | DefaultGateway is the default gateway reference to use for the tenant. This is only used for load balancer hostname and tunneling. |  | Optional: \{\} <br /> |
| `gateway` _[GatewaySettings](#gatewaysettings)_ |  |  |  |
| `disableHTTPRoute` _boolean_ |  |  |  |
| `disableGRPCRoute` _boolean_ |  |  |  |
| `disableTCPRoute` _boolean_ |  |  |  |
| `disableUDPRoute` _boolean_ |  |  |  |
| `disableTLSRoute` _boolean_ |  |  |  |
| `disableBackendTrafficPolicy` _boolean_ |  |  |  |
| `disableClientTrafficPolicy` _boolean_ |  |  |  |


#### GatewayAPIState







_Appears in:_
- [TenantStateStatus](#tenantstatestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `classMappings` _[GatewayClassMapping](#gatewayclassmapping) array_ | ClassMappings defines effective gateway class name mappings from tenant clusters to the management cluster. |  | MaxItems: 32 <br />Optional: \{\} <br /> |
| `enforceReferenceGrants` _boolean_ | EnforceReferenceGrants is the effective (Config default, Tenant override)<br />value of spec.gatewayAPI.enforceReferenceGrants for this tenant. |  | Optional: \{\} <br /> |


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


#### GatewayClassMapping



GatewayClassMapping defines a gateway class mapping from tenant clusters to the management cluster.



_Appears in:_
- [GatewayAPISettings](#gatewayapisettings)
- [GatewayAPIState](#gatewayapistate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `source` _string_ | Source is the gateway class name in the tenant cluster. |  | MaxLength: 253 <br />MinLength: 1 <br /> |
| `target` _string_ | Target is the gateway class name in the management cluster. |  | MaxLength: 253 <br />MinLength: 1 <br /> |


#### GatewaySettings



GatewaySettings defines the settings for the gateway resource.



_Appears in:_
- [GatewayAPISettings](#gatewayapisettings)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `limit` _integer_ | Limit is the maximum number of gateways to create.<br />If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this<br />is that it is not possible for KubeLB to know which resources are safe to remove. |  |  |


#### HTTPHealthCheck



HTTPHealthCheck configures an HTTP/1.1 active health check.



_Appears in:_
- [HealthCheck](#healthcheck)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `path` _string_ | Path is the HTTP request path used for the health check. Defaults to "/". |  | Optional: \{\} <br /> |
| `host` _string_ | Host is the value of the Host/authority header on the health check request.<br />Defaults to the cluster name (Envoy default) when unset. |  | Optional: \{\} <br /> |
| `expectedStatuses` _integer array_ | ExpectedStatuses is the list of HTTP status codes considered healthy.<br />Defaults to [200] when unset. Each value must be in the range 100-599. |  | items:Maximum: 599 <br />items:Minimum: 100 <br />Optional: \{\} <br /> |


#### HealthCheck



HealthCheck configures Envoy active health checking for the upstream clusters
backing this resource. When unset, KubeLB applies a default TCP connect-only
check. This is a whole-struct override: the effective check is taken from the
first tier that sets it (Route/LoadBalancer > Tenant > Config > built-in
default), never merged field-by-field across tiers. Fields left unset within
the chosen tier fall back to the built-in defaults documented below.
For more info: https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/health_checking



_Appears in:_
- [ConfigSpec](#configspec)
- [LoadBalancerSpec](#loadbalancerspec)
- [RouteSpec](#routespec)
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[HealthCheckType](#healthchecktype)_ | Type of health check to perform. Defaults to TCP (connect-only) when unset. |  | Enum: [TCP HTTP GRPC] <br />Optional: \{\} <br /> |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#duration-v1-meta)_ | Interval between health checks. Defaults to 5s. |  | Optional: \{\} <br /> |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#duration-v1-meta)_ | Timeout for each health check attempt. Defaults to 5s. |  | Optional: \{\} <br /> |
| `healthyThreshold` _integer_ | HealthyThreshold is the number of consecutive successful checks before an<br />unhealthy endpoint is marked healthy. Defaults to 2. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `unhealthyThreshold` _integer_ | UnhealthyThreshold is the number of consecutive failed checks before a<br />healthy endpoint is marked unhealthy. Defaults to 3. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `http` _[HTTPHealthCheck](#httphealthcheck)_ | HTTP configures an HTTP health check. Used only when Type is HTTP. |  | Optional: \{\} <br /> |
| `grpc` _[GRPCHealthCheck](#grpchealthcheck)_ | GRPC configures a gRPC health check. Used only when Type is GRPC. |  | Optional: \{\} <br /> |


#### HealthCheckType

_Underlying type:_ _string_



_Validation:_
- Enum: [TCP HTTP GRPC]

_Appears in:_
- [HealthCheck](#healthcheck)

| Field | Description |
| --- | --- |
| `TCP` |  |
| `HTTP` |  |
| `GRPC` |  |


#### HostnameStatus







_Appears in:_
- [LoadBalancerStatus](#loadbalancerstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `hostname` _string_ | Hostname contains the hostname of the load-balancer. |  | Optional: \{\} <br /> |
| `tlsEnabled` _boolean_ | TLSEnabled is true if certificate is created for the hostname. |  | Optional: \{\} <br /> |
| `dnsRecordCreated` _boolean_ | DNSRecordCreated is true if DNS record is created for the hostname. |  | Optional: \{\} <br /> |


#### IngressSettings



IngressSettings defines the settings for the ingress.



_Appears in:_
- [ConfigSpec](#configspec)
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `class` _string_ | Class is the class of the ingress to use.<br />This has higher precedence than the value specified in the Config. |  | Optional: \{\} <br /> |
| `disable` _boolean_ | Disable is a flag that can be used to disable Ingress for a tenant. |  |  |


#### Insight



Insight is a single finding produced by the KubeLB insights engine: a
configuration or posture problem the management cluster can see and the
operator can act on. Insights are operator-facing; they are not synced to
tenant clusters.



_Appears in:_
- [InsightList](#insightlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `Insight` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[InsightSpec](#insightspec)_ |  |  |  |
| `status` _[InsightStatus](#insightstatus)_ |  |  |  |


#### InsightCategory

_Underlying type:_ _string_

InsightCategory groups findings by the kind of problem they describe.

_Validation:_
- Enum: [security reliability cost hygiene migration]

_Appears in:_
- [InsightSpec](#insightspec)

| Field | Description |
| --- | --- |
| `security` |  |
| `reliability` |  |
| `cost` |  |
| `hygiene` |  |
| `migration` |  |


#### InsightDismissalReason

_Underlying type:_ _string_

InsightDismissalReason explains why a finding was dismissed. It is required
on dismissal so the fleet-wide dismissal mix stays analysable.

_Validation:_
- Enum: [working_as_intended accepted_risk false_positive low_priority other]

_Appears in:_
- [InsightTriage](#insighttriage)

| Field | Description |
| --- | --- |
| `working_as_intended` |  |
| `accepted_risk` |  |
| `false_positive` |  |
| `low_priority` |  |
| `other` |  |


#### InsightEvidence



InsightEvidence is a pointer into live cluster state that supports the
finding. Evidence is always a reference, never a copy, so an Insight cannot
go stale against the object it describes.



_Appears in:_
- [InsightSpec](#insightspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[InsightEvidenceType](#insightevidencetype)_ | Type of reference. |  | Enum: [FieldRef Condition ObjectRef] <br /> |
| `ref` _string_ | Ref is the reference itself, in "<Kind>/<name>#<field or condition>" form. |  | MaxLength: 512 <br />MinLength: 1 <br /> |
| `note` _string_ | Note explains what the reference shows. |  | MaxLength: 512 <br />Optional: \{\} <br /> |


#### InsightEvidenceType

_Underlying type:_ _string_

InsightEvidenceType describes what an evidence entry points at.

_Validation:_
- Enum: [FieldRef Condition ObjectRef]

_Appears in:_
- [InsightEvidence](#insightevidence)

| Field | Description |
| --- | --- |
| `FieldRef` | InsightEvidenceFieldRef points at a field on an object, e.g.<br />"Config/default#spec.waf.skipValidation".<br /> |
| `Condition` | InsightEvidenceCondition points at a status condition, e.g.<br />"TenantState/default#BackendTransportChangePending".<br /> |
| `ObjectRef` | InsightEvidenceObjectRef points at a whole object.<br /> |


#### InsightList



InsightList contains a list of Insight.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `InsightList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Insight](#insight) array_ |  |  |  |


#### InsightRemediation



InsightRemediation describes how to resolve a finding. KubeLB never applies
it: the snippet is documentation, not an action.



_Appears in:_
- [InsightSpec](#insightspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `summary` _string_ | Summary is the one-line fix. |  | MaxLength: 1024 <br />Optional: \{\} <br /> |
| `snippet` _string_ | Snippet is an optional YAML example of the fix. It is text only and is<br />never applied by KubeLB. |  | MaxLength: 8192 <br />Optional: \{\} <br /> |


#### InsightSeverity

_Underlying type:_ _string_

InsightSeverity is how much the finding matters. The values match the
OpenReports severity enum so findings can be mirrored into Report objects
without a translation table.

_Validation:_
- Enum: [critical high medium low info]

_Appears in:_
- [InsightSpec](#insightspec)

| Field | Description |
| --- | --- |
| `critical` |  |
| `high` |  |
| `medium` |  |
| `low` |  |
| `info` |  |


#### InsightSpec



InsightSpec is the finding. Everything except triage is written by the
insights engine and is overwritten on every sweep.



_Appears in:_
- [Insight](#insight)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `check` _string_ | Check is the registry ID of the check that produced this finding, e.g.<br />KLB001. It is immutable: a check ID is a permanent contract that docs,<br />dashboards and suppression lists reference. |  | Pattern: `^KLB[0-9]\{3\}$` <br /> |
| `slug` _string_ | Slug is the human-readable name of the check, e.g. waf-detection-only. |  | MaxLength: 63 <br /> |
| `category` _[InsightCategory](#insightcategory)_ | Category groups the finding. |  | Enum: [security reliability cost hygiene migration] <br /> |
| `severity` _[InsightSeverity](#insightseverity)_ | Severity is how much the finding matters. |  | Enum: [critical high medium low info] <br /> |
| `message` _string_ | Message describes this specific finding, including any fleet-relative<br />context ("4 of 6 tenants with public routes enforce WAF"). |  | MaxLength: 1024 <br /> |
| `targetRefs` _[InsightTargetRef](#insighttargetref) array_ | TargetRefs are the objects the finding is about. |  | MaxItems: 32 <br />MinItems: 1 <br /> |
| `evidence` _[InsightEvidence](#insightevidence) array_ | Evidence points at the live state that produced the finding. |  | MaxItems: 16 <br />Optional: \{\} <br /> |
| `remediation` _[InsightRemediation](#insightremediation)_ | Remediation describes how to fix the finding. |  | Optional: \{\} <br /> |
| `docsURL` _string_ | DocsURL links to the check's documentation. |  | MaxLength: 512 <br />Optional: \{\} <br /> |
| `triage` _[InsightTriage](#insighttriage)_ | Triage is the operator's verdict. It is the only user-owned field on this<br />object: the engine reads it and never writes it. |  | Optional: \{\} <br /> |


#### InsightState

_Underlying type:_ _string_

InsightState is the effective state of a finding, computed by the engine from
the detection result and the operator's triage.

_Validation:_
- Enum: [Open Acknowledged Snoozed Dismissed Fixed]

_Appears in:_
- [InsightStatus](#insightstatus)

| Field | Description |
| --- | --- |
| `Open` |  |
| `Acknowledged` |  |
| `Snoozed` |  |
| `Dismissed` |  |
| `Fixed` | InsightStateFixed means the engine no longer detects the finding. It is<br />machine-observed, never set by an operator.<br /> |


#### InsightStatus



InsightStatus is the engine-computed effective state of a finding.



_Appears in:_
- [Insight](#insight)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[InsightState](#insightstate)_ | State combines the detection result with the operator's triage. |  | Enum: [Open Acknowledged Snoozed Dismissed Fixed] <br />Optional: \{\} <br /> |
| `firstSeen` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#time-v1-meta)_ | FirstSeen is when the finding was first detected. It survives a<br />fix-and-reappear cycle so flapping stays visible. |  | Optional: \{\} <br /> |
| `lastEvaluated` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#time-v1-meta)_ | LastEvaluated is the last sweep that considered this finding. |  | Optional: \{\} <br /> |
| `fixedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#time-v1-meta)_ | FixedAt is when the engine stopped detecting the finding. Fixed insights<br />are deleted after a retention period. |  | Optional: \{\} <br /> |


#### InsightTargetRef



InsightTargetRef identifies an object the finding is about.



_Appears in:_
- [InsightSpec](#insightspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | APIVersion of the target. |  | MaxLength: 253 <br />MinLength: 1 <br /> |
| `name` _string_ | Name of the target. |  | MaxLength: 253 <br />MinLength: 1 <br /> |
| `namespace` _string_ | Namespace of the target. Empty for cluster-scoped objects. |  | MaxLength: 253 <br />Optional: \{\} <br /> |


#### InsightTriage



InsightTriage is the operator's verdict on a finding. It is the only part of
an Insight that users write; the engine preserves it verbatim across sweeps.



_Appears in:_
- [InsightSpec](#insightspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[InsightTriageState](#insighttriagestate)_ | State is the verdict. |  | Enum: [Acknowledged Snoozed Dismissed] <br /> |
| `reason` _[InsightDismissalReason](#insightdismissalreason)_ | Reason explains a dismissal. Required when state is Dismissed, forbidden<br />otherwise. |  | Enum: [working_as_intended accepted_risk false_positive low_priority other] <br />Optional: \{\} <br /> |
| `snoozeUntil` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#time-v1-meta)_ | SnoozeUntil is when the finding reopens. Required when state is Snoozed,<br />forbidden otherwise. |  | Optional: \{\} <br /> |


#### InsightTriageState

_Underlying type:_ _string_

InsightTriageState is the operator's verdict on a finding.

_Validation:_
- Enum: [Acknowledged Snoozed Dismissed]

_Appears in:_
- [InsightTriage](#insighttriage)

| Field | Description |
| --- | --- |
| `Acknowledged` | InsightTriageAcknowledged means the finding is seen and accepted as work<br />to do. It keeps counting towards the posture score.<br /> |
| `Snoozed` | InsightTriageSnoozed hides the finding until snoozeUntil passes, after<br />which it reopens on its own.<br /> |
| `Dismissed` | InsightTriageDismissed closes the finding for good. A dismissed finding<br />that is detected again stays dismissed.<br /> |


#### InsightsSettings



InsightsSettings defines the global settings for the insights engine.



_Appears in:_
- [ConfigSpec](#configspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disabledChecks` _string array_ | DisabledChecks lists check IDs the engine must not run, for example<br />["KLB010"]. Existing findings for a disabled check are removed on the<br />next sweep. |  | MaxItems: 64 <br />items:Pattern: `^KLB[0-9]\{3\}$` <br />Optional: \{\} <br /> |


#### KubernetesSource







_Appears in:_
- [RouteSource](#routesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resource` _[Unstructured](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#unstructured-unstructured-v1)_ |  |  | EmbeddedResource: \{\} <br />Optional: \{\} <br /> |
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
| `addressesReference` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectreference-v1-core)_ | AddressesReference is a reference to the Addresses object that contains the IP addresses.<br />If this field is set, the Addresses field will be ignored. |  | Optional: \{\} <br /> |
| `ports` _[EndpointPort](#endpointport) array_ | Port numbers available on the related IP addresses.<br />This field is ignored for routes that are using kubernetes resources as the source. |  | MinItems: 1 <br />Optional: \{\} <br /> |


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


#### LoadBalancerPolicy

_Underlying type:_ _string_



_Validation:_
- Enum: [RoundRobin LeastRequest Random]

_Appears in:_
- [ConfigSpec](#configspec)
- [LoadBalancerSpec](#loadbalancerspec)
- [RouteSpec](#routespec)
- [TenantSpec](#tenantspec)

| Field | Description |
| --- | --- |
| `RoundRobin` |  |
| `LeastRequest` |  |
| `Random` |  |


#### LoadBalancerPort



LoadBalancerPort contains information on service's port.



_Appears in:_
- [LoadBalancerSpec](#loadbalancerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The name of this port within the service. This must be a DNS_LABEL.<br />All ports within a Spec must have unique names. When considering<br />the endpoints for a Service, this must match the 'name' field in the<br />EndpointPort.<br />Optional if only one ServicePort is defined on this service. |  | Optional: \{\} <br /> |
| `protocol` _[Protocol](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#protocol-v1-core)_ | The IP protocol for this port. Defaults to "TCP". |  | Enum: [TCP UDP] <br /> |
| `port` _integer_ | The port that will be exposed by the LoadBalancer. |  |  |


#### LoadBalancerSettings



LoadBalancerSettings defines the settings for the load balancers.



_Appears in:_
- [ConfigSpec](#configspec)
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `class` _string_ | Class is the class of the load balancer to use.<br />This has higher precedence than the value specified in the Config. |  | Optional: \{\} <br /> |
| `limit` _integer_ | Limit is the maximum number of load balancers to create.<br />If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this<br />is that it is not possible for KubeLB to know which resources are safe to remove. |  |  |
| `disable` _boolean_ | Disable is a flag that can be used to disable L4 load balancing for a tenant. |  |  |


#### LoadBalancerSpec



LoadBalancerSpec defines the desired state of LoadBalancer



_Appears in:_
- [LoadBalancer](#loadbalancer)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoints` _[LoadBalancerEndpoints](#loadbalancerendpoints) array_ | Sets of addresses and ports that comprise an exposed user service on a cluster. |  | MinItems: 1 <br />Required: \{\} <br /> |
| `ports` _[LoadBalancerPort](#loadbalancerport) array_ | The list of ports that are exposed by the load balancer service.<br />only needed for layer 4 |  | Optional: \{\} <br /> |
| `hostname` _string_ | Hostname is the domain name at which the load balancer service will be accessible.<br />When hostname is set, KubeLB will create a route(ingress or httproute) for the service, and expose it with TLS on the given hostname. |  | Optional: \{\} <br /> |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#servicetype-v1-core)_ | type determines how the Service is exposed. Defaults to ClusterIP. Valid<br />options are ExternalName, ClusterIP, NodePort, and LoadBalancer.<br />"ExternalName" maps to the specified externalName.<br />"ClusterIP" allocates a cluster-internal IP address for load-balancing to<br />endpoints. Endpoints are determined by the selector or if that is not<br />specified, by manual construction of an Endpoints object. If clusterIP is<br />"None", no virtual IP is allocated and the endpoints are published as a<br />set of endpoints rather than a stable IP.<br />"NodePort" builds on ClusterIP and allocates a port on every node which<br />routes to the clusterIP.<br />"LoadBalancer" builds on NodePort and creates an<br />external load-balancer (if supported in the current cloud) which routes<br />to the clusterIP.<br />More info: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types | ClusterIP | Optional: \{\} <br /> |
| `externalTrafficPolicy` _[ServiceExternalTrafficPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#serviceexternaltrafficpolicy-v1-core)_ | externalTrafficPolicy denotes if this Service desires to route external traffic to<br />node-local or cluster-wide endpoints. "Local" preserves the client source IP and avoids<br />a second hop for LoadBalancer and Nodeport type services, but risks potentially imbalanced<br />traffic spreading. "Cluster" obscures the client source IP and may cause a second hop to<br />another node, but should have good overall load-spreading. |  | Optional: \{\} <br /> |
| `persistence` _[LoadBalancerPersistence](#loadbalancerpersistence)_ | Persistence configures backend endpoint persistence. When omitted,<br />KubeLB keeps the default non-sticky load balancing behavior.<br />SourceIP persistence is based on the source IP observed by KubeLB Envoy<br />for TCP and UDP traffic, which may be a gateway, node, or NAT address in<br />proxied topologies.<br />Takes precedence over LoadBalancerPolicy, which cannot be honoured at the<br />same time: persistence is a correctness requirement the workload states,<br />a distribution policy is a preference. |  | Optional: \{\} <br /> |
| `loadBalancerPolicy` _[LoadBalancerPolicy](#loadbalancerpolicy)_ | LoadBalancerPolicy defines the load balancing policy for this LoadBalancer's Envoy cluster.<br />Overrides Tenant and Config-level settings. |  | Enum: [RoundRobin LeastRequest Random] <br />Optional: \{\} <br /> |
| `timeouts` _[EnvoyTimeouts](#envoytimeouts)_ | Timeouts defines per-LoadBalancer Envoy timeouts. Overrides<br />Tenant and Config timeouts per-field. |  | Optional: \{\} <br /> |
| `healthCheck` _[HealthCheck](#healthcheck)_ | HealthCheck defines the active health check for this LoadBalancer's Envoy cluster.<br />Whole-struct override: replaces Tenant and Config-level checks entirely. |  | Optional: \{\} <br /> |
| `upstreamTLS` _[UpstreamTLSConfig](#upstreamtlsconfig)_ | UpstreamTLS configures TLS for connections from KubeLB's Envoy proxy to backend endpoints.<br />When not set, Envoy connects using plain TCP. |  | Optional: \{\} <br /> |


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
| `loadBalancer` _[LoadBalancerStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#loadbalancerstatus-v1-core)_ | LoadBalancer contains the current status of the load-balancer,<br />if one is present. |  | Optional: \{\} <br /> |
| `service` _[ServiceStatus](#servicestatus)_ | Service contains the current status of the LB service. |  | Optional: \{\} <br /> |
| `hostname` _[HostnameStatus](#hostnamestatus)_ | Hostname contains the status for hostname resources. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta) array_ | Conditions describe the LoadBalancer as observed by the KubeLB manager. |  | Optional: \{\} <br /> |


#### NamedNetworkPolicy



NamedNetworkPolicy is a NetworkPolicySpec with an explicit name.



_Appears in:_
- [NetworkPolicySettings](#networkpolicysettings)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the network policy. |  | MinLength: 1 <br /> |
| `spec` _[NetworkPolicySpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#networkpolicyspec-v1-networking)_ | Spec is the NetworkPolicySpec for this policy. |  |  |


#### NetworkPolicySettings



NetworkPolicySettings defines the network policy configuration for tenants.
Default policies:
  - kubelb-deny-all-ingress: Default deny all ingress traffic to tenant namespace
  - kubelb-allow-same-namespace: Allow pod-to-pod traffic within tenant namespace
  - kubelb-allow-manager-ingress: Allow ingress from KubeLB manager namespace
  - kubelb-allow-dns-egress: Allow DNS resolution via kube-system (port 53 UDP/TCP)
  - kubelb-allow-xds-egress: Allow xDS control plane communication to manager (port 8001/TCP)
  - kubelb-allow-metrics-ingress: Allow Prometheus metrics scraping (port 19001/TCP)
  - kubelb-allow-envoy-ingress: Allow all ingress to envoy proxy pods for LoadBalancer traffic
  - kubelb-allow-envoy-egress: Allow all egress from envoy proxy pods to reach tenant NodePorts



_Appears in:_
- [ConfigSpec](#configspec)
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ | Enable to install network policies by default for all tenants.<br />By default(null/false), network policy automation is disabled. This will be enabled by default in a future release. |  | Optional: \{\} <br /> |
| `disabledPolicies` _string array_ | DisabledPolicies is a list of default policy names to skip (e.g. ["kubelb-deny-all-ingress"]). |  | Optional: \{\} <br /> |
| `additionalPolicies` _[NamedNetworkPolicy](#namednetworkpolicy) array_ | AdditionalPolicies are extra named network policies created alongside remaining defaults. |  | Optional: \{\} <br /> |


#### PerEndpointCircuitBreaker



PerEndpointCircuitBreaker defines circuit breaker thresholds that apply to individual endpoints.



_Appears in:_
- [CircuitBreaker](#circuitbreaker)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxConnections` _integer_ | MaxConnections is the maximum number of connections that Envoy will establish to a single endpoint.<br />If not specified, the default is 1024. |  | Maximum: 4.294967295e+09 <br />Minimum: 0 <br />Optional: \{\} <br /> |


#### PrometheusSecretKeyReference



PrometheusSecretKeyReference selects one key from a Secret in the KubeLB
manager namespace.



_Appears in:_
- [PrometheusSettings](#prometheussettings)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Secret. |  |  |
| `key` _string_ | Key within the Secret's data. |  |  |


#### PrometheusSettings



PrometheusSettings configures the Prometheus query endpoint the manager
reads metrics from.



_Appears in:_
- [ConfigSpec](#configspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `url` _string_ | URL is the base URL of the Prometheus query API, for example<br />http://prometheus-operated.monitoring.svc:9090. |  | Pattern: `^https?://.+` <br /> |
| `bearerTokenSecretRef` _[PrometheusSecretKeyReference](#prometheussecretkeyreference)_ | BearerTokenSecretRef reads a bearer token used to authenticate to Prometheus. |  | Optional: \{\} <br /> |
| `caCertSecretRef` _[PrometheusSecretKeyReference](#prometheussecretkeyreference)_ | CACertSecretRef reads a PEM CA bundle used to verify a TLS Prometheus endpoint. |  | Optional: \{\} <br /> |
| `insecureSkipVerify` _boolean_ | InsecureSkipVerify disables TLS certificate verification for the endpoint. |  | Optional: \{\} <br /> |


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
| `endpoints` _[LoadBalancerEndpoints](#loadbalancerendpoints) array_ | Sets of addresses and ports that comprise an exposed user service on a cluster.<br />This field is required for Routes that represent traffic-forwarding resources (Ingress, Gateway routes).<br />It is optional for policy resources like BackendTrafficPolicy. |  | Optional: \{\} <br /> |
| `source` _[RouteSource](#routesource)_ | Source contains the information about the source of the route. This is used when the route is created from external sources. |  | Optional: \{\} <br /> |
| `loadBalancerPolicy` _[LoadBalancerPolicy](#loadbalancerpolicy)_ | LoadBalancerPolicy defines the load balancing policy for this Route's Envoy clusters.<br />Overrides Tenant and Config-level settings. |  | Enum: [RoundRobin LeastRequest Random] <br />Optional: \{\} <br /> |
| `timeouts` _[EnvoyTimeouts](#envoytimeouts)_ | Timeouts defines per-Route Envoy timeouts. Overrides Tenant and<br />Config timeouts per-field. |  | Optional: \{\} <br /> |
| `healthCheck` _[HealthCheck](#healthcheck)_ | HealthCheck defines the active health check for this Route's Envoy clusters.<br />Whole-struct override: replaces Tenant and Config-level checks entirely. |  | Optional: \{\} <br /> |


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
| `name` _string_ | The name of this port within the service. This must be a DNS_LABEL.<br />All ports within a ServiceSpec must have unique names. When considering<br />the endpoints for a Service, this must match the 'name' field in the<br />EndpointPort.<br />Optional if only one ServicePort is defined on this service. |  | Optional: \{\} <br /> |
| `protocol` _[Protocol](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#protocol-v1-core)_ | The IP protocol for this port. Supports "TCP", "UDP", and "SCTP".<br />Default is TCP. | TCP | Optional: \{\} <br /> |
| `appProtocol` _string_ | The application protocol for this port.<br />This is used as a hint for implementations to offer richer behavior for protocols that they understand.<br />This field follows standard Kubernetes label syntax.<br />Valid values are either:<br />* Un-prefixed protocol names - reserved for IANA standard service names (as per<br />RFC-6335 and https://www.iana.org/assignments/service-names).<br />* Kubernetes-defined prefixed names:<br />  * 'kubernetes.io/h2c' - HTTP/2 prior knowledge over cleartext as described in https://www.rfc-editor.org/rfc/rfc9113.html#name-starting-http-2-with-prior-<br />  * 'kubernetes.io/ws'  - WebSocket over cleartext as described in https://www.rfc-editor.org/rfc/rfc6455<br />  * 'kubernetes.io/wss' - WebSocket over TLS as described in https://www.rfc-editor.org/rfc/rfc6455<br />* Other protocols should use implementation-defined prefixed names such as<br />mycompany.com/my-custom-protocol. |  | Optional: \{\} <br /> |
| `port` _integer_ | The port that will be exposed by this service. |  |  |
| `targetPort` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#intorstring-intstr-util)_ | Number or name of the port to access on the pods targeted by the service.<br />Number must be in the range 1 to 65535. Name must be an IANA_SVC_NAME.<br />If this is a string, it will be looked up as a named port in the<br />target Pod's container ports. If this is not specified, the value<br />of the 'port' field is used (an identity map).<br />This field is ignored for services with clusterIP=None, and should be<br />omitted or set equal to the 'port' field.<br />More info: https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service |  | Optional: \{\} <br /> |
| `nodePort` _integer_ | The port on each node on which this service is exposed when type is<br />NodePort or LoadBalancer.  Usually assigned by the system. If a value is<br />specified, in-range, and not in use it will be used, otherwise the<br />operation will fail.  If not specified, a port will be allocated if this<br />Service requires one.  If this field is specified when creating a<br />Service which does not need it, creation will fail. This field will be<br />wiped when updating a Service to no longer need it (e.g. changing type<br />from NodePort to ClusterIP).<br />More info: https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport |  | Optional: \{\} <br /> |
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
| `immutable` _boolean_ |  |  | Optional: \{\} <br /> |
| `data` _object (keys:string, values:integer array)_ |  |  | Optional: \{\} <br /> |
| `stringData` _object (keys:string, values:string)_ |  |  | Optional: \{\} <br /> |
| `type` _[SecretType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#secrettype-v1-core)_ |  |  | Optional: \{\} <br /> |
| `status` _[SyncSecretStatus](#syncsecretstatus)_ |  |  | Optional: \{\} <br /> |


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
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed for this SyncSecret by the controller. |  | Optional: \{\} <br /> |
| `phase` _[SyncSecretPhase](#syncsecretphase)_ | Phase is the current lifecycle phase of the SyncSecret. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta) array_ | Conditions represents the latest available observations of the SyncSecret's state. |  | Optional: \{\} <br /> |


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
| `replicas` _integer_ | Replicas is the number of Envoy Proxy replicas for this tenant.<br />This field is ignored if Config.Spec.EnvoyProxy.UseDaemonset is true. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#resourcerequirements-v1-core)_ | Resources defines the resource requirements for the Envoy Proxy container. |  | Optional: \{\} <br /> |


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


#### TenantProxy



TenantProxy configures the tenant-cluster Envoy proxy for the MTLS
backend transport.



_Appears in:_
- [BackendTransport](#backendtransport)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceType` _[TenantProxyServiceType](#tenantproxyservicetype)_ | ServiceType selects the Service type used to expose the tenant proxy<br />to the management Envoy. With NodePort (default), the CCM publishes<br />node addresses plus the allocated NodePort. With LoadBalancer, the<br />CCM publishes the Service's load balancer ingress IPs/hostnames and<br />the management Envoy dials the fixed tenant proxy port (15443). | NodePort | Enum: [NodePort LoadBalancer] <br />Optional: \{\} <br /> |
| `workload` _[TenantProxyWorkload](#tenantproxyworkload)_ | Workload selects how the tenant proxy pods are scheduled. DaemonSet<br />(default) runs one proxy per node. Deployment runs a fixed number of<br />replicas spread across nodes; the CCM then publishes only the node<br />addresses that host proxy pods so the management Envoy never dials a<br />node without a local proxy. | DaemonSet | Enum: [DaemonSet Deployment] <br />Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of tenant proxy pods when Workload is<br />Deployment. Ignored for DaemonSet. | 2 | Minimum: 1 <br />Optional: \{\} <br /> |


#### TenantProxyServiceType

_Underlying type:_ _string_





_Appears in:_
- [TenantProxy](#tenantproxy)

| Field | Description |
| --- | --- |
| `NodePort` |  |
| `LoadBalancer` |  |


#### TenantProxyWorkload

_Underlying type:_ _string_





_Appears in:_
- [TenantProxy](#tenantproxy)

| Field | Description |
| --- | --- |
| `DaemonSet` |  |
| `Deployment` |  |


#### TenantSpec



TenantSpec defines the desired state of Tenant



_Appears in:_
- [Tenant](#tenant)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `propagatedAnnotations` _map[string]string_ | PropagatedAnnotations defines the set of annotation key patterns that will be propagated to load balancing resources.<br />Keys support shell-style glob patterns (e.g. "nginx.ingress.kubernetes.io/*"). Keep the value empty to allow any value;<br />otherwise the value is a comma-separated list of permitted values for exact match.<br />Tenant configuration has higher precedence than the annotations specified at the Config level. |  | Optional: \{\} <br /> |
| `propagateAllAnnotations` _boolean_ | PropagateAllAnnotations defines whether all annotations will be propagated to load balancing resources.<br />If set to true, PropagatedAnnotations is ignored. DeniedAnnotations still applies on top of this flag.<br />Tenant configuration has higher precedence than the value specified at the Config level. |  | Optional: \{\} <br /> |
| `deniedAnnotations` _string array_ | DeniedAnnotations is a list of annotation key patterns that are excluded from propagation, regardless of<br />PropagateAllAnnotations or PropagatedAnnotations. Patterns support shell-style globbing (e.g. "nginx.ingress.kubernetes.io/*").<br />Tenant configuration has higher precedence than the value specified at the Config level. |  | Optional: \{\} <br /> |
| `defaultAnnotations` _object (keys:[AnnotatedResource](#annotatedresource), values:[Annotations](#annotations))_ | DefaultAnnotations defines the list of annotations(key-value pairs) that will be set on the load balancing resources if not already present. A special key `all` can be used to apply the same<br />set of annotations to all resources.<br />Tenant configuration has higher precedence than the annotations specified at the Config level. |  | Optional: \{\} <br /> |
| `loadBalancer` _[LoadBalancerSettings](#loadbalancersettings)_ |  |  |  |
| `ingress` _[IngressSettings](#ingresssettings)_ |  |  |  |
| `gatewayAPI` _[GatewayAPISettings](#gatewayapisettings)_ |  |  |  |
| `dns` _[DNSSettings](#dnssettings)_ |  |  |  |
| `certificates` _[CertificatesSettings](#certificatessettings)_ |  |  |  |
| `tunnel` _[TenantTunnelSettings](#tenanttunnelsettings)_ |  |  |  |
| `waf` _[TenantWAFSettings](#tenantwafsettings)_ |  |  |  |
| `envoyProxy` _[TenantEnvoyProxy](#tenantenvoyproxy)_ | EnvoyProxy defines tenant-level overrides for Envoy Proxy configuration.<br />Fields set here take precedence over Config.Spec.EnvoyProxy. |  | Optional: \{\} <br /> |
| `circuitBreaker` _[CircuitBreaker](#circuitbreaker)_ | CircuitBreaker defines the circuit breaker configuration for this tenant's Envoy clusters.<br />Overrides Config-level settings. |  | Optional: \{\} <br /> |
| `timeouts` _[EnvoyTimeouts](#envoytimeouts)_ | Timeouts defines tenant-level Envoy timeouts. Overrides Config<br />timeouts per-field. Route/LoadBalancer-level timeouts override<br />these. |  | Optional: \{\} <br /> |
| `networkPolicy` _[NetworkPolicySettings](#networkpolicysettings)_ | NetworkPolicy defines network policy settings for this tenant's namespace.<br />Tenant has higher precedence than the settings specified at the Config level. |  | Optional: \{\} <br /> |
| `loadBalancerPolicy` _[LoadBalancerPolicy](#loadbalancerpolicy)_ | LoadBalancerPolicy defines the load balancing policy for this tenant's Envoy clusters.<br />Overrides Config-level settings. |  | Enum: [RoundRobin LeastRequest Random] <br />Optional: \{\} <br /> |
| `healthCheck` _[HealthCheck](#healthcheck)_ | HealthCheck defines the active health check for this tenant's Envoy clusters.<br />Whole-struct override: replaces the Config-level check entirely.<br />LoadBalancer/Route settings override this. |  | Optional: \{\} <br /> |
| `allowedDomains` _string array_ | List of allowed domains for the tenant. This is used to restrict the domains that can be used<br />for the tenant. If specified, applies on all the components such as Ingress, GatewayAPI, DNS, certificates, etc.<br />Examples:<br />- ["*.example.com"] -> this allows subdomains at the root level such as example.com and test.example.com but won't allow domains at one level above like test.test.example.com<br />- ["**.example.com"] -> this allows all subdomains of example.com such as test.dns.example.com and dns.example.com<br />- ["example.com"] -> this allows only example.com<br />- ["**"] or ["*"] -> this allows all domains<br />Note: "**" was added as a special case to allow any levels of subdomains that come before it. "*" works for only 1 level.<br />Default: value is ["**"] and all domains are allowed. | [**] | Optional: \{\} <br /> |


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
| `gatewayAPI` _[GatewayAPIState](#gatewayapistate)_ |  |  |  |
| `backendTransport` _[BackendTransport](#backendtransport)_ |  |  |  |
| `allowedDomains` _string array_ |  |  |  |
| `timeouts` _[EnvoyTimeouts](#envoytimeouts)_ | Timeouts is the tenant-effective Envoy timeout configuration<br />(Tenant overrides merged over Config, per field). Consumed by the<br />tenant-side proxy render, which cannot read Config or Tenant. |  | Optional: \{\} <br /> |


#### TenantStatus



TenantStatus defines the observed state of Tenant



_Appears in:_
- [Tenant](#tenant)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed for this Tenant by the controller. |  | Optional: \{\} <br /> |
| `phase` _[TenantPhase](#tenantphase)_ | Phase is the current lifecycle phase of the Tenant. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta) array_ | Conditions represents the latest available observations of the Tenant's state. |  | Optional: \{\} <br /> |


#### TenantTunnelSettings



TenantTunnelSettings defines the settings for the tunnel.



_Appears in:_
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `limit` _integer_ | Limit is the maximum number of tunnels to create.<br />If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this<br />is that it is not possible for KubeLB to know which resources are safe to remove. |  |  |
| `disable` _boolean_ | Disable is a flag that can be used to disable tunneling for a tenant. |  |  |


#### TenantWAFPolicy



TenantWAFPolicy defines a tenant-authored Web Application Firewall policy for
L7 routes. Unlike the cluster-scoped WAFPolicy, it is namespaced and created
by tenants in their own tenant cluster. It applies to HTTPRoute and GRPCRoute
resources owned by that tenant only.



_Appears in:_
- [TenantWAFPolicyList](#tenantwafpolicylist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `TenantWAFPolicy` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[TenantWAFPolicySpec](#tenantwafpolicyspec)_ |  |  |  |
| `status` _[TenantWAFPolicyStatus](#tenantwafpolicystatus)_ |  |  |  |


#### TenantWAFPolicyList



TenantWAFPolicyList contains a list of TenantWAFPolicy.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubelb.k8c.io/v1alpha1` | | |
| `kind` _string_ | `TenantWAFPolicyList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[TenantWAFPolicy](#tenantwafpolicy) array_ |  |  |  |


#### TenantWAFPolicySpec



TenantWAFPolicySpec defines the desired state of TenantWAFPolicy.
Exactly one targeting method must be used: targetRef, targetSelector, or default.
Setting multiple targeting methods is invalid. Policies without any targeting are ignored.
Feature stage: Beta



_Appears in:_
- [TenantWAFPolicy](#tenantwafpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `default` _boolean_ | Default when set to true applies this policy to all of this tenant's routes.<br />It is the tenant-scoped analogue of WAFPolicy.global and never affects other<br />tenants or global config.<br />Mutually exclusive with TargetRef and TargetSelector.<br />Policies without default, targetRef, or targetSelector are ignored. |  | Optional: \{\} <br /> |
| `targetRef` _[WAFTargetRef](#waftargetref)_ | TargetRef identifies a specific route by name and optionally namespace.<br />For tenant policies, Kind is HTTPRoute or GRPCRoute and<br />namespace/originNamespace refer to the tenant-cluster namespace.<br />Mutually exclusive with Default and TargetSelector. |  | Optional: \{\} <br /> |
| `targetSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#labelselector-v1-meta)_ | TargetSelector selects routes or HTTPRoute/GRPCRoute resources by label.<br />It checks whether the route has the labels or the labels of the HTTPRoute/GRPCRoute resource. In case of a<br />conflict, the labels of the Route resource takes precedence.<br />Mutually exclusive with Default and TargetRef. |  | Optional: \{\} <br /> |
| `directives` _string array_ | Directives contains SecLang/ModSecurity directives passed to Coraza.<br />Reference: https://coraza.io/docs/seclang/directives/<br />Tenant directives are untrusted. They are validated at sync time by<br />SanitizeTenantDirectives, a default-deny allowlist: dangerous directives<br />(SecRemoteRules, filesystem Include, log/path directives, exec/setenv, and<br />ctl actions targeting admin rule IDs) are rejected. The MaxItems/MaxLength<br />caps below are structural CRD limits; an admin can tighten them further at<br />runtime via Config.spec.waf.maxDirectivesPerPolicy and maxDirectiveLength. |  | MaxItems: 64 <br />items:MaxLength: 1024 <br />Optional: \{\} <br /> |
| `failureMode` _[WAFFailureMode](#waffailuremode)_ | FailureMode defines behavior when WAF filter creation fails.<br />- Closed: Block traffic if WAF cannot be applied (default)<br />- Open: Allow traffic without WAF protection<br />Tenants may set this, but an admin enforceFailureMode on Config or Tenant<br />overrides the tenant-chosen value. | Closed | Enum: [Open Closed] <br />Optional: \{\} <br /> |


#### TenantWAFPolicyStatus



TenantWAFPolicyStatus defines the observed state of TenantWAFPolicy.



_Appears in:_
- [TenantWAFPolicy](#tenantwafpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta) array_ | Conditions describe the current state of the TenantWAFPolicy. |  | Optional: \{\} <br /> |


#### TenantWAFSettings



TenantWAFSettings defines the tenant-scoped settings for tenant-authored WAF policies.



_Appears in:_
- [TenantSpec](#tenantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disableTenantPolicies` _boolean_ | DisableTenantPolicies disables tenant-authored WAF policies (TenantWAFPolicy)<br />for this tenant. Admin-authored WAF (WAFPolicy) still applies. |  | Optional: \{\} <br /> |
| `limit` _integer_ | Limit is the maximum number of TenantWAFPolicies for this tenant.<br />If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this<br />is that it is not possible for KubeLB to know which resources are safe to remove.<br />Overrides Config.spec.waf.tenantPolicyLimit; Tenant has higher precedence than Config. |  | Optional: \{\} <br /> |
| `enforceFailureMode` _[WAFFailureMode](#waffailuremode)_ | EnforceFailureMode, when set, overrides the tenant-chosen failureMode on this<br />tenant's TenantWAFPolicies. Takes precedence over the Config-level value. |  | Enum: [Open Closed] <br />Optional: \{\} <br /> |


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
| `serviceName` _string_ | ServiceName is the name of the service created for this tunnel |  | Optional: \{\} <br /> |
| `routeRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectreference-v1-core)_ | RouteRef is a reference to the route (HTTPRoute or Ingress) created for this tunnel |  | Optional: \{\} <br /> |


#### TunnelSettings



TunnelSettings defines the global settings for Tunnel resources.



_Appears in:_
- [ConfigSpec](#configspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `limit` _integer_ | Limit is the maximum number of tunnels to create.<br />If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this<br />is that it is not possible for KubeLB to know which resources are safe to remove. |  |  |
| `connectionManagerURL` _string_ | ConnectionManagerURL is the URL of the connection manager service that handles tunnel connections.<br />This is required if tunneling is enabled.<br />For example: "https://con.example.com" |  | Optional: \{\} <br /> |
| `disable` _boolean_ | Disable indicates whether tunneling feature should be disabled. |  | Optional: \{\} <br /> |


#### TunnelSpec



TunnelSpec defines the desired state of Tunnel



_Appears in:_
- [Tunnel](#tunnel)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `hostname` _string_ | Hostname is the hostname of the tunnel. If not specified, the hostname will be generated by KubeLB. |  | Optional: \{\} <br /> |


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
| `hostname` _string_ | Hostname contains the actual hostname assigned to the tunnel |  | Optional: \{\} <br /> |
| `url` _string_ | URL contains the full URL to access the tunnel |  | Optional: \{\} <br /> |
| `connectionManagerURL` _string_ | ConnectionManagerURL contains the URL that clients should use to establish tunnel connections |  | Optional: \{\} <br /> |
| `phase` _[TunnelPhase](#tunnelphase)_ | Phase represents the current phase of the tunnel |  | Optional: \{\} <br /> |
| `resources` _[TunnelResources](#tunnelresources)_ | Resources contains references to the resources created for this tunnel |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta) array_ | Conditions represents the current conditions of the tunnel |  | Optional: \{\} <br /> |


#### UpstreamService



UpstreamService is a wrapper over the corev1.Service object.
This is required as kubebuilder:validation:EmbeddedResource marker adds the x-kubernetes-embedded-resource to the array instead of
the elements within it. Which results in a broken CRD; validation error. Without this marker, the embedded resource is not properly
serialized to the CRD.



_Appears in:_
- [KubernetesSource](#kubernetessource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[ServiceSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#servicespec-v1-core)_ | Spec defines the behavior of a service.<br />https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |
| `status` _[ServiceStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#servicestatus-v1-core)_ | Most recently observed status of the service.<br />Populated by the system.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### UpstreamTLSConfig



UpstreamTLSConfig configures TLS for connections from KubeLB's Envoy proxy to backend endpoints.
When not set, Envoy connects using plain TCP (no TLS).



_Appears in:_
- [LoadBalancerSpec](#loadbalancerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `policy` _[UpstreamTLSPolicy](#upstreamtlspolicy)_ | Policy defines the upstream TLS verification mode. |  | Enum: [Insecure Verify] <br />Required: \{\} <br /> |
| `caSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#localobjectreference-v1-core)_ | CASecretRef references a Secret containing the CA certificate for backend verification.<br />The Secret must contain a "ca.crt" key. Required when policy is "Verify". |  | Optional: \{\} <br /> |


#### UpstreamTLSPolicy

_Underlying type:_ _string_

UpstreamTLSPolicy defines how KubeLB's Envoy proxy handles TLS to backends.

_Validation:_
- Enum: [Insecure Verify]

_Appears in:_
- [UpstreamTLSConfig](#upstreamtlsconfig)

| Field | Description |
| --- | --- |
| `Insecure` | UpstreamTLSPolicyInsecure enables TLS but skips certificate verification (ACCEPT_UNTRUSTED).<br />Use for self-signed certs, certs without SANs, or expired certs.<br /> |
| `Verify` | UpstreamTLSPolicyVerify enables TLS and verifies the backend certificate against a provided CA.<br /> |


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
- [TenantWAFPolicySpec](#tenantwafpolicyspec)
- [TenantWAFSettings](#tenantwafsettings)
- [WAFPolicySpec](#wafpolicyspec)
- [WAFSettings](#wafsettings)

| Field | Description |
| --- | --- |
| `Open` | WAFFailureModeOpen allows traffic through without WAF protection if filter fails.<br /> |
| `Closed` | WAFFailureModeClosed blocks traffic if WAF filter cannot be applied.<br /> |


#### WAFPolicy



WAFPolicy defines Web Application Firewall policy for L7 routes.
Applies to HTTPRoute and GRPCRoute resources.



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
Feature stage: Beta



_Appears in:_
- [WAFPolicy](#wafpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `global` _boolean_ | Global when set to true applies this policy to all routes for all tenants within a KubeLB installation.<br />Mutually exclusive with TargetRef and TargetSelector.<br />Policies without global, targetRef, or targetSelector are ignored. |  | Optional: \{\} <br /> |
| `targetRef` _[WAFTargetRef](#waftargetref)_ | TargetRef identifies a specific route by name and optionally namespace.<br />Mutually exclusive with Global and TargetSelector. |  | Optional: \{\} <br /> |
| `targetSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#labelselector-v1-meta)_ | TargetSelector selects routes or HTTPRoute/GRPCRoute resources by label.<br />It checks whether the route has the labels or the labels of the HTTPRoute/GRPCRoute resource. In case of a<br />conflict, the labels of the Route resource takes precedence.<br />Mutually exclusive with Global and TargetRef. |  | Optional: \{\} <br /> |
| `directives` _string array_ | Directives contains SecLang/ModSecurity directives passed to Coraza.<br />Reference: https://coraza.io/docs/seclang/directives/<br />If empty, the following OWASP CRS defaults are applied:<br />  - SecRuleEngine On<br />  - SecRequestBodyAccess On<br />  - SecRequestBodyLimit 13107200<br />  - Include @crs-setup-conf<br />  - Include @owasp_crs/*.conf<br />The MaxItems/MaxLength caps below are structural CRD limits mirroring<br />TenantWAFPolicy. They bound a single policy's blast radius; the CRS ruleset<br />itself lives in the WASM binary, not the directive list, so these caps do<br />not limit the protections in effect. |  | MaxItems: 64 <br />items:MaxLength: 1024 <br />Optional: \{\} <br /> |
| `failureMode` _[WAFFailureMode](#waffailuremode)_ | FailureMode defines behavior when WAF filter creation fails.<br />- Closed: Block traffic if WAF cannot be applied (default)<br />- Open: Allow traffic without WAF protection | Closed | Enum: [Open Closed] <br />Optional: \{\} <br /> |


#### WAFPolicyStatus



WAFPolicyStatus defines the observed state of WAFPolicy.



_Appears in:_
- [WAFPolicy](#wafpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta) array_ | Conditions describe the current state of the WAFPolicy. |  | Optional: \{\} <br /> |


#### WAFSettings



WAFSettings defines settings for the WAF (Web Application Firewall).



_Appears in:_
- [ConfigSpec](#configspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `wasmInitContainerImage` _string_ | WASMInitContainerImage overrides the image used for the WASM init container.<br />If empty, defaults to the kubelb-manager image detected at runtime. |  | Optional: \{\} <br /> |
| `skipValidation` _boolean_ | SkipValidation skips directive validation for WAFPolicies.<br />When true, all WAFPolicies are marked as valid without parsing. |  | Optional: \{\} <br /> |
| `enableTenantPolicies` _boolean_ | EnableTenantPolicies is the global opt-in for tenant-authored WAF policies<br />(TenantWAFPolicy). Defaults to false: when unset, TenantWAFPolicies are<br />ignored and their CRD/controller stay inert, so upgrades see zero behavior<br />change until an admin enables the feature. |  | Optional: \{\} <br /> |
| `enforceFailureMode` _[WAFFailureMode](#waffailuremode)_ | EnforceFailureMode, when set, overrides the tenant-chosen failureMode on<br />every TenantWAFPolicy cluster-wide. A per-Tenant EnforceFailureMode takes<br />precedence over this value. |  | Enum: [Open Closed] <br />Optional: \{\} <br /> |
| `tenantPolicyLimit` _integer_ | TenantPolicyLimit is the maximum number of TenantWAFPolicies allowed per tenant.<br />If a lower limit is set than the number of reources that exist, the limit will be disallow creation of new resources but will not delete existing resources. The reason behind this<br />is that it is not possible for KubeLB to know which resources are safe to remove.<br />If nil, the number of TenantWAFPolicies per tenant is unlimited. |  | Optional: \{\} <br /> |
| `maxDirectivesPerPolicy` _integer_ | MaxDirectivesPerPolicy is the runtime cap on the number of directive lines<br />per TenantWAFPolicy enforced by the sanitizer (multi-line directive items<br />are counted per line). Defaults to 64, matching the TenantWAFPolicy CRD item<br />cap. Set to 0 for unlimited. | 64 | Optional: \{\} <br /> |
| `maxDirectiveLength` _integer_ | MaxDirectiveLength is the runtime cap on the length of a single tenant<br />directive line enforced by the sanitizer. Defaults to 1024, matching the<br />TenantWAFPolicy CRD per-item length cap. Set to 0 for unlimited. | 1024 | Optional: \{\} <br /> |


#### WAFTargetRef



WAFTargetRef identifies a route by name.



_Appears in:_
- [TenantWAFPolicySpec](#tenantwafpolicyspec)
- [WAFPolicySpec](#wafpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `group` _string_ | Group is the API group of the target resource. | gateway.networking.k8s.io |  |
| `namespace` _string_ | Namespace is the management cluster namespace (e.g., tenant-primary).<br />If omitted, matches across all namespaces. |  | Optional: \{\} <br /> |
| `originNamespace` _string_ | OriginNamespace is the namespace of the original resource in the tenant<br />cluster (the `kubelb.k8c.io/origin-ns`). Two routes synced into the same<br />management namespace from different tenant-cluster namespaces can share an<br />origin name; set OriginNamespace to disambiguate them. If omitted, origin<br />namespace is not considered during matching. |  | Optional: \{\} <br /> |
| `name` _string_ | Name is the name of the target resource which could either be the name of the resource in management cluster<br />that is generated by KubeLB or the `kubelb.k8c.io/origin-name` that is the original name of the resource in the tenant cluster. |  | MinLength: 1 <br /> |


