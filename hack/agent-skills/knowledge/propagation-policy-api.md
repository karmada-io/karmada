# PropagationPolicy

## Overview

`PropagationPolicy` defines how Kubernetes resources managed by Karmada are propagated across member clusters.

It is one of the core Karmada policy APIs and is responsible for:
- selecting resources
- defining placement targets
- controlling propagation behavior
- configuring scheduling preferences

A `PropagationPolicy` works together with:
- Karmada Scheduler
- `ResourceBinding`
- `Work`
- member cluster controllers

The policy determines where and how workloads should be distributed in a multi-cluster environment.

## Basic Workflow

The propagation workflow is typically:

```text
Resource
   ↓
PropagationPolicy
   ↓
ResourceBinding
   ↓
Scheduler Decision
   ↓
Work Objects
   ↓
Member Clusters
```

## API Version

Typical API version:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
```

API versions may evolve across Karmada releases.

## Core Concepts

### Resource Selection

`resourceSelectors` define which Kubernetes resources should be managed by the policy.

Example:

```yaml
resourceSelectors:
  - apiVersion: apps/v1
    kind: Deployment
    name: nginx
```

A policy may select:
- Deployments
- Services
- ConfigMaps
- StatefulSets
- custom resources

### Cluster Placement

`placement` defines where resources should be propagated.

Example:

```yaml
placement:
  clusterAffinity:
    clusterNames:
      - member1
      - member2
```

This instructs Karmada to place selected resources into specific member clusters.

### Scheduling Behavior

Karmada Scheduler evaluates:
- cluster availability
- affinity rules
- taints and tolerations
- spread constraints
- failover rules
- replica scheduling preferences

The scheduler creates corresponding `ResourceBinding` objects.

## Minimal Example

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx

  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
```

This policy propagates the `nginx` Deployment to:
- `member1`
- `member2`

## Placement Strategies

### Explicit Cluster Placement

Directly specify target clusters.

```yaml
placement:
  clusterAffinity:
    clusterNames:
      - member1
      - member2
```

Useful for:
- controlled environments
- testing
- static topology

### Label-Based Placement

Clusters can be selected using labels.

Example:

```yaml
placement:
  clusterAffinity:
    labelSelector:
      matchLabels:
        region: ap-southeast
```

Useful for:
- regional placement
- dynamic scaling
- environment grouping

### Replica Scheduling

Replica scheduling distributes workload replicas across clusters.

Example:

```yaml
placement:
  replicaScheduling:
    replicaSchedulingType: Divided
    replicaDivisionPreference: Weighted
```

Common strategies:
- `Duplicated`
- `Divided`

## Related Karmada Resources

### ResourceBinding

`ResourceBinding` stores scheduling decisions generated from a `PropagationPolicy`.

It contains:
- selected clusters
- replica assignments
- propagation state

### Work

`Work` objects contain Kubernetes manifests propagated to member clusters.

Each target cluster receives corresponding `Work` resources.

### OverridePolicy

`OverridePolicy` customizes cluster-specific configuration during propagation.

Examples:
- image registry replacement
- StorageClass override
- environment-specific settings

## Common Use Cases

### Multi-Cluster Deployment

Deploy workloads across multiple Kubernetes clusters.

### Geo-Distributed Applications

Place workloads into clusters from multiple regions or countries.

### High Availability

Replicate workloads across clusters for failover resilience.

### Edge Computing

Distribute workloads to edge clusters close to users or devices.

## Common Mistakes

### Resource Selector Mismatch

Incorrect resource selectors prevent policy matching.

Example issue:
- wrong resource name
- wrong API version
- wrong namespace scope

Check:
```bash
kubectl get deployment
```

### Cluster Does Not Exist

Propagation fails if referenced clusters are not joined to Karmada.

Check:

```bash
kubectl get clusters
```

### Missing Scheduler Decision

If scheduling fails:
- no `ResourceBinding` may be created
- propagation stops before `Work` generation

Check:

```bash
kubectl get resourcebinding
```

### Namespace Propagation Issues

Resources depending on namespaces may fail if namespace propagation is missing.

### Invalid Placement Rules

Conflicting affinity or spread constraints can block scheduling.

## Troubleshooting Workflow

### Step 1 — Verify Policy

```bash
kubectl get propagationpolicy
kubectl describe propagationpolicy <policy-name>
```

### Step 2 — Verify ResourceBinding

```bash
kubectl get resourcebinding
kubectl describe resourcebinding <binding-name>
```

### Step 3 — Verify Work Objects

```bash
kubectl get work -A
```

### Step 4 — Verify Member Cluster Resources

Switch kubeconfig context:

```bash
kubectl config use-context member1
```

Then inspect resources:

```bash
kubectl get deployment
```

## Best Practices

### Use Label-Based Placement

Prefer cluster labels over hardcoded cluster names when possible.

### Keep Policies Small and Focused

Avoid overly broad selectors.

### Combine With OverridePolicy

Use `OverridePolicy` for cluster-specific customization instead of duplicating workloads.

### Validate Placement Early

Test policies in development clusters before production rollout.

## Example: Deployment + Policy

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx
```

### PropagationPolicy

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-policy
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx

  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
```

## Related Components

The following Karmada components participate in propagation:

- Karmada API Server
- Karmada Controller Manager
- Karmada Scheduler
- Execution Controller
- Binding Controller
- Cluster Controller

## References

Useful references:
- Karmada policy documentation
- Scheduler documentation
- ResourceBinding documentation
- OverridePolicy documentation
- Multi-cluster scheduling guides