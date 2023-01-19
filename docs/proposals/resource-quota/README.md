---
title: ResourceQuota
authors:
- "@aven-ai"
  reviewers:
- "@TBD"
  approvers:
- "@TBD"

  creation-date: 2021-08-20

---


# ResourceQuota

## Summary


With the widespread used of multi-clusters, the single-cluster quota management `ResourceQuota` in Kubernetes can no longer meet the administrator's resource management and restriction requirements for federated clusters. Resource administrators often need to stand on the global dimension to manage and control the total consumption of resources by each business.

Creating a corresponding `namespace` and `ResourceQuotas` under each Kubernetes cluster is a common practice, and then Kubernetes will limit the resources according to `ResourceQuotas`. However, today's administrators may be challenged by the exploding service volume, sub-cluster scaling needs, and resources of different amounts and types.

In addition, Karmada supports the propagation of `ResourceQuota` objects to Kubernetes clusters through a PropagationPolicy to create quotas for multi-clusters using native K8s APIs. However, it is impossible to freely adjust and limit the global resource usage of a business by PropagationPolicies solely, and doing this could be really troublesome. We need a global quota for Karmada, not just `ResourceQuota` on sub-clusters.

This document describes a quota system **`KarmadaQuota`** for Karmada. As a part of admission control, the **`KarmadaQuota`** enforcing hard resource usage limits per namespace.

The following design documents partially refer to the Kubernetes ResourceQuota design.

## Motivation

### Goals

 - Supports the global quota management for multi-clusters on Karmada.
 - Allows administrators to perform quota management on Karmada to balance multi-clusters resources allocation, monitor usage, and control billing for different businesses.

### Non-Goals

 - Provide complex quotas to meet various business scenarios.

## Proposal

### Function overview

1. Ability to enumerate resource usage limits per namespace.
2. Ability to monitor resource usage for tracked resources.
3. Ability to reject resource usage exceeding hard quotas.

### User Stories

#### Story 1
As a Karmada administrator, I want to create a resource limit of 100 core CPUs for business A.

The available resources on clusters:
 - cluster1: 20C
 - cluster2: 50C
 - cluster3: 100C
 - ...

Generally, administrators have two methods. 
 - In each cluster, manually divide a small quota for business A. 
 - Or use Karmada PropagationPolicy to propagate the same quota to sub-clusters.

Neither of these two methods can meet the needs of administrators. 

Now, the administrator can directly create a 100 core CPUs Quota in Karmada for business A by Karmada's **KarmadaQuota**. The Quota can control all the available resources of the sub-clusters, administrators no longer need to operate `ResourceQouota` in the sub-clusters.

#### Story 2
As a business user, I want to know how many quotas I have in total and how many quotas I currently used.

Business users usually pay attention to the total resource usage of Quota that they have applied for, rather than how many resources are used in a specific cluster. 

Now we can create a total quota for business A (business representative) on karmada, and Karmada itself can monitor the resource usage of the tracked resource, not the monitor of sub-clusters quota.


### Notes/Constraints/Caveats (Optional)

### Risks and Mitigations

1. If a customer creates a pod in a member cluster, Karmada is not able to perceive it, and KarmadaQuota will not limit it too. It reverts the usage of a single cluster.

