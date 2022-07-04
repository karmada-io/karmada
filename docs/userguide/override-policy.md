# Override Policy

The [OverridePolicy][1] and [ClusterOverridePolicy][2] are used to declare override rules for resources when 
they are propagating to different clusters.

## Difference between OverridePolicy and ClusterOverridePolicy
ClusterOverridePolicy represents the cluster-wide policy that overrides a group of resources to one or more clusters while OverridePolicy will apply to resources in the same namespace as the namespace-wide policy. For cluster scoped resources, apply ClusterOverridePolicy by policies name in ascending. For namespaced scoped resources, first apply ClusterOverridePolicy, then apply OverridePolicy.

## Resource Selector

ResourceSelectors restricts resource types that this override policy applies to. If you ignore this field it means matching all resources.

Resource Selector required `apiVersion` field which represents the API version of the target resources and `kind` which represents the Kind of the target resources. 
The allowed selectors are as follows:
- `namespace`: namespace of the target resource.
- `name`: name of the target resource
- `labelSelector`: A label query over a set of resources.

#### Examples
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
      namespace: test
      labelSelector:
        matchLabels:
          app: nginx
  overrideRules:
  ...
```
It means override rules above will only be applied to `Deployment` which is named nginx in test namespace and has labels with `app: nginx`.

## Target Cluster

Target Cluster defines restrictions on the override policy that only applies to resources propagated to the matching clusters. If you ignore this field it means matching all clusters.

The allowed selectors are as follows:
- `labelSelector`: a filter to select member clusters by labels.
- `fieldSelector`: a filter to select member clusters by fields. Currently only three fields of provider(cluster.spec.provider), zone(cluster.spec.zone), and region(cluster.spec.region) are supported.
- `clusterNames`: the list of clusters to be selected.
- `exclude`: the list of clusters to be ignored.

### labelSelector

#### Examples
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example
spec:
  ...
  overrideRules:
    - targetCluster:
        labelSelector:
          matchLabels:
            cluster: member1 
      overriders:
      ...
```
It means override rules above will only be applied to those resources propagated to clusters which has `cluster: member1` label.

### fieldSelector

#### Examples
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example
spec:
  ...
  overrideRules:
    - targetCluster:
        fieldSelector:
          matchExpressions:
            - key: region
              operator: In
              values:
                - cn-north-1
      overriders:
      ...
```
It means override rules above will only be applied to those resources propagated to clusters which has the `spec.region` field with values in [cn-north-1].

### fieldSelector

#### Examples
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example
spec:
  ...
  overrideRules:
    - targetCluster:
        fieldSelector:
          matchExpressions:
            - key: region
              operator: In
              values:
                - cn-north-1
      overriders:
      ...
```
It means override rules above will only be applied to those resources propagated to clusters which has the `spec.region` field with values in [cn-north-1].

### clusterNames

#### Examples
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example
spec:
  ...
  overrideRules:
    - targetCluster:
        clusterNames:
          - member1
      overriders:
      ...
```
It means override rules above will only be applied to those resources propagated to clusters whose clusterNames are member1.

### exclude

#### Examples
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example
spec:
  ...
  overrideRules:
    - targetCluster:
        exclude:
          - member1
      overriders:
      ...
```
It means override rules above will only be applied to those resources propagated to clusters whose clusterNames are not member1.

## Overriders

Karmada offers various alternatives to declare the override rules:
- `ImageOverrider`: dedicated to override images for workloads.
- `CommandOverrider`: dedicated to override commands for workloads.
- `ArgsOverrider`: dedicated to override args for workloads.
- `PlaintextOverrider`: a general-purpose tool to override any kind of resources.

### ImageOverrider
The `ImageOverrider` is a refined tool to override images with format `[registry/]repository[:tag|@digest]`(e.g.`/spec/template/spec/containers/0/image`) for workloads such as `Deployment`.

The allowed operations are as follows:
- `add`: appends the registry, repository or tag/digest to the image from containers.
- `remove`: removes the registry, repository or tag/digest from the image from containers. 
- `replace`: replaces the registry, repository or tag/digest of the image from containers.

#### Examples
Suppose we create a deployment named `myapp`.
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  ...
spec:
  template:
    spec:
      containers:
        - image: myapp:1.0.0
          name: myapp
