---
title: Statization of Propagation Policy
authors:
  - "@chaosi-zju"
reviewers:
  - "@RainbowMango"
approvers:
  - "@RainbowMango"

creation-date: 2023-11-17
---

# Statization of Propagation Policy

## Summary

In this paper, we propose an improvement to the Policy enforcement mechanism, called Policy Staticization.
Before the introduction of this feature, modification of a Policy would immediately affect the ResourceTemplate it manages, 
potentially causing a huge shock to the system, which is too sensitive.

We want Policy to be just some API-based static system strategies. Modification of the Policy should not actively cause changes to the system behavior, 
but should be triggered by ResourceTemplate. Just like if a ResourceTemplate has already been propagated by a Policy, don't change its states for any Policy changes,
but you can re-propagate it by latest Policy when ResourceTemplate itself updated.

So, I propose to staticize the PropagationPolicy/ClusterPropagationPolicy as:

* If a ResourceTemplate is not managed by a Policy, when there is a matching Policy in the system,
  the ResourceTemplate will be distributed according to the Policy with the highest priority.
* If the ResourceTemplate is already managed by a Policy, the distribution status of the Resource Template remains unchanged,
  regardless of whether the Policy has been modified or a higher-priority Policy applied.
* When a user submits a change to a ResourceTemplate, if there is a matchable higher priority Policy in the system,
  the ResourceTemplate will follow that Policy to adjust its distribution states; otherwise, if the previous matching Policy is modified,
  the ResourceTemplate will also follow the modified Policy to adjust the distribution states.

## Motivation

In order to facilitate a deeper understanding of the motivation, first take you into the real business scenario,
there are two roles in this scenario:

* Normal user: they have no knowledge of Karmada and don't want to learn how to use Karmada, 
  and is only responsible for deploying resources of their own application in a Namespace.
* Cluster administrator: a person who is familiar with Karmada and is responsible for maintaining Karmada CRD resources such as ClusterPropagationPolicy.

The cluster administrator does not know which users, which namespaces, and which resources will be available in the future, 
so he creates a global default ClusterPropagationPolicy to manage all resource templates.

As normal users come on board, many resources are already deployed in the cluster federation.

At this point, the cluster administrator has the following two types of requirements:

* He needs to change the global ClusterPropagationPolicy in the future, for example, to add a new distribution cluster for all the resources of all the users, 
  but he is afraid that the update of the ClusterPropagationPolicy will lead to a large number of workloads restarting, 
  and even the restarting will fail to affect the user's business, which is a high operational risk. 
  He would like to avoid the risk as a cluster administrator as much as possible.
* In the future, there may be individual users who need a customized distribution policy, for example, 
  if the global ClusterPropagationPolicy is configured for dual-cluster distribution, but an individual user's resources need triple-cluster distribution, 
  the administrator would like to have a way to individually configure a customized distribution policy for the individual user.


```mermaid
graph LR
A(fa:fa-user-cog Cluster Administrator) --> B[Create Global Default \n ClusterPropagationPolicy]
B --> C{Change\nRequirements}
C --> D[Update Default \n ClusterPropagationPolicy]
C --> E[Customize Another \n ClusterPropagationPolicy]
D --> F{{Demand: don't affect existing resource immediately}}
E --> G{{Demand: take over batch of resources from Default CPP}}
```

## Proposal

The current Policy mechanism has some limitations, we need to optimize the behavior of Policy.

### Guideline

Any workload is running fine, don't roughly change them!

### Base Principle

* PropagationPolicy / ClusterPropagationPolicy are just a bunch of API-based system functions, shouldn't actively cause any existing workload distribution changes in the system.
* Any destructive changes should be triggered from the ResourceTemplate.
* After ResourceTemplate changed, there must be a matching Policy for subsequent propagation. If there is no matching policy, it should be pending.

> When ambiguity is encountered, clarification should be based on the perspective of what kind of system behavior is easier for users to understand
> and what kind of operational results are more acceptable to users.

### Detail Proposal

Based on the above design principles, it is proposed to modify the default behavior of Policy:

* If a ResourceTemplate is not managed by a Policy, when there is a matching Policy in the system, 
  the ResourceTemplate will be distributed according to the Policy with the highest priority.
