# Override Policy

The [OverridePolicy][1] and [ClusterOverridePolicy][2] are used to declare override rules for resources when 
they are propagating to different clusters.

## Difference between OverridePolicy and ClusterOverridePolicy
TBD
## Resource Selector
TBD
## Target Cluster
TBD
## Overriders

Karmada offers various alternatives to declare the override rules:
- `ImageOverrider`: dedicated to override images for workloads.
- `CommandOverrider`: dedicated to override commands for workloads.
- `ArgsOverrider`: dedicated to override args for workloads.
- `PlaintextOverrider`: a general-purpose tool to override any kind of resources.

### ImageOverrider
TBD
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
  overriders:
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
  overriders:
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
TBD

[1]: https://github.com/karmada-io/karmada/blob/c37bedc1cfe5a98b47703464fed837380c90902f/pkg/apis/policy/v1alpha1/override_types.go#L13
[2]: https://github.com/karmada-io/karmada/blob/c37bedc1cfe5a98b47703464fed837380c90902f/pkg/apis/policy/v1alpha1/override_types.go#L189