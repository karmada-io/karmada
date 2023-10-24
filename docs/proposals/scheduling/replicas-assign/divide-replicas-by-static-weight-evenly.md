---
title: divide replicas by static weight evenly
authors:
- "@chaosi-zju"
reviewers:
- "@RainbowMango"
- "@XiShanYongYe-Chang"
- "@zhzhuang-zju"
approvers:
- "@RainbowMango"

creation-date: 2023-10-19
---

# Divide replicas by static weight evenly
## Summary


* Assign residual replicas to clusters randomly when using static weight dividing strategy.
* Add last schedule result and weight to ResourceBinding (clusters/replicas already exist, last weight should been added).
* Calculate the random assignment based on the last schedule result and weight.

In following proposal, I will first introduce why and how to divide replicas by static weight evenly,
then I will give you a multiple examples on how to divide replicas, and then summarizing the rules from the stories.
Eventually, the design detail will be easier to realize basing on the rules we summarized, instead of directly listing user stories.

## Motivation

### What's the problem?

Supposing there are two deployments using static weight dividing strategy, each deployment's replicas
is odd number, e.g: 3. 

Yet, if their static weight are all `member1:member2 = 1:1`, then member1 cluster
will eventually be assigned 4 replicas in sum, member2 cluster will eventually be assigned 2 replicas in sum, the replicas
is assigned not evenly. 

In a word, under static weights, the residual are not distributed evenly enough, 
leading to uneven use of resources among member clusters.

### Current realization

Example: 
```    
sum-replicas = 7, clusters = [ member1, member2 ], weight = 1:1
```

Calculation:

* Step1：
```  
member1 replicas = (member1 weight / all weight) * replicas = (1/2) * 7 = 3
member2 replicas = (member2 weight / all weight) * replicas = (1/2) * 7 = 3
residual replicas = 1
```

* Step 2: sort clusters (firstly by weight, then by cluster name)
* Step 3: assign residual replicas to clusters, each cluster one replicas by order (so, 1 residual replicas will
  assign to member1)
* Step 4: member1 replicas = 4, member2 replicas = 3

### Expected goals

**Goal 1：** 

change above `Step 3` to "assign residual replicas to clusters randomly, so that the residual replicas has the same probability of 
being assigned to each member cluster".

However, this may result in a new problem: if user updated the PropagationPolicy of the deployment, sometimes
we don't want to change the replicas distribution. For example, user add a label to PropagationPolicy or 
change cluster weight from `1:1` to `2:2`, in this case there is no need to reschedule to re-trigger 
"assign residual replicas to clusters randomly".

So, here comes **Goal 2:** 

"avoid unnecessary assign of random replicas when rescheduling".

## Proposal

Assign residual replicas divided by static weight to clusters randomly.

### User Story

Assuming the ：

```    
sum-replicas = 6, clusters = [ member1, member2, member3, member4 ], weight = 1:1:1:1
```

If this random assignment is introduced, the steps of replicas assignment are：

```
member1 = 1, memebr2 = 1, member3 = 1, member4 = 1 (divide by static weight)
->
the remain 2 replicas can random to any two clusters
->
member1 = 2, memebr2 = 1, member3 = 2, member4 = 1 (one random result)
```

considering what's the expected result after different
modification of PropagationPolicy.

#### Example 1

**User behavior**: update PropagationPolicy but keep `placement` field unchanged (i.e: add a label to PropagationPolicy)

**Excepted result**: the sum-replicas/clusters/weight unchanged, so keep assignment unchanged (no re-random)

#### Example 2

**User behavior**: last PropagationPolicy is Duplicated mode or DynamicWeight mode.

**Excepted result**: directly re-schedule, re-random.

#### Example 3.1

**User behavior**: scale replicas down, from 6 to 2.

**Excepted result**: directly re-schedule, re-random.
```
member1 = 0, member2 = 0, member3 = 0, member4 = 0 (divide by static weight, changed)
->
the remain 2 replicas may random to any two clusters
->
member1 = 0, member2 = 0, member3 = 1, member4 = 1 (one random result)
```

#### Example 3.2

**User behavior**: scale replicas down, from 6 to 5.

