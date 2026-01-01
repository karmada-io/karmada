---
title: 增强 Karmada 调度器的可扩展性以支持定制需求
authors:
- "charesQQ"
  reviewers:
- TBD
  approvers:
- TBD

creation-date: 2025-12-25

---

# 增强 Karmada 调度器的可扩展性以定制需求

## 概述 (Summary)

当前 Karmada 调度器提供了强大的多集群调度能力，但在支持企业内部落地的时候, 满足定制化调度需求方面存在一些局限性。本提案旨在增强 Karmada 调度器的可扩展性架构，使其能够更好地支持：

1. **API不够灵活**：例如按机房/区域, 要求指定机房/区域的副本数,而不是按照权重
2. **AssignReplicas不满足需求**：例如不基于分数, 而是在指定机房/区域副本数约束下,基于最小化变更平均分配实例数
3. **高级调度约束**：例如调度策略依赖服务属性等额外画像数据进行调度决策

本提案提出了一个可扩展的调度框架增强方案，允许用户通过标准化的插件机制扩展调度器能力，而不需要修改核心调度器代码。这种设计既保持了 Karmada 调度器的通用性，又满足了企业用户的定制化需求。

## 动机 (Motivation)

### 背景问题

### 目标 (Goals)

本提案的目标是提供一个**通用的、可扩展的调度框架增强方案**，使得：

1. **插件化扩展**：用户可以通过实现标准插件接口来扩展调度器能力，而无需修改核心代码
2. **向后兼容**：现有的调度配置和行为保持不变，新特性通过可选方式启用

### 非目标 (Non-Goals)

以下内容不在本提案的范围内：

1. **调度器性能优化**：虽然插件机制会考虑性能，但大规模性能优化不是本提案的主要目标
2. **调度算法改进**：本提案关注可扩展性架构，而非特定调度算法的改进

本提案提出了一个**增加扩展点, 并增加灵活的API字段的方案**，核心思想是在保持现有调度流程的基础上，丰富API并增加新的扩展点

### 用户故事 (User Stories)

基于上述 4 种实际支持的策略，我们提供以下用户故事：

#### 故事 1：指定机房自动调度（基于策略 1: Idcs）

**背景**：我管理着一个微服务应用，需要部署到华东和华北两个机房，但不确定具体应该分配多少副本到每个机房。

**需求**：
- 指定目标机房：华东（idc-east）和华北（idc-north）
- 让调度器根据各集群的资源使用情况自动分配副本
- 优先分配到资源充足的集群，实现负载均衡
  **当前实现**：
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

**调度结果示例**：
- idc-east-cluster-1: 12 副本（CPU 使用率 60%）
- idc-east-cluster-2: 8 副本（CPU 使用率 75%）
- idc-north-cluster-1: 10 副本（CPU 使用率 65%）

---

#### 故事 2：指定机房及副本数（基于策略 2: SpecifiedIdcs）


**背景**：我需要确保核心业务在华东机房部署 20 个副本，在华北机房部署 10 个副本（符合容灾要求）。

**需求**：
- 华东机房：20 个副本
- 华北机房：10 个副本
- 在每个机房内，调度器根据集群资源情况自动分配

**当前实现**：
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

**调度结果示例**：
- idc-east-cluster-1: 8 副本
- idc-east-cluster-2: 7 副本
- idc-east-cluster-3: 5 副本
- idc-north-cluster-1: 6 副本
- idc-north-cluster-2: 4 副本

---

#### 故事 3：指定机房均衡调度（基于策略 3: SpecifiedBalancedIdcs）


**背景**：我的应用需要高可用部署，要求在指定机房内的所有集群间尽可能均衡分配副本。

**需求**：
- 华东机房：20 个副本，均衡分配到 4 个集群
- 华北机房：10 个副本，均衡分配到 2 个集群
- 每个集群的副本数差异最小化

**当前实现**：
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

