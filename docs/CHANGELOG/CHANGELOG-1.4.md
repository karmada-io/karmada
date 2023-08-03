<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.4.4](#v144)
  - [Downloads for v1.4.4](#downloads-for-v144)
  - [Changelog since v1.4.3](#changelog-since-v143)
    - [Changes by Kind](#changes-by-kind)
      - [Bug Fixes](#bug-fixes)
      - [Others](#others)
- [v1.4.3](#v143)
  - [Downloads for v1.4.3](#downloads-for-v143)
  - [Changelog since v1.4.2](#changelog-since-v142)
    - [Changes by Kind](#changes-by-kind-1)
      - [Bug Fixes](#bug-fixes-1)
      - [Others](#others-1)
- [v1.4.2](#v142)
  - [Downloads for v1.4.2](#downloads-for-v142)
  - [Changelog since v1.4.1](#changelog-since-v141)
    - [Changes by Kind](#changes-by-kind-2)
      - [Bug Fixes](#bug-fixes-2)
      - [Others](#others-2)
- [v1.4.1](#v141)
  - [Downloads for v1.4.1](#downloads-for-v141)
  - [Changelog since v1.4.0](#changelog-since-v140)
    - [Changes by Kind](#changes-by-kind-3)
      - [Bug Fixes](#bug-fixes-3)
      - [Others](#others-3)
- [v1.4.0](#v140)
  - [Downloads for v1.4.0](#downloads-for-v140)
  - [Karmada v1.4 Release Notes](#karmada-v14-release-notes)
    - [What's New](#whats-new)
      - [#Declarative Resource Interpreter](#declarative-resource-interpreter)
      - [PropagationPolicy/ClusterPropagationPolicy priority](#propagationpolicyclusterpropagationpolicy-priority)
      - [Instrumentation improvement](#instrumentation-improvement)
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

* [v1.4.0](#v140)
    * [Downloads for v1.4.0](#downloads-for-v140)
    * [Karmada v1.4 Release Notes](#karmada-v14-release-notes)
        * [1.4 What's New](#whats-new)
        * [Other Notable Changes](#other-notable-changes)
        * [Other](#other)

# v1.4.4
## Downloads for v1.4.4

Download v1.4.4 in the [v1.4.4 release page](https://github.com/karmada-io/karmada/releases/tag/v1.4.4).

## Changelog since v1.4.3
### Changes by Kind
#### Bug Fixes
- `karmada-scheduler`: Fixed the issue that empty deployment can still be propagated to member clusters even when `--enableEmptyWorkloadPropagation` flag is false. (#3642, @chaunceyjiang)
- `karmada-controller-manager`: Fixed the panic issue in case of the grade number of resourceModel is less than the number of resources. (#3608, @sunbinnnnn)

#### Others
None.

# v1.4.3
## Downloads for v1.4.3

Download v1.4.3 in the [v1.4.3 release page](https://github.com/karmada-io/karmada/releases/tag/v1.4.3).

## Changelog since v1.4.2
### Changes by Kind
#### Bug Fixes
- `karmada-search`: support pod subresource (attach, exec, port-forward) through global proxy. (#3100, @ikaven1024)
- `karmada-search`: Fixed the problem that ResourceVersion base64 encrypted repeatedly when starting multiple informers to watch resource. (#3388, @niuyueyang1996)
- `karmada-search`: Fixed paging list in karmada search proxy in large-scale member clusters issue. (#3450, @ikaven1024)
- `karmada-search`: Fixed contecnt-type header issue in HTTP response. (#3514, @callmeoldprince)
- `karmada-controller-manager`: Fixed the issue that RBAC resources whose name contains uppercase characters can not be propagated. (#3215, @whitewindmills)
- `karmada-controller-mamager`: Fixed Lua's built-in string function can not be used issue in ResourceInterpreterCustomization. (#3301, @chaunceyjiang)
- `karmada-controller-manager`: Fixed the control plane endpointslices cannot be deleted issue. (#3353, @wenchezhao)
- `karmadactl`: Fixed unable to view the options of `karmadactl addons enable/disable` issue. (#3306, @lonelyCZ)

#### Others
None.

# v1.4.2
## Downloads for v1.4.2

Download v1.4.2 in the [v1.4.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.4.2).

## Changelog since v1.4.1
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed `LabelsOverrider` and `AnnotationsOverrider` failed to add new items issue in case of `label`/`annotation` is nil. (#2972, @chaunceyjiang)
- `karmada-controller-manager`: `labelsOverrider/annotationsOverrider` supports `composed-labels`, like testannotation/projectId: <label-value>. (#3047, @chaunceyjiang)
- `karmadactl`: Grant karmada-agent permission to access resourceinterpretercustomizations for `init` command. (#2986, @jwcesign)
- `karmada-agent`: Check if the resource exists before creating it. Sometimes the resource is created in advance, to give less privilege to Karmada. (#3002, @jwcesign)
- `karmada-search`: filter out not ready clusters. (#3016, @yanfeng1992)
- `karmada-search`: avoid proxy request block when member cluster down. (#3030, @ikaven1024)
- `karmada-webhook`: Fixed the issue that the InterpretDependency operation can't be registered. (#3074, @XiShanYongYe-Chang)

#### Others
None.

# v1.4.1
## Downloads for v1.4.1

Download v1.4.1 in the [v1.4.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.4.1).

## Changelog since v1.4.0
### Changes by Kind
#### Bug Fixes
- `karmadactl`: Fixed `karmada-agent` installed by the `register` command can not delete works due to lack of permission issue. (#2904, @lonelyCZ)
- `karmadactl`: Fixed the default ValidatingWebhookConfiguration for `resourceinterpreterwebhook` not working issue. (#2924, @qingwave)
- `karmadactl`: Fixed the default ValidatingWebhookConfiguration for `resourceinterpretercustomizations` not working issue. (#2927, @chaunceyjiang)
- `karmadactl`: Fixed the error of resources whose name contains colons failing to be created when using `karmadactl apply`. (#2931, @Poor12)
- `karmada-controller-manager`/`karmada-agent`: Fixed misjudgment of deployment and statefuleset health status. (#2944, @Fish-pro)
- `karmada-controller-manager`/`karmada-agent`: Fixed failed to sync work status issue due to the informer being accidentally shut down. (#2937, @Poor12)
- `karmada-scheduler`: Fixed a corner case that re-schedule be skipped in case of the cluster becomes not fit. (#2955, @jwcesign)

#### Others
- Karmada is now built with Golang 1.19.4. (#2913, @qingwave)

# v1.4.0
## Downloads for v1.4.0

Download v1.4.0 in the [v1.4.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.4.0).

## Karmada v1.4 Release Notes
### What's New
#### #Declarative Resource Interpreter

The [Interpreter Framework](https://karmada.io/docs/next/userguide/globalview/customizing-resource-interpreter/) is designed for interpreting the structure of arbitrary resource types. It consists of `built-in` and `customized` interpreters, this release introduced another brand-new customized interpreter.

With the newly introduced `declarative` [interpreter](https://karmada.io/docs/userguide/globalview/customizing-resource-interpreter), users can quickly customize resource interpreters for both Kubernetes resources and CRD resources by the rules declared in the `ResourceInterpreterCustomization` API specification. Compared with the interpreter customized by the webhook, it gets the rules from the declarative specifications instead of requiring an additional webhook component.

The new command named `interpret` in the `karmadactl` could be used to test the rules before applying them to the system.
Some examples are provided to help users better understand how this interpreter can be used.

(Feature contributor: @jameszhangyukun @ikaven1024 @chaunceyjiang @XiShanYongYe-Chang @RainbowMango)

#### PropagationPolicy/ClusterPropagationPolicy priority
Users are now able to declare the priority for both `PropagationPolicy` and `ClusterPropagationPolicy`, a policy will be applied for the matched resource templates if there are no other policies with higher priority at the point of the resource template be processed.

The priority could be used by the system administrator to manage and control the policies. Refer to [Configure PropagationPolicy priority](https://karmada.io/docs/userguide/scheduling/resource-propagating#configure-propagationpolicy-priority) for more details.

(Feature contributor: @Garrybest @jwcesign)

#### Instrumentation improvement

This release enhanced observability significantly through metrics and events.
The metrics can be queried by the endpoint(`/metrics`) of each component using an HTTP scrap, and they are served in Prometheus format. And the events are reported to the relevant resource objects respectively.

Refer to [events](https://karmada.io/docs/reference/instrumentation/event) and [metrics](https://karmada.io/docs/reference/instrumentation/metrics) for more details.

(Feature contributor: @Poor12)

### Other Notable Changes

#### API Changes

- Introduce priority to PropagationPolicy. ([#2758](https://github.com/karmada-io/karmada/pull/2758), @RainbowMango)
- Introduced `LabelsOverrider` and `AnnotationsOverrider` for overriding labels and annotations.([#2584](https://github.com/karmada-io/karmada/pull/2584), @chaunceyjiang)
- evolute PropagateDeps FeatureGate to Beta and enable it by default. ([#2875](https://github.com/karmada-io/karmada/pull/2875), @XiShanYongYe-Chang)
- evolute Failover/GracefulEviction FeatureGate to Beta and enable it by default. ([#2876](https://github.com/karmada-io/karmada/pull/2876), @jwcesign)
- evolute CustomizedClusterResourceModeling FeatureGate to Beta and enable it by default. ([#2877](https://github.com/karmada-io/karmada/pull/2877), @Poor12)

#### Bug Fixes
- `karmada-search`: fix concurrent map writes panic while list objects via proxy. ([#2483](https://github.com/karmada-io/karmada/pull/2483), @ikaven1024)
- `karmada-search`: Fixed returned ResourceVersion by proxy not stable issue. ([#2746](https://github.com/karmada-io/karmada/pull/2746), @cmicat)
- `karmada-controller-manager`/`karmada-agent`: Fixed pod information can not be collected issue when building resource summary. ([#2489](https://github.com/karmada-io/karmada/pull/2489), @Poor12)
- `karmada-controller-manager`: use cluster secret ref namespace in unified-auth-controller when generate ClusterRoleBinding. ([#2516](https://github.com/karmada-io/karmada/pull/2516), @XiShanYongYe-Chang)
- `karmada-controller-manager`: fix the error of resources whose name contains colons failing to be created. ([#2549](https://github.com/karmada-io/karmada/pull/2549), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the panic when cluster ImpersonatorSecretRef is nil.([#2675](https://github.com/karmada-io/karmada/pull/2675), @stingshen)
- `karmada-controller-manager`: Fix serviceaccount continual regeneration by service account controller.([#2578](https://github.com/karmada-io/karmada/pull/2578), @Poor12)
- `karmada-controller-manager`: Disable the preemption matching of pp/cpp priority. ([#2734](https://github.com/karmada-io/karmada/pull/2734), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fix clusterOverridePolicy and overridePolicy with nil resource selector could not work. ([#2771](https://github.com/karmada-io/karmada/pull/2771), @wuyingjun-lucky)
- `karmada-controlle-managerr`: ignore resource that do not match with policy before apply policy. ([#2786](https://github.com/karmada-io/karmada/pull/2786), @XiShanYongYe-Chang)
- `karmada-agent`: Fixed `ServiceExport` controller can not report `endpointSlices` issue(due to miss `create` permission). ([#2515](https://github.com/karmada-io/karmada/pull/2515), @lonelyCZ)
- `karmadactl`: Fixed `init` can not honor IPV6 address issue when generating kubeconfig file. ([#2450](https://github.com/karmada-io/karmada/pull/2450), @duanmengkk)
- `karmadactl`: Fixed `--karmada-data` directory not initialization issue in `init` command. ([#2548](https://github.com/karmada-io/karmada/pull/2548), @jwcesign)
- `karmadactl`: Fixed `init` commands print incorrect register command issue. ([#2707](https://github.com/karmada-io/karmada/pull/2707), @Songjoy)
- `karmadactl`: Fix namespace already exists. ([#2505](https://github.com/karmada-io/karmada/pull/2505), @cleverhu)
- `karmada-webhook`: Fixed failed to set resource selector default namespace if the relevant OverridePolicy and PropagationPolicy with namespace unset issue. ([#2858](https://github.com/karmada-io/karmada/pull/2858), @carlory)

#### Security
- `Security`: Add limitReader to `io.ReadAll` which could limit the memory request and avoid DoS attacks. ([#2765](https://github.com/karmada-io/karmada/pull/2765), @Poor12)

#### Features & Enhancements
- `karmadactl`: Improve karmada init help output. ([#2342](https://github.com/karmada-io/karmada/pull/2342), @my-git9)
- `karmadactl`: karmadactl prohibit input extra arguments for `init` command. ([#2497](https://github.com/karmada-io/karmada/pull/2497), @helen-frank)
- `karmadactl`/`chart`: The `init` no longer creates redundant ServiceAccounts for components except `karmada-agent`. ([#2523](https://github.com/karmada-io/karmada/pull/2523), @carlory)
- `karmadactl`: Fixed options of `deinit` can not be shown issue. ([#2540](https://github.com/karmada-io/karmada/pull/2540), @helen-frank)
- `karmadactl/chart`: if karmada is installed by using karmadactl or helm chart with default configuration, the image version of karmada-kube-controller-manager/karmada-apiserver will be kube-controller-manager:v1.25.2/kube-apiserver:v1.25.2. ([#2539](https://github.com/karmada-io/karmada/pull/2539), @jwcesign)
- `karmadactl`: Introduced `--karmada-apiserver-advertise-address` flag to specify Karmada APIserver's address to the `init` sub-command. ([#2550](https://github.com/karmada-io/karmada/pull/2550), @wuyingjun-lucky)
- `karmadactl`: introduce --enable-cert-rotation option to register command.([#2596](https://github.com/karmada-io/karmada/pull/2596), @lonelyCZ)
- `karmadactl`: uncordon add dryrun ([#2760](https://github.com/karmada-io/karmada/pull/2760), @helen-frank)
- `karmadactl`: karmadactl get add validate cluster exist ([#2787](https://github.com/karmada-io/karmada/pull/2787), @helen-frank)
- `karmadactl`: add liveness probe into the kube-controller-manager component ([#2817](https://github.com/karmada-io/karmada/pull/2817), @carlory)
- `karmadactl`: `init` add `--image-registry` flags ([#2655](https://github.com/karmada-io/karmada/pull/2655), @helen-frank)
- `karmadactl`: add interpreter command for resource interpret customizations. ([#2750](https://github.com/karmada-io/karmada/pull/2750), @ikaven1024)
- `karmadactl`: add execute mod for interpret command. ([#2824](https://github.com/karmada-io/karmada/pull/2824), @ikaven1024)
- `karmada-search`: objects returned by proxy will have `resource.karmada.io/cached-from-cluster` annotation to indicate which member cluster from.  ([#2469](https://github.com/karmada-io/karmada/pull/2469), @ikaven1024)
- `karmada-search`: users can get the real resource request metrics while using the proxy. ([#2481](https://github.com/karmada-io/karmada/pull/2481), @ikaven1024)
- `karmada-search`: users now can use `--disable-search` and `--disable-proxy` options to disable search and proxy feature (default both are enabled).([#2650](https://github.com/karmada-io/karmada/pull/2650), @ikaven1024)
- `karmada-controller-manager`: add implicit priority for PropagationPolicy. ([#2267](https://github.com/karmada-io/karmada/pull/2267), @Garrybest)
- `karmada-controller-manager`: Introduced resource label `namespace.karmada.io/skip-auto-propagation: "true"` which can label the namespaces that should be skipped from auto propagation.([#2696](https://github.com/karmada-io/karmada/pull/2696), @jwcesign)
- `karmada-controller-manager`: allow users to update the `.spec.resourceSelectors` filed of `PropagationPolicy/ClusterPropagationPolicy`. ([#2562](https://github.com/karmada-io/karmada/pull/2562), @XiShanYongYe-Chang)
- karmada-controller-manager`: Introduce priority to PropagationPolicy. ([#2767](https://github.com/karmada-io/karmada/pull/2767), @jwcesign)
- `karmada-scheduler-estimator`: leverage scheduler cache to estimate replicas.([#2704](https://github.com/karmada-io/karmada/pull/2704), @Garrybest)
- `karmada-controller-manager`: do not propagate finalizers to member cluster ([#2870](https://github.com/karmada-io/karmada/pull/2870), @stingshen)
- `karmada-scheduler`/`karmada-scheduler-descheduler`: Introduced `--scheduler-estimator-service-prefix` flag for discovery estimators.([#2527](https://github.com/karmada-io/karmada/pull/2527), @carlory)
- `karmada-scheduler`: add scheduling diagnosis.([#2302](https://github.com/karmada-io/karmada/pull/2302), @Garrybest)
- `karmada-agent`: introduce auto certificate rotation function.([#2596](https://github.com/karmada-io/karmada/pull/2596), @lonelyCZ)
- `karmada-webhook`: Prevent modifying and creating `ResourceInterpreterCustomization` using the same interpretation rules. ([#2755](https://github.com/karmada-io/karmada/pull/2755), @chaunceyjiang)
- validate cluster fields: provider, region and zone. ([#2849](https://github.com/karmada-io/karmada/pull/2849), @carlory)

### Other
#### Dependencies
- Download images from docker hub by default. ([#2795](https://github.com/karmada-io/karmada/pull/2795), @jwcesign)
- Karmada is now built with Golang 1.19.3. ([#2857](https://github.com/karmada-io/karmada/pull/2857), @RainbowMango)

#### Helm Chart
- `Helm Chart`: add descheduler name suffix of chart deployment manifest ([#2330](https://github.com/karmada-io/karmada/pull/2330), @calvin0327)
- `HelmChart`:  Fixed liveness probe misconfiguration which caused kube-controller-manager to always `CrashLoopBackup`. ([#2277](https://github.com/karmada-io/karmada/pull/2277), @calvin0327 )
- `chart`: Fix use custom certs lead to post-install-job failed and kube-controller-manager crash by missing /etc/karmada/pki/server-ca.key.([#2637](https://github.com/karmada-io/karmada/pull/2637), @631068264)

#### Instrumentation
- `Instrumentation`: Introduced the `GetDependenciesSucceed` and `GetDependenciesFailed` to the resource template. Introduced the `SyncScheduleResultToDependenciesSucceed` and `SyncScheduleResultToDependenciesFailed` to `resourceBinding` object. ([#2773](https://github.com/karmada-io/karmada/pull/2773), @Poor12 )
- `Instrumentation`: Introduce `EvictWorkloadFromClusterSucceed`, `EvictWorkloadFromClusterFailed` to the `binding` object and its reference. Refactor the event name of `TaintManagerEviction`. ([#2835](https://github.com/karmada-io/karmada/pull/2835), @Poor12)
- `Instrumentation`: Introduced the `resource_find_matched_policy_duration_seconds`, `resource_apply_policy_duration_seconds`, `policy_apply_attempts_total`, `binding_sync_work_duration_seconds`, `work_sync_workload_duration_seconds` metrics. ([#2868](https://github.com/karmada-io/karmada/pull/2868), @Poor12 )
- `Instrumentation`: Introduced the `CreateExecutionSpaceSucceed` and `RemoveExecutionSpaceSucceed` events to `Cluster` object. ([#2688](https://github.com/karmada-io/karmada/pull/2688), @Poor12)
- `Instrumentation`: Introduced the `ApplyOverridePolicySucceed` and `ApplyOverridePolicyFailed` events to workload. ([#2764](https://github.com/karmada-io/karmada/pull/2764), @Poor12)
- `Instrumentation`: Introduced the `ReflectStatusToWorkSucceed`, `ReflectStatusToWorkFailed`, `InterpretHealthSucceed` and `InterpretHealthFailed` events to `work` object. ([#2770](https://github.com/karmada-io/karmada/pull/2770), @Poor12)
- `Instrumentation`: Introduced the `SyncImpersonationConfigSucceed` and `SyncImpersonationConfigFailed` to the `cluster` object.([#2796](https://github.com/karmada-io/karmada/pull/2796), @Poor12)
- `Instrumentation`: Apply `AggregateStatusFailed`, `AggregateStatusSucceed`, `SyncWorkSucceed`, and `SyncWorkFailed` to the `federatedResourceQuota` object. ([#2812](https://github.com/karmada-io/karmada/pull/2812), @Poor12)
- `Instrumentation`: introduce `SyncDerivedServiceSucceed` and `SyncDerivedServiceFailed` to the `serviceImport` object. ([#2830](https://github.com/karmada-io/karmada/pull/2830), @Poor12)
- `Instrumentation`: Introduced the `cluster_ready_info`, `cluster_node_number`, `cluster_ready_node_number`, `cluster_memory_allocatable_bytes`, `cluster_cpu_allocatable_number`, `cluster_pod_allocatable_number`, `cluster_memory_allocated_bytes`, `cluster_cpu_allocated_number`, `cluster_pod_allocated_number`, `cluster_sync_status_duration` to record the cluster status in `karmada-controller-manager` and `karmada-agent`. (@Poor12 [#2496](https://github.com/karmada-io/karmada/pull/2496)
- `Instrumentation`: introduced the `framework_extension_point_duration_seconds` and `plugin_execution_duration_seconds` metrics for `karmada-scheduler`.([#2087](https://github.com/karmada-io/karmada/pull/2087), @Poor12)
