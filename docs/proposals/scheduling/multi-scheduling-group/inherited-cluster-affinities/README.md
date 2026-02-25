---
title: Inherited Cluster Affinities
authors:
- "@zhzhuang-zju"
- "@vie-serendipity"
reviewers:
- TBD
approvers:
- TBD

creation-date: 2026-01-05

---

# Inherited Cluster Affinities

## Summary

Karmada currently supports declaring a set of candidate clusters through `clusterAffinity`, or multiple sets of candidate clusters through `ClusterAffinities` (which combines multiple `clusterAffinity` terms in a specific order). However, in either approach, each `clusterAffinity` represents an independent, mutually exclusive cluster set during a single scheduling process—the scheduler ultimately selects only one cluster group defined by one `clusterAffinity` or its subset.

This model has limitations in hybrid cloud scenarios (such as coexistence of local data centers and public clouds). In practical use, local clusters typically serve as the preferred resource pool, while public cloud clusters act as extensions or backup resources. The two are not completely independent and mutually exclusive relationships, but should be **automatically used supplementarily based on priority when local resources are insufficient**.

To address this, this proposal extended the Multiple Scheduling Groups feature to support inherited cluster selection across affinity groups. When enabled, subsequent affinity groups automatically inherit cluster sets from previous groups, enabling progressive cluster expansion strategies. This mechanism will enable Karmada to better support workload scheduling in hybrid cloud environments and improve the deployment practicality of online applications in terms of elasticity.

## Motivation

### Goals

- Extend the API of PropagationPolicy to hold inherited cluster affinities declaration.
- Propose the implementation ideas for involved components, including `karmada-controller-manager`, `karmada-webhook` and `karmada-scheduler`.

### Non-Goals

- Cluster utilization is an effective means of controlling cluster resource usage but is not within the scope of this proposal.

## Proposal

### User Stories

#### Story 1
As a user running GPU-based inference workloads in both my own IDC GPU cluster and a cloud GPU cluster, I need to keep GPU costs low so I only want to use cloud GPUs for extra peak traffic instead of baseline load. When traffic changes and FHPA scales my workloads, Karmada should always scale out in order from IDC to cloud and scale in in reverse order from cloud back to IDC, so that my GPU usage remains cost‑efficient.

### Risks and Mitigations

This proposal maintains backward compatibility. It will introduce entirely new APIs, so systems built with previous versions of Karmada can be seamlessly migrated to the new version. Previous configurations (YAMLs) can be applied to the new version of Karmada without any behavior change.

## Design Details

### API change

#### Approach 1:

Extend the `ClusterAffinity` API by adding a `Supplements` field to describe supplementary cluster group configurations. This field allows users to define one or more alternative cluster groups for a single Affinity Group. When the primary cluster group has insufficient resources or is unavailable, the scheduler can automatically fallback to these supplementary cluster groups for workload deployment. Note: Supplements is an array that can set multiple tiers of extensible cluster groups, with scheduling priority decreasing as the tier level increases.

```go
// ClusterAffinity represents the filter to select clusters.
type ClusterAffinity struct {
    // Omitted, as there are no changes.

    // new added API field
    Supplements []ClusterAffinity `json:"supplements,omitempty"`
}
```

The following configuration declares a ClusterAffinity with an extended cluster group:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinity:
      clusterNames:
        - cluster1
      supplements:
        - clusterNames:
          - cluster2
          - cluster3
