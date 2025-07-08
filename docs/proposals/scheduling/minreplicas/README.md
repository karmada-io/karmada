
---
title: Support Per-Cluster Min/Max Replicas in PropagationPolicy
authors:
- "@vie-serendipity"
reviewers:
- TBD
approvers:
- TBD

creation-date: 2025-07-08

---

# Support Per-Cluster MinReplicas in PropagationPolicy

## Summary

Currently, Karmada's PropagationPolicy does not support any `minReplicas` constraint for replica scheduling. This limits flexibility in multi-cluster scenarios, as users cannot enforce minimum replica requirements at either the global or per-cluster level. This proposal introduces a new, extensible struct `ClusterConstraint` under `ClusterPreferences`, allowing users to specify `minReplicas` globally or for specific clusters/groups via `ClusterConstraintTerms`. This enhancement is controlled by a feature gate and aims to provide fine-grained control over replica distribution.

## Motivation

In real-world multi-cluster deployments, users expect every candidate cluster participating in scheduling to have at least one or more replicas of the workload. This ensures that each cluster is always ready to handle business traffic, especially in failover scenarios when other clusters become unavailable. However, due to imbalanced resource capacities and current scheduling strategies, some candidate clusters may end up with zero replicas for both static and dynamic weight preferences.

This proposal addresses this gap by allowing users to specify global and per-cluster (or group) minimum and maximum replica constraints via `ClusterConstraint` and `ClusterConstraintTerms`, ensuring high availability and seamless failover for critical workloads.

### Goals

- Provide a clear and extensible API structure for future constraints.
- Support min replica constraints in Karmada scheduling for both static and dynamic weight preferences.
- Ensure backward compatibility and feature gate control for the new fields.

### Non-Goals

- Changing the default scheduling logic for clusters that do not specify constraints.
- Implementing advanced constraint types beyond min replicas in this iteration.

## Proposal

Introduce a new struct `ClusterConstraint` under `ClusterPreferences` in the PropagationPolicy CRD. This struct supports global `minReplicas` field (optional), and a list of `ClusterConstraintTerms` for per-cluster or per-group constraints. The scheduler will respect these constraints during replica assignment, with per-cluster/group terms taking precedence over global settings.

### User Stories (Optional)

#### Story 1

As a platform admin, I want to ensure that every candidate cluster always has at least one replica of my workload, so that if any cluster fails, the remaining clusters can immediately take over business traffic without interruption. By setting `minReplicas` for each cluster, I can guarantee high availability and seamless failover.

### Notes/Constraints/Caveats (Optional)

- The new fields are ignored unless the feature gate is enabled.
- Validation and scheduling logic must be updated to respect these constraints.
- If both global and per-cluster/group constraints are set, the value in `ClusterConstraintTerms` takes precedence for the specified clusters/groups.

### Risks and Mitigations

- **Risk:** Increased complexity in scheduling logic may introduce bugs or unexpected behaviors.
  **Mitigation:** Comprehensive unit and integration tests, gradual rollout with the feature gate.

## Design Details

### API Design

The `ClusterConstraint` struct is introduced under `ClusterPreferences` in the PropagationPolicy CRD. This struct allows specifying global `minReplicas` constraints, as well as a list of `ClusterConstraintTerms` for per-cluster or per-group constraints. The fields are optional and only take effect when the feature gate is enabled.

```go
// ClusterConstraint defines global and per-cluster/group scheduling constraints for replicas.
type ClusterConstraint struct {
    // MinReplicas specifies the minimum number of replicas that should be assigned to each selected cluster.
    // This is a global field for all clusters, mainly used when ClusterConstraintTerms is not set.
    // If a cluster/group is specified in ClusterConstraintTerms, the value in ClusterConstraintTerms takes precedence.
    // +kubebuilder:validation:Minimum=0
    // +optional
    MinReplicas *int32 `json:"minReplicas,omitempty"`

    // MaxReplicas specifies the maximum number of replicas that can be assigned to each selected cluster.
    // This is a global field for all clusters, mainly used when ClusterConstraintTerms is not set.
    // If a cluster/group is specified in ClusterConstraintTerms, the value in ClusterConstraintTerms takes precedence.
    // +kubebuilder:validation:Minimum=0
    // +optional
    // MaxReplicas *int32 `json:"maxReplicas,omitempty"`

    // ClusterConstraintTerms defines scheduling constraints for specific clusters or groups of clusters.
    // If a cluster/group is specified here, its value takes precedence over the global MinReplicas/MaxReplicas.
    // +optional
    ClusterConstraintTerms []ClusterConstraintTerm `json:"clusterConstraintTerms,omitempty"`
}

// ClusterConstraintTerm defines scheduling constraints for a cluster or group of clusters.
type ClusterConstraintTerm struct {
    // TargetCluster describes the filter to select clusters for this constraint term.
    // +required
    TargetCluster ClusterAffinity `json:"targetCluster,omitempty"`

    // MinReplicas specifies the minimum number of replicas for the cluster(s) specified by 'TargetCluster'.
    // +kubebuilder:validation:Minimum=0
    // +optional
    MinReplicas int32 `json:"minReplicas,omitempty"`

    // MaxReplicas specifies the maximum number of replicas for the cluster(s) specified by 'TargetCluster'.
    // +kubebuilder:validation:Minimum=0
    // +optional
    // MaxReplicas *int32 `json:"maxReplicas,omitempty"`
}
```