**调度结果示例**（均衡分配）：
- idc-east-cluster-1: 5 副本
- idc-east-cluster-2: 5 副本
- idc-east-cluster-3: 5 副本
- idc-east-cluster-4: 5 副本
- idc-north-cluster-1: 5 副本
- idc-north-cluster-2: 5 副本

---

#### 故事 4：指定集群及副本数（基于策略 4: SpecifiedClusters）


**背景**：我的核心交易系统有严格的监管要求，必须精确控制每个集群的副本数。

**需求**：
- bj-prod-cluster: 必须 10 个副本（监管要求）
- sh-prod-cluster: 必须 8 个副本（业务要求）
- gz-dr-cluster: 必须 5 个副本（灾备要求）

**当前实现**：
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

**调度结果**（精确匹配）：
- bj-prod-cluster: 10 副本
- sh-prod-cluster: 8 副本
- gz-dr-cluster: 5 副本

#### 故事 5：与 CronHPA 集成的动态调度（自定义特性）


**背景**：我们的电商系统在促销期间需要动态扩缩容，并且需要将deployment传播到混合集群。

**需求**：
- 平时流量低，仅在自建 IDC 部署
- 促销期间通过 CronHPA 自动扩容，扩容副本调度到云厂商集群（mixed 类型集群）
- 促销结束后自动缩容，云厂商集群副本数归零, 副本数不受karmada管控

**当前实现**：
自定义调度器通过 `cronhpaChanged` 检测 CronHPA 资源的变化，自动将deployment调度到 `mixed` 类型集群。

```yaml
# CronHPA 配置
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
      schedule: "0 18 * * *"  # 每天18:00扩容
      targetReplicas: 50
    - name: scale-down
      schedule: "0 2 * * *"   # 每天02:00缩容
      targetReplicas: 0
---
# PropagationPolicy 配置
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

**调度结果**：
- **平时（30 副本）**：
  - idc-self-cluster-1: 15 副本
  - idc-self-cluster-2: 15 副本
- **促销期间（50 副本）**：
  - idc-self-cluster-1: 15 副本
  - idc-self-cluster-2: 15 副本
  - mixed-cloud-cluster: 0 副本（自动分配到 mixed 集群, 配合interperterwebhook retain特性, 不修改spec.replicas, 由子集群控制该副本数）

---

#### 故事 6：NodeLabels 插件过滤不匹配节点的集群（自定义特性）


**背景**：我的应用需要运行在特定 CPU 架构（如 ARM）或特定节点标签的节点上。

**需求**：
- 只调度到有 ARM 节点的集群
- 确保调度前检查节点标签，避免调度失败

**当前实现**：
自定义调度器的 `NodeLabels` 插件在 Filter 阶段检查集群中是否有满足 nodeSelector 的节点。

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
# Deployment 配置
apiVersion: apps/v1
kind: Deployment
metadata:
  name: arm-app
spec:
  template:
    spec:
      nodeSelector:
        kubernetes.io/arch: arm64  # NodeLabels 插件会检查此标签
```

**调度行为**：
- 调度器过滤掉没有 ARM 节点的集群
- 仅调度到有 `kubernetes.io/arch: arm64` 节点的集群

---

**调度流程与扩展点**：

```
┌─────────────────────────────────────────────────────────────────┐
│                      调度流程与扩展点                              │
└─────────────────────────────────────────────────────────────────┘

1. Filter 阶段（Karmada 原生）
   ├─ 用途: 过滤不满足条件的集群
   └─ 现有插件: ClusterAffinity, TaintToleration 等

2. Score 阶段（Karmada 原生）
   ├─ 用途: 对候选集群进行打分
   └─ 现有插件: ClusterLocality, ClusterAffinity 等

3. 🌟 AssignReplicas 阶段（核心扩展点 - 新增）
   ├─ 扩展点: AssignReplicasPlugin
   ├─ 说明: **替换 Karmada 原生的 assignReplicas 函数**
   ├─ 执行时机: Filter 和 Score 之后
   ├─ 输入: 已过滤和打分的集群列表、总副本数
   ├─ 输出: 每个集群的具体副本数分配
   └─ 用途: **自定义副本分配逻辑**，例如：
      • 指定副本数：精确指定每个机房/集群的副本数
      • 集成外部系统：CronHPA、配额管理、成本优化等
      • 复杂分配策略：最小化变更、基于资源使用率等
      • 特殊需求：混合集群支持、多层级分配等
```

