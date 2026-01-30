---
title: Workload Affinity / Anti-Affinity Support
kep-number: 521
authors:
  - "@mszacillo"
reviewers:
  - "@kevin-wangzefeng"
  - "@RainbowMango"
  - "@seanlaii"
approvers:
  - "@kevin-wangzefeng"
  - "@RainbowMango"
creation-date: 2025-08-22
last-updated: 2026-01-07
---

# Karmada Workload Affinity / Anti-Affinity Support

## Summary

In Kubernetes, it is possible to define pod-level anti-affinity which will cause K8s to schedule pods to run on different nodes. This can be useful in cases where users want to ensure high availability. In Karmada, the same sort of behavior would be useful when trying to guarantee that separately deployed but related workloads are scheduled on different clusters.

This proposal describes the problem statement, the desired final behavior, as well as the required api and code-level changes.

## Motivation

Karmada's PropagationPolicy API already provides pretty fine-grained scheduling configurations, allowing users to set different replica scheduling strategies, spread constraints, and define cluster affinity rules. However, in cases in which users may want to ensure related workloads to exist in different clusters, or in cases in which users may want guarantee multiple workloads are scheduled to the same cluster, these settings are not enough.

### Goals

- Extend the Propagation Policy API to support workload affinity and anti-affinity based scheduling rules.
- Provide a way for users who want HA for their data processing pipelines (or other workloads) with anti-affinity.
- Provide a solution for users who need workloads to be colocated and scheduled to the same clusters.

### Non-Goals

- The support of `PreferredDuringScheduling` is deferred from this proposal, as the use-case is not totally defined and this scheduling strategy is more computationally expensive
- Cross-namespace anti-affinity / affinity is not supported. Existing API design inherits namespace from the PropagationPolicy, so affinity terms are limited to within the namespace. This can be added in the future if there are sufficient use-cases.
- Additional topologyKey support is also not included in the API design and we instead rely directly on the clusters when enforcing affinity rules. This can be added to the API in the future if there are use-cases for indexing by other zone/region topologies.

### User Stories

#### Story 1: Anti-Affinity with FlinkDeployments

As a user of Apache Flink, there may be cases where my data pipeline is has a very low tolerance for downtime (e.g. the job cannot stop processing data for more than a couple seconds). In cases like this I cannot rely on Karmada's cluster failover feature alone since by the time Karmada is able to reschedule my application, the downstream consumers will already have gaps in their data. The solution is to run two duplicate processing pipelines for HA purposes, allowing time for Karmada to schedule the impacted application while the other continues processing.

Karmada should support being able to schedule these types of duplicate pipelines, by making sure that the applications are never scheduled to the same cluster (which would break HA gaurantees).

In the picture below, we can see two workloads which represent two duplicate pipelines scheduled for HA purposes. Karmada will make sure that during the scheduling phase, it will not schedule the application to a cluster if a workload with a `karmada.io/group` label and matching key exists:

![workload-anti-affinity](workload-anti-affinity.png)

#### Story 2: Affinity with Distributed Training Workloads

For very large training jobs, a user may start to run into memory and batch size limits where datasets don't fit into a single worker's GPU memory. As a result, the user may want to shard the training data using multiple smaller training jobs.

These jobs would be applied to Karmada, and the Karmada scheduler should be able to handle scheduling these jobs to the same cluster using affinity rules.

#### Story 3: Affinity for Co-located Applications

There may be cases in which users want to schedule workloads to the same cluster to minimize latency by avoiding cross-cluster network hops between services. Some examples of such as case could be:

- A FlinkDeployment with Kafka Consumers (for ultra-low latency requirements)
- A RayService that relies on RedisCluster for caching (colocated prevents cross-cluster network hops)

## Proposal

This proposal extends the PropagationPolicy API to provide support for workload affinity and anti-affinity scheduling rules. Upon scheduling, Karmada will be able to filter feasible clusters based on the workload's affinity and anti-affinity preferences.

This proposal is divided into several steps, see below:

- [ ] `PropagationPolicy` API changes to add `WorkloadAffinity` field, consisting of either `Affinity` or `AntiAffinity` terms.
- [ ] `ResourceBinding` API change to add `AffinityGroups` field, which will store the instantiated affinity group names.
- [ ] `ClusterInfo` and `ClusterSnapshot` enhancements to include affinity metadata
- [ ] Karmada Scheduler changes required to support affinity rules
- [ ] Addition of affinity / anti-affinity filter plugins

