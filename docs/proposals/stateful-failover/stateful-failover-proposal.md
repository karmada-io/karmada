---
title: Stateful Failover Support in Karmada
authors:
  - "@Dyex719"
  - "@mszacillo"
reviewers:
- "@RainbowMango"
- "@XiShanYongYe-Chang"
- "@zhzhuang-zju"
approvers:
- "@RainbowMango"

create-date: 2024-06-28

---

# Stateful Failover Support in Karmada

Special thanks to `@RainbowMango` and `@XiShanYongYe-Chang` for their help with the API design!

## Summary

Karmada can be currently used to intelligently schedule all types of resources (both generic Kubernetes objects as well as user-applied CRDs). It is particularly useful for ensuring stateful application resilience in a multi-cluster environment in which applications may be rescheduled if a cluster becomes healthy.

However, Karmada’s scheduling logic runs on the assumption that resources that are scheduled and rescheduled are stateless. In some cases, users may desire to conserve a certain state so that applications can resume from where they left off in the previous cluster.

For CRDs dealing with data-processing (such as Flink or Spark), it can be particularly useful to restart applications from a previous checkpoint. That way applications can seamlessly resume processing data while avoiding double processing.

This proposal would like to introduce a more generalized way for users to define application state preservation in the context of cluster to cluster failovers.

## Motivation

This proposal aims to provide a framework for stateful CRDs to keep track of the state required during failover so that processing can be resumed from that state after failover has completed.

## Proposal

Karmada introduces a higher level of resiliency to kubernetes-deployed applications through the use of its failover feature. However, this failover feature does not currently account for state. Stateless applications have the benefit of being more fault-tolerant, as the loss of an application will not impact user sessions. For stateful applications however (such as Flink), the loss of the application can will result in loss of session data that can prevent the application from recovering correctly.

This proposal introduces a way for users to persist state required for their application in the event of a failover. That way, once rescheduled, users can configure their applications to resume using the metadata saved by Karmada. Since stateful applications have different implementations of how to retrieve the last state, it is important that this feature is configurable.

To support this feature, we will need at minimum:
- A mechanism with which stateful applications can preserve state during failover and reload before resuming.
- A way users can configure which state will be preserved by the Karmada
- Flags that will denote when a resource has been failed over by Karmada

`Note`: One important detail is that if all the replicas of the stateful application are not migrated together, it is not clear when the state needs to be restored. In this proposal we focus on the use case where all the replicas of a stateful application are migrated together. One way to ensure this is to make all the replicas scheduled together using spreadConstraints.

```yaml
spreadConstraints:
  - spreadByField: cluster
    maxGroups: 1
    minGroups: 1
```

### High Level Goals

The proposal can be logically separated into three different efforts:
1. Extension of the ResourceBindingStatus API to include FailoverHistory field, FailoverHistory implementation
2. Addition of flags which mark resources when they failover
3. Extension of the PropagationPolicy API to include state preservation rules, which define which state will be conserved during failover


### Use-case with Flink:

In Flink for example, checkpoints are snapshots that store the state of the application till a particular moment in time. As a result they contain information about all the records processed till that moment. This information is collected and persisted continuously to some persistent storage at specific intervals.

In Flink, to retrieve the last state from the persistent store, we would require metadata about the job being restored (Job ID) along with the path where the state is being persisted.

The Flink Operator can then use this information to retrieve the last state by providing a few modifications to the Flink CRD spec.

```
Spec:
Flink Configuration:
	initialSavepointPath: “desired checkpoint path to be resumed from (s3p://)”
upgradeMode: savepoint
state: running
```

We currently use a mutating webhook (Kyverno) along with a custom microservice to retrieve the last state from the persistent store and mutate the CRD spec to to include the above details.

We believe that Karmada would benefit from having a generic way to store the job metadata/information required for failover and a label/annotation to indicate that failover has happened so that this can be extended to any stateful application.

To summarize, we currently resume from the last state with some custom changes and third party services which:
 1. Append a flag to the ResourceBinding of the CRD to indicate a failover. When the application is rescheduled, a label is appended to the CRD to mark it as failed-over.
 2. A custom Kyverno policy then reads the failover label and grabs the latest checkpoint for the application based on its jobID.
 3. The application can then resume from the fetched checkpoint and recommence processing.

## Stateful Failover API Design

### ResourceBindingStatus API

We can extend existing ResourceBindingStatus API with a new field "FailoverHistory", which would be added by the cluster + application failover controller to keep track when an application has been failed-over.