## 设计细节 (Design Details)

### API 设计

#### 1. PropagationPolicy API 扩展

```go
// PropagationPolicy 扩展
type PropagationSpec struct {
    // ... 现有字段 ...
    
    // AdvancedScheduling 提供高级调度配置能力
    // Key: 调度策略名称（如 "specified-idcs", "cronhpa-config" 等）
    // Value: 策略配置（JSON 格式，由插件自行解析）
    // +optional
    AdvancedScheduling map[string]runtime.RawExtension `json:"advancedScheduling,omitempty"`
}
```

**设计说明**：
- ✅ **极简设计**：map 结构，key 是策略名，value 是配置
- ✅ **最大灵活性**：插件可以定义任意配置格式
- ✅ **易于扩展**：添加新策略只需增加新的 key
- ✅ **向后兼容**：可以平滑迁移现有注解配置

---

#### 2. 插件接口定义

本提案新增 `AssignReplicasPlugin` 扩展点，**用于替换 Karmada 原生的 `assignReplicas` 函数**。

```go
package framework

import (
    "context"
    
    clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
    workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
    "k8s.io/apimachinery/pkg/runtime"
)

// Plugin 所有调度插件的父接口
type Plugin interface {
    Name() string
}

// 🌟 AssignReplicasPlugin 副本分配插件（核心扩展点）
// 用于替换 Karmada 原生的 assignReplicas 函数
// 在 Filter 和 Score 之后执行，负责将副本分配到具体集群
//
// ⚠️ 重要：调度器只能注册一个 AssignReplicasPlugin
// 因为该插件返回的是最终的副本分配结果，不支持多个插件链式调用
type AssignReplicasPlugin interface {
    Plugin
    
    // AssignReplicas 根据自定义逻辑分配副本到集群
    // 参数：
    //   - ctx: 上下文
    //   - binding: 资源绑定信息
    //   - clusters: 已过滤和打分的集群列表（按分数排序）
    //   - totalReplicas: 需要分配的总副本数
    // 返回：
    //   - []TargetCluster: 每个集群的副本数分配结果（最终结果，不会被其他插件修改）
    //   - error: 错误信息
    AssignReplicas(
        ctx context.Context,
        binding *workv1alpha2.ResourceBinding,
        clusters []ClusterWithScore,
        totalReplicas int32,
    ) ([]TargetCluster, error)
}

// ClusterWithScore 带分数的集群信息
type ClusterWithScore struct {
    Cluster *clusterv1alpha1.Cluster
    Score   int64  // 来自 Score 阶段的分数
}

// TargetCluster 目标集群及副本数
type TargetCluster struct {
    Cluster  *clusterv1alpha1.Cluster
    Replicas int32
}

// PluginFactory 插件工厂函数
type PluginFactory func(configuration runtime.Object) (Plugin, error)

// 注册插件
func RegisterPlugin(name string, factory PluginFactory) {
    // 实现略
}
```

**设计要点**：

1. **默认插件与自定义替换**：
  - Karmada 提供一个**默认插件 `DefaultAssignReplicasPlugin`**，实现了当前原生的 `assignReplicas` 逻辑
  - 用户可以通过注册自定义插件来**替换**默认插件
  - 调度器在启动时注册插件，运行时调用已注册的插件

2. **⚠️ 单一插件约束**：
  - **调度器只能注册一个 `AssignReplicasPlugin`**
  - 原因：该插件返回的是**最终的副本分配结果**，不支持多个插件链式调用
  - 如果需要支持多种分配策略，应在一个插件内实现多种逻辑，通过 `AdvancedScheduling` 配置选择

