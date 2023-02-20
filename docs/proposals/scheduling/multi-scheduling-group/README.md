---
title: Multiple scheduling group

authors:
- "@RainbowMango"

reviewers:
- "@chaunceyjiang"
- "@Garrybest"
- "@lonelyCZ"
- "@Poor12"
- "@XiShanYongYe-Chang"

approvers:
- "@kevin-wangzefeng"

creation-date: 2023-02-06

---

# Multiple scheduling group

## Summary

The current PropagationPolicy supports declaring `only one` group of clusters, that
is the `.spec.placement.clusterAffinity`, e.g.
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: foo
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: foo
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
```
The `clusterAffinity` supplies a group of candidate clusters to the `karmada-scheduler`.
The `karmada-scheduler` makes schedule decisions among the candidate clusters
according to relevant restrictions(like spreadConstraint, filter plugins, and so on),
and the scheduling results are either successful or failure.
- successful: successfully selected a group of clusters(probably a subset of candidates) for the referencing resource.
- failure: failed to select a group of clusters that satisfied the restrictions.

This proposal proposes a strategy that multiple `clusterAffinity` could be declared,
and the `karmada-scheduler` could make the decision by evaluating each `clusterAffinity`
in a specified order.

## Motivation

The cluster administrators usually classify their clusters by categories(like,
by provider, usage, and so on) into different groups, and hoping to deploy
workload to the preferred groups and leave a backup group in case of preferred
group doesn't satisfy scheduling restrictions like lack of resources.

The Karmada community received a lot of feedback about that from end users, like
issue [#780](https://github.com/karmada-io/karmada/issues/780) and
[#2085](https://github.com/karmada-io/karmada/issues/2085).

### Goals

- Extend the API of PropagationPolicy to hold multiple affinity groups declaration.
- Extend the API of ResourceBinding to annotate the affinity group that the scheduler is inspecting.
- Propose the implementation ideas for involved components, including `karmada-controller-manager`, `karmada-webhook` and `karmada-scheduler`.

### Non-Goals

- **The relative priority of clusters in the same group**

This proposal focuses on introducing multiple affinity groups, for the relative
priority of clusters in the same group topic is out of scope. And such requirements
could be done by `weightPreference`(.spec.placement.replicaScheduling.weightPreference) or
define a strategy for `karmada-scheduler` to score specified clusters.

- **Scheduling re-balance**

Indeed, oftentimes people want the `karmada-scheduler` to perform an extra schedule
for different purposes, like the question addressed by the discussion
[#3069](https://github.com/karmada-io/karmada/discussions/3069), these kind of
issues should be tracked by another proposal.

## Proposal

### User Stories (Optional)

#### As a user, I prefer to deploy my applications on clusters in local data center to save costs.

I have some clusters in my data center as well as some managed clusters from cloud
providers(like AWS and Google Cloud). Since the cost of the managed clusters is higher
than private clusters, so I prefer to deploy applications on private clusters and
take the managed clusters as a backup.

#### As a user, I want to deploy applications on the primary cluster and leave a backup cluster for disaster recovery cases.

I have two clusters, the primary, and the backup cluster, I want Karmada to deploy
my applications on the primary cluster and migrate applications on the backup cluster
when the primary cluster becomes unavailable due to like data center power off or
maintenance task.

### Notes/Constraints/Caveats (Optional)

This proposal mainly focuses on the changes in `PropagationPolicy` and `ResourceBinding`,
but it will also be applied to `ClusterPropagationPolicy` and `ClusterResourceBinding`.

### Risks and Mitigations

This proposal maintains the backward compatibility, the system built with previous
versions of Karmada can be seamlessly migrated to the new version.
The previous configurations(yamls) could be applied to the new version of Karmada and
without any behavior change.

## Design Details

### API change

#### PropagationPolicy API change

This proposal proposes a new field `ClusterAffinities` for declaring
multiple affinity terms in `.spec.placement` of `PropagationPolicy`.
```go
// Placement represents the rule for select clusters.
type Placement struct {
	// ClusterAffinity represents scheduling restrictions to a certain set of clusters.
	// If not set, any cluster can be scheduling candidate.
	// +optional
	ClusterAffinity *ClusterAffinity `json:"clusterAffinity,omitempty"`

    // ClusterAffinities represents scheduling restrictions to multiple cluster
    // groups that indicated by ClusterAffinityTerm.
    //
    // The scheduler will evaluate these groups one by one in the order they
    // appear in the spec, the group that does not satisfy scheduling restrictions
    // will be ignored which means all clusters in this group will not be selected
    // unless it also belongs to the next group(a cluster could belong to multiple
    // groups).
    //
    // If none of the groups satisfy the scheduling restrictions, then scheduling
    // fails, which means no cluster will be selected.
    //
    // Note:
    //   1. ClusterAffinities can not co-exist with ClusterAffinity.
    //   2. If both ClusterAffinity and ClusterAffinities are not set, any cluster
    //      can be scheduling candidates.
    //
    // Potential use case 1:
    // The private clusters in the local data center could be the main group, and
    // the managed clusters provided by cluster providers could be the secondary
    // group. So that the Karmada scheduler would prefer to schedule workloads
    // to the main group and the second group will only be considered in case of
    // the main group does not satisfy restrictions(like, lack of resources).
    //
    // Potential use case 2:
    // For the disaster recovery scenario, the clusters could be organized to
    // primary and backup groups, the workloads would be scheduled to primary
    // clusters firstly, and when primary cluster fails(like data center power off),
    // Karmada scheduler could migrate workloads to the backup clusters.
    //
    // +optional
    ClusterAffinities []ClusterAffinityTerm `json:"clusterAffinities,omitempty"`

	// ClusterTolerations represents the tolerations.
	// +optional
	ClusterTolerations []corev1.Toleration `json:"clusterTolerations,omitempty"`

	// SpreadConstraints represents a list of the scheduling constraints.
	// +optional
	SpreadConstraints []SpreadConstraint `json:"spreadConstraints,omitempty"`

	// ReplicaScheduling represents the scheduling policy on dealing with the number of replicas
	// when propagating resources that have replicas in spec (e.g. deployments, statefulsets) to member clusters.
	// +optional
	ReplicaScheduling *ReplicaSchedulingStrategy `json:"replicaScheduling,omitempty"`
}

