<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.11.9](#v1119)
  - [Downloads for v1.11.9](#downloads-for-v1119)
  - [Changelog since v1.11.8](#changelog-since-v1118)
    - [Changes by Kind](#changes-by-kind)
      - [Bug Fixes](#bug-fixes)
      - [Others](#others)
- [v1.11.8](#v1118)
  - [Downloads for v1.11.8](#downloads-for-v1118)
  - [Changelog since v1.11.7](#changelog-since-v1117)
    - [Changes by Kind](#changes-by-kind-1)
      - [Bug Fixes](#bug-fixes-1)
      - [Others](#others-1)
- [v1.11.7](#v1117)
  - [Downloads for v1.11.7](#downloads-for-v1117)
  - [Changelog since v1.11.6](#changelog-since-v1116)
    - [Changes by Kind](#changes-by-kind-2)
      - [Bug Fixes](#bug-fixes-2)
      - [Others](#others-2)
- [v1.11.6](#v1116)
  - [Downloads for v1.11.6](#downloads-for-v1116)
  - [Changelog since v1.11.5](#changelog-since-v1115)
    - [Changes by Kind](#changes-by-kind-3)
      - [Bug Fixes](#bug-fixes-3)
      - [Others](#others-3)
- [v1.11.5](#v1115)
  - [Downloads for v1.11.5](#downloads-for-v1115)
  - [Changelog since v1.11.4](#changelog-since-v1114)
    - [Changes by Kind](#changes-by-kind-4)
      - [Bug Fixes](#bug-fixes-4)
      - [Others](#others-4)
- [v1.11.4](#v1114)
  - [Downloads for v1.11.4](#downloads-for-v1114)
  - [Changelog since v1.11.3](#changelog-since-v1113)
    - [Changes by Kind](#changes-by-kind-5)
      - [Urgent Upgrade Notes](#urgent-upgrade-notes)
      - [Bug Fixes](#bug-fixes-5)
      - [Others](#others-5)
- [v1.11.3](#v1113)
  - [Downloads for v1.11.3](#downloads-for-v1113)
  - [Changelog since v1.11.2](#changelog-since-v1112)
    - [Changes by Kind](#changes-by-kind-6)
      - [Bug Fixes](#bug-fixes-6)
      - [Others](#others-6)
- [v1.11.2](#v1112)
  - [Downloads for v1.11.2](#downloads-for-v1112)
  - [Changelog since v1.11.1](#changelog-since-v1111)
    - [Changes by Kind](#changes-by-kind-7)
      - [Bug Fixes](#bug-fixes-7)
      - [Others](#others-7)
- [v1.11.1](#v1111)
  - [Downloads for v1.11.1](#downloads-for-v1111)
  - [Changelog since v1.11.0](#changelog-since-v1110)
    - [Changes by Kind](#changes-by-kind-8)
      - [Bug Fixes](#bug-fixes-8)
      - [Others](#others-8)
- [v1.11.0](#v1110)
  - [Downloads for v1.11.0](#downloads-for-v1110)
  - [What's New](#whats-new)
    - [Cluster-Level Resource Propagation Pause and Resume](#cluster-level-resource-propagation-pause-and-resume)
    - [Karmadactl Offers More Advanced Features](#karmadactl-offers-more-advanced-features)
    - [Consistent generation semantics for multi-cluster workloads](#consistent-generation-semantics-for-multi-cluster-workloads)
    - [Karmada Operator Supports Custom CRD Download Strategy](#karmada-operator-supports-custom-crd-download-strategy)
  - [Other Notable Changes](#other-notable-changes)
    - [API Changes](#api-changes)
    - [Deprecation](#deprecation)
    - [Bug Fixes](#bug-fixes-9)
    - [Security](#security)
    - [Features & Enhancements](#features--enhancements)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Charts](#helm-charts)
    - [Instrumentation](#instrumentation)
  - [Contributors](#contributors)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.11.9
## Downloads for v1.11.9

Download v1.11.9 in the [v1.11.9 release page](https://github.com/karmada-io/karmada/releases/tag/v1.11.9).

## Changelog since v1.11.8
### Changes by Kind
#### Bug Fixes
- `karmada-agent`: Fixed the issue where a new pull-mode cluster may overwrite the existing member clusters. ([#6262](https://github.com/karmada-io/karmada/pull/6262), @zhzhuang-zju)
- `karmadactl`: Fixed the issue where option `discovery-timeout` fails to work properly. ([#6278](https://github.com/karmada-io/karmada/pull/6278), @seanlaii)
- `karmada-controller-manager`: Fixed the issue that the result label of `federatedhpa_pull_metrics_duration_seconds` is always `success`. ([#6314](https://github.com/karmada-io/karmada/pull/6314), @tangzhongren)
- `karmada-controller-manager`/`karmada-agent`: Fixed the issue that cluster status update interval shorter than configured `--cluster-status-update-frequency`. ([#6337](https://github.com/karmada-io/karmada/pull/6337), @RainbowMango)
- `helm`: Fixed the issue where the required ServiceAccount was missing when the certificate mode was set to custom. ([#6240](https://github.com/karmada-io/karmada/pull/6240), @seanlaii)

#### Others
None.

# v1.11.8
## Downloads for v1.11.8

Download v1.11.8 in the [v1.11.8 release page](https://github.com/karmada-io/karmada/releases/tag/v1.11.8).

## Changelog since v1.11.7
### Changes by Kind
#### Bug Fixes
- `karmada-agent`: Fixed a panic issue where the agent does not need to report secret when registering cluster. ([#6224](https://github.com/karmada-io/karmada/pull/6224), @jabellard)
- `karmada-controller-manager`: Fixed the issue that the gracefulEvictionTask of ResourceBinding can not be cleared in case of schedule fails. ([#6236](https://github.com/karmada-io/karmada/pull/6236), @XiShanYongYe-Chang)

#### Others
None.

# v1.11.7
## Downloads for v1.11.7

Download v1.11.7 in the [v1.11.7 release page](https://github.com/karmada-io/karmada/releases/tag/v1.11.7).

## Changelog since v1.11.6
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue where the `detector` unnecessary updates for RB issue. ([#6169](https://github.com/karmada-io/karmada/pull/6169), @CharlesQQ)
- `karmada-search`: Fixed the issue that namespaces in different ResourceRegistry might be overwritten. ([#6101](https://github.com/karmada-io/karmada/pull/6101), @JimDevil)

#### Others
- The base image `alpine` now has been promoted from 3.21.2 to 3.21.3. ([#6121](https://github.com/karmada-io/karmada/pull/6121))
- Karmada(release-1.11) now built with Golang v1.22.12. ([#6143](https://github.com/karmada-io/karmada/pull/6143), @sachinparihar)

# v1.11.6
## Downloads for v1.11.6

Download v1.11.6 in the [v1.11.6 release page](https://github.com/karmada-io/karmada/releases/tag/v1.11.6).

## Changelog since v1.11.5
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that newly created attached-ResourceBinding be mystically garbage collected. ([#6054](https://github.com/karmada-io/karmada/pull/6054), @whitewindmills)

#### Others
- The base image `alpine` now has been promoted from 3.21.0 to 3.21.2. ([#6038](https://github.com/karmada-io/karmada/pull/6038))
- Karmada(release-1.11) now built with Golang v1.22.11. ([#6070](https://github.com/karmada-io/karmada/pull/6070), @y1hao)

# v1.11.5
## Downloads for v1.11.5

Download v1.11.5 in the [v1.11.5 release page](https://github.com/karmada-io/karmada/releases/tag/v1.11.5).

## Changelog since v1.11.4
### Changes by Kind
#### Bug Fixes
- `karmada-webhook`: Fixed panic when validating ResourceInterpreterWebhookConfiguration with unspecified service port. ([#5966](https://github.com/karmada-io/karmada/pull/5966), @seanlaii)
- `karmada-controller-manager`: Fixed the issue of missing work queue metrics. ([#5981](https://github.com/karmada-io/karmada/pull/5981), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the bug of WorkloadRebalancer doesn't get deleted after TTL. ([#5994](https://github.com/karmada-io/karmada/pull/5994), @deefreak)

#### Others
None.

# v1.11.4
## Downloads for v1.11.4

Download v1.11.4 in the [v1.11.4 release page](https://github.com/karmada-io/karmada/releases/tag/v1.11.4).

## Changelog since v1.11.3
### Changes by Kind
#### Urgent Upgrade Notes
- The feature `Failover` now has been disabled by default, which should be explicitly enabled to avoid unexpected incidents. ([#5941](https://github.com/karmada-io/karmada/pull/5941), @XiShanYongYe-Chang)

If you are using the feature `Failover`, please enable it explicitly by adding the `--feature-gates=Failover=true,<other feature>` flag to the `karmada-controller-manager` component. If you are not using this feature, this change will have no impact.

#### Bug Fixes
- `karmadactl`: Fixed `karmada-metrics-adapter` use the incorrect certificate issue when deployed via karmadactl `init`. ([#5857](https://github.com/karmada-io/karmada/pull/5857), @KhalilSantana)
- `karmada-controller-manager`: Fixed the corner case where the reconciliation of aggregating status might be missed in case of component restart. ([5882](https://github.com/karmada-io/karmada/pull/5882), @liangyuanpeng)
- `karmada-controller-manager`: Fixed the problem of ResourceBinding remaining after the resource template is deleted in the dependencies distribution scenario. ([#5951](https://github.com/karmada-io/karmada/pull/5951), @XiShanYongYe-Chang)
- `karmada-scheduler`: Avoid filtering out clusters if the API enablement is incomplete during re-scheduling. ([#5930](https://github.com/karmada-io/karmada/pull/5930), @XiShanYongYe-Chang)

#### Others
- The base image `alpine` now has been promoted from `3.20.3` to `3.21.0`. ([#5919](https://github.com/karmada-io/karmada/pull/5919))

# v1.11.3
## Downloads for v1.11.3

Download v1.11.3 in the [v1.11.3 release page](https://github.com/karmada-io/karmada/releases/tag/v1.11.3).

## Changelog since v1.11.2
### Changes by Kind
#### Bug Fixes
- `karmadactl`: The `--force` option of `unjoin` command now try to clean up resources propagated in member clusters. ([#5848](https://github.com/karmada-io/karmada/pull/5848), @chaosi-zju)

#### Others
None.

# v1.11.2
## Downloads for v1.11.2

Download v1.11.2 in the [v1.11.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.11.2).

## Changelog since v1.11.1
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Ignored StatefulSet Dependencies with PVCs created via the VolumeClaimTemplates. ([#5686](https://github.com/karmada-io/karmada/pull/5686), @seanlaii)
- `karmada-scheduler`: Fixed unexpected modification of original `ResourceSummary` due to lack of deep copy. ([#5724](https://github.com/karmada-io/karmada/pull/5724), @RainbowMango)
- `karmada-scheduler`: Fixes an issue where resource model grades were incorrectly matched based on resource requests. Now only grades that can provide sufficient resources will be selected. ([#5728](https://github.com/karmada-io/karmada/pull/5728), @RainbowMango)
- `karmada-search`: Modify the logic of checking whether the resource is registered when selecting the plugin. ([#5737](https://github.com/karmada-io/karmada/pull/5737), @seanlaii)

#### Others
None.

# v1.11.1
## Downloads for v1.11.1

Download v1.11.1 in the [v1.11.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.11.1).

## Changelog since v1.11.0
### Changes by Kind
#### Bug Fixes
- `karmada-operator`: Fixed the issue where the manifests for the `karmada-scheduler` and `karmada-descheduler` components were not parsed correctly. ([#5550](https://github.com/karmada-io/karmada/pull/5550) @zhzhuang-zju)
- `karmadactl`：Fixed the issue where commands `create`, `annotate`, `delete`, `edit`, `label`, and `patch` cannot specify the namespace flag. ([#5513](https://github.com/karmada-io/karmada/pull/5513) @zhzhuang-zju)
- `karmadactl`: Fixed the issue that karmadactl addon failed to install karmada-scheduler-estimator due to unknown flag. ([#5538](https://github.com/karmada-io/karmada/pull/5538) @chaosi-zju)

#### Others
- The base image `alpine` now has been promoted from `alpine:3.20.2` to `alpine:3.20.3`.
- Karmada(release-1.11) now using Golang v1.22.7. ([#5531](https://github.com/karmada-io/karmada/pull/5531) @RainbowMango)

# v1.11.0
## Downloads for v1.11.0

Download v1.11.0 in the [v1.11.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.11.0).

## What's New

### Cluster-Level Resource Propagation Pause and Resume

This release provides a capability that supports the pause and resume of resource propagation at the cluster granularity, bringing more possibilities to developing, operating, and maintaining the system.

In some scenarios, Karmada users would like to control the timing of the synchronization of the above resource changes themselves, such as:

* As a developer, when the Karmada control plane competes with member clusters for control of resources, there is a situation where resources are repeatedly updated. Pausing the process of synchronizing the resource to the member clusters would be helpful to quickly locate the problem.

* As a release manager, this feature allows for control over which clusters receive application updates, thus achieving a rolling update cluster by cluster.

With the cluster-level resource propagation pause and resume capabilities, users will be able to better control the propagation of resources.

For a detailed description of this feature, see the [User Guide](https://karmada.io/docs/next/userguide/scheduling/resource-propagating/#suspend-and-resume-of-resource-propagation).

(Feature contributors: @a7i, @XiShanYongYe-Chang)

### Karmadactl Offers More Advanced Features

In this release, the Karmada community is dedicated to enhancing Karmadactl capabilities and improving its functionalities from multiple perspectives.

- Richer command set

Karmadactl has implemented new commands, such as `create`, `patch`, `delete`, `label`, `annotate`, `edit`, `attach`, `top node`,`api-resources` and `explain`, which allow users to perform more operations on resources in the Karmada control plane or member clusters.

- Richer capabilities

Karmadactl introduces the `--operation-scope` flag to control the scope of command operations. With the new flag, the commands `get`, `describe`, `exec`, and `explain` can operate on resources in the Karmada control plane or member clusters.

- More detailed command output information

The output of the `karmadactl get cluster` command now add the information of `Zones`, `Region`, `Provider`, `API-Endpoint`, and `Proxy-URL`.

With these capability enhancements, the operational experience of `karmadactl` could be improved. The new capabilities and more information about `karmadactl` can be obtained using `karmadactl --help`.

(Feature contributors: @hulizhe, @zhzhuang-zju, @whitewindmills, @a7i, @guozheng-shen, @grosser)

### Consistent generation semantics for multi-cluster workloads

In this release, Karmada has introduced consistent generation semantics for workloads running on multiple clusters. This update provides a reliable reference for release systems, enhancing the precision of multi-cluster deployments. By standardizing the generation semantics, Karmada simplifies the release process and ensures that workload statuses are consistently tracked, making it easier to manage and monitor applications across multiple clusters.

The following resource adaptations have been completed.

- GroupVersion: apps/v1
Kind: Deployment, DaemonSet, StatefulSet

- GroupVersion: apps.kruise.io/v1alpha1
Kind: CloneSet, DaemonSet

- GroupVersion: apps.kruise.io/v1beta1
Kind: StatefulSet

- GroupVersion: helm.toolkit.fluxcd.io/v2beta1
Kind: HelmRelease

- GroupVersion: kustomize.toolkit.fluxcd.io/v1
Kind: Kustomization

- GroupVersion: source.toolkit.fluxcd.io/v1
Kind: GitRepository

- GroupVersion: source.toolkit.fluxcd.io/v1beta2
Kind: Bucket, HelmChart, HelmRepository, OCIRepository

For a detailed description of this feature, see the [issue](https://github.com/karmada-io/karmada/issues/4870).

(Feature contributors: @yike21, @veophi, @whitewindmills, @liangyuanpeng, @zhy76)

### Karmada Operator Supports Custom CRD Download Strategy

CRD (Custom Resource Definition) resources are crucial prerequisite resources used by the Karmada operator for provisioning a new Karmada instance. 
This release `Karmada-Operator` Supports Custom CRD Download Strategy. With this, users can specify the download path for CRD resources and define more download strategies, providing richer and more configurable CRD download capabilities.

For a detailed description of this feature, see the [Proposal: Custom CRD Download Strategy Support for Karmada Operator](https://github.com/karmada-io/karmada/tree/master/docs/proposals/operator-custom-crd-download-strategy)

(Feature contributors: @jabellard)

## Other Notable Changes
### API Changes
- Introduced `Suspension` to the `PropagationPolicy/ClusterPropagationPolicy` API to provide a cluster-level resource propagation pause and resume capabilities. ([#4838](https://github.com/karmada-io/karmada/pull/4838), @a7i)
- Introduced `Dispatching` condition to the `Work` API to represent the dispatching status. ([#5317](https://github.com/karmada-io/karmada/pull/5317), @a7i)
- `ResourceInterpreterCustomization`: Added two additional printer columns, TARGET-API-VERSION and TARGET-KIND, to represent the target resource type, these columns will be displayed in the output of kubectl get. ([#5077](https://github.com/karmada-io/karmada/pull/5077), @a7i)
- `PropagationPolicy`/`ClusterPropagationPolicy`: Added two additional printer columns, `Conflict-Resolution` and `Priority`, to represent the conflict resolution strategy and priority, these columns will be displayed in the output of kubectl get. ([#5077](https://github.com/karmada-io/karmada/pull/5077), @a7i)
- Introduced `CRDTarball` to the `Karmada` API to supports custom CRD download strategy. ([#5185](https://github.com/karmada-io/karmada/pull/5185) @jabellard)

### Deprecation
- The following labels that were deprecated(replaced by `propagationpolicy.karmada.io/permanent-id` and `clusterpropagationpolicy.karmada.io/permanent-id`) at release-1.10 now have been removed:
    * propagationpolicy.karmada.io/namespace 
    * propagationpolicy.karmada.io/name 
    * clusterpropagationpolicy.karmada.io/name
- Specification of merics and health probe port parameters. Karmada introduced the `--metrics-bind-address` and `--health-probe-bind-address` flags and deprecated the following labels. This is a compatible change as the default values remain unchanged from previous versions. (contributors: @whitewindmills, @seanlaii, @liangyuanpeng)
    * The flags deprecated by `karmada-agent` are:
      --bind-address
      --secure-port
    * The flags deprecated by `karmada-controller-manager` are:
      --bind-address
      --secure-port
    * The flags deprecated by `karmada-descheduler` are:
      --bind-address
      --secure-port
    * The flags deprecated by `karmada-scheduler` are:
      --bind-address
      --secure-port
    * The flags deprecated by `karmada-scheduler-estimator` are:
      --bind-address
      --secure-port

### Bug Fixes
- `karmada-scheduler-estimator`: Fixed the `Unschedulable` result returned by plugins to be treated as an exception issue. ([#5012](https://github.com/karmada-io/karmada/pull/5012), @mszacillo)
- `karmada-controller-manager`: Fixed the issue that the cluster-status-controller overwrites the remedyActions field. ([#5030](https://github.com/karmada-io/karmada/pull/5030), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that the default resource interpreter doesn't accurately interpret the numbers of replicas. ([#5095](https://github.com/karmada-io/karmada/pull/5095), @whitewindmills)
- `karmada-controller-manager`: Fixed the issue of residual work in the MultiClusterService feature. ([#5188](https://github.com/karmada-io/karmada/pull/5188), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that status aggregation against the resource template might be missed due to slow cache sync. ([#5318](https://github.com/karmada-io/karmada/pull/5318), @chaosi-zju)
- `karmada-controller-manager`: Fixed the error of cluster status old condition update will overwrite the newest condition. ([#5227](https://github.com/karmada-io/karmada/pull/5227), @XiShanYongYe-Chang)
- `karmada-controller-manager`: work status sync when work dispatching is suspended. ([#5403](https://github.com/karmada-io/karmada/pull/5403), @a7i)
- `karmada-aggregated-apiserver`: User can append a "/" at the end when configuring the cluster's apiEndpoint. ([#5432](https://github.com/karmada-io/karmada/pull/5432), @spiritNO1)
- Correct `ClusterResourceBinding` scope in `MutatingWebhookConfiguration`. ([#5252](https://github.com/karmada-io/karmada/pull/5252), @a7i)

### Security
- `Security Enhancement`: Introduced TLS certificate authentication mechanism to secure gRPC connections to `karmada-scheduler-estimator`. ([#5040](https://github.com/karmada-io/karmada/pull/5040), @zhzhuang-zju)
    * The flags added to `karmada-scheduler-estimator` are:
      --grpc-auth-cert-file
      --grpc-auth-key-file 
      --grpc-client-ca-file 
      --insecure-skip-grpc-client-verify 
    * The flags added to `karmada-scheduler` are:
      --scheduler-estimator-ca-file
      --scheduler-estimator-key-file
      --scheduler-estimator-cert-file
      --insecure-skip-estimator-verify
    * The flags added to `karmada-descheduler` are:
      --scheduler-estimator-ca-file
      --scheduler-estimator-key-file
      --scheduler-estimator-cert-file
      --insecure-skip-estimator-verify
    The added filed don't require any adoption during the process of upgrading from a previous version of Karmada, but gives an optional and recommended wait to secure the gRPC connection.
- In this release, Karmada has invested significant effort in enhancing Karmada maturity based on [Clomonitor check sets](https://clomonitor.io/docs/topics/checks/). So far, we have achieved a score of [99](https://clomonitor.io/projects/cncf/karmada), and the last check (Signed releases) will be passed after 5 releases. This indicates that the Karmada community has made great strides in both security and maturity. (contributors: @zhzhuang-zju, @B1F030, @adiya7302, @Akash-Singh04, @RainbowMango)
- Introduced [SBOM](https://www.aquasec.com/cloud-native-academy/supply-chain-security/sbom/) to release assests to enhance transparency around Karmada components and dependencies, bolster the security posture of our project. The composition and usage of SBOM can refer to [SBOM DOC](https://karmada.io/docs/next/administrator/security/verify-artifacts#sbom). ([#5110](https://github.com/karmada-io/karmada/pull/5110), @zhzhuang-zju)
- Introduced [SLSA provenance file](https://slsa.dev/spec/v0.1/) to release assets, with which Karmada users verify the artifacts to prevent counterfeiting. ([#5178](https://github.com/karmada-io/karmada/pull/5178), @zhzhuang-zju)

### Features & Enhancements
- `karmada-controller-manager`: Added work namespace/name annotation in the endpointslice resources to explain which work is associated. ([#5042](https://github.com/karmada-io/karmada/pull/5042), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Mark `.status.observedGeneration` of Deployment with `.metadata.generation` only when all members' statuses are algined with its resource template generation. ([#4867](https://github.com/karmada-io/karmada/pull/4867), @veophi)
- `karmada-controller-manager`: Mark `.status.observedGeneration` of Kustomization with `.metadata.generation` only when all members' statuses are algined with its resource template generation. ([#5084](https://github.com/karmada-io/karmada/pull/5084), @yike21)
- `karmada-controller-manager`: Mark `.status.observedGeneration` of StatefulSet with `.metadata.generation` only when all members' statuses are algined with its resource template generation. ([#5094](https://github.com/karmada-io/karmada/pull/5094), @zhy76)
- `karmada-controller-manager`: Mark `.status.observedGeneration` of GitRepository with `.metadata.generation` only when all members' statuses are algined with its resource template generation. ([#5086](https://github.com/karmada-io/karmada/pull/5086), @yike21)
- `karmada-controller-manager`: Mark `.status.observedGeneration` of CloneSet with `.metadata.generation` only when all members' statuses are algined with its resource template generation. ([#5057](https://github.com/karmada-io/karmada/pull/5057), @veophi)
- `karmada-controller-manager`: Mark `.status.observedGeneration` of DaemonSet with `.metadata.generation` only when all members' statuses are algined with its resource template generation. ([#5165](https://github.com/karmada-io/karmada/pull/5165), @whitewindmills)
- `karmada-controller-manager`: Mark `.status.observedGeneration` of daemonsets.apps.kruise.io with `.metadata.generation` only when all members' statuses are algined with its resource template generation. ([#5167](https://github.com/karmada-io/karmada/pull/5167), @whitewindmills)
- `karmada-controller-manager`: Mark `.status.observedGeneration` of StatefulSet with `.metadata.generation` only when all members' statuses are algined with its resource template generation. ([#5204](https://github.com/karmada-io/karmada/pull/5204), @liangyuanpeng)
- `karmada-controller-manager`: Mark `.status.observedGeneration` of helmrepositories.source.toolkit.fluxcd.io with `.metadata.generation` only when all members' statuses are algined with its resource template generation. ([#5196](https://github.com/karmada-io/karmada/pull/5196), @yike21)
- `karmada-controller-manager`: Mark `.status.observedGeneration` of buckets.source.toolkit.fluxcd.io with `.metadata.generation` only when all members' statuses are algined with its resource template generation. ([#5193](https://github.com/karmada-io/karmada/pull/5193), @yike21)
- `karmada-controller-manager`: Mark `.status.observedGeneration` of ocirepositories.source.toolkit.fluxcd.io with `.metadata.generation` only when all members' statuses are algined with its resource template generation. ([#5197](https://github.com/karmada-io/karmada/pull/5197), @yike21)
- `karmada-controller-manager`: Mark `.status.observedGeneration` of helmcharts.source.toolkit.fluxcd.io with `.metadata.generation` only when all members' statuses are algined with its resource template generation. ([#5194](https://github.com/karmada-io/karmada/pull/5194), @yike21)
- `karmada-controller-manager`: Mark `.status.observedGeneration` of helmreleases.helm.toolkit.fluxcd.io with `.metadata.generation` only when all members' statuses are algined with its resource template generation. ([#5311](https://github.com/karmada-io/karmada/pull/5311), @yike21)
- `karmada-controller-manager`: Add health probe argument `health-probe-bind-address`, and deprecate `--bind-address` and `--secure-port` flag. ([#5290](https://github.com/karmada-io/karmada/pull/5290), @seanlaii)
- `karmadactl`: Renamed join command options from --host-as-* to --karmada-as-*. ([#5099](https://github.com/karmada-io/karmada/pull/5099), @grosser)
- `karmadactl`: Expose the metrics port for PodMonitor. ([#5169](https://github.com/karmada-io/karmada/pull/5169), @whitewindmills)
- `karmadactl`: Add the reserved label `karmada.io/system` to resources created by the `join` command. ([#4620](https://github.com/karmada-io/karmada/pull/4620), @a7i)
- `karmadactl`: Introduced `--ca-cert-file` and `--ca-key-file` flags to `init` command to specify the root CA which will be used to issue the certificate for components. ([#5127](https://github.com/karmada-io/karmada/pull/5127), @guozheng-shen)
- `karmadactl`: The `get` command can show Karmada resources now. ([#5254](https://github.com/karmada-io/karmada/pull/5254), @hulizhe)
- `karmadactl`: The `describe` command can show details of Karmada resources now. ([#5392](https://github.com/karmada-io/karmada/pull/5392), @hulizhe)
- `karmadactl`: The `exec` command can execute a command in a Karmada container now. ([#5398](https://github.com/karmada-io/karmada/pull/5398), @hulizhe)
- `karmadactl`: add new command `top node` to display resource (CPU/memory) usage of nodes in member clusters. ([#4224](https://github.com/karmada-io/karmada/pull/4224), @zhzhuang-zju)
- `karmadactl`: add new command `create` to create a resource from a file or from stdin in Karmada control plane. ([#5399](https://github.com/karmada-io/karmada/pull/5399), @hulizhe)
- `karmadactl`: add new command `attach` to attach to a running container in Karmada control plane or a member cluster. ([#5395](https://github.com/karmada-io/karmada/pull/5395), @hulizhe)
- `karmadactl`: add new command `api-resources` to print the supported API resources on the server in Karmada control plane or a member cluster. ([#5394](https://github.com/karmada-io/karmada/pull/5394), @hulizhe)
- `karmadactl`: add new command `api-versions` to print the supported API versions on the server in Karmada control plane or a member cluster. ([#5394](https://github.com/karmada-io/karmada/pull/5394), @hulizhe)
- `karmadactl`: add new command `karmadactl explain` to describe fields and structure of various resources in Karmada control plane or a member cluster. ([#5393](https://github.com/karmada-io/karmada/pull/5393), @hulizhe)
- `karmadactl`: add new command `karmadactl delete` to delete resources. ([#5431](https://github.com/karmada-io/karmada/pull/5431), @zhzhuang-zju)
- `karmadactl`: add new command `karmadactl edit` to edit a resource on the server. ([#5434](https://github.com/karmada-io/karmada/pull/5434), @zhzhuang-zju)
- `karmadactl`: add new command `label` to update the labels on a resource. ([#5453](https://github.com/karmada-io/karmada/pull/5453), @zhzhuang-zju)
- `karmadactl`: add new command `annotate` to update the annotations on a resource. ([#5458](https://github.com/karmada-io/karmada/pull/5458), @zhzhuang-zju)
- `karmadactl`: add new command `patch`  to update fields of a resource. ([#5463](https://github.com/karmada-io/karmada/pull/5463), @zhzhuang-zju)
- `karmada-operator`: Introduced `--metrics-bind-address` and `--health-probe-bind-address` flags, it's a compatible change as the default value does not change from previous versions. ([#5174](https://github.com/karmada-io/karmada/pull/5174), @whitewindmills)
- `karmada-operator`: Introduced CRD download strategy that allows downloading CRD from a private source. ([#5185](https://github.com/karmada-io/karmada/pull/5185), @jabellard)
- `karmada-scheduler`: GroupClusters will sort clusters by score and availableReplica count. ([#5144](https://github.com/karmada-io/karmada/pull/5144), @mszacillo)
- `karmada-scheduler`: Add health probe argument `health-probe-bind-address` and metrics argument `metrics-bind-address`. Deprecate `--bind-address` and `--secure-port` flags. ([#5437](https://github.com/karmada-io/karmada/pull/5437), @liangyuanpeng)
- `karmada-webhook`: changed "app" label from mutating-config/validating-config to karmada-webhook to make them identifiyable. ([#5246](https://github.com/karmada-io/karmada/pull/5246), @grosser)
- `karmada-webhook`: Remove the limit of 63 name lengths with PropagationPolicy/ClusterPropagationPolicy resource. ([#5029](https://github.com/karmada-io/karmada/pull/5029), @XiShanYongYe-Chang)
- `karmada-agent`: Add health probe argument `health-probe-bind-address`, deprecate `--bind-address` and `--secure-port` flag. ([#5223](https://github.com/karmada-io/karmada/pull/5223), @whitewindmills)
- `karmada-scheduler-estimator`: Add health probe argument `health-probe-bind-address` and metrics argument `metrics-bind-address`. Deprecate `--bind-address` and `--secure-port` flags. ([#5273](https://github.com/karmada-io/karmada/pull/5273), @seanlaii)
- `karmada-descheduler`: Add health probe argument `health-probe-bind-address` and metrics argument `metrics-bind-address`. Deprecate `--bind-address` and `--secure-port` flags. ([#5435](https://github.com/karmada-io/karmada/pull/5435), @whitewindmills)
- Expose the metrics port for the karmada-controller-manager, scheduler、agent、karmada-webhook、descheduler and scheduler-estimator in local-up-karmada. ([#5428](https://github.com/karmada-io/karmada/pull/5428), @dzcvxe)
- Expose the metrics port for the karmada-controller-manager, scheduler、karmada-webhook and descheduler in operator installation. ([#5465](https://github.com/karmada-io/karmada/pull/5465), @chaosi-zju)
- Expose the default port for the karmada-controller-manager, scheduler and agent when creating a PodMonitor. ([#5139]https://github.com/karmada-io/karmada/pull/5139, wangxf1987)
- Added generic handling of priorityclass and namespace for default flinkdeployment interpreter. ([#5215](https://github.com/karmada-io/karmada/pull/5215), @mszacillo)
- add karmada.io/system=true label to newly created karmada-es-* namespaces. ([#5243](https://github.com/karmada-io/karmada/pull/5243), @grosser)
- add karmada.io/system=true label to internally created karmada cluster-roles and cluster-role-bindings. ([#5281](https://github.com/karmada-io/karmada/pull/5281), @grosser)
- cluster-level resource propagation pause and resume capabilities. ([#4838](https://github.com/karmada-io/karmada/pull/4838), @a7i)
- Adding FlinkDeployment v1beta1 CRD to supported third party resource customizatons. ([#5023](https://github.com/karmada-io/karmada/pull/5023), @mszacillo)

## Other
### Dependencies
- Karmada is now built with Go1.22.4. ([#5015](https://github.com/karmada-io/karmada/pull/5015), @grosser)
- karmada-apiserver and kube-controller-manager is using v1.28.9 by default. ([#5065](https://github.com/karmada-io/karmada/pull/5065), @liangyuanpeng)
- The base image `alpine` now has been promoted from `alpine:3.20.0` to `alpine:3.20.1`.
- karmada-apiserver and kube-controller-manager is using v1.29.6 by default. ([#5209](https://github.com/karmada-io/karmada/pull/5209), @liangyuanpeng)
- The base image `alpine` now has been promoted from `alpine:3.20.1` to `alpine:3.20.2`.
- Karmada is now built with Go1.22.6. ([#5335](https://github.com/karmada-io/karmada/pull/5335), @RainbowMango)

### Helm Charts
- helm install karmada components in order to reduce components crashing during the installation. ([#5010](https://github.com/karmada-io/karmada/pull/5010) @chaosi-zju)
- set karmada-metrics-adapter image pull policy to karmadaImagePullPolicy, in order to keep the same as other components. ([#5113](https://github.com/karmada-io/karmada/pull/5113), @chaosi-zju)
- expose metrics port for helm installation. ([#5168](https://github.com/karmada-io/karmada/pull/5168), @whitewindmills)
- ignore the static-resource Pod in the post-install check. ([#5369](https://github.com/karmada-io/karmada/pull/5369), @iawia002)
- Support custom cluster service CIDR in the helm chart. ([#5379](https://github.com/karmada-io/karmada/pull/5379), @iawia002)
- Fix the creation condition of metrics-adapter related APIService. ([#5378](https://github.com/karmada-io/karmada/pull/5378), @iawia002)
- fix controller can't restart in helm for dependent secret not found. ([#5305](https://github.com/karmada-io/karmada/pull/5305), @chaosi-zju)
- automatically clean up the static-resource Job after it completes. ([#5442](https://github.com/karmada-io/karmada/pull/5442), @iawia002)

### Instrumentation
- `karmada-controller-manager`: add metrics `recreate_resource_to_cluster` and `update_resource_to_cluster` for recreate/update resource event when sync work status. ([#5247](https://github.com/karmada-io/karmada/pull/5247), @chaosi-zju)

## Contributors
Thank you to everyone who contributed to this release!

Users whose commits are in this release (alphabetically by username)

- @08AHAD
- @a7i
- @aditya7302
- @Affan-7
- @Akash-Singh04
- @anujagrawal699 
- @B1F030 
- @chaosi-zju
- @dzcvxe
- @grosser
- @guozheng-shen
- @hulizhe
- @iawia002
- @mohamedawnallah
- @mszacillo
- @NishantBansal2003
- @jabellard
- @khanhtc1202
- @liangyuanpeng
- @qinguoyi
- @RainbowMango
- @renxiangyu_yewu
- @seanlaii
- @spiritNO1
- @tiansuo114
- @varshith257
- @veophi
- @wangxf1987
- @whitewindmills
- @xiaoloongfang 
- @XiShanYongYe-Chang
- @xovoxy
- @Yash Pandey
- @yike21
- @zhy76
- @zhzhuang-zju