3. **输入参数**：
  - `binding`：包含 PropagationPolicy 配置，插件可从 `binding.Spec.AdvancedScheduling` 读取自定义配置
  - `clusters`：已经通过 Filter 和 Score 的集群列表，按分数排序
  - `totalReplicas`：需要分配的总副本数

4. **输出结果**：
  - 返回每个集群的具体副本数（**最终结果**）
  - 插件需确保分配的总副本数等于 `totalReplicas`（除非有特殊需求）

4. **插件读取配置**：
   ```go
   // 示例：插件从 AdvancedScheduling 读取配置
   func (p *MyPlugin) AssignReplicas(
       ctx context.Context,
       binding *workv1alpha2.ResourceBinding,
       clusters []ClusterWithScore,
       totalReplicas int32,
   ) ([]TargetCluster, error) {
       // 从 AdvancedScheduling map 中读取配置
       config, exists := binding.Spec.AdvancedScheduling["specified-idcs"]
       if !exists {
           // 使用默认逻辑或返回错误
           return defaultAssign(clusters, totalReplicas), nil
       }
       
       // 解析配置
       var idcConfigs []IdcReplicaConfig
       if err := json.Unmarshal(config.Raw, &idcConfigs); err != nil {
           return nil, err
       }
       
       // 根据配置分配副本
       return p.assignByIdcConfig(clusters, totalReplicas, idcConfigs)
   }
   ```

---

#### 3. 调度器调用流程

调度器在 Filter 和 Score 之后，调用 `AssignReplicasPlugin` 来分配副本：

```go
// 伪代码：调度器中的实现
func (s *Scheduler) scheduleBinding(binding *workv1alpha2.ResourceBinding) error {
    // 1. Filter 阶段：过滤不满足条件的集群
    feasibleClusters := s.runFilterPlugins(binding)
    
    // 2. Score 阶段：对候选集群打分
    clustersWithScore := s.runScorePlugins(binding, feasibleClusters)
    
    // 3. AssignReplicas 阶段：分配副本（新增扩展点）
    // 🌟 调用已注册的 AssignReplicasPlugin（默认或自定义）
    targetClusters, err := s.runAssignReplicasPlugin(binding, clustersWithScore, totalReplicas)
    if err != nil {
        return err
    }
    
    // 4. 更新 ResourceBinding
    return s.updateBinding(binding, targetClusters)
}
```

**关键点**：
- ✅ **默认实现**：Karmada 提供默认插件，实现当前原生逻辑
- ✅ **灵活替换**：用户可以注册自定义插件完全替换默认实现
- ✅ **单一职责**：插件只负责副本分配，不影响 Filter 和 Score
- ⚠️ **单一插件**：只能注册一个 `AssignReplicasPlugin`，因为返回的是最终调度结果

---

#### 4. 插件实现示例

**示例 1：SpecifiedIdcsPlugin - 指定机房及副本数**

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

// SpecifiedIdcsPlugin 实现指定机房及副本数的分配策略
type SpecifiedIdcsPlugin struct{}

func (p *SpecifiedIdcsPlugin) Name() string {
    return SpecifiedIdcsPluginName
}

// IdcReplicaConfig 机房副本配置
type IdcReplicaConfig struct {
    Name     string `json:"name"`
    Replicas int32  `json:"replicas"`
}

