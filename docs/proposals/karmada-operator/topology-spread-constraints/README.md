---
title: Add Support for Specifying Topology Spread Constraints for Components Provisioned by the Operator
authors:
  - "@jabellard"
reviewers:
  - "@RainbowMango"
  - "@zhzhuang-zju"
approvers:
  - "@RainbowMango"

creation-date: 2026-04-26

---

# Add Support for Specifying Topology Spread Constraints for Components Provisioned by the Operator

## Summary

This proposal adds a `TopologySpreadConstraints` field to the `CommonSettings` struct in the `Karmada` CRD. This allows users to control how control plane component pod replicas are distributed across topology domains (zones, nodes, racks, etc.) using the standard Kubernetes [Topology Spread Constraints](https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/) mechanism.


## Motivation

Not all Karmada control plane components have the same resource profile. `karmada-search` is significantly more resource-hungry than other components. In clusters with small nodes, a common pattern is to add a node pool with larger VMs, label them (e.g., `nodepool: large-vm`), and pin `karmada-search` there via `nodeSelector`. Once pinned, you need to guarantee that the replicas are spread across the nodes in that pool. Without this, both replicas could land on the same node, meaning a single node failure takes down the entire search component.

Today, the only way to influence replica distribution is through `podAntiAffinity` (via the existing `Affinity` field). While anti-affinity can express "don't place two replicas on the same node," there is a critical difference in behavior when replicas exceed node count:

| Scenario | `topologySpreadConstraints` (`maxSkew: 1`) | `podAntiAffinity` (hard) |
|----------|---------------------------------------------|--------------------------|
| 2 replicas, 3 nodes | Different nodes | Different nodes |
| 3 replicas, 3 nodes | One per node | One per node |
| 4 replicas, 3 nodes | Distributes 2-1-1 | **4th replica stays Pending** |
| Node pool scales down to 2 nodes, 3 replicas | Replacement pod schedules (2-1 distribution) | **Replacement pod stays Pending** (no node without a match) |

`podAntiAffinity` enforces a strict one-per-node ceiling. `topologySpreadConstraints` keeps distribution as even as possible while remaining flexible — making it the safer choice for node pools that may grow or shrink over time.

Note that `topologySpreadConstraints` and `podAntiAffinity` are complementary and can be used together. When both are present, Kubernetes evaluates them jointly — a pod must satisfy all constraints to be scheduled. For example, a user could combine a zone-level topology spread constraint (to distribute replicas evenly across zones) with a node-level `podAntiAffinity` (to prevent two replicas on the same node within a zone).

This feature is equally valuable at the zone level. For components like `etcd` deployed with multiple replicas in multi-zone clusters, `topologySpreadConstraints` ensures even distribution so that a single zone failure doesn't take down a quorum.


## Goals

- Allow users to configure `TopologySpreadConstraints` on any Karmada control plane component via the `Karmada` CR.
- Follow the existing patcher pattern used for `Tolerations` and `Affinity`.


## Non-Goals

- Defining default topology spread constraints for any component. Components that currently ship with built-in `podAntiAffinity` rules in their manifest templates will continue to use those as the baseline.
- Merging user-provided constraints with any defaults. If the user specifies `topologySpreadConstraints`, their value is used as-is (consistent with how `Tolerations` and `Affinity` work today).


## Proposal

### User Stories

1. **Spreading a resource-hungry component across nodes in a targeted node pool**
   As a platform engineer, `karmada-search` is too resource-hungry for my cluster's small nodes. I've added a node pool with larger VMs and pinned `karmada-search` there using `nodeSelector`. I need to guarantee that the 2 replicas land on different nodes within that pool so a single node failure doesn't take out the entire search component. I also want a solution that won't break if I later scale to more replicas than there are nodes — `podAntiAffinity` would leave extra replicas stuck in `Pending`, but `topologySpreadConstraints` with `maxSkew: 1` (the maximum difference in pod count between any two topology domains is 1) will distribute them as evenly as possible.

   ```yaml
   apiVersion: operator.karmada.io/v1alpha1
   kind: Karmada
   metadata:
     name: karmada
     namespace: karmada-system
   spec:
     components:
       karmadaSearch:
         replicas: 2
         nodeSelector:
           nodepool: large-vm
         topologySpreadConstraints:
           - maxSkew: 1
             topologyKey: kubernetes.io/hostname
             whenUnsatisfiable: DoNotSchedule
             labelSelector:
               matchLabels:
                 app.kubernetes.io/name: karmada-search
         priorityClassName: system-node-critical
   ```

2. **Even zone distribution for the API server**
   As a platform engineer running Karmada in a 3-zone cluster, I want to ensure that my 3 `karmada-apiserver` replicas are evenly distributed — one per zone — so that a single zone outage leaves 2 of 3 replicas healthy and the API server remains available.

3. **Protecting etcd quorum across zones**
   As a cluster administrator running Karmada with 3 etcd replicas, I want to guarantee that etcd pods are spread across zones so that a single zone failure does not cause quorum loss.

4. **Multi-level spread (zone + node)**
   As a platform engineer, I want to spread replicas evenly across zones (hard constraint) while also making a best-effort attempt to spread them across nodes within each zone (soft constraint), to maximize fault tolerance at both the zone and node level.


### Design Details

#### API Change

Add the following field to `CommonSettings`:

```go
// TopologySpreadConstraints describes how the pods for this component should be
// spread across topology domains (e.g., zones, nodes) to achieve high availability.
// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
// +optional
TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
```

#### Patcher Change

Add a `WithTopologySpreadConstraints` builder method to the patcher and apply it in both `ForDeployment` and `ForStatefulSet`:

```go
func (p *Patcher) WithTopologySpreadConstraints(constraints []corev1.TopologySpreadConstraint) *Patcher {
    p.topologySpreadConstraints = constraints
    return p
}
```

```go
// In ForDeployment and ForStatefulSet:
if len(p.topologySpreadConstraints) > 0 {
    podSpec.TopologySpreadConstraints = p.topologySpreadConstraints
}
```


### Test Plan

- **Unit tests:** Extend patcher unit tests to verify `TopologySpreadConstraints` is correctly applied to both Deployment and StatefulSet pod specs, and that an empty slice results in no change.
- **Integration tests:** Add test cases for at least one Deployment-based component and the etcd StatefulSet to verify end-to-end wiring.
