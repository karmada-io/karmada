<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.19.0](#v1190)
  - [Downloads for v1.19.0](#downloads-for-v1190)
  - [Urgent Update Notes](#urgent-update-notes)
  - [What's New](#whats-new)
    - [Multi-Component Workload Scheduling Advances (Phase IV)](#multi-component-workload-scheduling-advances-phase-iv)
    - [Priority-Based Scheduling Promoted to Beta](#priority-based-scheduling-promoted-to-beta)
    - [Automatic Credential Rotation for Push Mode Clusters](#automatic-credential-rotation-for-push-mode-clusters)
    - [Significant Performance Improvements](#significant-performance-improvements)
  - [Other Notable Changes](#other-notable-changes)
    - [API Changes](#api-changes)
    - [Features & Enhancements](#features--enhancements)
    - [Deprecation](#deprecation)
    - [Bug Fixes](#bug-fixes)
    - [Security](#security)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Charts](#helm-charts)
    - [Instrumentation](#instrumentation)
    - [Performance](#performance)
  - [Contributors](#contributors)
- [v1.19.0-rc.0](#v1190-rc0)
  - [Downloads for v1.19.0-rc.0](#downloads-for-v1190-rc0)
  - [Changelog since v1.19.0-beta.0](#changelog-since-v1190-beta0)
  - [Urgent Update Notes](#urgent-update-notes-1)
  - [Changes by Kind](#changes-by-kind)
    - [API Changes](#api-changes-1)
    - [Features & Enhancements](#features--enhancements-1)
    - [Deprecation](#deprecation-1)
    - [Bug Fixes](#bug-fixes-1)
    - [Security](#security-1)
  - [Other](#other-1)
    - [Dependencies](#dependencies-1)
    - [Helm Charts](#helm-charts-1)
    - [Instrumentation](#instrumentation-1)
    - [Performance](#performance-1)
- [v1.19.0-beta.0](#v1190-beta0)
  - [Downloads for v1.19.0-beta.0](#downloads-for-v1190-beta0)
  - [Changelog since v1.19.0-alpha.1](#changelog-since-v1190-alpha1)
  - [Urgent Update Notes](#urgent-update-notes-2)
  - [Changes by Kind](#changes-by-kind-1)
    - [API Changes](#api-changes-2)
    - [Features & Enhancements](#features--enhancements-2)
    - [Deprecation](#deprecation-2)
    - [Bug Fixes](#bug-fixes-2)
    - [Security](#security-2)
  - [Other](#other-2)
    - [Dependencies](#dependencies-2)
    - [Helm Charts](#helm-charts-2)
    - [Instrumentation](#instrumentation-2)
    - [Performance](#performance-2)
- [v1.19.0-alpha.1](#v1190-alpha1)
  - [Downloads for v1.19.0-alpha.1](#downloads-for-v1190-alpha1)
  - [Changelog since v1.19.0-alpha.0](#changelog-since-v1190-alpha0)
  - [Urgent Update Notes](#urgent-update-notes-3)
  - [Changes by Kind](#changes-by-kind-2)
    - [API Changes](#api-changes-3)
    - [Features & Enhancements](#features--enhancements-3)
    - [Deprecation](#deprecation-3)
    - [Bug Fixes](#bug-fixes-3)
    - [Security](#security-3)
  - [Other](#other-3)
    - [Dependencies](#dependencies-3)
    - [Helm Charts](#helm-charts-3)
    - [Instrumentation](#instrumentation-3)
    - [Performance](#performance-3)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.19.0
## Downloads for v1.19.0

Download v1.19.0 in the [v1.19.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.19.0).

## Urgent Update Notes
None.

## What's New

### Multi-Component Workload Scheduling Advances (Phase IV)

Modern AI and big-data workloads (such as FlinkDeployment and distributed training jobs) are often composed of multiple pod templates with different resource requirements. The `MultiplePodTemplatesScheduling` capability continues to evolve in this release, delivering the next phase of the [multiple pod templates support proposal](https://github.com/karmada-io/karmada/blob/master/docs/proposals/scheduling/multi-podtemplate-support/multiple-pod-template-support.md):

- **Per-component scheduling results in the API**: Introduced a new field `spec.clusters[*].components` to both ResourceBinding and ClusterResourceBinding, to represent the per-component scheduling results for workloads with multiple pod templates.
- **Scheduler persistence**: With the `MultiplePodTemplatesScheduling` feature gate enabled, the scheduler now records per-component replica assignments in the scheduling result, laying the foundation for upcoming scale and rescheduling scenarios of multi-template workloads.

For more details, see the tracking issue [#7492](https://github.com/karmada-io/karmada/issues/7492).

(Feature contributors: @RainbowMango, @ranxi2001)

### Priority-Based Scheduling Promoted to Beta

The `PriorityBasedScheduling` feature gate has been promoted to **Beta** and is now **enabled by default**. Workloads are scheduled in priority order as declared via `spec.schedulePriority` in PropagationPolicy/ClusterPropagationPolicy, allowing critical workloads to be scheduled ahead of less important ones when they compete for scheduling throughput.

This release also brings further optimizations to the priority scheduling queue: within a single flush, bindings whose backoff or unschedulable timeout has completed are now moved into the active queue in priority order, so higher-priority bindings are retried first on a best-effort basis.

To opt out, set `--feature-gates=PriorityBasedScheduling=false` for `karmada-scheduler`.

(Feature contributors: @RainbowMango, @CaesarTY)

### Automatic Credential Rotation for Push Mode Clusters

In push mode, Karmada relies on long-lived informer watches against member clusters. Previously, when a member cluster's bearer token was rotated (a common practice in security-hardened environments), these long-lived watches would silently break until the components were restarted.

This release introduces a token-refreshing transport layer for push-mode informers: rotated credentials are now picked up automatically, ensuring resource watches keep working across member cluster token rotation without any manual intervention or component restart.

(Feature contributor: @zhuyulicfc49)

### Significant Performance Improvements

This release continues the systematic performance optimization effort with notable improvements to memory footprint and controller efficiency:

- **Reduced informer cache memory usage**: `karmada-controller-manager`, `karmada-agent`, and `karmada-scheduler-estimator` now strip `managedFields` from dynamic informer caches. In a test distributing 20,000 Deployments to two member clusters, peak memory usage of `karmada-controller-manager` dropped from **5 GB to 3.4 GB**.
- **Clearer controller responsibility boundary**: The execution controller now handles all member cluster object modifications, while the work-status controller is scoped to status collection only. This eliminates duplicated reconciliation work and fixes cases where workload status could be unexpectedly lost.

For more details, see the tracking issue [#7596](https://github.com/karmada-io/karmada/issues/7596).

(Feature contributors: @zach593)

## Other Notable Changes

### API Changes
- `Karmada API`: Introduced a new field `spec.clusters[*].components` to both ResourceBinding and ClusterResourceBinding, to represent the per-component scheduling results for workloads with multiple pod templates. This field will be populated by the scheduler once the `MultiplePodTemplatesScheduling` feature gate is enabled. ([#7837](https://github.com/karmada-io/karmada/pull/7837), @RainbowMango)
- `Karmada API`: Removed the deprecated `PurgeMode` values `Immediately` and `Graciously`. Use `Directly` and `Gracefully` instead. ([#7792](https://github.com/karmada-io/karmada/pull/7792), @FengyuanYin)

### Features & Enhancements
- `karmada-controller-manager`: Added support for automatically refreshing member cluster credentials for push-mode informers, ensuring long-lived resource watches continue working after token rotation. ([#7663](https://github.com/karmada-io/karmada/pull/7663), @zhuyulicfc49)
- `karmada-controller-manager`: Moved all member cluster object modifications to the execution controller, scoping the work-status controller to status collection only. ([#7552](https://github.com/karmada-io/karmada/pull/7552), @zach593)
- `karmada-operator`: Updated the default `kube-apiserver` and `kube-controller-manager` images from v1.35.2 to v1.36.2, and the default etcd image from 3.6.6-0 to 3.6.8-0. ([#7666](https://github.com/karmada-io/karmada/pull/7666), @ranxi2001)
- `karmada-scheduler`: Promoted the `PriorityBasedScheduling` feature gate to Beta and enabled it by default. Workloads now can be scheduled in priority order as declared via `spec.schedulePriority` in PropagationPolicy/ClusterPropagationPolicy. To opt out, set `--feature-gates=PriorityBasedScheduling=false`. ([#7845](https://github.com/karmada-io/karmada/pull/7845), @RainbowMango)
- `karmada-scheduler`: Added support for recording per-component replica assignments in the scheduling result (`spec.clusters[*].components`) of ResourceBinding/ClusterResourceBinding for workloads with multiple pod templates, when the `MultiplePodTemplatesScheduling` feature gate is enabled. ([#7833](https://github.com/karmada-io/karmada/pull/7833), @ranxi2001)
- `karmada-scheduler`: Moved bindings whose backoff or unschedulable timeout completed into the active queue in priority order within a single flush, so higher-priority bindings are retried first on a best-effort basis. ([#7814](https://github.com/karmada-io/karmada/pull/7814), @CaesarTY)
- `karmadactl`: Updated the default `kube-apiserver` and `kube-controller-manager` images from v1.35.2 to v1.36.2, and the default etcd image from 3.6.6-0 to 3.6.8-0. ([#7666](https://github.com/karmada-io/karmada/pull/7666), @ranxi2001)

### Deprecation
- `karmada-controller-manager`: The `recreate` label of the `create_resource_to_cluster` metric has been deprecated and will be removed in a future release. ([#7868](https://github.com/karmada-io/karmada/pull/7868), @RainbowMango)
- `karmada-scheduler-estimator`: The deprecated proto messages `ReplicaRequirements.resourceRequest`, `ComponentReplicaRequirements.resourceRequest`, `NodeClaim.nodeAffinity` and `NodeClaim.tolerations` have been removed from the estimator proto. ([#7590](https://github.com/karmada-io/karmada/pull/7590), @zhzhuang-zju)

### Bug Fixes
- `karmada-controller-manager`: Fixed an issue where the taint-manager eviction queue would enqueue bindings with indefinite taint tolerations. ([#7613](https://github.com/karmada-io/karmada/pull/7613), @mszacillo)
- `karmada-controller-manager`: Fixed an issue where `Cluster.status.remedyActions` could remain stale after an associated `Remedy` resource was removed. ([#7777](https://github.com/karmada-io/karmada/pull/7777), @ranxi2001)
- `karmada-scheduler`: Fixed the issue that WorkloadRebalancer-triggered rescheduling did not reevaluate multiple `clusterAffinities` in policy order starting from the first term. ([#5425](https://github.com/karmada-io/karmada/pull/5425), @bharathguvvala)
- `karmadactl`: Fixed the issue that `init` silently used `127.0.0.1` when `--cert-external-ip` was set to an invalid value. ([#7656](https://github.com/karmada-io/karmada/pull/7656), @Anand-240)

### Security
- The base image `alpine` has been promoted from `alpine:3.23.4` to `alpine:3.24.0` to address security concerns. ([#7627](https://github.com/karmada-io/karmada/pull/7627), @dependabot)

## Other

### Dependencies
- Karmada is now built with Golang v1.26.4. ([#7600](https://github.com/karmada-io/karmada/pull/7600), @RainbowMango)
- Karmada is now built with Golang v1.26.5. ([#7786](https://github.com/karmada-io/karmada/pull/7786), @FengyuanYin)
- Karmada is now built with Golang v1.26.6. ([#7840](https://github.com/karmada-io/karmada/pull/7840), @FengyuanYin)
- Karmada is now built with Golang v1.26.7. ([#7854](https://github.com/karmada-io/karmada/pull/7854), @FengyuanYin)
- Kubernetes dependencies have been updated to v1.36.2. ([#7634](https://github.com/karmada-io/karmada/pull/7634), @RainbowMango)
- Kubernetes dependencies have been updated to v1.36.4 to resolve security concerns. ([#7866](https://github.com/karmada-io/karmada/pull/7866), @RainbowMango)

### Helm Charts
- `Helm chart`: Added helm index for `v1.17.3`. ([#7589](https://github.com/karmada-io/karmada/pull/7589), @github-actions)
- `Helm chart`: Added helm index for `v1.17.4`. ([#7701](https://github.com/karmada-io/karmada/pull/7701), @github-actions)
- `Helm chart`: Added helm index for `v1.18.0`. ([#7588](https://github.com/karmada-io/karmada/pull/7588), @github-actions)
- `Helm chart`: Added helm index for `v1.18.1`. ([#7700](https://github.com/karmada-io/karmada/pull/7700), @github-actions)
- `Helm chart`: Added `scheduler.enableEmptyWorkloadPropagation` value (default `false`); when set to `true`, the chart renders `--enable-empty-workload-propagation=true` for `karmada-scheduler`. ([#7570](https://github.com/karmada-io/karmada/pull/7570), @tamarubin)
- `Helm chart`: Updated the default `kube-apiserver` and `kube-controller-manager` images from v1.35.2 to v1.36.2, and the default etcd image from 3.6.6-0 to 3.6.8-0. ([#7666](https://github.com/karmada-io/karmada/pull/7666), @ranxi2001)
- `Helm chart`: Fixed TLS certificate SAN mismatch when deploying to a custom namespace by adding systemNamespace SANs to `certs.auto.hosts`. ([#7624](https://github.com/karmada-io/karmada/pull/7624), @Priyanshu-u07)
- `Helm chart`: Fixed the issue that installing `schedulerEstimator` as a standalone component (`installMode: component`) failed to mount its certificate Secret because the Secret name was derived from the component release name instead of the host-mode release that created it. Added `schedulerEstimator.certs` (default `karmada-cert`) to configure the Secret name explicitly. ([#7815](https://github.com/karmada-io/karmada/pull/7815), @pujitha24)

### Instrumentation
None.

### Performance
- `karmada-controller-manager`/`karmada-agent`/`karmada-scheduler-estimator`: Stripped `managedFields` from dynamic informer caches to reduce memory usage. ([#7807](https://github.com/karmada-io/karmada/pull/7807), @zach593)

## Contributors

Thank you to everyone who contributed to this release!

Users whose commits are in this release (alphabetically by username)

- @A69SHUBHAM
- @Anand-240
- @asarj
- @bharathguvvala
- @CaesarTY
- @ded-furby
- @FAUST-BENCHOU
- @FengyuanYin
- @mszacillo
- @Nazihbenbrahim
- @Priyanshu-u07
- @pujitha24
- @QCsnakeSUFE
- @RainbowMango
- @ranxi2001
- @TamarRubin
- @tushar-pandhare
- @XiShanYongYe-Chang
- @zach593
- @zhuyulicfc49
- @zhzhuang-zju

# v1.19.0-rc.0

## Downloads for v1.19.0-rc.0

Download v1.19.0-rc.0 from the [v1.19.0-rc.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.19.0-rc.0).

## Changelog since v1.19.0-beta.0

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
- `Karmada API`: Removed the deprecated `PurgeMode` values `Immediately` and `Graciously`. Use `Directly` and `Gracefully` instead. ([#7792](https://github.com/karmada-io/karmada/pull/7792), @FengyuanYin)

### Features & Enhancements
- `karmada-controller-manager`: Added support for automatically refreshing member cluster credentials for push-mode informers, ensuring long-lived resource watches continue working after token rotation. ([#7663](https://github.com/karmada-io/karmada/pull/7663), @zhuyulicfc49)
- `karmada-scheduler`: Moved bindings whose backoff or unschedulable timeout completed into the active queue in priority order within a single flush, so higher-priority bindings were retried first on a best-effort basis. ([#7814](https://github.com/karmada-io/karmada/pull/7814), @CaesarTY)

### Deprecation
None.

### Bug Fixes
- `karmada-scheduler-estimator`: Fixed the issue that installing `schedulerEstimator` as a standalone component (`installMode: component`) failed to mount its certificate Secret because the Secret name was derived from the component release name instead of the host-mode release that created it. Added `schedulerEstimator.certs` (default `karmada-cert`) to configure the Secret name explicitly. ([#7815](https://github.com/karmada-io/karmada/pull/7815), @pujitha24)

### Security
None.

## Other

### Dependencies
None.

### Helm Charts
None.

### Instrumentation
None.

### Performance
None.

# v1.19.0-beta.0
## Downloads for v1.19.0-beta.0

Download v1.19.0-beta.0 in the [v1.19.0-beta.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.19.0-beta.0).

## Changelog since v1.19.0-alpha.1

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
None.

### Features & Enhancements
None.

### Deprecation
None.

### Bug Fixes
- `karmada-controller-manager`: Fixed an issue where the taint-manager eviction queue would enqueue bindings with indefinite taint tolerations. ([#7613](https://github.com/karmada-io/karmada/pull/7613), @mszacillo)
- `karmada-controller-manager`: Fixed an issue where `Cluster.status.remedyActions` could remain stale after an associated `Remedy` resource was removed. ([#7777](https://github.com/karmada-io/karmada/pull/7777), @ranxi2001)
- `karmada-scheduler`: Fixed the issue that WorkloadRebalancer-triggered rescheduling did not reevaluate multiple `clusterAffinities` in policy order starting from the first term. ([#5425](https://github.com/karmada-io/karmada/pull/5425), @bharathguvvala)

### Security
None.

## Other
### Dependencies
- Karmada is now built with Golang v1.26.5. ([#7786](https://github.com/karmada-io/karmada/pull/7786), @SipengShen01)

### Helm Charts
- `Helm chart`: Added helm index for `v1.17.4`. ([#7701](https://github.com/karmada-io/karmada/pull/7701), @github-actions)
- `Helm chart`: Added helm index for `v1.18.1`. ([#7700](https://github.com/karmada-io/karmada/pull/7700), @github-actions)

### Instrumentation
None.

### Performance
None.

# v1.19.0-alpha.1
## Downloads for v1.19.0-alpha.1

Download v1.19.0-alpha.1 in the [v1.19.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.19.0-alpha.1).

## Changelog since v1.19.0-alpha.0

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-chart`: Added `scheduler.enableEmptyWorkloadPropagation` Helm value, which defaults to `false`. When set to `true`, the chart renders `--enable-empty-workload-propagation=true` for `karmada-scheduler`. ([#7570](https://github.com/karmada-io/karmada/pull/7570), @tamarubin)

### Deprecation
- `karmada-scheduler-estimator`: The proto messages `ReplicaRequirements.resourceRequest`, `ComponentReplicaRequirements.resourceRequest`, `NodeClaim.nodeAffinity`, and `NodeClaim.tolerations` have been removed. ([#7590](https://github.com/karmada-io/karmada/pull/7590), @zhzhuang-zju)

### Bug Fixes
- `helm chart`: Fixed TLS certificate SAN mismatch when deploying to a custom namespace by adding systemNamespace SANs to certs.auto.hosts. ([#7624](https://github.com/karmada-io/karmada/pull/7624), @Priyanshu-u07)
- `karmadactl`: Fixed the issue that `init` silently used `127.0.0.1` when `--cert-external-ip` was set to an invalid value. ([#7656](https://github.com/karmada-io/karmada/pull/7656), @Anand-240)

### Security
None.

## Other

### Dependencies
- Karmada is now built with Golang v1.26.4. ([#7600](https://github.com/karmada-io/karmada/pull/7600), @RainbowMango)
- Kubernetes dependencies have been updated to v1.36.2. ([#7634](https://github.com/karmada-io/karmada/pull/7634), @RainbowMango)
- The base image `alpine` has been promoted from `alpine:3.23.4` to `alpine:3.24.0` to address security concerns. ([#7627](https://github.com/karmada-io/karmada/pull/7627), @dependabot)

### Helm Charts
- `Helm chart`: Added helm index for `v1.17.3`. ([#7589](https://github.com/karmada-io/karmada/pull/7589), @github-actions)
- `Helm chart`: Added helm index for `v1.18.0`. ([#7588](https://github.com/karmada-io/karmada/pull/7588), @github-actions)
- `Helm chart`: Updated the default `kube-apiserver` and `kube-controller-manager` images from v1.35.2 to v1.36.2, and updated the default etcd image from 3.6.6-0 to 3.6.8-0. ([#7666](https://github.com/karmada-io/karmada/pull/7666), @ranxi2001)

### Instrumentation
None.

### Performance
None.