// AssignReplicas 根据指定的机房配置分配副本
func (p *SpecifiedIdcsPlugin) AssignReplicas(
    ctx context.Context,
    binding *workv1alpha2.ResourceBinding,
    clusters []framework.ClusterWithScore,
    totalReplicas int32,
) ([]framework.TargetCluster, error) {
    
    // 1. 从 AdvancedScheduling 读取配置
    config, exists := binding.Spec.AdvancedScheduling["specified-idcs"]
    if !exists {
        return nil, fmt.Errorf("specified-idcs config not found")
    }
    
    // 2. 解析配置
    var idcConfigs []IdcReplicaConfig
    if err := json.Unmarshal(config.Raw, &idcConfigs); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }
    
    // 3. 按机房分组集群
    idcClusters := make(map[string][]framework.ClusterWithScore)
    for _, cluster := range clusters {
        idc := cluster.Cluster.Labels["topology.karmada.io/idc"]
        if idc != "" {
            idcClusters[idc] = append(idcClusters[idc], cluster)
        }
    }
    
    // 4. 根据配置分配副本
    targetClusters := make([]framework.TargetCluster, 0)
    
    for _, idcConfig := range idcConfigs {
        clustersInIdc, ok := idcClusters[idcConfig.Name]
        if !ok || len(clustersInIdc) == 0 {
            return nil, fmt.Errorf("no clusters found in idc %s", idcConfig.Name)
        }
        
        // 在机房内按资源使用率分配副本
        assigned := p.assignToIdcClusters(clustersInIdc, idcConfig.Replicas)
        targetClusters = append(targetClusters, assigned...)
    }
    
    return targetClusters, nil
}

// assignToIdcClusters 在机房内的集群中分配副本
func (p *SpecifiedIdcsPlugin) assignToIdcClusters(
    clusters []framework.ClusterWithScore,
    replicas int32,
) []framework.TargetCluster {
    // 按集群分数（资源使用率）排序，优先分配到资源充足的集群
    // 实现略...
    return []framework.TargetCluster{}
}

// 工厂函数
func NewSpecifiedIdcsPlugin(config runtime.Object) (framework.Plugin, error) {
    return &SpecifiedIdcsPlugin{}, nil
}

func init() {
    framework.RegisterPlugin(SpecifiedIdcsPluginName, NewSpecifiedIdcsPlugin)
}
```

**示例 2：SpecifiedClustersPlugin - 指定集群及副本数**

```go
// SpecifiedClustersPlugin 实现指定集群及副本数的分配策略
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
    
    // 1. 读取配置
    config, exists := binding.Spec.AdvancedScheduling["specified-clusters"]
    if !exists {
        return nil, fmt.Errorf("specified-clusters config not found")
    }
    
    var clusterConfigs []ClusterReplicaConfig
    if err := json.Unmarshal(config.Raw, &clusterConfigs); err != nil {
        return nil, err
    }
    
    // 2. 构建集群映射
    clusterMap := make(map[string]*clusterv1alpha1.Cluster)
    for _, c := range clusters {
        clusterMap[c.Cluster.Name] = c.Cluster
    }
    
    // 3. 直接按配置分配
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

### API 使用示例

#### 示例 1：指定机房及副本数（specified-idcs）

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

#### 示例 2：指定集群及副本数（specified-clusters）

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

## 优缺点分析

### ✅ 优点

**1. 高度灵活性**
- 用户可以实现自定义的 `AssignReplicasPlugin` 完全替换默认行为
- 支持任意复杂的副本分配逻辑（指定副本数、最小化变更、成本优化等）
- 可以集成外部系统（CronHPA、配额管理、监控系统等）

**2. 向后兼容**
- 默认使用 `DefaultAssignReplicasPlugin`，保持原生行为不变
- 用户可以选择性地启用自定义插件
- 不影响现有用户的使用

**3. 可扩展性**
- 通过 `AdvancedScheduling` map 字段，插件可以定义任意配置格式
- 无需修改 Karmada API，即可支持新的调度策略

### ⚠️ 缺点

**1. 单一插件限制**
- AssignReplicas 阶段只能注册一个插件（因为返回的是最终结果）

**2. 复杂性提高**
- 用户需要学习插件开发机制

**3. API 校验困难**
- `AdvancedScheduling` 字段过于灵活（map 结构），Karmada 无法校验其合法性
- 用户可以自定义 ValidatingWebhookConfiguration 进行配置校验

### 💡 收益说明

本方案采用插件化机制替代直接修改调度器代码，显著降低开发和维护成本：

**修改前（直接修改 Karmada 调度器源码）**：
1. 需要修改 Karmada 调度器框架代码
2. 每次升级要区分原生代码和修改代码，成本高

