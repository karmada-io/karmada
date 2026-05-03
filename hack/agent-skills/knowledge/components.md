# Karmada Components Guide

## Core Components

### karmada-apiserver
- HTTP API server providing the Karmada control plane API.
- Exposes all Karmada CRDs (PropagationPolicy, OverridePolicy, ResourceBinding, Work, etc.).
- Runs in `karmada-system` namespace.
- Port: typically 5443 (internal), exposes policy.karmada.io and work.karmada.io API groups.

### karmada-controller-manager
- Runs multiple controllers:
  - **Cluster controller**: Manages cluster lifecycle (join, unjoin, health).
  - **Binding controller**: Creates ResourceBinding/ClusterResourceBinding from matched policies.
  - **Execution controller**: Creates Work objects from ResourceBindings for each target cluster.
  - **Override controller**: Applies OverridePolicy rules to workloads.
  - **Namespace controller**: Auto-propagates namespaces (unless skipped).
  - **Taint manager**: Handles cluster taints and eviction.
  - **Graceful eviction controller**: Manages graceful eviction tasks.
  - **FederatedResourceQuota controller**: Manages federated resource quotas.
- Runs in `karmada-system` namespace.
- Logs: `kubectl logs -n karmada-system deployment/karmada-controller-manager`

### karmada-scheduler
- Schedule resources to member clusters based on Placement rules.
- Evaluates cluster affinity, tolerations, spread constraints, replica scheduling.
- Assigns clusters in ResourceBinding.spec.clusters.
- Supports weighted, aggregated, and dynamic replica distribution.
- Runs in `karmada-system` namespace.
- Logs: `kubectl logs -n karmada-system deployment/karmada-scheduler`

### karmada-agent
- Runs on each member cluster.
- Syncs Work objects from Karmada control plane to the member cluster.
- Reports status back (ManifestStatus, Health).
- Runs in `karmada-system` namespace on each member cluster.
- Logs: `kubectl logs -n karmada-system deployment/karmada-agent` (on member cluster)

### karmada-webhook
- Admission webhooks for Karmada CRDs.
- Validates PropagationPolicy, OverridePolicy, ResourceBinding, etc.
- Runs in `karmada-system` namespace.

### karmada-aggregated-apiserver
- Aggregated API server extending the Kubernetes API with Karmada-specific resources.
- Provides the `cluster.karmada.io` API group for Cluster resources.
- Runs in `karmada-system` namespace.

### etcd
- Backing store for the Karmada API server.
- Runs in `karmada-system` namespace (when using embedded etcd).

### karmada-search
- Optional component for cross-cluster resource search.
- Provides `search.karmada.io` API group.
- Allows searching resources across all member clusters from a single endpoint.

## Component Interaction Flow

```
User creates Deployment → karmada-apiserver stores it
    ↓
Binding controller detects Deployment matches PropagationPolicy
    ↓
Binding controller creates ResourceBinding with placement from policy
    ↓
Override controller applies OverridePolicy modifications to ResourceBinding
    ↓
karmada-scheduler assigns clusters to ResourceBinding.spec.clusters
    ↓
Execution controller creates Work objects (one per target cluster)
    ↓
karmada-agent on each member cluster syncs Work → creates resource locally
    ↓
karmada-agent reports status back to Work.status
    ↓
Binding controller aggregates status to ResourceBinding.status
```

## Key Namespaces

| Namespace | Purpose |
|-----------|---------|
| `karmada-system` | Karmada control plane components |
| `karmada-cluster` | Cluster registration resources |
| `karmada-es-<cluster>` | Execution space: Work objects for each member cluster |

## System Namespaces (Excluded from ClusterPropagationPolicy)

ClusterPropagationPolicy cannot match resources in:
- `karmada-system`
- `karmada-cluster`
- `karmada-es-*`

These namespaces are reserved for Karmada's internal operation.

## Feature Gates

| Feature Gate | Description | Stage |
|---|---|---|
| `PropagateDeps` | Auto-propagate dependent resources | Beta / GA |
| `Failover` | Application and cluster failover | Beta |
| `GracefulEviction` | Graceful eviction of workloads | Beta |
| `MultiplePodTemplatesScheduling` | Schedule workloads with multiple pod templates | Alpha |
| `StatefulFailoverInjection` | Preserve state during failover | Alpha |
| `PriorityBasedScheduling` | Priority-based workload scheduling | Alpha |
| `PriorityBasedPreemptiveScheduling` | Preemptive scheduling based on priority | Alpha (not yet implemented) |
| `Flux2GitopsAdaption` | Integration with Flux2 GitOps | Alpha |
| `ResourceQuotaEstimate` | Resource quota estimation | Alpha |
