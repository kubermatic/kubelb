# KubeLB

KubeLB centrally manages load balancers for Kubernetes clusters across multi-cloud and on-premise environments. It is a hub-and-spoke operator: tenant clusters announce their LB needs to a central manager, which configures Envoy to handle the actual traffic.

## Language

### Topology

**Manager**:
The central control plane. Runs the Envoy xDS server, reconciles LoadBalancer/Route/Tenant CRDs, deploys and configures the Envoy proxies that carry traffic.
_Avoid_: Hub, control plane, server.

**CCM** (Cloud Controller Manager):
The agent that runs in a tenant cluster. Watches Services/Ingresses/HTTPRoutes/GRPCRoutes/Gateways and propagates them as LoadBalancer/Route CRDs to the Manager.
_Avoid_: Agent, sidecar, spoke.

**Tenant**:
A registered consumer cluster, represented as a cluster-scoped `Tenant` CRD on the Manager. Owns a namespace on the Manager where its LoadBalancer/Route CRDs live.
_Avoid_: Customer, workspace, project.

**Tenant cluster** / **Manager cluster**:
The two physical Kubernetes clusters in the topology. Tenant clusters run the CCM; the manager cluster runs the Manager and the Envoy proxies.

**Envoy proxy deployment topology**:
How Envoy proxies are sized and shared. **Shared** = one Envoy per Tenant (default). **Global** = one Envoy across all Tenants. **Dedicated** (deprecated) = one Envoy per LoadBalancer.

### CRDs

**LoadBalancer**:
The Layer 4 contract from CCM to Manager. One per tenant Service of type LoadBalancer.
_Avoid_: Service, LB config.

**Route**:
The Layer 7 contract from CCM to Manager. One per tenant Ingress / HTTPRoute / GRPCRoute. Carries the original resource (as RawExtension) plus the replicated services.
_Avoid_: Ingress config, L7 record.

**Config**:
The Manager's global settings CRD. Provides defaults that Tenants and resources can override.
_Avoid_: Settings, options.

### Reconciliation

**Source kind** / **Source resource**:
A Kubernetes resource the CCM watches and translates into a Route CRD: Ingress, HTTPRoute, GRPCRoute. (A Service of type LoadBalancer is **not** a source kind — it produces a LoadBalancer CRD, not a Route.)
_Avoid_: Input resource, upstream.

**Source-to-Route reconciler**:
The unified CCM reconciler that turns source resources into Route CRDs. Per-kind variation (filter, status type, service extraction) is supplied by a **SourceAdapter**; the reconcile loop is shared.

**SourceAdapter**:
The per-source-kind plug-in to the source-to-Route reconciler. Provides the source object factory, filter, service extraction, and status mapping for one kind.

### Traffic

**xDS** / **Envoy snapshot**:
The runtime configuration the Manager serves to Envoy proxies. Built from the set of LoadBalancer + Route CRDs across all Tenants.

**NodePort hop**:
The egress path from KubeLB Envoy into a tenant cluster. Envoy connects to a tenant node IP on the Service NodePort, which routes to backend pods.

## Relationships

- A **Manager** serves many **Tenants**.
- Each **Tenant** owns a namespace on the **Manager**, holding its **LoadBalancer** and **Route** CRDs.
- A **CCM** in a tenant cluster watches **Source resources** and emits **Routes**, and watches `Service` (type LoadBalancer) and emits **LoadBalancers**.
- The **Source-to-Route reconciler** uses one **SourceAdapter** per **Source kind**.
- The **Manager** consumes **LoadBalancers** + **Routes** to produce the **Envoy snapshot**, served via **xDS**.

## Example dialogue

> **Dev:** "Why does the Ingress controller have nothing to do with envoy?"
>
> **Maintainer:** "Right — the **CCM**'s **Source-to-Route reconciler** only translates Ingress → **Route** CRD. The **Manager** picks up the **Route** and is the one that turns it into Envoy listeners and clusters in the **Envoy snapshot**."
>
> **Dev:** "Is a Service of type LoadBalancer a source kind?"
>
> **Maintainer:** "No. **Source kinds** are L7: Ingress, HTTPRoute, GRPCRoute — they all become **Routes**. A LoadBalancer-type Service is L4 and produces a **LoadBalancer** CRD via a separate reconciler."

## Flagged ambiguities

- "LoadBalancer" can mean either the CRD or the Kubernetes Service type. Prefer **LoadBalancer (CRD)** vs `Service of type LoadBalancer` when context is unclear.
- "Route" can mean either the KubeLB CRD or a Gateway API HTTPRoute/GRPCRoute. The KubeLB CRD is always **Route**; Gateway API resources are spelled out (HTTPRoute, GRPCRoute).