## API Changes

In order to support workload affinity / anti-affinity, we will need to make changes to the PropagationPolicy API spec. We will be borrowing the concept of pod affinity and anti-affinity from Kubernetes to create an analogous behavior on a workload level.

### PropagationPolicy

```golang
// Placement represents the rules for selecting target clusters.
type Placement struct {
	// WorkloadAffinity represents inter-workload affinity and anti-affinity
	// scheduling policies.
	// +optional
	WorkloadAffinity *WorkloadAffinity `json:"workloadAffinity,omitempty"`
}

// WorkloadAffinity defines inter-workload affinity and anti-affinity rules.
type WorkloadAffinity struct {
	// Affinity represents inter-workload affinity scheduling rules.
	// These are hard requirements: workloads will only be scheduled to clusters
	// that satisfy the affinity term if it is specified.
	//
	// For the first workload of an affinity group (when no workloads with a
	// matching label value exist in the system), the scheduler will not block
	// scheduling. This allows bootstrapping new workload groups without
	// encountering scheduling deadlocks, providing a better user experience.
	//
	// +optional
	Affinity *WorkloadAffinityTerm `json:"affinity,omitempty"`

	// AntiAffinity represents inter-workload anti-affinity scheduling rules.
	// These are hard requirements: workloads will be scheduled to avoid clusters
	// where matching workloads are already scheduled.
	//
	// +optional
	AntiAffinity *WorkloadAntiAffinityTerm `json:"antiAffinity,omitempty"`

	// Note: Both Affinity and AntiAffinity terms, if specified, are required to be
	// satisfied during scheduling. If more flexible rules are needed (for example,
	// preferred scheduling), PreferredAffinity and PreferredAntiAffinity fields
	// can be added in the future.
}
```

### WorkloadAntiAffinity

```golang
// WorkloadAntiAffinityTerm defines anti-affinity rules for separating workloads
// from specific workload groups.
type WorkloadAntiAffinityTerm struct {
	// GroupByLabelKey declares the label key on the workload resource template that
	// determines the anti-affinity group. Workloads with the same label value under
	// this key belong to the same anti-affinity group and will be separated.
	//
	// The scheduler maintains a global index of affinity groups in memory for
	// efficient lookup. Each affinity group is identified by a serialized
	// key-value pair and contains all workload resource templates that belong
	// to the group.
	//
	// Note: Affinity groups are scoped to the namespace. Workloads that use the
	// same anti-affinity label but reside in different namespaces are not treated
	// as part of the same group.
	//
	// The key must be a valid Kubernetes label key.
	//
	// Example: If GroupByLabelKey is "app.group", workloads with the label
	// "app.group=frontend" will avoid clusters where other
	// "app.group=frontend" workloads already exist.
	//
	// +required
	GroupByLabelKey string `json:"groupByLabelKey"`
}
```

### WorkloadAffinity

```golang
// WorkloadAffinityTerm defines affinity rules for co-locating workloads with
// specific workload groups.
type WorkloadAffinityTerm struct {
	// GroupByLabelKey declares the label key on the workload resource template that
	// determines the affinity group. Workloads with the same label value under
	// this key belong to the same affinity group.
	//
	// The scheduler maintains a global index of affinity groups in memory for
	// efficient lookup. Each affinity group is identified by a serialized
	// key-value pair and contains all workload resource templates that belong
	// to the group.
	//
	// Note: Affinity groups are scoped to the namespace. Workloads that use the
	// same affinity label but reside in different namespaces are not treated
	// as part of the same group.
	//
	// The key must be a valid Kubernetes label key.
	//
	// Example: If GroupByLabelKey is "app.group", workloads with the label
	// "app.group=frontend" will form one affinity group, while those with
	// "app.group=backend" will form another.
	//
	// +required
	GroupByLabelKey string `json:"groupByLabelKey"`
}
```

### ResourceBinding

