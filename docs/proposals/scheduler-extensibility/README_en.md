---
title: Enhancing Karmada Scheduler Extensibility to Support Customized Requirements
authors:
- "charesQQ"
reviewers:
- TBD
approvers:
- TBD

creation-date: 2025-12-25

---

# Enhancing Karmada Scheduler Extensibility to Support Customized Requirements

## Summary

While the current Karmada scheduler provides powerful multi-cluster scheduling capabilities, there are some limitations in meeting customized scheduling requirements when adopting Karmada in enterprise environments. This proposal aims to enhance the extensibility architecture of the Karmada scheduler to better support:

1. **API Flexibility**: For example, specifying replica counts by IDC/region instead of by weight
2. **AssignReplicas Requirements**: For example, average replica allocation with change minimization under specified IDC/region constraints, rather than score-based allocation
3. **Advanced Scheduling Constraints**: For example, scheduling decisions that depend on service attributes and other profiling data

This proposal presents an extensible scheduling framework enhancement that allows users to extend scheduler capabilities through a standardized plugin mechanism without modifying the core scheduler code. This design maintains the generality of the Karmada scheduler while meeting the customization needs of enterprise users.

## Motivation

### Background

### Goals

The goal of this proposal is to provide a **generic, extensible scheduling framework enhancement** that enables:

1. **Plugin-based Extension**: Users can extend scheduler capabilities by implementing standard plugin interfaces without modifying core code
2. **Backward Compatibility**: Existing scheduling configurations and behaviors remain unchanged, with new features enabled through optional mechanisms

### Non-Goals

The following items are out of scope for this proposal:

1. **Scheduler Performance Optimization**: While the plugin mechanism considers performance, large-scale performance optimization is not the primary goal
2. **Scheduling Algorithm Improvements**: This proposal focuses on extensibility architecture rather than specific scheduling algorithm improvements

This proposal presents a **solution to add extension points and flexible API fields**. The core idea is to enrich APIs and add new extension points while maintaining the existing scheduling flow.

### User Stories

Based on the 4 types of strategies actually supported, we provide the following user stories:

#### Story 1: Automatic Scheduling by Specified IDCs (Based on Strategy 1: Idcs)

**Background**: I manage a microservice application that needs to be deployed in East China and North China IDCs, but I'm uncertain about the exact replica distribution between IDCs.

**Requirements**:
- Specify target IDCs: East China (idc-east) and North China (idc-north)
- Let the scheduler automatically distribute replicas based on cluster resource utilization
- Prioritize clusters with sufficient resources to achieve load balancing

**Current Implementation**:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: microservice-policy
  annotations:
    scheduler.karmada.io/replica-scheduling-strategy: |
      {
        "idcs": [
          {"name": "idc-east"},
          {"name": "idc-north"}
        ]
      }
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: microservice-app
  placement:
    clusterAffinity:
      labelSelector:
        matchLabels:
          env: production
    replicaScheduling:
      replicaSchedulingType: Divided
  replicas: 30
```

**Scheduling Result Example**:
- idc-east-cluster-1: 12 replicas (60% CPU utilization)
- idc-east-cluster-2: 8 replicas (75% CPU utilization)
- idc-north-cluster-1: 10 replicas (65% CPU utilization)

**Alternative Using Karmada SpreadConstraints**:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: microservice-policy-spread
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: microservice-app
  placement:
    clusterAffinity:
      labelSelector:
        matchLabels:
          env: production
    spreadConstraints:
      - spreadByLabel: topology.karmada.io/idc
        maxGroups: 2
        minGroups: 2
    replicaScheduling:
      replicaSchedulingType: Divided
  replicas: 30
```

**Explanation**:
- `spreadByLabel: topology.karmada.io/idc`: Distribute by IDC label dimension
- `minGroups: 2`: Ensure deployment in at least 2 IDCs
- `maxGroups: 2`: Limit deployment to at most 2 IDCs
- Prerequisite: Clusters need labels `topology.karmada.io/idc`, for example:
  - East China clusters: `topology.karmada.io/idc: idc-east`
  - North China clusters: `topology.karmada.io/idc: idc-north`
- The scheduler automatically distributes replicas between the two IDCs, with specific allocation determined by cluster resource conditions

---

#### Story 2: Specified IDCs with Replica Counts (Based on Strategy 2: SpecifiedIdcs)

**Background**: I need to ensure my core business deploys 20 replicas in East China IDC and 10 replicas in North China IDC (to meet disaster recovery requirements).

**Requirements**:
- East China IDC: 20 replicas
- North China IDC: 10 replicas
- Within each IDC, the scheduler automatically distributes based on cluster resources

**Use Case Example**: A recommendation service depends on a third-party service that has 2x more instances in East China IDC than in North China IDC, while also requiring high availability across IDCs. For example, if the third-party service has 100 instances in East China and 50 in North China, the recommendation service needs to deploy in the same ratio (2:1) across both IDCs to ensure call efficiency and high availability while avoiding cross-IDC call latency.

**Current Implementation**:
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: core-service-policy
  annotations:
    scheduler.karmada.io/replica-scheduling-strategy: |
      {
        "specifiedIdcs": [
          {"name": "idc-east", "replicas": 20},
          {"name": "idc-north", "replicas": 10}
        ]
      }
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: core-service
  placement:
    clusterAffinity:
      labelSelector:
        matchLabels:
          env: production
    replicaScheduling:
      replicaSchedulingType: Divided
  replicas: 30
