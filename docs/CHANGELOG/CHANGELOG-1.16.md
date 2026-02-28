<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.16.3](#v1163)
  - [Downloads for v1.16.3](#downloads-for-v1163)
  - [Changelog since v1.16.2](#changelog-since-v1162)
    - [Changes by Kind](#changes-by-kind)
      - [Bug Fixes](#bug-fixes)
      - [Others](#others)
- [v1.16.2](#v1162)
  - [Downloads for v1.16.2](#downloads-for-v1162)
  - [Changelog since v1.16.1](#changelog-since-v1161)
    - [Changes by Kind](#changes-by-kind-1)
      - [Bug Fixes](#bug-fixes-1)
- [v1.16.1](#v1161)
  - [Downloads for v1.16.1](#downloads-for-v1161)
  - [Changelog since v1.16.0](#changelog-since-v1160)
    - [Changes by Kind](#changes-by-kind-2)
      - [Bug Fixes](#bug-fixes-2)
      - [Others](#others-1)
- [v1.16.0](#v1160)
  - [Downloads for v1.16.0](#downloads-for-v1160)
  - [Urgent Update Notes](#urgent-update-notes)
  - [What's New](#whats-new)
    - [Multi-Component Scheduling Support](#multi-component-scheduling-support)
    - [Enhance Replica Assignment with Webster Algorithm](#enhance-replica-assignment-with-webster-algorithm)
    - [Eviction Queue Rate Limiting](#eviction-queue-rate-limiting)
    - [Remarkable Performance Optimization of the Karmada Controllers](#remarkable-performance-optimization-of-the-karmada-controllers)
  - [Other Notable Changes](#other-notable-changes)
    - [API Changes](#api-changes)
    - [Features & Enhancements](#features--enhancements)
    - [Deprecation](#deprecation)
    - [Bug Fixes](#bug-fixes-3)
    - [Security](#security)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Charts](#helm-charts)
    - [Instrumentation](#instrumentation)
    - [Performance](#performance)
  - [Contributors](#contributors)
- [v1.16.0-rc.0](#v1160-rc0)
  - [Downloads for v1.16.0-rc.0](#downloads-for-v1160-rc0)
  - [Changelog since v1.16.0-beta.0](#changelog-since-v1160-beta0)
  - [Urgent Update Notes](#urgent-update-notes-1)
  - [Changes by Kind](#changes-by-kind-3)
    - [API Changes](#api-changes-1)
    - [Features & Enhancements](#features--enhancements-1)
    - [Deprecation](#deprecation-1)
    - [Bug Fixes](#bug-fixes-4)
    - [Security](#security-1)
  - [Other](#other-1)
    - [Dependencies](#dependencies-1)
    - [Helm Charts](#helm-charts-1)
    - [Instrumentation](#instrumentation-1)
    - [Performance](#performance-1)
- [v1.16.0-beta.0](#v1160-beta0)
  - [Downloads for v1.16.0-beta.0](#downloads-for-v1160-beta0)
  - [Changelog since v1.16.0-alpha.2](#changelog-since-v1160-alpha2)
  - [Urgent Update Notes](#urgent-update-notes-2)
  - [Changes by Kind](#changes-by-kind-4)
    - [API Changes](#api-changes-2)
    - [Features & Enhancements](#features--enhancements-2)
    - [Deprecation](#deprecation-2)
    - [Bug Fixes](#bug-fixes-5)
    - [Security](#security-2)
  - [Other](#other-2)
    - [Dependencies](#dependencies-2)
    - [Helm Charts](#helm-charts-2)
    - [Instrumentation](#instrumentation-2)
    - [Performance](#performance-2)
- [v1.16.0-alpha.2](#v1160-alpha2)
  - [Downloads for v1.16.0-alpha.2](#downloads-for-v1160-alpha2)
  - [Changelog since v1.16.0-alpha.1](#changelog-since-v1160-alpha1)
  - [Urgent Update Notes](#urgent-update-notes-3)
  - [Changes by Kind](#changes-by-kind-5)
    - [API Changes](#api-changes-3)
    - [Features & Enhancements](#features--enhancements-3)
    - [Deprecation](#deprecation-3)
    - [Bug Fixes](#bug-fixes-6)
    - [Security](#security-3)
  - [Other](#other-3)
    - [Dependencies](#dependencies-3)
    - [Helm Charts](#helm-charts-3)
    - [Instrumentation](#instrumentation-3)
- [v1.16.0-alpha.1](#v1160-alpha1)
  - [Downloads for v1.16.0-alpha.1](#downloads-for-v1160-alpha1)
  - [Changelog since v1.15.0](#changelog-since-v1150)
  - [Urgent Update Notes](#urgent-update-notes-4)
  - [Changes by Kind](#changes-by-kind-6)
    - [API Changes](#api-changes-4)
    - [Features & Enhancements](#features--enhancements-4)
    - [Deprecation](#deprecation-4)
    - [Bug Fixes](#bug-fixes-7)
    - [Security](#security-4)
  - [Other](#other-4)
    - [Dependencies](#dependencies-4)
    - [Helm Charts](#helm-charts-4)
    - [Instrumentation](#instrumentation-4)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.16.3
## Downloads for v1.16.3

Download v1.16.3 in the [v1.16.3 release page](https://github.com/karmada-io/karmada/releases/tag/v1.16.3).

## Changelog since v1.16.2
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue where the job status aggregator could enter an error loop due to a race condition when setting the initial `startTime`. ([#7158](https://github.com/karmada-io/karmada/pull/7158), @rohan-019)
- `karmada-controller-manager`: Fixed CronFederatedHPA scale-up from zero failure when the replicas field is missing. ([#7212](https://github.com/karmada-io/karmada/pull/7212), @zhengjr9)
- `karmada-controller-manager`: Fixed an issue where a per-task `GracePeriodSeconds` value could leak to subsequent graceful eviction tasks, causing premature or delayed evictions. ([#7187](https://github.com/karmada-io/karmada/pull/7187), @Ady0333)
- `karmada-controller-manager`: Fixed an issue where dependency updates could overwrite other controller annotations during retry conflicts. ([#7216](https://github.com/karmada-io/karmada/pull/7216), @Ady0333)
- `karmada-scheduler`: Fixed a scheduler panic caused by a divide-by-zero error when calculating spread constraints with no valid clusters. ([#7234](https://github.com/karmada-io/karmada/pull/7234), @XiShanYongYe-Chang)
- `karmada-scheduler`: Fixed the bug in the backoff queue where the sorting function was incorrect, potentially causing high-priority items with long backoffs to block lower-priority items. ([#7231](https://github.com/karmada-io/karmada/pull/7231), @zhzhuang-zju)
- `karmada-scheduler-estimator`: Fixed the issue where the resource quota plugin failed to list resource quotas due to a missing namespace in the gRPC request. ([#7238](https://github.com/karmada-io/karmada/pull/7238), @zhzhuang-zju)

#### Others
- The base image `alpine` has now been promoted from `alpine:3.23.2` to `alpine:3.23.3`. ([#7162](https://github.com/karmada-io/karmada/pull/7162), @dependabot)

# v1.16.2
## Downloads for v1.16.2

Download v1.16.2 in the [v1.16.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.16.2).

## Changelog since v1.16.1

### Changes by Kind

#### Bug Fixes
- `karmada-controller-manager`: Fixed an issue where policy deletion could be blocked if a resource selector targeted a non-existent resource. ([#7083](https://github.com/karmada-io/karmada/pull/7083), @FAUST-BENCHOU)
- `karmada-scheduler`: Fixed bug preventing multi-component workloads from being rescheduled during cluster failover events. ([#7130](https://github.com/karmada-io/karmada/pull/7130), @mszacillo)
- `karmada-webhook`: Fixed an issue where the `condition.reason` was not set to `QuotaExceeded` when FederatedResourceQuota is exceeded. ([#7098](https://github.com/karmada-io/karmada/pull/7098), @kajal-jotwani)

# v1.16.1
## Downloads for v1.16.1

Download v1.16.1 in the [v1.16.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.16.1).

## Changelog since v1.16.0
### Changes by Kind
#### Bug Fixes
- `karmadactl`: Fixed the messy auto-completion suggestions for commands like 'get' and 'apply'. ([#7025](https://github.com/karmada-io/karmada/pull/7025), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the issue where PP/CPP cannot be deleted because the resources API selected by the PP/CPP do not exist on the control plane. ([#7028](https://github.com/karmada-io/karmada/pull/7028), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that `HelmRelease` did not define `observedGeneration` variable in the `statusAggregation` operation. ([#7059](https://github.com/karmada-io/karmada/pull/7059), @FAUST-BENCHOU)

#### Others
- The base image `alpine` has been promoted from `alpine:3.22.2` to `alpine:3.23.0`. ([#7004](https://github.com/karmada-io/karmada/pull/7004), @dependabot)
- The base image `alpine` has been promoted from `alpine:3.23.0` to `alpine:3.23.2`. ([#7037](https://github.com/karmada-io/karmada/pull/7037), @dependabot)

# v1.16.0
## Downloads for v1.16.0

Download v1.16.0 in the [v1.16.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.16.0).

## Urgent Update Notes

## What's New

### Multi-Component Scheduling Support

In Karmada v1.16.0, we introduce **multi-component scheduling**, a new capability that enables the **complete and unified 
placement of multi-component workloads**—those composed of multiple interrelated components (e.g., jobManager and taskManagers of FlinkDeployment)—**into a 
single member cluster with sufficient resources**.

This feature builds upon the precise **resource interpreter** support added in v1.15, which allows Karmada to accurately 
understand the resource topology of complex workloads. In v1.16, the scheduler leverages this insight to:

- Estimate how many full sets of components can fit into a member cluster based on **ResourceQuota** limits.
- Predict schedulability using **actual node resource availability** across clusters.

Karmada v1.16.0 ships with built-in **resource interpreters** for the following multi-component workload types:
- `FlinkDeployment` (`flink.apache.org/v1beta1/FlinkDeployment`)
- `SparkApplication` (`sparkoperator.k8s.io/v1beta2/SparkApplication`)
- `Volcano Job` (`batch.volcano.sh/v1alpha1/Job`)
- `MPIJob` (`kubeflow.org/v2beta1/MPIJob`)
- `RayCluster` (`ray.io/v1/RayCluster`)
- `RayJob` (`ray.io/v1/RayJob`)
- `TFJob` (`kubeflow.org/v1/TFJob`)
- `PyTorchJob` (`kubeflow.org/v1/PyTorchJob`)

If you are using custom multi-component workloads, you can extend Karmada's resource interpreter to support them as well.  

> **Note**: Multi-component scheduling is **disabled by default**. To use it, enable the `MultiplePodTemplatesScheduling` 
> feature gate in your Karmada control plane components.

This release marks a significant step for modern distributed applications in Karmada—combining accurate resource interpretation, 
quota-aware accounting, and holistic cross-cluster scheduling in a single, cohesive framework.

For a detailed description of this feature, see the [Proposal: Multiple Pod Templates Scheduling](https://github.com/karmada-io/karmada/blob/master/docs/proposals/scheduling/multi-podtemplate-support/multiple-pod-template-support.md)

(Feature contributors: @mszacillo, @seanlaii, @RainbowMango, @zhzhuang-zju, @XiShanYongYe-Chang)

### Enhance Replica Assignment with Webster Algorithm

Karmada supports multiple replica scheduling strategies such as `DynamicWeight`, `Aggregated`, and `StaticWeight` to distribute 
workload replicas across member clusters. At the core of these strategies is the algorithm that maps cluster weights to actual replica counts.

In this release, we introduce the `Webster method`, also known as the `Sainte-Laguë` method, to improve replica assignment 
during cross-cluster scheduling. The previous algorithm had notable limitations: when the total number of replicas increased, 
some clusters could unexpectedly receive fewer replicas, and the assignment lacked strong idempotency.

By adopting the `Webster algorithm`, Karmada now achieves:

- **Monotonic replica assignment**: Increasing the total replica count will never cause any cluster to lose replicas, ensuring 
consistent and intuitive behavior.
- **Fair handling of remainder replicas**: When distributing replicas among clusters with equal weights, priority is given 
to the cluster with fewer current replicas. This "smaller-first" approach promotes balanced deployment and better satisfies 
high availability (HA) requirements.

This update enhances the stability, fairness, and predictability of workload distribution across clusters, making replica 
scheduling more robust in multi-cluster environments.

(Feature contributors: @RainbowMango, @zhzhuang-zju, @akakakakakaa)

### Eviction Queue Rate Limiting

In multi-cluster environments, when clusters experience failures, resources need to be evicted from failing clusters and 
rescheduled to healthy ones. If multiple clusters fail simultaneously or in rapid succession, aggressive eviction and rescheduling 
operations can overwhelm the healthy clusters and the control plane, potentially leading to cascading failures.

This release introduces an eviction queue with rate limiting capabilities for the Karmada taint manager. The eviction queue 
enhances the failover mechanism by controlling the rate of resource evictions using a configurable fixed rate parameter. 
The implementation also provides metrics for monitoring the eviction process, improving overall system observability.

This can be especially useful if:
- You need to prevent cascading failures during large-scale outages, ensuring the system doesn't get overwhelmed by too many eviction operations at once.
- You would like to configure eviction behavior based on your environment's characteristics. For example, using a lower eviction 
rate in production environments with mission-critical workloads versus higher rates in development or test setups.
- You need to monitor the performance of the eviction queue, including the number of pending evictions, processing latency, 
and success/failure rates, to tune the configuration and respond to operational issues.

Key features of the eviction queue include:
- **Configurable Fixed Rate Limiting**: Configure the eviction rate per second through the `--eviction-rate` command-line flag.
- **Comprehensive Metrics Support**: Provides metrics for queue depth, resource kind, processing latency, and success/failure 
rates for monitoring and troubleshooting.

By introducing the rate limiting mechanism, administrators can better control resource scheduling efficiency during cluster 
failover, balancing service stability with scheduling flexibility and efficiency.

For detailed usage of this feature, see [User Guide: Configuring Eviction Rate Limiting](https://karmada.io/docs/next/userguide/failover/cluster-failover#configuring-eviction-rate-limiting)

(Feature contributors: @wanggang, @XiShanYongYe-Chang)

### Remarkable Performance Optimization of the Karmada Controllers

In this release, the performance optimization team continues to enhance Karmada's performance with significant improvements in the controllers.

In release-1.15, we introduced the [controller-runtime priority queue](https://github.com/kubernetes-sigs/controller-runtime/issues/2374), 
which allows controllers built on controller-runtime to prioritize the most recent changes after a restart or leader transition, thereby 
significantly reducing downtime during service restarts. 

For controllers not built on controller-runtime, such 
as the detector controller, we extend this capability in release-1.16 by enabling the priority-queue functionality 
for all controllers that use async workers. **Now, we can achieve downtime reduction for all controllers after a restart or leader transition.**

The test setup includes 5,000 Deployments and their PropagationPolicies, with the `ControllerPriorityQueue` feature gate 
enabled in the karmada-controller-manager. Immediately following the restart of the karmada-controller-manager, a Deployment 
is manually modified while the work queue still contains a substantial number of pending events. The update event for the 
Deployment is promptly responded to and processed by Karmada controllers, and is ultimately synced to member clusters quickly.

These test results prove that the performance of karmada controllers has been greatly enhanced in v1.16. In the future, we 
will continue to carry out systematic performance optimizations on the controllers and the scheduler.

For the detailed progress and test report, please refer to [PR: [Performance] enable asyncPriorityWorker in all controllers](https://github.com/karmada-io/karmada/pull/6965).

(Performance contributors: @zach593)

## Other Notable Changes

### API Changes
- Introduced `Components` field to `ResourceInterpreterContext` in the `ResourceInterpreterResponse` to support interpreting components for webhook interpreter. ([#6740](https://github.com/karmada-io/karmada/pull/6740), @RainbowMango)
- `karmada-operator`: Introduced a `PodDisruptionBudget` field to the `CommonSettings` of `Karmada` API for supporting PodDisruptionBudgets (PDBs) for Karmada control plane components. ([#6895](https://github.com/karmada-io/karmada/pull/6895), @baiyutang)

### Features & Enhancements
- `karmadactl`: The `init` command now supports customizing Karmada component command line flags. ([#6637](https://github.com/karmada-io/karmada/pull/6637), @luyb177)
- `karmadactl`: The `init` command now supports customizing Karmada component command line flags via the configuration file. ([#6785](https://github.com/karmada-io/karmada/pull/6785), @luyb177)
- `karmadactl`: The `init` command's default `kube-apiserver` and `kube-controller-manager` images have been updated from v1.31.3 to v1.34.1. And the default `etcd` image has been updated from 3.5.16-0 to 3.6.0-0. ([#6970](https://github.com/karmada-io/karmada/pull/6970), @baiyutang)
- `karmada-controller-manager`: Introduced built-in interpreter for Volcano `Job`. ([#6790](https://github.com/karmada-io/karmada/pull/6790), @dekaihu)
- `karmada-controller-manager`: Computed effective field values for attached ResourceBindings when referenced by multiple ResourceBindings. ([#6796](https://github.com/karmada-io/karmada/pull/6796), @Kexin2000)
- `karmada-controller-manager`: Introduced a built-in interpreter for Kubeflow Notebooks. ([#6814](https://github.com/karmada-io/karmada/pull/6814), @dekaihu)
- `karmada-controller-manager`: Introduced built-in interpreter for SparkApplication. ([#6818](https://github.com/karmada-io/karmada/pull/6818), @liaolecheng)
- `karmada-controller-manager`: Introduced built-in interpreter for PyTorchJob. ([#6826](https://github.com/karmada-io/karmada/pull/6826), @pokerfaceSad)
- `karmada-controller-manager`: Introduced built-in resource interpreter for Kubernetes ReplicaSet workloads. ([#6833](https://github.com/karmada-io/karmada/pull/6833), @ryanwuer)
- `karmada-controller-manager`: Introduced built-in interpreter for MPIJob. ([#6927](https://github.com/karmada-io/karmada/pull/6927), @liaolecheng)
- `karmada-controller-manager`: Introduced `--resource-eviction-rate` flag to specify the eviction rate during cluster failover. ([#6777](https://github.com/karmada-io/karmada/pull/6777), @whosefriendA)
- `karmada-scheduler`: Implemented `MaxAvailableComponentSets` interface for general estimator based on resource summary. ([#6812](https://github.com/karmada-io/karmada/pull/6812), @mszacillo)
- `karmada-scheduler`: Migrated dynamic weight assignment to use the Webster algorithm. ([#6837](https://github.com/karmada-io/karmada/pull/6837), @zhzhuang-zju)
- `karmada-scheduler`: Enabled the capability for multiple component estimation in the scheduler. The feature is gated behind MultiplePodTemplatesScheduling. ([#6857](https://github.com/karmada-io/karmada/pull/6857), @RainbowMango)
- `karmada-scheduler`: Implemented `getMaximumSetsBasedOnResourceModels` in general estimator. ([#6912](https://github.com/karmada-io/karmada/pull/6912), @mszacillo)
- `karmada-scheduler-estimator`: Introduced MaxAvailableComponentSetsRequest & MaxAvailableComponentSetsResponse for component scheduling. ([#6787](https://github.com/karmada-io/karmada/pull/6787), @seanlaii)
- `karmada-scheduler-estimator`: Added plugins in estimator for component scheduling. ([#6864](https://github.com/karmada-io/karmada/pull/6864), @seanlaii)
- `karmada-scheduler-estimator`: Added `ResourceQuota` plugin for multi-component scheduling. ([#6875](https://github.com/karmada-io/karmada/pull/6875), @seanlaii)
- `karmada-scheduler-estimator`: Implemented maxAvailableComponentSets for the accurate estimator. ([#6876](https://github.com/karmada-io/karmada/pull/6876), @mszacillo)
- `karmada-scheduler-estimator`: Refactored the replica estimation logic by moving the node resource-based calculation into a dedicated, default plugin. ([#6877](https://github.com/karmada-io/karmada/pull/6877), @zhzhuang-zju)
- `karmada-scheduler-estimator`: Implemented the noderesource plugin for multi-component scheduling estimation. ([#6896](https://github.com/karmada-io/karmada/pull/6896), @zhzhuang-zju)
- `karmada-webhook`: Enabled federated resource quota calculation for multi-component scheduling. ([#6934](https://github.com/karmada-io/karmada/pull/6934), @zhzhuang-zju)
- `ResourceInterpreter`: Enabled `GetComponents` interpreter operation through Webhook Interpreter. ([#6745](https://github.com/karmada-io/karmada/pull/6745), @seanlaii)
- `ResourceInterpreter`: Added maxAvailableComponentSets to estimator interface. ([#6765](https://github.com/karmada-io/karmada/pull/6765), @mszacillo)
- `karmada-operator`: Added PodDisruptionBudget (PDB) support to enable high availability guarantees for all control plane components during planned disruptions. ([#6933](https://github.com/karmada-io/karmada/pull/6933), @baiyutang)
- `karmada-operator`: Updated default `kube-apiserver`, `kube-controller-manager` to v1.34.1, and `etcd` to v3.6.0-0. ([#6971](https://github.com/karmada-io/karmada/pull/6971), @baiyutang)

### Deprecation
- `karmada-operator`: Deprecated external etcd fields `CAData`, `CertData`, and `KeyData` have been removed. ([#6860](https://github.com/karmada-io/karmada/pull/6860), @jabellard)
- `karmadactl`: The flag `--etcd-init-image` of command `init` has been marked deprecated because it is no longer used, and will be removed in the future release. ([#6966](https://github.com/karmada-io/karmada/pull/6966), @satvik2131)

### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that `rbSpec.Components` is not updated when the template is updated. ([#6723](https://github.com/karmada-io/karmada/pull/6723), @zhzhuang-zju)
- `karmada-controller-manager`: Added a default timeout (32s) for the member cluster client to avoid the controller hanging when the member cluster does not respond. ([#6830](https://github.com/karmada-io/karmada/pull/6830), @Tanxin2333)
- `karmada-controller-manager`: Fixed the Job status cannot be aggregated issue due to the missing `JobSuccessCriteriaMet` condition when using kube-apiserver v1.32+ as Karmada API server. ([#6964](https://github.com/karmada-io/karmada/pull/6964), @RainbowMango)
- `karmada-controller-manager`: Fixed the issue that attached resource changes were not synchronized to the cluster in the dependencies distributor. ([#6931](https://github.com/karmada-io/karmada/pull/6931), @XiShanYongYe-Chang)
- `karmada-metrics-adapter`: Fixed a panic when querying node metrics by name caused by using the wrong GroupVersionResource (PodsGVR instead of NodesGVR) when creating a lister. ([#6838](https://github.com/karmada-io/karmada/pull/6838), @vie-serendipity)
- `karmada-operator`: Fixed the issue that CRDs cannot be updated during upgrades of the Karmada instance. ([#6775](https://github.com/karmada-io/karmada/pull/6775), @jabellard)
- `karmada-scheduler`: Fixed the issue where increasing the total number of replicas can cause some clusters to receive fewer replicas under the StaticWeight strategy by introducing the Webster algorithm. ([#6793](https://github.com/karmada-io/karmada/pull/6793), @zhzhuang-zju)
- `karmada-webhook`: Fixed the issue that resourcebinding validating webhook may panic when ReplicaRequirements of a Component in rbSpec.Components is nil. ([#6755](https://github.com/karmada-io/karmada/pull/6755), @zhzhuang-zju)
- `karmadactl`: Fixed the issue that the `register` command still uses the cluster-info endpoint when registering a pull-mode cluster, even if the user provides the API server endpoint. ([#6866](https://github.com/karmada-io/karmada/pull/6866), @ssenecal-modular)
- `ResourceInterpreter`: Fixed the issue that when an object API field name contains dots or colons, it would cause the resource interpreter to fail. ([#6749](https://github.com/karmada-io/karmada/pull/6749), @zhzhuang-zju)

### Security
None.

## Other
### Dependencies
- Karmada is now built with Golang v1.24.10. ([#6961](https://github.com/karmada-io/karmada/pull/6961), @RainbowMango)
- Kubernetes dependencies have been updated to v1.34.1. ([#6868](https://github.com/karmada-io/karmada/pull/6868), @RainbowMango)
- Updated `sigs.k8s.io/controller-runtime` from `v0.21.0` to `v0.22.4`. ([#6929](https://github.com/karmada-io/karmada/pull/6929), @RainbowMango)
- The base image `alpine` has been promoted from 3.22.1 to 3.22.2. ([#6822](https://github.com/karmada-io/karmada/pull/6822), @dependabot)

### Helm Charts
- The default `kube-apiserver` and `kube-controller-manager` images have been updated from v1.31.3 to v1.34.1. The default `etcd` Image has been updated from 3.5.16-0 to 3.6.0-0. ([#6948](https://github.com/karmada-io/karmada/pull/6948), @7h3-3mp7y-m4n)

### Instrumentation
- `karmada-controller-manager`: Added a new Warning event `DependencyPolicyConflict` to surface when dependency policies have conflicts. ([#6795](https://github.com/karmada-io/karmada/pull/6795), @Kexin2000)
- `karmada-controller-manager`: Added new metrics for the failover eviction queue to enhance observability. ([#6778](https://github.com/karmada-io/karmada/pull/6778), @whosefriendA)
- The `cluster` and `cluster_name` Prometheus metric labels previously used to denote a Karmada member cluster have been deprecated and will be removed in the `1.18` release. The newly introduced `member_cluster` metric label name will now be used for that purpose moving forward. ([#6932](https://github.com/karmada-io/karmada/pull/6932), @jabellard)

### Performance
- `karmada-controller-manager`: After enabling the `ControllerPriorityQueue` feature gate, the asyncWorker uses a priority queue based implementation, which affects the processing order of items in the resource detector and causes the monitoring metric `workqueue_depth` to be split into multiple series. ([#6919](https://github.com/karmada-io/karmada/pull/6919), @zach593)
- `karmada-controller-manager`: The `ControllerPriorityQueue` feature gate now applies to all controllers, including async workers. Enable it with `--feature-gates=ControllerPriorityQueue=true`. ([#6965](https://github.com/karmada-io/karmada/pull/6965), @zach593)

## Contributors
- 7h3-3mp7y-m4n
- baiyutang
- dekaihu
- devarsh10
- ikaven1024
- jabellard
- karmada-bot
- kasanatte
- Kexin2000
- liaolecheng
- LivingCcj
- luyb177
- mszacillo
- owenowenisme
- pokerfaceSad
- RainbowMango
- rayo1uo
- rohan-019
- ryanwuer
- satvik2131
- seanlaii
- ssenecal-modular
- SunsetB612
- vie-serendipity
- vsha96
- warjiang
- whosefriendA
- XiShanYongYe-Chang
- zach593
- zhzhuang-zju

# v1.16.0-rc.0
## Downloads for v1.16.0-rc.0

Download v1.16.0-rc.0 in the [v1.16.0-rc.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.16.0-rc.0).

## Changelog since v1.16.0-beta.0

## Urgent Update Notes

## Changes by Kind

### API Changes
- `karmada-operator`: Introduced a `PodDisruptionBudget` field to the `CommonSettings` of `Karmada` API for supporting PodDisruptionBudgets (PDBs) for Karmada control plane components. ([#6895](https://github.com/karmada-io/karmada/pull/6895), @baiyutang)

### Features & Enhancements
- `karmada-controller-manager`: Introduced a built-in interpreter for Kubeflow Notebooks. ([#6814](https://github.com/karmada-io/karmada/pull/6814), @dekaihu)

### Deprecation
None.

### Bug Fixes
None.

### Security
None.

## Other
### Dependencies
- Kubernetes dependencies have been updated to v1.34.1. ([#6868](https://github.com/karmada-io/karmada/pull/6868), @RainbowMango)

### Helm Charts
None.

### Instrumentation
None.

### Performance
None.

# v1.16.0-beta.0
## Downloads for v1.16.0-beta.0

Download v1.16.0-beta.0 in the [v1.16.0-beta.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.16.0-beta.0).

## Changelog since v1.16.0-alpha.2

## Urgent Update Notes

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmadactl`: The `init` command now supports customizing Karmada component command line flags via the configuration file. ([#6785](https://github.com/karmada-io/karmada/pull/6785), @luyb177)
- `karmada-controller-manager`: Introduced built-in interpreter for Volcano `Job`. ([#6790](https://github.com/karmada-io/karmada/pull/6790), @dekaihu)
- `karmada-controller-manager`: Added a new Warning event `DependencyPolicyConflict` to surface when dependency policies have conflicts. ([#6795](https://github.com/karmada-io/karmada/pull/6795), @Kexin2000)
- `karmada-controller-manager`: Compute effective field values for attached ResourceBindings when referenced by multiple ResourceBindings: - ConflictResolution: any Overwrite → Overwrite; else Abort. - PreserveResourcesOnDeletion: any true → true; else false. Applies only to dependency-generated ResourceBindings; policy-owned bindings are unaffected. ([#6796](https://github.com/karmada-io/karmada/pull/6796), @Kexin2000)
- `karmada-controller-manager`: Introduced built-in resource interpreter for Kubernetes ReplicaSet workloads. ([#6833](https://github.com/karmada-io/karmada/pull/6833), @ryanwuer)
- `karmada-controller-manager`: Introduced `--resource-eviction-rate` flag to specify the eviction rate during cluster failover. ([#6777](https://github.com/karmada-io/karmada/pull/6777), @whosefriendA)
- `karmada-scheduler`: Enabled the capability for multiple component estimation in the scheduler. The feature is gated behind MultiplePodTemplatesScheduling. ([#6857](https://github.com/karmada-io/karmada/pull/6857), @RainbowMango)
- `karmada-scheduler`: Migrate dynamic weight assignment to use the Webster algorithm. ([#6837](https://github.com/karmada-io/karmada/pull/6837), @zhzhuang-zju)
- `karmada-scheduler-estimator`: Allow adding plugins in estimator for component scheduling. ([#6864](https://github.com/karmada-io/karmada/pull/6864), @seanlaii)
- `karmada-scheduler-estimator`: Add `ResourceQuota` plugin for multi-component scheduling. ([#6875](https://github.com/karmada-io/karmada/pull/6875), @seanlaii)
- `karmada-scheduler-estimator`: implement the noderesource plugin for multi-component scheduling estimation. ([#6896](https://github.com/karmada-io/karmada/pull/6896), @zhzhuang-zju)
- `karmada-scheduler`/`karmada-scheduler-estimator`: Implements maxAvailableComponentSets for the accurate estimator. ([#6876](https://github.com/karmada-io/karmada/pull/6876), @mszacillo)
- `karmada-scheduler-estimator`: Refactors the replica estimation logic by moving the node resource-based calculation into a dedicated, default plugin. ([#6877](https://github.com/karmada-io/karmada/pull/6877), @zhzhuang-zju)

### Deprecation
- `karmada-operator`: Deprecated external etcd fields `CAData`, `CertData`, and `KeyData` have been removed. ([#6860](https://github.com/karmada-io/karmada/pull/6860), @jabellard)

### Bug Fixes
- `karmadactl`: Fixed the issue that the `register` command still uses the cluster-info endpoint when registering a pull-mode cluster, even if the user provides the API server endpoint. ([#6866](https://github.com/karmada-io/karmada/pull/6866), @ssenecal-modular)

### Security
None.

## Other
### Dependencies
- Karmada is now built with Golang v1.24.9. ([#6853](https://github.com/karmada-io/karmada/pull/6853), @rayo1uo)

### Helm Charts
None.

### Instrumentation
None.

### Performance
None.

# v1.16.0-alpha.2
## Downloads for v1.16.0-alpha.2

Download v1.16.0-alpha.2 in the [v1.16.0-alpha.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.16.0-alpha.2).

## Changelog since v1.16.0-alpha.1

## Urgent Update Notes

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmadactl`: The `init` command now supports customizing Karmada component command line flags. ([#6637](https://github.com/karmada-io/karmada/pull/6637), @luyb177)
- `karmada-scheduler-estimator`: Introduce MaxAvailableComponentSetsRequest & MaxAvailableComponentSetsResponse for component scheduling. ([#6787](https://github.com/karmada-io/karmada/pull/6787), @seanlaii)
- `karmada-scheduler`: Implemented `MaxAvailableComponentSets` interface for general estimator based on resource summary. ([#6812](https://github.com/karmada-io/karmada/pull/6812), @mszacillo)

### Deprecation
None.

### Bug Fixes
- `karmada-metrics-adapter`: Fixed a panic when querying node metrics by name caused by using the wrong GroupVersionResource (PodsGVR instead of NodesGVR) when creating a lister. ([#6838](https://github.com/karmada-io/karmada/pull/6838), @vie-serendipity)
- `karmadactl`: Fixed the issue that the `register` command still uses the cluster-info endpoint when registering a pull-mode cluster, even if the user provides the API server endpoint. ([#6866](https://github.com/karmada-io/karmada/pull/6866), @ssenecal-modular)

### Security
None.

## Other
### Dependencies
- The base image `alpine` now has been promoted from 3.22.1 to 3.22.2. ([#6822](https://github.com/karmada-io/karmada/pull/6822), @dependabot)

### Helm Charts
None.

### Instrumentation
None.

# v1.16.0-alpha.1
## Downloads for v1.16.0-alpha.1

Download v1.16.0-alpha.1 in the [v1.16.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.16.0-alpha.1).

## Changelog since v1.15.0

## Urgent Update Notes

## Changes by Kind

### API Changes
- Introduced `Components` field to `ResourceInterpreterContext` in the `ResourceInterpreterResponse` to support interpreting components for webhook interpreter. ([#6740](https://github.com/karmada-io/karmada/pull/6740), @RainbowMango)

### Features & Enhancements
- `ResourceInterpreter`: Enable `GetComponents` interpreter operation through Webhook Interpreter. ([#6745](https://github.com/karmada-io/karmada/pull/6745), @seanlaii)
- `ResourceInterpreter`: Adding maxAvailableComponentSets to estimator interface. ([#6765](https://github.com/karmada-io/karmada/pull/6765), @mszacillo)

### Deprecation
None.

### Bug Fixes
- `karmada-scheduler`: Fixed the issue where increasing the total number of replicas can cause some clusters to receive fewer replicas under the StaticWeight strategy by introducing the Webster algorithm. ([#6793](https://github.com/karmada-io/karmada/pull/6793), @RainbowMango, @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the issue that `rbSpec.Components` is not updated when the template is updated. ([#6723](https://github.com/karmada-io/karmada/pull/6723), @zhzhuang-zju)
- `ResourceInterpreter`: Fixed the issue that when an object API field name contains dots or colons, it would cause the resource interpreter to fail. ([#6749](https://github.com/karmada-io/karmada/pull/6749), @zhzhuang-zju)
- `karmada-webhook`: Fixed the issue that resourcebinding validating webhook may panic when ReplicaRequirements of a Component in rbSpec.Components is nil. ([#6755](https://github.com/karmada-io/karmada/pull/6755), @zhzhuang-zju)
- `karmada-operator`: Fixed the issue that CRDs can not be updated during upgrades of the Karmada instance. ([#6775](https://github.com/karmada-io/karmada/pull/6775), @jabellard)

### Security
- Bump github.com/vektra/mockery to v3.5.5 to address security concerns(GO-2025-3900). ([#6761](https://github.com/karmada-io/karmada/pull/6761), @RainbowMango)

## Other
### Dependencies

### Helm Charts
- `Helm chart`: Added helm index for 1.15 release. ([#6727](https://github.com/karmada-io/karmada/pull/6727), @liaolecheng)

### Instrumentation
None.