```

**Example 1: Add the registry when workloads are propagating to specific clusters.**
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example
spec:
  ...
  overrideRules:
    - overriders:
        imageOverrider:
          - component: Registry
            operator: add
            value: test-repo
```
It means `add` a registry`test-repo` to the image of `myapp`.

After the policy is applied for `myapp`, the image will be:
```yaml
      containers:
        - image: test-repo/myapp:1.0.0
          name: myapp
```

**Example 2: replace the repository when workloads are propagating to specific clusters.**
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example
spec:
  ...
  overrideRules:
    - overriders:
        imageOverrider:
          - component: Repository
            operator: replace
            value: myapp2
```
It means `replace` the repository from `myapp` to `myapp2`.

After the policy is applied for `myapp`, the image will be:
```yaml
      containers:
        - image: myapp2:1.0.0
          name: myapp
```

**Example 3: remove the tag when workloads are propagating to specific clusters.**
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example
spec:
  ...
  overrideRules:
    - overriders:
        imageOverrider:
          - component: Tag
            operator: remove
```
It means `remove` the tag of the image `myapp`.

After the policy is applied for `myapp`, the image will be:
```yaml
      containers:
        - image: myapp
          name: myapp
```

### CommandOverrider
The `CommandOverrider` is a refined tool to override commands(e.g.`/spec/template/spec/containers/0/command`) 
for workloads, such as `Deployment`.

The allowed operations are as follows:
- `add`: appends one or more flags to the command list.
- `remove`: removes one or more flags from the command list.

#### Examples
Suppose we create a deployment named `myapp`.
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  ...
spec:
  template:
    spec:
      containers:
        - image: myapp
          name: myapp
          command:
            - ./myapp
            - --parameter1=foo
            - --parameter2=bar
```

**Example 1: Add flags when workloads are propagating to specific clusters.**
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example
spec:
  ...
  overrideRules:
    - overriders:
        commandOverrider:
          - containerName: myapp
            operator: add
            value:
              - --cluster=member1
```
It means `add`(appending) a new flag `--cluster=member1` to the `myapp`.

After the policy is applied for `myapp`, the command list will be:
```yaml
      containers:
        - image: myapp
          name: myapp
          command:
            - ./myapp
            - --parameter1=foo
            - --parameter2=bar
            - --cluster=member1
```

**Example 2: Remove flags when workloads are propagating to specific clusters.**
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example
spec:
  ...
  overrideRules:
    - overriders:
        commandOverrider:
          - containerName: myapp
            operator: remove
            value:
              - --parameter1=foo
```
It means `remove` the flag `--parameter1=foo` from the command list.

After the policy is applied for `myapp`, the `command` will be:
```yaml
      containers:
        - image: myapp
          name: myapp
          command:
            - ./myapp
            - --parameter2=bar
```

### ArgsOverrider
The `ArgsOverrider` is a refined tool to override args(such as `/spec/template/spec/containers/0/args`) for workloads,
such as `Deployments`.

The allowed operations are as follows:
- `add`: appends one or more args to the command list.
- `remove`: removes one or more args from the command list.

Note: the usage of `ArgsOverrider` is similar to `CommandOverrider`, You can refer to the `CommandOverrider` examples.

### PlaintextOverrider
The `PlaintextOverrider` is a simple overrider that overrides target fields according to path, operator and value, just like `kubectl patch`.

The allowed operations are as follows:
- `add`: appends one or more elements to the resources.
- `remove`: removes one or more elements from the resources.
- `replace`: replaces one or more elements from the resources.

Suppose we create a configmap named `myconfigmap`.
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: myconfigmap
  ...
data:
  example: 1
```

**Example 1: replace data of the configmap when resources are propagating to specific clusters.**
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example
spec:
  ...
  overrideRules:
    - overriders:
        plaintext:
          - path: /data/example
            operator: replace
            value: 2
```
It means `replace` data of the configmap from `example: 1` to the `example: 2`.

After the policy is applied for `myconfigmap`, the configmap will be:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: myconfigmap
  ...
data:
  example: 2
```

[1]: https://github.com/karmada-io/karmada/blob/c37bedc1cfe5a98b47703464fed837380c90902f/pkg/apis/policy/v1alpha1/override_types.go#L13
[2]: https://github.com/karmada-io/karmada/blob/c37bedc1cfe5a98b47703464fed837380c90902f/pkg/apis/policy/v1alpha1/override_types.go#L189