```

**Scheduling Result Example**:
- idc-east-cluster-1: 8 replicas
- idc-east-cluster-2: 7 replicas
- idc-east-cluster-3: 5 replicas
- idc-north-cluster-1: 6 replicas
- idc-north-cluster-2: 4 replicas

---

#### Story 3: Balanced Scheduling within Specified IDCs (Based on Strategy 3: SpecifiedBalancedIdcs)

**Background**: My application requires high availability deployment with balanced replica distribution across all clusters within specified IDCs.

**Requirements**:
- East China IDC: 20 replicas, balanced across 4 clusters
- North China IDC: 10 replicas, balanced across 2 clusters
- Minimize replica count differences between clusters

**Use Case Example**: In overseas business regions, inter-cluster high availability under IDC constraints is the highest priority, ensuring balanced instance distribution across clusters to prevent service instability when a cluster fails.

**Current Implementation**:
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: ha-service-policy
  annotations:
    scheduler.karmada.io/replica-scheduling-strategy: |
      {
        "specifiedBalancedIdcs": [
          {"name": "idc-east", "replicas": 20},
          {"name": "idc-north", "replicas": 10}
        ]
      }
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: ha-service
  placement:
    clusterAffinity:
      labelSelector:
        matchLabels:
          env: production
    replicaScheduling:
      replicaSchedulingType: Divided
  replicas: 30
```

**Scheduling Result Example** (balanced distribution):
- idc-east-cluster-1: 5 replicas
- idc-east-cluster-2: 5 replicas
- idc-east-cluster-3: 5 replicas
- idc-east-cluster-4: 5 replicas
- idc-north-cluster-1: 5 replicas
- idc-north-cluster-2: 5 replicas

---

#### Story 4: Specified Clusters with Replica Counts (Based on Strategy 4: SpecifiedClusters)

**Background**: My core trading system has strict regulatory requirements and must precisely control replica counts for each cluster.

**Requirements**:
- bj-prod-cluster: Must have 10 replicas (regulatory requirement)
- sh-prod-cluster: Must have 8 replicas (business requirement)
- gz-dr-cluster: Must have 5 replicas (disaster recovery requirement)

**Use Case Example: Migrating from KubeFed to Karmada**

A company previously managed multi-cluster applications using KubeFed and now plans to migrate to Karmada. In KubeFed, applications have been running stably for years across clusters with specific scheduling topology:
- cluster-prod-1: 15 replicas (optimized configuration over time)
- cluster-prod-2: 12 replicas (adjusted based on historical load)
- cluster-prod-3: 8 replicas (considering network latency and user distribution)
- cluster-dr: 5 replicas (disaster recovery cluster)

**Migration Requirement**: Cannot change replica counts across clusters during migration to avoid introducing risks

**Current Implementation**:
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: trading-system-policy
  annotations:
    scheduler.karmada.io/replica-scheduling-strategy: |
      {
        "specifiedClusters": [
          {"name": "bj-prod-cluster", "replicas": 10},
          {"name": "sh-prod-cluster", "replicas": 8},
          {"name": "gz-dr-cluster", "replicas": 5}
        ]
      }
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: trading-system
  placement:
    clusterAffinity:
      clusterNames:
        - bj-prod-cluster
        - sh-prod-cluster
        - gz-dr-cluster
    replicaScheduling:
      replicaSchedulingType: Divided
  replicas: 23
```

**Scheduling Result** (exact match):
- bj-prod-cluster: 10 replicas
- sh-prod-cluster: 8 replicas
- gz-dr-cluster: 5 replicas

---

**KubeFed Migration Configuration Example**:

```yaml
# Original KubeFed Configuration (reference)
apiVersion: types.kubefed.io/v1beta1
kind: FederatedDeployment
metadata:
  name: my-app
spec:
  template:
    spec:
      replicas: 40  # Total replicas
  overrides:
    - clusterName: cluster-prod-1
      clusterOverrides:
        - path: "/spec/replicas"
          value: 15
    - clusterName: cluster-prod-2
      clusterOverrides:
        - path: "/spec/replicas"
          value: 12
    - clusterName: cluster-prod-3
      clusterOverrides:
        - path: "/spec/replicas"
          value: 8
    - clusterName: cluster-dr
      clusterOverrides:
        - path: "/spec/replicas"
          value: 5

---
# Karmada Configuration After Migration (maintaining scheduling topology)
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: my-app-migration-policy
  annotations:
    scheduler.karmada.io/replica-scheduling-strategy: |
      {
        "specifiedClusters": [
          {"name": "cluster-prod-1", "replicas": 15},
          {"name": "cluster-prod-2", "replicas": 12},
          {"name": "cluster-prod-3", "replicas": 8},
          {"name": "cluster-dr", "replicas": 5}
        ]
      }
    migration.karmada.io/source: "kubefed"
    migration.karmada.io/date: "2025-01-05"
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: my-app
  placement:
    clusterAffinity:
      clusterNames:
        - cluster-prod-1
        - cluster-prod-2
        - cluster-prod-3
        - cluster-dr
    replicaScheduling:
      replicaSchedulingType: Divided
  replicas: 40
```

#### Story 5: Integration with CronHPA for Dynamic Scheduling (Custom Feature)

**Background**: Our e-commerce system requires dynamic scaling during promotional periods and needs to propagate deployments to mixed clusters.

**Requirements**:
- Normal operation with low traffic: Deploy only in self-built IDCs
- During promotions: Auto-scale using CronHPA, scheduling scaled replicas to cloud vendor clusters (mixed-type clusters)
- After promotions: Auto-scale down, cloud vendor cluster replicas return to zero, replica counts not controlled by Karmada

**Current Implementation**:
Custom scheduler detects CronHPA resource changes through `cronhpaChanged` and automatically schedules deployments to `mixed` type clusters.

```yaml
# CronHPA Configuration
apiVersion: autoscaling/v1alpha1
kind: CronHPA
metadata:
  name: ecommerce-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: ecommerce-app
  rules:
    - name: scale-up
      schedule: "0 18 * * *"  # Scale up at 18:00 daily
      targetReplicas: 50
    - name: scale-down
      schedule: "0 2 * * *"   # Scale down at 02:00 daily
      targetReplicas: 0
