<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.6.6](#v166)
  - [Downloads for v1.6.6](#downloads-for-v166)
  - [Changelog since v1.6.5](#changelog-since-v165)
    - [Changes by Kind](#changes-by-kind)
      - [Bug Fixes](#bug-fixes)
      - [Others](#others)
- [v1.6.5](#v165)
  - [Downloads for v1.6.5](#downloads-for-v165)
  - [Changelog since v1.6.4](#changelog-since-v164)
    - [Changes by Kind](#changes-by-kind-1)
      - [Bug Fixes](#bug-fixes-1)
      - [Others](#others-1)
- [v1.6.4](#v164)
  - [Downloads for v1.6.4](#downloads-for-v164)
  - [Changelog since v1.6.3](#changelog-since-v163)
    - [Changes by Kind](#changes-by-kind-2)
      - [Bug Fixes](#bug-fixes-2)
      - [Others](#others-2)
- [v1.6.3](#v163)
  - [Downloads for v1.6.3](#downloads-for-v163)
  - [Changelog since v1.6.2](#changelog-since-v162)
    - [Changes by Kind](#changes-by-kind-3)
      - [Bug Fixes](#bug-fixes-3)
      - [Others](#others-3)
- [v1.6.2](#v162)
  - [Downloads for v1.6.2](#downloads-for-v162)
  - [Changelog since v1.6.1](#changelog-since-v161)
    - [Changes by Kind](#changes-by-kind-4)
      - [Bug Fixes](#bug-fixes-4)
      - [Others](#others-4)
- [v1.6.1](#v161)
  - [Downloads for v1.6.1](#downloads-for-v161)
  - [Changelog since v1.6.0](#changelog-since-v160)
    - [Changes by Kind](#changes-by-kind-5)
      - [Bug Fixes](#bug-fixes-5)
      - [Others](#others-5)
- [v1.6.0](#v160)
  - [Downloads for v1.6.0](#downloads-for-v160)
  - [What's New](#whats-new)
    - [FederatedHPA](#federatedhpa)
    - [Application Failover](#application-failover)
    - [Karmada Operator](#karmada-operator)
    - [Third-party Resource Interpreter](#third-party-resource-interpreter)
  - [Other Notable Changes](#other-notable-changes)
    - [API Changes](#api-changes)
    - [Bug Fixes](#bug-fixes-6)
    - [Security](#security)
    - [Features & Enhancements](#features--enhancements)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Chart](#helm-chart)
    - [Instrumentation](#instrumentation)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.6.6
## Downloads for v1.6.6

Download v1.6.6 in the [v1.6.6 release page](https://github.com/karmada-io/karmada/releases/tag/v1.6.6).

## Changelog since v1.6.5
### Changes by Kind
#### Bug Fixes
- `karmada-controllr-manager`: concurrency of `binding-status controller` inherit from the concurrency of `binding` declared in global options. ([#4461](https://github.com/karmada-io/karmada/pull/4461), @XiShanYongYe-Chang)
- `karmada-search`: Add the logic of checking whether the resource API to be retrieved is installed in the cluster. ([#4590](https://github.com/karmada-io/karmada/pull/4590), @XiShanYongYe-Chang)
- `karmada-search`: support accept content type `as=Table` in the proxy global resource function. ([#4596](https://github.com/karmada-io/karmada/pull/4596), @jwcesign)

#### Others
- The base image `alpine` now has been promoted from `alpine:3.18.5` to `alpine:3.19.1`. ([#4601](https://github.com/karmada-io/karmada/pull/4601), @Fish-pro)

# v1.6.5
## Downloads for v1.6.5

Download v1.6.5 in the [v1.6.5 release page](https://github.com/karmada-io/karmada/releases/tag/v1.6.5).

## Changelog since v1.6.4
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: only update `aggregatedStatus` and `conditions` fields during `binding-status controller` updating status of binding. ([#4235](https://github.com/karmada-io/karmada/pull/4235), @jwcesign)
- `karmada-controller-manager`: Pruned job labels `batch.kubernetes.io/controller-uid` and `batch.kubernetes.io/job-name` which were introduced by Kubernetes 1.27. ([#4265](https://github.com/karmada-io/karmada/pull/4265), @liangyuanpeng)
- `karmada-search`: Fix lock race affects watch RestChan not close, causing client hanging when performing watch. ([#4241](https://github.com/karmada-io/karmada/pull/4241), @xigang)

#### Others
- `release1.6`: The base image `alpine` now has been promoted from `alpine:3.17.1` to `alpine:3.18.5`. ([#4381](https://github.com/karmada-io/karmada/pull/4381), @zhzhuang-zju)
- Karmada is now built with Go1.20.12. ([#4395](https://github.com/karmada-io/karmada/pull/4395), @RainbowMango)

# v1.6.4
## Downloads for v1.6.4

Download v1.6.4 in the [v1.6.4 release page](https://github.com/karmada-io/karmada/releases/tag/v1.6.4).

## Changelog since v1.6.3
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fix panic when FederatedHPA's SelectPolicy is nil and FederatedHPA webhook is disabled. ([#4104](https://github.com/karmada-io/karmada/pull/4104), @jwcesign)

#### Others
NONE

# v1.6.3
## Downloads for v1.6.3

Download v1.6.3 in the [v1.6.3 release page](https://github.com/karmada-io/karmada/releases/tag/v1.6.3).

## Changelog since v1.6.2
### Changes by Kind
#### Bug Fixes
- `karmada-search`: Fixed a panic due to concurrent mutating objects in the informer cache. ([#3978](https://github.com/karmada-io/karmada/pull/3978), @chaosi-zju)
- `karmada-controller-manager`: Avoid updating directly cached resource templates. ([#3893](https://github.com/karmada-io/karmada/pull/3893), @whitewindmills)
- `karmada-aggregated-apiserver`: Fixed the issue that can not proxy `exec` request to a proxy issue. ([#4022](https://github.com/karmada-io/karmada/pull/4022), @jwcesign)

#### Others
- Bump k8s.io dependencies to v0.26.4 to fix a possible panic. ([#3925](https://github.com/karmada-io/karmada/pull/3925), @liangyuanpeng)

# v1.6.2
## Downloads for v1.6.2

Download v1.6.2 in the [v1.6.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.6.2).

## Changelog since v1.6.1
### Changes by Kind
#### Bug Fixes
- Chart: Fixed the issue that `karmada-search` no ETCD secret volume mount when using external ETCD.
- `karmada-controller-manager`:  Fixed a boundary case where the `propagationpolicy.karmada.io/name` in the label of the resource template would not be removed after deleting PP.

#### Others
None.

# v1.6.1
## Downloads for v1.6.1

Download v1.6.1 in the [v1.6.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.6.1).

## Changelog since v1.6.0
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed `Applied` condition of ResourceBinding is always true issue. (#3724, @jwcesign)
- `karmada-controller-manager`: fix panic when printing log when lastSuccessfulTime of cronjob is nil. (#3698, @yike21)
- `karmada-webhook`: When application failover is enabled, users are prevented from setting propagateDeps to `false`. (#3766, @RuliXu)

#### Others
None.

# v1.6.0
## Downloads for v1.6.0

Download v1.6.0 in the [v1.6.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.6.0).

## What's New

### FederatedHPA

Introduced `FederatedHPA` API to address the requirements that scale workloads across clusters.
`FederatedHPA` works similarly to HPA in a single cluster. Karmada aggregates metrics from multiple clusters through the `karmada-metrics-adaptor` component and then scales Pod replicas. The scaled replicas will be distributed to multiple clusters according to the declaration of `PropagationPolicy` or `ClusterPropagationPolicy`.

See [FederatedHPA Proposal](https://github.com/karmada-io/karmada/blob/master/docs/proposals/hpa/federatedhpa-v2.md) for more details.

(Feature contributor: @chaunceyjiang @jwcesign @Poor12)

### Application Failover

In a multi-cluster scenario, the fault may come from the cluster as a whole, or it may be because the application cannot adapt to a certain cluster. Users are now able to declare strategies of application failover about when and how to migrate the unhealthy application in `PropagationPolicy` and `ClusterPropagationPolicy`. Karmada will automatically migrate unhealthy applications to other available clusters to improve their availability.

See [application-level failover](https://karmada.io/docs/next/userguide/failover/application-failover) for more details.
(Feature contributor: @Poor12, @RainbowMango)

### Karmada Operator

Karmada provides [CLI tools and Helm Charts](https://karmada.io/docs/installation/) for installation and deployment in previous releases. In this release, the Karmada operator is provided as another declarative deployment method.

The Karmada operator is a method for installing, upgrading, and deleting Karmada instances. It builds upon the basic Karmada resource and controller concepts and provides convenience to centrally manage the entire lifecycle of Karmada instances in a global cluster. With the operator, users can extend Karmada with custom resources (CRs) to manage their instances not only in local clusters but also in remote clusters.

See [quick start](https://github.com/karmada-io/karmada/blob/master/operator/README.md) for more details.
(Feature contributor: @calvin0327, @lonelyCZ, @Poor12)

### Third-party Resource Interpreter

Karmada's Resource Interpreter Framework is designed for interpreting resource structure. It consists of built-in and customized interpreters. Karmada has bundled the following open-sourced resources so that users can save the effort to customize them, including Argo Workflow, Flux CD, Kyverno, and OpenKruise. They have been verified by the community.

(Feature contributor: @yike12, @chaunceyjiang, @Poor12)

## Other Notable Changes
### API Changes
- API change: The length of `AffinityName` in `PropagationPolicy` now is restricted to [1, 32], and must be a qualified name. ([#3442](https://github.com/karmada-io/karmada/pull/3442), @chaunceyjiang)
- API change: Introduced short name `wk` for resource `Work`. ([#3468](https://github.com/karmada-io/karmada/pull/3468), @yanfeng1992)

### Bug Fixes
- `karmada-webhook`: Introduced validation to ensure the `.spec.placement.spreadConstraints.maxGroups/minGroups` in PropagationPolicy is declared with a reasonable value. ([#3232](https://github.com/karmada-io/karmada/pull/3232), @whitewindmills )
- `karmada-webhook`: Validated the predicate path for imageOverride. ([#3397](https://github.com/karmada-io/karmada/pull/3397), @chaunceyjiang)
- `karmada-webhook`: Added the missing federatedresourcequota validation config. ([#3523](https://github.com/karmada-io/karmada/pull/3523), @chaunceyjiang)
- `karmada-controller-manager`: Fixed the issue that RB/CRB labels were not merged when syncing new changes. ([#3239](https://github.com/karmada-io/karmada/pull/3239), @lxtywypc)
- `karmada-controller-manager`: Fixed the issue that control plane endpointslices cannot be deleted. ([#3348](https://github.com/karmada-io/karmada/pull/3348), @wenchezhao)
- `karmada-controller-manager`: Corrected the issue of adding duplicate eviction tasks. ([#3456](https://github.com/karmada-io/karmada/pull/3456), @jwcesign)
- `karmada-controller-manager`: Fixed a corner case that when there were tasks in the GracefulEvictionTasks queue, graceful-eviction-controller would not work after restarting karmada-controller-manager. ([#3475](https://github.com/karmada-io/karmada/pull/3475), @chaunceyjiang)
- `karmada-controller-manager`: Fixed the panic issue in the case that the grade number of resourceModel is less than the number of resources. ([#3591](https://github.com/karmada-io/karmada/pull/3591), @sunbinnnnn)
- `karmadactl`: Resolved the failure to view the options of `karmadactl addons enable/disable`. ([#3298](https://github.com/karmada-io/karmada/pull/3298), @Poor12)
- `karmada-scheduler`: Resolved unexpected re-scheduling due to mutating informer cache issue. ([#3393](https://github.com/karmada-io/karmada/pull/3393), @whitewindmills)
- `karmada-scheduler`: Fixed the issue of inconsistent Generation and SchedulerObservedGeneration. ([#3455](https://github.com/karmada-io/karmada/pull/3455), @Poor12)
- `karmada-search`: Fixed the paging list issue in karmada search proxy in large-scale member clusters. ([#3402](https://github.com/karmada-io/karmada/pull/3402), @ikaven1024)
- `karmada-search`: Fixed a panic in ResourceRegistry controller caused by receiving DeletedFinalStateUnknown object from the cache. ([#3478](https://github.com/karmada-io/karmada/pull/3478), @xigang)
- `karmada-search`: Fixed contecnt-type header issue in HTTP responses. ([#3505](https://github.com/karmada-io/karmada/pull/3505), @callmeoldprince)

### Security

### Features & Enhancements
- `karmadactl`: Introduced `--image-pull-secrets` flag to `init` command to specify the secret. ([#3237](https://github.com/karmada-io/karmada/pull/3237), @my-git9)
- `karmadactl`: Introduced `--force` flag to `addons disable` command. ([#3266](https://github.com/karmada-io/karmada/pull/3266), @my-git9)
- `karmadactl`: Introduced support for running `init` within a pod. ([#3338](https://github.com/karmada-io/karmada/pull/3338), @lonelyCZ)
- `karmadactl`: Introduced `--host-cluster-domain` flag to command `init` and `addons` to specify the host cluster domain. ([#3292](https://github.com/karmada-io/karmada/pull/3292), @tedli)
- `karmadactl`: Introduced `--private-image-registry` flag to `addons` command to specify image registry. ([#3345](https://github.com/karmada-io/karmada/pull/3345), @my-git9)
- `karmadactl`: Introduced `--purge-namespace` flag for `deinit` command to skip namespace deletion during uninstallation. ([#3326](https://github.com/karmada-io/karmada/pull/3326), @my-git9)
- `karmadactl`: Introduced `--auto-create-policy` and `--policy-name` flags for `promote` command to customize the policy during the promotion. ([#3494](https://github.com/karmada-io/karmada/pull/3494), @LronDC)
- `karmada-aggregated-apiserver`: Increased `.metadata.generation` once the desired state of the `Cluster` object is changed. ([#3241](https://github.com/karmada-io/karmada/pull/3241), @XiShanYongYe-Chang)
- `karmada-controller-mamager`: Provided support for Lua's built-in string function in ResourceInterpreterCustomization. ([#3256](https://github.com/karmada-io/karmada/pull/3256), @chaunceyjiang)
- `karmada-controller-manager`:  The overriders `commandsOverrider` and `argOverride` in `OverridePolicy` now support `Job` resources. ([#3414](https://github.com/karmada-io/karmada/pull/3414), @chaunceyjiang)
- `karmada-controller-manager`: Allowed setting wildcards for `--skippedPropagatingNamespaces` flag. ([#3373](https://github.com/karmada-io/karmada/pull/3373), @chaunceyjiang)
- `karmada-controller-manager`/`karmada-agent`: Supported connection to resourceInterpretWebhook without DNS Service. ([#2999](https://github.com/karmada-io/karmada/pull/2999), @lxtywypc)
- `karmada-controller-manager`: The `--skipped-propagating-namespaces` flags now can take regular expressions to represent namespaces and defaults to `kube-*`. ([#3433](https://github.com/karmada-io/karmada/pull/3433), @chaunceyjiang)
- `karmada-controller-manager`: Introduced `--concurrent-propagation-policy-syncs`/`--concurrent-cluster-propagation-policy-syncs` flags to specify concurrent syncs for PropagationPolicy and ClusterPropagationPolicy. ([#3511](https://github.com/karmada-io/karmada/pull/3511), @zach593)
- `karmada-search`: Introduced unified-auth support for proxy. ([#3279](https://github.com/karmada-io/karmada/pull/3279), @XiShanYongYe-Chang)
- `karmada-search`: Returned the actual resource list from search API. ([#3312](https://github.com/karmada-io/karmada/pull/3312), @tedli )
- `karmada-search`: Fixed the problem that ResourceVersion base64 encrypted repeatedly when starting multiple informers to watch resources. ([#3376](https://github.com/karmada-io/karmada/pull/3376), @niuyueyang1996)
- `karmada-search`: Supported namespace filters in RR for search proxy. ([#3527](https://github.com/karmada-io/karmada/pull/3527), @ikaven1024)
- `karmada-scheduler`: Optimized the region selection algorithm. ([#3259](https://github.com/karmada-io/karmada/pull/3259), @whitewindmills)
- `karmada-scheduler`: Introduced `clusterEviction` plugin to skip the clusters that are in the process of eviction. ([#3469](https://github.com/karmada-io/karmada/pull/3469))
- `karmada-webhook`: Inroduced validation for `MultiClusterIngress` objects. ([#3516](https://github.com/karmada-io/karmada/pull/3516), @XiShanYongYe-Chang)

## Other
### Dependencies
- Karmada is now built with Kubernetes v1.26.2 dependencies. (fix CVE-2022-41723) ([#3252](https://github.com/karmada-io/karmada/pull/3252), @RainbowMango)
- Karmada (v1.6) is now built in Go 1.20.4. ([#3565](https://github.com/karmada-io/karmada/pull/3565), @RainbowMango)

### Helm Chart
- chart: Introduced `controllers` config for `karmada-controller-manager`. ([#3240](https://github.com/karmada-io/karmada/pull/3240), @Poor12)
- Users can specify command-line parameters other than the default parameters in helm values ​​through controllerManager.extraCommandArgs. ([#3268](https://github.com/karmada-io/karmada/pull/3268), @Poor12)
- chart: Introduced support for customizing tolerances for internal etcd. ([#3336](https://github.com/karmada-io/karmada/pull/3336), @chaunceyjiang)
- chart: Fixed the issue of resource residue after deletion(`helm uninstall`). ([#3473](https://github.com/karmada-io/karmada/pull/3473), @7sunarni)

### Instrumentation
None