* If the ResourceTemplate is already managed by a Policy, the distribution status of the Resource Template remains unchanged, 
  regardless of whether the Policy has been modified or a higher-priority Policy applied.
* When a user submits a change to a ResourceTemplate, if there is a matchable higher priority Policy in the system, 
  the ResourceTemplate will follow the Policy to adjust its distribution states; otherwise, if the previous matching Policy is modified, 
  the ResourceTemplate will also follow the modified Policy to adjust the distribution states.

After applying this scheme, the two problems mentioned in the above background can be solved:
* For the problem of high risk of modifying the global policy, the modification of the policy no longer actively triggers the state change of the ResourceTemplate, 
  and the ResourceTemplate will follow the modified Policy Adjustment Distribution Policy only after the user submits changes to the ResourceTemplate.
* For the problem that individual users need to customize the distribution policy, individual users can customize a higher-priority Policy, 
  and then submit changes to the relevant ResourceTemplate, and the ResourceTemplate will follow the higher-priority Policy to adjust the distribution states.

### User Stories

Cluster administrators would prefer to configure a global ClusterPropagationPolicy (naming default-cpp) as the default propagation policy applied by all users in certain usage scenarios, for example:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterPropagationPolicy
metadata:
  name: default-cpp
spec:
  placement:
    clusterAffinity:
      clusterNames:
      - member1
      - member2
  resourceSelectors:
  - apiVersion: apps/v1
    kind: Deployment
```

#### Story 1

If the cluster administrator wants to modify this global ClusterPropagationPolicy (naming default-cpp),
for example to add a new distribution cluster, as shown below:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterPropagationPolicy
metadata:
  name: default-cpp
spec:
  placement:
    clusterAffinity:
      clusterNames:
      - member1
      - member2
      - member3     # member3 is added as a new distribution cluster
  resourceSelectors: 
  - apiVersion: apps/v1
    kind: Deployment
```

Cluster administrators can submit the change directly, and the ClusterPropagationPolicy change will not be actively applied to the resource template, 
and the new Policy will take effect only after the user submits the change to the ResourceTemplate.

In this way, there is more flexibility and control.

#### Story 2

Individual users who need a customized propagation policy can be helped by the cluster administrator to create a higher priority ClusterPropagationPolicy (naming custom-cpp) as shown below:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterPropagationPolicy
metadata:
  name: custom-cpp
spec:
  placement:
    clusterAffinity:
      clusterNames:
      - member1
      - member2
      - member4      # member4 is added as a new distribution cluster
  priority: 100
  resourceSelectors: 
  - apiVersion: apps/v1
    kind: Deployment
    namespace: user-ucs
```

As you see, this policy has a higher priority (`priority: 100`), and it will take effect when a user submits a change to the corresponding ResourceTemplate.

Karmada might also consider providing cluster administrators with the ability to batch refresh ResourceTemplates via karmadactl.

### General Case

#### Case 1

1. create ResourceTemplate DeployA
2. create PropagationPolicy Policy1 (match to DeployA)

```mermaid
sequenceDiagram
  participant DeployA
  participant Policy1
  participant Karmada

  
  DeployA ->> Karmada: create
  activate Karmada
  Policy1 ->> Karmada: create
  Karmada -->> DeployA: propagate it by Policy1
  deactivate Karmada
```

> Note: DeployA is constantly waiting for a matchable Policy, so when Policy1 applied, DeployA would be directly propagated
> by Policy1 without an additional updates.

#### Case 2

1. create PropagationPolicy Policy1 (match to DeployA)
2. create ResourceTemplate DeployA
3. update Policy1
4. update DeployA

```mermaid
sequenceDiagram
  participant DeployA
  participant Policy1
  participant Karmada

  Policy1 ->> Karmada: create
  activate Karmada
  DeployA ->> Karmada: create
  Karmada -->> DeployA: propagate it by Policy1
  deactivate Karmada
  
  Policy1 ->> Karmada: update
  activate Karmada
  loop
    Policy1 --> Karmada: no action util DeployA updated
  end
  deactivate Karmada

  DeployA ->> Karmada: update
  activate Karmada
  Karmada -->> DeployA: propagate it by Policy1
  deactivate Karmada

