---
title: Resolving Conflicts with Dependency Distribution Policies
authors:
  - "@Kexin2000"
reviewers:
  - "@XiShanYongYe-Chang"
approvers:
  - "@RainbowMango"

creation-date: 2025-08-27
---

# Resolving Conflicts with Dependency Distribution Policies

## Summary

This document describes a change to make behavior clearer when multiple resources share the same dependent resource (for example, several Deployments use the same ConfigMap or Secret).

If those independent resources are distributed by different PropagationPolicy and those policies disagree on distribution fields, the ResourceBinding for the shared resource can get conflicting settings. We will:

- detect such disagreements,
- pick a clear, deterministic value for the attached ResourceBinding, and
- write a Warning event so users can see and fix the conflict.

This proposal only covers two fields for now: `ConflictResolution` and `PreserveResourcesOnDeletion`.

## Motivation

Short background:

- Resources such as Deployments often reference the same objects such as ConfigMaps and Secrets.
- Each independent resource may be distributed by its own PropagationPolicy.
- If the independent ResourceBindings derived from those policies set different values for `ConflictResolution` or `PreserveResourcesOnDeletion`, the attached ResourceBinding can have conflicting field values.

Why we fix this:

- Conflicting values are confusing and can cause unexpected behavior.
- A simple, documented decision makes the system easier to understand and operate.

### Goals

This proposal aims to:

- Detect when multiple referencing ResourceBindings disagree on the two fields for a shared object.
- Decide a single value for each field for the attached ResourceBinding.
- Emit a Warning event when a disagreement is found.
- Keep the attached ResourceBinding fields up to date when spec.requiredBy changes.
- Do not change ResourceBindings that already have `spec.placement` set. These are independent ResourceBindings created and scheduled by policy; their strategy is explicit and out of scope for this resolution. The resolution only applies to attached ResourceBindings where `spec.placement` is nil.

### Non-Goals

This proposal does not:

- Change the PropagationPolicy API or default values.
- Try to handle any other policy fields yet — we only touch the two named fields.

## Proposal

Simple overview of how it works:

1. Dependencies Distributor handles attached ResourceBindings (those without `Placement`).
2. For an attached ResourceBinding, it looks at `spec.requiredBy` to identify the referencing ResourceBindings.
3. It reads the two policy fields from those referencing ResourceBindings and picks a single value for each field using fixed rules (below).
4. If the referencing ResourceBindings disagree, the controller updates the attached ResourceBinding with the chosen values and emits a Warning event that explains the conflict.
5. When `spec.requiredBy` changes later, the controller repeats the steps and updates the attached ResourceBinding.

### User stories

- Story: Two Deployments share a ConfigMap and their policies disagree. Dependencies Distributor resolves values for the attached ResourceBinding and writes a Warning event that tells the user which fields conflicted.

- Story: The user fixes one policy so both agree. The next controller run updates the attached ResourceBinding and no Warning is emitted.

- Story: One of the Deployments is deleted. The attached ResourceBinding is updated right away.

## Design Details

### Scope and Objects

- Applies only to dependency-generated attached ResourceBindings where Spec.Placement == nil.
- Independent ResourceBindings (with Placement) are not modified by this resolution process. Rationale: they already carry explicit scheduling and strategy from the matched policy; changing them here would override user intent.

### Resolution and Conflict Rules

We use simple rules to pick values.

- ConflictResolution

  - If any referencing ResourceBinding uses `Overwrite`, the resolved value is `Overwrite`.
  - Otherwise the resolved value is `Abort` (this also covers the empty/default case).
  - A conflict exists if at least one referencing ResourceBinding wants `Overwrite` and at least one wants `Abort`.

- PreserveResourcesOnDeletion
  - If any referencing ResourceBinding sets this to `true`, the resolved value is `true`.
  - Otherwise the resolved value is `false` (this includes nil/default).
  - A conflict exists if at least one referencing ResourceBinding wants `true` and at least one wants `false`.

### Controller Changes

Dependencies Distributor will do this work whenever it reconciles an attached ResourceBinding (it skips ResourceBindings that already have `spec.placement`):

1. Read the attached ResourceBinding and its `spec.requiredBy` list.
2. For each snapshot in `spec.requiredBy`, load the referenced ResourceBinding from the API server.
3. Apply the rules above to decide `ConflictResolution` and `PreserveResourcesOnDeletion`.
4. Write the decided values to the attached ResourceBinding.
5. If there was a disagreement between referencing ResourceBindings, emit a Warning event that lists which field conflicted.

When an entry in `spec.requiredBy` is removed, the controller repeats these steps. If no entries remain, the controller clears the attached ResourceBinding’s strategy fields.

### Eventing

- New event reason: DependencyPolicyConflict.
- Type: Warning.
- Message includes concise hints, e.g.:
  - "ConflictResolution conflicted (Overwrite vs Abort)"
  - "PreserveResourcesOnDeletion conflicted (true vs false)"

These messages help users find and fix the policies that disagree.

### API Changes

- No CRD schema changes are required for ResourceBinding or PropagationPolicy.
- A new event reason constant is introduced under the events package.

### Test Plan

We will add E2E tests to ensure the logic is correct and easy to maintain.

- **Conflict on both fields**: Create two Deployments that share a ConfigMap and give them conflicting policies on both `ConflictResolution` and `PreserveResourcesOnDeletion`. Verify that a Warning event appears listing both conflicts, and the attached ResourceBinding contains the resolved values (Overwrite+true).

- **Conflict on ConflictResolution only**: Create two Deployments with policies that disagree only on `ConflictResolution` but agree on `PreserveResourcesOnDeletion`. Verify the Warning event mentions only the ConflictResolution conflict, and the attached ResourceBinding is resolved to Overwrite with the agreed preserve value.

