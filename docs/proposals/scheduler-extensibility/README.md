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
2. **打分算法不满足需求**：例如不基于分数, 而是基于最小化变更原则, 打散扩缩容的副本到各成员集群
3. **高级调度约束**：例如调度策略依赖集群资源使用率,服务属性等额外画像数据进行调度决策
4. **框架层不支持特定需求**：例如某些服务如果开启了特定功能,就必须包含在调度结果中

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
  - mixed-cloud-cluster: 50 副本（自动分配到 mixed 集群, 由子集群控制该副本数）

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

3. 🌟 ReplicaScheduling 阶段（核心扩展点 - 新增）
   ├─ 扩展点: ReplicaSchedulingPlugin
   ├─ 执行时机: Filter 和 Score 之后
   ├─ 输入: 已过滤和打分的集群列表、总副本数
   ├─ 输出: 每个集群的具体副本数分配
   └─ 用途: 
      • 自定义副本分配逻辑（指定副本数、按机房分配等）
      • 集成外部系统（CronHPA、配额管理、成本优化等）
      • 实现复杂的副本分配策略（最小化变更、基于资源使用率等）
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

### 注解到 API 字段的迁移

#### 现有注解格式

```yaml
annotations:
  scheduler.karmada.io/replica-scheduling-strategy: |
    {
      "specifiedIdcs": [
        {"name": "idc-east", "replicas": 20},
        {"name": "idc-north", "replicas": 10}
      ]
    }
```

#### 新 API 字段格式

```yaml
advancedScheduling:
  specified-idcs:
    - name: "idc-east"
      replicas: 20
    - name: "idc-north"
      replicas: 10
```