2. If a customer use karmada to propagate a controller which can create pods in the member clusters, as the quota webhook can't check these pods' resources, these pods will be created in the member clusters, which leads to resource use more than limited. There are currently no mitigation measures. Please try to avoid such a use, and stay tuned for the updates from the Karmada community.
See [discuss this situation](https://github.com/karmada-io/karmada/pull/632).

## Design Details

### General Idea

 1. The quota management of multi-cluster Karmada is expected to a centralized design,  all quota management information and resource verification will be closed-loop within karmada. Do not have too much coupling with the outside world or sub-clusters.
 2. The sub-clusters do not need to create ResourceQuotas to manage its own resources. Quota information is stored in Karmada etcd, and cluster resources are managed by Karmada. The subordinate clusters of Karmada is equivalent to the node of Kubernetes.

### Data Model API

In order to distinguish the functions and positioning of Karmada's multi-cluster management quota and the `ResourceQuota` of k8s, the quota API on Karmada is defined as **`KarmadaQuota`**.

The **KarmadaQuota** object is scoped to a **Namespace**.

```go
// KarmadaQuota sets aggregate quota restrictions enforced per namespace
type KarmadaQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the desired quota.
	// +optional
	Spec corev1.ResourceQuotaSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// Status defines the actual enforced quota and its current usage.
	// +optional
	Status corev1.ResourceQuotaStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KarmadaQuotaList is a list of KarmadaQuota resources.
type KarmadaQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []KarmadaQuota `json:"items" protobuf:"bytes,2,rep,name=items"`
}

```

### Quota Tracked Resources

**`KarmadaQuota`** supports resources almost similar to k8s native **`ResourceQuota`**. But unlike k8s, KarmadaQuota only supports objects and resources that can be distributed to sub-clusters through **`PropagationPolicy`**. If an object or resource does not need to be distributed to sub-clusters, then the KarmadaQuota does not track it.

The following example resources are supported by the quota system:

| Resource | Description |
| ------------ | ----------- |
| cpu | Total requested cpu usage |
| memory | Total requested memory usage |
| pods | Total number of active pods where phase is pending or active.  |
| services | Total number of services |
| secrets | Total number of secrets |

If a third-party wants to track additional resources, it must follow the
resource naming conventions prescribed by Kubernetes. This means the resource must have a fully-qualified name (i.e. mycompany.org/shinynewresource)

### Resource Requirements: Requests vs. Limits

The admission check and resource consumption deduction of `KarmadaQuota` are achieved by tracking **`Resourcebinding`**. Resourcebinding of karmada is designed to only support ResourceRequest, and the `limit` of pod Container does not be distributed to the sub-cluster, so KarmadaQuota only supports Request type resources

The following is the API definition of ResourceBinding's replicaRequirements, the `resourceRequest` is the resource Request that karmada will distribute to the sub-cluster by PropagationPolicy.
```go 
// ReplicaRequirements represents the requirements required by each replica.
type ReplicaRequirements struct {
	// A node selector represents the union of the results of one or more label queries
	// over a set of nodes; that is, it represents the OR of the selectors represented
	// by the node selector terms.
	// +optional
	NodeAffinity *corev1.NodeSelector `json:"nodeAffinity,omitempty"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// If specified, the pod's tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// ResourceRequest represents the resources required by each replica.
	// +optional
	ResourceRequest corev1.ResourceList `json:"resourceRequest,omitempty"`
}
````

### Scopes in ResourceQuota

Each quota can have an associated set of scopes. A quota will only measure usage for a resource if it matches the intersection of enumerated scopes.

When a scope is added to the quota, it limits the number of resources it supports to those that pertain to the scope. Resources specified on the quota outside of the allowed set results in a validation error.

| Scope | Description |
| --- | --------- | 
| PriorityClass | Match pods that references the specified priority class. |

The scopeSelector supports the following values in the operator field:

 - In
 - NotIn
 - Exists
 - DoesNotExist

If the operator is In or NotIn, the values field must have at least one value. For example:

```go
scopeSelector:
  matchExpressions:
    - scopeName: PriorityClass
      operator: In
      values:
        - middle
```


### karmada Quota Controller

A resource quota controller monitors observed usage for tracked resources in the
**Namespace**.

If there is observed difference between the current usage stats versus the
current **KarmadaQuota.Status**, the controller posts an update of the
currently observed usage metrics to the **KarmadaQuota** via the /status
endpoint.

The resource quota controller is the only component capable of monitoring and recording usage updates after a DELETE operation since admission control is incapable of guaranteeing a DELETE request actually succeeded.

### karmada Quota webhook

The **KarmadaQuota** plug-in introspects all incoming admission requests.

It makes decisions by evaluating the incoming object against all defined
**KarmadaQuota.Status.Hard** resource limits in the request namespace. If
acceptance of the resource would cause the total usage of a named resource to exceed its hard limit, the request is denied.

If the incoming request does not cause the total usage to exceed any of the enumerated hard resource limits, the plug-in will post a
**KarmadaQuota.Status** document to the server to atomically update the
observed usage based on the previously read **KarmadaQuota.ResourceVersion**.
This keeps incremental usage atomically consistent, but does introduce a
bottleneck (intentionally) into the system.

To optimize system performance, it is encouraged that all resource quotas are
tracked on the same **KarmadaQuota** document in a **Namespace**. As a result,
it is encouraged to impose a cap on the total number of individual quotas that
are tracked in the **Namespace** to 1 in the **KarmadaQuota** document.

### Test Plan

- Propose E2E test cases according to our use cases above:
  - Test we can enumerate resource usage limits per namespace.
  - Test we can monitor resource usage for tracked resources.
  - Test we can reject resource usage exceeding hard quotas.

- Propose a tool that people can test the script.

## More information

See [k8s resource quota document](https://kubernetes.io/docs/concepts/policy/resource-quotas/) and the [example of Resource Quota](https://kubernetes.io/docs/tasks/administer-cluster/quota-api-object/) for more information.
