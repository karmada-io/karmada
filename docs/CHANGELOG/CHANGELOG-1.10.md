<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.10.0](#v1100)
  - [Downloads for v1.10.0](#downloads-for-v1100)
  - [What's New](#whats-new)
    - [Workload Rebalance](#workload-rebalance)
    - [Got rid of the restriction on the length of resource template name](#got-rid-of-the-restriction-on-the-length-of-resource-template-name)
  - [Other Notable Changes](#other-notable-changes)
    - [API Changes](#api-changes)
    - [Deprecation](#deprecation)
    - [Bug Fixes](#bug-fixes)
    - [Security](#security)
    - [Features & Enhancements](#features--enhancements)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Charts](#helm-charts)
    - [Instrumentation](#instrumentation)
  - [Contributors](#contributors)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

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