**修改后（使用插件化扩展机制）**：
1. 可以按照 [Karmada 官方文档](https://karmada.io/zh/docs/developers/customize-karmada-scheduler) 的方式进行自定义调度器开发，无需修改 Karmada 调度器原生代码
2. 升级操作：更新 go mod，重新编译即可

---

## 实施方案 (Implementation Plan)

### 方案：插件化 AssignReplicas 扩展点

**描述**：将 Karmada 原生的 `assignReplicas` 函数改造为可扩展的插件机制。

**核心改动**：

#### 1. 调度器框架改造

**文件**：`pkg/scheduler/core/generic_scheduler.go`

```go
// 原有代码（简化）
func (g *genericScheduler) Schedule(ctx context.Context, binding *workv1alpha2.ResourceBinding) (ScheduleResult, error) {
    // Filter
    feasibleClusters := g.findClustersThatFit(ctx, binding)
    
    // Score
    clustersWithScore := g.prioritizeClusters(ctx, binding, feasibleClusters)
    
    // AssignReplicas（原生逻辑）
    targetClusters := g.assignReplicas(binding, clustersWithScore, totalReplicas)
    
    return ScheduleResult{TargetClusters: targetClusters}, nil
}
```

**改造后**：

```go
func (g *genericScheduler) Schedule(ctx context.Context, binding *workv1alpha2.ResourceBinding) (ScheduleResult, error) {
    // Filter
    feasibleClusters := g.findClustersThatFit(ctx, binding)
    
    // Score
    clustersWithScore := g.prioritizeClusters(ctx, binding, feasibleClusters)
    
    // 🌟 AssignReplicas（支持插件扩展）
    var targetClusters []TargetCluster
    var err error
    
    // 检查是否有注册的 AssignReplicasPlugin
    plugin := g.framework.GetAssignReplicasPlugin()
    if plugin != nil {
        // 使用插件分配副本
        targetClusters, err = plugin.AssignReplicas(ctx, binding, clustersWithScore, totalReplicas)
    } else {
        // 使用原生逻辑（向后兼容）
        targetClusters = g.assignReplicas(binding, clustersWithScore, totalReplicas)
    }
    
    if err != nil {
        return ScheduleResult{}, err
    }
    
    return ScheduleResult{TargetClusters: targetClusters}, nil
}
```

#### 2. Framework 增加插件支持

**文件**：`pkg/scheduler/framework/interface.go`

```go
// Framework 接口增加方法
type Framework interface {
    // ... 现有方法 ...
    
    // GetAssignReplicasPlugin 返回注册的 AssignReplicasPlugin（如果有）
    GetAssignReplicasPlugin() AssignReplicasPlugin
}
```

**文件**：`pkg/scheduler/framework/runtime/framework.go`

```go
type frameworkImpl struct {
    // ... 现有字段 ...
    
    assignReplicasPlugin AssignReplicasPlugin
}

func (f *frameworkImpl) GetAssignReplicasPlugin() AssignReplicasPlugin {
    return f.assignReplicasPlugin
}

// 初始化时注册插件
func NewFramework(registry Registry) (Framework, error) {
    f := &frameworkImpl{
        // ... 现有初始化 ...
    }
    
    // 🌟 从注册表中获取 AssignReplicasPlugin
    // 如果用户注册了自定义插件，使用自定义插件
    // 否则使用默认插件 DefaultAssignReplicasPlugin
    pluginName := getRegisteredAssignReplicasPlugin(registry)
    plugin, err := registry[pluginName](nil, f)
    if err != nil {
        return nil, err
    }
    f.assignReplicasPlugin = plugin.(AssignReplicasPlugin)
    
    return f, nil
}
```

#### 3. 实现默认插件

**文件**：`pkg/scheduler/framework/plugins/defaultassign/default_assign.go`

```go
package defaultassign

import (
    "context"
    
    clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
    workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
    "github.com/karmada-io/karmada/pkg/scheduler/framework"
)

const Name = "DefaultAssignReplicas"

// DefaultAssignReplicasPlugin 实现 Karmada 原生的副本分配逻辑
type DefaultAssignReplicasPlugin struct{}

func (p *DefaultAssignReplicasPlugin) Name() string {
    return Name
}

// AssignReplicas 使用原生的副本分配算法
func (p *DefaultAssignReplicasPlugin) AssignReplicas(
    ctx context.Context,
    binding *workv1alpha2.ResourceBinding,
    clusters []framework.ClusterWithScore,
    totalReplicas int32,
) ([]framework.TargetCluster, error) {
    // 🌟 这里实现当前 Karmada 的 assignReplicas 逻辑
    // 1. 根据 ReplicaSchedulingStrategy (Duplicated/Divided)
    // 2. 根据 ReplicaDivisionPreference (Weighted/Aggregated)
    // 3. 根据 SpreadConstraints
    // 实现略...
    
    return targetClusters, nil
}

// 工厂函数
func New(config runtime.Object) (framework.Plugin, error) {
    return &DefaultAssignReplicasPlugin{}, nil
}
```

#### 4. 注册默认插件

**文件**：`pkg/scheduler/scheduler.go`

```go
import (
    "github.com/karmada-io/karmada/pkg/scheduler/framework"
    "github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/defaultassign"
)

func init() {
    // 🌟 注册默认插件
    framework.RegisterPlugin(defaultassign.Name, defaultassign.New)
}

func NewScheduler(...) *Scheduler {
    // 调度器初始化时，创建 Framework
    // Framework 会自动使用已注册的 AssignReplicasPlugin
    framework, err := framework.NewFramework(registry)
    // ...
}
```

#### 5. 用户自定义插件替换默认实现

用户可以实现自己的插件并注册：

**自定义插件示例**：

```go
package myplugin

import "github.com/karmada-io/karmada/pkg/scheduler/framework"

const Name = "MyCustomAssignReplicas"

type MyPlugin struct{}

func (p *MyPlugin) Name() string { return Name }

func (p *MyPlugin) AssignReplicas(...) ([]framework.TargetCluster, error) {
    // 🌟 实现自定义的副本分配逻辑
    return customLogic(...)
}

func init() {
    // 注册自定义插件
    framework.RegisterPlugin(Name, func(config runtime.Object) (framework.Plugin, error) {
        return &MyPlugin{}, nil
    })
}
```

**说明**：
- 用户实现自定义插件后，通过 `framework.RegisterPlugin` 注册
- 调度器启动时会注册并使用该插件替换默认实现
- ⚠️ 只能注册一个 `AssignReplicasPlugin`

---

## 测试计划 (Test Plan)

### 单元测试
- 测试插件接口的正确性
- 测试默认插件的副本分配逻辑
- 测试自定义插件的注册和调用
- 测试插件配置解析逻辑

### 集成测试
- 测试默认插件的端到端流程
- 测试自定义插件替换默认插件
- 测试插件与调度器的集成
- 测试插件配置动态加载

### 性能测试
- 对比启用插件前后的调度延迟
- 确保性能影响 < 10%

---

## 升级和兼容性 (Upgrade and Compatibility)

### 向后兼容

- ✅ 默认使用 `DefaultAssignReplicasPlugin`，保持原生调度行为
- ✅ 用户可以选择性地注册自定义插件替换默认实现
- ✅ 不影响现有调度行为（除非显式配置自定义插件）

### 升级路径

1. 升级 Karmada 调度器（包含默认插件）
2. 验证调度行为未受影响（使用默认插件）
3. 如需自定义，实现并注册自定义插件

---

## 参考资料 (References)

- [Kubernetes Scheduler Framework](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/)
- [Karmada Scheduler](https://github.com/karmada-io/karmada/tree/master/pkg/scheduler)
- [Karmada PropagationPolicy](https://karmada.io/docs/userguide/scheduling/override-policy/)

---

## 附录 (Appendix)

### 常用配置格式参考

#### 指定机房列表（idcs）

```yaml
advancedScheduling:
  idcs:
    - name: "idc-east"
    - name: "idc-north"
```

#### 指定机房及副本数（specified-idcs）

```yaml
advancedScheduling:
  specified-idcs:
    - name: "idc-east"
      replicas: 20
    - name: "idc-north"
      replicas: 10
```

#### 指定机房均衡调度（specified-balanced-idcs）

```yaml
advancedScheduling:
  specified-balanced-idcs:
    - name: "idc-east"
      replicas: 20  # 在 idc-east 的所有集群中均衡分配
    - name: "idc-north"
      replicas: 10  # 在 idc-north 的所有集群中均衡分配
```

#### 指定集群及副本数（specified-clusters）

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

#### CronHPA 集成

```yaml
advancedScheduling:
  cronhpa-config:
    name: "my-cronhpa"
    namespace: "default"
```

### 插件开发最佳实践

1. **配置验证**：在插件初始化时验证配置格式
2. **错误处理**：提供清晰的错误信息
3. **日志记录**：记录关键决策过程
4. **性能优化**：避免耗时操作，必要时使用缓存
5. **测试覆盖**：提供完整的单元测试

### FAQ

**Q1: 如果不配置插件，调度器行为会改变吗？**

A: 不会。调度器默认使用 `DefaultAssignReplicasPlugin`，该插件实现了 Karmada 原生的 `assignReplicas` 逻辑，行为完全一致。

**Q2: 为什么只能注册一个 AssignReplicasPlugin？**

A: 因为 `AssignReplicasPlugin` 返回的是**最终的副本分配结果**（每个集群的具体副本数），这是调度决策的最后一步。不像 Filter 或 Score 插件可以链式调用，AssignReplicas 的输出就是调度器要使用的最终结果，不会再被其他插件修改。

如果需要支持多种分配策略：
- **推荐**：在一个插件内实现多种逻辑，根据 `AdvancedScheduling` 配置动态选择

**Q3: 如果需要多种策略怎么办？**

A: 建议实现一个**通用的分配插件**，在插件内部支持多种策略：

```go
type UniversalAssignPlugin struct{}

func (p *UniversalAssignPlugin) AssignReplicas(...) ([]TargetCluster, error) {
    // 根据 AdvancedScheduling 配置判断策略类型
    if _, ok := binding.Spec.AdvancedScheduling["specified-idcs"]; ok {
        return p.assignByIdcs(...)
    } else if _, ok := binding.Spec.AdvancedScheduling["specified-clusters"]; ok {
        return p.assignByClusters(...)
    } else if _, ok := binding.Spec.AdvancedScheduling["cronhpa-config"]; ok {
        return p.assignByCronHPA(...)
    }
    
    // 默认策略
    return p.assignDefault(...)
}
```

**Q3: 插件如何读取配置？**

A: 插件从 `binding.Spec.AdvancedScheduling` map 中读取配置，key 是策略名称，value 是 JSON 配置。

**Q4: 原生的 assignReplicas 逻辑会保留吗？**

A: 会保留。原生逻辑被实现为 `DefaultAssignReplicasPlugin` 插件，作为默认插件使用。

**Q5: 这个扩展点能完全替换原生的 assignReplicas 吗？**

A: 是的。插件可以完全控制副本如何分配到集群，包括：
- 指定每个集群的精确副本数
- 基于外部系统数据进行分配
- 实现复杂的分配算法（最小化变更、基于成本等）
- 集成 CronHPA 等外部系统

**Q6: 插件的性能如何？**

A: 插件是调度器内部的 Go 代码，性能与原生逻辑相当。默认插件 `DefaultAssignReplicasPlugin` 的性能与当前实现完全一致。如果自定义插件需要调用外部系统，建议使用缓存优化。

**Q7: 如何使用自定义插件替换默认插件？**

A: 只需两步：
1. 实现自定义插件并注册：`framework.RegisterPlugin("MyPlugin", factory)`
2. 调度器启动时会使用已注册的插件替换默认实现