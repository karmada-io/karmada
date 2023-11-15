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

creation-date: 2023-11-11
---

# Divide replicas by static weight evenly

## Summary

Karmada supports specifying the number of replicas to be allocated to each member cluster in a static weight ratio,
e.g., the user specifies that the total replicas of a Deployment is 5,
and the requirement is to distribute them into two clusters, member1 and member2, with a static weight of 1:1.

According to the current implementation, the result must be 3 replicas for member1 and 2 replicas for member2.
(5 is an odd number, so it can't be divided equally, and member1 allocates one more).

This is because for clusters with equal weights, the remainder prefers the cluster with the smaller dictionary order of cluster names 
in current implementation (the remainder is the number of replicas left over if the total replicas is not exactly divided by the static weights).

If there are many such Deployments, the total replicas of member1 cluster will be significantly higher than that of member2 cluster, 
i.e., a potential problem of uneven replicas distribution.

Therefore, in order to solve the uneven replicas allocation problem without affecting the scheduling inertia, 
I propose to optimize the sorting of clusters when allocating the remainder as:

* Sort the clusters first by weights
* If the weights are equal, then the clusters are sorted according to the current number of replicas
   (more current number of replicas implies that the remainders of the last scheduling were randomized to such clusters,
   and in order to keep the inertia in this scheduling, such clusters should also be prioritized).
* If the weights are equal and the current number of replicas is equal too, then the clusters will be randomized.


## Motivation

### Problem Summary

The essence of above case is that the "total replicas" is not divisible by the "sum weights", and a remainder is generated.

The remainder is preferred allocating to clusters with higher dictionary order of cluster name when they have equal weights,
which caused unevenness of replicas distribution.

Therefore, Karmada needs to do:

* The remainder should be assigned to clusters having equal weights with equal probability 
* Scheduler rescheduling should ensure inertia of the scheduling result

### Current realization

#### Example
```
sum replicas = 7，clusters = [member1, member2, member3]，weight = 2 : 1 : 1
```

#### Calculation Formula

```console
each cluster allocations = (total replicas * each cluster weight) / sum weights

remainders = total replicas - sum (each cluster allocations)
```

#### Detail Steps

* Calculate cluster allocations and remainders：

```
  member1 = 7 * 2 / 4 = 3
  member2 = 7 * 1 / 4 = 1
  member3 = 7 * 1 / 4 = 1
  
  so, remainders = 2
```

* Sort clusters: first by weight, then by dictionary order of cluster name (member1 > member2 > member3)
* Assign the remainders to the sorted clusters one by one (one for member1 and another for member2)
* Eventually: member1 = 4、member2 = 2、member3 = 1

### Expected goals

As in the above example, since member2 and member3 have equal weights, when assigning the remainder, it must be assigned to member2 first, 
which is not expected

We expect that it will be assigned to member2 and member3 with equal probability.


## Proposal

Since the unevenness of the current implementation is caused by sorting the clusters in dictionary order, 
the simplest way is changing it to directly randomize the ordering when the weights are equal.

However, it requires consideration of scheduling inertia, avoiding biased rescheduling results due to the introduction of randomization.

Therefore, to solve the problem of uneven replicas allocation, I propose to optimize the ordering of clusters when allocating remainders as:

1. Sort the clusters first by weights
2. If the weights are equal, then the clusters are sorted according to the current number of replicas
   (more current number of replicas implies that the remainders of the last scheduling were randomized to such clusters,
   and in order to keep the inertia in this scheduling, such clusters should also be prioritized).
3. If the weights are equal and the current number of replicas is equal too, then the clusters will be randomized.


### User Stories

#### Story 1：expanded replicas scenario

In expanding replicas scenario, we expect directly increasing replicas in several clusters,
avoiding different results calculated when rescheduling due to randomness, which leads to some clusters increasing replicas, while others reducing replica.

Yet this proposal can avoid this problem:

```
sum replicas = 7，clusters = [member1, member2, member3, member4]，weight = 2 : 1 : 1 : 1

1、Calculation：【each cluster allocations】= 2、1、1、1，【remainders】= 2          // 7*2/5=2、7*1/5=1
2、Sorting：member1 > member2 = member3 = member4
3、One possible result：3、2、1、1

Supposing sum replicas change from 7 to 8

1、Calculation：【each cluster allocations】= 3、1、1、1，【remainders】= 2          // 8*2/5=3、8*1/5=1
2、Sorting：member1 > member2 > member3 = member4                                 // member2 has more current replicas than member3 / member4
3、Result：4、2、1、1  (results ensured inertia)
```

#### Story 2：reduced replicas scenario

In reducing replicas scenario, we expect directly reducing replicas in several clusters,
avoiding different results calculated when rescheduling due to randomness, which leads to some clusters increasing replicas, while others reducing replica.

Yet this proposal can avoid this problem:

```
sum replicas = 9，clusters = [member1, member2, member3, member4]，weight = 2 : 1 : 1 : 1

1、Calculation：【each cluster allocations】= 3、1、1、1，【remainders】= 3         // 9*2/5=3、9*1/5=1
2、Sorting：member1 > member2 = member3 = member4
3、One possible result：4、2、2、1

Supposing sum replicas change from 9 to 8

1、Calculation：【each cluster allocations】= 3、1、1、1，【remainders】= 2         // 8*2/5=3、8*1/5=1
2、Sorting：member1 > member2 = member3 > member4                                // member2 and member3 has more current replicas than member4
3、Result：4、2、1、1 or 4、1、2、1    (results ensured inertia)

```

#### Story 3：modifying Weight Scenarios

Modifying the weight scenario generally involves increasing or decreasing each cluster replicas, rescheduling can be directly recalculated.

However, assuming that the weights are adjusted and the sum replicas is also adjusted, it is important to ensure inertia as much as possible.

```
sum replicas = 6，clusters = [member1, member2, member3, member4]，weight = 1 : 1 : 1 : 1

1、Calculation：【each cluster allocations】= 1、1、1、1，【remainders】= 2         // 6*1/4=1
2、Sorting：member1 = member2 = member3 = member4
3、One possible result：2、1、2、1

1）Supposing weight change to 2 : 1 : 1 : 1

1、Calculation：【each cluster allocations】= 2、1、1、1，【remainders】= 1         // 6*2/5=2、6*1/5=1
2、Sorting：member1 > member3 > member2 = member4                                // member3 has more current replicas than member2 / member4
3、Result：3 : 1 : 1 : 1 (Minimal adjustments based on weights)

2）Supposing weight change to 2 : 1 : 1 : 1, and sum replicas change to 7

1、Calculation：【each cluster allocations】= 2、1、1、1，【remainders】= 2         // 7*2/5=2、7*1/5=1
2、Sorting：member1 > member3 > member2 = member4                                // member3 has more current replicas than member2 / member4
3、Result：3 : 1 : 2 : 1  (results ensured inertia)
```

#### Story 4：expanded clusters scenario

```
sum replicas = 5，clusters = [member1, member2, member3]，weight = 1 : 1 : 1

1、Calculation：【each cluster allocations】= 1、1、1，【remainders】= 2           // 5*1/3=1
2、Sorting：member1 = member2 = member3
3、One possible result：2、1、2

Supposing after clusters expanded，clusters = [member1, member2, member3, member4]，weight = 1 : 1 : 1 : 1

1、Calculation：【each cluster allocations】= 1、1、1、1，【remainders】= 1        // 5*1/4=1
2、Sorting：member1 = member3 > member2 > member4
3、Result：1、1、2、1 or 2、1、1、1 (equivalent to move one replica from member1/member3 to member4, others keep unchanged)
```

#### Story 5：reduced clusters scenario

```
sum replicas = 5，clusters = [member1, member2, member3, member4]，weight = 1 : 1 : 1 : 1

1、Calculation：【each cluster allocations】= 1、1、1、1，【remainders】= 1        // 5*1/4=1
2、Sorting：member1 = member2 = member3 = member4
3、One possible result：2、1、1、1

Supposing after clusters reduced，clusters = [member1, member2, member3]，weight = 1 : 1 : 1

1、Calculation：【each cluster allocations】= 1、1、1，【remainders】= 2           // 5*1/3=1
2、Sorting：member1 > member2 = member3
3、Result：2、2、1 or 2、1、2 (equivalent to move one replica from member4 to member1/member3, others keep unchanged)
```
