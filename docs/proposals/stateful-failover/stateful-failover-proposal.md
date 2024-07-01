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

## Summary

Karmada can be currently used to intelligently schedule all types of resources (both generic Kubernetes objects as well as user-applied CRDs). It is particularly useful for ensuring stateful application resilience in a multi-cluster environment in which applications may be rescheduled if a cluster becomes healthy.

However, Karmada’s scheduling logic runs on the assumption that resources that are scheduled and rescheduled are stateless. In some cases, users may desire to conserve a certain state so that applications can resume from where they left off in the previous cluster. 

For CRDs dealing with data-processing (such as Flink or Spark), it can be particularly useful to restart applications from a previous checkpoint. That way applications can seamlessly resume processing data while avoiding double processing. 

This proposal would like to introduce a more generalized way for users to define failover actions, which can be used in the case of stateful CRD failover.


## Motivation

This proposal aims to provide a framework for stateful CRDs to keep track of the state required during failover so that processing can be resumed from that state after failover has completed.

### Goals

To add support for stateful application failover:
- Extend ResourceBinding API to include FailoverHistory field
- Extend PropagationPolicy API to include status items that should be persisted during failover

## Proposal

Stateful applications need a way to read the last saved state to resume processing from that state after failover. 

To enable this we split this proposal into two sections: 
- A mechanism for when a failover happened so that stateful applications can load the previous state first before resuming processing.
- At the very minimum, a way of storing and reading state/metadata related to the job being failed over.

Since stateful applications have different implementations of how to retrieve the last state given this the job metadata we would then rely on those individual implementations to fetch all the details related to the last state. 

**NOTE: To be discussed with Karmada community**

One important detail is that if all the replicas of the stateful application are not migrated together, it is not clear when the state needs to be restored. In this proposal we focus on the use case where all the replicas of a stateful application are migrated together. One way to ensure this is to make all the replicas scheduled together using spreadConstraints.

```
spreadConstraints:
  - spreadByField: cluster
    maxGroups: 1
    minGroups: 1
```


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

## Design Details

### Support Stateful Failover Options

#### ResourceBinding API Change

We can extend the ResourceBindingStatus with a new field "FailoverHistory", which would be added by the cluster + application failover controller to keep track when the replica has been failed-over. 

```
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

The FailoverHistory is a list of FailoverHistoryItem objects and is updated every time a failover happens until a certain limit. The limit is set by another field persistedFields.maxHistory which is defined in the propogation policy.

```
type FailoverHistoryItem struct {

    // FailoverTime represents the timestamp when the workload failed over
    // It is represented in RFC3339 form(like '2021-04-25T10:02:10Z') and is in UTC
	FailoverTime *metav1.Time `json:"failoverTime,omitempty"`

    // OriginCluster represents the cluster name from which the workload failed over from
	OriginCluster string `json:"originCluster,omitempty"`

    // DestinationCluster represents the cluster name from which the workload failed over to
	DestinationCluster string `json:"destinationCluster,omitempty"`

    // PersistedDuringFailover contains the fields required by the stateful application to resume from that state after failover
	PersistedDuringFailover []PersistedFailoverItem `json:"persistedFailoverItem,omitempty"` 
}
```

The FailoverHistoryItem object contains information relevant to a failover and an additional object called "PersistedDuringFailover" which keeps track of the metadata (both the fields that need to be persisted and how to access those fields) that is required by the stateful operation to resume processing from that state.

```
type PersistedDuringFailover struct {
	
    // LabelName represents the name of the line that will be persisted for the replica
    // in case there is a failover to a new cluster.
	LabelName string `json:"labelName,omitempty"`

    // PersistedItem is a pointer to the status item that should be persisted to the rescheduled 
    // replica during a failover. This should be input in the form: obj.status.<path-to-item> 
	PersistedStatusItem string `json:"persistedStatusItem,omitempty"`
}
```

The PersistedDuringFailover object keeps track of the value of a field during failover so that this information can be used to resume processing from this point onward. This object consists of two fields, LabelName and PersistedStatusItem which are both defined in the propogation policy.

#### PropagationPolicy API Change

We propose to add two fields to a propogation policy to enable stateful failover.
1. persistedFields.maxHistory: This sets the max limit on the amount of stateful failover history that is persisted before the older entries are overwritten. If this is set to 5, the resourcebinding will store a maximum of 5 failover entries in FailoverHistory before it overwrites the older history.
2. persistedFields.fields: This is a list of the fields that are required by the stateful application to be persisted during failover to resume processing from a particular case. This takes in a list of field names as well as how to access them from the spec. 

Example propagation policy for Flink jobs that uses the persistedFields.maxHistory and persistedFields.fields:

```
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: flinkdep-policy
spec:
  failover:
    application:
      decisionConditions:
        tolerationSeconds: 90
      purgeMode: Graciously
      gracePeriodSeconds: 10
    persistedFields.maxHistory: 5
    persistedFields.fields:
      - LabelName: jobID
        PersistedStatusItem: obj.status.jobStatus.jobID
  resourceSelectors:
    - apiVersion: flink.apache.org/v1beta1
      kind: FlinkDeployment
      namespace: <>
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