```

Since `ClusterAffinities` is a combination of multiple `ClusterAffinity` terms, it will also support using the `Supplements` field to declare tiered expansion relationships. In this case, the semantics are:
- Each `ClusterAffinity` within `ClusterAffinities` can declare its own `Supplements` field, representing the extended cluster groups for that specific affinity group.
- The relationship between different `ClusterAffinity` terms in `ClusterAffinities` remains mutually exclusive. The next `ClusterAffinity` will only be considered when both the base clusters and the extended clusters of the previous `ClusterAffinity` fail to satisfy the scheduling requirements.

#### Approach 2:

Similar to Approach 1, but instead of adding the `Supplements` field to the `ClusterAffinity` struct, it is added to the `ClusterAffinityTerm` struct. This approach better aligns with the existing `ClusterAffinities` structure and provides clearer semantics.

```go
// ClusterAffinityTerm selects a set of cluster.
type ClusterAffinityTerm struct {
	// AffinityName is the name of the cluster group.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	// +required
	AffinityName string `json:"affinityName"`

	ClusterAffinity `json:",inline"`

	// new added API field
	Supplements []ClusterAffinity `json:"supplements,omitempty"`
}
```

The following configuration declares a ClusterAffinityTerm with an extended cluster group:
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinities:
      - affinityName: "cost-sensitive"
        clusterNames:
          - cluster1
        supplements:
          - clusterNames:
            - cluster2
            - cluster3
```

#### Approach 3:

Currently, ClusterAffinities have mutually exclusive relationships between ClusterAffinity terms, but they can also have a tiered supplementary relationship.

In this approach, the role of the existing `ClusterAffinities` is modified to simply declare multiple `ClusterAffinity` groups. By introducing `AffinityStrategy.Mode` to the placement API, this new API determines how to utilize the `ClusterAffinity` terms defined in `ClusterAffinities`. This approach enriches the semantics of the original multi-scheduling group feature and improves extensibility.

```go
type Placement struct { 
    ClusterAffinities []ClusterAffinityTerm `json:"clusterAffinities,omitempty"`  
    
    // AffinityStrategy defines how cluster affinities are evaluated  
    // +optional  
    AffinityStrategy *AffinityStrategy `json:"affinityStrategy,omitempty"`  
}  
  
type AffinityStrategy struct {  
    // Mode defines the scheduling mode  
    // +kubebuilder:validation:Enum=Exclusive;Inherited
    // +kubebuilder:default=Exclusive
    // +optional  
    Mode string `json:"mode,omitempty"`  
} 
```

The following configuration declares a inherited cluster affinities between multiple clusterAffinity terms:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinities:
      - affinityName: primary
        clusterNames:
          - cluster1
      - affinityName: backup
        clusterNames:
          - cluster2
          - cluster3
    affinityStrategy:
      mode: Inherited
```

#### Approach 4:

Add a new API `PreferredClusterAffinities` at the same level as ClusterAffinities to declare cluster groups with priority relationships.

```go
// Placement represents the rule for select clusters.
type Placement struct {
    // PreferredClusterAffinities represents scheduling preferences to multiple cluster
    // groups that indicated by ClusterAffinityTerm with priority-based selection.
    //
    // Unlike ClusterAffinities which are mutually exclusive (scheduler selects only one group),
    // PreferredClusterAffinities allows the scheduler to use multiple cluster groups based on
    // priority and resource availability.
    // +optional
    PreferredClusterAffinities []ClusterAffinityTerm `json:"preferredClusterAffinities,omitempty"`
}
```

The following configuration declares a preferredClusterAffinities with two affinity terms:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    preferredClusterAffinities:
      - affinityName: primary
        clusterNames:
          - cluster1
      - affinityName: backup
        clusterNames:
          - cluster2
          - cluster3
```

Regardless of which API is used, during the scheduling process, the scheduler will first try to schedule the workload to member cluster `cluster1` as much as possible. If `cluster1` cannot accommodate all replicas, the remaining replicas will continue to be allocated among clusters `cluster2` and `cluster3`.

If `cluster1` is unavailable, the scheduler will use the clusters in the extended cluster group (i.e., `cluster2` and `cluster3`) for scheduling. Note that when `cluster1` becomes available again, **workload migration will not be automatically triggered**; users need to explicitly use the `WorkloadRebalancer` resource to trigger rescheduling, or indirectly trigger it by adjusting workload replica counts.

Each scheduling will **preferentially use the higher priority affinity group** under current conditions, i.e., the group with the earlier order in ClusterAffinityTerm.

The overall process can be described as the following diagram:

![inherited-cluster-affinities](statics/inherited-cluster-affinity-scheduling.drawio.png)

### Components change

#### karmada-scheduler

