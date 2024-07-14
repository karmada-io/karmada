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

In the Karmada control plane, we require a resource protection feature. Once the user specify that the resources (such as Deployment, StatefulSet, Service, Namespace, and so on) needs to be protected, the request will be rejected when the user attempts to delete them.

## Motivation

In order to prevent catastrophic effects when users delete resources, such as Namespace or Cluster resources, on the Karmada control plane due to misuse, we need to provide a resource deletion protection mechanism.

### Goals

- Prevent the user's deletion operation if the resource is protected.

## Protected resources

- CustomResourceDefinitions(CRD)
- Kubernetes resources
- Resource extend by [Kubernetes API Aggregate Layer](https://kubernetes.io/zh-cn/docs/concepts/extend-kubernetes/api-extension/apiserver-aggregation/)

## Proposal

### User Story

#### Story 1: Protect the namespace

As an end user, I want to protect my `minio` namespace, so that they won't be accidentally deleted.

![user-story-1](statics/user-story-1.drawio.svg)

#### Story 2: Protect the whole application

As an end user, I want to protect my `MySQL` application, so that they won't be accidentally deleted.

Our users may need to provide further protection for certain resources, not limited to just `Namespace`. They might want to secure resources such as `Deployment`,`StatefulSet`,`Service` and so on, for example, **all resources related to `MySQL` application**.

### Design Details

Add a new label `resourcetemplate.karmada.io/deletion-protected` with enumerated values including `Always`. Users can set this label for the resource to be protected. When the label value is set to `Always`, the user performs a delete operation, and `karmada-webhook` will reject the request, thus preventing the target resource form being deleted.

#### New label added

```go
// ResourceDeletionProtectionLabelKey is the label key used to add to the resource to 
// indicate the deletion protection policy of the resource.
ResourceDeletionProtectionLabelKey = "resourcetemplate.karmada.io/deletion-protected"
// ResourceDeletionProtectionAlways indicate that deletion protection is always performed.
ResourceDeletionProtectionAlways = "Always"
```

#### Webhook Configuration

The ValidatingWebhookConfiguration resource can be configured so the karmada-webhook only focuses on resource deletion operations and the resource labels include `resourcetemplate.karmada.io/deletion-protected`.

```yaml
rules:
  - operations: ["DELETE"]
    apiGroups: ["*"]
    apiVersions: ["*"]
    resources: ["*"]
    scope: "*"
objectSelector:
  matchExpressions:
    - { key: "resourcetemplate.karmada.io/deletion-protected", operator: "Exists" }
 ```

In the karmada-webhook implementation of the corresponding route, we can directly get the value of the key `resourcetemplate.karmada.io/deletion-protected` label in the resource without decoding the request object, and if the value is `Always`, the user's delete operation is rejected, otherwise it will be passed.

#### What users need to do

Users can add this label to a resources to set deletion protection on the target resource with the following command:

```
kubectl label <resource-type> <resource-name> resourcetemplate.karmada.io/deletion-protected=Always
```

When users no longer wants to protect the protected resource, the following command is available:

```
kubectl label <resource-type> <resource-name> resourcetemplate.karmada.io/deletion-protected-
```

Alternatively, users can modify the resource directly using the `kubectl edit` command.

#### Pros and Cons

##### Pros

- Simplicity of solution, simplicity of realization.

##### Cons

- Users need to process the label one by one for the resources to be protected.
- The permission management is scattered, unable to follow the resources for permission control and unified handling.

### Q&A

#### Why choose Label over Annotations to tag resources?

Annotations are typically used to store metadata that is not used for identification and selection, such as descriptive information or the status information of management tools. In this case, labels like "cannot be deleted" or "protected status" are not suitable as annotations.

#### Whether resource deletion protection needs to be enabled by default

If subsequently needed, we can add a feature gate to enable deletion protection by default for all resources.

## Alternatives

Add a new CRD `ResourceDeletionProtection` to specify the resources which should be protected.

### CRD Example

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

### Webhook Implement Detail

Whenever we receive a `DELETE` request from a user/manager, our webhook will use an informer to list all `ResourceDeletionProtection` resources to check if the resource is protected. If it is, the operation will be prevented.

### Pros and Cons

#### Pros

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

## Test Plan

### UT

Test cover the new methods.

### E2E

| Test Case                                         | Case Description                                                     |
|---------------------------------------------------|----------------------------------------------------------------------|
| Delete resources with no protection               | Test if the system allows deletion of a resource with no protection. |
| Delete resources with protection                  | Test if the system deny deletion of a resource that has protection.  |
| Delete resources after protection has been remove | Test the deletion a resource once its protection been removed.       |