```go
// ResourceBindingStatus represents the overall status of the strategy as well as the referenced resources.
type ResourceBindingStatus struct {
	// SchedulerObservedGeneration is the generation(.metadata.generation) observed by the scheduler.
	// If SchedulerObservedGeneration is less than the generation in metadata means the scheduler hasn't confirmed
	// the scheduling result or hasn't done the schedule yet.
	// +optional
	SchedulerObservedGeneration int64 `json:"schedulerObservedGeneration,omitempty"`

	// SchedulerObservedAffinityName is the name of affinity term that is
	// the basis of current scheduling.
	// +optional
	SchedulerObservedAffinityName string `json:"schedulerObservingAffinityName,omitempty"`

	// Conditions contain the different condition statuses.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// AggregatedStatus represents status list of the resource running in each member cluster.
	// +optional
	AggregatedStatus []AggregatedStatusItem `json:"aggregatedStatus,omitempty"`

    // FailoverHistory represents history of the previous failovers of this resource
	FailoverHistory []FailoverHistoryItem `json:"failoverHistory,omitempty"`

}
```

### FailoverHistoryItem API

The FailoverHistory is a list of FailoverHistoryItem objects and is updated every time a failover happens. The amount of items can be configured up to a certain limit, as to minimize excess status length. This limit is set by another field persistedFields.maxHistory which is defined in the propogation policy.

```go
// FailoverHistoryItem represents either a failover event in the history.
type FailoverHistoryItem struct {
	// Reason denotes the type of failover.
	// +required
	Reason FailoverReason `json:"reason"`

	// StartTime is the timestamp of when the failover occurred.
	// +required
	StartTime metav1.Time `json:"failoverTime"`

    // FromCluster is the cluster name from which application was migrated.
	// +required
	FromCluster string `json:"fromCluster"`

	// ClustersBeforeFailover records the clusters where the application was running prior to failover.
	// +required
	ClusterBeforeFailover []string `json:"originalCluster"`

	// ClustersAfterFailover records the clusters where the application is running after failover.
	// +optional
	ClusterAfterFailover []string `json:"targetCluster,omitempty"`

	// PreservedLabelState represents the application state information collected from the original cluster,
	// and it will be injected into the new cluster in the form of application labels.
	// +optional
	PreservedLabelState map[string]string `json:"preservedLabelState,omitempty"`
}

// FailoverReason represents the reason for the failover.
type FailoverReason string

const (
	// ClusterFailover represents the failover is due to cluster issues.
	ClusterFailover FailoverReason = "ClusterFailover"

	// ApplicationFailover represents the failover is due to application issues.
	ApplicationFailover FailoverReason = "ApplicationFailover" // Failover due to application issues, handled by health interpretation.
)
```

The FailoverHistoryItem object contains information relevant to a failover in addition to the `PreservedLabelState` which will be configured by the tenant within their PropagationPolicy.

### PropagationPolicy API

We propose to add two fields to the propogation policy to enable stateful failover:

1. `ApplicationFailoverBehavior.MaxHistory`: This sets the max limit on the amount of stateful failover history that is persisted before the older entries are overwritten. If this is set to 5, the resourcebinding will store a maximum of 5 failover entries in FailoverHistory before it overwrites the older history.
2. `ApplicationFailoverBehavior.StatePreservation`: This is a list of the fields that are required by the stateful application to be persisted during failover to resume processing from a particular case. This takes in a list of field names as well as how to access them from the spec.

```go
// ApplicationFailoverBehavior indicates application failover behaviors.
type ApplicationFailoverBehavior struct {
	// FailoverHistoryItems will automatically be appended to the ResourceBindingStatus.
	// If the default setting of 5 is too excessive, the max history can be configured here.
	//
	// +optional
	MaxHistory int64 `json:"maxHistory,omitempty"`

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

	// Note: We probably need more policies to control how to feed the new cluster with the
	// preserved state data in the future. Such as:
	// - Is it always acceptable to feed data as the label? Is there a need for annotation?
	// - If each label name should be started with a prefix, like `karmada.io/failover-preserving-<fieldname>: <state>`
	// Sure, we can default with the label, this structure just makes room for future extensions.
	//
	// For instance, we can introduce a policy if someone wants to control how the preserving state
	// feed to new clusters. This is probably not included this time.
	// RestorePolicy determines when and how the preserved state should be restored.
	// RestorePolicy RestorePolicy `json:"restorePolicy"`
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

Example propagation policy for Flink jobs that uses the persistedFields.maxHistory and persistedFields.fields:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: flink-deployment-policy
spec:
  failover:
    application:
      decisionConditions:
        tolerationSeconds: 90
      maxHistory: 5
      purgeMode: Immediately
      statePreservation:
        rules:
          - aliasLabelName: jobId
            jsonPath: "obj.status.jobStatus.jobID"
  resourceSelectors:
    - apiVersion: flink.apache.org/v1beta1
      kind: FlinkDeployment
      namespace: example-namespace
  propagateDeps: true
  placement:
    replicaScheduling:
      replicaDivisionPreference: Aggregated
      replicaSchedulingType: Divided
    spreadConstraints:
      - spreadByField: cluster
        maxGroups: 1
        minGroups: 1
```
