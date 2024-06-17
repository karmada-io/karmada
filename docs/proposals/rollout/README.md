---
title: Rollout Interpreter Mechanism for Workload
authors:
- "@veophi" # Authors' github accounts here.
reviewers:
- "@XiShanYongYe-Chang"
- "@yike21"
- "@RainbowMango"
approvers:
- TBD

creation-date: 2024-06-17

---

# Rollout Interpreter Mechanism for Workload

<!--
This is the title of your KEP. Keep it short, simple, and descriptive. A good
title can help communicate what the KEP is and should be considered as part of
any review.
-->

## Summary

<!--
This section is incredibly important for producing high-quality, user-focused
documentation such as release notes or a development roadmap. 

A good summary is probably at least a paragraph in length.
-->

A Stable, Reliable, Predictable rollout mechanism is the cornerstone of releases and changes in production environment.

## Motivation

<!--
This section is for explicitly listing the motivation, goals, and non-goals of
this KEP.  Describe why the change is important and the benefits to users.
-->

Karmada's current workload synchronization mechanism is basic, causing major of workloads to lose their native semantic of some fileds(e.g., `maxUnavailble`/`partition`) about rollout. This crude synchronization mechanism renders the rollout process of workloads in Karmada unpredictable and unsafe, thereby significantly hindering the practical implementation of Karmada's comprehensive solution in real-world production environments.
For example, under the current workload synchronization mechanism, if a federated workload with `maxUnavailable: 5` is spread across 10 clusters, it might inadvertently make 50 Pods unavailable instead of the intended 5, threatening deployment stability.
These issues pose barriers to Karmada's enterprise adoption, as they undermine the precise control needed for reliable, compliant deployment in production environments. Refining synchronization to handle complex deployment mechanism is essential for Karmada's wider acceptance.

### Goals

<!--
List the specific goals of the KEP. What is it trying to achieve? How will we
know that this has succeeded?
-->

- Provide a mechanism that ensures the Rollout-related semantics such as MaxUnavailable, MaxSurge, and Partition for the majority of workload types remain consistent across a federation, mirroring their behavior in a single-cluster setting.
- Provide implementations for frequently used workloads such as Deployment and CloneSet within this mechanism.

### Non-Goals

<!--
What is out of scope for this KEP? Listing non-goals helps to focus discussion
and make progress.
-->

- Provide federated support for Rollout Strategies like Argo-Rollout, Flagger, and Kruise-Rollout.

## Proposal

<!--
This is where we get down to the specifics of what the proposal actually is.
This should have enough detail that reviewers can understand exactly what
you're proposing, but should not include things like API designs or
implementation. What is the desired outcome and how do we measure success?.
The "Design Details" section below is for the real
nitty-gritty.
-->

### User Stories (Optional)

<!--
Detail the things that people will be able to do if this KEP is implemented.
Include as much detail as possible so that people can understand the "how" of
the system. The goal here is to make this feel real for users without getting
bogged down.
-->

#### Story #1: Deployment Rolling Update with 2 maxUnavailable and 1 maxSurge

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rollout-demo
spec:
  replicas: 10
  strategy:
    rollingUpdate:
      maxUnavailable: 2
      maxSurge: 1
  template:
    metadata:
      labels:
        app: rollout-demo
    spec:
      containers:
        - name: main
          image: nginx:v2 # upgrade rollout-demo from V1 -> V2
  selector:
    matchLabels:
      app: rollout-demo
---
apiVersion: karmada.io/v1alpha2
kind: ResourceBinding
metadata:
  name: rollout-demo-deployment
spec:
  clusters: # scheduled to 2 clusters
    - name: cluster-1
      replicas: 4
    - name: cluster-2
      replicas: 6
```

We expect that the Deployment Rolling Update with 2 maxUnavailable and 1 maxSurge will be scheduled to 2 clusters, and the rollout process will be consistent with the semantics in a single-cluster setting.
During rolling processing, the number of unavailable Pods will not exceed 2, and the number of Pods will not exceed 11.

#### Story #2: Batch Release with Partition
```yaml
apiVersion: apps.kruise.io/v1alpha1
kind: CloneSet
metadata:
  name: rollout-demo
spec:
  replicas: 10
  updateStrategy:
    partition: 4
    maxUnavailable: 2
  template:
    metadata:
      labels:
        app: rollout-demo
    spec:
      containers:
        - name: main
          image: nginx:v2 # upgrade rollout-demo from V1 -> V2
  selector:
    matchLabels:
      app: rollout-demo
---
apiVersion: karmada.io/v1alpha2
kind: ResourceBinding
metadata:
  name: rollout-demo-cloneset
spec:
  clusters: # scheduled to 2 clusters
    - name: cluster-1
      replicas: 3
    - name: cluster-2
      replicas: 7
