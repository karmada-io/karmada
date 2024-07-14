<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.7.5](#v175)
  - [Downloads for v1.7.5](#downloads-for-v175)
  - [Changelog since v1.7.4](#changelog-since-v174)
    - [Changes by Kind](#changes-by-kind)
      - [Bug Fixes](#bug-fixes)
      - [Others](#others)
- [v1.7.4](#v174)
  - [Downloads for v1.7.4](#downloads-for-v174)
  - [Changelog since v1.7.3](#changelog-since-v173)
    - [Changes by Kind](#changes-by-kind-1)
      - [Bug Fixes](#bug-fixes-1)
      - [Others](#others-1)
- [v1.7.3](#v173)
  - [Downloads for v1.7.3](#downloads-for-v173)
  - [Changelog since v1.7.2](#changelog-since-v172)
    - [Changes by Kind](#changes-by-kind-2)
      - [Bug Fixes](#bug-fixes-2)
      - [Others](#others-2)
- [v1.7.2](#v172)
  - [Downloads for v1.7.2](#downloads-for-v172)
  - [Changelog since v1.7.1](#changelog-since-v171)
    - [Changes by Kind](#changes-by-kind-3)
      - [Bug Fixes](#bug-fixes-3)
      - [Others](#others-3)
- [v1.7.1](#v171)
  - [Downloads for v1.7.1](#downloads-for-v171)
  - [Changelog since v1.7.0](#changelog-since-v170)
    - [Changes by Kind](#changes-by-kind-4)
      - [Bug Fixes](#bug-fixes-4)
      - [Others](#others-4)
- [v1.7.0](#v170)
  - [Downloads for v1.7.0](#downloads-for-v170)
  - [What's New](#whats-new)
    - [CronFederatedHPA](#cronfederatedhpa)
    - [MultiClusterService](#multiclusterservice)
    - [PropagationPolicy Preemption](#propagationpolicy-preemption)
    - [Migrating resources in batches](#migrating-resources-in-batches)
    - [FederatedHPA](#federatedhpa)
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

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.7.5
## Downloads for v1.7.5

Download v1.7.5 in the [v1.7.5 release page](https://github.com/karmada-io/karmada/releases/tag/v1.7.5).

## Changelog since v1.7.4
### Changes by Kind
#### Bug Fixes
None.

#### Others
- The base image `alpine` now has been promoted from `alpine:3.19.1` to `alpine:3.20.0`.

# v1.7.4
## Downloads for v1.7.4

Download v1.7.4 in the [v1.7.4 release page](https://github.com/karmada-io/karmada/releases/tag/v1.7.4).

## Changelog since v1.7.3
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed incorrect annotation markup when policy preemption occurs. ([#4770](https://github.com/karmada-io/karmada/pull/4770), @XiShanYongYe-Chang)

#### Others
None.

# v1.7.3
## Downloads for v1.7.3

Download v1.7.3 in the [v1.7.3 release page](https://github.com/karmada-io/karmada/releases/tag/v1.7.3).

## Changelog since v1.7.2
### Changes by Kind
#### Bug Fixes
- `karmada-controllr-manager`: concurrency of `binding-status controller` inherit from the concurrency of `binding` declared in global options. ([#4340](https://github.com/karmada-io/karmada/pull/4340), @lxtywypc)
- `karmada-search`: Add the logic of checking whether the resource API to be retrieved is installed in the cluster. ([#4589](https://github.com/karmada-io/karmada/pull/4589), @XiShanYongYe-Chang)
- `karmada-search`: support accept content type `as=Table` in the proxy global resource function. ([#4595](https://github.com/karmada-io/karmada/pull/4595), @jwcesign)

#### Others
- The base image `alpine` now has been promoted from `alpine:3.18.5` to `alpine:3.19.1`. ([#4600](https://github.com/karmada-io/karmada/pull/4600), @Fish-pro)

# v1.7.2
## Downloads for v1.7.2

Download v1.7.2 in the [v1.7.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.7.2).

## Changelog since v1.7.1
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: only update `aggregatedStatus` and `conditions` fields during `binding-status controller` updating status of binding. ([#4234](https://github.com/karmada-io/karmada/pull/4234), @jwcesign)
- `karmada-controller-manager`: Pruned job labels `batch.kubernetes.io/controller-uid` and `batch.kubernetes.io/job-name` which were introduced by Kubernetes 1.27. ([#4263](https://github.com/karmada-io/karmada/pull/4263), @liangyuanpeng)
- `karmada-search`: Fix lock race affects watch RestChan not close, causing client hanging when performing watch. ([#4239](https://github.com/karmada-io/karmada/pull/4239), @xigang)
- `karmada-operator`: Fixed can not load Karmada v1.7.0 crds issue.  ([#4207](https://github.com/karmada-io/karmada/pull/4207), @liangyuanpeng)

#### Others
- `release1.7`: The base image `alpine` now has been promoted from `alpine:3.18.3` to `alpine:3.18.5` ([#4380](https://github.com/karmada-io/karmada/pull/4380), @zhzhuang-zju)
- Karmada is now built with Go1.20.12.  ([#4369](https://github.com/karmada-io/karmada/pull/4396), @RainbowMango)

# v1.7.1
## Downloads for v1.7.1

Download v1.7.1 in the [v1.7.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.7.1).

## Changelog since v1.7.0
### Changes by Kind
#### Bug Fixes
- `karmada-aggregated-apiserver`: Fixed the issue that can not proxy `exec` request to a proxy issue. ([#4021](https://github.com/karmada-io/karmada/pull/4021), @jwcesign)
- `karmada-operator`: Fixed the issue that karmada-metrics-adapter was not removed after deleting a Karmada instance. ([#4060](https://github.com/karmada-io/karmada/pull/4060), @wawa0210)
- `karmada-controller-manager`: Fix panic when FederatedHPA's SelectPolicy is nil and FederatedHPA webhook is disabled. ([#4085](https://github.com/karmada-io/karmada/pull/4085), @watermeion)
- `karmada-webhook`: Fixed to validate spec.types of MultiClusterService API. ([#4114](https://github.com/karmada-io/karmada/pull/4114), @lonelyCZ)

#### Others
None.

# v1.7.0
## Downloads for v1.7.0

Download v1.7.0 in the [v1.7.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.7.0).

## What's New

### CronFederatedHPA

Introduced `CronFederatedHPA` API, which represents a collection of repeating schedules to scale the replica number of a specific workload. It is used for regular auto scaling scenarios, and it can scale any workloads that have a scale subresource or FederatedHPA.

See [CronFederated HPA documents](https://karmada.io/docs/next/userguide/autoscaling/cronfederatedhpa/) for more details.

(Feature contributors: @jwcesign, @chaunceyjiang, @RainbowMango)

### MultiClusterService

Introduced `MultiClusterService` API, which controls the exposure of services to multiple external clusters and also enables service discovery between clusters.

See [Expose and discover multi-cluster services](https://github.com/karmada-io/karmada/blob/master/docs/proposals/networking/multiclusterservice.md) for more details.

(Feature contributors: @XiShanYongYe-Chang, @chaunceyjiang, @yike21)

### PropagationPolicy Preemption

The PropagationPolicy/ClusterPropagationPolicy is now able to preempt resources from another PropagationPolicy/ClusterPropagationPolicy as per priority by declaring the preemption behavior. Preemption will be disabled by default for backward compatibility.

This feature is now controlled by feature gates `PropagationPolicyPreemption` with the alpha state (disabled by default).
See [PropagationPolicy Priority and Preemption](https://github.com/karmada-io/karmada/tree/master/docs/proposals/scheduling/policy-preemption) for more details.

(Feature contributors: @Poor12, @whitewindmills, @jwcesign, @RainbowMango)

### Migrating resources in batches

Legacy cluster resources can now be migrated to Karmada in batches by PropagationPolicy or ClusterPropagationPolicy.
By specifying the `ConflictResolution` in the PropagationPolicy/ClusterPropagationPolicy, workloads can be migrated to Karmada smoothly without container termination or restarts.

See [Migrate In Batch](https://karmada.io/docs/next/administrator/migration/migrate-in-batch) for more details.

(Feature contributors: @chaosi-zju, @RainbowMango)

### FederatedHPA

FederatedHPA is now able to scale replicas based on `custom metrics` in addition to CPU and memory.

See [FederatedHPA scales with custom metrics](https://karmada.io/docs/next/tutorials/autoscaling-with-custom-metrics) for more details.

(Feature contributor: @chaunceyjiang)

## Other Notable Changes
### API Changes
- Added print columns for `FederatedHPA` API including reference, minpods, maxpods, and replicas. ([#3622](https://github.com/karmada-io/karmada/pull/3622), @Poor12)
- Added `CronFederatedHPA` API based on [the proposal](https://github.com/karmada-io/karmada/blob/master/docs/proposals/hpa/cronfederatedhpa.md). ([#3692](https://github.com/karmada-io/karmada/pull/3692), @RainbowMango)
- Introduced `Preemption` to both PropagationPolicy and ClusterPropagationPolicy to declare the behaviors of preemption. ([#3788](https://github.com/karmada-io/karmada/pull/3788), @RainbowMango)
- Introduced `ConflictResolution` to both PropagationPolicy and ClusterPropagationPolicy to declare how potential conflicts should be handled. ([#3801](https://github.com/karmada-io/karmada/pull/3810), @RainbowMango)
- Introduced a new field `Zones` to represent multiple zones of a member cluster. ([#3933](https://github.com/karmada-io/karmada/pull/3933), @RainbowMango)

### Deprecation
- `karmada-controller-manager`: Removed `hpa` controller in favor of `FederatedHPA`. ([#3852](https://github.com/karmada-io/karmada/pull/3852), @jwcesign)
- The `Zone`(.spec.zone) was never used and now has been deprecated in favor of the newly introduced `Zones`. ([#3933](https://github.com/karmada-io/karmada/pull/3933), @RainbowMango)
- `karmadactl`:  Deprecated `--cluster-zone` flag which will be removed in future releases and introduced `--cluster-zones` flag. ([#3995](https://github.com/karmada-io/karmada/pull/3995), @whitewindmills)

### Bug Fixes
- `karmada-metrics-adapter`: Fixed the issue that when different clusters have the same pod name, the annotations of different metrics have the same value. ([#3647](https://github.com/karmada-io/karmada/pull/3647), @chaunceyjiang)
- `karmada-controller-manager`: Fixed panic when printing log in the case that lastSuccessfulTime of cronjob is nil. ([#3683](https://github.com/karmada-io/karmada/pull/3683), @chaunceyjiang)
- `karmada-controller-manager`: Fixed the issue that the `Applied` condition of ResourceBinding is always true. ([#3709](https://github.com/karmada-io/karmada/pull/3709), @chaunceyjiang)
- `karmada-controller-manager`: Fixed a boundary case where the `propagationpolicy.karmada.io/name` in the label of the resource template would not be removed after deleting PP. ([#3848](https://github.com/karmada-io/karmada/pull/3848), @chaunceyjiang)
- `karmada-controller-manager`: Fixed the issue that dependent resources are created and deleted repeatedly when the dependent resource has a status field. ([#3868](https://github.com/karmada-io/karmada/pull/3868), @chaunceyjiang)
- `karmada-controller-manager`: Avoid updating directly cached resource templates. ([#3879](https://github.com/karmada-io/karmada/pull/3879), @whitewindmills)
- `kamrada-controller-manager`: Fixed the issue that federated-HPA plain metric calculation is incorrect when usageRatio == 1.0, keep same with resource replicas. ([#3922](https://github.com/karmada-io/karmada/pull/3922), @zach593)
- `karmada-webhook`: When application failover is enabled, users are prevented from setting propagateDeps to `false`. ([#3739](https://github.com/karmada-io/karmada/pull/3739), @chaunceyjiang)
- `karmada-search`: Fixed a panic due to concurrent mutating objects in the informer cache. ([#3966](https://github.com/karmada-io/karmada/pull/3966), @ikaven1024)
- Fixed the inability to sync list issues in the case that the client disconnects from the member cluster. The fixes apply to the following components:
  * karmada-controller-manager
  * karmada-agent
  * karmada-descheduler
  * karmada-scheduler
  * karmada-metrics-adapter
  * karmada-search
    ([#3908](https://github.com/karmada-io/karmada/pull/3908), @WulixuanS)

### Security
- The base image `alpine` now has been promoted from `alpine:3.18.2` to `alpine:3.18.3`. ([#3942](https://github.com/karmada-io/karmada/pull/3942), @Rajan-226)
- Security: Sets an upper bound for all components on the lifetime of idle keep-alive connections and time to read the headers of incoming requests. ([#3951](https://github.com/karmada-io/karmada/pull/3951), @zishen)

### Features & Enhancements
- `karmadactl`: The `--wait-component-ready-timeout` flag has been introduced in the `init` command to specify the component installation timeout. ([3665](https://github.com/karmada-io/karmada/pull/3665), @helen-frank)
- `karmadactl`: Added `karmada-metrics-adapter` to addons. ([#3717](`karmadactl`: Added `karmada-metrics-adapter` to addons.), @chaunceyjiang)
- `karmadactl`: Introduced `top` command. ([#3593](https://github.com/karmada-io/karmada/pull/3593), @chaunceyjiang)
- `karmadactl join/register`, `karmada-agent`: Added labels on the namespace created by Karmada. ([#3839](https://github.com/karmada-io/karmada/pull/3839), @zhy76)
- `karmadactl`: Granted full permissions of Karmada resources to `admin` during deployment of Karmada with `init`. ([#3937](https://github.com/karmada-io/karmada/pull/3937), @zhy76)
- `karmada-controller-manager`: Supported aggregating the status of a pod's initContainer. ([#3574](https://github.com/karmada-io/karmada/pull/3574), @chaunceyjiang)
- `karmada-controller-manager`: Supported modification synchronization of custom resources as dependencies. ([#3614](https://github.com/karmada-io/karmada/pull/3614), @chaunceyjiang)
- `karmada-controller-manager`: Implemented proxy header of cluster APIs. ([#3631](https://github.com/karmada-io/karmada/pull/3631), @Poor12)
- `karmada-controller-manager`: The `--cluster-cache-sync-timeout` flag is now used to specify the sync timeout of the control plane cache in addition to the member cluster's cache. The default value has been increased to 2 minutes. ([#3874](https://github.com/karmada-io/karmada/pull/3874), @lxtywypc)
- `karmada-controller-manger`: Added Ingresses to the default dependencinterpreter. ([#3885](https://github.com/karmada-io/karmada/pull/3885), @chaunceyjiang)
- `karmada-controller-manager`: Introduced a LabelSelector field to DependentObjectReference. ([#3811](https://github.com/karmada-io/karmada/pull/3811), @chaunceyjiang)
- `karmada-controller-manager`: Added the pod replica interpreter by default. ([#3876](https://github.com/karmada-io/karmada/pull/3876), @whitewindmills)
- `karmada-controller-manager`: Added dependencies of ServiceImport to the default interpreter. ([#3939](https://github.com/karmada-io/karmada/pull/3939), @chaunceyjiang)
- `karmada-controller-manager`: Included the UID of the owner resource in labels and provided the details in annotations. If users are using related labels as label selectors, they should switch to using the UID as the label selector. ([#4007](https://github.com/karmada-io/karmada/pull/4007), @jwcesign)
- `karmada-scheduler`: Introduced new scheduling condition reasons: NoClusterFit, SchedulerError, Unschedulable, Success. ([#3741](https://github.com/karmada-io/karmada/pull/3741), @whitewindmills)
- `karmada-operator`: Supported disabling karmada cascading deletion. ([#3577](https://github.com/karmada-io/karmada/pull/3577), @calvin0327)
- `karmada-operator`: Allowed installing the `karmada-metrics-adapter` addon. ([#3732](https://github.com/karmada-io/karmada/pull/3732), @calvin0327)
- `karmada-webhook`: Allowed custom metrics configuration of FederatedHPA. ([#3826](https://github.com/karmada-io/karmada/pull/3826), @jwcesign)

## Other
### Dependencies
- The base image `alpine` now has been promoted from `alpine:3.17.1` to `alpine:3.18.2`. ([#3671](https://github.com/karmada-io/karmada/pull/3671), @yanggangtony)
- Karmada is now built with Kubernetes v1.27.3 dependencies. ([#3730](https://github.com/karmada-io/karmada/pull/3730), @RainbowMango)
- Karmada(v1.7) is now built with Go 1.20.6. ([#3865](https://github.com/karmada-io/karmada/pull/3865), @parthn2)

### Helm Charts
- Fixed the issue that `karmada-search` no ETCD secret volume mount when using external ETCD. ([#3777](https://github.com/karmada-io/karmada/pull/3777), @my-git9)
- Granted full permissions of Karmada resources to `admin`. ([#3957](https://github.com/karmada-io/karmada/pull/3957), @Vacant2333)
- Now able to render `PodDisruptionBudget` for resources. ([#3955](https://github.com/karmada-io/karmada/pull/3955), @a7i)

### Instrumentation
- Karmada images now are signed with cosign. ([3434](https://github.com/karmada-io/karmada/pull/3434), @liangyuanpeng)
- Removed specific labels from the following metrics of `karmada-controller-manager` to reduce the metrics count:
  * resource_match_policy_duration_seconds: removed `apiVersion`/`kind`/`name`/`namespace`.
  * resource_apply_policy_duration_seconds: removed `apiVersion`/`kind`/`name`/`namespace`.
  * policy_apply_attempts_total: removed `namespace`/`name`.
  * binding_sync_work_duration_seconds: removed `namespace`/`name`.
  * work_sync_workload_duration_seconds: removed `namespace`/`name`.
    ([#3795](https://github.com/karmada-io/karmada/pull/3795), @jwcesign)
- Introduced the `TaintClusterSucceed` event to `Cluster` object and merged `TaintClusterByConditionFailed` and `RemoveTargetClusterFailed` to `TaintClusterFailed`. ([#2736](https://github.com/karmada-io/karmada/pull/2736), @Poor12)
- Introduced `cronfederatedhpa_process_duration_seconds` and `cronfederatedhpa_rule_process_duration_seconds` metrics for CronFederatedHPA. ([#3979](https://github.com/karmada-io/karmada/pull/3979), @whitewindmills)
- Introduced `federatedhpa_process_duration_seconds` and `federatedhpa_pull_metrics_duration_seconds` for FederatedHPA, which will be emitted by `karmada-controller-manager`. ([#3972](https://github.com/karmada-io/karmada/pull/3972), @zhy76)
