# What's New
## Aggregated Kubernetes API Endpoint
The newly introduced `karmada-aggregated-apiserver` component aggregates all registered clusters and allows users to 
access member clusters through Karmada by the proxy endpoint, e.g.

- Retrieve `Node` from `member1`:  /apis/cluster.karmada.io/v1alpha1/clusters/member1/proxy/api/v1/nodes
- Retrieve `Pod` from `member2`: /apis/cluster.karmada.io/v1alpha1/clusters/member2/proxy/api/v1/namespaces/default/pods
  Please refer to user guide for more details.
  

## Promoting Workloads from Legacy Clusters to Karmada
Legacy workloads running in Kubernetes now can be promoted to Karmada smoothly without container restart.
In favor of `promote` commands added to Karmada CLI, any kind of Kubernetes resources can be promoted to Karmada easily, e.g.

```shell
# Promote deployment(default/nginx) from cluster1 to Karmada
kubectl karmada promote deployment nginx -n default -c cluster1
```


## Verified Integration with Ecosystem
Benefiting from the Kubernetes native API support, Karmada can easily integrate the single cluster ecosystem for multi-cluster, 
multi-cloud purpose. The following components have been verified by the Karmada community:

- argo-cd: refer to [working with argo-cd](https://github.com/karmada-io/website/blob/main/docs/userguide/cicd/working-with-argocd.md)
- Flux: refer to [propagating helm charts with flux](https://github.com/karmada-io/karmada/issues/861#issuecomment-998540302)
- Istio: refer to [working with Istio](https://github.com/karmada-io/website/blob/main/docs/userguide/service/working-with-istio-on-flat-network.md)
- Filebeat: refer to [working with Filebeat](https://github.com/karmada-io/website/blob/main/docs/administrator/monitoring/working-with-filebeat.md)
- Submariner: refer to [working with Submariner](https://github.com/karmada-io/website/blob/main/docs/userguide/network/working-with-submariner.md)
- Velero: refer to [working with Velero](https://github.com/karmada-io/website/blob/main/docs/administrator/backup/working-with-velero.md)
- Prometheus: refer to [working with Prometheus](https://github.com/karmada-io/website/blob/main/docs/administrator/monitoring/working-with-prometheus.md)


## OverridePolicy Improvements
By leverage of the new-introduced `RuleWithCluster` fields to `OverridePolicy` and `ClusterOverridePolicy`, users are now 
able to define override policies with a single policy for specified workloads.

## Karmada Installation Improvements
Introduced `init` command to `Karmada CLI`. Users are now able to install Karmada by a single command.

Please refer to [Installing Karmada](https://github.com/karmada-io/website/blob/main/docs/installation/installation.md) for more details.


## Configuring Karmada Controllers
Now all controllers provided by Karmada work as plug-ins. Users are now able to turn off any of them from the default 
enabled list.

See `--controllers` flag of `karmada-controller-manager` and `karmada-agent` for more details.


## Resource Interpreter Webhook Enhancement
Introduced `ReviseReplica` support for the `Resource Interpreter Webhook` framework, which enables scheduling all 
customized workloads just like Kubernetes native ones.

Refer to [Resource Interpreter Webhook Proposal](../proposals/resource-interpreter-webhook/README.md) for more design details.


# Other Notable Changes
## Bug Fixes
- `karmada-controller-manager`: Fixed the issue that the annotation of resource template cannot be updated. [#1012](https://github.com/karmada-io/karmada/pull/1025)
- `karmada-controller-manager`: Fixed the issue of generating binding reference key. [#1003](https://github.com/karmada-io/karmada/pull/1025)
- `karmada-controller-manager`: Fixed the inefficiency of en-queue failed task issue. [#1068](https://github.com/karmada-io/karmada/pull/1025)
## Features & Enhancements
- `Karmada CLI`: Introduced `--cluster-provider` flag to `join` command to specify provider of joining cluster. [#1025](https://github.com/karmada-io/karmada/pull/1025)
- `Karmada CLI`: Introduced `taint` command to set taints for clusters. [#889](https://github.com/karmada-io/karmada/pull/1025)
- `Karmada CLI`: The Applied condition of Work and `Scheduled/FullyApplied` of `ResourceBinding` are available for `kubectl get`. [#1110](https://github.com/karmada-io/karmada/pull/1025)
- `karmada-controller-manager`: The cluster discovery feature now supports `v1beta1` of `cluster-api`. [#1029](https://github.com/karmada-io/karmada/pull/1025)
- `karmada-controller-manager`: The `Job`'s `startTime` and `completionTime` now available at resource template. [#1034](https://github.com/karmada-io/karmada/pull/1025)
- `karmada-controller-manager`: introduced `--controllers` flag to enable or disable controllers. [#1083](https://github.com/karmada-io/karmada/pull/1025)
- `karmada-controller-manager`: Support retain `ownerReference` from observed objects. [#1116](https://github.com/karmada-io/karmada/pull/1025)
- `karmada-controller-manager` and `karmada-agent`: Introduced `cluster-cache-sync-timeout` flag to specify the time waiting for cache sync. [#1112](https://github.com/karmada-io/karmada/pull/1025)
## Instrumentation (Metrics and Events)
- `karmada-scheduler-estimator`: Introduced `/metrics` endpoint to emit metrics. [#1030](https://github.com/karmada-io/karmada/pull/1025)
- Introduced `ApplyPolicy` and `ScheduleBinding` events for resource template. [#1070](https://github.com/karmada-io/karmada/pull/1025)
## Deprecation
- The `ReplicaSchedulingPolicy` API deprecated at v0.9.0 now has been removed in favor of `ReplicaScheduling` of `PropagationPolicy`. [#1161](https://github.com/karmada-io/karmada/pull/1025)
