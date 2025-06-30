---
title: Support cluster failover for stateful workloads
authors:
  - "@XiShanYongYe-Chang"
reviewers:
  - "@mszacillo"
  - "@Dyex719"
  - "@RainbowMango"
approvers:
  - "@kevin-wangzefeng"
  - "@RainbowMango"
creation-date: 2025-06-26
---

# Support cluster failover for stateful workloads

This proposal is modified base on the design of https://github.com/karmada-io/karmada/pull/5116.

## Summary

The advantage of stateless workloads is their higher fault tolerance, as the loss of workloads does not affect user sessions. However, for stateful workloads (such as Flink), the loss of workloads may result in the loss of session data, preventing the workload from recovering correctly.

For workloads, failover scenarios include two cases: the first is when the workload itself fails and needs to be migrated to another cluster; the second is when the cluster encounter a failure, such as a network partition, requiring some workloads within the cluster to be migrated.

The Karmada community has completed the design and development for the first scenario in version v1.12: https://github.com/karmada-io/karmada/issues/5788. This proposal will build upon the previous design experience to support the second scenario, which involves stateful workload failover caused by cluster failures. The API design will unify the handling of common aspects in both scenarios to avoid redundant fields and improve the user experience.

## Motivation

Karmada currently supports propagating various types of resources, including Kubernetes objects and CRDs, which is particularly useful for ensuring the resilience of stateful workloads in multi-cluster environments. However, the Karmada system's processing logic is based on the assumption that the propagated resources are stateless. In failover scenarios, users may want to preserve a certain state before failure so that workloads can resume from the point where they stopped in the previous cluster.

For CRDs that handle data processing (such as Flink or Spark), restarting the application from a previous checkpoint can be particularly useful. This allows the application to seamlessly resume processing from the point of interruption while avoiding duplicate processing. Karmada v1.12 supports stateful workload migration after application failures, and the next step is to extend support for stateful workload migration following cluster failures.

### Goals

Support stateful application cluster failover, allowing users to customize the state information that needs to be preserved for workloads in the failed cluster within Karmada. This information is then passed to the migrated workloads, enabling the newly started workloads to recover from the state where the previous cluster stopped.

### No-Goals

- No optimization will be made for cluster failover functionality.
- No optimization will be made for application failover functionality.

## Proposal

### Flink use case

This section helps to provide a basic understanding of how Flink applications work.

In Flink applications, checkpoints are snapshots of the application's state at a specific point in time. They contain information about all records processed up to that moment. This information is continuously persisted to durable storage at regular intervals. To retrieve the latest Flink application state from persistent storage, the `jobId` of the job being recovered and the path to the persisted state are required. Users can then use this information to modify the Flink CRD accordingly to restore from the latest state.

```yaml
jobStatus:
  checkpointInfo:
    formatType: FULL
    lastCheckpoint:
      formatType: FULL
      timeStamp: 1734462491693
      triggerType: PERIODIC
    lastPeriodicCheckpointTimestamp: 1734462491693
    triggerId: 447b790bb9c88da9ae753f00f45acb0e
    triggerTimestamp: 1734462506836
    triggerType: PERIODIC
  jobId: e6fdb5c0997c11b0c62d796b3df25e86
```

The recovery process for Flink workloads can be summarized as follows:

1. Karmada obtains the `{ .jobStatus.jobId }` from the FlinkDeployment in the failed cluster.
2. Karmada synchronizes the `jobId` to the migrated FlinkDeployment using a label with a `<user-defined key>:<jobId>`.
3. The user retrieves checkpoint data using the `jobId` and generates the initialSavepointPath: `/<shared-path>/<job-namespace>/<jobID>/checkpoints/<checkpoint>`.
4. The user injects the initialSavepointPath information into the migrated FlinkDeployment.
5. The Flink Operator restarts Flink from the savepoint.

### User Stories

#### Story 1

As an administrator or operator, I can define the state preservation strategy for Flink applications deployed on Karmada after a cluster failure. This includes preserving the `jobID` of the Flink application, allowing the application to resume processing from the point of failure using the `jobID` after migration.

## Design Details

This proposal provides users with a method to persist the necessary state of workloads during cluster failover migration. By configuring propagation policies, users can leverage the state information preserved by the Karmada system to restore workloads once they are migrated due to cluster failure. Since stateful workloads may have different ways to recover from the latest state, configurability of this feature is crucial for users.

To support this feature, we need at least:
- A mechanism for stateful workloads to preserve state during cluster failover migration and to reload the preserved state upon workload recovery;
- The ability for users to customize which state information to preserve during the state preservation phase;
- The ability for users to customize how the preserved state information is injected into the migrated workload during the state recovery phase.

> Note: An important detail is that if all replicas of a stateful workload are not migrated together, the timing for state recovery becomes uncertain for the stateful workload. Therefore, this proposal focuses on scenarios where all replicas of a stateful workload are migrated together.

### State Preservation

Key design points for state preservation are as follows:

1. The state information to be preserved is parsed from the workload's `.status` field; state information not present in `.status` cannot be preserved.
2. Users can customize one or more pieces of state information to be preserved for the workload.
3. The state preservation method can be unified for workload migration caused by both cluster failure and application failure.
4. The preserved state information is stored as key-value pairs in the `gracefulEvictionTask` of the `ResourceBinding` resource related to the workload.

Modification of PropagationPolicy API