```

#### Case 3

1. create PropagationPolicy Policy1 (priority=1, match to DeployA)
2. create ResourceTemplate DeployA
3. create Policy2 (priority=2, match to DeployA too)
4. update DeployA

```mermaid
sequenceDiagram
  participant DeployA
  participant Policy1
  participant Policy2
  participant Karmada

  Policy1 ->> Karmada: create (priority=1)
  activate Karmada
  DeployA ->> Karmada: create
  Karmada -->> DeployA: propagate it by Policy1
  deactivate Karmada
  
  Policy2 ->> Karmada: create (priority=2)
  activate Karmada
  loop
    Policy2 --> Karmada: no action util DeployA updated
  end
  deactivate Karmada

  DeployA ->> Karmada: update
  activate Karmada
  Karmada -->> DeployA: propagate it by Policy2
  deactivate Karmada
```

#### Case 4

1. create PropagationPolicy Policy1 (priority=1, match to DeployA)
2. create ResourceTemplate DeployA
3. update Policy1
4. create Policy2 (priority=2, match to DeployA too)
5. update DeployA

```mermaid
sequenceDiagram
  participant DeployA
  participant Policy1
  participant Policy2
  participant Karmada

  Policy1 ->> Karmada: create (priority=1)
  activate Karmada
  DeployA ->> Karmada: create
  Karmada -->> DeployA: propagate it by Policy1
  deactivate Karmada
  
  Policy1 ->> Karmada: update
  activate Karmada
  loop
    Policy1 --> Karmada: no action util DeployA updated
  end
  deactivate Karmada

  Policy2 ->> Karmada: create (priority=2)
  activate Karmada
  loop
    Policy2 --> Karmada: no action util DeployA updated
  end
  deactivate Karmada

  DeployA ->> Karmada: update
  activate Karmada
  Karmada -->> DeployA: propagate it by Policy2
  deactivate Karmada
```

#### Case 5

1. create PropagationPolicy Policy1 (priority=2, match to DeployA)
2. create ResourceTemplate DeployA
3. create Policy2 (priority=1, match to DeployA too)
4. update Policy1 (no longer match to DeployA)
5. update DeployA

```mermaid
sequenceDiagram
  participant DeployA
  participant Policy1
  participant Policy2
  participant Karmada

  Policy1 ->> Karmada: create (priority=2)
  activate Karmada
  DeployA ->> Karmada: create
  Karmada -->> DeployA: propagate it by Policy1
  deactivate Karmada
  
  Policy2 ->> Karmada: create (priority=1)
  activate Karmada
  loop
    Policy2 --> Karmada: no action (lower priority + DeployA not updated)
  end
  deactivate Karmada

  Policy1 ->> Karmada: update (no longer match to DeployA)
  activate Karmada
  loop
    Policy1 --> Karmada: no action util DeployA updated
  end
  deactivate Karmada

  DeployA ->> Karmada: update  
  activate Karmada
  Karmada -->> DeployA: propagate it by Policy1
  
  deactivate Karmada
```

### Corner Case

#### Case 1

1. create PropagationPolicy Policy1 (placement=member1, match to DeployA)
2. create ResourceTemplate DeployA (replicas=2)
3. delete Policy1
4. update DeployA (change replicas from 2 to 5)
5. create PropagationPolicy Policy2 (placement=member2, match to DeployA)

```mermaid
sequenceDiagram
  participant DeployA
  participant Policy1
  participant Policy2
  participant Karmada

  Policy1 ->> Karmada: create (placement=member1)
  activate Karmada
  DeployA ->> Karmada: create (replicas=2)
  Karmada -->> DeployA: propagate it to member1 by Policy1
  deactivate Karmada
  
  Policy1 ->> Karmada: delete
  activate Karmada
  loop
    Policy1 --> Karmada: no action util DeployA updated
  end
  deactivate Karmada

  DeployA ->> Karmada: update (replicas 2 -> 5)
  activate Karmada
  Karmada -->> DeployA: add to waiting list for no matchable policy
  Note over DeployA, Karmada: the existing 2 replicas unchanged, the additional 3 replicas pending
  deactivate Karmada

  Policy2 ->> Karmada: create (placement=member2)
  activate Karmada
  Karmada -->> DeployA: all 5 replicas propagated to member2 by Policy2 immediately
  deactivate Karmada
