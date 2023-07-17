---
title: Resource deletion protection
authors:
- "@Vacant2333"
reviewers:
- "@XiShanYongYe-Chang"
- "@zishen"
- "@RainbowMango"
- "@kevin-wangzefeng"
approvers:
- "@RainbowMango"
- "@kevin-wangzefeng"

creation-date: 2023-07-17

---

# Resource Deletion Protection

## Summary

In the Karmada control plane, we require a resource protection feature. Once users specify the resources to be protected (such as Deployment, StatefulSet, Service, Namespace, and so on), if any of these protected resources are deleted, we need to prevent this operation.

## Motivation

This feature is highly necessary, as the mistaken deletion of a resource in the control plane could potentially lead to severe consequences.

### Goals

- Prevent the user's deletion operation if the resource is protected.

## Protected resources

- CRD
- Kubernetes resources
- Resource extend by [Aggregate Layer](https://kubernetes.io/zh-cn/docs/concepts/extend-kubernetes/api-extension/apiserver-aggregation/)

## Proposal

### User Story

#### Story 1: Protect the namespace

As a manager, I want to protect my `minio` namespace, so that they won't be accidentally deleted.

![user-story-1](statics/user-story-1.drawio.svg)

#### Story 2: Protect the whole application

As a manager, I want to protect my `MySQL` application, so that they won't be accidentally deleted.

Our users may need to provide further protection for certain resources, not limited to just `Namespace`. They might want to secure resources such as `Deployment`,`StatefulSet`,`Service` and so on, for example, **all resources related to `MySQL` application**.

### Design Details

We can add a CRD `ResourceDeletionProtection` to specify the resources which should be protected.

#### CRD Example

Resource Scope: Cluster

Target User: Cluster administrators(**manager**)

```yaml
kind: ResourceDeletionProtection
metadata:
  name: sample
spec:
  protectedResources:
    - resourceSelector:
        - apiVersion: "apps/v1"
          kind: "Deployment"
          namespace: "sample"
      resourceNames:
        - "deployment-1"
        - "deployment-2"
    - resourceSelector:
        - apiVersion: "v1"
          kind: "Service"
          name: "service-1"
          namespace: "sample"
      resourceNames:
        - "service-1"
```

#### Webhook Implement Detail

Whenever we receive a `DELETE` request from a user/manager, our webhook will use an informer to list all `ResourceDeletionProtection` resources to check if the resource is protected. If it is, the operation will be prevented.

#### Pros

- No need for add label to resources.
- Centralized permission control.
- More comprehensive presentation to users.

#### Cons

- Requires adding a CRD, leading to a more complex implementation.
- Requires list `ResourceDeletionProtection` resource in the webhook.

### Q&A

#### Would this potentially lead to abuse of the webhook?

It's not necessarily abuse. I believe that in most scenarios, deletion operations are relatively infrequent.

#### Should there be a switch provided to users, and should the feature be disabled by default?

I believe it's unnecessary. If `ResourceDeletionProtection` is not created, it can be understood as being in a disabled state, as long as our webhook is functioning properly.

#### What will happen If user want to delete resource by `--force`?

We may not be able to distinguish between the force delete and normal delete operations, so for force delete, we will still follow the original logic and prevent users from deleting the resource.

## Alternatives

### Implemented by labels

**User** can add a label `karmada.io/deletion-protected=always` to the resource that needs protection.

If a resource that a user attempts to delete with the label `karmada.io/deletion-protected=always` with a value of `always`, then we will prevent its operation through the webhook.

#### Add the protected tag

`kubectl label <resource-type> <resource-name> karmada.io/deletion-protected=always`

`karmadactl protect <resource-type> <resource-name>`

#### Remove the protected tag

`kubectl label <resource-type> <resource-name> karmada.io/deletion-protected=none`

`karmadactl unprotect <resource-type> <resource-name>`

#### Why choose Label over Annotations to tag resources?

Annotations are typically used to store metadata that is not used for identification and selection, such as descriptive information or the status information of management tools. In this case, labels like "cannot be deleted" or "protected status" are not suitable as annotations.

#### Enable protection by default (imagine)

We can add a switch to enable deletion protection by default for all resources, if user want to delete the resources, he must remove the label first.

#### Pros

- Easy to implement

#### Cons

- The user needs to add/remove tags to resources one by one.
- The permission management is scattered, unable to follow the resources for permission control and unified handling.
- A label is not a runtime state, and in certain situations, labels might be removed (such as when a controller is deleted and then the resource is re-created).

## Test Plan

### UT

Test cover the new methods.

### E2E

| Test Case                                         | Case Description                                                     |
|---------------------------------------------------|----------------------------------------------------------------------|
| Delete resources with no protection               | Test if the system allows deletion of a resource with no protection. |
| Delete resources with protection                  | Test if the system deny deletion of a resource that has protection.  |
| Delete resources after protection has been remove | Test the deletion a resource once its protection been removed.       |