---
# PropagationPolicy Configuration
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: ecommerce-policy
  annotations:
    scheduler.karmada.io/replica-scheduling-strategy: |
      {
        "specifiedIdcs": [
          {"name": "idc-self", "replicas": 30}
        ]
      }
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: ecommerce-app
  placement:
    clusterAffinity:
      labelSelector:
        matchLabels:
          env: production
    replicaScheduling:
      replicaSchedulingType: Divided
```

**Scheduling Result**:
- **Normal operation (30 replicas)**:
  - idc-self-cluster-1: 15 replicas
  - idc-self-cluster-2: 15 replicas
- **During promotion (50 replicas)**:
  - idc-self-cluster-1: 15 replicas
  - idc-self-cluster-2: 15 replicas
  - mixed-cloud-cluster: 0 replicas (automatically assigned to mixed cluster, cooperating with interpreter webhook retain feature, not modifying spec.replicas, controlled by member cluster)

---

#### Story 6: NodeLabels Plugin Filters Clusters Without Matching Nodes (Custom Feature)

**Background**: My application needs to run on nodes with specific CPU architectures (like ARM) or specific node labels.

**Requirements**:
- Only schedule to clusters with ARM nodes
- Verify node labels before scheduling to avoid scheduling failures

**Current Implementation**:
Custom scheduler's `NodeLabels` plugin checks during Filter phase whether clusters have nodes satisfying nodeSelector.

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: arm-app-policy
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: arm-app
  placement:
    clusterAffinity:
      labelSelector:
        matchLabels:
          arch: arm64
    replicaScheduling:
      replicaSchedulingType: Divided
---
# Deployment Configuration
apiVersion: apps/v1
kind: Deployment
metadata:
  name: arm-app
spec:
  template:
    spec:
      nodeSelector:
        kubernetes.io/arch: arm64  # NodeLabels plugin checks this label
```

**Scheduling Behavior**:
- Scheduler filters out clusters without ARM nodes
- Only schedules to clusters with `kubernetes.io/arch: arm64` nodes

---

**Scheduling Flow and Extension Points**:

```
┌─────────────────────────────────────────────────────────────────┐
│                 Scheduling Flow and Extension Points              │
└─────────────────────────────────────────────────────────────────┘

1. Filter Phase (Karmada Native)
   ├─ Purpose: Filter clusters that don't meet conditions
   └─ Existing plugins: ClusterAffinity, TaintToleration, etc.

2. Score Phase (Karmada Native)
   ├─ Purpose: Score candidate clusters
   └─ Existing plugins: ClusterLocality, ClusterAffinity, etc.

3. 🌟 AssignReplicas Phase (Core Extension Point - New)
   ├─ Extension Point: AssignReplicasPlugin
   ├─ Description: **Replaces Karmada native assignReplicas function**
   ├─ Execution: After Filter and Score
   ├─ Input: Filtered and scored cluster list, total replica count
   ├─ Output: Specific replica count allocation for each cluster
   └─ Use Cases: **Custom replica allocation logic**, such as:
      • Specified replica counts: Precisely specify replica counts for each IDC/cluster
      • External system integration: CronHPA, quota management, cost optimization, etc.
      • Complex allocation strategies: Change minimization, resource utilization-based, etc.
      • Special requirements: Mixed cluster support, multi-level allocation, etc.
```

## Design Details

### API Design

#### 1. PropagationPolicy API Extension

```go
// PropagationPolicy Extension
type PropagationSpec struct {
    // ... existing fields ...
    
    // AdvancedScheduling provides advanced scheduling configuration capabilities
    // Key: Scheduling strategy name (e.g., "specified-idcs", "cronhpa-config", etc.)
    // Value: Strategy configuration (JSON format, parsed by plugins)
    // +optional
    AdvancedScheduling map[string]runtime.RawExtension `json:"advancedScheduling,omitempty"`
}
```

**Design Notes**:
- ✅ **Minimal Design**: Map structure, key is strategy name, value is configuration
- ✅ **Maximum Flexibility**: Plugins can define arbitrary configuration formats
- ✅ **Easy to Extend**: Adding new strategies only requires adding new keys
- ✅ **Backward Compatible**: Can smoothly migrate existing annotation configurations

---

#### 2. Plugin Interface Definition

This proposal introduces the `AssignReplicasPlugin` extension point, **used to replace Karmada native `assignReplicas` function**.

##### 2.1 Core Interface Definition

```go
package framework

import (
    "context"
    
    clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
    workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
    "k8s.io/apimachinery/pkg/runtime"
)

// Plugin parent interface for all scheduling plugins
type Plugin interface {
    Name() string
}

// 🌟 AssignReplicasPlugin replica allocation plugin (core extension point)
// Used to replace Karmada native assignReplicas function
// Executes after Filter and Score phases, responsible for allocating replicas to specific clusters
//
// ⚠️ Important: Multiple AssignReplicasPlugins can be registered, but only one can be active/used
// Because this plugin returns the final replica allocation result, does not support chaining multiple plugins
type AssignReplicasPlugin interface {
    Plugin
    
    // AssignReplicas allocates replicas to clusters based on custom logic
    AssignReplicas(
        ctx context.Context,
        binding *workv1alpha2.ResourceBinding,
        clusters []ClusterWithScore,
        totalReplicas int32,
    ) ([]TargetCluster, error)
}

// ClusterWithScore cluster information with score
type ClusterWithScore struct {
    Cluster *clusterv1alpha1.Cluster
    Score   int64  // Score from Score phase
}

// TargetCluster target cluster and replica count
type TargetCluster struct {
    Cluster  *clusterv1alpha1.Cluster
    Replicas int32
}

// PluginFactory plugin factory function
type PluginFactory func(configuration runtime.Object) (Plugin, error)

// Register plugin
func RegisterPlugin(name string, factory PluginFactory) {
    // Implementation omitted
}
```