// ClusterAffinityTerm selects a set of cluster.
type ClusterAffinityTerm struct {
	// AffinityName is the name of the cluster group.
	// +required
	AffinityName string `json:"affinityName"`

	ClusterAffinity `json:",inline"`
}
```
Each affinity term essentially is the named `ClusterAffinity`. During the scheduling
phase, the scheduler evaluates terms in the order they appear in the spec one by
one, and turns to the next term if the term does not satisfy restrictions.

The following configuration sample declares 3 affinity terms:
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
      - affinityName: dc-shanghai
        clusterNames:
          - unavailable
      - affinityName: dc-beijing
        clusterNames:
          - member1
      - affinityName: dc-hongkong
        clusterNames:
          - member2
```

During the scheduling phase, the scheduler will first look at the affinity
named `dc-shanghai` and try to select a feasible cluster set from it.

If no feasible cluster is found from the term, the scheduler will try against the
next term which is the one named `dc-beijing`. And once the scheduler successfully
selected a feasible cluster set from this affinity term, the scheduler will not
continue to look at the following terms.

And, in case of cluster `member1` becomes unavailable, by leveraging the
[Failover feature](https://karmada.io/docs/userguide/failover/failover-overview)
the scheduler will start looking for alternatives `in current affinity term`, and
if that fails, it will look at the term named `dc-hongkong`.

**Note:** Each affinity term is completely independent, and the scheduler will
only select one term at a time during scheduling. But it is allowed for the same
cluster present in multiple affinity terms.

#### ResourceBinding API change

In order to track the affinity term that the scheduler last evaluated,
the proposal proposes a new field named `SchedulerObservedAffinityName` to
`.status` of `ResourceBinding`, so that the scheduler can continue the previous
scheduling cycle in any cases of re-schedule.

```go
// ResourceBindingStatus represents the overall status of the strategy as well as the referenced resources.
type ResourceBindingStatus struct {
	// SchedulerObservedGeneration is the generation(.metadata.generation) observed by the scheduler.
	// If SchedulerObservedGeneration is less than the generation in metadata means the scheduler hasn't confirmed
	// the scheduling result or hasn't done the schedule yet.
	// +optional
	SchedulerObservedGeneration int64 `json:"schedulerObservedGeneration,omitempty"`

	// SchedulerObservedAffinityName is the affinity terms that
	// scheduler looking at.
	// +optional
	SchedulerObservedAffinityName string `json:"schedulerObservingAffinityName,omitempty"`

	// Conditions contain the different condition statuses.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// AggregatedStatus represents status list of the resource running in each member cluster.
	// +optional
	AggregatedStatus []AggregatedStatusItem `json:"aggregatedStatus,omitempty"`
}
```

E.g:

```yaml
status:
  aggregatedStatus:
  - applied: true
    clusterName: member1
    health: Healthy
    status:
      availableReplicas: 2
      readyReplicas: 2
      replicas: 2
      updatedReplicas: 2
  conditions:
  - lastTransitionTime: "2023-02-04T09:38:20Z"
    message: All works have been successfully applied
    reason: FullyAppliedSuccess
    status: "True"
    type: FullyApplied
  - lastTransitionTime: "2023-02-04T09:38:20Z"
    message: Binding has been scheduled
    reason: BindingScheduled
    status: "True"
    type: Scheduled
  schedulerObservedGeneration: 2
  SchedulerObservedAffinityName: ds-hongkong
```

The `SchedulerObservedAffinityName: ds-hongkong` means the scheduling result is based on
the affinity term named `ds-hongkong`, in case of re-scheduling, the scheduler should
continue to evaluate from this affinity term.

### Components change


#### karmada-controller-manager

When creating or updating `ResourceBidning`/`ClusterResourceBinding`, the added
`OrderedClusterAffinities` in `PropagationPolicy`/`ClusterPropagationPolicy` should
be synced.

#### karmada-scheduler

Currently, the karmada-scheduler only runs a single loop by accepting an affinity
term, that is [ScheduleAlgorithm interface](https://github.com/karmada-io/karmada/blob/32b8f21b79017e2f4154fbe0677cb63fb18b120c/pkg/scheduler/core/generic_scheduler.go#L22).

With this proposal, the `ScheduleAlgorithm interface` probably be invoked multi
time, and feed a different affinity term each time until the schedule succeeds.

#### karmada-webhook

Since it doesn't make sense for the newly introduced `ClusterAffinities`
co-exist with the previous `ClusterAffinity`, so the webhook should perform extra
validation work to prevent misleading configuration.

In addition, the `affinityName` of each affinity terms should be unique among all terms.

### Test Plan

- All current testing should be passed, no break change would be involved by this feature.
- Add new E2E tests to cover the feature, the scope should include:
  * Workload propagating with scheduling type `Duplicated`.
  * Workload propagating with scheduling type `Divided`
  * Failover scenario

## Alternatives

Introducing a new field to specify cluster priority was one of the first suggested
ideas.(tracked by [#842](https://github.com/karmada-io/karmada/pull/842)).
This approach tried to reuse the terms in Kubernetes Pod affinity.
An example of this approach is as follows:
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
 name: foo
spec:
 resourceSelectors:
   - apiVersion: apps/v1
     kind: Deployment
     name: foo
 placement:
   clusterAffinity:
     clusterNames:
       - member1
       - member2
     propagatePriority:
       - weight: 30
         preference:
           matchExpressions:
             - key: topology
               operator: In
               values:
                 - us 
       - weight: 20
         preference:
           matchExpressions:
             - key: topology
               operator: In
               values:
                 - cn 
```
The `.spec.placement.clusterAffinity.propagatePriority` is used to specify the
cluster preference by reusing the [PreferredSchedulingTerm of Kubernetes](https://github.com/kubernetes/kubernetes/blob/fc002b2f07250a462bb1b471807708b542472c18/staging/src/k8s.io/api/core/v1/types.go#L3028-L3035).

However, the `PreferredSchedulingTerm` relies on group cluster by the label selector
and field selector, the `matchExpressions` involves too many nested layers that
probably make the configuration hard to maintain.