```


We expect that the CloneSet cloud be paused when the number of Pods with the new image reaches 4, and the rollout process will be consistent with the semantics in a single-cluster setting. 
During rolling processing, the number of unavailable Pods will not exceed 2.


### Risks and Mitigations

<!--
What are the risks of this proposal, and how do we mitigate? 

How will security be reviewed, and by whom?

How will UX be reviewed, and by whom?

Consider including folks who also work outside the SIG or subproject.
-->

- May further reduce the efficiency of distribution synchronization of workloads.

## Architecture
The primary approach involves using feedback from the StatusController on single-cluster workload statuses to derive the next rollout configurations. This ensures that throughout the Rollout process, rollout configurations such as `Partition`, `MaxUnavailable`, and `MaxSurge` maintain their original native meanings, and that the federated Rollout behavior aligns with single-cluster  practices.

![architecture](rollout-mechanism-arch.png)

## Changes required

<!--
This section should contain enough information that the specifics of your
change are understandable. This may include API specs (though not always
required) or even code snippets. If there's any ambiguity about HOW your
proposal will be implemented, this is the place to discuss them.
-->

### Code Changes required
- New Methods in Resource Interpreter
```golang
package resourceinterpreter

type ResourceInterpreter interface {
	// Get & Set for Partition
	GetPartition(object *unstructured.Unstructured) (*intstr.IntOrString, error)
	RevisePartition(object *unstructured.Unstructured, partition *intstr.IntOrString) (*unstructured.Unstructured, error)

	// Get & Set for MaxSurge
	GetMaxSurge(object *unstructured.Unstructured) (*intstr.IntOrString, error)
	ReviseMaxSurge(object *unstructured.Unstructured, maxSurge *intstr.IntOrString) (*unstructured.Unstructured, error)

	// Get & Set for MaxUnavailble
	GetMaxUnavailable(object *unstructured.Unstructured) (*intstr.IntOrString, err error)
	ReviseMaxUnavailable(object *unstructured.Unstructured, maxUnavailable *intstr.IntOrString) (*unstructured.Unstructured, error)

	RolloutInterperter
}
```

- New Interpreter for Rollout
```golang
package rolloutinterpreter
type RolloutInterperter interface {
	ReviseRolloutConfigurations(resourceTemplate *unstructured.Unstructured, workloads *map[string]*unstructured.Unstructured, works map[string]*workv1alpha1.Work) (map[string]*unstructured.Unstructured, error)
}

type universalRolloutInterperter struct {
}

func (uri *universalRolloutInterperter) ReviseRolloutConfigurations(resourceTemplate *unstructured.Unstructured, workloads *map[string]*unstructured.Unstructured, works map[string]*workv1alpha1.Work) (map[string]*unstructured.Unstructured, error) {
	modifiedWorkloads := make(map[string]*unstructured.Unstructured, len(workloads))
	/*
	   1. calculate partition for each workload if has partition interperter
	   2. calculate maxUnavailble for each workload if has maxUnavailable interperter
	   3. calculate maxSurge for each workload if has maxSurge interperter
	   4. patch these calculated fields to each workload
	*/
	return modifiedWorkloads
}
```

- Binding Controller invoke the rollout interpreter before creating/updating to etcd
```golang
package binding

// ensureWork ensure Work to be created or updated.
func ensureWork(
	c client.Client, resourceInterpreter resourceinterpreter.ResourceInterpreter, resourceTemplate *unstructured.Unstructured,
	overrideManager overridemanager.OverrideManager, binding metav1.Object, scope apiextensionsv1.ResourceScope,
) error {
	.... ....

	// revise replicas for each target cluster
	workloads := map[string]*unstructured.Unstructured
	for _, cluster := range targetClusters {
		clone := resourceTemplate.DeepCopy()
		if needReviseReplicas(replicas, placement) {
			clone, err = resourceInterperter.ReviseReplicas()
			... ...
		}
		workloads[cluster.Name] = clone
	}

	// revise rollout configurations for each target workload
	if needReviseRolloutConfigurations(workload) {
		works, err := listWorksByClusters(targetClusters)
		... ...

		workloads, err = resourceInterperter.ReviseRolloutConfiguration(resourceTemplate, workloads, works)
		... ...
	}

	// override and apply
	for _, cluster := range targetClusters {
		workload := workloads[cluster.Name]
		cops, ops, err := overrideManager.ApplyOverridePolicies(workload, cluster.Name)
		... ...

		err = CreateOrUpdateWork(c, workMeta, workload)
	}
	return nil
}
```

### Test Plan

<!--
**Note:** *Not required until targeted at a release.*

Consider the following in developing a test plan for this enhancement:
- Will there be e2e and integration tests, in addition to unit tests?
- How will it be tested in isolation vs with other components?

No need to outline all test cases, just the general strategy. Anything
that would count as tricky in the implementation, and anything particularly
challenging to test, should be called out.

-->

// TODO

## Alternatives

<!--
What other approaches did you consider, and why did you rule them out? These do
not need to be as detailed as the proposal, but should include enough
information to express the idea and why it was not acceptable.
-->

<!--
Note: This is a simplified version of kubernetes enhancement proposal template.
https://github.com/kubernetes/enhancements/tree/3317d4cb548c396a430d1c1ac6625226018adf6a/keps/NNNN-kep-template
-->