```

In-Depth Interpretation:

* When no matching Policy, ResourceTemplate should be pending for propagation and remain unchanged until a new Policy occurred.
  Besides, deletion should happen when the ResourceTemplate is deleted, any policy changes should not cause deletion.
* Refer to the K8s mechanism, each pod is scheduled individually, so after there is a satisfying scheduling condition, it is directly scheduled.
  But our ResourceBinding is the overall scheduling, if you enter the scheduler processing, the existing 2 instances will also be calculated together.
  We can only follow the inertia rule as much as possible, and try not to move the 3 instances that are already running.
* Instances in the pending state don't count towards the overall state of the system already running. So when the new Policy occurred, 
  scheduling the pending 3 instances down is beneficial to the overall system.

#### Case 2

1. create PropagationPolicy Policy1 (placement=member1, match to DeployA)
2. create ResourceTemplate DeployA
3. karmada-detector restart before corresponding binding created
4. update Policy1 (placement=member2, still match to DeployA)
5. karmada-detector restart
6. update DeployA
7. karmada-detector restart before corresponding binding updated

```mermaid
sequenceDiagram
  participant DeployA
  participant Policy1
  participant Karmada

  Policy1 ->> Karmada: create (placement=member1)
  activate Karmada
  DeployA ->> Karmada: create
  Karmada ->> Karmada: karmada-detector restart before binding created
  Karmada -->> DeployA: propagate it to member1 by Policy1
  deactivate Karmada
  
  Policy1 ->> Karmada: update (placement=member2)
  activate Karmada
  loop
    Policy1 --> Karmada: no action util DeployA updated
  end
  deactivate Karmada

  Karmada ->> Karmada: karmada-detector restart
  loop
    Policy1 --> Karmada: no action util DeployA updated
  end

  DeployA ->> Karmada: update
  activate Karmada
  Karmada ->> Karmada: karmada-detector restart before binding updated
  Karmada -->> DeployA: propagate it to member2 by Policy1
  deactivate Karmada