```golang
type ResourceBindingSpec struct {
	// AffinityGroups represents instantiated grouping results derived from
	// Placement.WorkloadAffinity. It is used to keep workloads in the same
	// affinity group co-located and workloads in the same anti-affinity group
	// separated across clusters.
	//
	// This field is populated by controllers and consumed by the scheduler
	// during scheduling decisions.
	// +optional
	AffinityGroups *BindingAffinityGroups `json:"affinityGroups,omitempty"`
}

// BindingAffinityGroups stores instantiated affinity and anti-affinity group
// names derived from PropagationPolicy or ClusterPropagationPolicy
// WorkloadAffinity. Each group name is serialized as "<labelKey>=<labelValue>"
// (for example, "app.group=frontend").
//
// These group names are used by the scheduler to co-locate workloads that belong
// to the same affinity group and to separate workloads that belong to the same
// anti-affinity group.
//
// Note: If support for multiple groups is required in the future, prefer adding
// list-typed fields (for example, AffinityGroups or AntiAffinityGroups) to
// preserve backward compatibility.
type BindingAffinityGroups struct {
	// AffinityGroup is the instantiated group name derived from affinity rules.
	// +optional
	AffinityGroup string `json:"affinityGroup,omitempty"`

	// AntiAffinityGroup is the instantiated group name derived from anti-affinity rules.
	// +optional
	AntiAffinityGroup string `json:"antiAffinityGroup,omitempty"`
}
```

### WorkloadAffinityTerm API Definition Options

While constructing the WorkloadAffinity and WorkloadAntiAffinityTerm API definitions, we thought of a couple options. They are presented here in order, with option 3 being the one we moved forward with.

#### Option 1: Define LabelSelector + MatchLabelKeys

We originally tried to model WorkloadAffinityTerms closely after the PodAffinityTerm in K8s. This included several fields that allowed for fine-grained control over affinity behavior, such as LabelSelectors, MatchLabelKeys, and TopologyKey.

```golang
// WorkloadAffinityTerm selects a set of peer workloads used to evaluate inter-workload affinity / anti-affinity.
//
// Peer Selection:
// - Match `labelSelector`.
// - Additionally, for each key in `matchLabelKeys`, peers must have the same
//   label value as this workload.
//
// Evaluation:
// - For affinity: a candidate cluster is required if a matching peer is found on that cluster.
// - For anti-affinity: a candidate cluster is rejected if a matching peer is found on that cluster.
type WorkloadAffinityTerm struct {

  // A label query over a set of resources, in this case workloads.
  // If it's null, this WorkloadAffinityTerm matches with no workload.
  // +optional
  LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

  // AND with LabelSelector, but values come from THIS workload’s labels.
  // e.g., ["ha.karmada.io/group"] → match workloads sharing the same group value.
  // +optional
  MatchLabelKeys []string `json:"matchLabelKeys,omitempty"`

  // // Cluster label key defining the co-location domain (e.g., topology.karmada.io/cluster|zone|region).
  // // +required
  // TopologyKey string `json:"topologyKey,omitempty"`
  // By design we have flexibility to add TopologyKey in the future, however we do not want to over-engineer before we get more
  // community feedback on use-cases.
}
```

Benefits:
- Allows for fine-grained control over labels used for selecting affinity peers
- Allows for greater flexibility to add topologyKey or namespaceScope in the future

Drawbacks:
- A bit more complicated for user, may not be intuitive for users to define scheduler configurations AND add the same labels to the AffinityTerm API

#### Option 2: Simplified API With Scheduler Configurations

This option greatly simplified the WorkloadAffinityTerm design with a simple boolean for switching the feature on and off.

Since indexing can become very expensive when making scheduling decisions based on anti-affinity / affinity terms, we could  expose a configuration to the Karmada Scheduler which would limit the set of labels that Karmada must keep track of. For example, if the user would like to enforce anti-affinity using the `karmada.io/group` label then they would define this argument on the scheduler:

```yaml
  containers:
  - command:
    - /bin/karmada-scheduler
    - --kubeconfig=/etc/kubeconfig
    - --metrics-bind-address=$(POD_IP):8080
    - --health-probe-bind-address=$(POD_IP):10351
    - --affinity-label-keys="karmada.io/group" <----- Karmada will ONLY index on this label and ignore all others
```

```golang
// WorkloadAffinityTerm selects a set of peer workloads used to evaluate inter-workload affinity / anti-affinity.
//
// Peer Selection:
// - Match based on value of the keys defined in --affinity-label-keys configuration
//
// Evaluation:
// - For affinity: a candidate cluster is required if a matching peer is found on that cluster.
// - For anti-affinity: a candidate cluster is rejected if a matching peer is found on that cluster.
type WorkloadAffinityTerm struct {

  // Determines if Karmada scheduler will take affinity terms into account for this workload.
  // Affinity labels are controlled via the --affinity-label-keys configuration defined on the Karmada Scheduler.
  // Affinity and anti-affinity peers are determined based off matching values of the keys specified in the scheduler configuration.
  AffinityEnabled bool `json:"affinityEnabled,omitempty"`
}
```

