# What's New
## Resource Interpreter Webhook
The newly introduced Resource Interpreter Webhook framework allows users to implement their own CRD plugins that will be
consulted at all parts of propagation process. With this feature, CRDs and CRs will be propagated just like Kubernetes 
native resources, which means all scheduling primitives also support custom resources. An example as well as some helpful 
utilities are provided to help users better understand how this framework works.

Refer to [Proposal](../proposals/resource-interpreter-webhook/README.md) for more details.


## Significant Scheduling Enhancement
- Introduced `dynamicWeight` primitive to `PropagationPolicy` and `ClusterPropagationPolicy`. With this feature, replicas 
could be divided by a dynamic weight list, and the weight of each cluster will be calculated based on the available 
replicas during scheduling.

  This feature can balance the cluster's utilization significantly. [#841](https://github.com/karmada-io/karmada/pull/841)

- Introduced `Job` schedule (divide) support. A `Job` that desires many replicas now could be divided into many clusters 
  just like `Deployment`.

  This feature makes it possible to run huge Jobs across small clusters. [#898](https://github.com/karmada-io/karmada/pull/898)


## Workloads Observation from Karmada Control Plane
After workloads (e.g. Deployments) are propagated to member clusters, users may also want to get the overall workload 
status across many clusters, especially the status of each `pod`. In this release, a `get` subcommand was introduced to 
the `kubectl-karmada`. With this command, user are now able to get all kinds of resources deployed in member clusters from 
the Karmada control plane.

For example (get `deployment` and `pods` across clusters):

```shell
$ kubectl karmada get deployment
NAME    CLUSTER   READY   UP-TO-DATE   AVAILABLE   AGE   ADOPTION
nginx   member2   1/1     1            1           19m   Y
nginx   member1   1/1     1            1           19m   Y
$ kubectl karmada get pods
NAME                     CLUSTER   READY   STATUS    RESTARTS   AGE
nginx-6799fc88d8-vzdvt   member1   1/1     Running   0          31m
nginx-6799fc88d8-l55kk   member2   1/1     Running   0          31m
```


# Other Notable Changes
- karmada-scheduler-estimator: The number of pods becomes an important reference when calculating available replicas for 
  the cluster. [#777](https://github.com/karmada-io/karmada/pull/777)
- The labels (`resourcebinding.karmada.io/namespace`, `resourcebinding.karmada.io/name`, `clusterresourcebinding.karmada.io/name`) 
  which were previously added on the Work object now have been moved to annotations. 
  [#752](https://github.com/karmada-io/karmada/pull/752)
- Bugfix: Fixed the impact of cluster unjoining on resource status aggregation. [#817](https://github.com/karmada-io/karmada/pull/817)
- Instrumentation: Introduced events (`SyncFailed` and `SyncSucceed`) to the Work object. [#800](https://github.com/karmada-io/karmada/pull/800)
- Instrumentation: Introduced condition (`Scheduled`) to the `ResourceBinding` and `ClusterResourceBinding`. 
  [#823](https://github.com/karmada-io/karmada/pull/823)
- Instrumentation: Introduced events (`CreateExecutionNamespaceFailed` and `RemoveExecutionNamespaceFailed`) 
  to the Cluster object. [#749](https://github.com/karmada-io/karmada/pull/749)
- Instrumentation: Introduced several metrics (`workqueue_adds_total`, `workqueue_depth`, `workqueue_longest_running_processor_seconds`, 
  `workqueue_queue_duration_seconds_bucket`) for `karmada-agent` and `karmada-controller-manager`. [#831](https://github.com/karmada-io/karmada/pull/831)
- Instrumentation: Introduced condition (`FullyApplied`) to the `ResourceBinding` and `ClusterResourceBinding`. [#825](https://github.com/karmada-io/karmada/pull/825)
- karmada-scheduler: Introduced feature gates. [#805](https://github.com/karmada-io/karmada/pull/805)
- karmada-controller-manager: Deleted resources from member clusters that use "Background" as the default 
  delete option. [#970](https://github.com/karmada-io/karmada/pull/970)
  