```go
// FailoverBehavior indicates failover behaviors in case of an application or
// cluster failure.
type FailoverBehavior struct {
    // Application indicates failover behaviors in case of application failure.
    // If this value is nil, failover is disabled.
    // If set, the PropagateDeps should be true so that the dependencies could
    // be migrated along with the application.
    // +optional
    Application *ApplicationFailoverBehavior `json:"application,omitempty"`

    // Cluster indicates failover behaviors in case of cluster failure.
    // If this value is nil, failover is disabled.
    // +optional
    // Cluster *ClusterFailoverBehavior `json:"cluster,omitempty"`

    // StatePreservation defines the policy for preserving and restoring state data
    // during failover events for stateful applications.
    //
    // When an application fails over from one cluster to another, this policy enables
    // the extraction of critical data from the original resource configuration.
    // Upon successful migration, the extracted data is then re-injected into the new
    // resource, ensuring that the application can resume operation with its previous
    // state intact.
    // This is particularly useful for stateful applications where maintaining data
    // consistency across failover events is crucial.
    // If not specified, means no state data will be preserved.
    //
    // Note: This requires the StatefulFailoverInjection feature gate to be enabled,
    // which is alpha.
    // +optional
    StatePreservation *StatePreservation `json:"statePreservation,omitempty"`
}

// StatePreservation defines the policy for preserving state during failover events.
type StatePreservation struct {
    // Rules contains a list of StatePreservationRule configurations.
    // Each rule specifies a JSONPath expression targeting specific pieces of
    // state data to be preserved during failover events. An AliasLabelName is associated
    // with each rule, serving as a label key when the preserved data is passed
    // to the new cluster.
    // +required
    Rules []StatePreservationRule `json:"rules"`
}

// StatePreservationRule defines a single rule for state preservation.
// It includes a JSONPath expression and an alias name that will be used
// as a label key when passing state information to the new cluster.
type StatePreservationRule struct {
    // AliasLabelName is the name that will be used as a label key when the preserved
    // data is passed to the new cluster. This facilitates the injection of the
    // preserved state back into the application resources during recovery.
    // +required
    AliasLabelName string `json:"aliasLabelName"`

    // JSONPath is the JSONPath template used to identify the state data
    // to be preserved from the original resource configuration.
    // The JSONPath syntax follows the Kubernetes specification:
    // https://kubernetes.io/docs/reference/kubectl/jsonpath/
    //
    // Note: The JSONPath expression will start searching from the "status" field of
    // the API resource object by default. For example, to extract the "availableReplicas"
    // from a Deployment, the JSONPath expression should be "{.availableReplicas}", not
    // "{.status.availableReplicas}".
    //
    // +required
    JSONPath string `json:"jsonPath"`
}
```

In this proposal, the `StatePreservation` field is promoted from the `ApplicationFailoverBehavior` struct to the `FailoverBehavior` struct. Therefore, whether it is a cluster failure or an application failure, the `StatePreservation` configuration remains consistent for users.

The `ResourceBinding` API already meets the design goals and can directly reuse the `PreservedLabelState` and `ClustersBeforeFailover` fields in the `gracefulEvictionTask` without introducing new modifications.

### State Recovery

Key design points for state recovery are as follows:

1. State recovery is achieved by injecting the preserved state information as labels into the migrated new workload;
2. The timing of label injection is when the workload is migrated to a new cluster (to confirm the cluster is the result of new scheduling, the previous cluster scheduling information needs to be stored in `gracefulEvictionTask`). During the propagation of the workload, the controller injects the preserved state information into the manifest of the work object.

> Note: The controller logic for state recovery can fully reuse the modifications made previously for application failover to support stateful workloads. No new changes are required in this proposal.

### Feature Gate

In Karmada v1.12, the `StatefulFailoverInjection` FeatureGate was introduced to control whether state recovery is performed during the recovery process of stateful applications after application failure. In this proposal, we extend the control of this feature gate to cover stateful application recovery scenarios after cluster failure, thereby unifying the control of stateful application failover recovery.

## System Failure Impact and Mitigation

What situations may cause the feature to fail, and will it cause system crashes?

As mentioned above, this proposal focuses on scenarios where all replicas of a stateful application are migrated together. If users split the replicas of a stateful application and propagate them across different clusters, and the system migrates only some replicas, the stateful application failover recovery feature may not work effectively.

This feature will not cause system crashes.

## Graduation Criteria

### Alpha to Beta

- The `StatePreservation` definition in the `PropagationPolicy` API will transition to Beta;
- The `PreservedLabelState` and `ClustersBeforeFailover` fields in the `ResourceBinding` API will transition to Beta;
- End-to-end (E2E) testing has been completed.

### GA Graduation

- Scalability and performance testing;
- The `StatePreservation` definition in the `PropagationPolicy` API reaches GA;
- The `PreservedLabelState` and `ClustersBeforeFailover` fields in the `ResourceBinding` API reach GA.

## Notes/Constraints/Caveats

None

## Risks and Mitigations

None

## Impact Analysis on Related Features

No related features are affected at this time.

## Upgrade Impact Analysis

In this proposal, the `StatePreservation` field is promoted from the `ApplicationFailoverBehavior` struct to the `FailoverBehavior` struct. To ensure a smooth upgrade, the following steps can be taken:

- Mark the `StatePreservation` field in the `ApplicationFailoverBehavior` struct as deprecated, and remove it after the `PropagationPolicy` API is upgraded;
- Maintain compatibility with both old and new API definitions, ensuring functionality remains effective when users still use the old API definition;
- Remove support for the old API definition in the feature release, meaning that `StatePreservation` defined in `ApplicationFailoverBehavior` will no longer be effective.

## Test Planning

UT: Add unit tests for the newly added functions
E2E: Add end-to-end test cases for stateful workload recovery after cluster failover.
