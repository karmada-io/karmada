<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.15.4](#v1154)
  - [Downloads for v1.15.4](#downloads-for-v1154)
  - [Changelog since v1.15.3](#changelog-since-v1153)
    - [Changes by Kind](#changes-by-kind)
      - [Bug Fixes](#bug-fixes)
      - [Others](#others)
- [v1.15.3](#v1153)
  - [Downloads for v1.15.3](#downloads-for-v1153)
  - [Changelog since v1.15.2](#changelog-since-v1152)
    - [Changes by Kind](#changes-by-kind-1)
      - [Bug Fixes](#bug-fixes-1)
- [v1.15.2](#v1152)
  - [Downloads for v1.15.2](#downloads-for-v1152)
  - [Changelog since v1.15.1](#changelog-since-v1151)
    - [Changes by Kind](#changes-by-kind-2)
      - [Bug Fixes](#bug-fixes-2)
      - [Others](#others-1)
- [v1.15.1](#v1151)
  - [Downloads for v1.15.1](#downloads-for-v1151)
  - [Changelog since v1.15.0](#changelog-since-v1150)
    - [Changes by Kind](#changes-by-kind-3)
      - [Bug Fixes](#bug-fixes-3)
- [v1.15.0](#v1150)
  - [Downloads for v1.15.0](#downloads-for-v1150)
  - [Urgent Update Notes](#urgent-update-notes)
  - [What's New](#whats-new)
    - [Precise Quota Support for Multi-Component Workloads](#precise-quota-support-for-multi-component-workloads)
    - [Support Stateful Application Cluster Failover](#support-stateful-application-cluster-failover)
    - [Remarkable Performance Optimization of the Karmada Controllers and Schedulers](#remarkable-performance-optimization-of-the-karmada-controllers-and-schedulers)
      - [karmada controllers](#karmada-controllers)
      - [karmada scheduler](#karmada-scheduler)
  - [Other Notable Changes](#other-notable-changes)
    - [API Changes](#api-changes)
    - [Features & Enhancements](#features--enhancements)
    - [Deprecation](#deprecation)
    - [Bug Fixes](#bug-fixes-4)
    - [Security](#security)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Charts](#helm-charts)
    - [Instrumentation](#instrumentation)
    - [Performance](#performance)
  - [Contributors](#contributors)
- [v1.15.0-rc.0](#v1150-rc0)
  - [Downloads for v1.15.0-rc.0](#downloads-for-v1150-rc0)
  - [Changelog since v1.15.0-beta.0](#changelog-since-v1150-beta0)
  - [Urgent Update Notes](#urgent-update-notes-1)
  - [Changes by Kind](#changes-by-kind-4)
    - [API Changes](#api-changes-1)
    - [Features & Enhancements](#features--enhancements-1)
    - [Deprecation](#deprecation-1)
    - [Bug Fixes](#bug-fixes-5)
    - [Security](#security-1)
  - [Other](#other-1)
    - [Dependencies](#dependencies-1)
    - [Helm Charts](#helm-charts-1)
    - [Instrumentation](#instrumentation-1)
    - [Performance](#performance-1)
- [v1.15.0-beta.0](#v1150-beta0)
  - [Downloads for v1.15.0-beta.0](#downloads-for-v1150-beta0)
  - [Changelog since v1.15.0-alpha.2](#changelog-since-v1150-alpha2)
  - [Urgent Update Notes](#urgent-update-notes-2)
  - [Changes by Kind](#changes-by-kind-5)
    - [API Changes](#api-changes-2)
    - [Features & Enhancements](#features--enhancements-2)
    - [Deprecation](#deprecation-2)
    - [Bug Fixes](#bug-fixes-6)
    - [Security](#security-2)
  - [Other](#other-2)
    - [Dependencies](#dependencies-2)
    - [Helm Charts](#helm-charts-2)
    - [Instrumentation](#instrumentation-2)
    - [Performance](#performance-2)
- [v1.15.0-alpha.2](#v1150-alpha2)
  - [Downloads for v1.15.0-alpha.2](#downloads-for-v1150-alpha2)
  - [Changelog since v1.15.0-alpha.1](#changelog-since-v1150-alpha1)
  - [Urgent Update Notes](#urgent-update-notes-3)
  - [Changes by Kind](#changes-by-kind-6)
    - [API Changes](#api-changes-3)
    - [Features & Enhancements](#features--enhancements-3)
    - [Deprecation](#deprecation-3)
    - [Bug Fixes](#bug-fixes-7)
    - [Security](#security-3)
  - [Other](#other-3)
    - [Dependencies](#dependencies-3)
    - [Helm Charts](#helm-charts-3)
    - [Instrumentation](#instrumentation-3)
    - [Performance](#performance-3)
- [v1.15.0-alpha.1](#v1150-alpha1)
  - [Downloads for v1.15.0-alpha.1](#downloads-for-v1150-alpha1)
  - [Changelog since v1.14.0](#changelog-since-v1140)
  - [Urgent Update Notes](#urgent-update-notes-4)
  - [Changes by Kind](#changes-by-kind-7)
    - [API Changes](#api-changes-4)
    - [Features & Enhancements](#features--enhancements-4)
    - [Deprecation](#deprecation-4)
    - [Bug Fixes](#bug-fixes-8)
    - [Security](#security-4)
  - [Other](#other-4)
    - [Dependencies](#dependencies-4)
    - [Helm Charts](#helm-charts-4)
    - [Instrumentation](#instrumentation-4)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.15.4
## Downloads for v1.15.4

Download v1.15.4 in the [v1.15.4 release page](https://github.com/karmada-io/karmada/releases/tag/v1.15.4).

## Changelog since v1.15.3
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue where PP/CPP cannot be deleted because the resources API selected by the PP/CPP do not exist on the control plane. ([#7029](https://github.com/karmada-io/karmada/pull/7029), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that `HelmRelease` did not define `observedGeneration` variable in the `statusAggregation` operation. ([#7060](https://github.com/karmada-io/karmada/pull/7060), @FAUST-BENCHOU)

#### Others
- The base image `alpine` has been promoted from `alpine:3.22.2` to `alpine:3.23.0`. ([#7003](https://github.com/karmada-io/karmada/pull/7003), @dependabot)
- The base image `alpine` has been promoted from `alpine:3.23.0` to `alpine:3.23.2`. ([#7036](https://github.com/karmada-io/karmada/pull/7036), @dependabot)

# v1.15.3
## Downloads for v1.15.3

Download v1.15.3 in the [v1.15.3 release page](https://github.com/karmada-io/karmada/releases/tag/v1.15.3).

## Changelog since v1.15.2
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that `rbSpec.Components` is not updated when the template is updated. ([#6954](https://github.com/karmada-io/karmada/pull/6954), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the Job status cannot be aggregated issue due to the missing `JobSuccessCriteriaMet` condition when using kube-apiserver v1.32+ as Karmada API server. ([#6976](https://github.com/karmada-io/karmada/pull/6976), @RainbowMango)
- `karmada-controller-manager`: Fixed the issue that attached resource changes were not synchronized to the cluster in the dependencies distributor. ([#6982](https://github.com/karmada-io/karmada/pull/6982), @XiShanYongYe-Chang)
- `karmada-webhook`: Fixed the issue that resourcebinding validating webhook may panic when ReplicaRequirements of a Component in rbSpec.Components is nil. ([#6954](https://github.com/karmada-io/karmada/pull/6954), @zhzhuang-zju)
- `karmada-webhook`: Fixed the issue where the `FederatedResourceQuota` was not promptly updated when a multi-component workload generated new scheduling results. ([#6954](https://github.com/karmada-io/karmada/pull/6954), @zhzhuang-zju)
- `karmadactl`: Fixed the issue that the `register` command still uses the cluster-info endpoint when registering a pull-mode cluster, even if the user provides the API server endpoint. ([#6882](https://github.com/karmada-io/karmada/pull/6882), @ssenecal-modular)

# v1.15.2
## Downloads for v1.15.2

Download v1.15.2 in the [v1.15.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.15.2).

## Changelog since v1.15.1
### Changes by Kind
#### Bug Fixes
- `karmada-metrics-adapter`: Fixed a panic when querying node metrics by name caused by using the wrong GroupVersionResource (PodsGVR instead of NodesGVR) when creating a lister. ([#6844](https://github.com/karmada-io/karmada/pull/6844), @vie-serendipity)

#### Others
- Karmada is now built with Golang v1.24.9. ([#6856](https://github.com/karmada-io/karmada/pull/6856), @rayo1uo)

# v1.15.1
## Downloads for v1.15.1

Download v1.15.1 in the [v1.15.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.15.1).

## Changelog since v1.15.0
### Changes by Kind
#### Bug Fixes
- `ResourceInterpreter`: Fixed the issue that when an object API field name contains dots or colons, it would cause the resource interpreter to fail. ([#6767](https://github.com/karmada-io/karmada/pull/6767), @rohan-019)
- `karmada-operator`: Fixed the issue that CRDs can not be updated during upgrades of the Karmada instance. ([#6802](https://github.com/karmada-io/karmada/pull/6802), @RainbowMango)

# v1.15.0
## Downloads for v1.15.0

Download v1.15.0 in the [v1.15.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.15.0).

## Urgent Update Notes
None.

## What's New

### Precise Quota Support for Multi-Component Workloads

Previously, Karmada uses the `GetReplicas` interface to obtain workload replica and resource requests for calculating total resource requirements. However, many CRDs (such as FlinkDeployments) consist of multiple podTemplates or components, each with different resource requirements. The current interface cannot express these differences, leading to inaccurate resource calculations.

This release adds support for precise resource calculation of multi-component workloads through the new `GetComponents` interface. Karmada can now retrieve replica and resource requests for each component within a workload, enabling accurate resource accounting for Multi-Component Workloads.

This can be especially useful if:
- You need to track resource consumption and limits of Multi-Component Workloads.
- You would like to prevent unnecessary scheduling of Multi-Component Workloads by verifying quota resource limits.

As Multi-Component Workloads become more common in cloud-native ecosystems, Karmada recognizes the need for enhanced support for these workloads. Precise quota management is just one aspect of this effort. In the next release, we will further improve scheduling support for multi-component workloads — stay tuned!

For a detailed description of this feature, see the [Proposal: Multiple Pod Templates Support](https://github.com/karmada-io/karmada/tree/master/docs/proposals/scheduling/multi-podtemplate-support)

(Feature contributors: @mszacillo, @seanlaii, @RainbowMango, @zclyne, @Dyex719, @zhzhuang-zju, @liaolecheng)

### Support Stateful Application Cluster Failover

Karmada currently supports propagating various types of resources, including Kubernetes objects and CRDs. This also includes stateful workloads, enabling multi-cluster resilience and elastic scaling for distributed applications. The advantage of stateless workloads is their higher fault tolerance, as the loss of workloads does not impact application correctness or require data recovery. In contrast, stateful workloads (like Flink) depend on checkpointed or saved state to recover after failures. If that state is lost, the job may not be able to resume from where it left off.

For workloads, failover scenarios include two cases: the first is when the workload itself fails and needs to be migrated to another cluster; the second is when the cluster encounters a failure, such as node crashes, network partitions, or control plane outages, requiring some workloads within the cluster to be migrated.

The Karmada community has completed the design and development for the first scenario in version v1.12. In version v1.15, we will support the second scenario: cluster failover for stateful applications. We will ensure that when the migration mode is `Directly`, the Karmada system will first remove the application from the original cluster, and only after the application is completely removed can it be scheduled to the new cluster.

(Feature contributors: @XiShanYongYe-Chang, @liaolecheng)

### Remarkable Performance Optimization of the Karmada Controllers and Schedulers

In this release, the performance optimization team continues to enhance Karmada's performance with significant improvements in the controllers and the scheduler.

#### karmada controllers

For controllers, the implementation of the [controller-runtime priority queue](https://github.com/kubernetes-sigs/controller-runtime/issues/2374) allows controllers built on controller-runtime to prioritize the most recent changes after a restart or leader transition. This approach defers the processing of create events and prioritizes user-triggered resources, which can significantly reduce downtime during service restarts and failovers.

The test setup includes 5,000 Deployments, 2,500 Policies, and 5,000 Bindings, with the `ControllerPriorityQueue` feature gate enabled in the karmada-controller-manager. Immediately following the restart of the karmada-controller-manager, a Deployment and a Policy are updated while the work queue still contains a substantial number of pending events. The update events for both the Deployment and Policy are promptly responded to and processed by the controllers.

#### karmada scheduler

For the scheduler, optimizations are made to reduce redundant calculations during scheduling and decrease the frequency of remote calls to karmada-scheduler-estimator. These changes greatly improve the scheduler's QPS (Queries Per Second).

The test measures QPS by recording the time taken to schedule 5,000 ResourceBinding objects. The results are as follows:

- scheduling QPS increased from ~15 to ~22, a 46% improvement.
- scheduling gRPC requests to karmada-scheduler-estimator reduced from ~10000 to ~5000, a 50% reduction.

These test results prove that the performance of karmada controllers and karmada scheduler has been greatly enhanced in v1.15. In the future, we will continue to carry out systematic performance optimizations on the controllers and the scheduler.

For the detailed progress and test report, please refer to [Issue: [Performance] Overview of performance improvements for v1.15](https://github.com/karmada-io/karmada/issues/6516).

(Performance contributors: @zach593, @LivingCcj, @CharlesQQ, @zhzhuang-zju, @XiShanYongYe-Chang)

## Other Notable Changes
### API Changes
- Introduced `Components` field to `ResourceBinding`/`ClusterResourceBinding` API to represent the requirements of multiple pod templates. ([#6649](https://github.com/karmada-io/karmada/pull/6649), @RainbowMango)
- Introduced `ComponentResource` field to `ResourceInterpreterCustomization` API to hold rules for discovering the resource requirements of workloads consisting of multiple components. ([#6659](https://github.com/karmada-io/karmada/pull/6659), @RainbowMango)
- Introduced `spec.crdTarball.httpSource.proxy` field to `Karmada` API to optionally set a proxy for downloading CRD tarballs from an HTTP source. ([#6577](https://github.com/karmada-io/karmada/pull/6577), @jabellard)
- Introduced `spec.failover.cluster` field to the `PropagationPolicy` API for application-level cluster failover. ([#6610](https://github.com/karmada-io/karmada/pull/6610), @XiShanYongYe-Chang)
- The failover purge modes `Immediately`/`Graciously` have been deprecated and replaced by `Directly`/`Gracefully` respectively. ([#6387](https://github.com/karmada-io/karmada/pull/6387), @XiShanYongYe-Chang)

### Features & Enhancements
- `karmada-controller-manager`: Enhanced ServiceAccount retention logic to also preserve `imagePullSecrets`, preventing their continuous regeneration in member clusters. ([#6532](https://github.com/karmada-io/karmada/pull/6532), @whitewindmills)
- `karmada-controller-manager`: Added resource interpreter support for OpenKruise SidecarSet. Karmada can now interpret and manage OpenKruise SidecarSet resources across clusters, including multi-cluster status aggregation, health checks, dependency resolution for ConfigMaps and Secrets, and comprehensive test coverage. ([#6524](https://github.com/karmada-io/karmada/pull/6524), @abhi0324)
- `karmada-controller-manager`: Added resource interpreter support for OpenKruise UnitedDeployment. Karmada can now interpret and manage OpenKruise UnitedDeployment resources across clusters, including multi-cluster status aggregation, health checks, dependency resolution for ConfigMaps and Secrets, and comprehensive test coverage. ([#6533](https://github.com/karmada-io/karmada/pull/6533), @abhi0324)
- `karmada-controller-manager`: The detector now supports the `GetComponents` hook. This allows Karmada to resolve and schedule workloads with multiple components. ([#6665](https://github.com/karmada-io/karmada/pull/6665), @seanlaii)
- `karmada-controller-manager`: FederatedResourceQuota utilizes the new API `ResourceBindingSpec.Components` to accurately calculate resource requests for multi-template workloads (e.g., FlinkDeployment). ([#6701](https://github.com/karmada-io/karmada/pull/6701), @liaolecheng)
- `karmada-aggregated-apiserver`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6507](https://github.com/karmada-io/karmada/pull/6507), @ritzdevp)
- `karmada-scheduler-estimator`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6457](https://github.com/karmada-io/karmada/pull/6457), @linyao22)
- `karmada-scheduler`: Added `QuotaExceeded` as the reason for the legacy `Scheduled` condition on ResourceBinding when scheduling fails due to FederatedResourceQuota limits. ([#6481](https://github.com/karmada-io/karmada/pull/6481), @mszacillo)
- `karmada-scheduler`: Introduced `--logging-format` flag, which can be set to `json` to enable JSON logging. ([#6473](https://github.com/karmada-io/karmada/pull/6473), @zzklachlan)
- `karmada-webhook`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6430](https://github.com/karmada-io/karmada/pull/6430), @seanlaii)
- `karmada-search`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6466](https://github.com/karmada-io/karmada/pull/6466), @liwang0513)
- `karmada-metrics-adapter`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6439](https://github.com/karmada-io/karmada/pull/6439), @nihar4276)
- `karmada-descheduler`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6493](https://github.com/karmada-io/karmada/pull/6493), @mszacillo)
- `karmada-operator`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6496](https://github.com/karmada-io/karmada/pull/6496), @LeonZh0u)
- `ResourceInterpreter`: Implement the method `GetComponents` for `ConfigurableInterpreter`. ([#6678](https://github.com/karmada-io/karmada/pull/6678), @seanlaii)
- `ResourceInterpreter`: Implement the method `GetComponents` for `thirdpartyInterpreter`. ([#6687](https://github.com/karmada-io/karmada/pull/6687), @seanlaii)
- `ResourceInterpreter`: Implement the method `GetComponents` for `FlinkDeployment` custom interpreter. ([#6697](https://github.com/karmada-io/karmada/pull/6697), @mszacillo)
- `karmadactl`: Introduced `interpretComponent` operation in `karmadactl interpret`. ([#6696](https://github.com/karmada-io/karmada/pull/6696), @seanlaii)

### Deprecation
- `karmada-controller-manager`/`karmada-agent`: The deprecated label `propagation.karmada.io/instruction`, which was designed to suspend Work propagation, has now been removed. ([#6512](https://github.com/karmada-io/karmada/pull/6512), @XiShanYongYe-Chang)
- `karmada-controller-manager`: The flag `--failover-eviction-timeout` has been deprecated in release-1.14, now has been removed. ([#6450](https://github.com/karmada-io/karmada/pull/6450), @XiShanYongYe-Chang)
- `karmada-webhook`: The `--default-not-ready-toleration-seconds` and `--default-unreachable-toleration-seconds` flags which were deprecated in release-1.14, now has been removed. ([#6444](https://github.com/karmada-io/karmada/pull/6444), @XiShanYongYe-Chang)

### Bug Fixes
- `karmada-controller-manager`/`karmada-agent`: Fixed the issue that informer cache sync failures for specific resources incorrectly shut down all informers for that cluster, affecting resource distribution and work status synchronization. ([#6647](https://github.com/karmada-io/karmada/pull/6647), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the issue that the informer cache gets unexpectedly modified during usage. ([#6544](https://github.com/karmada-io/karmada/pull/6544), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed an issue where the ConfigurableInterpreter could report as synced before its underlying informer cache was fully synchronized, preventing potential out-of-sync errors during startup. ([#6602](https://github.com/karmada-io/karmada/pull/6602), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that resources are deleted unexpectedly due to the resourceInterpreter not properly handling the error and cache not being synced. ([#6612](https://github.com/karmada-io/karmada/pull/6612), @liaolecheng)
- `karmada-controller-manager`: Fixed the issue that ensures resource interpreter cache sync before starting controllers. ([#6626](https://github.com/karmada-io/karmada/pull/6626), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that endpointslice and work resources remain when using MCS and MCI simultaneously and then deleting them. ([#6622](https://github.com/karmada-io/karmada/pull/6622), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that EndpointSlices are deleted unexpectedly due to the EndpointSlice informer cache not being synced. ([#6434](https://github.com/karmada-io/karmada/pull/6434), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that reporting repeated EndpointSlice resources leads to duplicate backend IPs. ([#6523](https://github.com/karmada-io/karmada/pull/6523), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that resources will be recreated after being deleted on the cluster when resource is suspended for dispatching. ([#6525](https://github.com/karmada-io/karmada/pull/6525), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that a workload propagated with `duplicated mode` can bypass quota checks during scale up. ([#6474](https://github.com/karmada-io/karmada/pull/6474), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the issue where the federated-resource-quota-enforcement-controller miscalculates quota usage. ([#6477](https://github.com/karmada-io/karmada/pull/6477), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the bug that cluster can not be unjoined in case of the `--enable-taint-manager=false` or the feature gate `Failover` is disabled. ([#6446](https://github.com/karmada-io/karmada/pull/6446), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that `taint-manager` didn't honour `--no-execute-taint-eviction-purge-mode` when evicting `ClusterResourceBinding`. ([#6491](https://github.com/karmada-io/karmada/pull/6491), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue where pp-related claimMetadata was not properly cleaned up when deleting PropagationPolicy with Lazy activationPreference. ([#6656](https://github.com/karmada-io/karmada/pull/6656), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the issue that the relevant fields in rb and pp are inconsistent. ([#6674](https://github.com/karmada-io/karmada/pull/6674), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the issue that purgeMode not properly set when tolerationTime is 0 when cluster failover occurs. ([#6690](https://github.com/karmada-io/karmada/pull/6690), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that the binding's suspension persists when the policy deletes the suspension configuration. ([#6707](https://github.com/karmada-io/karmada/pull/6707), @zhzhuang-zju)
- `karmada-search`: Ensure the effective execution of informerfactory.WaitForCacheSync. ([#6554](https://github.com/karmada-io/karmada/pull/6554), @NickYadance)
- `karmadactl`: Fixed the issue of `promote` command that the interpreter cache is out of sync before first use. ([#6669](https://github.com/karmada-io/karmada/pull/6669), @XiShanYongYe-Chang)
- `karmada-interpreter-webhook-example`: Fixed write response error for broken HTTP connection issue. ([#6603](https://github.com/karmada-io/karmada/pull/6603), @whitewindmills)
- `ResourceInterpreter`: Using json-safe clone to prevent shared table references in `FlinkDeployment` luaResult. ([#6710](https://github.com/karmada-io/karmada/pull/6710), @mszacillo)
- `helm`: Fixed the issue where `helm upgrade` failed to update Karmada's static resources properly. ([#6395](https://github.com/karmada-io/karmada/pull/6395), @deefreak)

### Security
- Bump go version to 1.24.5 for addressing CVE-2025-4674 concern. ([#6557](https://github.com/karmada-io/karmada/pull/6557), @seanlaii)

## Other
### Dependencies
- Kubernetes dependencies have been updated to v1.33.2. ([#6498](https://github.com/karmada-io/karmada/pull/6498), @RainbowMango)
- Upgraded sigs.k8s.io/metrics-server to v0.8.0. ([#6548](https://github.com/karmada-io/karmada/pull/6548), @seanlaii)
- Upgraded sigs.k8s.io/kind to v0.29.0. ([#6549](https://github.com/karmada-io/karmada/pull/6549), @seanlaii)
- Upgraded vektra/mockery to v3.5.1, switching to a configuration-driven approach via mockery.yaml and removing deprecated v2 flags like `--inpackage` and `--name`. ([#6550](https://github.com/karmada-io/karmada/pull/6550), @liaolecheng)
- Upgraded controller-gen to v0.18.0. ([#6558](https://github.com/karmada-io/karmada/pull/6558), @seanlaii)
- Karmada is now built with Golang v1.24.6. ([#6630](https://github.com/karmada-io/karmada/pull/6630), @liaolecheng)
- The base image `alpine` now has been promoted from 3.21.3 to 3.22.1. ([#6560](https://github.com/karmada-io/karmada/pull/6560))

### Helm Charts
- `Chart`: Introduced parameter `certs.auto.rootCAExpiryDays` for root CA certification expiry customization. ([#6447](https://github.com/karmada-io/karmada/pull/6447), @ryanwuer)
- `karmada-search`: karmada-search helm chart template now references the resources from `search.resources`. ([#6517](https://github.com/karmada-io/karmada/pull/6517), @seanlaii)
- `Helm chart`: The Karmada Operator chart now allows setting environment variables and extra arguments for the Karmada operator deployment. ([#6596](https://github.com/karmada-io/karmada/pull/6596), @jabellard)
- `Helm Chart`: Users can now configure feature gates for karmada-scheduler via the Helm Chart using the `scheduler.featureGates` field in values.yaml. ([#6631](https://github.com/karmada-io/karmada/pull/6631), @seanlaii)

### Instrumentation

### Performance
- `karmada-controller-manager`/`karmada-agent`: Introduced a feature gate `ControllerPriorityQueue` that allows controllers to prioritize recent events, improving responsiveness during startup but potentially increasing the overall reconciliation frequency. ([#6662](https://github.com/karmada-io/karmada/pull/6662), @zach593)
- `karmada-scheduler`: Optimized the scheduler step, invoking calculate `availableReplicas` is reduced to once, improving the performance. ([#6597](https://github.com/karmada-io/karmada/pull/6597), @LivingCcj)

## Contributors

Thank you to everyone who contributed to this release!

Users whose commits are in this release (alphabetically by username)

- abhi0324
- abhinav-1305
- Bhaumik10
- CaesarTY
- cbaenziger
- deefreak
- greenmoon55
- iawia002
- jabellard
- jennryaz
- liaolecheng
- linyao22
- LivingCcj
- liwang0513
- mohamedawnallah
- mohit-nagaraj
- mszacillo
- RainbowMango
- ritzdevp
- ryanwuer
- seanlaii
- tessapham
- wangbowen1401
- wenhuwang
- whitewindmills
- XiShanYongYe-Chang
- zach593
- zclyne
- zhangsquared
- zhuyulicfc49
- zhzhuang-zju
- zzklachlan

# v1.15.0-rc.0
## Downloads for v1.15.0-rc.0

Download v1.15.0-rc.0 in the [v1.15.0-rc.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.15.0-rc.0).

## Changelog since v1.15.0-beta.0

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
- `karmada-operator`: Introduced `spec.crdTarball.httpSource.proxy` field in `Karmada` API to optionally set a proxy for downloading CRD tarballs from an HTTP source. ([#6577](https://github.com/karmada-io/karmada/pull/6577), @jabellard)

### Features & Enhancements
- `karmada-scheduler`: Added `QuotaExceeded` as the reason for the legacy `Scheduled` condition on ResourceBinding when scheduling fails due to FederatedResourceQuota limits. ([#6481](https://github.com/karmada-io/karmada/pull/6481), @mszacillo)

### Deprecation
None.

### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that the informer cache gets unexpectedly modified during usage. ([#6544](https://github.com/karmada-io/karmada/pull/6544), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed an issue where the ConfigurableInterpreter could report as synced before its underlying informer cache was fully synchronized, preventing potential out-of-sync errors during startup. ([#6602](https://github.com/karmada-io/karmada/pull/6602), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that resources are deleted unexpectedly due to the resourceInterpreter not properly handling the error and cache not being synced. ([#6612](https://github.com/karmada-io/karmada/pull/6612), @liaolecheng)
- `karmada-controller-manager`: Fixed the issue that endpointslice and work resources residue when using MCS and MCI simultaneously and then deleting them. ([#6622](https://github.com/karmada-io/karmada/pull/6622), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that ensures resource interpreter cache sync before starting controllers. ([#6626](https://github.com/karmada-io/karmada/pull/6626), @XiShanYongYe-Chang)
- `karmada-search`: Ensure the effective execution of informerfactory.WaitForCacheSync. ([#6554](https://github.com/karmada-io/karmada/pull/6554), @NickYadance)

### Security
None.

## Other
### Dependencies
- Karmada is now built with Golang v1.24.6. ([#6630](https://github.com/karmada-io/karmada/pull/6630), @liaolecheng)
- The base image `alpine` now has been promoted from 3.22.0 to 3.22.1. ([#6560](https://github.com/karmada-io/karmada/pull/6560))

### Helm Charts
- `Helm chart`: Karmada Operator chart now possible to set environment variables and extra arguments for the Karmada operator deployment. ([#6596](https://github.com/karmada-io/karmada/pull/6596), @jabellard)

### Instrumentation
None.

### Performance
None.

# v1.15.0-beta.0
## Downloads for v1.15.0-beta.0

Download v1.15.0-beta.0 in the [v1.15.0-beta.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.15.0-beta.0).

## Changelog since v1.15.0-alpha.2

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-controller-manager`: Added resource interpreter support for OpenKruise SidecarSet. Karmada can now interpret and manage OpenKruise SidecarSet resources across clusters, including multi-cluster status aggregation, health checks, dependency resolution for ConfigMaps and Secrets, and comprehensive test coverage. ([#6524](https://github.com/karmada-io/karmada/pull/6524), @abhi0324)
- `karmada-controller-manager`: Added resource interpreter support for OpenKruise UnitedDeployment. Karmada can now interpret and manage OpenKruise UnitedDeployment resources across clusters, including multi-cluster status aggregation, health checks, dependency resolution for ConfigMaps and Secrets, and comprehensive test coverage. ([#6533](https://github.com/karmada-io/karmada/pull/6533), @abhi0324)

### Deprecation
- The deprecated label `propagation.karmada.io/instruction`, which was designed to suspend Work propagation, has now been removed. ([#6512](https://github.com/karmada-io/karmada/pull/6512), @XiShanYongYe-Chang)

### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that EndpointSlice are deleted unexpectedly due to the EndpointSlice informer cache not being synced. ([#6434](https://github.com/karmada-io/karmada/pull/6434), @XiShanYongYe-Chang)

### Security
- Bump go version to 1.24.5 for addressing CVE-2025-4674 concern. ([#6557](https://github.com/karmada-io/karmada/pull/6557), @seanlaii)

## Other
### Dependencies
- Upgraded sigs.k8s.io/metrics-server to v0.8.0. ([#6548](https://github.com/karmada-io/karmada/pull/6548), @seanlaii)
- Upgraded sigs.k8s.io/kind to v0.29.0. ([#6549](https://github.com/karmada-io/karmada/pull/6549), @seanlaii)
- Upgraded vektra/mockery to v3.5.1, switching to a configuration-driven approach via mockery.yaml and removing deprecated v2 flags like --inpackage and --name. ([#6550](https://github.com/karmada-io/karmada/pull/6550), @liaolecheng)
- Upgraded controller-gen to v0.18.0. ([#6558](https://github.com/karmada-io/karmada/pull/6558), @seanlaii)

### Helm Charts
None.

### Instrumentation
None.

### Performance
None.

# v1.15.0-alpha.2
## Downloads for v1.15.0-alpha.2

Download v1.15.0-alpha.2 in the [v1.15.0-alpha.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.15.0-alpha.2).

## Changelog since v1.15.0-alpha.1

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-controller-manager`: Enhanced ServiceAccount retention logic to also preserve `imagePullSecrets`, preventing their continuous regeneration in member clusters. ([#6532](https://github.com/karmada-io/karmada/pull/6532), @whitewindmills)
- `karmada-aggregated-apiserver`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6507](https://github.com/karmada-io/karmada/pull/6507), @ritzdevp)

### Deprecation
None.

### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that resources will be recreated after being deleted on the cluster when resource is suspended for dispatching. ([#6525](https://github.com/karmada-io/karmada/pull/6525), @XiShanYongYe-Chang)

### Security
None.

## Other
### Dependencies
- Kubernetes dependencies have been updated to v1.33.2. ([#6498](https://github.com/karmada-io/karmada/pull/6498), @RainbowMango)

### Helm Charts
- `karmada-search`: karmada-search helm chart template now references the resources from `search.resources`. ([#6517](https://github.com/karmada-io/karmada/pull/6517), @seanlaii)

### Instrumentation
None.

### Performance
None.

# v1.15.0-alpha.1
## Downloads for v1.15.0-alpha.1

Download v1.15.0-alpha.1 in the [v1.15.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.15.0-alpha.1).

## Changelog since v1.14.0

## Urgent Update Notes

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-webhook`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6430](https://github.com/karmada-io/karmada/pull/6430), @seanlaii)
- `karmada-scheduler-estimator`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6457](https://github.com/karmada-io/karmada/pull/6457), @linyao22)
- `karmada-scheduler`: Introduced `--logging-format` flag, which can be set to `json` to enable JSON logging. ([#6473](https://github.com/karmada-io/karmada/pull/6473), @zzklachlan)
- `karmada-search`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6466](https://github.com/karmada-io/karmada/pull/6466), @liwang0513)
- `karmada-metrics-adapter`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6439](https://github.com/karmada-io/karmada/pull/6439), @nihar4276)
- `karmada-descheduler`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6493](https://github.com/karmada-io/karmada/pull/6493), @mszacillo)
- `karmada-operator`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6496](https://github.com/karmada-io/karmada/pull/6496), @LeonZh0u)

### Deprecation
- `karmada-webhook`: The `--default-not-ready-toleration-seconds` and `--default-unreachable-toleration-seconds` flags which were deprecated in release-1.14, now has been removed. ([#6444](https://github.com/karmada-io/karmada/pull/6444), @XiShanYongYe-Chang)
- `karmaa-controller-manager`: The flag `--failover-eviction-timeout` has been deprecated in release-1.14, now has been removed. ([#6450](https://github.com/karmada-io/karmada/pull/6450), @XiShanYongYe-Chang)

### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that a workload propagated with `duplicated mode` can bypass quota checks during scale up. ([#6474](https://github.com/karmada-io/karmada/pull/6474), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the issue where the federated-resource-quota-enforcement-controller miscalculates quota usage. ([#6477](https://github.com/karmada-io/karmada/pull/6477), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the bug that cluster can not be unjoined in case of the `--enable-taint-manager=false` or the feature gate `Failover` is disabled. ([#6446](https://github.com/karmada-io/karmada/pull/6446), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that `taint-maanger` didn't honour`--no-execute-taint-eviction-purge-mode` when evicting `ClusterResourceBinding`. ([#6491](https://github.com/karmada-io/karmada/pull/6491), @XiShanYongYe-Chang)
- `helm`:  Fixed the issue where `helm upgrade` failed to update Karmada's static resources properly. ([#6395](https://github.com/karmada-io/karmada/pull/6395), @deefreak)

### Security
None.

## Other
### Dependencies
- The base image `alpine` now has been promoted from 3.21.3 to 3.22.0. ([#6419](https://github.com/karmada-io/karmada/pull/6419))
- Bump go version to 1.24.4. ([#6490](https://github.com/karmada-io/karmada/pull/6490), @seanlaii)

### Helm Charts
- `Chart`: Introduced parameter `certs.auto.rootCAExpiryDays` for root ca certification expiry customization. ([#6447](https://github.com/karmada-io/karmada/pull/6447), @ryanwuer)

### Instrumentation
None.