```

##  Design Details

### Work Disassembly

#### FeatureGate

A new FeatureGate naming `StaticPolicy` is introduced, disabled by default.

We will advise users to enable it to try this feature in release announcement.

#### The policy worker in Detector

two modifications:

* When Policy updated, the current implementation would first find the corresponding Binding and then add the bound 
  ResourceTemplate to the processor's queue for reconciliation to make sure that Policy's updates can be synchronized to Binding.
  However, if this `StaticPolicy` enabled, just skip this step.
* When Policy updated, the current implementation would try to handle the preemption process if preemption is enabled.
  However, if this `StaticPolicy` enabled, just skip this step.

others, keep unchanged.

#### The template worker in Detector

two modifications:

* When ResourceTemplate updated, the current implementation would first check if it has been claimed by a Policy, if so, just apply it. 
  However, if this `StaticPolicy` enabled, just skip this step (ResourceTemplate will always try to find the most matchable policy and 
  apply it when updated).
* When ResourceTemplate updated, a new step will be added to the event filter process if this `StaticPolicy` enabled.
  That is if this modification only changed the labels with `.karmada.io` prefix, it would be regarded as Karmada self's behavior,
  and the update would be ignored.

others, keep unchanged.

#### Policy version upgrade

may be no necessary modifications.

> 1. This feature didn't introduce modification to API, only changed the default behavior when Policy or ResourceTemplate changed.
> 2. In future version, the `preemption` filed will be deprecated, there may be old version Policy object in etcd contains this filed,
> but new version API doesn't support this filed, then this filed will be automatically abandoned, which doesn't affect the upgrade.

#### Other work

* Complete rectification of all affected UT and e2e test cases
* Complete rectification of all affected community documents
* Supplement new e2e test cases
* Publish a release announcement with detailed upgrade guidance documents and clear upgrade precautions.

### Test Plan

**Test Case 1**
1. create deployment nginx
2. create propogationpolicy pp1 (match to nginx, propagate to member1), expect seeing nginx follow pp1 propagated to member1

**Test Case 2**
1. create propogationpolicy pp1 (match to nginx, propagate to member1)
2. create deployment nginx, expect seeing nginx follow pp1 propagated to member1
3. modify pp1 (still match to nginx, change to propagate to member2), expect seeing nginx keep unchanged
4. modify nginx, expect seeing nginx follow pp1 propagated to member2

**Test Case 3**
1. create propogationpolicy pp1 (match to nginx, propagate to member1, priority=1)
2. create deployment nginx, expect seeing nginx follow pp1 propagated to member1
3. create propogationpolicy pp2 with higher priority (match to nginx, propagate to member2, priority=2), expect seeing nginx keep unchanged
4. modify nginx, expect seeing nginx follow pp2 propagated to member2

**Test Case 4**
1. create propogationpolicy pp1 (match to nginx, propagate to member1, priority=1)
2. create deployment nginx, expect seeing nginx follow pp1 propagated to member1
3. modify pp1 (still match to nginx, change to propagate to member3), expect seeing nginx keep unchanged
4. create propogationpolicy pp2 with higher priority (match to nginx, propagate to member2, priority=2), expect seeing nginx keep unchanged
5. modify nginx, expect seeing nginx follow pp2 propagated to member2

**Test Case 5**
1. create propogationpolicy pp1 (match to nginx, propagate to member1, priority=1)
2. create propogationpolicy pp2 with higher priority (match to nginx, propagate to member2, priority=2)
3. create deployment nginx, expect seeing nginx follow pp2 propagated to member2
4. modify pp2 (no longer match to nginx), expect seeing nginx keep unchanged (but the policy label of nginx and related binding should be deleted)
5. modify nginx, expect seeing nginx follow pp1 propagated to member1

**Test Case 6**
1. create propogationpolicy pp1 (match to nginx, propagate to member1)
2. create deployment nginx (replicas is 2), expect seeing nginx follow pp1 propagated to member1
3. delete pp1, expect seeing nginx keep unchanged (but the policy label of nginx and related binding should be deleted)
4. modify nginx (replicas change to 5), expect seeing nginx existing 2 replicas keep unchanged, while additional 3 replicas is in pending status
5. create propogationpolicy pp2 (match to nginx, propagate to member2), expect seeing nginx all 5 replicas follow pp2 propagated to member2

> Note: ClusterPropagationPolicy has 6 similar test case as above.

## Risks and Mitigations

### Compatibility risk

Risk：The current version of Propagation Policy takes effect immediately and does not support preemption by default. 
The introduction of this feature is equivalent to Propagation Policy not taking effect immediately 
and supporting preemption by default, which is a big change in behavior and has the risk of breaking compatibility.

Mitigations：

* **Karmada v1.9 version：control this feature by a feature-gate, turn off by default, advising users to turn on.**
* **Karmada v1.10 version：control this feature by a feature-gate, turn on by default, allowing users to turn off in special cases.**

> The previous design was not good enough, this proposal makes more sense. Even if it destroys compatibility,
it has to be changed to benefit more users in the future

### Performance risk

none

## Questions

### How to identify whether ResourceTemplate was modified by a user or by Karmada itself

The new Policy will take effect when ResourceTemplate updated, but Karmada itself can also modify the ResourceTemplate,
e.g., adding a label, updating the status field.

The desired result is that the new Policy should take effect only by users' change, not by Karmada's own change.

So how can you tell if a ResourceTemplate was modified by a user or by Karmada itself?

Answers：

* If a modification only changes the label whose key contains the karmada-specific key prefix `.karmada.io`,
  we regard it as modification of Karmada itself, and it should be ignored.
* Updates of status field should be ignored.
* Theoretically, we should not modify the user's ResourceTemplate, if there is a need to add something to it in the future, 
  there must be a way to clearly distinguish that this is a field modified by Karmada in the design stage. If you can't distinguish it, 
  you should choose other ways to realize it to avoid modifying the user's ResourceTemplate.

### How to identify whether ResourceTemplate is in effect under the current Policy or the previous version

Answers: 

* First of all, users need to understand that Policies are static and do not take effect immediately, 
and that the distribution of user's ResourceTemplates is inert. 
* Then, if you want to see the ResourceTemplate is in effect under which version Policy , check it at ResourceBinding.

### How to put the Policy into effect immediately in special cases

Answers:

A new `karmadactl` command will be added to support proactively triggering the reconciliation of all ResourceTemplates managed by a given policy.

Just like:
```shell
karmadactl reconcile all -l clusterpropagationpolicy.karmada.io/name=policy-xxx
```