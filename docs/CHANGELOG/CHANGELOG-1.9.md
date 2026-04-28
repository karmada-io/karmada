<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.9.10](#v1910)
  - [Downloads for v1.9.10](#downloads-for-v1910)
  - [Changelog since v1.9.9](#changelog-since-v199)
    - [Changes by Kind](#changes-by-kind)
      - [Urgent Upgrade Notes](#urgent-upgrade-notes)
      - [Bug Fixes](#bug-fixes)
      - [Others](#others)
- [v1.9.9](#v199)
  - [Downloads for v1.9.9](#downloads-for-v199)
  - [Changelog since v1.9.8](#changelog-since-v198)
    - [Changes by Kind](#changes-by-kind-1)
      - [Bug Fixes](#bug-fixes-1)
      - [Others](#others-1)
- [v1.9.8](#v198)
  - [Downloads for v1.9.8](#downloads-for-v198)
  - [Changelog since v1.9.7](#changelog-since-v197)
    - [Changes by Kind](#changes-by-kind-2)
      - [Bug Fixes](#bug-fixes-2)
      - [Others](#others-2)
- [v1.9.7](#v197)
  - [Downloads for v1.9.7](#downloads-for-v197)
  - [Changelog since v1.9.6](#changelog-since-v196)
    - [Changes by Kind](#changes-by-kind-3)
      - [Bug Fixes](#bug-fixes-3)
      - [Others](#others-3)
- [v1.9.6](#v196)
  - [Downloads for v1.9.6](#downloads-for-v196)
  - [Changelog since v1.9.5](#changelog-since-v195)
    - [Changes by Kind](#changes-by-kind-4)
      - [Bug Fixes](#bug-fixes-4)
      - [Others](#others-4)
- [v1.9.5](#v195)
  - [Downloads for v1.9.5](#downloads-for-v195)
  - [Changelog since v1.9.4](#changelog-since-v194)
    - [Changes by Kind](#changes-by-kind-5)
      - [Bug Fixes](#bug-fixes-5)
      - [Others](#others-5)
- [v1.9.4](#v194)
  - [Downloads for v1.9.4](#downloads-for-v194)
  - [Changelog since v1.9.3](#changelog-since-v193)
    - [Changes by Kind](#changes-by-kind-6)
      - [Bug Fixes](#bug-fixes-6)
      - [Others](#others-6)
- [v1.9.3](#v193)
  - [Downloads for v1.9.3](#downloads-for-v193)
  - [Changelog since v1.9.2](#changelog-since-v192)
    - [Changes by Kind](#changes-by-kind-7)
      - [Bug Fixes](#bug-fixes-7)
      - [Others](#others-7)
- [v1.9.2](#v192)
  - [Downloads for v1.9.2](#downloads-for-v192)
  - [Changelog since v1.9.1](#changelog-since-v191)
    - [Changes by Kind](#changes-by-kind-8)
      - [Bug Fixes](#bug-fixes-8)
      - [Others](#others-8)
- [v1.9.1](#v191)
  - [Downloads for v1.9.1](#downloads-for-v191)
  - [Changelog since v1.9.0](#changelog-since-v190)
    - [Changes by Kind](#changes-by-kind-9)
      - [Bug Fixes](#bug-fixes-9)
      - [Others](#others-9)
- [v1.9.0](#v190)
  - [Downloads for v1.9.0](#downloads-for-v190)
  - [What's New](#whats-new)
    - [Lazy mode of PropagationPolicy](#lazy-mode-of-propagationpolicy)
    - [Cluster condition-based remedy system](#cluster-condition-based-remedy-system)
    - [Scheduler Estimator Enhancements](#scheduler-estimator-enhancements)
  - [Other Notable Changes](#other-notable-changes)
    - [API Changes](#api-changes)
    - [Deprecation](#deprecation)
    - [Bug Fixes](#bug-fixes-10)
    - [Security](#security)
    - [Features & Enhancements](#features--enhancements)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Charts](#helm-charts)
    - [Instrumentation](#instrumentation)
  - [Contributors](#contributors)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.9.10
## Downloads for v1.9.10

Download v1.9.10 in the [v1.9.10 release page](https://github.com/karmada-io/karmada/releases/tag/v1.9.10).

## Changelog since v1.9.9
### Changes by Kind
#### Urgent Upgrade Notes
- The feature `Failover` now has been disabled by default, which should be explicitly enabled to avoid unexpected incidents. ([#5948](https://github.com/karmada-io/karmada/pull/5948), @XiShanYongYe-Chang)

If you are using the feature `Failover`, please enable it explicitly by adding the `--feature-gates=Failover=true,<other feature>` flag to the `karmada-controller-manager` component. If you are not using this feature, this change will have no impact.

#### Bug Fixes
- `karmadactl`: Fixed `karmada-metrics-adapter` use the incorrect certificate issue when deployed via karmadactl `init`. ([#5859](https://github.com/karmada-io/karmada/pull/5859), @seanlaii)
- `karmada-controller-manager`: Fixed the corner case where the reconciliation of aggregating status might be missed in case of component restart. ([#5884](https://github.com/karmada-io/karmada/pull/5884), @liangyuanpeng)
- `karmada-scheduler`: Avoid filtering out clusters if the API enablement is incomplete during re-scheduling. ([#5932](https://github.com/karmada-io/karmada/pull/5932), @XiShanYongYe-Chang)

#### Others
- The base image `alpine` now has been promoted from `alpine:3.20.3` to `alpine:3.21.0`. ([#5922](https://github.com/karmada-io/karmada/pull/5922))

# v1.9.9
## Downloads for v1.9.9

Download v1.9.9 in the [v1.9.9 release page](https://github.com/karmada-io/karmada/releases/tag/v1.9.9).

## Changelog since v1.9.8
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that status aggregation against the resource template might be missed due to slow cache sync. ([#5853](https://github.com/karmada-io/karmada/pull/5853), @chaosi-zju)
- `karmadactl`: The `--force` option of `unjoin` command now try to clean up resources propagated in member clusters. ([#5849](https://github.com/karmada-io/karmada/pull/5849), @chaosi-zju)

#### Others
None.

# v1.9.8
## Downloads for v1.9.8

Download v1.9.8 in the [v1.9.8 release page](https://github.com/karmada-io/karmada/releases/tag/v1.9.8).

## Changelog since v1.9.7
### Changes by Kind
#### Bug Fixes
- `karmada-aggregated-apiserver`: User can append a "/" at the end when configuring the cluster's apiEndpoint. ([#5557](https://github.com/karmada-io/karmada/pull/5557), @spiritNO1)
- `karmada-controller-manager`: Ignored StatefulSet Dependencies with PVCs created via the VolumeClaimTemplates. ([#5688](https://github.com/karmada-io/karmada/pull/5688), @seanlaii)
- `karmada-scheduler`: Fixed unexpected modification of original `ResourceSummary` due to lack of deep copy. ([#5726](https://github.com/karmada-io/karmada/pull/5726), @RainbowMango)
- `karmada-scheduler`: Fixes an issue where resource model grades were incorrectly matched based on resource requests. Now only grades that can provide sufficient resources will be selected. ([#5730](https://github.com/karmada-io/karmada/pull/5730), @RainbowMango)
- `karmada-search`: Modify the logic of checking whether the resource is registered when selecting the plugin. ([#5735](https://github.com/karmada-io/karmada/pull/5735), @seanlaii)

#### Others
None.

# v1.9.7
## Downloads for v1.9.7

Download v1.9.7 in the [v1.9.7 release page](https://github.com/karmada-io/karmada/releases/tag/v1.9.7).

## Changelog since v1.9.6
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the error of cluster status old condition update will overwrite the newest condition. ([#5402](https://github.com/karmada-io/karmada/pull/5402), @XiShanYongYe-Chang)

#### Others
- The base image `alpine` now has been promoted from `alpine:3.20.2` to `alpine:3.20.3`.

# v1.9.6
## Downloads for v1.9.6

Download v1.9.6 in the [v1.9.6 release page](https://github.com/karmada-io/karmada/releases/tag/v1.9.6).

## Changelog since v1.9.5
### Changes by Kind
#### Bug Fixes
None.

#### Others
- The base image `alpine` now has been promoted from `alpine:3.20.1` to `alpine:3.20.2`. ([#5269](https://github.com/karmada-io/karmada/pull/5269))
- Bump golang version to `v1.20.14`. ([#5374](https://github.com/karmada-io/karmada/pull/5374) @zhzhuang-zju)

# v1.9.5
## Downloads for v1.9.5

Download v1.9.5 in the [v1.9.5 release page](https://github.com/karmada-io/karmada/releases/tag/v1.9.5).

## Changelog since v1.9.4
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: fix the issue of residual work in the MultiClusterService feature. ([#5212](https://github.com/karmada-io/karmada/pull/5212), @XiShanYongYe-Chang)

#### Others
None.

# v1.9.4
## Downloads for v1.9.4

Download v1.9.4 in the [v1.9.4 release page](https://github.com/karmada-io/karmada/releases/tag/v1.9.4).

## Changelog since v1.9.3
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that the default resource interpreter doesn't accurately interpret the numbers of replicas. ([#5107](https://github.com/karmada-io/karmada/pull/5107), @whitewindmills)

#### Others
- The base image `alpine` now has been promoted from `alpine:3.20.0` to `alpine:3.20.1`. ([#5089](https://github.com/karmada-io/karmada/pull/5089))

# v1.9.3
## Downloads for v1.9.3

Download v1.9.3 in the [v1.9.3 release page](https://github.com/karmada-io/karmada/releases/tag/v1.9.3).

## Changelog since v1.9.2
### Changes by Kind
#### Bug Fixes
- `karmada-scheduler-estimator`: Fixed the `Unschedulable` result returned by plugins to be treated as an exception issue. ([#5026](https://github.com/karmada-io/karmada/pull/5026), @RainbowMango)
- `karmada-controller-manager`: Fixed an issue that the cluster-status-controller overwrites the remedyActions field. ([#5045](https://github.com/karmada-io/karmada/pull/5045), @XiShanYongYe-Chang)

#### Others
None.

# v1.9.2
## Downloads for v1.9.2

Download v1.9.2 in the [v1.9.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.9.2).

## Changelog since v1.9.1
### Changes by Kind
#### Bug Fixes
None.

#### Others
- `karmadactl`:  The policy for when to pull a container image now is `IfNotPresent in `init` command. ([#4988](https://github.com/karmada-io/karmada/pull/4988), @zhzhuang-zju)
- The base image `alpine` now has been promoted from `alpine:3.19.1` to `alpine:3.20.0`.

# v1.9.1
## Downloads for v1.9.1

Download v1.9.1 in the [v1.9.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.9.1).

## Changelog since v1.9.0
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed incorrect annotation markup when policy preemption occurs. ([#4772](https://github.com/karmada-io/karmada/pull/4772), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fix the problem that labels cannot be deleted via Karmada propagation. ([#4797](https://github.com/karmada-io/karmada/pull/4797), @whitewindmills)
- `karmada-controller-manager`: Fix the problem that work.karmada.io/permanent-id constantly changes with every update. ([#4819](https://github.com/karmada-io/karmada/pull/4819), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the bug of mcs binding losing resourcebinding.karmada.io/permanent-id label. ([#4822](https://github.com/karmada-io/karmada/pull/4822), @XiShanYongYe-Chang)

#### Others
None.

# v1.9.0
## Downloads for v1.9.0

Download v1.9.0 in the [v1.9.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.9.0).

## What's New

### Lazy mode of PropagationPolicy

The PropgationPolicy now supports lazy mode that can delay the activation of modification. In contrast to the default behavior, modifications to the PropagationPolicy will not immediately trigger scheduling and redistribute the workload but will be delayed until changes happen on workloads themselves. It will be useful in using global PropagationPolicy to manage huge amounts of workloads.

The ability also applies to ClusterPropagationPolicy and it is disabled by default for backward compatibility, users need to enable it explicitly by specifying the `activationPreference` in PropagationPolicy or ClusterPropagationPolicy.

See [Proposal of the LazyActivation preference for Policy](https://github.com/karmada-io/karmada/blob/master/docs/proposals/scheduling/activation-preference/lazy-activation-preference.md) for more details.

(Feature contributors: @chaosi-zju)

### Cluster condition-based remedy system

The applications running within a cluster may be disrupted by cluster failures, it's an effective practice to deploy applications across multiple clusters, by leveraging load balancers that can route business traffic simultaneously to these clusters.

This release introduced an automated healing capability based on the health status of clusters. It relies on a third-party system to identify cluster failures and report through Conditions of a Cluster object, with the remedy capability, Karmada can take predefined actions according to the reported cluster failures. This can be used to prevent business traffic from the failure cluster, and thus can enhance availability by promptly responding to and mitigating the impact of cluster failures.

(Feature contributor: @XiShanYongYe-Chang)

### Scheduler Estimator Enhancements

The Karmada Scheduler Estimator introduced a plugin that takes resource quota into account during the estimation process.
It is in alpha state and controlled by the feature gate `ResourceQuotaEstimate`, once this feature is enabled, the Karmada Scheduler Estimator can estimate the number of workload replicas based on resource quota.

(Feature contributors: @wengyao04)

## Other Notable Changes
### API Changes
- Introduced `ActivationPreference` to the `PropagationPolicy` and `ClusterPropagationPolicy` to indicate how the referencing resource template will be propagated, in case of policy changes. ([#4577](https://github.com/karmada-io/karmada/pull/4577), @chaosi-zju)
- Introduced the `Remedy` CRD in the `remedy.karmada.io` group. ([#4635](https://github.com/karmada-io/karmada/pull/4635), @XiShanYongYe-Chang)
- Introduced `RemedyActions` to the `Cluster` API to represent the remedy actions. ([#4635](https://github.com/karmada-io/karmada/pull/4635), @XiShanYongYe-Chang)
- Introduced `TrafficBlockClusters` and `ServiceLocations` to the `MultiClusterIngress` API. ([#4635](https://github.com/karmada-io/karmada/pull/4635), @XiShanYongYe-Chang)
- Added additional printer columns `KIND` for `Work` CRD. ([#2066](https://github.com/karmada-io/karmada/pull/2066), @lonelyCZ)

### Deprecation
- None

### Bug Fixes
- `karmada-controller-manager`: fix incorrect `forType` in `cluster-resource-binding-status controller` from `ResouceBinding` to `ClusterResourceBinding`. ([#4338](https://github.com/karmada-io/karmada/pull/4338), @lxtywypc)
- `karmada-controller-manager`: Fix resource(work/resource in member clusters) conflicting between PP and MCS. ([#4414](https://github.com/karmada-io/karmada/pull/4414), @jwcesign)
- `karmada-controller-manager`: Fixed the issue that always trying to re-create resources that should not be propagated. ([#4422](https://github.com/karmada-io/karmada/pull/4422), @jwcesign)
- `karmada-controller-manager`: Fixed the issue that the service of MCS can not be propagated to newly joined clusters. ([#4423](https://github.com/karmada-io/karmada/pull/4423), @jwcesign)
- `karmada-controller-manager`: Fix the bug that losing the chance to un-claim resource template in case of deleting ClusterPropagationPolicy. ([#4387](https://github.com/karmada-io/karmada/pull/4387), @whitewindmills)
- `karmada-controller-manager`: clean the finalizer of MCS if the ExposureType transforms from CrossCluster to LoadBalancer. ([#4448](https://github.com/karmada-io/karmada/pull/4448), @jwcesign)
- `karmada-controller-manager`: ignore reconcile the mcs(triggered by svc) if the mcs is not CrossCluster. ([#4452](https://github.com/karmada-io/karmada/pull/4452), @jwcesign)
- `karmada-controller-manager`: Fixed the issue that grace eviction is being blocked due to skipping InterpretHealth for resources without status. ([#4453](https://github.com/karmada-io/karmada/pull/4453), @chaosi-zju)
- `karmada-controller-manager`: Fixed a corner case that `applied` status on Work/ResourceBinding not updated in case of re-create failed. ([#4341](https://github.com/karmada-io/karmada/pull/4341), @zhzhuang-zju)
- `karmadactl`: Fixed return err in case of  secret.spec. caBundle is nil. ([#4371](https://github.com/karmada-io/karmada/pull/4371), @CharlesQQ)
- `karmadactl`:  Register cluster install karmada-agent should set leader-elect-resouce-namespace. ([#4404](https://github.com/karmada-io/karmada/pull/4404), @yanfeng1992)
- `karmada-search`: Add the logic of checking whether the resource API to be retrieved is installed in the cluster. ([#4554](https://github.com/karmada-io/karmada/pull/4554), @yanfeng1992)
- `karmada-search`: support accept content type `as=Table` in the proxy global resource function. ([#4580](https://github.com/karmada-io/karmada/pull/4580), @niuyueyang1996)
- `karmada-scheduler`: reschedule the replicas of the disappear clusters in PP/CPP. ([#4586](https://github.com/karmada-io/karmada/pull/4586), @jwcesign)
- `karmada-operator`: Fixed the issue that the component can not be redeployed due to service update is not allowed. ([#4649](https://github.com/karmada-io/karmada/pull/4649), @laihezhao)

### Security
- `Security`: Disabled unsafe lua packages and only safe packages are allowed when customing resource interpreter. ([#4519](https://github.com/karmada-io/karmada/pull/4519), @XiShanYongYe-Chang)

### Features & Enhancements
- `karmada-controller-manager`: Make `multiclusterservice` aware of Cluster changes. ([#4360](https://github.com/karmada-io/karmada/pull/4360), @Rains6)
- `karmada-controller-manager`: dispatch eps to the newly joined consumption clusters. ([#4356](https://github.com/karmada-io/karmada/pull/4356), @jwcesign)
- `karmada-controller-manager`: The control plane is responsible for deleting expired EPS of MultiClusterService. ([#4383](https://github.com/karmada-io/karmada/pull/4383), @jwcesign)
- `karmada-controller-manager: use Patch() instead of Update() when updating workload status because the error rate is too high.` ([#4094](https://github.com/karmada-io/karmada/pull/4094), @zach593)
- `karmada-operator`: Support install `karmada-search` with operator. ([#4316](https://github.com/karmada-io/karmada/pull/4316), @zhzhuang-zju)
- `karmada-operator`: Enable embedded object meta generated for karmada operator CRD. ([#4315](https://github.com/karmada-io/karmada/pull/4315), @zhzhuang-zju)
- `karmada-webhook`: prevent updates to mcs.types or when multiple types are involved. ([#4454](https://github.com/karmada-io/karmada/pull/4454), @jwcesign)
- `karmada-search`: Implement node/pod tableconvert, Other resource keep use defaultTableConvert. ([#4584](https://github.com/karmada-io/karmada/pull/4584), @niuyueyang1996)

## Other
### Dependencies
- The base image `alpine` now has been promoted from `alpine:3.18.3` to `alpine:3.18.5`. ([#4376](https://github.com/karmada-io/karmada/pull/4376), @zhzhuang-zju)
- Kubernetes dependencies now have been bumped to v1.28.5. ([#4463](https://github.com/karmada-io/karmada/pull/4463), @RainbowMango)
- Bump golang.org/x/crypto to v.0.17.0 to fix CVE(CVE-2023-48795) concerns. ([#4489](https://github.com/karmada-io/karmada/pull/4489), @zhzhuang-zju)
- The base image `alpine` now has been promoted from `alpine:3.18.5` to `alpine:3.19.1` ([#4598](https://github.com/karmada-io/karmada/pull/4598), @Fish-pro)

### Helm Charts
- `Helm Chart`: Provided the ability to label resources. ([#4337](https://github.com/karmada-io/karmada/pull/4337), @a7i)
- `Helm chart`: Updated helm index for 1.8.0 release, and added the karmada-operator chart.([#4349](https://github.com/karmada-io/karmada/pull/4349), @wrhight)
- `Helm Chart`: Make `karmada-metrics-adapter` installation optional with host mode. ([#4375](https://github.com/karmada-io/karmada/pull/4375), @yizhang-zen)
- `Helm Chart`: Make hook-delete-policy in helm job configurable. ([#4393](https://github.com/karmada-io/karmada/pull/4393), @chaosi-zju)

### Instrumentation
- `Instrumentation`: Event `SyncServiceFailed` will be emitted in case MCS fails to sync service to target clusters. ([#4433](https://github.com/karmada-io/karmada/pull/4433), @jwcesign)

## Contributors
Thank you to everyone who contributed to this release!

Users whose commits are in this release (alphabetically by username)

- @a7i
- @Affan-7
- @chaosi-zju
- @CharlesQQ
- @chengleqi
- @dongjiang1989
- @Fish-pro
- @helen-frank
- @hezhizhen
- @ipsum-0320
- @jwcesign
- @laihezhao
- @Larry-shuo
- @liangyuanpeng
- @lonelyCZ
- @lxtywypc
- @niuyueyang1996
- @RainbowMango
- @Rains6
- @Vacant2333
- @wengyao04
- @whitewindmills
- @wm775825
- @wrhight
- @XiShanYongYe-Chang
- @yanfeng1992
- @yanggangtony
- @yizhang-zen
- @zach593
- @zhzhuang-zju
- @zll600
