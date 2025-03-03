---
title: Support enforcing resource limits on Karmada control-plane with federated resource quota
authors:
- "@mszacillo"
reviewers:
- "@RainbowMango"
- "@XiShanYongYe-Chang"
approvers:
- "@RainbowMango"
- "XiShanYongYe-Chang"

creation-date: 2025-03-02

---

# Support Enforcing Resource Limits on Karmada Control Plane with FederatedResourceQuota

## Summary

This proposal aims to enhance the FederatedResourceQuota to enforce namespaced resource limits on a Karmada control-plane level. With the feature, it will be easier for users of application and cluster failover to ensure that fallback clusters have enough resources to house all the user's workflows. Ideally, this is a feature that can be configured and toggled, which would provider users with more control over enforcing their namespaces resources.

## Motivation

The desire for this feature comes from the use of application and cluster failover. With these features toggled applications are expected to migrate between clusters, and it is up to us as platform owners to make sure each cluster has sufficient resources to be able to host those applications. We believe controlling resource limits on a Karmada level will simplify this resource quota management.

The existing FederatedResourceQuota only provides Karmada administrators with the ability to manage ResourceQuotas via static assignment on member clusters. This saves administrators some time by not requiring a PropagationPolicy for their ResourceQuotas.

However, this feature does not work entirely with application and cluster failover. Since static assignment of resource quotas requires that users subdivide their quota between clusters, each member cluster’s resource quota will be less than the total quota allocated to the user. This means during failover, the other member cluster will not have sufficient resources to host failed-over applications.

### Goals
 - Enhance the FederatedResourceQuota to support enforcing namespaces resource limits on the Karmada control-plane level
 - Make this feature configurable and toggleable

### Non-Goals

## Proposal

1. FederatedResourceQuota should enforce Overall resource limits if Static Assignment is not defined.
    - FederatedResourceQuota will be updated whenever resources are applied against the relevant namespace
    - We can either updated FRQ by default, or consider including a scope for the quota
2. A custom controller will be responsible for updating the overall resource usage
3. A validation webhook (or admission controller) will block users from applying or updating resources that will go above total resource allowances

## API Changes

The ResourceBindingSpec will include a FederatedResourceQuota reference, which can be updated one of two ways:

1. If there is a FederatedResourceQuota in the namespace, add it to the created binding spec by default
2. If there is no FederatedResourceQuota, leave empty.
3. If the FederatedResourceQuota has a scope, determine if the resource matches the scope, and then add the pointer in the binding if needed.

```go
// ResourceBindingSpec represents the expectation of ResourceBinding.
type ResourceBindingSpec struct {
    ***

    // FederatedResourceQuota represents the name of the ResourceQuota that will be used for this resource
	// +optional
	FederatedResourceQuota string `json:"federatedResourceQuota,omitempty"`

    ***
}
```

## Design Details

### Controller Change

**Reconciliation**: Controller will reconcile whenever a resource binding is created, updated, or deleted. Controller will only reconcile if the resource in question has a pointer to a FederatedResourceQuota.

**Reconcile Logic**:  When reconciling, the controller will fetch the list of ResourceBindings by namespace and add up their resource requirements. The existing implementation grabs all RBs by namespace, however this could probably be improved by only calculating the delta of the applied resource, rather than calculating the entire resource footprint of the namespace.

### Scheduler Change

Note: Since the controller is listening to RBs, the FederatedResourceQuota will be calculated after the resource binding has been created or updated.
If a user bulk-applies a bunch of resources at once, it could be possible for the user to go above the quota’s limits. In this case, we should also check that the quota is honored before deciding to schedule the resource to a member cluster.

### Admission Webhook

As part of this change, we will do two things:

1. Edit the existing FederatedResourceQuota validating webhook to prevent users from toggling StaticAssignments on/off when the resource quota is already in use. Since the overall limits and static assignments have different controllers, we don’t want them both reconciling the resource at once.
2. Create a ValidatingWebhook to enforce FederatedResourceQuota limits. The existing implementation reuses Karmada default + thirdparty resource interpreters to calculate the predicted delta resource usage for the quota. If the applied resource goes above the limit, then the webhook will deny the request.