Benefits
- Very simple API definition and mental model: "turn on standardized affinity/anti-affinity for these workloads"

Drawbacks
- Less expressiveness in API, user must rely solely on keys defined in the scheduler configuration
- Global coupling: changing flag on scheduler changes semantics for all workloads on cluster

#### Option 3: Define a GroupLabelKey

To minimize indexing complexity, Karmada will directly rely on the Group Label Key to identify workloads that belong to the same logical affinity group.

```golang
// WorkloadAffinityTerm defines affinity rules for co-locating with specific workload groups.
type WorkloadAffinityTerm struct {
    // GroupByLabelKey declares the label key on the workload resource template that
    // determines the affinity group. Workloads with the same label value under this key
    // belong to the same affinity group.
    //
    // The scheduler maintains a global index of affinity groups in memory for efficient
    // lookup. Each affinity group is identified by a serialized key-value pair, and
    // contains all workload resource templates that belong to the group.
    //
    // The key must be a valid Kubernetes label key.
    //
    // Example: If GroupByLabelKey is "app.group", workloads with label "app.group=frontend"
    // will form one affinity group, while those with "app.group=backend" will form another.
    //
    // +required
    GroupByLabelKey string `json:"groupByLabelKey"`
}
```

An example of its use in a PropagationPolicy is the following:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: flinkdeployment-anti-affinity
spec:
  workloadAffinity:
    affinity:
      groupByLabelKey: "app.type"
  # Workloads with label app.type=indexer will NOT be scheduled to clusters
  # where other app.type=indexer workloads already exist.
  # The first workload will be scheduled normally (bootstrap behavior).
```

## Code Changes

The following code changes are suggested based on the demo branch which was created during the proposal stage, those code changes can be viewed [here](https://github.com/mszacillo/karmada/commit/752f5d9befd25f54e603489c537bf48b1c1e9cee#diff-1d47ba2a598082cc700b6d728aad23daabdf01f27dfc3339b850ecccd40d9742).

### 1. Changes to Snapshot

During each scheduling cycle, we regenerate a snapshot containing all clusters in the federation. At the moment, this snapshot is pretty simple, with ClusterInfo being a wrapper for the Cluster object. For the purposes of the demo, the ClusterInfo list was converted to a Cluster list instead. This was due to some circular dependencies between the cache and framework packages.

To support affinity / anti-affinity, altering the snapshot will be a requirement as we need a place to index and store the group info for our workloads using affinity / anti-affinity terms. For minimal changes, we could introduce just two new data structures:
- A map of GroupName (fetched directly from RB) -> other matching RBs

That way we can track other RBs that are part of the same AffinityGroup. From those RB, we can check which Clusters they are currently scheduled to and filter those clusters out before scheduling.

### 2. Changes to Scheduler Cache

In order to keep the Snapshot up to date with the latest RB changes, we can add updates to the ResourceBinding [event handler](https://github.com/karmada-io/karmada/blob/4e1da87b07760f7f5426e9a085648d03bb7b7bb0/pkg/scheduler/event_handler.go#L139). Based on whether we are responding to a creation, update (removal or addition of affinity rules), or a deletion - we will need to index or unindex the binding.

This can be handled by adding new methods to the cache to handle each case:

```golang

	// Cache should be updated in response to RB changes
	OnResourceBindingAdd(obj interface{})
	OnResourceBindingUpdate(old, cur interface{})
	OnResourceBindingDelete(obj interface{})

```

### 3. Changes to Scheduler

The main scheduler class will require minimal changes, besides updating the `RunFilterPlugins` method to accept `Snapshot` as an argument. This Snapshot will be used by the affinity / anti-affinity plugin to determine filtering.


### 4. Addition of Workload Affinity Filter Plugin

This feature will require the addition of a filter plugin which will filter out clusters that violate the pending workload’s affinity or anti-affinity rules or the rules of one of the workloads currently on the cluster.

For anti-affinity, the filter plugin will check each cluster and determine if the cluster satisfiesWorkloadAntiAffinity by verifying that the cluster matches the topologyKey, and that the pending workload is not conflicting with the anti-affinity counts of any workloads already on the cluster.