##### 2.2 Detailed Explanation of AssignReplicas Method

**Method Signature**:
```go
AssignReplicas(
    ctx context.Context,
    binding *workv1alpha2.ResourceBinding,
    clusters []ClusterWithScore,
    totalReplicas int32,
) ([]TargetCluster, error)
```

**Input Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `ctx` | `context.Context` | Context object for passing request-scoped values, cancellation signals, and deadlines |
| `binding` | `*workv1alpha2.ResourceBinding` | Resource binding object containing scheduling strategy configuration |
| `clusters` | `[]ClusterWithScore` | Candidate cluster list after Filter and Score phases, sorted by score (high to low) |
| `totalReplicas` | `int32` | Total number of replicas to allocate |

**Parameter Details**:

1. **ctx (context.Context)**
   - Standard Go context object
   - Purposes:
     - Pass request-scoped values (e.g., trace ID, user information)
     - Support timeout control and cancellation
     - Plugins can use `ctx.Done()` to check if early exit is needed
   - Example:
     ```go
     select {
     case <-ctx.Done():
         return nil, ctx.Err()
     default:
         // Continue execution
     }
     ```

2. **binding (*workv1alpha2.ResourceBinding)**
   - ResourceBinding object containing complete PropagationPolicy configuration
   - Key fields:
     - `binding.Spec.Resource`: Resource information to schedule (e.g., Deployment namespace, name)
     - `binding.Spec.Replicas`: Total replica count (matches `totalReplicas` parameter)
     - `binding.Spec.Placement`: Scheduling configuration (clusterAffinity, spreadConstraints, etc.)
     - `binding.Spec.AdvancedScheduling`: **Custom scheduling configuration** (plugins read configuration from here)
   - Plugins can read custom configuration from `binding.Spec.AdvancedScheduling` map:
     ```go
     config, exists := binding.Spec.AdvancedScheduling["specified-idcs"]
     if exists {
         // Parse and use custom configuration
         var idcConfigs []IdcReplicaConfig
         json.Unmarshal(config.Raw, &idcConfigs)
     }
     ```

3. **clusters ([]ClusterWithScore)**
   - Cluster list processed through Filter and Score phases
   - Clusters not meeting conditions already filtered out (e.g., insufficient resources, not meeting affinity)
   - Sorted by Score in descending order
   - `ClusterWithScore` structure:
     ```go
     type ClusterWithScore struct {
         Cluster *clusterv1alpha1.Cluster  // Cluster object
         Score   int64                      // Scheduling score (higher is better)
     }
     ```
   - Cluster object key information:
     - `Cluster.Name`: Cluster name
     - `Cluster.Labels`: Cluster labels (e.g., `topology.karmada.io/idc: idc-east`)
     - `Cluster.Spec.Taints`: Cluster taints
     - `Cluster.Status.Conditions`: Cluster status (Ready, ResourceInsufficient, etc.)
   - Note:
     - If Filter phase filtered out all clusters, `clusters` may be empty
     - Plugins should handle empty list cases

4. **totalReplicas (int32)**
   - Total number of replicas to allocate
   - From PropagationPolicy's `spec.replicas` field
   - Plugins should ensure allocated replica count sum equals `totalReplicas` (unless special requirements)

**Return Values**:

| Return Value | Type | Description |
|--------------|------|-------------|
| First return | `[]TargetCluster` | Replica allocation result, specifying replica count for each cluster |
| Second return | `error` | Error message, returns `nil` if allocation successful |

**Return Value Details**:

1. **[]TargetCluster**
   - Final result of replica allocation
   - `TargetCluster` structure:
     ```go
     type TargetCluster struct {
         Cluster  *clusterv1alpha1.Cluster  // Target cluster object
         Replicas int32                      // Replica count allocated to this cluster
     }
     ```
   - Requirements:
     - Sum of all `Replicas` should equal `totalReplicas`
     - `Replicas` should be greater than 0 (clusters with 0 replicas can be excluded from results)
     - Cluster objects should come from input `clusters` list
   - Example:
     ```go
     return []TargetCluster{
         {Cluster: clusters[0].Cluster, Replicas: 15},
         {Cluster: clusters[1].Cluster, Replicas: 10},
         {Cluster: clusters[2].Cluster, Replicas: 5},
     }, nil
     ```

2. **error**
   - Returns `nil` if allocation successful
   - Returns specific error message if allocation fails
   - Common error scenarios:
     - Configuration parsing failure: `fmt.Errorf("failed to parse config: %w", err)`
     - Insufficient clusters: `fmt.Errorf("not enough clusters: need %d, got %d", required, len(clusters))`
     - Replica count mismatch: `fmt.Errorf("total replicas mismatch: expected %d, got %d", totalReplicas, actualTotal)`
     - Cluster not found: `fmt.Errorf("cluster %s not found", clusterName)`

**Usage Constraints**:

1. **Replica Sum Constraint**:
   - Typically, allocated replica sum should equal `totalReplicas`
   - Special cases (e.g., mixed clusters, Retain mode) can differ from `totalReplicas`

2. **Cluster Source Constraint**:
   - `Cluster` objects in returned `TargetCluster` should come from input `clusters` list
   - Should not create new cluster objects or select clusters that didn't pass Filter

3. **Idempotency Requirement**:
   - Same input should produce same output (deterministic scheduling)
   - If randomness needed, use deterministic random seed

4. **Performance Requirement**:
   - Plugins should execute efficiently, avoiding time-consuming operations
   - If external system access needed, recommend using caching
   - Avoid blocking operations in plugins

##### 2.3 Design Considerations

