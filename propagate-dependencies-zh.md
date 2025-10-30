---
title: 分发依赖项
---

Deployment、Job、Pod、DaemonSet 和 StatefulSet 的依赖项（ConfigMap 和 Secret）可自动传播到成员集群。本文档将演示如何使用该特性，更多设计细节可参考 [依赖项自动传播](https://github.com/karmada-io/karmada/blob/master/docs/proposals/dependencies-automatically-propagation/README.md)。

## 前提条件

### 已安装 Karmada

可参考 [快速开始](https://github.com/karmada-io/karmada#quick-start) 安装 Karmada，也可直接运行 `hack/local-up-karmada.sh` 脚本（该脚本也用于运行 E2E 测试用例）。

### 开启 PropagateDeps 特性

自 Karmada v1.4 版本起，`PropagateDeps` 特性门控（Feature Gate）已演进至 Beta 阶段，且默认开启。若您使用的是 Karmada 1.3 及更早版本，需手动开启该特性门控：

```bash
kubectl edit deployment karmada-controller-manager -n karmada-system
```

在 `karmada-controller-manager` 容器的 `args` 列表中添加 `--feature-gates=PropagateDeps=true`。

## 示例演示

创建包含 Deployment 和其依赖 ConfigMap 的 YAML 文件，内容如下：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-nginx
  labels:
    app: my-nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-nginx
  template:
    metadata:
      labels:
        app: my-nginx
    spec:
      containers:
        - image: nginx
          name: my-nginx
          ports:
            - containerPort: 80
          volumeMounts:
            - name: configmap
              mountPath: "/configmap"
      volumes:
        - name: configmap
          configMap:
            name: my-nginx-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-nginx-config
data:
  nginx.properties: |
    proxy-connect-timeout: "10s"
    proxy-read-timeout: "10s"
    client-max-body-size: "2m"
```

创建针对上述 Deployment 的传播策略，并设置 `propagateDeps: true` 以开启依赖项自动传播

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: my-nginx-propagation
spec:
  propagateDeps: true
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: my-nginx
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

策略执行成功后，Deployment 及其依赖的 ConfigMap 会自动传播到目标成员集群，可通过以下命令验证：

```bash
$  kubectl --kubeconfig /etc/karmada/karmada-apiserver.config get propagationpolicy
NAME                   AGE
my-nginx-propagation   16s
$  kubectl --kubeconfig /etc/karmada/karmada-apiserver.config get deployment
NAME       READY   UP-TO-DATE   AVAILABLE   AGE
my-nginx   2/2     2            2           22m
# member cluster1
$ kubectl config use-context member1
Switched to context "member1".
$  kubectl get deployment
NAME       READY   UP-TO-DATE   AVAILABLE   AGE
my-nginx   1/1     1            1           25m
$  kubectl get configmap
NAME               DATA   AGE
my-nginx-config    1      26m
# member cluster2
$ kubectl config use-context member2
Switched to context "member2".
$ kubectl get deployment
NAME       READY   UP-TO-DATE   AVAILABLE   AGE
my-nginx   1/1     1            1           27m
$  kubectl get configmap
NAME               DATA   AGE
my-nginx-config    1      27m
```

## 处理策略冲突

当多个资源共享同一个依赖资源（例如，两个 Deployment 都使用同一个 ConfigMap），
并且这些资源由不同的 PropagationPolicy 对象分发且策略设置存在冲突时，Karmada 需要
决定如何处理这个共享的依赖项。

### 什么是策略冲突

当出现以下情况时会发生 PropagationPolicy 冲突：

- 多个独立资源（如 `Deployment A` 和 `Deployment B`）引用同一个依赖资源（如 `ConfigMap`）
- 每个独立资源由不同的 `PropagationPolicy` 匹配
- 这些 PropagationPolicy 对象为 `conflictResolution` 或 `preserveResourcesOnDeletion` 字段设置了不同的值

当这种情况发生时，为共享 ConfigMap 创建的 attached ResourceBinding 将从引用它的 ResourceBinding 收到冲突的指令。

### 可能冲突的字段

有两个字段可能发生冲突：

- **conflictResolution**：控制 Karmada 如何处理成员集群中的资源冲突
  - `Overwrite`：如果资源已存在于成员集群中，Karmada 将覆盖该资源
  - `Abort`：如果资源已存在，Karmada 将停止并报告错误
- **preserveResourcesOnDeletion**：控制从 Karmada 中删除资源时是否应保留这些资源
  - `true`：即使从 Karmada 中删除，也在成员集群中保留资源
  - `false`：从 Karmada 中删除时，同时从成员集群中移除资源

### Karmada 如何解决冲突

Karmada 通过优先使用非默认值来自动解决冲突：

- **ConflictResolution**：默认值为 `Abort`。如果任何引用的 ResourceBinding 使用 `Overwrite`，则解决后的值为 `Overwrite`。
- **PreserveResourcesOnDeletion**：默认值为 `false`。如果任何引用的 ResourceBinding 将此字段设置为 `true`，则解决后的值为 `true`。

当检测到冲突时，Karmada 会发出一个原因为 `DependencyPolicyConflict` 的 Warning 事件来提醒您。

### 示例：检测和解决冲突

创建两个共享同一个 ConfigMap 的 Deployment：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: shared-config
  namespace: default
data:
  app.properties: |
    proxy-connect-timeout: "10s"
    proxy-read-timeout: "10s"
    client-max-body-size: "2m"
---
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
            name: shared-config
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
            name: shared-config
```

创建两个具有冲突设置的 PropagationPolicy 对象：

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
---
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

应用这些资源后，Karmada 将：

1. 为 `app-a` 和 `app-b` 创建独立的 ResourceBinding，包含各自的 PropagationPolicy 设置
2. 为共享的 ConfigMap `shared-config` 创建一个 attached ResourceBinding
3. 检测到两个 PropagationPolicy 对象之间的冲突
4. 使用规则解决冲突：`conflictResolution: Overwrite`（Overwrite 胜出）、`preserveResourcesOnDeletion: true`（true 胜出）
5. 发出一个 Warning 事件

### 查看冲突

检查共享 ConfigMap 的 attached ResourceBinding：

```bash
$ kubectl get resourcebindings
$ kubectl get resourcebinding shared-config-configmap -o yaml
```

您将在 `spec` 中看到解决后的值：

```yaml
apiVersion: work.karmada.io/v1alpha2
kind: ResourceBinding
metadata:
  name: shared-config-configmap
  namespace: default
spec:
  conflictResolution: Overwrite
  preserveResourcesOnDeletion: true
  resource:
    apiVersion: v1
    kind: ConfigMap
    name: shared-config
    namespace: default
  requiredBy:
    - name: app-a-deployment
      namespace: default
    - name: app-b-deployment
      namespace: default
```

检查冲突事件：

```bash
$ kubectl get events -n default --field-selector reason=DependencyPolicyConflict
```

您将看到类似这样的 Warning 事件：

```
LAST SEEN   TYPE      REASON                     OBJECT                                    MESSAGE
2m42s       Warning   DependencyPolicyConflict   resourcebinding/shared-config-configmap   Dependency policy conflict detected: ConflictResolution conflicted (Overwrite vs Abort); PreserveResourcesOnDeletion conflicted (true vs false).
```

### 解决冲突

要消除冲突警告，请对齐 PropagationPolicy 设置：

**方案 1**：对齐所有冲突的 PropagationPolicy 设置以保持一致

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: app-a-policy
  namespace: default
spec:
  conflictResolution: Abort # 对齐保持一致
  preserveResourcesOnDeletion: false # 对齐保持一致
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

应用此更改后，attached ResourceBinding 中解决后的值将从 `Overwrite/true` 变为 `Abort/false`。

**方案 2**：为每个 Deployment 使用单独的 ConfigMap

不共享单个 ConfigMap，而是为每个 Deployment 创建单独的 ConfigMap 对象。这可以完全避免冲突：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-a-config
  namespace: default
data:
  app.properties: |
    proxy-connect-timeout: "10s"
    proxy-read-timeout: "10s"
    client-max-body-size: "2m"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-b-config
  namespace: default
data:
  app.properties: |
    proxy-connect-timeout: "10s"
    proxy-read-timeout: "10s"
    client-max-body-size: "2m"
```

然后更新每个 Deployment 以引用其自己的 ConfigMap：

```yaml
# 在 app-a Deployment 中，更新 spec.template.spec.volumes：
volumes:
  - name: configmap
    configMap:
      name: app-a-config
```

```yaml
# 在 app-b Deployment 中，更新 spec.template.spec.volumes：
volumes:
  - name: configmap
    configMap:
      name: app-b-config
```
