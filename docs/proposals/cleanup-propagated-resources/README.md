---
short-desc: Cleanup member cluster resources on unjoin
title: Cleanup Member Cluster Resources
authors:
  - "@huiwq1990"
reviewers:
  - "@RainbowMango"
approvers:
  - ""
creation-date: 2021-07-03
last-updated: 2020-07-03
status: provisional
---

# Cleanup propagated resources

## Table of Contents

* [Cleanup propagated resources](#cleanup-propagated-resources)
  * [Table of Contents](#table-of-contents)
  * [Motivation](#motivation)
    * [Goals](#goals)
  * [Proposals](#proposal)
    * [Implementation Details](#implementation-details)
      * [Best Effort Strategy](#best-effort-strategy)
      * [Required Strategy](#required-strategy)
    * [Addition ClusterConditionTypes (optional)](#additional-clusterconfigtypes-optional)
      * [Unjoining ClusterConfigurationType](#unjoining-clusterconfigurationtype)
      * [Failed ClusterConfigurationType](#failed-clusterconfigurationtype)

This document proposes a mechanism to specify that a member cluster resource should be removed from a managed cluster when leaving.

## Motivation

When a cluster is unjoined, Karmada should provide a mechanism to clean up the resources propagated by Karmada. Currently, when unjoining a cluster, `Karmada` first tries to remove the propagated resources, and will skip the removal if the cluster is not ready.  

### Goals

* Define how users can indicate that propagated resource should be removed when a cluster leaves
* Define cluster removal strategies

## Proposals

The Cluster struct should be updated to contain a `RemoveStrategy` member. `RemoveStrategy` now supports `Needless` and `Required`.

- The `Needless` strategy will not clean up any propagated resources.
- The `Required` strategy will halt the unjoin process and set the `Cluster resource` in a failed state when encountering errors. Unjoining is blocked until all propagated resources have been removed successfully.

`RemoveStrategy` defaults to `needless` on all cluster. A user must explicitly set a removal strategy for the joined cluster.


### Implementation details

* Update cluster to add a new attribute:
  ```
    RemoveStrategy *RemoveStrategy `json:"removeStrategy,omitempty"`
  ```
  
  ```
    type RemoveStrategy string
  
    const (
      RemoveStrategyNeedless   RemoveStrategy = "Needless"
      RemoveStrategyRequired   RemoveStrategy = "Required"
    )
  ```
  
* Add a flag `remove-strategy` to `karmadactl join` , and set the value to `RemoveStrategy` attribute

* During unjoin cluster, `karmada` should consider remove strategy



In kubefed's propose, the author suggest add a `BestEffort` strategy, reviewers say we need a certain value, so we should not use it too. `Karmada` use the `BestEffort` strategy, currently.

#### Needless Strategy

Not need cleanup propagated resources when unjoining cluster. `Karmada` should use this strategy as default value,  consider the business risk.

#### Required Strategy

Clusters with the "required" removal strategy must remove execution namespaces successfully on unjoin. After removing the work resource, the client must verify that the propagated resource is no longer present. If a resource cannot be removed:

* UnjoinCluster should return a formatted error containing which `work` could not be removed
* A ClusterCondition should be added to the `cluster` status with a message indicating which resources could not be removed
* (optional) Set the ConditionType to `Failed`

### Execution Controller

When cluster is unjoined, `karmada` should process by `cluster` remove strategy:

-  `Needless` strategy ignore `work` delete event
-  `Required` strategy return true if the `work` delete success, and set clustercondition if delete fail



### Additional ClusterConditionTypes (optional)

Presently, there are only two ClusterConditionTypes,  `Unjoining` and `UnjoinFailed`. Given the `required` remove strategy, a `Cluster` may end up in a state where the cluster is `Ready` but is in the process of "Unjoining". Adding additional conditions will make it easier to understand life cycle of cluster operator.

```yaml
status:
  conditions:
  - lastTransitionTime: "2021-05-08T12:34:15Z"
    lastUpdateTime: "2021-05-08T12:34:30Z"
    message: Unjoining resources.
    reason: UnjoinCluster
    status: "True"
    type: Unjoining
```



#### Unjoining ClusterConditionType

The `Unjoining` condition should be set on a cluster when exec  `karmadactl unjoin` command.

#### Failed ClusterConditionType

In the event of an error, such as failure to remove a resource with a `required` remove strategy, the cluster should enter a `UnjoinFailed` condition.

