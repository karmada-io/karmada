# Resource Propagating

The [PropagationPolicy](https://github.com/karmada-io/karmada/blob/master/pkg/apis/policy/v1alpha1/propagation_types.go#L13) and [ClusterPropagationPolicy](https://github.com/karmada-io/karmada/blob/master/pkg/apis/policy/v1alpha1/propagation_types.go#L292) APIs are provided to propagate resources. For the differences between the two APIs, please see [here](../frequently-asked-questions.md#what-is-the-difference-between-propagationpolicy-and-clusterpropagationpolicy).

Here, we use PropagationPolicy as an example to describe how to propagate resources.

## Before you start

[Install Karmada](../installation/installation.md) and prepare the [karmadactl command-line](../installation/install-kubectl-karmada.md) tool.

## Deploy a simplest multi-cluster Deployment

### Create a PropagationPolicy object

You can propagate a Deployment by creating a PropagationPolicy object defined in a YAML file. For example, this YAML
 file describes a Deployment object named nginx under default namespace need to be propagated to member1 cluster:

```yaml
# propagationpolicy.yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: example-policy # The default namespace is `default`.
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx # If no namespace is specified, the namespace is inherited from the parent object scope.
  placement:
    clusterAffinity:
      clusterNames:
        - member1
```

1. Create a propagationPolicy base on the YAML file:
```shell
kubectl apply -f propagationpolicy.yaml
```
2. Create a Deployment nginx resource:
```shell
kubectl create deployment nginx --image nginx
```
> Note: The resource exists only as a template in karmada. After being propagated to a member cluster, the behavior of the resource is the same as that of a single kubernetes cluster.

> Note: Resources and PropagationPolicy are created in no sequence.
3. Display information of the deployment:
```shell
karmadactl get deployment
```
The output is similar to this:
```shell
The karmadactl get command now only supports the push mode. [ member3 ] is not running in push mode.

NAME    CLUSTER   READY   UP-TO-DATE   AVAILABLE   AGE   ADOPTION
nginx   member1   1/1     1            1           52s   Y
```
4. List the pods created by the deployment:
```shell
karmadactl get pod -l app=nginx
```
The output is similar to this:
```shell
The karmadactl get command now only supports the push mode. [ member3 ] is not running in push mode.

NAME                     CLUSTER   READY   STATUS    RESTARTS   AGE
nginx-6799fc88d8-s7vv9   member1   1/1     Running   0          52s
```

### Update PropagationPolicy

You can update the propagationPolicy by applying a new YAML file. This YAML file propagates the Deployment to the member2 cluster.

```yaml
# propagationpolicy-update.yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: example-policy
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinity:
      clusterNames: # Modify the selected cluster to propagate the Deployment.
        - member2
```

1. Apply the new YAML file:
```shell
kubectl apply -f propagationpolicy-update.yaml
```
2. Display information of the deployment (the output is similar to this):
```shell
The karmadactl get command now only supports the push mode. [ member3 ] is not running in push mode.

NAME    CLUSTER   READY   UP-TO-DATE   AVAILABLE   AGE   ADOPTION
nginx   member2   1/1     1            1           5s    Y
```
3. List the pods of the deployment (the output is similar to this):
```shell
The karmadactl get command now only supports the push mode. [ member3 ] is not running in push mode.

NAME                     CLUSTER   READY   STATUS    RESTARTS   AGE
nginx-6799fc88d8-8t8cc   member2   1/1     Running   0          17s
```
> Note: Updating the `.spec.resourceSelectors` field to change hit resources is currently not supported.

### Update Deployment

You can update the deployment template. The changes will be automatically synchronized to the member clusters.

1. Update deployment replicas to 2
2. Display information of the deployment (the output is similar to this):
```shell
The karmadactl get command now only supports the push mode. [ member3 ] is not running in push mode.

NAME    CLUSTER   READY   UP-TO-DATE   AVAILABLE   AGE     ADOPTION
nginx   member2   2/2     2            2           7m59s   Y
```
3. List the pods of the deployment (the output is similar to this):
```shell
The karmadactl get command now only supports the push mode. [ member3 ] is not running in push mode.

NAME                     CLUSTER   READY   STATUS    RESTARTS   AGE
nginx-6799fc88d8-8t8cc   member2   1/1     Running   0          8m12s
nginx-6799fc88d8-zpl4j   member2   1/1     Running   0          17s
```

### Delete a propagationPolicy

Delete the propagationPolicy by name:
```shell
kubectl delete propagationpolicy example-policy
```
Deleting a propagationPolicy does not delete deployments propagated to member clusters. You need to delete deployments in the karmada control-plane:
```shell
kubectl delete deployment nginx
```

## Deploy deployment into a specified set of target clusters

`.spec.placement.clusterAffinity` field of PropagationPolicy represents scheduling restrictions on a certain set of clusters, without which any cluster can be scheduling candidates.

It has four fields to set:
- LabelSelector
- FieldSelector
- ClusterNames
- ExcludeClusters

### LabelSelector

LabelSelector is a filter to select member clusters by labels. It uses `*metav1.LabelSelector` type. If it is non-nil and non-empty, only the clusters match this filter will be selected.

PropagationPolicy can be configured as follows:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: test-propagation
spec:
  ...
  placement:
    clusterAffinity:
      labelSelector:
        matchLabels:
          location: us
    ...
```

PropagationPolicy can also be configured as follows:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: test-propagation
spec:
  ...
  placement:
    clusterAffinity:
      labelSelector:
        matchExpressions:
        - key: location
          operator: In
          values:
          - us
    ...
```

For a description of `matchLabels` and `matchExpressions`, you can refer to [Resources that support set-based requirements](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#resources-that-support-set-based-requirements).

### FieldSelector

FieldSelector is a filter to select member clusters by fields. If it is non-nil and non-empty, only the clusters match this filter will be selected.

PropagationPolicy can be configured as follows:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  ...
  placement:
    clusterAffinity:
      fieldSelector:
        matchExpressions:
        - key: provider
          operator: In
          values:
          - huaweicloud
        - key: region
          operator: NotIn
          values:
          - cn-south-1
    ...
```

If multiple `matchExpressions` are specified in the `fieldSelector`, the cluster must match all `matchExpressions`.

The `key` in `matchExpressions` now supports three values: `provider`, `region`, and `zone`, which correspond to the `.spec.provider`, `.spec.region`, and `.spec.zone` fields of the Cluster object, respectively.

The `operator` in `matchExpressions` now supports `In` and `NotIn`.

### ClusterNames

Users can set the `ClusterNames` field to specify the selected clusters.

PropagationPolicy can be configured as follows:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  ...
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
    ...
```

### ExcludeClusters

Users can set the `ExcludeClusters` fields to specify the clusters to be ignored.

PropagationPolicy can be configured as follows:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  ...
  placement:
    clusterAffinity:
      exclude:
        - member1
        - member3
    ...
```

## Configuring Multi-Cluster HA for Deployment


## Multi-Cluster Failover

Please refer to [Failover feature of Karmada](failover.md#failover-feature-of-karmada).

## Propagate specified resources to clusters


## Adjusting the instance propagation policy of Deployment in clusters