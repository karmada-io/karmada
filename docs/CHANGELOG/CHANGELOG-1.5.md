<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.5.4](#v154)
  - [Downloads for v1.5.4](#downloads-for-v154)
  - [Changelog since v1.5.3](#changelog-since-v153)
    - [Changes by Kind](#changes-by-kind)
    - [Bug Fixes](#bug-fixes)
    - [Others](#others)
- [v1.5.3](#v153)
  - [Downloads for v1.5.3](#downloads-for-v153)
  - [Changelog since v1.5.2](#changelog-since-v152)
    - [Changes by Kind](#changes-by-kind-1)
      - [Bug Fixes](#bug-fixes-1)
      - [Others](#others-1)
- [v1.5.2](#v152)
  - [Downloads for v1.5.2](#downloads-for-v152)
  - [Changelog since v1.5.1](#changelog-since-v151)
    - [Changes by Kind](#changes-by-kind-2)
      - [Bug Fixes](#bug-fixes-2)
      - [Others](#others-2)
- [v1.5.1](#v151)
  - [Downloads for v1.5.1](#downloads-for-v151)
  - [Changelog since v1.5.0](#changelog-since-v150)
    - [Changes by Kind](#changes-by-kind-3)
      - [Bug Fixes](#bug-fixes-3)
      - [Others](#others-3)
- [v1.5.0](#v150)
  - [Downloads for v1.5.0](#downloads-for-v150)
  - [What's New](#whats-new)
    - [Multiple Scheduling Groups](#multiple-scheduling-groups)
    - [New Way to Customize Scheduler](#new-way-to-customize-scheduler)
  - [Other Notable Changes](#other-notable-changes)
    - [API Changes](#api-changes)
    - [Bug Fixes](#bug-fixes-4)
    - [Security](#security)
    - [Features & Enhancements](#features--enhancements)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Chart](#helm-chart)
    - [Instrumentation](#instrumentation)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.5.4
## Downloads for v1.5.4

Download v1.5.4 in the [v1.5.4 release page](https://github.com/karmada-io/karmada/releases/tag/v1.5.4).

## Changelog since v1.5.3
### Changes by Kind
### Bug Fixes
- `karmada-search`: Fixed a panic due to concurrent mutating objects in the informer cache. ([#3977](https://github.com/karmada-io/karmada/pull/3977), @chaosi-zju)
- `karmada-controller-manager`: Avoid updating directly cached resource templates. ([#3894](https://github.com/karmada-io/karmada/pull/3894), @whitewindmills)

### Others
- Bump k8s.io dependencies to v0.26.4 to fix a possible panic. ([#3930](https://github.com/karmada-io/karmada/pull/3930), @liangyuanpeng)

# v1.5.3
## Downloads for v1.5.3

Download v1.5.3 in the [v1.5.3 release page](https://github.com/karmada-io/karmada/releases/tag/v1.5.3).

## Changelog since v1.5.2
### Changes by Kind
#### Bug Fixes
- Chart: Fixed the issue that `karmada-search` no ETCD secret volume mount when using external ETCD. (#3785, @my-git9)

#### Others
None.

# v1.5.2
## Downloads for v1.5.2

Download v1.5.2 in the [v1.5.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.5.2).

## Changelog since v1.5.1
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed `Applied` condition of ResourceBinding is always true issue. (#3723, @jwcesign)
- `karmada-controller-manager`: Fixed the panic issue in case of the grade number of resourceModel is less than the number of resources. (#3610, @sunbinnnnn)
- `karmada-scheduler`: Fixed the issue that empty deployment can still be propagated to member clusters even when `--enableEmptyWorkloadPropagation` flag is false. (#3641, @chaunceyjiang)

#### Others
None.

# v1.5.1
## Downloads for v1.5.1

Download v1.5.1 in the [v1.5.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.5.1).

## Changelog since v1.5.0
### Changes by Kind
#### Bug Fixes
- `karmada-search`: Fixed the problem that ResourceVersion base64 encrypted repeatedly when starting multiple informers to watch resource. (#3387, @niuyueyang1996)
- `karmada-search`: Fixed paging list in karmada search proxy in large-scale member clusters issue. (#3449, @ikaven1024)
- `karmada-search`: Fixed contecnt-type header issue in HTTP response. (#3513, @callmeoldprince)
- `karmada-controller-mamager`: Fixed Lua's built-in string function can not be used issue in ResourceInterpreterCustomization. (#3282, @chaunceyjiang)
- `karmada-controller-manager`: Fixed the control plane endpointslices cannot be deleted issue. (#3354, @wenchezhao)
- `karmada-controller-manager`: Fixed a corner case that when there are tasks in the GracefulEvictionTasks queue, graceful-eviction-controller will not work after restarting karmada-controller-manager. (#3490, @chaunceyjiang)
- `karmada-scheduler`: Fixed unexpected re-scheduling due to mutating informer cache issue. (#3428, @whitewindmills)
- `karmada-scheduler`: Fixed the issue of inconsistent Generation and SchedulerObservedGeneration. (#3477, @Poor12)
- `karmadactl`: Fixed unable to view the options of `karmadactl addons enable/disable` issue. (#3305, @lonelyCZ)

#### Others
None.

# v1.5.0
## Downloads for v1.5.0

Download v1.5.0 in the [v1.5.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.5.0).

## What's New
### Multiple Scheduling Groups

Users are now able to declare multiple groups of clusters to both `PropagationPolicy` and `ClusterPropagationPolicy` by leveraging the newly introduced `ClusterAffinities` field. The scheduler will evaluate these groups one by one in the order they appear in the specification until it finds the one that satisfies scheduling restrictions.

This feature allows the Karmada scheduler to first schedule applications to lower-cost clusters or migrate applications from a primary cluster to backup clusters in the case of cluster failure.

See [multiple scheduling group proposal](https://github.com/karmada-io/karmada/tree/master/docs/proposals/scheduling/multi-scheduling-group) for more info.

(Feature contributor: @XiShanYongYe-Chang @RainbowMango)

### New Way to Customize Scheduler

The default scheduler is now able to work with any number of third-party customized schedulers. Similar to Kubernetes, the workloads will be scheduled by the default scheduler if the scheduler name is not declared in `PropagationPolicy` or `ClusterPropagationPolicy`.

See [customize scheduler](https://karmada.io/docs/next/developers/customize-karmada-scheduler) for more details.

(Feature contributor: @Poor12)

## Other Notable Changes
### API Changes
- Introduced `Affinities` to both `PropagationPolicy` and `ClusterPropagationPolicy`. ([#3105](https://github.com/karmada-io/karmada/pull/3105), @RainbowMango)
- Introduced `Placement` to the `ResoureBinding`/`ClusterResourceBinding` API. ([#2702](https://github.com/karmada-io/karmada/pull/2702), @Poor12)
- Introduced `SchedulerObservedAffinityName` to both `ResourceBinding` and `ClusterResourceBinding`. ([#3163](https://github.com/karmada-io/karmada/pull/3163), @RainbowMango)


### Bug Fixes
- `karmadactl`: Fixed the issue that `karmada-agent` installed by the `register` command cannot delete works due to lack of permissions. ([#2902](https://github.com/karmada-io/karmada/pull/2902), @lonelyCZ)
- `karmadactl`: Fixed the issue that the default ValidatingWebhookConfiguration for `resourceinterpreterwebhook` was not working. ([#2915](https://github.com/karmada-io/karmada/pull/2915), @whitewindmills)
- `karmadactl`: Fixed the issue that the default ValidatingWebhookConfiguration for `resourceinterpretercustomizations` was not working. ([#2916](https://github.com/karmada-io/karmada/pull/2916), @chaunceyjiang)
- `karmadactl`: Fixed the error of resources whose name contains colons failed to be created when using `karmadactl apply`. ([#2919](https://github.com/karmada-io/karmada/pull/2919), @Poor12)
- `karmadactl init`: Granted karmada-agent permissions to access resourceinterpretercustomizations. ([#2984](https://github.com/karmada-io/karmada/pull/2984), @jwcesign)
- `karmada-controller-manager`: Generated PolicyRules from given subjects for impersonation deduplicate. ([#2911](https://github.com/karmada-io/karmada/pull/2911), @yanfeng1992)
- `karmada-controller-manager`/`karmada-agent`: Fixed the failure to sync work status due to the informer being accidentally shut down. ([#2930](https://github.com/karmada-io/karmada/pull/2930), @Poor12)
- `karmada-controller-manager`/`karmada-agent`: Fixed misjudgment of deployment and statefuleset health status. ([#2928](https://github.com/karmada-io/karmada/pull/2928), @Fish-pro)
- `karmada-controller-manager`: Fixed `LabelsOverrider` and `AnnotationsOverrider` failures to add new items in the case of null `label`/`annotation`. ([#2971](https://github.com/karmada-io/karmada/pull/2971), @chaunceyjiang)
- `karmada-controller-manager`: `labelsOverrider/annotationsOverrider` supports `composed-labels`, like testannotation/projectId: <label-value>. ([#3037](https://github.com/karmada-io/karmada/pull/3037), @chaunceyjiang)
- `karmada-controller-manager`: Fixed the issue that RBAC resources whose name contains uppercase characters cannot be propagated. ([#3201](https://github.com/karmada-io/karmada/pull/3201), @whitewindmills)
- `karmada-agent`: Check whether the resource exists before creating it. Sometimes the resource is created in advance, giving less privilege to Karmada. ([#2988](https://github.com/karmada-io/karmada/pull/2988), @jwcesign)
- `karmada-scheduler`: Fixed a corner case that re-scheduling was skipped in the case that the cluster becomes not fit. ([#2912](https://github.com/karmada-io/karmada/pull/2912), @jwcesign)
- karmada-search: Filtered out not-ready clusters. ([#3010](https://github.com/karmada-io/karmada/pull/3010), @yanfeng1992)
- `karmada-search`: Avoided proxy request block when member clusters were down. ([#3027](https://github.com/karmada-io/karmada/pull/3027), @ikaven1024)
- `karmada-webhook`: Validated replicaSchedulingType and replicaDivisionPreference. ([#3014](https://github.com/karmada-io/karmada/pull/3014), @chaunceyjiang)
- `karmada-webhook`: Fixed the issue that the InterpretDependency operation cannot be registered. ([#3052](https://github.com/karmada-io/karmada/pull/3052), @whitewindmills)
- `karmada-search`: Supported pod subresource (attach, exec, port-forward) through global proxy. ([#3098](https://github.com/karmada-io/karmada/pull/3098), @ikaven1024)


### Security
- golang.org/x/net updates to v0.6.0 to fix CVE-2022-41717. ([#3048](https://github.com/karmada-io/karmada/pull/3048), @fengshunli)

### Features & Enhancements
- `karmadactl`: Introduced `--kube-image-tag` flag to the `init` command to specify the Kubernetes image version. ([#2840](https://github.com/karmada-io/karmada/pull/2840), @helen-frank)
- `karmadactl`: The `--cluster-context` flag of `join` command now takes `current-context` by default. ([#2956](https://github.com/karmada-io/karmada/pull/2956), @helen-frank)
- `karmadactl`: Added edit mode for interpret commands. ([#2831](https://github.com/karmada-io/karmada/pull/2831), @ikaven1024)
- karmadactl: Introduced `--cert-validity-period` for `init` to make the validity period of cert configurable. ([#3156](https://github.com/karmada-io/karmada/pull/3156), @lonelyCZ)
- `karmada-controller-manager`: Now the `OverridePolicy` and `ClusterOverridePolicy` will be applied by implicit priority order. The one with the lower priority will be applied before the one with the higher priority. ([#2609](https://github.com/karmada-io/karmada/pull/2609))
- `karmada-controller-manager`: Users are now able to apply multiple dependencies interpreter configurations. ([#2884](https://github.com/karmada-io/karmada/pull/2884), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Built-in interpreter supports StatefulSets. ([#3009](https://github.com/karmada-io/karmada/pull/3009), @chaunceyjiang)
- `karmada-controller-manager`:  Retained the labels added to resources by member clusters. ([#3088](https://github.com/karmada-io/karmada/pull/3088), @chaunceyjiang)
- `karmada-controller-manager`: Default interpreter supports CronJob aggregated status. ([#3129](https://github.com/karmada-io/karmada/pull/3129), @chaunceyjiang)
- `karmada-controller-manager`: Supports PodDisruptionBudget resource in default interpreter. ([#2997](https://github.com/karmada-io/karmada/pull/2997), @a7i)
- `karmada-controller-manager`: Support for removing annotations/labels propagated through karmada. ([#3099](https://github.com/karmada-io/karmada/pull/3099), @chaunceyjiang)
- `karmada-webhook`: Added validation for policy.spec.placement.orderedClusterAffinities. (#3164, @XiShanYongYe-Chang)
- `karmada-webhook`: Validated the fieldSelector of overridepolicy. ([#3193](https://github.com/karmada-io/karmada/pull/3193), @chaunceyjiang)

## Other
### Dependencies
- Kubernetes images will now be pulled from registry.k8s.io instead of k8s.gcr.io, to be in alignment with current community initiatives. ([#2882](https://github.com/karmada-io/karmada/pull/2882), @Zhuzhenghao)
- Karmada is now built with Golang 1.19.4. ([#2908](https://github.com/karmada-io/karmada/pull/2908), @qingwave)
- Karmada is now built with Golang 1.19.5. ([#3067](https://github.com/karmada-io/karmada/pull/3067), @yanggangtony)
- Karmada is now built with Kubernetes v1.26.1 dependencies. ([#3080](https://github.com/karmada-io/karmada/pull/3080), @RainbowMango)
- The base image `alpine` now has been promoted from `alpine:3.15.1` to `alpine:3.17.1`. ([#3045](https://github.com/karmada-io/karmada/pull/3045), @fengshunli)

### Helm Chart
- Fixed helm template missing yaml directive marker to separate API Service resources. ([#2963](https://github.com/karmada-io/karmada/pull/2963), @a7i)
- Fixed missing karmada-search helm template strategy. ([#2994](https://github.com/karmada-io/karmada/pull/2994), @a7i)
- Fixed karmada-agent helm template strategy indentation. ([#2993](https://github.com/karmada-io/karmada/pull/2993), @a7i)
- Chart: karmada-search installation supports specifying an external etcd. ([#3120](https://github.com/karmada-io/karmada/pull/3120), @my-git9)
- Chart: Supports custom labels variable for etcd. ([#3138](https://github.com/karmada-io/karmada/pull/3138), @my-git9)

### Instrumentation
- `Instrumentation`: Introduced the `pool_get_operation_total`, `pool_put_operation_total` metrics to `karmada-controller-manager` and `karmada-agent`. ([#2883](https://github.com/karmada-io/karmada/pull/2883), @ikaven1024)