**Default Plugin and Custom Replacement**:
- Karmada provides a **default plugin `DefaultAssignReplicasPlugin`** implementing current native `assignReplicas` logic
- Users can **replace** the default plugin by registering custom plugins
- Scheduler registers plugins at startup, specified via `--plugins` parameter

**⚠️ Single Plugin Constraint**:
- **Multiple AssignReplicasPlugins can be registered, but only one can be active/used**
- Reason: This plugin returns **final replica allocation result**, does not support chaining multiple plugins
- If multiple allocation strategies needed, implement multiple logics within one plugin, selecting via `AdvancedScheduling` configuration

**Plugin Reading Custom Configuration Example**:
   ```go
   // Example: Plugin reading configuration from AdvancedScheduling
   func (p *MyPlugin) AssignReplicas(
       ctx context.Context,
       binding *workv1alpha2.ResourceBinding,
       clusters []ClusterWithScore,
       totalReplicas int32,
   ) ([]TargetCluster, error) {
       // Read configuration from AdvancedScheduling map
       config, exists := binding.Spec.AdvancedScheduling["specified-idcs"]
       if !exists {
           // Use default logic or return error
           return defaultAssign(clusters, totalReplicas), nil
       }
       
       // Parse configuration
       var idcConfigs []IdcReplicaConfig
       if err := json.Unmarshal(config.Raw, &idcConfigs); err != nil {
           return nil, err
       }
       
       // Allocate replicas based on configuration
       return p.assignByIdcConfig(clusters, totalReplicas, idcConfigs)
   }
   ```

---

#### 3. Scheduler Invocation Flow

The scheduler calls `AssignReplicasPlugin` after Filter and Score to allocate replicas:

```go
// Pseudocode: Implementation in scheduler
func (s *Scheduler) scheduleBinding(binding *workv1alpha2.ResourceBinding) error {
    // 1. Filter Phase: Filter out clusters not meeting conditions
    feasibleClusters := s.runFilterPlugins(binding)
    
    // 2. Score Phase: Score candidate clusters
    clustersWithScore := s.runScorePlugins(binding, feasibleClusters)
    
    // 3. AssignReplicas Phase: Allocate replicas (new extension point)
    // 🌟 Call registered AssignReplicasPlugin (default or custom)
    targetClusters, err := s.runAssignReplicasPlugin(binding, clustersWithScore, totalReplicas)
    if err != nil {
        return err
    }
    
    // 4. Update ResourceBinding
    return s.updateBinding(binding, targetClusters)
}
```

**Key Points**:
- ✅ **Default Implementation**: Karmada provides default plugin implementing current native logic
- ✅ **Flexible Replacement**: Users can register custom plugins to completely replace default implementation
- ✅ **Single Responsibility**: Plugin only handles replica allocation, doesn't affect Filter and Score
- ⚠️ **Single Plugin**: Multiple `AssignReplicasPlugin` can be registered but only one can be active/used, as it returns final scheduling result

---

#### 4. Plugin Implementation Examples

**Example 1: SpecifiedIdcsPlugin - Specified IDCs with Replica Counts**

```go
package plugins

import (
    "context"
    "encoding/json"
    "fmt"
    
    clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
    workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
    "github.com/karmada-io/karmada/pkg/scheduler/framework"
)

const SpecifiedIdcsPluginName = "SpecifiedIdcs"

// SpecifiedIdcsPlugin implements allocation strategy for specified IDCs with replica counts
type SpecifiedIdcsPlugin struct{}

func (p *SpecifiedIdcsPlugin) Name() string {
    return SpecifiedIdcsPluginName
}

// IdcReplicaConfig IDC replica configuration
type IdcReplicaConfig struct {
    Name     string `json:"name"`
    Replicas int32  `json:"replicas"`
}

// AssignReplicas allocates replicas based on specified IDC configuration
func (p *SpecifiedIdcsPlugin) AssignReplicas(
    ctx context.Context,
    binding *workv1alpha2.ResourceBinding,
    clusters []framework.ClusterWithScore,
    totalReplicas int32,
) ([]framework.TargetCluster, error) {
    
    // 1. Read configuration from AdvancedScheduling
    config, exists := binding.Spec.AdvancedScheduling["specified-idcs"]
    if !exists {
        return nil, fmt.Errorf("specified-idcs config not found")
    }
    
    // 2. Parse configuration
    var idcConfigs []IdcReplicaConfig
    if err := json.Unmarshal(config.Raw, &idcConfigs); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }
    
    // 3. Group clusters by IDC
    idcClusters := make(map[string][]framework.ClusterWithScore)
    for _, cluster := range clusters {
        idc := cluster.Cluster.Labels["topology.karmada.io/idc"]
        if idc != "" {
            idcClusters[idc] = append(idcClusters[idc], cluster)
        }
    }
    
    // 4. Allocate replicas based on configuration
    targetClusters := make([]framework.TargetCluster, 0)
    
    for _, idcConfig := range idcConfigs {
        clustersInIdc, ok := idcClusters[idcConfig.Name]
        if !ok || len(clustersInIdc) == 0 {
            return nil, fmt.Errorf("no clusters found in idc %s", idcConfig.Name)
        }
        
        // Allocate replicas within IDC based on resource utilization
        assigned := p.assignToIdcClusters(clustersInIdc, idcConfig.Replicas)
        targetClusters = append(targetClusters, assigned...)
    }
    
    return targetClusters, nil
}

// assignToIdcClusters allocates replicas among clusters within IDC
func (p *SpecifiedIdcsPlugin) assignToIdcClusters(
    clusters []framework.ClusterWithScore,
    replicas int32,
) []framework.TargetCluster {
    // Sort by cluster score (resource utilization), prioritize resource-abundant clusters
    // Implementation omitted...
    return []framework.TargetCluster{}
}

// Factory function
func NewSpecifiedIdcsPlugin(config runtime.Object) (framework.Plugin, error) {
    return &SpecifiedIdcsPlugin{}, nil
}

func init() {
    framework.RegisterPlugin(SpecifiedIdcsPluginName, NewSpecifiedIdcsPlugin)
}
```

