<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.12.8](#v1128)
  - [Downloads for v1.12.8](#downloads-for-v1128)
  - [Changelog since v1.12.7](#changelog-since-v1127)
    - [Changes by Kind](#changes-by-kind)
      - [Bug Fixes](#bug-fixes)
      - [Others](#others)
- [v1.12.7](#v1127)
  - [Downloads for v1.12.7](#downloads-for-v1127)
  - [Changelog since v1.12.6](#changelog-since-v1126)
    - [Changes by Kind](#changes-by-kind-1)
      - [Bug Fixes](#bug-fixes-1)
      - [Others](#others-1)
- [v1.12.6](#v1126)
  - [Downloads for v1.12.6](#downloads-for-v1126)
  - [Changelog since v1.12.5](#changelog-since-v1125)
    - [Changes by Kind](#changes-by-kind-2)
      - [Bug Fixes](#bug-fixes-2)
      - [Others](#others-2)
- [v1.12.5](#v1125)
  - [Downloads for v1.12.5](#downloads-for-v1125)
  - [Changelog since v1.12.4](#changelog-since-v1124)
    - [Changes by Kind](#changes-by-kind-3)
      - [Bug Fixes](#bug-fixes-3)
      - [Others](#others-3)
- [v1.12.4](#v1124)
  - [Downloads for v1.12.4](#downloads-for-v1124)
  - [Changelog since v1.12.3](#changelog-since-v1123)
    - [Changes by Kind](#changes-by-kind-4)
      - [Bug Fixes](#bug-fixes-4)
      - [Others](#others-4)
- [v1.12.3](#v1123)
  - [Downloads for v1.12.3](#downloads-for-v1123)
  - [Changelog since v1.12.2](#changelog-since-v1122)
    - [Changes by Kind](#changes-by-kind-5)
      - [Bug Fixes](#bug-fixes-5)
      - [Others](#others-5)
- [v1.12.2](#v1122)
  - [Downloads for v1.12.2](#downloads-for-v1122)
  - [Changelog since v1.12.1](#changelog-since-v1121)
    - [Changes by Kind](#changes-by-kind-6)
      - [Bug Fixes](#bug-fixes-6)
      - [Others](#others-6)
- [v1.12.1](#v1121)
  - [Downloads for v1.12.1](#downloads-for-v1121)
  - [Changelog since v1.12.0](#changelog-since-v1120)
    - [Changes by Kind](#changes-by-kind-7)
      - [Bug Fixes](#bug-fixes-7)
      - [Others](#others-7)
- [v1.12.0](#v1120)
  - [Downloads for v1.12.0](#downloads-for-v1120)
  - [What's New](#whats-new)
    - [Support Roll Back Migration Safely](#support-roll-back-migration-safely)
    - [Stateful Application Failover Support](#stateful-application-failover-support)
    - [Karmada Operator Enhancement: Multi-Data Center Redundancy and Disaster Recovery(DR)](#karmada-operator-enhancement-multi-data-center-redundancy-and-disaster-recoverydr)
    - [OverridePolicy now support partially override values inside JSON and YAML fields](#overridepolicy-now-support-partially-override-values-inside-json-and-yaml-fields)
  - [Other Notable Changes](#other-notable-changes)
    - [API Changes](#api-changes)
    - [Deprecation](#deprecation)
    - [Bug Fixes](#bug-fixes-8)
    - [Security](#security)
    - [Features & Enhancements](#features--enhancements)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Charts](#helm-charts)
    - [Instrumentation](#instrumentation)
  - [Contributors](#contributors)
- [v1.12.0-beta.0](#v1120-beta0)
  - [Downloads for v1.12.0-beta.0](#downloads-for-v1120-beta0)
  - [Changelog since v1.12.0-alpha.1](#changelog-since-v1120-alpha1)
  - [Urgent Update Notes](#urgent-update-notes)
  - [Changes by Kind](#changes-by-kind-8)
    - [API Changes](#api-changes-1)
    - [Features & Enhancements](#features--enhancements-1)
    - [Deprecation](#deprecation-1)
    - [Bug Fixes](#bug-fixes-9)
    - [Security](#security-1)
  - [Other](#other-1)
    - [Dependencies](#dependencies-1)
    - [Helm Charts](#helm-charts-1)
    - [Instrumentation](#instrumentation-1)
- [v1.12.0-alpha.1](#v1120-alpha1)
  - [Downloads for v1.12.0-alpha.1](#downloads-for-v1120-alpha1)
  - [Changelog since v1.11.0](#changelog-since-v1110)
  - [Urgent Update Notes](#urgent-update-notes-1)
  - [Changes by Kind](#changes-by-kind-9)
    - [API Changes](#api-changes-2)
    - [Features & Enhancements](#features--enhancements-2)
    - [Deprecation](#deprecation-2)
    - [Bug Fixes](#bug-fixes-10)
    - [Security](#security-2)
  - [Other](#other-2)
    - [Dependencies](#dependencies-2)
    - [Helm Charts](#helm-charts-2)
    - [Instrumentation](#instrumentation-2)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.12.8
## Downloads for v1.12.8

Download v1.12.8 in the [v1.12.8 release page](https://github.com/karmada-io/karmada/releases/tag/v1.12.8).

## Changelog since v1.12.7
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that resources will be recreated after being deleted on the cluster when resource is suspended for dispatching. ([#6538](https://github.com/karmada-io/karmada/pull/6538), @luyb177)
- `karmada-controller-manager`: Fixed the issue that EndpointSlice are deleted unexpectedly due to the EndpointSlice informer cache not being synced. ([#6585](https://github.com/karmada-io/karmada/pull/6585), @XiShanYongYe-Chang)

#### Others
- The base image `alpine` now has been promoted from 3.22.0 to 3.22.1. ([#6562](https://github.com/karmada-io/karmada/pull/6562))

# v1.12.7
## Downloads for v1.12.7

Download v1.12.7 in the [v1.12.7 release page](https://github.com/karmada-io/karmada/releases/tag/v1.12.7).

## Changelog since v1.12.6
### Changes by Kind
#### Bug Fixes
None.

#### Others
- The base image `alpine` now has been promoted from 3.21.3 to 3.22.0. ([#6422](https://github.com/karmada-io/karmada/pull/6422))

# v1.12.6
## Downloads for v1.12.6

Download v1.12.6 in the [v1.12.6 release page](https://github.com/karmada-io/karmada/releases/tag/v1.12.6).

## Changelog since v1.12.5
### Changes by Kind
#### Bug Fixes
- `karmada-agent`: Fixed the issue where a new pull-mode cluster may overwrite the existing member clusters. ([#6261](https://github.com/karmada-io/karmada/pull/6261), @zhzhuang-zju)
- `karmadactl`: Fixed the issue where option `discovery-timeout` fails to work properly. ([#6279](https://github.com/karmada-io/karmada/pull/6279), @seanlaii)
- `karmada-controller-manager`: Fixed the issue that the result label of `federatedhpa_pull_metrics_duration_seconds` is always `success`. ([#6313](https://github.com/karmada-io/karmada/pull/6313), @tangzhongren)
- `karmada-controller-manager`/`karmada-agent`: Fixed the issue that cluster status update interval shorter than configured `--cluster-status-update-frequency`. ([#6336](https://github.com/karmada-io/karmada/pull/6336), @RainbowMango)

#### Others
None.

# v1.12.5
## Downloads for v1.12.5

Download v1.12.5 in the [v1.12.5 release page](https://github.com/karmada-io/karmada/releases/tag/v1.12.5).

## Changelog since v1.12.4
### Changes by Kind
#### Bug Fixes
- `karmada-agent`: Fixed a panic issue where the agent does not need to report secret when registering cluster. ([#6223](https://github.com/karmada-io/karmada/pull/6223), @jabellard)
- `karmada-controller-manager`: Fixed the issue that the gracefulEvictionTask of ResourceBinding can not be cleared in case of schedule fails. ([#6235](https://github.com/karmada-io/karmada/pull/6235), @XiShanYongYe-Chang)
- `helm`: Fixed the issue where the required ServiceAccount was missing when the certificate mode was set to custom. ([#6241](https://github.com/karmada-io/karmada/pull/6241), @seanlaii)

#### Others
None.

# v1.12.4
## Downloads for v1.12.4

Download v1.12.4 in the [v1.12.4 release page](https://github.com/karmada-io/karmada/releases/tag/v1.12.4).

## Changelog since v1.12.3
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue where the `detector` unnecessary updates for RB issue. ([#6167](https://github.com/karmada-io/karmada/pull/6167), @XiShanYongYe-Chang)
- `karmada-search`: Fixed the issue that namespaces in different ResourceRegistry might be overwritten. ([#6100](https://github.com/karmada-io/karmada/pull/6100), @JimDevil)

#### Others
- The base image `alpine` now has been promoted from 3.21.2 to 3.21.3. ([#6123](https://github.com/karmada-io/karmada/pull/6123))
- Karmada(release-1.12) now built with Golang v1.22.12. ([#6132](https://github.com/karmada-io/karmada/pull/6132), @sachinparihar)

# v1.12.3
## Downloads for v1.12.3

Download v1.12.3 in the [v1.12.3 release page](https://github.com/karmada-io/karmada/releases/tag/v1.12.3).

## Changelog since v1.12.2
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that newly created attached-ResourceBinding be mystically garbage collected. ([#6055](https://github.com/karmada-io/karmada/pull/6055), @whitewindmills)

#### Others
- The base image `alpine` now has been promoted from 3.21.0 to 3.21.2. ([#6041](https://github.com/karmada-io/karmada/pull/6041))
- Karmada(release-1.12) now built with Golang v1.22.11. ([#6069](https://github.com/karmada-io/karmada/pull/6069), @y1hao)

# v1.12.2
## Downloads for v1.12.2

Download v1.12.2 in the [v1.12.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.12.2).

## Changelog since v1.12.1
### Changes by Kind
#### Bug Fixes
- `karmada-webhook`: Fixed panic when validating ResourceInterpreterWebhookConfiguration with unspecified service port. ([#5967](https://github.com/karmada-io/karmada/pull/5967), @seanlaii)
- `karmada-controller-manager`: Fixed the issue of missing work queue metrics. ([#5980](https://github.com/karmada-io/karmada/pull/5980), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the bug of WorkloadRebalancer doesn't get deleted after TTL. ([#5995](https://github.com/karmada-io/karmada/pull/5995), @deefreak)
- `karmada-operator`: Fixed the issue that external ETCD certificate be overwritten by generated in-cluster ETCD certificate. ([#6008](https://github.com/karmada-io/karmada/pull/6008), @jabellard)

#### Others
None.

# v1.12.1
## Downloads for v1.12.1

Download v1.12.1 in the [v1.12.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.12.1).

## Changelog since v1.12.0
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the problem of ResourceBinding remaining after the resource template is deleted in the dependencies distribution scenario. ([#5950](https://github.com/karmada-io/karmada/pull/5950), @XiShanYongYe-Chang)

#### Others
- The base image `alpine` now has been promoted from `3.20.3` to `3.21.0`. ([#5927](https://github.com/karmada-io/karmada/pull/5927))

# v1.12.0
## Downloads for v1.12.0

Download v1.12.0 in the [v1.12.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.12.0).

## What's New

### Support Roll Back Migration Safely

This release introduces `PreserveResourcesOnDeletion` field to both PropagationPolicy and ClusterPropagationPolicy API, which provides the ability to rollback migration safely.

The `PreserveResourcesOnDeletion` field controls whether resources should be preserved on the member clusters when the resource template is deleted in the Karmada control-plane. If it is set to true, resources will be preserved on the member clusters.

This feature is particularly useful during workload migration scenarios to ensure that rollback can occur quickly without affecting the workloads running on the member clusters.

For a detailed description of this feature, please refer to [How to Roll Back Migration Operations](https://karmada.io/docs/next/administrator/migration/migrate-in-batch/#how-to-roll-back-migration-operations) and [docs(proposal): Migration Rollback Protection](https://github.com/karmada-io/karmada/pull/5101).

(Feature contributors: @CharlesQQ @XiShanYongYe-Chang @RainbowMango @a7i @chaosi-zju @wulemao)

### Stateful Application Failover Support

This release introduces a new feature for stateful application failover, it provides a generalized way for users to define application state preservation in the context of cluster-to-cluster failovers.

In the previous releases, Karmada’s scheduling logic runs on the assumption that resources that are scheduled and rescheduled are stateless. In some cases, users may desire to conserve a certain state so that applications can resume from where they left off in the previous cluster.

For CRDs dealing with data-processing (such as Flink or Spark), it can be particularly useful to restart applications from a previous checkpoint. That way applications can seamlessly resume processing data while avoiding double processing.

For a detailed description of this feature, please refer to [[Feature] Stateful Application Failover Support](https://github.com/karmada-io/karmada/issues/5788) and [docs: Stateful Application Failover Support](https://karmada.io/docs/next/userguide/failover/application-failover/#stateful-application-failover-support).

(Feature contributors: @Dyex719 @mszacillo @RainbowMango @XiShanYongYe-Chang)

### Karmada Operator Enhancement: Multi-Data Center Redundancy and Disaster Recovery(DR)

This release, karmada-operator makes a number of enhancements to support high availability scenarios, including:
- introduces support for a Custom CA Certificate for Karmada Instances;
- adds The ability to retrieve external etcd client credentials from secret;
- adds The ability to specify extra volumes and volume mounts for Karmada components;
- exposes APIServer Service provisioned by karmada-operator;
- the ability to utilize external etcd. 

These enhancements allow Karmada-operator to deploy a highly available managed Karmada control plane across multiple management clusters that can span various data centers, thus fulfilling disaster recovery requirements.

By implementing this architecture and configuring the managed control plane instances to use the same CA certificate and underlying etcd instance, you can create a stretched instance that operates across multiple management clusters. 
This setup allows secure access through a unified, load-balanced API endpoint. Ultimately, this arrangement enhances resilience against data center outages, complies with disaster recovery requirements, and minimizes the risk of service disruptions.

For a detailed description of this feature, see the [Proposal: Support Custom CA Certificate for Karmada Control Plane](https://github.com/karmada-io/karmada/tree/master/docs/proposals/karmada-operator/custom_ca_cert)

(Feature contributors: @jabellard, @RainbowMango, @chaosi-zju, @zhzhuang-zju)

### OverridePolicy now support partially override values inside JSON and YAML fields

This release introduces a new feature for `OverridePolicy` that allows users to partially override specific values in JSON and YAML resources, rather than replacing the entire configuration.
This enhancement ensures minimal modifications and improves ease of use, catering to scenarios where users only want to adjust certain values.

The allowed operations are as follows:

+ `add`: appends new key-value pairs at the specified sub path.
+ `remove`: removes specific key-value pairs at the specified sub path.
+ `replace`: replaces existing values with new values at the specified sub path.

For a detailed description of this feature, see the [Proposal: Structured configuration overrider](https://github.com/karmada-io/karmada/tree/master/docs/proposals/structured-configuration) and [docs: fieldoverrider docs](https://karmada.io/docs/next/userguide/scheduling/override-policy/#fieldoverrider).

(Feature contributors: @Patrick0308, @sophiefeifeifeiya, @chaunceyjiang)

## Other Notable Changes
### API Changes
- Introduced `SecretRef` to `Karmada` API as part of the configuration for connecting to an external etcd cluster can be used to reference a secret that contains credentials for connecting to an external etcd cluster. ([#5699](https://github.com/karmada-io/karmada/pull/5699), @jabellard)
- Introduced `extraVolumes` and `extraVolumemounts` to the `Karmada` API to optionally specify extra volumes and volume mounts for the Karmada API server component. ([#5509](https://github.com/karmada-io/karmada/pull/5509), @jabellard)
- Introduced `ApiServerService` field to `Karmada` API as part of the Karmada instance status can be used to reference the API Server service for that instance. This is useful for scenarios where higher level operators need to discover the API Server service of a Karmada instance  for tasks like setting up ingress traffic. ([#5775](https://github.com/karmada-io/karmada/pull/5775), @jabellard)
- Introduced `CustomCertificate.ApiServerCACert` field to `Karmada` API as part of the `Karmada` spec to specify the reference to a secret that contains a custom CA certificate for the Karmada API Server. ([#5842](https://github.com/karmada-io/karmada/pull/5842), @jabellard)
- API change: The ServiceType of the Karmada API server in `Karmada` API now has been restricted to `ClusterIP`, `NodePort` and `LoadBalancer`. ([#5769](https://github.com/karmada-io/karmada/pull/5769), @RainbowMango)
- Introduced a new condition `CompleteAPIEnablements` to `Cluster` API to represent the API collection status. ([#5400](https://github.com/karmada-io/karmada/pull/5400), @whitewindmills)
- Introduced `PreserveResourcesOnDeletion` field to both `PropagationPolicy` and `ClusterPropagationPolicy` API, which provides the ability to roll back migration safely. ([#5575](https://github.com/karmada-io/karmada/pull/5575), @RainbowMango)
- API Change: Introduced `FieldOverrider` to both `OverridePolicy` and `ClusterOverridePolicy`, which provides the ability to override structured data nested in manifest like ConfigMap or Secret. ([#5581](https://github.com/karmada-io/karmada/pull/5581), @RainbowMango)
- Introduced `PurgeMode` to `GracefulEvictionTask` in `ResourceBinding` and `ClusterResourceBinding` API. ([#5816](https://github.com/karmada-io/karmada/pull/5816), @mszacillo)
- Introduced `StatePreservation` to `PropagationPolicy`, which will be used to preserve status in case of application failover. ([#5885](https://github.com/karmada-io/karmada/pull/5885), @RainbowMango)

### Deprecation
- `ExternalEtcd.CAData`, `ExternalEtcd.CertData` and `ExternalEtcd.KeyData` in `Karmada` API are deprecated and will be removed in a future version. Use SecretRef for providing client connection credentials. ([#5699](https://github.com/karmada-io/karmada/pull/5699), @jabellard)
- The following flags have been deprecated in release `v1.11.0` and now have been removed:
    * `karmada-agent`: ([#5548](https://github.com/karmada-io/karmada/pull/5548), @whitewindmills)
      * --bind-address
      * --secure-port
    * `karmada-controller-manager`: ([#5549](https://github.com/karmada-io/karmada/pull/5549), @whitewindmills)
      * --bind-address 
      * --secure-port
    * `karmada-scheduler-estimator`: ([#5555](https://github.com/karmada-io/karmada/pull/5555), @seanlaii)
      * --bind-address 
      * --secure-port
    * `karmada-scheduler`: ([#5551](https://github.com/karmada-io/karmada/pull/5551), @chaosi-zju)
      * --bind-address 
      * --secure-port
    * `karmada-descheduler`: ([#5552](https://github.com/karmada-io/karmada/pull/5552), @chaosi-zju)
      * --bind-address 
      * --secure-port

### Bug Fixes
- `karmada-scheduler`: Fixed unexpected modification of original `ResourceSummary` due to lack of deep copy. ([#5685](https://github.com/karmada-io/karmada/pull/5685), @LivingCcj)
- `karmada-scheduler`: Fixes an issue where resource model grades were incorrectly matched based on resource requests. Now only grades that can provide sufficient resources will be selected. ([#5706](https://github.com/karmada-io/karmada/pull/5706), @RainbowMango)
- `karmada-scheduler`: skip the filter if the cluster is already in the list of scheduling result even if the API is missed. ([#5216](https://github.com/karmada-io/karmada/pull/5216), @yanfeng1992)
- `karmada-controller-manager`: Fixed the corner case where the reconciliation of aggregating status might be missed in case of component restart. ([#5865](https://github.com/karmada-io/karmada/pull/5865), @zach593)
- `karmada-controller-manager`: Ignored StatefulSet Dependencies with PVCs created via the VolumeClaimTemplates. ([#5568](https://github.com/karmada-io/karmada/pull/5568), @jklaw90)
- `karmada-controller-manager`: Clean up the residual annotations when resources are preempted by pp from cpp. ([#5563](https://github.com/karmada-io/karmada/pull/5563), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed an issue that policy claim metadata might be lost during the rapid deletion and creation of `PropagationPolicy(s)`/`ClusterPropagationPolicy(s)`. ([#5319](https://github.com/karmada-io/karmada/pull/5319), @zhzhuang-zju)
- `karmadactl`: Fixed the issue where commands `create`, `annotate`, `delete`, `edit`, `label`, and `patch` cannot specify the namespace flag. ([#5487](https://github.com/karmada-io/karmada/pull/5487), @zhzhuang-zju)
- `karmadactl`: Fixed the issue that `karmadactl addon` failed to install `karmada-scheduler-estimator` due to unknown flag. ([#5523](https://github.com/karmada-io/karmada/pull/5523), @chaosi-zju)
- `karmadactl`: Fixed `karmada-metrics-adapter` use the incorrect certificate issue when deployed via karmadactl `init`. ([#5840](https://github.com/karmada-io/karmada/pull/5840), @KhalilSantana)
- `karmada-operator`: Fixed the issue where the manifests for the `karmada-scheduler` and `karmada-descheduler` components were not parsed correctly. ([#5546](https://github.com/karmada-io/karmada/pull/5546), @jabellard)
- `karmada-operator`: Fixed `system:admin` can not proxy to member cluster issue. ([#5572](https://github.com/karmada-io/karmada/pull/5572), @chaosi-zju)
- `karmada-search`: Modify the logic of checking whether the resource is registered when selecting the plugin. ([#5662](https://github.com/karmada-io/karmada/pull/5662), @yanfeng1992)
- `karmada-aggregate-apiserver`: limit aggregate apiserver http method to get. User can modify member cluster's object with * in aggregated apiserver url. ([#5430](https://github.com/karmada-io/karmada/pull/5430), @spiritNO1)

### Security
In this release, the Karmada community is committed to enhancing the security of Karmada and improving the robustness of Karmada system operations. By combing components to minimize permissions and reinforcing default configurations for installations, the Karmada system's security has been significantly strengthened to protect against potential threats in an increasingly complex multi-cloud environment.
- Component Permissions Minimization
    * Reconfigure the `karmadactl register` to minimize access permissions to the Karmada control plane for its registered PULL mode clusters. ([#5793](https://github.com/karmada-io/karmada/pull/5793), @zhzhuang-zju)
    * `karmada-operator`: minimize the rbac permissions for karmada-operator. ([#5586](https://github.com/karmada-io/karmada/pull/5586), @B1F030)
- `karmadactl init`: add CRDs archive verification to enhance file system robustness. ([#5713](https://github.com/karmada-io/karmada/pull/5713), @zhzhuang-zju)
- `karmada-operator`: add CRDs archive verification to enhance file system robustness. ([#5703](https://github.com/karmada-io/karmada/pull/5703), @zhzhuang-zju)
- `karmadactl init`: Eliminate unnecessary and potentially exploitable information from command output. ([#5714](https://github.com/karmada-io/karmada/pull/5714), @zhzhuang-zju)

### Features & Enhancements
- `karmada-controller-manager`: introduces the `agentcsrapproving` controller to provide the capability for the agent's CSR to be automatically approved. ([#5825](https://github.com/karmada-io/karmada/pull/5825), @zhzhuang-zju)
- `karmada-controller-manager`: update taint-manager to config eviction task with purgeMode. ([#5879](https://github.com/karmada-io/karmada/pull/5879), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Build eviction task for application failover when using purgeMode Immediately. ([#5881](https://github.com/karmada-io/karmada/pull/5881), @mszacillo)
- `karmada-controller-manager`: build PreservedLabelState when triggering eviction in RB/CRB application controller. ([#5887](https://github.com/karmada-io/karmada/pull/5887), @XiShanYongYe-Chang)
- `karmada-controller-manager`: keep preserveResourcesOnDeletion of the dependent resource consistent with that of the primary resource. ([#5717](https://github.com/karmada-io/karmada/pull/5717), @XiShanYongYe-Chang)
- `karmada-controller-manager`: set conflictResolution for dependent resources. ([#4418](https://github.com/karmada-io/karmada/pull/4418), @chaunceyjiang)
- `karmada-controller-manager`: The health status of resources without ResourceInterpreter customization will be treated as healthy by default. ([#5530](https://github.com/karmada-io/karmada/pull/5530), @a7i)
- `karmada-controller-manager`: Unique controller names and remove ambitions when reporting metrics. ([#5799](https://github.com/karmada-io/karmada/pull/5799), @chaosi-zju)
- `karmada-controller-manager`: Introduced `--concurrent-dependent-resource-syncs` flags to specify the number of dependent resource that are allowed to sync concurrently. ([#5809](https://github.com/karmada-io/karmada/pull/5809), @CharlesQQ)
- `karmada-controller-manager`: Cleanup works from clusters with eviction task when purge mode is immediately. ([#5889](https://github.com/karmada-io/karmada/pull/5889), @mszacillo))
- `karmada-controller-manager`: Inject preservedLabelState to the failover to clusters. ([#5893](https://github.com/karmada-io/karmada/pull/5893), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Introduced feature gate `StatefulFailoverInjection` to control whether Karmada collects and injects state information during a failover event for stateful application. ([#5897](https://github.com/karmada-io/karmada/pull/5897), @RainbowMango)
- `karmada-controller-manager`: The feature `Failover` now has been disabled by default, which should be explicitly enabled to avoid unexpected incidents. ([#5899](https://github.com/karmada-io/karmada/pull/5899), @RainbowMango)
- `karmadactl`: Implementing autocompletion for karmadactl to save a lot of typing. ([#5533](https://github.com/karmada-io/karmada/pull/5533), @zhzhuang-zju)
- `karmadactl`: Added shorthand letter `s` to 'operation-scope' flags across commands. ([#5483](https://github.com/karmada-io/karmada/pull/5483), @ahorine)
- `karmadactl`: `karmadactl init` support multiple label selection ability with flag `EtcdNodeSelectorLabels`. ([#5321](https://github.com/karmada-io/karmada/pull/5321), @tiansuo114)
- `karmadactl`: `karmadactl init` supports deployment through configuration files. ([#5357](https://github.com/karmada-io/karmada/pull/5357), @tiansuo114)
- `karmadactl`: new command `karmadactl unregister` supports unregister a pull mode cluster. ([#5626](https://github.com/karmada-io/karmada/pull/5626), @wulemao)
- `karmadactl`: set `PreserveResourcesOnDeletion` by default in auto-created propagation policy during promotion process. ([#5601](https://github.com/karmada-io/karmada/pull/5601), #wulemao)
- `karmadactl`: The `--force` option of `unjoin` command now try to clean up resources propagated in member clusters. ([#4451](https://github.com/karmada-io/karmada/pull/4451), @zhzhuang-zju)
- `karmadactl`: RBAC permissions for pull mode clusters registered with the `register` command are minimized when accessing the Karmada control plane. ([#5793](https://github.com/karmada-io/karmada/pull/5793), @zhzhuang-zju)
- `karmada-operator`: The new `SecretRef` field added as part of the configuration for connecting to an external etcd cluster can be used to reference a secret that contains credentials for connecting to an external etcd cluster. ([#5699](https://github.com/karmada-io/karmada/pull/5699), @jabellard)
- `karmada-operator`: Adds one-click script to install a Karmada instance through the `karmada-operator`. ([#5519](https://github.com/karmada-io/karmada/pull/5519), @zhzhuang-zju)
- `karmada-operator`:  enable LoadBalancer type karmada-apiserver service. ([#5773](https://github.com/karmada-io/karmada/pull/5423), @chaosi-zju)
- `karmada-scheduler`: implement group score calculation instead of take the highest score of clusters. ([#5621](https://github.com/karmada-io/karmada/pull/5621), @ipsum-0320)
- `karmada-scheduler`: The `scheduler-estimator-service-namespace` flag is introduced, which can be used to explicitly specify the namespace that should be used to discover scheduler estimator services. For backwards compatibility, when not explicitly set, the default value of `karmada-system` is retained. ([#5478](https://github.com/karmada-io/karmada/pull/5478), @jabellard)
- `karmada-descheduler`: Introduced leaderElection options including: `--leader-elect-lease-duration`, `--leader-elect-renew-deadline`, `--leader-elect-retry-period`, the default value not changed compared to previous version. ([#5787](https://github.com/karmada-io/karmada/pull/5787), @yanfeng1992)
- `karmada-desheduler`: The `scheduler-estimator-service-namespace` flag is introduced, which can be used to explicitly specify the namespace that should be used to discover scheduler estimator services. For backwards compatibility, when not explicitly set, the default value of `karmada-system` is retained. ([#5478](https://github.com/karmada-io/karmada/pull/5478), @jabellard)
- `karmada-search`: Implement search proxy cache initialization post-start-hook. ([#5846](https://github.com/karmada-io/karmada/pull/5846), @XiShanYongYe-Chang)
- `karmada-search`: Support field selector for corev1 resources. ([#5801](https://github.com/karmada-io/karmada/pull/5801), @SataQiu)
- `karmada-scheduler-estimator`: grpc connection adds the support for custom DNS Domain. ([#5472](https://github.com/karmada-io/karmada/pull/5472), @zhzhuang-zju)
- `karmada-webhook`: validate fieldOverrider operation. ([#5671](https://github.com/karmada-io/karmada/pull/5671), @chaunceyjiang)
- implement preserveResourcesOnDeletion to support migration rollback. ([#5597](https://github.com/karmada-io/karmada/pull/5597), @a7i)
- Introduced `FieldOverrider` for overriding values in JSON and YAML. ([#5591](https://github.com/karmada-io/karmada/pull/5591), @sophiefeifeifeiya)
- Support PurgeMode setting in evection tasks. ([#5821](https://github.com/karmada-io/karmada/pull/5821), @XiShanYongYe-Chang)

## Other
### Dependencies
- The base image `alpine` now has been promoted from `alpine:3.20.2` to `alpine:3.20.3`.
- Kubernetes dependencies have been updated to v1.31.2. ([#5807](https://github.com/karmada-io/karmada/pull/5807), @RainbowMango)
- `Karmada` now built with Golang v1.22.9. ([#5820](https://github.com/karmada-io/karmada/pull/5820), @RainbowMango)
- `karmada-apiserver` and `kube-controller-manager` is using v1.31.3 by default. ([#5851](https://github.com/karmada-io/karmada/pull/5851), @chaosi-zju)
- `etcd`: update default version to 3.5.16-0. ([#5854](https://github.com/karmada-io/karmada/pull/5854), @chaosi-zju)

### Helm Charts
- `Helm chart`: Added helm index for v1.10.0 and v1.11.0 release. ([#5579](https://github.com/karmada-io/karmada/pull/5579), @chaosi-zju)

### Instrumentation
- Unique controller names and remove ambitions when reporting metrics. ([#5799](https://github.com/karmada-io/karmada/pull/5799), @chaosi-zju)

## Contributors
Thank you to everyone who contributed to this release!

Users whose commits are in this release (alphabetically by username)

- @a7i
- @ahorine
- @anujagrawal699
- @B1f030
- @chaosi-zju
- @CharlesQQ
- @chaunceyjiang
- @husnialhamdani
- @iawia002
- @ipsum-0320
- @jabellard
- @jklaw90
- @KhalilSantana
- @LavredisG
- @liangyuanpeng
- @LivingCcj
- @MAVRICK-1
- @mohamedawnallah
- @mszacillo
- @RainbowMango
- @SataQiu
- @seanlaii
- @sophiefeifeifeiya
- @tiansuo114
- @wangxf1987
- @whitewindmills
- @wulemao
- @XiShanYongYe-Chang
- @xovoxy
- @yanfeng1992
- @yelshall
- @zach593
- @zhzhuang-zju

# v1.12.0-beta.0
## Downloads for v1.12.0-beta.0

Download v1.12.0-beta.0 in the [v1.12.0-beta.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.12.0-beta.0).

## Changelog since v1.12.0-alpha.1

## Urgent Update Notes

## Changes by Kind

### API Changes
- Introduced `SecretRef` to `Karmada` API as part of the configuration for connecting to an external etcd cluster can be used to reference a secret that contains credentials for connecting to an external etcd cluster. ([#5699](https://github.com/karmada-io/karmada/pull/5699), @jabellard)

### Features & Enhancements
- `karmada-scheduler-estimator`: grpc connection adds the support for custom DNS Domain. ([#5472](https://github.com/karmada-io/karmada/pull/5472), @zhzhuang-zju)
- `karmada-operator`: The new `SecretRef` field added as part of the configuration for connecting to an external etcd cluster can be used to reference a secret that contains credentials for connecting to an external etcd cluster. ([#5699](https://github.com/karmada-io/karmada/pull/5699), @jabellard)
- `karmada-operator`: Adds one-click script to install a Karmada instance through the `karmada-operator`. ([#5519](https://github.com/karmada-io/karmada/pull/5519), @zhzhuang-zju)
- `karmada-controller-manager`: keep preserveResourcesOnDeletion of the dependent resource consistent with that of the primary resource. ([#5717](https://github.com/karmada-io/karmada/pull/5717), @XiShanYongYe-Chang)
- `karmada-controller-manager`: set conflictResolution for dependent resources. ([#4418](https://github.com/karmada-io/karmada/pull/4418), @chaunceyjiang)
- `karmadactl`: `karmadactl init` supports deployment through configuration files. ([#5357](https://github.com/karmada-io/karmada/pull/5357), @tiansuo114)
- `karmadactl`: new command `karmadactl unregister` supports unregister a pull mode cluster. ([#5626](https://github.com/karmada-io/karmada/pull/5626), @wulemao)
- `karmada-scheduler`: implement group score calculation instead of take the highest score of clusters. ([#5621](https://github.com/karmada-io/karmada/pull/5621), @ipsum-0320)

### Deprecation
- `ExternalEtcd.CAData`, `ExternalEtcd.CertData` and `ExternalEtcd.KeyData` in `Karmada` API are deprecated and will be removed in a future version. Use SecretRef for providing client connection credentials. ([#5699](https://github.com/karmada-io/karmada/pull/5699), @jabellard)

### Bug Fixes
- `karmada-scheduler`: Fixed unexpected modification of original `ResourceSummary` due to lack of deep copy. ([#5685](https://github.com/karmada-io/karmada/pull/5685), @LivingCcj)
- `karmada-scheduler`: Fixes an issue where resource model grades were incorrectly matched based on resource requests. Now only grades that can provide sufficient resources will be selected. ([#5706](https://github.com/karmada-io/karmada/pull/5706), @RainbowMango)
- `karmada-search`: Modify the logic of checking whether the resource is registered when selecting the plugin. ([#5662](https://github.com/karmada-io/karmada/pull/5662), @yanfeng1992)

### Security
- `karmada-operator`: minimize the rbac permissions for karmada-operator. ([#5586](https://github.com/karmada-io/karmada/pull/5586), @B1F030)

## Other
### Dependencies

### Helm Charts

### Instrumentation

# v1.12.0-alpha.1
## Downloads for v1.12.0-alpha.1

Download v1.12.0-alpha.1 in the [v1.12.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.12.0-alpha.1).

## Changelog since v1.11.0

## Urgent Update Notes

## Changes by Kind

### API Changes
- Introduced `extraVolumes` and `extraVolumemounts` to the `Karmada` API to optionally specify extra volumes and volume mounts for the Karmada API server component. ([#5509](https://github.com/karmada-io/karmada/pull/5509), @jabellard)
- Introduced a new condition `CompleteAPIEnablements` to represent api collection status of clusters. ([#5400](https://github.com/karmada-io/karmada/pull/5400), @whitewindmills)
- Introduced `PreserveResourcesOnDeletion` field to both PropagationPolicy and ClusterPropagationPolicy API, which provides the ability to roll back migration safely. ([#5575](https://github.com/karmada-io/karmada/pull/5575), @RainbowMango)
- API Change: Introduced `FieldOverrider` to both OverridePolicy and ClusterOverridePolicy, which provides the ability to override structured data nested in manifest like ConfigMap or Secret. ([#5581](https://github.com/karmada-io/karmada/pull/5581), @RainbowMango)

### Features & Enhancements
- implement preserveResourcesOnDeletion to support migration rollback. ([#5597](https://github.com/karmada-io/karmada/pull/5597), @a7i)
- Introduced `FieldOverrider` for overriding values in JSON and YAML. ([#5591](https://github.com/karmada-io/karmada/pull/5591), @sophiefeifeifeiya)
- `karmadactl`: Implementing autocompletion for karmadactl to save a lot of typing. ([#5533](https://github.com/karmada-io/karmada/pull/5533), @zhzhuang-zju)
- `karmadactl`: Added shorthand letter `s` to 'operation-scope' flags across commands. ([#5483](https://github.com/karmada-io/karmada/pull/5483), @ahorine)
- `karmadactl`: `karmadactl init` support multiple label selection ability with flag `EtcdNodeSelectorLabels`. ([#5321](https://github.com/karmada-io/karmada/pull/5321), @tiansuo114)
- `karmadactl`: set `PreserveResourcesOnDeletion` by default in auto-created propagation policy during promotion process. ([#5601](https://github.com/karmada-io/karmada/pull/5601), #wulemao)
- `karmada-sheduler`: The `scheduler-estimator-service-namespace` flag is introduced, which can be used to explicitly specify the namespace that should be used to discover scheduler estimator services. For backwards compatibility, when not explicitly set, the default value of `karmada-system` is retained. ([#5478](https://github.com/karmada-io/karmada/pull/5478), @jabellard)
- `karmada-desheduler`: The `scheduler-estimator-service-namespace` flag is introduced, which can be used to explicitly specify the namespace that should be used to discover scheduler estimator services. For backwards compatibility, when not explicitly set, the default value of `karmada-system` is retained. ([#5478](https://github.com/karmada-io/karmada/pull/5478), @jabellard)
- `karmada-controller-manager`: The health status of resources without ResourceInterpreter customization will be treated as healthy by default. ([#5530](https://github.com/karmada-io/karmada/pull/5530), @a7i)
- `karmada-webhook`: validate fieldOverrider operation. ([#5671](https://github.com/karmada-io/karmada/pull/5671), @chaunceyjiang)

### Deprecation
- The following flags have been deprecated in release `v1.11.0` and now have been removed: 
    * `karmada-agent`: ([#5548](https://github.com/karmada-io/karmada/pull/5548), @whitewindmills)
        * --bind-address
        * --secure-port
    * `karmada-controller-manager`: ([#5549](https://github.com/karmada-io/karmada/pull/5549), @whitewindmills)
        * --bind-address
        * --secure-port
    * `karmada-scheduler-estimator`: ([#5555](https://github.com/karmada-io/karmada/pull/5555), @seanlaii)
        * --bind-address
        * --secure-port
    * `karmada-scheduler`: ([#5551](https://github.com/karmada-io/karmada/pull/5551), @chaosi-zju)
        * --bind-address
        * --secure-port
    * `karmada-descheduler`: ([#5552](https://github.com/karmada-io/karmada/pull/5552), @chaosi-zju)
        * --bind-address
        * --secure-port

### Bug Fixes
- `karmada-operator`: Fixed the issue where the manifests for the `karmada-scheduler` and `karmada-descheduler` components were not parsed correctly. ([#5546](https://github.com/karmada-io/karmada/pull/5546), @jabellard)
- `karmada-operator`: Fixed `system:admin` can not proxy to member cluster issue. ([#5572](https://github.com/karmada-io/karmada/pull/5572), @chaosi-zju)
- `karmada-aggregate-apiserver`: limit aggregate apiserver http method to get. User can modify member cluster's object with * in aggregated apiserver url. ([#5430](https://github.com/karmada-io/karmada/pull/5430), @spiritNO1)
- `karmada-scheduler`: skip the filter if the cluster is already in the list of scheduling result even if the API is missed. ([#5216](https://github.com/karmada-io/karmada/pull/5216), @yanfeng1992)
- `karmada-controller-manager`: Ignored StatefulSet Dependencies with PVCs created via the VolumeClaimTemplates. ([#5568](https://github.com/karmada-io/karmada/pull/5568), @jklaw90)
- `karmada-controller-manager`: Clean up the residual annotations when resources are preempted by pp from cpp. ([#5563](https://github.com/karmada-io/karmada/pull/5563), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed an issue that policy claim metadata might be lost during the rapid deletion and creation of PropagationPolicy(s)/ClusterPropagationPolicy(s). ([#5319](https://github.com/karmada-io/karmada/pull/5319), @zhzhuang-zju)
- `karmadactl`：Fixed the issue where commands `create`, `annotate`, `delete`, `edit`, `label`, and `patch` cannot specify the namespace flag. ([#5487](https://github.com/karmada-io/karmada/pull/5487), @zhzhuang-zju)
- `karmadactl`: Fixed the issue that karmadactl addon failed to install karmada-scheduler-estimator due to unknown flag. ([#5523](https://github.com/karmada-io/karmada/pull/5523), @chaosi-zju)

### Security

## Other
### Dependencies
- `karmada-apiserver` and `kube-controller-manager` is using v1.30.4 by default. ([#5515](https://github.com/karmada-io/karmada/pull/5515), @liangyuanpeng)
- The base image `alpine` now has been promoted from `alpine:3.20.2` to `alpine:3.20.3`.
- Karmada now using Golang v1.22.7. ([#5529](https://github.com/karmada-io/karmada/pull/5529), @yelshall)

### Helm Charts
- `Helm chart`: Added helm index for v1.10.0 and v1.11.0 release. ([#5579](https://github.com/karmada-io/karmada/pull/5579), @chaosi-zju)

### Instrumentation
