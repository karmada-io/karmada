<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.13.5](#v1135)
  - [Downloads for v1.13.5](#downloads-for-v1135)
  - [Changelog since v1.13.4](#changelog-since-v1134)
    - [Changes by Kind](#changes-by-kind)
      - [Bug Fixes](#bug-fixes)
      - [Others](#others)
- [v1.13.4](#v1134)
  - [Downloads for v1.13.4](#downloads-for-v1134)
  - [Changelog since v1.13.3](#changelog-since-v1133)
    - [Changes by Kind](#changes-by-kind-1)
      - [Bug Fixes](#bug-fixes-1)
      - [Others](#others-1)
- [v1.13.3](#v1133)
  - [Downloads for v1.13.3](#downloads-for-v1133)
  - [Changelog since v1.13.2](#changelog-since-v1132)
    - [Changes by Kind](#changes-by-kind-2)
      - [Bug Fixes](#bug-fixes-2)
      - [Others](#others-2)
- [v1.13.2](#v1132)
  - [Downloads for v1.13.2](#downloads-for-v1132)
  - [Changelog since v1.13.1](#changelog-since-v1131)
    - [Changes by Kind](#changes-by-kind-3)
      - [Bug Fixes](#bug-fixes-3)
      - [Others](#others-3)
- [v1.13.1](#v1131)
  - [Downloads for v1.13.1](#downloads-for-v1131)
  - [Changelog since v1.13.0](#changelog-since-v1130)
    - [Changes by Kind](#changes-by-kind-4)
      - [Bug Fixes](#bug-fixes-4)
      - [Others](#others-4)
- [v1.13.0](#v1130)
  - [Downloads for v1.13.0](#downloads-for-v1130)
  - [Urgent Update Notes](#urgent-update-notes)
  - [What's New](#whats-new)
    - [Application Priority Scheduling](#application-priority-scheduling)
    - [Support for resource scheduling suspend and resume capabilities](#support-for-resource-scheduling-suspend-and-resume-capabilities)
    - [Karmada Operator Continuous Enhancement](#karmada-operator-continuous-enhancement)
    - [Remarkable Performance Optimization of the Karmada Controllers](#remarkable-performance-optimization-of-the-karmada-controllers)
    - [Release of the First Version of the Karmada Dashboard](#release-of-the-first-version-of-the-karmada-dashboard)
  - [Other Notable Changes](#other-notable-changes)
    - [API Changes](#api-changes)
    - [Deprecation](#deprecation)
    - [Bug Fixes](#bug-fixes-5)
    - [Security](#security)
    - [Features & Enhancements](#features--enhancements)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Charts](#helm-charts)
    - [Instrumentation](#instrumentation)
  - [Contributors](#contributors)
- [v1.13.0-rc.0](#v1130-rc0)
  - [Downloads for v1.13.0-rc.0](#downloads-for-v1130-rc0)
  - [Changelog since v1.13.0-beta.0](#changelog-since-v1130-beta0)
  - [Urgent Update Notes](#urgent-update-notes-1)
  - [Changes by Kind](#changes-by-kind-5)
    - [API Changes](#api-changes-1)
    - [Features & Enhancements](#features--enhancements-1)
    - [Deprecation](#deprecation-1)
    - [Bug Fixes](#bug-fixes-6)
    - [Security](#security-1)
  - [Other](#other-1)
    - [Dependencies](#dependencies-1)
    - [Helm Charts](#helm-charts-1)
    - [Instrumentation](#instrumentation-1)
- [v1.13.0-beta.0](#v1130-beta0)
  - [Downloads for v1.13.0-beta.0](#downloads-for-v1130-beta0)
  - [Changelog since v1.13.0-alpha.2](#changelog-since-v1130-alpha2)
  - [Urgent Update Notes](#urgent-update-notes-2)
  - [Changes by Kind](#changes-by-kind-6)
    - [API Changes](#api-changes-2)
    - [Features & Enhancements](#features--enhancements-2)
    - [Deprecation](#deprecation-2)
    - [Bug Fixes](#bug-fixes-7)
    - [Security](#security-2)
  - [Other](#other-2)
    - [Dependencies](#dependencies-2)
    - [Helm Charts](#helm-charts-2)
    - [Instrumentation](#instrumentation-2)
- [v1.13.0-alpha.2](#v1130-alpha2)
  - [Downloads for v1.13.0-alpha.2](#downloads-for-v1130-alpha2)
  - [Changelog since v1.13.0-alpha.1](#changelog-since-v1130-alpha1)
  - [Urgent Update Notes](#urgent-update-notes-3)
  - [Changes by Kind](#changes-by-kind-7)
    - [API Changes](#api-changes-3)
    - [Features & Enhancements](#features--enhancements-3)
    - [Deprecation](#deprecation-3)
    - [Bug Fixes](#bug-fixes-8)
    - [Security](#security-3)
  - [Other](#other-3)
    - [Dependencies](#dependencies-3)
    - [Helm Charts](#helm-charts-3)
    - [Instrumentation](#instrumentation-3)
- [v1.13.0-alpha.1](#v1130-alpha1)
  - [Downloads for v1.13.0-alpha.1](#downloads-for-v1130-alpha1)
  - [Changelog since v1.12.0](#changelog-since-v1120)
  - [Urgent Update Notes](#urgent-update-notes-4)
  - [Changes by Kind](#changes-by-kind-8)
    - [API Changes](#api-changes-4)
    - [Features & Enhancements](#features--enhancements-4)
    - [Deprecation](#deprecation-4)
    - [Bug Fixes](#bug-fixes-9)
    - [Security](#security-4)
  - [Other](#other-4)
    - [Dependencies](#dependencies-4)
    - [Helm Charts](#helm-charts-4)
    - [Instrumentation](#instrumentation-4)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.13.5
## Downloads for v1.13.5

Download v1.13.5 in the [v1.13.5 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.5).

## Changelog since v1.13.4
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that resources will be recreated after being deleted on the cluster when resource is suspended for dispatching. ([#6537](https://github.com/karmada-io/karmada/pull/6537), @luyb177)
- `karmada-controller-manager`: Fixed the issue that EndpointSlice are deleted unexpectedly due to the EndpointSlice informer cache not being synced. ([#6584](https://github.com/karmada-io/karmada/pull/6584), @XiShanYongYe-Chang)

#### Others
- The base image `alpine` now has been promoted from 3.22.0 to 3.22.1. ([#6561](https://github.com/karmada-io/karmada/pull/6561))

# v1.13.4
## Downloads for v1.13.4

Download v1.13.4 in the [v1.13.4 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.4).

## Changelog since v1.13.3
### Changes by Kind
#### Bug Fixes
None.

#### Others
- The base image `alpine` now has been promoted from 3.21.3 to 3.22.0. ([#6421](https://github.com/karmada-io/karmada/pull/6421))

# v1.13.3
## Downloads for v1.13.3

Download v1.13.3 in the [v1.13.3 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.3).

## Changelog since v1.13.2
### Changes by Kind
#### Bug Fixes
- `karmada-scheduler`: Fixed the issue where resource scheduling suspension may become ineffective during a cluster update. ([#6360](https://github.com/karmada-io/karmada/pull/6360), @zhzhuang_zju)
- `karmada-scheduler`: Fixed the issue where bindings that fail occasionally will be treated as unschedulableBindings when feature gate PriorityBasedScheduling is enabled. ([#6374](https://github.com/karmada-io/karmada/pull/6374), @zhzhuang_zju)

#### Others
None.

# v1.13.2
## Downloads for v1.13.2

Download v1.13.2 in the [v1.13.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.2).

## Changelog since v1.13.1
### Changes by Kind
#### Bug Fixes
- `karmada-agent`: Fixed the issue where a new pull-mode cluster may overwrite the existing member clusters. ([#6259](https://github.com/karmada-io/karmada/pull/6259), @zhzhuang-zju)
- `karmadactl`: Fixed the issue where option `discovery-timeout` fails to work properly. ([#6280](https://github.com/karmada-io/karmada/pull/6280), @seanlaii)
- `karmada-controller-manager`: Fixed the issue that the result label of `federatedhpa_pull_metrics_duration_seconds` is always `success`. ([#6312](https://github.com/karmada-io/karmada/pull/6312), @tangzhongren)
- `karmada-controller-manager`/`karmada-agent`: Fixed the issue that cluster status update interval shorter than configured `--cluster-status-update-frequency`. ([#6335](https://github.com/karmada-io/karmada/pull/6335), @RainbowMango)

#### Others
None.

# v1.13.1
## Downloads for v1.13.1

Download v1.13.1 in the [v1.13.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.1).

## Changelog since v1.13.0
### Changes by Kind
#### Bug Fixes
- `karmada-operator`: The `karmada-app` label key previously used for control plane components and for the components of the operator itself has been changed to the more idiomatic `app.kubernetes.io/name` label key. ([#6200](https://github.com/karmada-io/karmada/pull/6200), @jabellard)
- `karmada-agent`: Fixed a panic issue where the agent does not need to report secret when registering cluster. ([#6222](https://github.com/karmada-io/karmada/pull/6222), @jabellard)
- `karmada-controller-manager`: Fixed the issue that the gracefulEvictionTask of ResourceBinding can not be cleared in case of schedule fails. ([#6234](https://github.com/karmada-io/karmada/pull/6234), @XiShanYongYe-Chang)
- `helm`: Fixed the issue where the required ServiceAccount was missing when the certificate mode was set to custom. ([#6242](https://github.com/karmada-io/karmada/pull/6242), @seanlaii)

#### Others
None.

# v1.13.0
## Downloads for v1.13.0

Download v1.13.0 in the [v1.13.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.0).

## Urgent Update Notes
- The Karmada Lua interpreter will no longer support the Lua functions `string.rep` and `string.gsub`. Typically, these functions are not
  frequently used in custom Karmada resource interpreters. They are disabled due to potential security risks. Before upgrading, please review
  your Lua scripts to verify whether these functions are being used. If they are, please replace them with alternative implementations.

## What's New

### Application Priority Scheduling

In some real-world scenarios, like AI training, some jobs are more critical than others and require preferential treatment in terms of scheduling and resource allocation. To enhance support for the above scenarios, this release karmada introduces the application priority scheduling feature, which allows users to set the priority of applications to control the order of application scheduling. Applications with higher priority will be scheduled ahead of other applications with lower priority. 

With this feature, users can:

- Classify workloads based on priority to achieve that high-priority workloads are scheduled ahead of others, rather than waiting to be scheduled in the order they were queued.  This will allow users to maintain the quality of service for important workloads even during resource contention.
- Set a higher priority for AI training tasks, ensuring that they can use GPU resources preferentially and guaranteeing the normal progress of AI training tasks.

For a detailed description of this feature, see the [Proposal: Binding Priority and Preemption](https://github.com/karmada-io/karmada/tree/master/docs/proposals/scheduling/binding-priority-preemption)

(Feature contributors: @whitewindmills, @LeonZh0u, @seanlaii, @wengyao04, @zclyne)

### Support for resource scheduling suspend and resume capabilities

Resource propagation in Karmada can be simply understood as two phases, resource scheduling and resource propagation. The propagation phase could already be flexibly [suspended and resumed](https://karmada.io/docs/next/userguide/scheduling/resource-propagating/#suspend-and-resume-of-resource-propagation) by users, a 
feature introduced in release v1.11. Now, the scheduling phase also has this feature.

By expanding the `ResourceBinding.ResourceBindingSpec.Spec.Suspension` field, users gain control over the suspension and resumption of scheduling. This enhancement opens up opportunities for external controllers to implement more advanced functionalities. For example:

- It helps in multi-tenancy scenarios, enabling priority control for different tenants. A cluster administrator can perform workload sorting and other operations on the upper layer, and resume scheduling if the sorting is completed.

- Quota checks are also possible during scheduling. Workloads will be dispatched only when the tenant resources are sufficient. Otherwise, the workload will be suspended for scheduling to achieve multi-tenant quota control.

For a detailed description of this feature, see the [Proposal: Support for resource scheduling suspend and resume capabilities](https://github.com/karmada-io/karmada/tree/master/docs/proposals/scheduling-suspension)

(Feature contributors: @Vacant2333, @Monokaix)

### Karmada Operator Continuous Enhancement

This release continues to enhance the Karmada Operator, which is responsible for managing the lifecycle of Karmada components. The following features are added:

- Support for API Server sidecar in Karmada Operator: This feature allows users to customize the Karmada API server container with sidecar containers, enabling users to integrate auxiliary services such as KMS plugins for configuring encryption at rest. For more information, see the [Proposal: Add Support for API Server Sidecar Containers in Karmada Operator](https://github.com/karmada-io/karmada/tree/master/docs/proposals/karmada-operator/api-server-sidecard-containers)
- Support for Component Priority Class Configuration in Karmada Operator: This feature allows users to configure the priority class for Karmada control plane components in the Karmada CR, ensuring reliable resource allocation and system stability across workloads. For more information, see the [Proposal: Support Priority Class Configuration for Karmada Control Plane Components](https://github.com/karmada-io/karmada/tree/master/docs/proposals/karmada-operator/component-priority-class-name)

These enhancements allow karmada-operator to be more flexible and customizable, enhancing the overall karmada system reliability and stability.

(Feature contributors: @jabellard)

### Remarkable Performance Optimization of the Karmada Controllers

As Karmada is being adopted by an increasing number of vendors and the deployment scale is growing larger, the performance optimization of Karmada has become a top priority. Karmada adopters spontaneously organize themselves to optimize the performance of Karmada, and achieved remarkable results.

In this release, the performance optimization of Karmada mainly centers around the controllers. The primary focus is to address the issue where the detector and binding-controller consume a significant amount of time during component restarts or master switches in large-scale data scenarios.

To address this, in v1.13, we carried out systematic performance optimizations. Testing was done on a 12C CPU, 28G RAM physical machine with 1000 deployments and 1000 configmaps (2000 resourceBindings). After restarting karmada-controller-manager, workqueue metrics showed significant improvements:
For detector: Element Average Processing Time(1-minute Sampling Period) Peak from 2.96s to 1.74s (41.22% reduction), total queue-clear time decreased by 60%, and peak-to-clear time decreased by 33.33%.
For binding-controller: Element Average Processing Time(1-minute Sampling Period) Peak from 1.98s to 0.96s (51.72% reduction), total queue-clear time decreased by 25%, and peak-to-clear time also decreased by 25%.

These data prove that the performance of these two controllers has been greatly enhanced in v1.13. In the future, we will continue to carry out systematic performance optimizations on the controllers and the scheduler.

(Feature contributors: @zach593, @CharlesQQ, @chaosi-zju)

### Release of the First Version of the Karmada Dashboard

With the continuous efforts of several enthusiastic developers, karmada dashboard finally welcomes its milestone first version (v0.1.0)! This version marks a significant step forward in the visual management of Karmada.

Karmada Dashboard is a general-purpose, web-based control panel for Karmada which is a multi-cluster management project. It aims to simplify the operation process of multi-cluster management and enhance the user experience. Through the Dashboard, users can intuitively view the cluster status, resource distribution, and task execution status. At the same time, users can also easily complete configuration adjustments and policy deployments.

The main features of this version include:
- Cluster Management: It provides cluster access and an overview of the cluster status, including health status and nodes number.
- Resource Management: It manages the configuration of business resources, covering namespaces, workloads, services, etc.
- Policy Management: It conducts the management of Karmada policies, including propagation policies, override policies, etc.

For a detailed description and usage guide, see the [Karmada Dashboard](https://github.com/karmada-io/dashboard)

(Feature contributors: @samzong, @RainbowMango, @warjiang, @axif0, @chouchongYHMing, @chaosi-zju, @Heylosky, @adwait-godbole, @carlory, @chouchongYHMing, @devadapter, @guozheng-shen, @jhnine, @LeonZh0u, @LiZhenCheng9527, @shauvet, @vibgreon, @zhouqunjie-cs)

## Other Notable Changes
### API Changes
- Add `SchedulePriority` field to `PropagationPolicy` and `ClusterPropagationPolicy`: ([#5962](https://github.com/karmada-io/karmada/pull/5962), @seanlaii)
  - Supports multiple priority class sources (Kubernetes, PodTemplate, Federated[future])
  - Configurable priority class resolution strategy
  - Maintains compatibility with Kubernetes priority behavior
  - Optional fields preserve backward compatibility
- Add `SchedulePriority`, `Priority`, and `PreemptionPolicy` fields to `ResourceBinding` API to support priority-based scheduling and preemption control. ([#5963](https://github.com/karmada-io/karmada/pull/5963), @seanlaii)
  - Priority: Defines scheduling priority (higher values = higher priority)
  - PreemptionPolicy: Controls whether binding can preempt lower priority bindings
- Make `priorityClassSource` in the `SchedulePriority` of `PropagationPolicy` a required field. ([#6163](https://github.com/karmada-io/karmada/pull/6163), @seanlaii)
- `API Change`: Introduced `PriorityClassName` in `Karmada` API which will be used to specify the priority class name of that component. ([#6068](https://github.com/karmada-io/karmada/pull/6068), @jabellard)
- `API Change`: Introduced `Scheduling` suspension in both `ResourceBinding` and `ClusterResourceBinding` which will be used for third-party systems to suspend application scheduling. ([#5937](https://github.com/karmada-io/karmada/pull/5937), @Monokaix)
- `karmada-operator`: The new `sidecarContainers` field as part of the Karmada API server component config can optionally be used to specify sidecar containers for the Karmada API server. ([#6133](https://github.com/karmada-io/karmada/pull/6133), @jabellard)

### Deprecation
- Replace `grpc.DialContext` and `grpc.WithBlock` with `grpc.NewClient` since DialContext and WithBlock are deprecated while maintaining the original functionality. ([#6026](https://github.com/karmada-io/karmada/pull/6026), @seanlaii)
- `karmadactl`: The flag `--ca-cert-path` of command `register` has been marked deprecated because it has never been used, and will be removed in the future release. ([#5862](https://github.com/karmada-io/karmada/pull/5862), @zhzhuang-zju)

### Bug Fixes
- `karmada-controller-manager`: Fixed an issue that the scheduling suspension on ResourceBinding might be mistakenly overwritten. ([#6062](https://github.com/karmada-io/karmada/pull/6062), @Monokaix)
- `karmada-controller-manager`: Fixed the issue that newly created attached-ResourceBinding be mystically garbage collected. ([#6034](https://github.com/karmada-io/karmada/pull/6034), @whitewindmills)
- `karmada-controller-manager`: Fixed the problem of ResourceBinding remaining after the resource template is deleted in the dependencies distribution scenario. ([#5943](https://github.com/karmada-io/karmada/pull/5943), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the bug of WorkloadRebalancer doesn't get deleted after TTL. ([#5989](https://github.com/karmada-io/karmada/pull/5989), @chaosi-zju)
- `karmada-controller-manager`: Fixed the issue of missing work queue metrics. ([#5972](https://github.com/karmada-io/karmada/pull/5972), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue where the `detector` unnecessary updates for RB issue. ([#6157](https://github.com/karmada-io/karmada/pull/6157), @CharlesQQ)
- `karmada-operator`: fix the "no such host" error when accessing the /convert webhook if Karmada instance is deployed in a namespace other than karmada-system. ([#6079](https://github.com/karmada-io/karmada/pull/6079), @zhzhuang-zju)
- `karmada-operator`: Fixed the issue that external ETCD certificate be overwritten by generated in-cluster ETCD certificate. ([#5976](https://github.com/karmada-io/karmada/pull/5976), @jabellard)
- `karmada-operator`: Fixed the error of validation failure when some fields of the Karmada CR are not configured. ([#6158](https://github.com/karmada-io/karmada/pull/6158), @zhzhuang-zju)
- `karmada-webhook`: Fixed panic when validating ResourceInterpreterWebhookConfiguration with unspecified service port. ([#5960](https://github.com/karmada-io/karmada/pull/5960), @XiShanYongYe-Chang)
- `karmada-search`: Fixed the issue that namespaces in different ResourceRegistry might be overwritten. ([#6065](https://github.com/karmada-io/karmada/pull/6065), @JimDevil)
- `karmadactl`: fix the "no such host" error when accessing the /convert webhook if Karmada instance is deployed in a namespace other than karmada-system via the `init` command. ([#6079](https://github.com/karmada-io/karmada/pull/6079), @zhzhuang-zju)

### Security
- `karmada-controller-manager`: For security reasons, we made the following changes to restrict the string library functions used when users customize the Karmada Lua interpreter. ([#6087](https://github.com/karmada-io/karmada/pull/6087), @zhzhuang-zju)
    1. do not allow users to use string.gsub and string.rep when interpreting resources with lua scripts, which may be used to create overly long strings.
    2. limit the length of the string type parameters of the function to 1000,000.
    3. add timeout checks to the internal for loops within the functions.

### Features & Enhancements
- `karmada-controller-manager`: FlinkDeployment health interpreter improvements, adding status.error to reflected status. ([#6073](https://github.com/karmada-io/karmada/pull/6073), @mszacillo)
- `karmada-controller-manager`: Populate schedule priority when building ResourceBinding. ([#6165](https://github.com/karmada-io/karmada/pull/6165), @LeonZh0u)
- `karmada-metrics-adapter`: Introduced `--metrics-bind-address` flag which will be used to expose Prometheus metrics. ([#6013](https://github.com/karmada-io/karmada/pull/6013), @chaosi-zju)
- `karmada-operator`: standardize the naming of karmada config in karmada-operator installation method. ([#6082](https://github.com/karmada-io/karmada/pull/6082), @seanlaii)
- `karmadactl`: command `init` now can specify the priority class name of the karmada components, default to `system-node-critical`. ([#6110](https://github.com/karmada-io/karmada/pull/6110), @zhzhuang-zju)
- `karmadactl`: The `unjoin` command is restricted to only unjoin push mode member clusters. The `unregister` command is restricted to only unregister pull mode member clusters. ([#6081](https://github.com/karmada-io/karmada/pull/6081), @zhzhuang-zju)
- `karmadactl`: standardize the naming of karmada config in karmadactl installation method. ([#5797](https://github.com/karmada-io/karmada/pull/5797), @chaosi-zju)
- `karmadactl`: Add Fish shell autocompletion support for improved command-line efficiency. ([#5876](https://github.com/karmada-io/karmada/pull/5876), @tiansuo114)
- `karmadactl`: command `addon` now can specify the priority class name of the karmada components, default to `system-node-critical`. ([#6111](https://github.com/karmada-io/karmada/pull/6111), @zhzhuang-zju)

## Other
### Dependencies
- Karmada now built with Golang v1.22.12. ([#6131](https://github.com/karmada-io/karmada/pull/6131), @sachinparihar)
- The base image `alpine` now has been promoted from 3.21.2 to 3.21.3. ([#6124](https://github.com/karmada-io/karmada/pull/6124))
- update kubernetes version to v1.31.3 ([#5910](https://github.com/karmada-io/karmada/pull/5910), @dongjiang1989)

### Helm Charts
- `helm`: The new `PriorityClassName` field added as part of the Karmada control plane component configurations can be used to specify the priority class name of that component, default to `system-node-critical`. ([#6108](https://github.com/karmada-io/karmada/pull/6108), @zhzhuang-zju)
- upgrade helm chart index to v1.12.0. ([#5918](https://github.com/karmada-io/karmada/pull/5918), @chaosi-zju)

### Instrumentation
- The cluster status-related metrics, emitting from `karmada-controller-manager`, will be cleaned up after the cluster is removed. ([#5866](https://github.com/karmada-io/karmada/pull/5866), @CharlesQQ)

## Contributors
Thank you to everyone who contributed to this release!

Users whose commits are in this release (alphabetically by username)

- @adwait-godbole
- @anujagrawal699
- @axif0
- @carlory
- @chaosi-zju
- @CharlesQQ
- @chouchongYHMing
- @devadapter
- @dongjiang1989
- @gabrielsrs
- @guozheng-shen
- @Heylosky
- @iawia002
- @jabellard
- @jhnine
- @JimDevil
- @LavredisG
- @LeonZh0u
- @LiZhenCheng9527
- @ls-2018
- @mohamedawnallah
- @Monokaix
- @mszacillo
- @RainbowMango
- @sachinparihar
- @samzong
- @seanlaii
- @shauvet
- @SkySingh04
- @tiansuo114
- @Vacant2333
- @vibgreon
- @warjiang
- @whitewindmills
- @XiShanYongYe-Chang
- @y1hao
- @yashpandey06
- @zach593
- @zhouqunjie-cs
- @zhzhuang-zju
- @ZwangaMukwevho

# v1.13.0-rc.0
## Downloads for v1.13.0-rc.0

Download v1.13.0-rc.0 in the [v1.13.0-rc.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.0-rc.0).

## Changelog since v1.13.0-beta.0

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-controller-manager`: FlinkDeployment health interpreter improvements, adding status.error to reflected status. ([#6073](https://github.com/karmada-io/karmada/pull/6073), @mszacillo)
- `karmada-operator`: standardize the naming of karmada config in karmada-operator installation method. ([#6082](https://github.com/karmada-io/karmada/pull/6082), @seanlaii)
- `karmadactl`: command `init` now can specify the priority class name of the karmada components, default to `system-node-critical`. ([#6110](https://github.com/karmada-io/karmada/pull/6110), @zhzhuang-zju)
- `karmadactl`: The `unjoin` command is restricted to only unjoin push mode member clusters. The `unregister` command is restricted to only unregister pull mode member clusters. ([#6081](https://github.com/karmada-io/karmada/pull/6081), @zhzhuang-zju)

### Deprecation
None.

### Bug Fixes
- `karmada-operator`: fix the "no such host" error when accessing the /convert webhook if Karmada instance is deployed in a namespace other than karmada-system. ([#6079](https://github.com/karmada-io/karmada/pull/6079), @zhzhuang-zju)
- `karmadactl`: fix the "no such host" error when accessing the /convert webhook if Karmada instance is deployed in a namespace other than karmada-system via the `init` command. ([#6079](https://github.com/karmada-io/karmada/pull/6079), @zhzhuang-zju)

### Security
None.

## Other
### Dependencies
None.

### Helm Charts
- `helm`: The new `PriorityClassName` field added as part of the Karmada control plane component configurations can be used to specify the priority class name of that component, default to `system-node-critical`. ([#6108](https://github.com/karmada-io/karmada/pull/6108), @zhzhuang-zju)

### Instrumentation
- The cluster status-related metrics, emitting from `karmada-controller-manager`, will be cleaned up after the cluster is removed. ([#5866](https://github.com/karmada-io/karmada/pull/5866), @CharlesQQ)

# v1.13.0-beta.0
## Downloads for v1.13.0-beta.0

Download v1.13.0-beta.0 in the [v1.13.0-beta.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.0-beta.0).

## Changelog since v1.13.0-alpha.2

## Urgent Update Notes
- The Karmada Lua interpreter will no longer support the Lua functions `string.rep` and `string.gsub`. Typically, these functions are not 
frequently used in custom Karmada resource interpreters. They are disabled due to potential security risks. Before upgrading, please review 
your Lua scripts to verify whether these functions are being used. If they are, please replace them with alternative implementations.

## Changes by Kind

### API Changes
- `API Change`: Introduced `PriorityClassName` in `Karmada` API which will be used to specify the priority class name of that component. ([#6068](https://github.com/karmada-io/karmada/pull/6068), @jabellard)

### Features & Enhancements
- `karmadactl`: standardize the naming of karmada config in karmadactl installation method. ([#5797](https://github.com/karmada-io/karmada/pull/5797), @chaosi-zju)

### Deprecation
None.

### Bug Fixes
- `karmada-controller-manager`: Fixed an issue that the scheduling suspension on ResourceBinding might be mistakenly overwritten. ([#6062](https://github.com/karmada-io/karmada/pull/6062), @Monokaix)
- `karmada-search`: Fixed the issue that namespaces in different ResourceRegistry might be overwritten. ([#6065](https://github.com/karmada-io/karmada/pull/6065), @JimDevil)

### Security
- `karmada-controller-manager`: For security reasons, we made the following changes to restrict the string library functions used when users customize the Karmada Lua interpreter. ([#6087](https://github.com/karmada-io/karmada/pull/6087), @zhzhuang-zju)
  1. do not allow users to use string.gsub and string.rep when interpreting resources with lua scripts, which may be used to create overly long strings.
  2. limit the length of the string type parameters of the function to 1000,000.
  3. add timeout checks to the internal for loops within the functions.

## Other
### Dependencies
- Karmada now built with Golang v1.22.11. ([#6066](https://github.com/karmada-io/karmada/pull/6066), @y1hao)

### Helm Charts
None.

### Instrumentation
None.

# v1.13.0-alpha.2
## Downloads for v1.13.0-alpha.2

Download v1.13.0-alpha.2 in the [v1.13.0-alpha.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.0-alpha.2).

## Changelog since v1.13.0-alpha.1

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-metrics-adapter`: Introduced `--metrics-bind-address` flag which will be used to expose Prometheus metrics. ([#6013](https://github.com/karmada-io/karmada/pull/6013), @chaosi-zju)

### Deprecation
- Replace `grpc.DialContext` and `grpc.WithBlock` with `grpc.NewClient` since DialContext and WithBlock are deprecated while maintaining the original functionality. ([#6026](https://github.com/karmada-io/karmada/pull/6026), @seanlaii)

### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that newly created attached-ResourceBinding be mystically garbage collected. ([#6034](https://github.com/karmada-io/karmada/pull/6034), @whitewindmills)

### Security
None.

## Other
### Dependencies
- The base image `alpine` now has been promoted from 3.21.0 to 3.21.2. ([#6040](https://github.com/karmada-io/karmada/pull/6040))

### Helm Charts
None.

### Instrumentation
None.

# v1.13.0-alpha.1
## Downloads for v1.13.0-alpha.1

Download v1.13.0-alpha.1 in the [v1.13.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.0-alpha.1).

## Changelog since v1.12.0

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
- `API Change`: Introduced `Scheduling` suspension in both `ResourceBinding` and `ClusterResourceBinding` which will be used for third-party systems to suspend application scheduling. ([#5937](https://github.com/karmada-io/karmada/pull/5937), @Monokaix)

### Features & Enhancements
- `karmadactl`: Add Fish shell autocompletion support for improved command-line efficiency. ([#5876](https://github.com/karmada-io/karmada/pull/5876), @tiansuo114)

### Deprecation
- `karmadactl`: The flag `--ca-cert-path` of command `register` has been marked deprecated because it has never been used, and will be removed in the future release. ([#5862](https://github.com/karmada-io/karmada/pull/5862), @zhzhuang-zju)

### Bug Fixes
- `karmada-controller-manager`: Fixed the problem of ResourceBinding remaining after the resource template is deleted in the dependencies distribution scenario. ([#5943](https://github.com/karmada-io/karmada/pull/5943), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the bug of WorkloadRebalancer doesn't get deleted after TTL. ([#5989](https://github.com/karmada-io/karmada/pull/5989), @chaosi-zju)
- `karmada-controller-manager`: Fixed the issue of missing work queue metrics. ([#5972](https://github.com/karmada-io/karmada/pull/5972), @XiShanYongYe-Chang)
- `karmada-webhook`: Fixed panic when validating ResourceInterpreterWebhookConfiguration with unspecified service port. ([#5960](https://github.com/karmada-io/karmada/pull/5960), @XiShanYongYe-Chang)
- `karmada-operator`: Fixed the issue that external ETCD certificate be overwritten by generated in-cluster ETCD certificate. ([#5976](https://github.com/karmada-io/karmada/pull/5976), @jabellard)

### Security
None.

## Other
### Dependencies
- update kubernetes version to v1.31.3 ([#5910](https://github.com/karmada-io/karmada/pull/5910), @dongjiang1989)
- The base image `alpine` now has been promoted from `3.20.3` to `3.21.0`. ([#5920](https://github.com/karmada-io/karmada/pull/5920))

### Helm Charts
- upgrade helm chart index to v1.12.0. ([#5918](https://github.com/karmada-io/karmada/pull/5918), @chaosi-zju)

### Instrumentation
None.