- **Conflict on PreserveResourcesOnDeletion only**: Create two Deployments with policies that agree on `ConflictResolution` but disagree on `PreserveResourcesOnDeletion`. Verify the Warning event mentions only the preserve flag conflict, and the attached ResourceBinding is resolved with the agreed ConflictResolution and preserve=true.

- **Conflict resolved by updating policy**: Start with conflicting policies, verify the conflict Warning appears. Then update one policy to align with the other. Verify the Warning stops appearing and the attached ResourceBinding reflects the now-consistent values.

- **Conflict resolved by deleting policy**: Start with conflicting policies, verify the conflict Warning appears. Then delete one of the policies (which removes its independent ResourceBinding). Verify the attached ResourceBinding is updated to follow the remaining policy's values.

## Examples

This section shows a concrete example with two Deployments referencing the same ConfigMap, each matched by a different PropagationPolicy with conflicting field values.

### Step 1: Create ConfigMap and Deployments

Create a ConfigMap that will be shared by two Deployments:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
data:
  app.properties: |
    proxy-connect-timeout: "10s"
    proxy-read-timeout: "10s"
    client-max-body-size: "2m"
```

Create two Deployments that both mount this ConfigMap:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-a
  namespace: default
  labels:
    app: app-a
spec:
  replicas: 2
  selector:
    matchLabels:
      app: app-a
  template:
    metadata:
      labels:
        app: app-a
    spec:
      containers:
        - image: nginx
          name: nginx
          ports:
            - containerPort: 80
          volumeMounts:
            - name: configmap
              mountPath: "/configmap"
      volumes:
        - name: configmap
          configMap:
            name: my-config
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-b
  namespace: default
  labels:
    app: app-b
spec:
  replicas: 2
  selector:
    matchLabels:
      app: app-b
  template:
    metadata:
      labels:
        app: app-b
    spec:
      containers:
        - image: nginx
          name: nginx
          ports:
            - containerPort: 80
          volumeMounts:
            - name: configmap
              mountPath: "/configmap"
      volumes:
        - name: configmap
          configMap:
            name: my-config
```

### Step 2: Create PropagationPolicies with conflicting fields

Create a PropagationPolicy for `app-a` with `propagateDeps: true`, `conflictResolution: Overwrite`, and `preserveResourcesOnDeletion: true`:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: app-a-policy
  namespace: default
spec:
  conflictResolution: Overwrite
  preserveResourcesOnDeletion: true
  propagateDeps: true
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: app-a
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
    replicaScheduling:
      replicaDivisionPreference: Weighted
      replicaSchedulingType: Divided
      weightPreference:
        staticWeightList:
          - targetCluster:
              clusterNames:
                - member1
            weight: 1
          - targetCluster:
              clusterNames:
                - member2
            weight: 1
```

Create a PropagationPolicy for `app-b` with `propagateDeps: true`, `conflictResolution: Abort`, and `preserveResourcesOnDeletion: false`:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: app-b-policy
  namespace: default
spec:
  conflictResolution: Abort
  preserveResourcesOnDeletion: false
  propagateDeps: true
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: app-b
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
    replicaScheduling:
      replicaDivisionPreference: Weighted
      replicaSchedulingType: Divided
      weightPreference:
        staticWeightList:
          - targetCluster:
              clusterNames:
                - member1
            weight: 1
          - targetCluster:
              clusterNames:
                - member2
            weight: 1
```

### Step 3: Verify conflict detection and resolution

After creating the policies, Dependencies Distributor will:

1. Create independent ResourceBindings for `app-a` and `app-b` with their respective policy field values
2. Create an attached ResourceBinding for the shared ConfigMap `my-config`
3. Detect the conflict between the two policies
4. Resolve the conflict using the rules (Overwrite wins, true wins)
5. Emit a Warning event

## Alternatives

### Alternative 1: Reject Multiple References to Dependent Resources

**Approach:**

Prohibit a dependent resource (e.g., ConfigMap or Secret) from being referenced by multiple independent resources (e.g., Deployments). This would eliminate conflicts at the system level by enforcing a one-to-one relationship.

**Implementation:**

- The controller detects when a dependent resource is referenced by multiple independent resources and rejects the operation or emits an error.
- Users would be required to create separate copies of the dependent resource for each independent resource.

**Pros:**

- Completely eliminates the possibility of conflicts.
- Simple and straightforward implementation.

**Cons:**

- Breaks existing user patterns and use cases.
- Contradicts the design goal of resource sharing and reuse.
- Increases resource redundancy and management overhead.
- Poor user experience.

**Conclusion:** Rejected. This approach does not align with the goals of supporting resource sharing and would significantly disrupt existing workflows.

---

### Alternative 2: First-Come-First-Served Policy Inheritance

**Approach:**

Use a time-based ordering where the first independent resource to reference a dependent resource determines the policy. Subsequent references inherit or are ignored based on this initial policy.

**Implementation:**

- Track the first ResourceBinding that references the dependent resource.
- All policy fields for the attached ResourceBinding inherit from this first referencing ResourceBinding.
- If the first reference is deleted, switch to the next earliest reference.

**Pros:**

- Relatively simple to implement.
- Provides deterministic behavior in theory.

**Cons:**

- Policy source is opaque to users, making it difficult to understand why a particular policy is used.
- Creation order can be affected by concurrency, leading to unpredictability.
- Lack of observability makes troubleshooting difficult.
- Deleting and recreating resources could lead to unexpected policy changes.

**Conclusion:** Rejected. This approach lacks transparency and makes it difficult for users to reason about system behavior.
