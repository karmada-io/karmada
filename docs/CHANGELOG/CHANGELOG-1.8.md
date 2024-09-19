<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.8.7](#v187)
  - [Downloads for v1.8.7](#downloads-for-v187)
  - [Changelog since v1.8.6](#changelog-since-v186)
    - [Changes by Kind](#changes-by-kind)
      - [Bug Fixes](#bug-fixes)
      - [Others](#others)
- [v1.8.6](#v186)
  - [Downloads for v1.8.6](#downloads-for-v186)
  - [Changelog since v1.8.5](#changelog-since-v185)
    - [Changes by Kind](#changes-by-kind-1)
      - [Bug Fixes](#bug-fixes-1)
      - [Others](#others-1)
- [v1.8.5](#v185)
  - [Downloads for v1.8.5](#downloads-for-v185)
  - [Changelog since v1.8.4](#changelog-since-v184)
    - [Changes by Kind](#changes-by-kind-2)
      - [Bug Fixes](#bug-fixes-2)
      - [Others](#others-2)
- [v1.8.4](#v184)
  - [Downloads for v1.8.4](#downloads-for-v184)
  - [Changelog since v1.8.3](#changelog-since-v183)
    - [Changes by Kind](#changes-by-kind-3)
      - [Bug Fixes](#bug-fixes-3)
      - [Others](#others-3)
- [v1.8.3](#v183)
  - [Downloads for v1.8.3](#downloads-for-v183)
  - [Changelog since v1.8.2](#changelog-since-v182)
    - [Changes by Kind](#changes-by-kind-4)
      - [Bug Fixes](#bug-fixes-4)
      - [Others](#others-4)
- [v1.8.2](#v182)
  - [Downloads for v1.8.2](#downloads-for-v182)
  - [Changelog since v1.8.1](#changelog-since-v181)
    - [Changes by Kind](#changes-by-kind-5)
      - [Bug Fixes](#bug-fixes-5)
      - [Others](#others-5)
- [v1.8.1](#v181)
  - [Downloads for v1.8.1](#downloads-for-v181)
  - [Changelog since v1.8.0](#changelog-since-v180)
    - [Changes by Kind](#changes-by-kind-6)
      - [Bug Fixes](#bug-fixes-6)
      - [Others](#others-6)
- [v1.8.0](#v180)
  - [Downloads for v1.8.0](#downloads-for-v180)
  - [What's New](#whats-new)
    - [Multi Cluster Service](#multi-cluster-service)
    - [More balanced weight scheduling](#more-balanced-weight-scheduling)
    - [Security Enhancements](#security-enhancements)
    - [Resource Deletion Protection](#resource-deletion-protection)
  - [Other Notable Changes](#other-notable-changes)
    - [API Changes](#api-changes)
    - [Deprecation](#deprecation)
    - [Bug Fixes](#bug-fixes-7)
    - [Security](#security)
    - [Features & Enhancements](#features--enhancements)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Charts](#helm-charts)
    - [Instrumentation](#instrumentation)
  - [Contributors](#contributors)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.8.7
## Downloads for v1.8.7

Download v1.8.7 in the [v1.8.7 release page](https://github.com/karmada-io/karmada/releases/tag/v1.8.7).

## Changelog since v1.8.6
### Changes by Kind
#### Bug Fixes
None.

#### Others
- The base image `alpine` now has been promoted from `alpine:3.20.1` to `alpine:3.20.2`. ([#5270](https://github.com/karmada-io/karmada/pull/5270))

# v1.8.6
## Downloads for v1.8.6

Download v1.8.6 in the [v1.8.6 release page](https://github.com/karmada-io/karmada/releases/tag/v1.8.6).

## Changelog since v1.8.5
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: fix the issue of residual work in the MultiClusterService feature. ([#5213](https://github.com/karmada-io/karmada/pull/5213), @XiShanYongYe-Chang)

#### Others
None.

# v1.8.5
## Downloads for v1.8.5

Download v1.8.5 in the [v1.8.5 release page](https://github.com/karmada-io/karmada/releases/tag/v1.8.5).

## Changelog since v1.8.4
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that the default resource interpreter doesn't accurately interpret the numbers of replicas. ([#5106](https://github.com/karmada-io/karmada/pull/5108), @whitewindmills)

#### Others
- The base image `alpine` now has been promoted from `alpine:3.20.0` to `alpine:3.20.1`. ([#5088](https://github.com/karmada-io/karmada/pull/5093))

# v1.8.4
## Downloads for v1.8.4

Download v1.8.4 in the [v1.8.4 release page](https://github.com/karmada-io/karmada/releases/tag/v1.8.4).

## Changelog since v1.8.3
### Changes by Kind
#### Bug Fixes
None.

#### Others
- The base image `alpine` now has been promoted from `alpine:3.19.1` to `alpine:3.20.0`.

# v1.8.3
## Downloads for v1.8.3

Download v1.8.3 in the [v1.8.3 release page](https://github.com/karmada-io/karmada/releases/tag/v1.8.3).

## Changelog since v1.8.2
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed incorrect annotation markup when policy preemption occurs. ([#4770](https://github.com/karmada-io/karmada/pull/4770), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fix the problem that labels cannot be deleted via Karmada propagation. ([#4791](https://github.com/karmada-io/karmada/pull/4791), @whitewindmills)
- `karmada-controller-manager`: Fix the problem that work.karmada.io/permanent-id constantly changes with every update. ([#4820](https://github.com/karmada-io/karmada/pull/4820), @XiShanYongYe-Chang)

#### Others
None.

# v1.8.2
## Downloads for v1.8.2

Download v1.8.2 in the [v1.8.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.8.2).

## Changelog since v1.8.1
### Changes by Kind
#### Bug Fixes
- `karmada-search`: Add the logic of checking whether the resource API to be retrieved is installed in the cluster. ([#4588](https://github.com/karmada-io/karmada/pull/4588), @XiShanYongYe-Chang)
- `karmada-search`: support accept content type `as=Table` in the proxy global resource function. ([#4594](https://github.com/karmada-io/karmada/pull/4594), @jwcesign)

#### Others
- The base image `alpine` now has been promoted from `alpine:3.18.5` to `alpine:3.19.1`. ([#4599](https://github.com/karmada-io/karmada/pull/4599), @Fish-pro)

# v1.8.1
## Downloads for v1.8.1

Download v1.8.1 in the [v1.8.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.8.1).

## Changelog since v1.8.0
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: fix incorrect `forType` in `cluster-resource-binding-status controller` from `ResouceBinding` to `ClusterResourceBinding`. ([#4365](https://github.com/karmada-io/karmada/pull/4365), @lxtywypc)
- `karmadactl`: Fixed return err in case of `secret.spec.caBundle` is nil. ([#4391](https://github.com/karmada-io/karmada/pull/4391), @XiShanYongYe-Chang)

#### Others
- Karmada is now built with Go1.20.12.  ([#4397](https://github.com/karmada-io/karmada/pull/4397), @RainbowMango)

# v1.8.0
## Downloads for v1.8.0

Download v1.8.0 in the [v1.8.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.8.0).

## What's New

### Multi Cluster Service

With the newly designed `MultiClusterService` API,  users can easily share services across clusters and the services can be consumed directly with the domain name.

See [Service discovery with native Kubernetes naming and resolution](https://github.com/karmada-io/karmada/tree/master/docs/proposals/service-discovery) for more details.

(Feature contributors: @bivas, @XiShanYongYe-Chang, @jwcesign, @Rains6)

### More balanced weight scheduling

With the optimized static weight replica assignment algorithm, the replicas can be evenly scheduled to desired clusters, this guarantees that resource consumption in large-scale scenarios is more balanced.

See [Divide replicas by static weight evenly](https://github.com/karmada-io/karmada/blob/master/docs/proposals/scheduling/replicas-assign/divide-replicas-by-static-weight-evenly.md) for more details.

(Feature contributors: @chaosi-zju)

### Security Enhancements

This release significantly enhanced the project's security including the removal of insecure component access methods(see [#4024](https://github.com/karmada-io/karmada/issues/4024)), ensuring all components deployed by installation tools have secure configurations by default(see [#4191](https://github.com/karmada-io/karmada/issues/4191)), and so on.

(Feature contributor: @chaosi-zju, @yanfeng1992, @zhzhuang-zju)

### Resource Deletion Protection

Any resources deployed through Karmada can be protected to avoid accidental deletion. The resource template deployed in Karmada may exist widely across multiple clusters, accidental deletion of resource template can lead to serious accidents, such as accidental deletion of namespaces may cause all resources in the namespace to be cascaded removed. To eliminate this concern, this feature provides protection policies for any resource type, protected resources are not allowed to be removed.

See [Resource Deletion Protection](https://github.com/karmada-io/karmada/tree/master/docs/proposals/resource-deletion-protection) for more details.

(Feature contributor: @Vacant2333, @XiShanYongYe-Chang)

## Other Notable Changes
### API Changes
- `MultiClusterService`: Introduced `ServiceProvisionClusters` and `ServiceConsumptionClusters` which will be used to specify service source and consumption place. ([#4290](https://github.com/karmada-io/karmada/pull/4290), @jwcesign)

### Deprecation
- The following label on `resource template` now has been deprecated and will be removed from the following releases:
  * `propagationpolicy.karmada.io/name` replaced by `propagationpolicy.karmada.io/permanent-id`
  * `propagationpolicy.karmada.io/namespace` replaced by `propagationpolicy.karmada.io/permanent-id`
  * `clusterpropagationpolicy.karmada.io/name` replaced by `clusterpropagationpolicy.karmada.io/permanent-id`
  * `propagationpolicy.karmada.io/uid` replaced by `propagationpolicy.karmada.io/permanent-id`
  * `clusterpropagationpolicy.karmada.io/uid` replaced by `clusterpropagationpolicy.karmada.io/permanent-id`
    These labels were used to refer to `PropagationPolicy` or `ClusterPropagationPolicy`, now they are replaced by the permanent label.

### Bug Fixes
- `karmada-aggregated-apiserver`: Fixed the issue that can not proxy `exec` request to a proxy issue. ([#4020](https://github.com/karmada-io/karmada/pull/4020), @XiShanYongYe-Chang)
- `karmada-aggregated-apiserver`: Fix exec failure due to incorrupt TLS config. ([#4206](https://github.com/karmada-io/karmada/pull/4206), @jwcesign)
- `karmada-operator`: Fixed the issue that karmada-metrics-adapter was not removed after deleting a Karmada instance. ([#4056](https://github.com/karmada-io/karmada/pull/4056), @wawa0210)
- `karmada-operator`: Fixed can not load Karmada v1.7.0 crds issue. ([#4130](https://github.com/karmada-io/karmada/pull/4130), @liangyuanpeng)
- `karmada-operator`: Fix karmada instance aggregated service externalName and namespace error. ([#4210](https://github.com/karmada-io/karmada/pull/4210), @wawa0210)
- `karmada-operator`: Fixed an issue that components can not be deployed due to invalid manifests. ([#4318](https://github.com/karmada-io/karmada/pull/4318), @calvin0327)
- `karmada-operator`: resolve resource version conflict when updating service. ([#4320](https://github.com/karmada-io/karmada/pull/4320), @calvin0327)
- `karmada-webhook`: Fixed to validate spec.types of MultiClusterService API. ([#4096](https://github.com/karmada-io/karmada/pull/4096), @lonelyCZ)
- `karmada-controller-manager`: Fix panic when FederatedHPA's SelectPolicy is nil and FederatedHPA webhook is disabled. ([#4103](https://github.com/karmada-io/karmada/pull/4103), @jwcesign)
- `karmada-controller-manager`: Fixed a panic issue due to `index out of range` in resource model functionality. ([#4145](https://github.com/karmada-io/karmada/pull/4145), @halfrost)
- `karmada-controller-manager`: Fixed inconsistent generation issue between `metadata.generation` and `status.observedGeneration`. ([#4138](https://github.com/karmada-io/karmada/pull/4138), @jwcesign)
- `karmada-controller-manager`: only update `aggregated status` and `conditions` fields during `binding-status controller` updating status of binding. ([#4226](https://github.com/karmada-io/karmada/pull/4226), @lxtywypc)
- `karmada-controller-manager`: Fix the bug that losing the chance to unclaim resource template in case of PropagationPolicy/ClusterPropagationPolicy is removed. ([#4245](https://github.com/karmada-io/karmada/pull/4245), @whitewindmills)
- `karmada-search`: Fixed can not access ResourceRegistry issue due to misconfigured singular name. ([#4144](https://github.com/karmada-io/karmada/pull/4144), @zhzhuang-zju)
- `karmada-search`: Fixed the ResourceRegistry is incompatible in the upgrade scenario. ([#4171](https://github.com/karmada-io/karmada/pull/4171), @zhzhuang-zju)
- `karmada-search`: Fix lock race affects watch RestChan not close, causing client watch api to hang. ([#4212](https://github.com/karmada-io/karmada/pull/4212), @xigang)

### Security
- Introduced `trivy` for image security scanning. ([#4123](https://github.com/karmada-io/karmada/pull/4123), @zhzhuang-zju)
- bump golang.org/x/net to v0.17.0 to address CVE(CVE-2023-39325, CVE-2023-44487) concerns. ([#4167](https://github.com/karmada-io/karmada/pull/4167), @RainbowMango)
- Bump golang.org/grpc to v1.56.3 to address security concerns(GHSA-m425-mq94-257g). ([#4196](https://github.com/karmada-io/karmada/pull/4196), @zhzhuang-zju)
- `karmadactl`: The `karmada-apiserver` and `karmada-aggregated-apiserver` installed by the `init` command will take `--tls-min-version=VersionTLS13` by default. ([#4181](https://github.com/karmada-io/karmada/pull/4181), @yanfeng1992)
- `karmadactl`: The `karmada-search` and `karmada-metrics-adapter` installed by the `addon` command will take `--tls-min-version=VersionTLS13` by default. ([#4193](https://github.com/karmada-io/karmada/pull/4193), @yanfeng1992)
- The`karmada-apiserver`, `karmada-aggregated-apiserver`, `karmada-search`, and `karmada-metrics-adapter` installed by `karmada-operator` and `chart` will take `--tls-min-version=VersionTLS13` by default. ([#4198](https://github.com/karmada-io/karmada/pull/4198), @zhzhuang-zju)
- Fixed CVE-2016-2183 by setting Golang's secure cipher suites to ETCD's cipher suites. ([#4253](https://github.com/karmada-io/karmada/pull/4253), @zhzhuang-zju)

### Features & Enhancements
- `karmadactl`: The `init` now can setup production a production-ready HA control plane by using an external HA ETCD cluster. ([#3898](https://github.com/karmada-io/karmada/pull/3898), @tedli)
- `karmadactl`: Introduced `--cluster-zones` flag to `register` command to declare cluster zones during registering a cluster. ([#4069](https://github.com/karmada-io/karmada/pull/4069), @MingZhang-YBPS)
- `karmadactl`: Introduced `--proxy-server-address` flag to `register` command to declare proxy server during registering a cluster. ([#4076](https://github.com/karmada-io/karmada/pull/4076), @yanfeng1992)
- `karmadactl`: Introduced `--cluster` flag to the `top` command to specify the cluster name. ([#4223](https://github.com/karmada-io/karmada/pull/4223), @zhzhuang-zju)
- `karmadactl`: Introduced `--dependencies` flag to `promote` command, which will be used to promote dependencies. ([#4135](https://github.com/karmada-io/karmada/pull/4135), @zhy76)
- `karmada-controller-manager`: Now the `currentReplicas` and `desiredReplicas` in the status of HPA will be aggregated to the resource template by default. ([#4064](https://github.com/karmada-io/karmada/pull/4064), @chaosi-zju)
- `karmada-controller-manager`: Introduced `hpaReplicasSyncer` controller which syncs workload's replicas from the member cluster to the control plane. ([#4072](https://github.com/karmada-io/karmada/pull/4072), @lxtywypc)
- `karmada-controller-manager`: Now the replicas of deployments can be automatically retained if it is scaling with an HPA. ([#4078](https://github.com/karmada-io/karmada/pull/4078), @chaosi-zju)
- `karmada-controller-manager`: Fix panic caused by concurrent global variable reads and writes. ([#4176](https://github.com/karmada-io/karmada/pull/4176), @jwcesign)
- `karmada-controller-manager`: Ignored `configmap/extension-apiserver-authentication` from propagation. ([#4237](https://github.com/karmada-io/karmada/pull/4237), @zhzhuang-zju)
- `karmada-controller-manager`: Pruned job labels `batch.kubernetes.io/controller-uid` and `batch.kubernetes.io/job-name` which were introduced by Kubernetes 1.27. ([#4160](https://github.com/karmada-io/karmada/pull/4160), @liangyuanpeng)
- `karmada-controller-manager`: Introduced permanent ID to PropagationPolicy, ClusterPropagationPolicy, ResourceBinding, ClusterResourceBinding, and Work. ([#4199](https://github.com/karmada-io/karmada/pull/4199), @jwcesign)
- `karmada-controller-manager`: Resource models now support any arbitrary resource type and are no longer limited to `cpu`, `memory`, `storage`, and `ephemeral-storage`. ([#4307](https://github.com/karmada-io/karmada/pull/4307), @chaosi-zju)
- `karmada-controller-manager`: Introduced `multiclusterservice` controller to sync MultiClusterService. ([#4323](https://github.com/karmada-io/karmada/pull/4323), @Rains6)
- `karmada-agent`: Introduced `--cluster-zones` flag to declare cluster zones during registering a cluster. ([#4069](https://github.com/karmada-io/karmada/pull/4069), @MingZhang-YBPS)
- `karmada-scheduler`: Introduced rate limiter options including: `--rate-limiter-base-delay`, `--rate-limiter-max-delay`, `--rate-limiter-qps`, and `--rate-limiter-bucket-size`, the default value not changed compared to the previous version. ([#4105](https://github.com/karmada-io/karmada/pull/4105), @yanfeng1992)
- `karmada-scheduler`: Fixed an issue that the scheduler does not ignore RB/CRB by scheduler name during enqueuing. ([#4139](https://github.com/karmada-io/karmada/pull/4139), @yanfeng1992)
- `karmada-scheduler`: Introduced leaderElection options including: `--leader-elect-lease-duration`, `--leader-elect-renew-deadline`, `--leader-elect-retry-period`, the default value not changed compared to the previous version. ([#4158](https://github.com/karmada-io/karmada/pull/4158), @yanfeng1992)
- `karmada-scheduler`: The replicas now can be evenly distributed across clusters in case of scheduling replicas based on static weight. ([#4219](https://github.com/karmada-io/karmada/pull/4219), @chaosi-zju)
- `karmada-aggregated-apiserver`: Add ca to check the validation of the member clusters' server certificate. ([#4183](https://github.com/karmada-io/karmada/pull/4183), @jwcesign)
- `karmada-aggregated-apiserver`: Supports cross-cluster unified query. ([#4254](https://github.com/karmada-io/karmada/pull/4254), @chaunceyjiang)
- `karmada-operator`: Support config required resources for each Karmada component. ([#4059](https://github.com/karmada-io/karmada/pull/4059), @wawa0210)
- `karmada-operator`: Attached kubeconfig file into status(.status.secretRef) after installation. ([#3789](https://github.com/karmada-io/karmada/pull/3789), @calvin0327)
- `karmada-operator`: The default installation version of Karmada now is v1.7. ([#4127](https://github.com/karmada-io/karmada/pull/4127), @MolisXYliu)
- `karmada-operator`: The default installed version of Karmada now depends on the operator version. ([#4133](https://github.com/karmada-io/karmada/pull/4133), @chaosi-zju)
- `karmada operator`: Now able to install karmada on a remote cluster. ([#3926](https://github.com/karmada-io/karmada/pull/3926), @calvin0327)

## Other
### Dependencies
- Bump Golang version to Go1.20.10. ([#4169](https://github.com/karmada-io/karmada/pull/4169), @vikash485)
- Karmada(v1.8) is now built with Go1.20.11. ([#4334](https://github.com/karmada-io/karmada/pull/4334), @CoderTH)
- Kubernetes dependencies now has been bumped to v1.27.8. ([#4328](https://github.com/karmada-io/karmada/pull/4328), @RainbowMango)

### Helm Charts
- `helm-chart`: Fixed the issue that can not install `karmada-search`. ([#4034](https://github.com/karmada-io/karmada/pull/4034), @chaosi-zju)
- `Helm chart`: Added helm index for 1.7 release. ([#4051](https://github.com/karmada-io/karmada/pull/4051), @a7i)
- `Helm chart`: Added helm index for 1.7.1 release. ([#4178](https://github.com/karmada-io/karmada/pull/4178), @a7i)
- Fixed render issue of podDisruptionBudget in controller-manager. ([#4188](https://github.com/karmada-io/karmada/pull/4188), @a7i)
- Now able to install the component `karmada-metrics-adapter` with the chart. ([#4303](https://github.com/karmada-io/karmada/pull/4303), @jwcesign)

### Instrumentation
- `karmada-controller-manager`: Skip emitting the `ApplyPolicyFailed` event for the resource template which does not match any PropagationPolicy. ([#4052](https://github.com/karmada-io/karmada/pull/4052), @wu0407)

## Contributors
Thank you to everyone who contributed to this release!

Users whose commits are in this release (alphabetically by username)
- @22RC
- @a7i
- @calvin0327
- @chaosi-zju
- @chaunceyjiang
- @CoderTH
- @codrAlxx
- @huangyutongs
- @halfrost
- @JadeFlute0127
- @jwcesign
- @liangyuanpeng
- @lonelyCZ
- @lxtywypc
- @MingZhang-YBPS
- @MolisXYliu
- @RainbowMango
- @Rains6
- @tedli
- @Vacant2333
- @vikash485
- @watermeionG
- @wawa0210
- @whitewindmills
- @wrhight
- @wu0407
- @xigang
- @XiShanYongYe-Chang
- @Yan-Daojiang
- @yanfeng1992
- @zhy76
- @zhzhuang-zju