**Excepted result**: re-schedule, re-random should base on previous random result.
```
member1 = 1, member2 = 1, member3 = 1, member4 = 1 (divide by static weight, unchanged)
->
the remain 1 replicas should random to member1 or member3, basing on previous assignment.
->
member1 = 1, member2 = 1, member3 = 2, member4 = 1 (one random result)
```

#### Example 3.3

**User behavior**: scale replicas up, from 6 to 7.

**Excepted result**: re-schedule, re-random should base on previous random result.
```
member1 = 1, member2 = 1, member3 = 1, member4 = 1 (divide by static weight, unchanged)
->
remain 3 replicas, 2 of them should random to member1 and member3, 
the rest 1 should random to member2 or member4, basing on previous assignment.
->
member1 = 2, member2 = 1, member3 = 2, member4 = 2 (one random result)
```

#### Example 3.4

**User behavior**: scale replicas up, from 6 to 9.

**Excepted result**: directly re-schedule、re-random.
```
member1 = 2, member2 = 2, member3 = 2, member4 = 2 (divide by static weight, changed)
->
the remain 1 replicas may random to any cluster
->
member1 = 3, member2 = 2, member3 = 2, member4 = 2 (one random result)
```

#### Example 4

**User behavior**: update weight to `2:1:1:1`.

**Excepted result**: directly re-schedule、re-random.
```
member1 = 2, member2 = 1, member3 = 1, member4 = 1 (divide by static weight, changed)
->
the remain 1 replicas may random to any cluster
->
member1 = 2, member2 = 2, member3 = 1, member4 = 1 (one random result)
```

> Question: whether it is required the remain 1 replicas been assigned to member3 ?

#### Example 5

**User behavior**: clusters changed, target clusters are `member1:member2:member3 = 1:1:1`.

**Excepted result**: directly re-schedule、re-random.
```
member1 = 2, member2 = 2, member3 = 2 (divide by static weight, changed)
```

### Proposal Summary

From the above stories we can summarize the following rules：

* Given previous schedule result and weight, we can infer how to assign residual replicas to clusters randomly.
* If the result of replicas "divide by static weight" been changed, we need just re-schedule and re-random directly.
* If unchanged, we should firstly infer the last random result, and re-random based on the last result.

So, I propose:

* Assign residual replicas to clusters randomly when using static weight dividing strategy.
* Add last schedule result and weight to ResourceBinding (clusters/replicas already exist, last weight should been added).
* Calculate the random assignment based on the last schedule result and weight.

### Risks and Mitigations

none

## Design Details

### API Modify

```go
type ScheduleResult struct {
    // ReplicaScheduling represents the scheduling policy on dealing with the number of replicas
    // when propagating resources that have replicas in spec (e.g. deployments, statefulsets) to member clusters.
    // +optional
    ReplicaScheduling *policyv1alpha1.ReplicaSchedulingStrategy `json:"replicaScheduling,omitempty"`
    // Clusters represents target member clusters where the resource to be deployed.
    // +optional
    Clusters []TargetCluster `json:"clusters,omitempty"`
}

// ResourceBindingStatus represents the overall status of the strategy as well as the referenced resources.
type ResourceBindingStatus struct {
    ...
    
    // LastScheduleResult
    // +optional
    LastScheduleResult *ScheduleResult `json:"lastScheduleResult,omitempty"`
  
    ...
}
```

### Calculation Algorithm

As for static weight schedule, we just need sum-replicas/clusters/weight info, define below struct:
```go
type scheduleResult struct {
    SumReplicas int32
    Clusters map[string]struct {
        weight int32
        replicas int32
    }
}
```

1. build last `scheduleResult` from `ResourceBinding.Status.LastScheduleResult` + `ResourceBinding.Spec.Replicas`
2. build this schedule request from `ResourceBinding.Spec.Placement.ReplicaScheduling` + `ResourceBinding.Spec.Replicas`
3. whether last replicas assignment fit this schedule request. (detail algorithm @zhzhuzang-zju)
4. if last schedule strategy is not static weight, directly re-schedule:
   * each_member_replicas = (member1 weight / all weight) * sum-replicas
   * residual replicas random to any clusters
5. 

### Test Plan

refer to the above stories.