---
title: Propagate dependencies
---

Deployment, Job, Pod, DaemonSet and StatefulSet dependencies (ConfigMaps and Secrets) can be propagated to member
clusters automatically. This document demonstrates how to use this feature. For more design details, please refer to
[dependencies-automatically-propagation](https://github.com/karmada-io/karmada/blob/master/docs/proposals/dependencies-automatically-propagation/README.md)

## Prerequisites

### Karmada has been installed

We can install Karmada by referring to [quick-start](https://github.com/karmada-io/karmada#quick-start), or directly run
`hack/local-up-karmada.sh` script which is also used to run our E2E cases.

### Enable PropagateDeps feature

`PropagateDeps` feature gate has evolved to Beta since Karmada v1.4 and is enabled by default. If you use Karmada 1.3 or earlier, you need to enable this feature gate.

```bash
kubectl edit deployment karmada-controller-manager -n karmada-system
```

Add `--feature-gates=PropagateDeps=true` option.

## Example

Create a Deployment mounted with a ConfigMap

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

Create a propagation policy with this Deployment and set `propagateDeps: true`.

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

Upon successful policy execution, the Deployment and ConfigMap are properly propagated to the member cluster.

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

## Handling Policy Conflicts

When multiple resources share the same dependent resource (for example, two Deployments both use the same ConfigMap), and those resources are propagated by different PropagationPolicy objects with conflicting settings, Karmada needs to decide how to handle the shared dependency.

### What is a Policy Conflict

A PropagationPolicy conflict happens when:

- Multiple independent resources (e.g., `Deployment A` and `Deployment B`) reference the same dependent resource (e.g., a `ConfigMap`)
- Each independent resource is matched by a different `PropagationPolicy`
- Those PropagationPolicy objects set different values for `conflictResolution` or `preserveResourcesOnDeletion`

When this happens, the attached ResourceBinding created for the shared ConfigMap will receive conflicting instructions
from the referencing ResourceBindings.

### Conflicting Fields

Two fields can conflict:

- **conflictResolution**: Controls how Karmada handles resource conflicts in member clusters
  - `Overwrite`: Karmada will overwrite the resource if it already exists in the member cluster
  - `Abort`: Karmada will stop and report an error if the resource already exists
- **preserveResourcesOnDeletion**: Controls whether resources should be preserved when deleted from Karmada
  - `true`: Keep the resources in member clusters even after deletion from Karmada
  - `false`: Remove the resources from member clusters when deleted from Karmada

### How Karmada Resolves Conflicts

Karmada automatically resolves conflicts by preferring non-default values:

- **ConflictResolution**: Defaults to `Abort`. If any referencing ResourceBinding uses `Overwrite`, the resolved value is `Overwrite`.
- **PreserveResourcesOnDeletion**: Defaults to `false`. If any referencing ResourceBinding sets this to `true`, the resolved value is `true`.

When a conflict is detected, Karmada emits a Warning event with reason `DependencyPolicyConflict` to alert you.

### Example: Detecting and Resolving Conflicts

Create two Deployments that share the same ConfigMap:

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

Create two PropagationPolicy objects with conflicting settings:

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

After applying these resources, Karmada will:

1. Create independent ResourceBindings for `app-a` and `app-b` with their respective PropagationPolicy settings
2. Create an attached ResourceBinding for the shared ConfigMap `shared-config`
3. Detect the conflict between the two PropagationPolicy objects
4. Resolve the conflict using the rules: `conflictResolution: Overwrite` (Overwrite wins), `preserveResourcesOnDeletion: true` (true wins)
5. Emit a Warning event

### Viewing Conflicts

Check the attached ResourceBinding for the shared ConfigMap:

```bash
$ kubectl get resourcebindings
$ kubectl get resourcebinding shared-config-configmap -o yaml
```

You will see the resolved values in the `spec`:

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

Check for conflict events:

```bash
$ kubectl get events -n default --field-selector reason=DependencyPolicyConflict
```

You will see a Warning event like:

```
LAST SEEN   TYPE      REASON                     OBJECT                                    MESSAGE
2m42s       Warning   DependencyPolicyConflict   resourcebinding/shared-config-configmap   Dependency policy conflict detected: ConflictResolution conflicted (Overwrite vs Abort); PreserveResourcesOnDeletion conflicted (true vs false).
```

### Resolving Conflicts

To eliminate the conflict warning, align the PropagationPolicy settings:

**Option 1**: Align all conflicting PropagationPolicy settings to be consistent

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: app-a-policy
  namespace: default
spec:
  conflictResolution: Abort # Aligned to be consistent
  preserveResourcesOnDeletion: false # Aligned to be consistent
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

After applying this change, the resolved values in the attached ResourceBinding will change from `Overwrite/true` to `Abort/false`.

**Option 2**: Use separate ConfigMaps for each Deployment

Instead of sharing a single ConfigMap, create separate ConfigMap objects for each Deployment. This avoids conflicts entirely:

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

Then update each Deployment to reference its own ConfigMap:

```yaml
# In app-a Deployment, update spec.template.spec.volumes:
volumes:
  - name: configmap
    configMap:
      name: app-a-config
```

```yaml
# In app-b Deployment, update spec.template.spec.volumes:
volumes:
  - name: configmap
    configMap:
      name: app-b-config
```
