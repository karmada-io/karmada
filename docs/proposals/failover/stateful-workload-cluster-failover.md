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

This proposal is modified base on the design of [#5116](https://github.com/karmada-io/karmada/pull/5116).

## Summary

The advantage of stateless workloads is their higher fault tolerance, as the loss of workloads does not impact application correctness or require data recovery. In contrast, stateful workloads (like Flink) depend on checkpointed or saved state to recover after failures. If that state is lost, the job may not be able to resume from where it left off.

For workloads, failover scenarios include two cases: the first is when the workload itself fails and needs to be migrated to another cluster; the second is when the cluster encounter a failure, such as a network partition, requiring some workloads within the cluster to be migrated.

Workload failover may be triggered either by the workload's own failure (requiring migration to another cluster) or by cluster failures (e.g., node crashes, network partitions, or control plane outages) that necessitate relocating affected workloads.

The Karmada community has completed the design and development for the first scenario in version v1.12: [#5788](https://github.com/karmada-io/karmada/issues/5788). This proposal will build upon the previous design experience to support the second scenario, which involves stateful workload failover caused by cluster failures. The API design will unify the handling of common aspects in both scenarios to avoid redundant fields and improve the user experience.

## Motivation

Karmada currently supports propagating various types of resources, including Kubernetes objects and CRDs, this also includes stateful workload, it enabling multi-cluster resilience and elastic scaling for distributed applications. However, the Karmada system's processing logic is based on the assumption that the propagated resources are stateless. In failover scenarios, users may want to preserve a certain state before failure so that workloads can resume from the point where they stopped in the previous cluster.

For CRDs that define resources involved in data processing (such as Flink or Spark), restarting the application from a previous checkpoint can be particularly useful. This allows the application to seamlessly resume processing from the point of interruption while avoiding duplicate processing. Karmada v1.12 supports stateful workload migration after application failures, and the next step is to extend support for stateful workload migration following cluster failures.

### Goals

Support stateful application cluster failover, allowing users to customize the state information that needs to be preserved for workloads in the failed cluster within Karmada. This information is then passed to the migrated workloads, enabling the newly started workloads to recover from the state where the previous cluster stopped.

### No-Goals

- No optimization will be made for cluster failover functionality.
- No optimization will be made for application failover functionality.

## Proposal

This proposal introduces a way for users to persist state required for their application in the event of the cluster failover. That way, once rescheduled, users can configure their applications to resume using the labels saved by Karmada. Since stateful applications have different implementations of how to retrieve the last state, it is important that this feature is configurable.

To support this feature, we will need at minimum:

- A mechanism with which stateful applications can preserve state during cluster failover and reload before resuming.
- A way users can configure which state will be preserved by the Karmada.
- A way users can configure how the saved information is displayed on the restored stateful application labels.

> Note: One important detail is that if all the replicas of the stateful application are not migrated together, it is not clear when the state needs to be restored. In this proposal we focus on the use case where all the replicas of a stateful application are migrated together. One way to ensure this is to make all the replicas scheduled together using spreadConstraints.

```yaml
spreadConstraints:
  - spreadByField: cluster
    maxGroups: 1
    minGroups: 1
```

### Flink use case

This section helps to provide a basic understanding of how Flink applications work.

In Flink applications, checkpoints are snapshots of the application's state at a specific point in time. They contain information about all records processed up to that moment. This information is continuously persisted to durable storage at regular intervals:

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

To retrieve the latest Flink application state from persistent storage, the `jobId` of the job being recovered and the path to the persisted state are required. The Flink Operator can then use this information to retrieve the last state by providing a few modifications to the Flink resource spec:

```yaml
spec:
  jobSpec:
    initialSavepointPath: “desired checkpoint path to be resumed from (s3p://)”
    upgradeMode: savepoint
    state: running
```

### User Stories

#### Story 1

As an administrator or operator, I can define the state preservation strategy for Flink applications deployed on Karmada after a cluster failure. This includes preserving the `jobID` of the Flink application, allowing the application to resume processing from the point of failure using the `jobID` after migration.

The recovery process for Flink workloads can be summarized as follows:

1. Karmada obtains the `{ .jobStatus.jobId }` from the FlinkDeployment in the failed cluster.
2. Karmada synchronizes the `jobId` to the migrated FlinkDeployment using a label with a `<user-defined key>:<jobId>`.
3. The user retrieves checkpoint data using the `jobId` and generates the initialSavepointPath: `/<shared-path>/<job-namespace>/<jobID>/checkpoints/<checkpoint>`.
4. The user injects the initialSavepointPath information into the migrated FlinkDeployment.
5. The Flink Operator restarts Flink from the savepoint.

For the above steps 3 and 4, I can achieve the goal by deploying Kyverno in the member cluster, defining Kyverno policies to intercepting the distribution of FlinkDeployment resources, and mutating their jobSpec. This enables the new FlinkDeployment to recover its state from the last checkpoint before cluster failover. For more discussion, see [discussion](https://github.com/karmada-io/karmada/issues/4969#issuecomment-2147171024).

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
    // +optional
    Cluster *ClusterFailoverBehavior `json:"cluster,omitempty"`
}

// ClusterFailoverBehavior indicates cluster failover behaviors.
type ClusterFailoverBehavior struct {
    // PurgeMode represents how to deal with the legacy applications on the
    // cluster from which the application is migrated.
    // Valid options are "Directly", "Gracefully".
    // Defaults to "Gracefully".
    // +kubebuilder:validation:Enum=Directly;Gracefully
    // +kubebuilder:default=Gracefully
    // +optional
    PurgeMode PurgeMode `json:"purgeMode,omitempty"`

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

In this proposal, we have uncommented the `Cluster` field in the `FailoverBehavior` struct to define behaviors related to cluster failover. We have also added the `StatePreservation` field to the `ClusterFailoverBehavior` struct, which is used to specify the policy for preserving and restoring state data of stateful applications during failover events. The type of this field is exactly the same as the `StatePreservation` field in the `ApplicationFailoverBehavior` struct.

In addition, the `PurgeMode` field has been added to the `FailoverBehavior` struct to specify how to migration applications from a cluster. Supported values for `PurgeMode` include `Directly` and `Gracefully`, with the default set to `Gracefully`. However, if the user does not specify the `.spec.failover.cluster` field, the migration strategy will be determined by the value of the `no-execute-taint-eviction-purge-mode` parameter in the `karmada-controller-manager` component.

The `ResourceBinding` API already meets the design goals and can directly reuse the `PreservedLabelState` and `ClustersBeforeFailover` fields in the `gracefulEvictionTask` without introducing new modifications.

#### Example

The following example demonstrates how to configure the state preservation policy using a Flink application:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: foo
spec:
  resourceSelectors:
    - apiVersion: flink.apache.org/v1beta1
      kind: FlinkDeployment
      name: foo
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
    spreadConstraints:
      - maxGroups: 1
        minGroups: 1
        spreadByField: cluster
  failover:
    cluster:
      purgeMode: Directly
      statePreservation:
        rules:
          - aliasLabelName: cluster.karmada.io/failover-jobid
            jsonPath: "{ .jobStatus.jobID }"

```

When a Flink application migration is triggered due to a cluster failure, the Karmada controller will, according to the above configuration, extract the jobID from the `.jobStatus.jobID` path in the Flink resource status of the failed cluster. Then, the controller will inject the jobID as a label into the Flink resource on the new cluster after migration, where the label key is `cluster.karmada.io/failover-jobid` and the value is the jobID.

#### Q&A

Q: Why do we use the same `StatePreservation` field in both `ClusterFailoverBehavior` and `ApplicationFailoverBehavior`?

A: Currently, defining `StatePreservation` in both places does seem somewhat redundant. However, we are also considering another point: we may not yet have sufficient scenarios to prove that the methods of obtaining information during application migration caused by cluster failures and application failures are exactly the same. [More question context](https://github.com/karmada-io/karmada/pull/6495#discussion_r2209788288).

### State Recovery

Key design points for state recovery are as follows:

1. State recovery is achieved by injecting the preserved state information as labels into the migrated new workload;
2. The timing of label injection is when the workload is migrated to a new cluster (to confirm the cluster is the result of new scheduling, the previous cluster scheduling information needs to be stored in `gracefulEvictionTask`). During the propagation of the workload, the controller injects the preserved state information into the manifest of the work object.

> Note: The controller logic for state recovery can fully reuse the modifications made previously for application failover to support stateful workloads. No new changes are required in this proposal.

It should be emphasized that when the application's migration mode `PurgeMode` is set to `Directly`, the Karmada system will first remove the application from the original cluster. Only after the application has been completely removed can it be scheduled to a new cluster. The current implementation does not yet meet this requirement and needs to be adjusted. State recovery can be performed after the application has been scheduled to the new cluster.

### Feature Gate

In Karmada v1.12, the `StatefulFailoverInjection` FeatureGate was introduced to control whether state recovery is performed during the recovery process of stateful applications after application failure. In this proposal, we extend the control of this feature gate to cover stateful application recovery scenarios after cluster failure, thereby unifying the control of stateful application failover recovery.

## System Failure Impact and Mitigation

None

## Graduation Criteria

### Alpha to Beta

- Decide whether to remove redundant fields from the `ApplicationFailoverBehavior` and `ClusterFailoverBehavior` structures;
- End-to-end (E2E) testing has been completed, ref [Test Planning](#test-planning);
- Feature Gate `StatefulFailoverInjection` enable by default;

### GA Graduation

- Scalability and performance testing;
- All konwn functional bugs have been fixe;
- Deprecated Feature Gate `StatefulFailoverInjection`;

## Notes/Constraints/Caveats

As mentioned above, this proposal focuses on scenarios where all replicas of a stateful application are migrated together. If users split the replicas of a stateful application and propagate them across clusters, and the system migrates only some replicas, the stateful application failover recovery feature may not work effectively.

## Risks and Mitigations

1. The cluster failover detection is not that accurate yet, and the detaction methods are relatively simple.
2. `Directly` purgeMode might be blocked if the connection between Karmada and the target cluster is broken.

## Impact Analysis on Related Features

No related features are affected at this time.

## Upgrade Impact Analysis

None

## Test Planning

UT: Add unit tests for the newly added functions
E2E: Add end-to-end test cases for stateful workload recovery after cluster failover.

### E2E Use Case Plnning

**Case 1:** A `Deployment` resource is propagated to member clusters via a `PropagationPolicy`. The `PropagationPolicy` resource is configured as follows:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: deploy-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: deploy
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
    spreadConstraints:
      - spreadByField: cluster
        maxGroups: 1
        minGroups: 1
  failover:
    cluster:
      purgeMode: Directly
      statePreservation:
        rules:
          - aliasLabelName: failover.karmada.io/replicas
            jsonPath: "{ .replicas }"
          - aliasLabelName: failover.karmada.io/readyReplicas
            jsonPath: "{ .readyReplicas }"
```

The `Deployment` resource will be propagated to one of the clusters. Then, a `NoExecute` taint will be added to that cluster to trigger the migration of the application. Check whether the target label exists on the migrated `Deployment` resource.

**Case 2:** A `StatefulSet` resource is propagated to member clusters via a `PropagationPolicy`. The `PropagationPolicy` resource is configured as follows:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: sts-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: StatefulSet
      name: sts
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
    spreadConstraints:
      - spreadByField: cluster
        maxGroups: 1
        minGroups: 1
  failover:
    cluster:
      purgeMode: Directly
      statePreservation:
        rules:
          - aliasLabelName: failover.karmada.io/replicas
            jsonPath: "{ .replicas }"
          - aliasLabelName: failover.karmada.io/readyReplicas
            jsonPath: "{ .readyReplicas }"
```

The `StatefulSet` resource will be propagated to one of the clusters. Then, a `NoExecute` taint will be added to that cluster to trigger the migration of the application. Check whether the target label exists on the migrated `StatefulSet` resource.

**Case 3:** A `FlinkDeployment` resource is propagated to member clusters via a `PropagationPolicy`. The `PropagationPolicy` resource is configured as follows:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: foo
spec:
  resourceSelectors:
    - apiVersion: flink.apache.org/v1beta1
      kind: FlinkDeployment
      name: foo
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
    spreadConstraints:
      - maxGroups: 1
        minGroups: 1
        spreadByField: cluster
  failover:
    cluster:
      purgeMode: Directly
      statePreservation:
        rules:
          - aliasLabelName: failover.karmada.io/failover-jobid
            jsonPath: "{ .jobStatus.jobID }"
```

The `FlinkDeployment` resource will be propagated to one of the clusters. Then, a `NoExecute` taint will be added to that cluster to trigger the migration of the application. Check whether the target label exists on the migrated `FlinkDeployment` resource.