The current scheduler only processes one affinity term in each scheduling loop. So, it needs to adjust the scheduling logic to support inherited cluster affinities scheduling.

The details are as follows:
1. The `schedule(Cluster)ResourceBindingWithClusterAffinities` scheduling entrypoint should be adjusted. If inherited cluster affinities scheduling is detected, it should be handled as a     
   single, unified process rather than being split into multiple independent scheduling processes.
2. `Filter` Stage: The `ClusterAffinity` plugin needs to be adjusted to support inherited cluster affinities. Specifically, a cluster passes the filter if it satisfies the scheduling conditions of   
   any affinity group.
3. `Score` Stage: The `ClusterAffinity` plugin needs to be adjusted to support inherited cluster affinities. Specifically, it should assign different base scores to clusters based on the tier of the
   affinity group they belong to (e.g., 5000 for the first tier, 4000 for the second, and so on). This is to ensure that clusters in higher-priority groups are preferred during the       
   subsequent cluster selection phase.
4. `Select` Stage: No changes are needed.
5. `Assign` Stage: This stage needs to be adjusted to support inherited cluster affinities. Specifically:
   5.1 Group clusters based on their affinity group tier.
   5.2 Iterate through the tiers in order, attempting to assign the remaining replicas. Clusters in each tier will be allocated as many replicas as possible until the
   resources for that tier are exhausted or all replicas have been assigned.
   5.3 Within each tier, the assignment logic will remain consistent with the current implementation.
   5.4 The process concludes successfully once all replicas are assigned. If unassigned replicas remain after iterating through all tiers, the scheduling process fails.

### Test Plan

- All current testing should be passed, no break change would be involved by this feature.
- Add new E2E tests to cover the feature, the scope should include:
    * Workload propagating with scheduling type `Duplicated`.
    * Workload propagating with scheduling type `Divided` (including scaling scenarios)

## Discussion Points

**Should multi-cluster priority scheduling support working with `SpreadConstraints`?**

Yes, they are independent APIs.

**How does inherited cluster affinities scheduling behave under different replica scheduling strategies?**

When considering different scheduling strategies, we need to analyze them in combination with the semantics of inherited cluster affinities. The semantics of inherited cluster affinities scheduling itself are to ensure that clusters can be used in order to achieve cost optimization. 
Extended cluster groups are used when the primary cluster group has insufficient resources (or cannot meet scheduling conditions).

| Replica Scheduling Strategy | Question                                                                                                        | Conclusion                                                                                                                                                                                                          |
|-----------------------------|-----------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Duplicated                  | Should distribution occur in all clusters, or only in the first-tier?                                           | Only distribute in first-tier. Because the primary cluster group can meet scheduling requirements                                                                                                                   |
| Aggregated                  | How does it behave?                                                                                             | From the perspective of a certain tier cluster group: this tier will try to fill all remaining replicas with Aggregated strategy until all clusters in this tier are filled or all remaining replicas are allocated |
| Divided/Static              | Is it supported? Fall back to directly splitting replicas among clusters according to configured static weights | Supported, but only distributes in first-tier. Because the primary cluster group can meet scheduling requirements                                                                                                   |
| Divided/Dynamic             | How does it behave?                                                                                             | From the perspective of a certain tier cluster group: this tier will try to fill all remaining replicas with Dynamic strategy until all clusters in this tier are filled or all remaining replicas are allocated    |

**How to handle when a cluster appears multiple times in different tiers?**

Determine its priority order based on the tier where it first appears.

**How to handle the old ClusterAffinities after introducing the new API?**

Inherited cluster affinities scheduling is a new use case that does not conflict with the existing ClusterAffinities use case. They cannot completely replace each other. Instead, it serves as a supplement and extension to the existing multi-scheduling group capabilities.

Current use case of ClusterAffinities: Multi-cluster group isolation (region, provider) and failover, used to select one from multiple candidate cluster groups. It serves as a supplement and extension to the existing multi-scheduling group capabilities.

Inherited cluster affinities: Used for cost optimization and elastic scaling. There is only one candidate cluster group, and clusters within one candidate cluster group are used in tiered order.