**Example 2: SpecifiedClustersPlugin - Specified Clusters with Replica Counts**

```go
// SpecifiedClustersPlugin implements allocation strategy for specified clusters with replica counts
type SpecifiedClustersPlugin struct{}

type ClusterReplicaConfig struct {
    Name     string `json:"name"`
    Replicas int32  `json:"replicas"`
}

func (p *SpecifiedClustersPlugin) AssignReplicas(
    ctx context.Context,
    binding *workv1alpha2.ResourceBinding,
    clusters []framework.ClusterWithScore,
    totalReplicas int32,
) ([]framework.TargetCluster, error) {
    
    // 1. Read configuration
    config, exists := binding.Spec.AdvancedScheduling["specified-clusters"]
    if !exists {
        return nil, fmt.Errorf("specified-clusters config not found")
    }
    
    var clusterConfigs []ClusterReplicaConfig
    if err := json.Unmarshal(config.Raw, &clusterConfigs); err != nil {
        return nil, err
    }
    
    // 2. Build cluster mapping
    clusterMap := make(map[string]*clusterv1alpha1.Cluster)
    for _, c := range clusters {
        clusterMap[c.Cluster.Name] = c.Cluster
    }
    
    // 3. Allocate directly based on configuration
    targetClusters := make([]framework.TargetCluster, 0)
    for _, config := range clusterConfigs {
        cluster, ok := clusterMap[config.Name]
        if !ok {
            return nil, fmt.Errorf("cluster %s not found or filtered out", config.Name)
        }
        
        targetClusters = append(targetClusters, framework.TargetCluster{
            Cluster:  cluster,
            Replicas: config.Replicas,
        })
    }
    
    return targetClusters, nil
}
```

---

### API Usage Examples

#### Example 1: Specified IDCs with Replica Counts (specified-idcs)

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: core-service-policy
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: core-service
  placement:
    clusterAffinity:
      labelSelector:
        matchLabels:
          env: production
  advancedScheduling:
    specified-idcs:
      - name: "idc-east"
        replicas: 20
      - name: "idc-north"
        replicas: 10
```

#### Example 2: Specified Clusters with Replica Counts (specified-clusters)

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: precise-allocation-policy
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: my-app
  advancedScheduling:
    specified-clusters:
      - name: "cluster-1"
        replicas: 15
      - name: "cluster-2"
        replicas: 10
      - name: "cluster-3"
        replicas: 5
```

---

## Pros and Cons Analysis

### ✅ Advantages

**1. High Flexibility**
- Users can implement custom `AssignReplicasPlugin` to completely replace default behavior
- Supports arbitrarily complex replica allocation logic (specified replica counts, change minimization, cost optimization, etc.)
- Can integrate with external systems (CronHPA, quota management, monitoring systems, etc.)

**2. Backward Compatibility**
- Default uses `DefaultAssignReplicasPlugin`, maintaining native behavior unchanged
- Users can optionally enable custom plugins
- Does not affect existing users

**3. Extensibility**
- Through `AdvancedScheduling` map field, plugins can define arbitrary configuration formats
- Can support new scheduling strategies without modifying Karmada API

### ⚠️ Disadvantages

**1. Single Plugin Limitation**
- AssignReplicas phase can only have one active plugin (because it returns final result)

**2. Increased Complexity**
- Users need to learn plugin development mechanism

**3. API Validation Difficulty**
- `AdvancedScheduling` field too flexible (map structure), Karmada cannot validate its legality
- Users can define custom ValidatingWebhookConfiguration for configuration validation

### 💡 Benefits Description

This solution uses plugin mechanism instead of directly modifying scheduler code, significantly reducing development and maintenance costs:

**Before (Direct Modification of Karmada Scheduler Source Code)**:
1. Need to modify Karmada scheduler framework code
2. Each upgrade requires distinguishing between native code and modified code, high cost

