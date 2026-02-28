<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.17.0](#v1170)
  - [Downloads for v1.17.0](#downloads-for-v1170)
  - [Urgent Update Notes](#urgent-update-notes)
  - [What's New](#whats-new)
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
- [v1.17.0-rc.0](#v1170-rc0)
  - [Downloads for v1.17.0-rc.0](#downloads-for-v1170-rc0)
  - [Changelog since v1.17.0-beta.0](#changelog-since-v1170-beta0)
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
- [v1.17.0-beta.0](#v1170-beta0)
  - [Downloads for v1.17.0-beta.0](#downloads-for-v1170-beta0)
  - [Changelog since v1.17.0-alpha.2](#changelog-since-v1170-alpha2)
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
- [v1.17.0-alpha.2](#v1170-alpha2)
  - [Downloads for v1.17.0-alpha.2](#downloads-for-v1170-alpha2)
  - [Changelog since v1.17.0-alpha.1](#changelog-since-v1170-alpha1)
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
- [v1.17.0-alpha.1](#v1170-alpha1)
  - [Downloads for v1.17.0-alpha.1](#downloads-for-v1170-alpha1)
  - [Changelog since v1.17.0-alpha.0](#changelog-since-v1170-alpha0)
  - [Urgent Update Notes](#urgent-update-notes-4)
  - [Changes by Kind](#changes-by-kind-3)
    - [API Changes](#api-changes-4)
    - [Features & Enhancements](#features--enhancements-4)
    - [Deprecation](#deprecation-4)
    - [Bug Fixes](#bug-fixes-4)
    - [Security](#security-4)
  - [Other](#other-4)
    - [Dependencies](#dependencies-4)
    - [Helm Charts](#helm-charts-4)
    - [Instrumentation](#instrumentation-4)
    - [Performance](#performance-4)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.17.0
## Downloads for v1.17.0

Download v1.17.0 in the [v1.17.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.17.0).

## Urgent Update Notes

## What's New

### Advanced Workload Placement with Affinity and Anti-Affinity

Karmada provides rich and powerful cluster-oriented scheduling capabilities. However, many applications have explicit inter-workload placement requirements for high availability, latency optimization, cost efficiency, and operational isolation. 
To address these deployment needs, this release introduces **workload affinity and anti-affinity**, a powerful scheduling feature that gives you fine-grained control over how workloads are placed across clusters.
- **Workload Affinity**: Ensures that related workloads (e.g., a microservice and its cache, or distributed training jobs) are scheduled onto the same cluster. This is critical for performance-sensitive applications that need to minimize network latency.
- **Workload Anti-Affinity**: Spreads workloads belonging to the same logical group across different clusters. This is essential for achieving high availability, as it prevents a single cluster failure from taking down all replicas of a critical application.

By adding a `workloadAffinity` configuration to your `PropagationPolicy`, you can define `affinity groups` to colocate or spread out workloads using labels on resource templates. 

This feature is particularly useful when:
- **High availability**: Ensuring duplicate workloads run on different clusters to avoid single points of failure.
- **Co-location**: Scheduling related workloads to the same cluster to minimize latency.
- **Resource isolation**: Separating workloads that should not share cluster resources.

Workload affinity provides administrators with precise, policy-based control over the application topology, directly improving both resilience and performance in multi-cluster deployments.

For a detailed description of this feature, see the [official documentation](https://karmada.io/docs/next/userguide/scheduling/propagation-policy#workloadaffinity) and the original
[proposal](https://github.com/karmada-io/karmada/blob/master/docs/proposals/scheduling/anti-affinity-scheduling-support/README.md).

(Feature contributors: @mszacillo, @kevin-wangzefeng, @RainbowMango, @dahuo98, @zhzhuang-zju, @XiShanYongYe-Chang)

### Continued Performance Optimization in Controllers

Building on our commitment to enhancing Karmada's operational efficiency, this release continues to deliver performance improvements across Karmada controllers, ensuring a more stable and responsive multi-cluster management experience.

#### ControllerPriorityQueue Promoted to Beta

The `ControllerPriorityQueue` feature, initially introduced in v1.15, has matured through two versions of rigorous iteration and testing. It is now promoted to **Beta** and **enabled by default**. 

This feature enables controllers to defer reconciliation of existing resources in the system and prioritize processing newly triggered changes after a restart or leader transition. By focusing on the latest user-triggered updates first, it significantly reduces service downtime during planned upgrades and controller restarts.

#### Optimized Dependency Distribution

This release optimizes the dependencies-distributor controller to address a critical performance bottleneck observed during karmada-controller-manager startup in large-scale environments.

Previously, concurrent updates to dependency-related ResourceBinding objects could lead to API conflicts and request backlogs. By refactoring the `create` and `update` logic to be more atomic and efficient, we have drastically improved throughput.

The test setup included 30,000 Workloads and their associated PropagationPolicies, along with 30,000 ConfigMaps serving as dependencies for these workloads. This optimization reduced the initial controller startup and queue processing time from **over 20 minutes to approximately 5 minutes**, ensuring faster response and greater system stability.

For the detailed progress and test report, please refer to [PR: [Performance] optimize the mechanism of create or update dependencies-distribute resourcebinding](https://github.com/karmada-io/karmada/pull/7153).

(Feature contributors: @Zach593, @LivingCcj)

## Other Notable Changes

### API Changes
- Introduced `WorkloadAffinity` to `PropagationPolicy` API to support affinity and anti-affinity workload scheduling. ([#7131](https://github.com/karmada-io/karmada/pull/7131), @mszacillo)
- Introduced `WorkloadAffinityGroups` to `ResourceBinding/ClusterResourceBinding`, which will be used to hold instantiated grouping results. ([#7144](https://github.com/karmada-io/karmada/pull/7144), @RainbowMango)
- `karmada-operator`: Introduced `Tolerations` and `Affinity` fields to the `CommonSettings` of `Karmada` API for supporting explicit tolerations and affinity for Karmada control plane components. ([#6480](https://github.com/karmada-io/karmada/pull/6480), @abhinav-1305)

### Features & Enhancements
- `karmada-controller-manager`: Populated `ResourceBinding` with `WorkloadAffinity` fields. ([#7166](https://github.com/karmada-io/karmada/pull/7166), @dahuo98)
- `karmada-controller-manager`: Introduced `WorkloadAffinity` feature gate, which defaults to false. ([#7148](https://github.com/karmada-io/karmada/pull/7148), @mszacillo; [#7195](https://github.com/karmada-io/karmada/pull/7195), @zhzhuang-zju)
- `karmada-operator`: Supported explicit tolerations and affinity for all Karmada control plane components via the `Karmada` CR. ([#6480](https://github.com/karmada-io/karmada/pull/6480), @abhinav-1305)
- `karmada-operator`: Added support for configuring priority class and pod disruption budget config for Karmada operator deployment. ([#7015](https://github.com/karmada-io/karmada/pull/7015), @jabellard)
- `karmada-operator`: The default `kube-apiserver` and `kube-controller-manager` images have been updated from v1.34.1 to v1.35.2. And the default `ETCD` Image has been updated from 3.6.0-0 to 3.6.6-0. ([#7229](https://github.com/karmada-io/karmada/pull/7229), @RainbowMango)
- `karmada-resource-interpreter`: Added `RayService` interpreter support. ([#7102](https://github.com/karmada-io/karmada/pull/7102), @seanlaii)
- `karmada-scheduler`: Added optional `FilterPluginWithContext` (`FilterWithContext` method) for filter plugins, and introduced a `WorkloadAntiAffinity` filter plugin gated by the `WorkloadAffinity` feature gate. ([#7177](https://github.com/karmada-io/karmada/pull/7177), @RainbowMango)
- `karmada-scheduler`: Implemented workload affinity and anti-affinity filter plugins to support co-locating workloads in the same `AffinityGroup` or isolating workloads in the same `AntiAffinityGroup` across clusters. ([#7189](https://github.com/karmada-io/karmada/pull/7189), @zhzhuang-zju)
- `karmada-scheduler`: Improved scheduling consistency for workload affinity groups during rapid scheduling cycles by introducing a dedicated cache for recently committed `ResourceBindings`. ([#7221](https://github.com/karmada-io/karmada/pull/7221), @zhzhuang-zju, @mszacillo)
- `karmada-webhook`: Added `namespace` validation for `spec.resourceSelectors` in `PropagationPolicy` and `OverridePolicy` to prevent unintended cross-namespace resource selection. ([#7176](https://github.com/karmada-io/karmada/pull/7176), @zhzhuang-zju)
- `karmada-webhook`: Disallowed setting the same `GroupByLabelKey` for `affinity` and `antiAffinity` in `WorkloadAffinity`. ([#7222](https://github.com/karmada-io/karmada/pull/7222), @XiShanYongYe-Chang)
- `karmadactl`: The default `kube-apiserver` and `kube-controller-manager` images have been updated from v1.34.1 to v1.35.2. And the default `ETCD` Image has been updated from 3.6.0-0 to 3.6.6-0. ([#7229](https://github.com/karmada-io/karmada/pull/7229), @RainbowMango)

### Deprecation
- `karmada-controller-manager`: The flags `--cluster-lease-duration` and `--cluster-lease-renew-interval-fraction` have been deprecated and will be removed in a future release. ([#7126](https://github.com/karmada-io/karmada/pull/7126), @FAUST-BENCHOU)
- `karmadactl`: `Etcd.Local.InitImage` in `Karmada Init Configuration` has been deprecated and will be removed in a future version. ([#6995](https://github.com/karmada-io/karmada/pull/6995), @zhzhuang-zju)
- `karmadactl`: Removed the deprecated flag `--etcd-init-image` of `init` command. ([#6974](https://github.com/karmada-io/karmada/pull/6974), @AbhinavPInamdar)

### Bug Fixes
- `karmada-controller-manager`: Fixed the issue where the job status aggregator could enter an error loop due to a race condition when setting the initial `startTime`. ([#7138](https://github.com/karmada-io/karmada/pull/7138), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed CronFederatedHPA scale-up from zero failure when the replicas field is missing. ([#7183](https://github.com/karmada-io/karmada/pull/7183), @zhengjr9)
- `karmada-controller-manager`: Fixed an issue where a per-task `GracePeriodSeconds` value could leak to subsequent graceful eviction tasks, causing premature or delayed evictions. ([#7184](https://github.com/karmada-io/karmada/pull/7184), @Ady0333)
- `karmada-controller-manager`: Fixed an issue where dependency updates could overwrite other controller annotations during retry conflicts. ([#7208](https://github.com/karmada-io/karmada/pull/7208), @Ady0333)
- `karmada-controller-manager`: Fixed an issue where policy deletion could be blocked if a resource selector targeted a non-existent resource. ([#7038](https://github.com/karmada-io/karmada/pull/7038), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the issue where PP/CPP cannot be deleted because the resources API selected by the PP/CPP do not exist on the control plane. ([#7024](https://github.com/karmada-io/karmada/pull/7024), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that `HelmRelease` did not define `observedGeneration` variable in the `statusAggregation` operation. ([#7057](https://github.com/karmada-io/karmada/pull/7057), @FAUST-BENCHOU)
- `karmada-scheduler`: Fixed a scheduler panic caused by a divide-by-zero error when calculating spread constraints with no valid clusters. ([#7154](https://github.com/karmada-io/karmada/pull/7154), @Aman-Cool)
- `karmada-scheduler`: Fixed an issue that prevented multi-component workloads from being rescheduled during cluster failover events. ([#7066](https://github.com/karmada-io/karmada/pull/7066), @mszacillo)
- `karmada-scheduler`: Fixed the issue that `dynamicScaleDown` used stale clusters for scale-down operations. ([#7110](https://github.com/karmada-io/karmada/pull/7110), @zhzhuang-zju)
- `karmada-scheduler`: Fixed the bug in the backoff queue where the sorting function was incorrect, potentially causing high-priority items with long backoffs to block lower-priority items. ([#6987](https://github.com/karmada-io/karmada/pull/6987), @rayo1uo)
- `karmada-scheduler-estimator`: Fixed the issue where the resource quota plugin failed to list resource quotas due to a missing namespace in the gRPC request. ([#7124](https://github.com/karmada-io/karmada/pull/7124), @zhzhuang-zju)
- `karmadactl`: Fixed the messy auto-completion suggestions for commands like `get` and `apply`. ([#7023](https://github.com/karmada-io/karmada/pull/7023), @zhzhuang-zju)
- `karmada-webhook`: Fixed an issue where the `condition.reason` was not set to `QuotaExceeded` when FederatedResourceQuota is exceeded. ([#7086](https://github.com/karmada-io/karmada/pull/7086), @kajal-jotwani)

### Security
- `Helm chart`: Enhanced the security posture of Karmada by integrating encryption-at-rest capabilities for the karmada-apiserver. It allows users to encrypt sensitive data stored in etcd by providing a custom encryption configuration through a Kubernetes Secret, which the apiserver will then utilize. ([#7164](https://github.com/karmada-io/karmada/pull/7164), @cmontemuino)

## Other

### Dependencies
- Upgraded Kubernetes dependencies to `v1.35.0`. ([#7149](https://github.com/karmada-io/karmada/pull/7149), @RainbowMango)
- Promoted the base image `alpine` from `alpine:3.22.2` to `alpine:3.23.3`. ([#7001](https://github.com/karmada-io/karmada/pull/7001), [#7034](https://github.com/karmada-io/karmada/pull/7034), [#7159](https://github.com/karmada-io/karmada/pull/7159), @dependabot)
- Karmada is now built with Golang v1.25.7. ([#7225](https://github.com/karmada-io/karmada/pull/7225), @RainbowMango)

### Helm Charts
- `Helm Chart`: Added helm index for 1.16 release. ([#6990](https://github.com/karmada-io/karmada/pull/6990), @zhzhuang-zju)
- `Helm Chart`: Enhanced the security posture of Karmada by integrating encryption-at-rest capabilities for the karmada-apiserver. It allows users to encrypt sensitive data stored in etcd by providing a custom encryption configuration through a Kubernetes Secret, which the apiserver will then utilize. ([#7164](https://github.com/karmada-io/karmada/pull/7164), @cmontemuino)
- `Helm Chart`: Upgraded `bitnami/common` dependency in karmada operator chart from `1.17.1` to `2.31.4`. ([#6994](https://github.com/karmada-io/karmada/pull/6994), @zhzhuang-zju)
- `Helm Chart`: The default `kube-apiserver` and `kube-controller-manager` images have been updated from v1.34.1 to v1.35.2. And the default `ETCD` Image has been updated from 3.6.0-0 to 3.6.6-0. ([#7229](https://github.com/karmada-io/karmada/pull/7229), @RainbowMango)

### Instrumentation
- `Instrumentation`: The metric `work_sync_workload_duration_seconds` no longer counts retriable Kubernetes 409 conflicts as errors, improving availability accuracy and reducing false alerts caused by conflict retry flapping. ([#7106](https://github.com/karmada-io/karmada/pull/7106), @RainbowMango)

### Performance
- `karmada-controller-manager`: Optimized the mechanism of `create` or `update` dependencies-distribute ResourceBinding. ([#7153](https://github.com/karmada-io/karmada/pull/7153), @LivingCcj)
- `karmada-controller-manager`/`karmada-agent`: The feature `ControllerPriorityQueue` is promoted to beta and enabled by default. ([#7200](https://github.com/karmada-io/karmada/pull/7200), @zach593)

## Contributors

Thank you to everyone who contributed to this release!

Users whose commits are in this release (alphabetically by username)

- @7h3-3mp7y-m4n
- @Abhay349
- @abhinav-1305
- @AbhinavPInamdar
- @Ady0333
- @Aman-Cool
- @Arhell
- @arnavgogia20
- @CharlesQQ
- @cmontemuino
- @dahuo98
- @FAUST-BENCHOU
- @gmarav05
- @goyalpalak18
- @jabellard
- @kajal-jotwani
- @LivingCcj
- @mohamedawnallah
- @mszacillo
- @RainbowMango
- @rayo1uo
- @seanlaii
- @SunsetB612
- @suresh-subramanian2013
- @vie-serendipity
- @warjiang
- @XiShanYongYe-Chang
- @yaten2302
- @yoursanonymous
- @zach593
- @zhengjr9
- @zhzhuang-zju

# v1.17.0-rc.0
## Downloads for v1.17.0-rc.0

Download v1.17.0-rc.0 in the [v1.17.0-rc.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.17.0-rc.0).

## Changelog since v1.17.0-beta.0

## Urgent Update Notes

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-scheduler`: Added optional `FilterPluginWithContext` (`FilterWithContext` method) for filter plugins, and introduced a `WorkloadAntiAffinity` filter plugin gated by the `WorkloadAffinity` feature gate. ([#7177](https://github.com/karmada-io/karmada/pull/7177), @RainbowMango)
- `karmada-scheduler`: Implemented workload affinity and anti-affinity filter plugins to support co-locating workloads in the same `AffinityGroup` or isolating workloads in the same `AntiAffinityGroup` across clusters. ([#7189](https://github.com/karmada-io/karmada/pull/7189), @zhzhuang-zju)
- `karmada-controller-manager`: Populated `ResourceBinding` and `ClusterResourceBinding` with `WorkloadAffinity` fields. ([#7166](https://github.com/karmada-io/karmada/pull/7166), @dahuo98)

### Deprecation
None.

### Bug Fixes
- `karmada-controller-manager`: Fixed the issue where the job status aggregator could enter an error loop due to a race condition when setting the initial `startTime`. ([#7138](https://github.com/karmada-io/karmada/pull/7138), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed CronFederatedHPA scale-up from zero failure when the replicas field is missing. ([#7183](https://github.com/karmada-io/karmada/pull/7183), @zhengjr9)
- `karmada-controller-manager`: Fixed an issue where a per-task `GracePeriodSeconds` value could leak to subsequent graceful eviction tasks, causing premature or delayed evictions. ([#7184](https://github.com/karmada-io/karmada/pull/7184), @Ady0333)
- `karmada-scheduler`: Fixed a scheduler panic caused by a divide-by-zero error when calculating spread constraints with no valid clusters. ([#7154](https://github.com/karmada-io/karmada/pull/7154), @Aman-Cool)

### Security
None.

## Other
### Dependencies
- Upgraded Kubernetes dependencies to v1.35.0. ([#7149](https://github.com/karmada-io/karmada/pull/7149), @RainbowMango)
- Promoted the base image `alpine` from `alpine:3.23.2` to `alpine:3.23.3`. ([#7159](https://github.com/karmada-io/karmada/pull/7159), @dependabot)

### Helm Charts
None.

### Instrumentation
None.

### Performance
None.

# v1.17.0-beta.0
## Downloads for v1.17.0-beta.0

Download v1.17.0-beta.0 in the [v1.17.0-beta.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.17.0-beta.0).

## Changelog since v1.17.0-alpha.2

## Urgent Update Notes

## Changes by Kind

### API Changes
- Introduced `WorkloadAffinity` to `PropagationPolicy` API to support affinity and anti-affinity workload scheduling. ([#7131](https://github.com/karmada-io/karmada/pull/7131), @mszacillo)
- Introduced `WorkloadAffinityGroups` to `ResourceBinding/ClusterResourceBinding`, which will be used to hold instantiated grouping results. ([#7144](https://github.com/karmada-io/karmada/pull/7144), @RainbowMango)

### Features & Enhancements
- Introduced `WorkloadAffinity` feature gate, default to false. ([#7148](https://github.com/karmada-io/karmada/pull/7148), @mszacillo)

### Deprecation
- `karmada-controller-manager`: The flags `--cluster-lease-duration` and `--cluster-lease-renew-interval-fraction` have been deprecated and will be removed in a future release. ([#7126](https://github.com/karmada-io/karmada/pull/7126), @FAUST-BENCHOU)

### Bug Fixes
- `karmada-scheduler`: Fixed an issue that prevented multi-component workloads from being rescheduled during cluster failover events. ([#7066](https://github.com/karmada-io/karmada/pull/7066), @mszacillo)
- `karmada-scheduler`: Fixed the issue that `dynamicScaleDown` used stale clusters for scale-down operations. ([#7110](https://github.com/karmada-io/karmada/pull/7110), @zhzhuang-zju)
- `karmada-scheduler-estimator`: Fixed the issue where the resource quota plugin failed to list resource quotas due to a missing namespace in the gRPC request. ([#7124](https://github.com/karmada-io/karmada/pull/7124), @zhzhuang-zju)

### Security
None.

## Other
### Dependencies
- Karmada is now built with Golang v1.25.6. ([#7141](https://github.com/karmada-io/karmada/pull/7141), @RainbowMango)

### Helm Charts
None.

### Instrumentation
None.

### Performance
None.

# v1.17.0-alpha.2
## Downloads for v1.17.0-alpha.2

Download v1.17.0-alpha.2 in the [v1.17.0-alpha.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.17.0-alpha.2).

## Changelog since v1.17.0-alpha.1

## Urgent Update Notes

## Changes by Kind

### API Changes
- `karmada-operator`: Introduced `Tolerations` and `Affinity` fields to the `CommonSettings` of `Karmada` API for supporting explicit tolerations and affinity for Karmada control plane components. ([#6480](https://github.com/karmada-io/karmada/pull/6480), @abhinav-1305)

### Features & Enhancements
- `karmada-operator`: Now supports explicit tolerations and affinity for all Karmada control plane components via the `Karmada` CR. ([#6480](https://github.com/karmada-io/karmada/pull/6480), @abhinav-1305)

### Deprecation
None.

### Bug Fixes
- `karmada-controller-manager`: Fixed an issue where policy deletion could be blocked if a resource selector targeted a non-existent resource. ([#7038](https://github.com/karmada-io/karmada/pull/7038), @zhzhuang-zju)
- `karmada-webhook`: Fixed an issue where the `condition.reason` was not set to `QuotaExceeded` when FederatedResourceQuota is exceeded. ([#7086](https://github.com/karmada-io/karmada/pull/7086), @kajal-jotwani)

### Security
None.

## Other
### Dependencies
None.

### Helm Charts
None.

### Instrumentation
- `Instrumentation`: The metric `work_sync_workload_duration_seconds` no longer counts retriable Kubernetes 409 conflicts as errors, improving availability accuracy and reducing false alerts caused by conflict retry flapping. ([#7106](https://github.com/karmada-io/karmada/pull/7106), @RainbowMango)

### Performance
None.

# v1.17.0-alpha.1
## Downloads for v1.17.0-alpha.1

Download v1.17.0-alpha.1 in the [v1.17.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.17.0-alpha.1).

## Changelog since v1.17.0-alpha.0

## Urgent Update Notes

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-operator`: Added support for configuring priority class and pod disruption budget config for Karmada operator deployment. ([#7015](https://github.com/karmada-io/karmada/pull/7015), @jabellard)

### Deprecation
- `karmadactl`: `Etcd.Local.InitImage` in `Karmada Init Configuration` has been deprecated and will be removed in a future version. ([#6995](https://github.com/karmada-io/karmada/pull/6995), @zhzhuang-zju)

### Bug Fixes
- `karmada-scheduler`: Fixed the bug in the backoff queue where the sorting function was incorrect, potentially causing high-priority items with long backoffs to block lower-priority items. ([#6987](https://github.com/karmada-io/karmada/pull/6987), @rayo1uo)
- `karmadactl`: Fixed the messy auto-completion suggestions for commands like 'get' and 'apply'. ([#7023](https://github.com/karmada-io/karmada/pull/7023), @zhzhuang-zju)
- `karmadactl`: Removed the deprecated flag `--etcd-init-image` of `init` command. ([#6974](https://github.com/karmada-io/karmada/pull/6974), @AbhinavPInamdar)
- `karmada-controller-manager`: Fixed the issue where PP/CPP cannot be deleted because the resources API selected by the PP/CPP do not exist on the control plane. ([#7024](https://github.com/karmada-io/karmada/pull/7024), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that `HelmRelease` did not define `observedGeneration` variable in the `statusAggregation` operation. ([#7057](https://github.com/karmada-io/karmada/pull/7057), @FAUST-BENCHOU)

### Security
None.

## Other
### Dependencies
- Kubernetes dependencies have been updated to `v1.34.2`. ([#6999](https://github.com/karmada-io/karmada/pull/6999), @RainbowMango)
- The base image `alpine` has been promoted from `alpine:3.22.2` to `alpine:3.23.0`. ([#7001](https://github.com/karmada-io/karmada/pull/7001), @dependabot)
- The base image `alpine` has been promoted from `alpine:3.23.0` to `alpine:3.23.2`. ([#7034](https://github.com/karmada-io/karmada/pull/7034), @dependabot)

### Helm Charts
- `Helm chart`: Added helm index for 1.16 release. ([#6990](https://github.com/karmada-io/karmada/pull/6990), @zhzhuang-zju)
- `helm`: Upgraded `bitnami/common` dependency in karmada operator chart from `1.17.1` to `2.31.4`. ([#6994](https://github.com/karmada-io/karmada/pull/6994), @zhzhuang-zju)

### Instrumentation
None.

### Performance
None.
