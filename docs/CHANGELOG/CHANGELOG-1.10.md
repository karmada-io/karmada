<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.10.11](#v11011)
  - [Downloads for v1.10.11](#downloads-for-v11011)
  - [Changelog since v1.10.10](#changelog-since-v11010)
    - [Changes by Kind](#changes-by-kind)
      - [Bug Fixes](#bug-fixes)
      - [Others](#others)
- [v1.10.10](#v11010)
  - [Downloads for v1.10.10](#downloads-for-v11010)
  - [Changelog since v1.10.9](#changelog-since-v1109)
    - [Changes by Kind](#changes-by-kind-1)
      - [Bug Fixes](#bug-fixes-1)
      - [Others](#others-1)
- [v1.10.9](#v1109)
  - [Downloads for v1.10.9](#downloads-for-v1109)
  - [Changelog since v1.10.8](#changelog-since-v1108)
    - [Changes by Kind](#changes-by-kind-2)
      - [Bug Fixes](#bug-fixes-2)
      - [Others](#others-2)
- [v1.10.8](#v1108)
  - [Downloads for v1.10.8](#downloads-for-v1108)
  - [Changelog since v1.10.7](#changelog-since-v1107)
    - [Changes by Kind](#changes-by-kind-3)
      - [Urgent Upgrade Notes](#urgent-upgrade-notes)
      - [Bug Fixes](#bug-fixes-3)
      - [Others](#others-3)
- [v1.10.7](#v1107)
  - [Downloads for v1.10.7](#downloads-for-v1107)
  - [Changelog since v1.10.6](#changelog-since-v1106)
    - [Changes by Kind](#changes-by-kind-4)
      - [Bug Fixes](#bug-fixes-4)
      - [Others](#others-4)
- [v1.10.6](#v1106)
  - [Downloads for v1.10.6](#downloads-for-v1106)
  - [Changelog since v1.10.5](#changelog-since-v1105)
    - [Changes by Kind](#changes-by-kind-5)
      - [Bug Fixes](#bug-fixes-5)
      - [Others](#others-5)
- [v1.10.5](#v1105)
  - [Downloads for v1.10.5](#downloads-for-v1105)
  - [Changelog since v1.10.4](#changelog-since-v1104)
    - [Changes by Kind](#changes-by-kind-6)
      - [Bug Fixes](#bug-fixes-6)
      - [Others](#others-6)
- [v1.10.4](#v1104)
  - [Downloads for v1.10.4](#downloads-for-v1104)
  - [Changelog since v1.10.3](#changelog-since-v1103)
    - [Changes by Kind](#changes-by-kind-7)
      - [Bug Fixes](#bug-fixes-7)
      - [Others](#others-7)
- [v1.10.3](#v1103)
  - [Downloads for v1.10.3](#downloads-for-v1103)
  - [Changelog since v1.10.2](#changelog-since-v1102)
    - [Changes by Kind](#changes-by-kind-8)
      - [Bug Fixes](#bug-fixes-8)
      - [Others](#others-8)
- [v1.10.2](#v1102)
  - [Downloads for v1.10.2](#downloads-for-v1102)
  - [Changelog since v1.10.1](#changelog-since-v1101)
    - [Changes by Kind](#changes-by-kind-9)
      - [Bug Fixes](#bug-fixes-9)
      - [Others](#others-9)
- [v1.10.1](#v1101)
  - [Downloads for v1.10.1](#downloads-for-v1101)
  - [Changelog since v1.10.0](#changelog-since-v1100)
    - [Changes by Kind](#changes-by-kind-10)
      - [Bug Fixes](#bug-fixes-10)
      - [Others](#others-10)
- [v1.10.0](#v1100)
  - [Downloads for v1.10.0](#downloads-for-v1100)
  - [What's New](#whats-new)
    - [Workload Rebalance](#workload-rebalance)
    - [Got rid of the restriction on the length of resource template name](#got-rid-of-the-restriction-on-the-length-of-resource-template-name)
  - [Other Notable Changes](#other-notable-changes)
    - [API Changes](#api-changes)
    - [Deprecation](#deprecation)
    - [Bug Fixes](#bug-fixes-11)
    - [Security](#security)
    - [Features & Enhancements](#features--enhancements)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Charts](#helm-charts)
    - [Instrumentation](#instrumentation)
  - [Contributors](#contributors)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.10.11
## Downloads for v1.10.11

Download v1.10.11 in the [v1.10.11 release page](https://github.com/karmada-io/karmada/releases/tag/v1.10.11).

## Changelog since v1.10.10
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue where the `detector` unnecessary updates for RB issue. ([#6170](https://github.com/karmada-io/karmada/pull/6170), @CharlesQQ)
- `karmada-search`: Fixed the issue that namespaces in different ResourceRegistry might be overwritten. ([#6102](https://github.com/karmada-io/karmada/pull/6102), @JimDevil)

#### Others
- The base image `alpine` now has been promoted from 3.21.2 to 3.21.3. ([#6122](https://github.com/karmada-io/karmada/pull/6122))

# v1.10.10
## Downloads for v1.10.10

Download v1.10.10 in the [v1.10.10 release page](https://github.com/karmada-io/karmada/releases/tag/v1.10.10).

## Changelog since v1.10.9
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that newly created attached-ResourceBinding be mystically garbage collected. ([#6053](https://github.com/karmada-io/karmada/pull/6053), @whitewindmills)

#### Others
- The base image `alpine` now has been promoted from 3.21.0 to 3.21.2. ([#6039](https://github.com/karmada-io/karmada/pull/6039))

# v1.10.9
## Downloads for v1.10.9

Download v1.10.9 in the [v1.10.9 release page](https://github.com/karmada-io/karmada/releases/tag/v1.10.9).

## Changelog since v1.10.8
### Changes by Kind
#### Bug Fixes
- `karmada-webhook`: Fixed panic when validating ResourceInterpreterWebhookConfiguration with unspecified service port. ([#5965](https://github.com/karmada-io/karmada/pull/5965), @seanlaii)
- `karmada-controller-manager`: Fixed the issue of missing work queue metrics. ([#5983](https://github.com/karmada-io/karmada/pull/5983), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the bug of WorkloadRebalancer doesn't get deleted after TTL. ([#5993](https://github.com/karmada-io/karmada/pull/5993), @deefreak)

#### Others
None.

# v1.10.8
## Downloads for v1.10.8

Download v1.10.8 in the [v1.10.8 release page](https://github.com/karmada-io/karmada/releases/tag/v1.10.8).

## Changelog since v1.10.7
### Changes by Kind
#### Urgent Upgrade Notes
- The feature `Failover` now has been disabled by default, which should be explicitly enabled to avoid unexpected incidents. ([#5947](https://github.com/karmada-io/karmada/pull/5947), @XiShanYongYe-Chang)

If you are using the feature `Failover`, please enable it explicitly by adding the `--feature-gates=Failover=true,<other feature>` flag to the `karmada-controller-manager` component. If you are not using this feature, this change will have no impact.

#### Bug Fixes
- `karmadactl`: Fixed `karmada-metrics-adapter` use the incorrect certificate issue when deployed via karmadactl `init`. ([#5860](https://github.com/karmada-io/karmada/pull/5860), @seanlaii)
- `karmada-controller-manager`: Fixed the corner case where the reconciliation of aggregating status might be missed in case of component restart. ([#5883](https://github.com/karmada-io/karmada/pull/5883), @liangyuanpeng)
- `karmada-controller-manager`: Fixed the problem of ResourceBinding remaining after the resource template is deleted in the dependencies distribution scenario. ([#5952](https://github.com/karmada-io/karmada/pull/5952), @XiShanYongYe-Chang)
- `karmada-scheduler`: Avoid filtering out clusters if the API enablement is incomplete during re-scheduling. ([#5931](https://github.com/karmada-io/karmada/pull/5931), @XiShanYongYe-Chang)

#### Others
- The base image `alpine` now has been promoted from `3.20.3` to `3.21.0`. ([#5921](https://github.com/karmada-io/karmada/pull/5921))

# v1.10.7
## Downloads for v1.10.7

Download v1.10.7 in the [v1.10.7 release page](https://github.com/karmada-io/karmada/releases/tag/v1.10.7).

## Changelog since v1.10.6
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that status aggregation against the resource template might be missed due to slow cache sync. ([#5844](https://github.com/karmada-io/karmada/pull/5844), @chaosi-zju)
- `karmadactl`: The `--force` option of `unjoin` command now try to clean up resources propagated in member clusters. ([#5845](https://github.com/karmada-io/karmada/pull/5845), @chaosi-zju)

#### Others
None.

# v1.10.6
## Downloads for v1.10.6

Download v1.10.6 in the [v1.10.6 release page](https://github.com/karmada-io/karmada/releases/tag/v1.10.6).

## Changelog since v1.10.5
### Changes by Kind
#### Bug Fixes
- `karmada-aggregated-apiserver`: User can append a "/" at the end when configuring the cluster's apiEndpoint. ([#5556](https://github.com/karmada-io/karmada/pull/5556), @spiritNO1)
- `karmada-controller-manager`: Ignored StatefulSet Dependencies with PVCs created via the VolumeClaimTemplates. ([#5687](https://github.com/karmada-io/karmada/pull/5687), @seanlaii)
- `karmada-scheduler`: Fixed unexpected modification of original `ResourceSummary` due to lack of deep copy. ([#5725](https://github.com/karmada-io/karmada/pull/5725), @RainbowMango)
- `karmada-scheduler`: Fixes an issue where resource model grades were incorrectly matched based on resource requests. Now only grades that can provide sufficient resources will be selected. ([#5729](https://github.com/karmada-io/karmada/pull/5729), @RainbowMango)
- `karmada-search`: Modify the logic of checking whether the resource is registered when selecting the plugin. ([#5736](https://github.com/karmada-io/karmada/pull/5736), @seanlaii)

#### Others
None.

# v1.10.5
## Downloads for v1.10.5

Download v1.10.5 in the [v1.10.5 release page](https://github.com/karmada-io/karmada/releases/tag/v1.10.5).

## Changelog since v1.10.4
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the error of cluster status old condition update will overwrite the newest condition. ([#5401](https://github.com/karmada-io/karmada/pull/5401), @XiShanYongYe-Chang)

#### Others
- The base image `alpine` now has been promoted from `alpine:3.20.2` to `alpine:3.20.3`.

# v1.10.4
## Downloads for v1.10.4

Download v1.10.4 in the [v1.10.4 release page](https://github.com/karmada-io/karmada/releases/tag/v1.10.4).

## Changelog since v1.10.3
### Changes by Kind
#### Bug Fixes
- `Helm`: fix wrong `ClusterResourceBinding` scope in `MutatingWebhookConfiguration`. ([#5262](https://github.com/karmada-io/karmada/pull/5262), @XiShanYongYe-Chang)

#### Others
- The base image `alpine` now has been promoted from `alpine:3.20.1` to `alpine:3.20.2`. ([#5268](https://github.com/karmada-io/karmada/pull/5268))
- Bump golang version to `v1.21.13` ([#5371](https://github.com/karmada-io/karmada/pull/5371), @zhzhuang-zju)

# v1.10.3
## Downloads for v1.10.3

Download v1.10.3 in the [v1.10.3 release page](https://github.com/karmada-io/karmada/releases/tag/v1.10.3).

## Changelog since v1.10.2
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: fix the issue of residual work in the MultiClusterService feature. ([#5211](https://github.com/karmada-io/karmada/pull/5211), @XiShanYongYe-Chang)

#### Others
- `karmada-scheduler`: GroupClusters will sort clusters by score and availableReplica count. ([#5180](https://github.com/karmada-io/karmada/pull/5180), @mszacillo)

# v1.10.2
## Downloads for v1.10.2

Download v1.10.2 in the [v1.10.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.10.2).

## Changelog since v1.10.1
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that the default resource interpreter doesn't accurately interpret the numbers of replicas. ([#5108](https://github.com/karmada-io/karmada/pull/5108), @whitewindmills)

#### Others
- The base image `alpine` now has been promoted from `alpine:3.20.0` to `alpine:3.20.1`. ([#5093](https://github.com/karmada-io/karmada/pull/5093))

# v1.10.1
## Downloads for v1.10.1

Download v1.10.1 in the [v1.10.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.10.1).

## Changelog since v1.10.0
### Changes by Kind
#### Bug Fixes
- `karmada-scheduler-estimator`: Fixed the `Unschedulable` result returned by plugins to be treated as an exception issue. ([#5027](https://github.com/karmada-io/karmada/pull/5027), @RainbowMango)
- `karmada-controller-manager`: Fixed an issue that the cluster-status-controller overwrites the remedyActions field. ([#5043](https://github.com/karmada-io/karmada/pull/5043), @XiShanYongYe-Chang)

#### Others
None.

# v1.10.0
## Downloads for v1.10.0

Download v1.10.0 in the [v1.10.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.10.0).

## What's New

### Workload Rebalance

This release introduced a workload rebalancing capability. It can actively trigger a brand fresh rescheduling to establish an entirely new replicas distribution across clusters. 

In some scenarios, the current distribution of replicas might not always be ideal, such as:

* Replicas were migrated due to cluster failover, but the cluster has now recovered.

* Replicas were migrated due to application-level failover, but each cluster now has sufficient resources to run the replicas.

* For the `Aggregated` scheduling strategy, replicas were initially distributed across multiple clusters due to resource constraints, but now a single cluster is sufficient to accommodate all replicas.
  
With the workload rebalancing capability, users can trigger a workload rebalancing on demand if the current replicas distribution is not optimal.

For a detailed description of this feature, see the [User Guide](https://karmada.io/docs/next/userguide/scheduling/workload-rebalancer), and for a specific demonstration, see the [Tutorial](https://karmada.io/docs/next/tutorials/workload-rebalancer).

(Feature contributors: @chaosi-zju)

### Got rid of the restriction on the length of resource template name

Due to historical design reasons, the name of the resource template will be used as the name of the label, thereby accelerating the retrieval of resources. Since Kubernetes limits the label to no more than 63 characters, this indirectly restricts the length of the resource template, seriously preventing users from migrating workload from legacy cluster to multiple clusters.

The work that got rid of this restriction started from release 1.8, we did sufficient preparatory work in both releases 1.8 and release 1.9 to ensure that users using the old version of Karmada can smoothly upgrade to the new version.

See [[Umbrella] Use permanent-id to replace namespace/name labels in the resource](https://github.com/karmada-io/karmada/issues/4711) for more details.

(Feature contributors: @liangyuanpeng, @whitewindmills, @XiShanYongYe-Chang)

## Other Notable Changes
### API Changes
- Introduced `ServiceAnnotations` to the `Karmada` API to provide an extra set of annotations to annotate karmada apiserver services. ([#4679](https://github.com/karmada-io/karmada/pull/4679), @calvin0327)
- Add a short name for resourceinterpretercustomizations CRD resource. ([#4872](https://github.com/karmada-io/karmada/pull/4872), @XiShanYongYe-Chang)
- Introduce a new API named `WorkloadRebalancer` to support rescheduling. ([#4841](https://github.com/karmada-io/karmada/pull/4841), @chaosi-zju)

### Deprecation
- The following labels have been deprecated from release `v1.8.0` and now have been removed:
    * `resourcebinding.karmada.io/uid`
    * `clusterresourcebinding.karmada.io/uid`
    * `work.karmada.io/uid`
    * `propagationpolicy.karmada.io/uid`
    * `clusterpropagationpolicy.karmada.io/uid`
- The following labels now have been deprecated and removed:
    * `resourcebinding.karmada.io/key` replaced by `resourcebinding.karmada.io/permanent-id`
    * `clusterresourcebinding.karmada.io/key` replaced by `clusterresourcebinding.karmada.io/permanent-id`
    * `work.karmada.io/namespace` replaced by `work.karmada.io/permanent-id`
    * `work.karmada.io/name` replaced by `work.karmada.io/permanent-id`
    * `resourcebinding.karmada.io/depended-id`
- `karmadactl`: The flag `--cluster-zone`, which was deprecated in release 1.7 and replaced by `--cluster-zones`, now has been removed. ([#4967](https://github.com/karmada-io/karmada/pull/4967), @RainbowMango)

### Bug Fixes
- `karmada-operator`: Fixed the `karmada-search` can not be deleted issue due to missing `app.kubernetes.io/managed-by` label. ([#4674](https://github.com/karmada-io/karmada/pull/4674), @laihezhao)
- `karmada-controller-manager`: Fixed deployment replicas syncer in case deployment status changed before label added. ([#4721](https://github.com/karmada-io/karmada/pull/4721), @chaosi-zju)
- `karmada-controller-manager`: Fixed incorrect annotation markup when policy preemption occurs. ([#4751](https://github.com/karmada-io/karmada/pull/4751), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue of EndpointSlice residual in case of the karmada-controller-manager restart. ([#4737](https://github.com/karmada-io/karmada/pull/4737), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed deployment replicas syncer in case that `status.replicas` haven't been collected from member cluster to template. ([#4729](https://github.com/karmada-io/karmada/pull/4729), @chaosi-zju)
- `karmada-controller-manager`: Fixed the problem that labels cannot be deleted via Karmada propagation. ([#4784](https://github.com/karmada-io/karmada/pull/4784), @whitewindmills)
- `karmada-controller-manager`: Fixed the problem that work.karmada.io/permanent-id constantly changes with every update. ([#4793](https://github.com/karmada-io/karmada/pull/4793), @whitewindmills)
- `resourceinterpreter`: Avoid delete the key with empty value in object (lua table).  ([#4656](https://github.com/karmada-io/karmada/pull/4656), @chaosi-zju)
- `karmada-controller-manager`: Fixed the bug of mcs binding losing resourcebinding.karmada.io/permanent-id label. ([#4818](https://github.com/karmada-io/karmada/pull/4818), @whitewindmills)
- `resourceinterpreter`: Prune deployment revision annotations. ([#4946](https://github.com/karmada-io/karmada/pull/4946), @a7i)
- `karmada-controller-manager`: Fix depended-by label value exceed 63 characters in dependencies-distributor. ([#4989](https://github.com/karmada-io/karmada/pull/4989), @XiShanYongYe-Chang)
- `karmadactl`:  Fix register cluster creates agent missing `--cluster-namespace` parameter. ([#5005](https://github.com/karmada-io/karmada/pull/5005), @yanfeng1992)

### Security
- Bump google.golang.org/protobuf from 1.31.0 to 1.33.0 fix CVE(CVE-2024-24786) concerns. ([#4715](https://github.com/karmada-io/karmada/pull/4715), @liangyuanpeng)
- Upgrade rsa key size from 2048 to 3072. ([#4955](https://github.com/karmada-io/karmada/pull/4955), @chaosi-zju)
- Replace `text/template` with `html/template`, which adds security protection such as HTML encoding and has stronger functions. ([#4957](https://github.com/karmada-io/karmada/pull/4957), @chaosi-zju)
- Grant the correct permissions when creating a file. ([#4960](https://github.com/karmada-io/karmada/pull/4960), @chaosi-zju)

### Features & Enhancements
- `karmada-controller-manager`: Using the natural ordering properties of red-black trees to sort the listed policies to ensure the higher priority (Cluster)PropagationPolicy being processed first to avoid possible multiple preemption. ([#4555](https://github.com/karmada-io/karmada/pull/4555), @whitewindmills)
- `karmada-controller-manager`: Introduced `deploymentReplicasSyncer` controller which syncs Deployment's replicas from the member cluster to the control plane, while previous `hpaReplicasSyncer` been replaced. ([#4707](https://github.com/karmada-io/karmada/pull/4707), @chaosi-zju)
- `karmada-metrics-adapter`: Introduced the `--profiling` and `--profiling-bind-address` flags to enable and control profiling. ([#4786](https://github.com/karmada-io/karmada/pull/4786), @chaosi-zju)
- `karmada-metrics-adapter`: Using `TransformFunc` to trim unused information to reduce memory usage. ([#4796](https://github.com/karmada-io/karmada/pull/4796), @chaunceyjiang)
- `karmadactl`: Introduced `--image-pull-policy` flag to the `init` command, which will be used to specify the image pull policy of all components. ([#4815](https://github.com/karmada-io/karmada/pull/4815), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Propagate `Secret` of type `kubernetes.io/service-account-token`. ([#4766](https://github.com/karmada-io/karmada/pull/4766), @a7i)
- `karmada-metrics-adapter`: Add QPS related parameters to control the request rate of metrics-adapter to member clusters. ([#4809](https://github.com/karmada-io/karmada/pull/4809), @chaunceyjiang)
- `thirdparty`: Show `status.labelSelector` for CloneSet. ([#4839](https://github.com/karmada-io/karmada/pull/4839), @veophi)
- `karmada-controller-manager`: Add finalizer for propagation policy. ([#4836](https://github.com/karmada-io/karmada/pull/4836), @whitewindmills)
- `karmada-scheduler`: Introduce a mechanism to scheduler to actively trigger rescheduling. ([#4848](https://github.com/karmada-io/karmada/pull/4848), @chaosi-zju)
- `karmada-operator`: Allow the user to specify `imagePullPolicy` in Karmada CR when installing via karmada-operator. ([#4863](https://github.com/karmada-io/karmada/pull/4863), @seanlaii)
- `karmada-controller-manager`: Support update event in WorkloadRebalancer. ([#4860](https://github.com/karmada-io/karmada/pull/4860), @chaosi-zju)
- `karmada-controller-manager`: Remove cluster specific `PersistentVolume` annotation `volume.kubernetes.io/selected-node`. ([#4943](https://github.com/karmada-io/karmada/pull/4943), @a7i)
- `karmada-webhook`: Add validation on policy permanent ID. ([#4964](https://github.com/karmada-io/karmada/pull/4964), @whitewindmills)
- `karmada-controller-manager`: Support auto delete WorkloadRebalancer when time up. ([#4894](https://github.com/karmada-io/karmada/pull/4894), @chaosi-zju)
- `karmadactl`: Integrated with OIDC authentication for cluster operation auditing and access control. ([#4883](https://github.com/karmada-io/karmada/pull/4883), @guozheng-shen)

## Other
### Dependencies
- Bump golang version to `v1.21.8`. ([#4706](https://github.com/karmada-io/karmada/pull/4706), @Ray-D-Song)
- Bump Kubernetes dependencies to `v1.29.4`. ([#4884](https://github.com/karmada-io/karmada/pull/4884), @RainbowMango)
- Karmada is now built with Go1.21.10. ([#4920](https://github.com/karmada-io/karmada/pull/4920), @zhzhuang-zju)
- The base image `alpine` now has been promoted from `alpine:3.19.1` to `alpine:3.20.0`.

### Helm Charts
- `Helm Chart`: Update operator crd when upgrading chart. ([#4693](https://github.com/karmada-io/karmada/pull/4693), @calvin0327)
- Upgrade `bitnami/common` dependency in karmada chart from `1.x.x` to `2.x.x`. ([#4829](https://github.com/karmada-io/karmada/pull/4829), @warjiang)

### Instrumentation

## Contributors
Thank you to everyone who contributed to this release!

Users whose commits are in this release (alphabetically by username)

- @a7i
- @Affan-7
- @B1F030
- @calvin0327
- @chaosi-zju
- @chaunceyjiang
- @dzcvxe
- @Fish-pro
- @grosser
- @guozheng-shen
- @hulizhe
- @Jay179-sudo
- @jwcesign
- @khanhtc1202
- @laihezhao
- @liangyuanpeng
- @my-git9
- @RainbowMango
- @Ray-D-Song
- @rohit-satya
- @seanlaii
- @stulzq
- @veophi
- @wangxf1987
- @warjiang
- @whitewindmills
- @wzshiming
- @XiShanYongYe-Chang 
- @yanfeng1992
- @yike21
- @yizhang-zen
- @zhzhuang-zju