**After (Using Plugin Extension Mechanism)**:
1. Can follow [Karmada Official Documentation](https://karmada.io/docs/developers/customize-karmada-scheduler) approach for custom scheduler development without modifying Karmada scheduler native code
2. Upgrade operation: Update go mod, recompile

---

## Implementation Plan

### Approach: Plugin-based AssignReplicas Extension Point

**Description**: Transform Karmada native `assignReplicas` function into an extensible plugin mechanism.

**Core Changes**:

#### 1. Scheduler Framework Modification

**File**: `pkg/scheduler/core/generic_scheduler.go`

```go
// Original code (simplified)
func (g *genericScheduler) Schedule(ctx context.Context, binding *workv1alpha2.ResourceBinding) (ScheduleResult, error) {
    // Filter
    feasibleClusters := g.findClustersThatFit(ctx, binding)
    
    // Score
    clustersWithScore := g.prioritizeClusters(ctx, binding, feasibleClusters)
    
    // AssignReplicas (native logic)
    targetClusters := g.assignReplicas(binding, clustersWithScore, totalReplicas)
    
    return ScheduleResult{TargetClusters: targetClusters}, nil
}
```

**After modification**:

```go
func (g *genericScheduler) Schedule(ctx context.Context, binding *workv1alpha2.ResourceBinding) (ScheduleResult, error) {
    // Filter
    feasibleClusters := g.findClustersThatFit(ctx, binding)
    
    // Score
    clustersWithScore := g.prioritizeClusters(ctx, binding, feasibleClusters)
    
    // 🌟 AssignReplicas (supports plugin extension)
    var targetClusters []TargetCluster
    var err error
    
    // Check if AssignReplicasPlugin registered
    plugin := g.framework.GetAssignReplicasPlugin()
    if plugin != nil {
        // Use plugin to allocate replicas
        targetClusters, err = plugin.AssignReplicas(ctx, binding, clustersWithScore, totalReplicas)
    } else {
        // Use native logic (backward compatible)
        targetClusters = g.assignReplicas(binding, clustersWithScore, totalReplicas)
    }
    
    if err != nil {
        return ScheduleResult{}, err
    }
    
    return ScheduleResult{TargetClusters: targetClusters}, nil
}
```

#### 2. Framework Plugin Support Addition

**File**: `pkg/scheduler/framework/interface.go`

```go
// Framework interface adds method
type Framework interface {
    // ... existing methods ...
    
    // GetAssignReplicasPlugin returns registered AssignReplicasPlugin (if any)
    GetAssignReplicasPlugin() AssignReplicasPlugin
}
```

**File**: `pkg/scheduler/framework/runtime/framework.go`

```go
type frameworkImpl struct {
    // ... existing fields ...
    
    assignReplicasPlugin AssignReplicasPlugin
}

func (f *frameworkImpl) GetAssignReplicasPlugin() AssignReplicasPlugin {
    return f.assignReplicasPlugin
}

// Register plugin during initialization
func NewFramework(registry Registry) (Framework, error) {
    f := &frameworkImpl{
        // ... existing initialization ...
    }
    
    // 🌟 Get AssignReplicasPlugin from registry
    // If user registered custom plugin, use custom plugin
    // Otherwise use default plugin DefaultAssignReplicasPlugin
    pluginName := getRegisteredAssignReplicasPlugin(registry)
    plugin, err := registry[pluginName](nil, f)
    if err != nil {
        return nil, err
    }
    f.assignReplicasPlugin = plugin.(AssignReplicasPlugin)
    
    return f, nil
}
```

#### 3. Implement Default Plugin

**File**: `pkg/scheduler/framework/plugins/defaultassign/default_assign.go`

```go
package defaultassign

import (
    "context"
    
    clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
    workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
    "github.com/karmada-io/karmada/pkg/scheduler/framework"
)

const Name = "DefaultAssignReplicas"

// DefaultAssignReplicasPlugin implements Karmada native replica allocation logic
type DefaultAssignReplicasPlugin struct{}

func (p *DefaultAssignReplicasPlugin) Name() string {
    return Name
}

// AssignReplicas uses native replica allocation algorithm
func (p *DefaultAssignReplicasPlugin) AssignReplicas(
    ctx context.Context,
    binding *workv1alpha2.ResourceBinding,
    clusters []framework.ClusterWithScore,
    totalReplicas int32,
) ([]framework.TargetCluster, error) {
    // 🌟 Implement current Karmada assignReplicas logic here
    // 1. Based on ReplicaSchedulingStrategy (Duplicated/Divided)
    // 2. Based on ReplicaDivisionPreference (Weighted/Aggregated)
    // 3. Based on SpreadConstraints
    // Implementation omitted...
    
    return targetClusters, nil
}

// Factory function
func New(config runtime.Object) (framework.Plugin, error) {
    return &DefaultAssignReplicasPlugin{}, nil
}
```

#### 4. Register Default Plugin

**File**: `pkg/scheduler/scheduler.go`

```go
import (
    "github.com/karmada-io/karmada/pkg/scheduler/framework"
    "github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/defaultassign"
)

func init() {
    // 🌟 Register default plugin
    framework.RegisterPlugin(defaultassign.Name, defaultassign.New)
}

func NewScheduler(...) *Scheduler {
    // When scheduler initializes, create Framework
    // Framework will automatically use registered AssignReplicasPlugin
    framework, err := framework.NewFramework(registry)
    // ...
}
```

#### 5. User Custom Plugin Replaces Default Implementation

Users can implement their own plugins and register them:

**Custom Plugin Example**:

```go
package myplugin

import "github.com/karmada-io/karmada/pkg/scheduler/framework"

const Name = "MyCustomAssignReplicas"

type MyPlugin struct{}

func (p *MyPlugin) Name() string { return Name }

func (p *MyPlugin) AssignReplicas(...) ([]framework.TargetCluster, error) {
    // 🌟 Implement custom replica allocation logic
    return customLogic(...)
}

func init() {
    // Register custom plugin
    framework.RegisterPlugin(Name, func(config runtime.Object) (framework.Plugin, error) {
        return &MyPlugin{}, nil
    })
}
```

**Configure Scheduler to Use Custom Plugin**:

Start scheduler using `--plugins` parameter to specify plugin:

```bash
karmada-scheduler \
  --kubeconfig=/etc/karmada/karmada-apiserver.config \
  --bind-address=0.0.0.0 \
  --secure-port=10351 \
  --plugins=MyCustomAssignReplicas,-DefaultAssignReplicas
  # Enable MyCustomAssignReplicas, disable DefaultAssignReplicas (use - prefix to disable)
```

**Notes**:
- After implementing custom plugin, register via `framework.RegisterPlugin`
- Specify which plugin to enable via `--plugins` parameter, separate multiple plugins with commas
- Use `-PluginName` syntax to disable plugins (minus prefix)
- Example: `--plugins=Plugin1,Plugin2,-Plugin3` means enable Plugin1 and Plugin2, disable Plugin3
- ⚠️ Multiple `AssignReplicasPlugin` can be registered, but only one can be active/used

---

## Test Plan

### Unit Tests
- Test plugin interface correctness
- Test default plugin replica allocation logic
- Test custom plugin registration and invocation
- Test plugin configuration parsing logic

### Integration Tests
- Test default plugin end-to-end flow
- Test custom plugin replacing default plugin
- Test plugin integration with scheduler
- Test dynamic plugin configuration loading

### Performance Tests
- Compare scheduling latency before and after enabling plugins
- Ensure performance impact < 10%

---

## Upgrade and Compatibility

### Backward Compatibility

- ✅ Default uses `DefaultAssignReplicasPlugin`, maintaining native scheduling behavior
- ✅ Users can optionally register custom plugins to replace default implementation
- ✅ Does not affect existing scheduling behavior (unless explicitly configuring custom plugin)

### Upgrade Path

1. Upgrade Karmada scheduler (includes default plugin)
2. Verify scheduling behavior unaffected (using default plugin)
3. If customization needed, implement and register custom plugin

---

## References

- [Kubernetes Scheduler Framework](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/)
- [Karmada Scheduler](https://github.com/karmada-io/karmada/tree/master/pkg/scheduler)
- [Karmada PropagationPolicy](https://karmada.io/docs/userguide/scheduling/override-policy/)

---

## Appendix

### Common Configuration Format Reference

#### Specified IDC List (idcs)

```yaml
advancedScheduling:
  idcs:
    - name: "idc-east"
    - name: "idc-north"
```

#### Specified IDCs with Replica Counts (specified-idcs)

```yaml
advancedScheduling:
  specified-idcs:
    - name: "idc-east"
      replicas: 20
    - name: "idc-north"
      replicas: 10
```

#### Specified IDCs with Balanced Scheduling (specified-balanced-idcs)

```yaml
advancedScheduling:
  specified-balanced-idcs:
    - name: "idc-east"
      replicas: 20  # Balance across all clusters in idc-east
    - name: "idc-north"
      replicas: 10  # Balance across all clusters in idc-north
```

#### Specified Clusters with Replica Counts (specified-clusters)

```yaml
advancedScheduling:
  specified-clusters:
    - name: "cluster-1"
      replicas: 15
    - name: "cluster-2"
      replicas: 10
    - name: "cluster-3"
      replicas: 5
```

#### CronHPA Integration

```yaml
advancedScheduling:
  cronhpa-config:
    name: "my-cronhpa"
    namespace: "default"
```

### Plugin Development Best Practices

1. **Configuration Validation**: Validate configuration format during plugin initialization
2. **Error Handling**: Provide clear error messages
3. **Logging**: Log key decision processes
4. **Performance Optimization**: Avoid time-consuming operations, use caching when necessary
5. **Test Coverage**: Provide comprehensive unit tests

### FAQ

**Q1: Will scheduler behavior change if no plugin is configured?**

A: No. The scheduler defaults to using `DefaultAssignReplicasPlugin`, which implements Karmada native `assignReplicas` logic with identical behavior.

**Q2: Why can only one AssignReplicasPlugin be active/used?**

A: Because `AssignReplicasPlugin` returns the **final replica allocation result** (specific replica count for each cluster), which is the last step of scheduling decisions. Unlike Filter or Score plugins that can be chained, AssignReplicas output is the final result the scheduler uses and won't be modified by other plugins. While multiple `AssignReplicasPlugin` can be registered, the scheduler can only select one to execute at runtime.

If multiple allocation strategies are needed:
- **Recommended**: Implement multiple logics within one plugin, dynamically selecting based on `AdvancedScheduling` configuration

**Q3: What if multiple strategies are needed?**

A: Recommend implementing a **universal allocation plugin** that supports multiple strategies internally:

```go
type UniversalAssignPlugin struct{}

func (p *UniversalAssignPlugin) AssignReplicas(...) ([]TargetCluster, error) {
    // Determine strategy type based on AdvancedScheduling configuration
    if _, ok := binding.Spec.AdvancedScheduling["specified-idcs"]; ok {
        return p.assignByIdcs(...)
    } else if _, ok := binding.Spec.AdvancedScheduling["specified-clusters"]; ok {
        return p.assignByClusters(...)
    } else if _, ok := binding.Spec.AdvancedScheduling["cronhpa-config"]; ok {
        return p.assignByCronHPA(...)
    }
    
    // Default strategy
    return p.assignDefault(...)
}
```

**Q3: How do plugins read configuration?**

A: Plugins read configuration from `binding.Spec.AdvancedScheduling` map, where key is strategy name and value is JSON configuration.

**Q4: Will native assignReplicas logic be preserved?**

A: Yes. Native logic is implemented as `DefaultAssignReplicasPlugin` plugin, used as default plugin.

**Q5: Can this extension point completely replace native assignReplicas?**

A: Yes. Plugins can completely control how replicas are allocated to clusters, including:
- Specify precise replica count for each cluster
- Allocate based on external system data
- Implement complex allocation algorithms (change minimization, cost-based, etc.)
- Integrate with external systems like CronHPA

**Q6: What is plugin performance like?**

A: Plugins are internal Go code in scheduler, performance comparable to native logic. Default plugin `DefaultAssignReplicasPlugin` performance identical to current implementation. If custom plugins need to call external systems, recommend using caching for optimization.

**Q7: How to use custom plugin to replace default plugin?**

A: Several steps required:

1. **Implement custom plugin and register**:
```go
package myplugin

import "github.com/karmada-io/karmada/pkg/scheduler/framework"

const Name = "MyCustomAssignReplicas"

type MyPlugin struct{}

func (p *MyPlugin) Name() string { return Name }

func (p *MyPlugin) AssignReplicas(...) ([]framework.TargetCluster, error) {
    // Implement custom logic
    return customLogic(...)
}

func init() {
    framework.RegisterPlugin(Name, func(config runtime.Object) (framework.Plugin, error) {
        return &MyPlugin{}, nil
    })
}
```

2. **Start scheduler using `--plugins` parameter to specify plugin**:
```bash
karmada-scheduler \
  --kubeconfig=/etc/karmada/karmada-apiserver.config \
  --bind-address=0.0.0.0 \
  --secure-port=10351 \
  --plugins=MyCustomAssignReplicas,-DefaultAssignReplicas
  # Enable MyCustomAssignReplicas, disable DefaultAssignReplicas (use - prefix to disable)
```