### Implementation Details

The implementation will involve karmada-webhook and karmada-scheduler.

### Webhook

Add new validation logic in the webhook to ensure that:

- The sum of `minReplicas` across all clusters does not exceed the total replicas.

#### Scheduler

First, determine the final minReplicas of each cluster based on the `ClusterConstraint` and `ClusterConstraintTerms`. `ClusterConstraintTerms` takes precedence, and if a cluster is declared multiple times in `ClusterConstraintTerms`, the minimum value is used.

Second, scheduling changes mainly focus on the Dispenser logic in the scheduler. It's necessary to add new logic before the original logic. Below is a pseudocode example for assigning replicas to clusters, ensuring each cluster meets its minReplicas constraint, and then distributing the remaining replicas according to weights:

```pseudo
Input:
  totalReplicas: int
  clusters: list of { name, minReplicas, weight }

// New logic to handle minReplicas constraints
// Step 1: Assign minReplicas to each cluster
assigned = {}
remaining = totalReplicas
for cluster in clusters:
    assigned[cluster.name] = cluster.minReplicas
    remaining -= cluster.minReplicas

// remaining is guaranteed to be non-negative, validated in webhook

// Original logic
// Step 2: Distribute remaining replicas by weight
totalWeight = sum(cluster.weight for cluster in clusters)
for cluster in clusters:
    if remaining == 0:
        break
    share = int(remaining * cluster.weight / totalWeight)
    assigned[cluster.name] += share
    remaining -= share

// Step 3: Distribute any leftovers (due to rounding)
for cluster in clusters:
    if remaining == 0:
        break
    assigned[cluster.name] += 1
    remaining -= 1

return assigned
```

## Test Plan

- Unit tests
  - Add ut to related webhook and scheduler change.

- E2E tests
  - Make sure invalid PropagationPolicy with minReplicas is rejected.
  - PropagationPolicy with minReplicas in `ClusterConstraint` is accepted and scheduling works as expected.
  - PropagationPolicy with minReplicas in `ClusterConstraintTerms` is accepted and scheduling works as expected.

## Alternatives

For dynamic preferences, the above design is reasonable. However, for static preferences, minReplicas might break the original weight ratio.

An alternative is as follows:

```pseudo
Input:
  totalReplicas: int
  clusters: list of { name, weight }

// Step 1: Assign replicas by weight, using ceiling
assigned = {}
totalWeight = sum(cluster.weight for cluster in clusters)
remaining = totalReplicas
for cluster in clusters:
    share = ceil(totalReplicas * cluster.weight / totalWeight)
    assigned[cluster.name] = share
    remaining -= share

// Step 2: If total assigned > totalReplicas, reduce assignments
while sum(assigned.values()) > totalReplicas:
    for cluster in clusters:
        if assigned[cluster.name] > 1:
            assigned[cluster.name] -= 1
            if sum(assigned.values()) == totalReplicas:
                break
    // If all clusters have only 1 replica, stop (no cluster will be reduced to 0)

return assigned
```

This approach ensures each cluster gets at least one replica, the total does not exceed the desired count, and the distribution remains as close as possible to the original weight ratio. However, this approach can only guarantees at least one replica per cluster and is inconsistent with previous implementations.

We prefer the first approach because it is more flexible and can be used in both static and dynamic weight preferences. The scheduler will keep the replica distribution as close as possible to the original weight ratio while ensuring minReplicas constraints, which is consistent with the intended semantics.
