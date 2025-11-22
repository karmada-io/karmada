<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.16.0-rc.0](#v1160-rc0)
  - [Downloads for v1.16.0-rc.0](#downloads-for-v1160-rc0)
  - [Changelog since v1.16.0-beta.0](#changelog-since-v1160-beta0)
  - [Urgent Update Notes](#urgent-update-notes)
  - [Changes by Kind](#changes-by-kind)
    - [API Changes](#api-changes)
    - [Features & Enhancements](#features--enhancements)
    - [Deprecation](#deprecation)
    - [Bug Fixes](#bug-fixes)
    - [Security](#security)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Charts](#helm-charts)
    - [Instrumentation](#instrumentation)
    - [Performance](#performance)
- [v1.16.0-beta.0](#v1160-beta0)
  - [Downloads for v1.16.0-beta.0](#downloads-for-v1160-beta0)
  - [Changelog since v1.16.0-alpha.2](#changelog-since-v1160-alpha2)
  - [Urgent Update Notes](#urgent-update-notes-1)
  - [Changes by Kind](#changes-by-kind-1)
    - [API Changes](#api-changes-1)
    - [Features & Enhancements](#features--enhancements-1)
    - [Deprecation](#deprecation-1)
    - [Bug Fixes](#bug-fixes-1)
    - [Security](#security-1)
  - [Other](#other-1)
    - [Dependencies](#dependencies-1)
    - [Helm Charts](#helm-charts-1)
    - [Instrumentation](#instrumentation-1)
    - [Performance](#performance-1)
- [v1.16.0-alpha.2](#v1160-alpha2)
  - [Downloads for v1.16.0-alpha.2](#downloads-for-v1160-alpha2)
  - [Changelog since v1.16.0-alpha.1](#changelog-since-v1160-alpha1)
  - [Urgent Update Notes](#urgent-update-notes-2)
  - [Changes by Kind](#changes-by-kind-2)
    - [API Changes](#api-changes-2)
    - [Features & Enhancements](#features--enhancements-2)
    - [Deprecation](#deprecation-2)
    - [Bug Fixes](#bug-fixes-2)
    - [Security](#security-2)
  - [Other](#other-2)
    - [Dependencies](#dependencies-2)
    - [Helm Charts](#helm-charts-2)
    - [Instrumentation](#instrumentation-2)
- [v1.16.0-alpha.1](#v1160-alpha1)
  - [Downloads for v1.16.0-alpha.1](#downloads-for-v1160-alpha1)
  - [Changelog since v1.15.0](#changelog-since-v1150)
  - [Urgent Update Notes](#urgent-update-notes-3)
  - [Changes by Kind](#changes-by-kind-3)
    - [API Changes](#api-changes-3)
    - [Features & Enhancements](#features--enhancements-3)
    - [Deprecation](#deprecation-3)
    - [Bug Fixes](#bug-fixes-3)
    - [Security](#security-3)
  - [Other](#other-3)
    - [Dependencies](#dependencies-3)
    - [Helm Charts](#helm-charts-3)
    - [Instrumentation](#instrumentation-3)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.16.0-rc.0
## Downloads for v1.16.0-rc.0

Download v1.16.0-rc.0 in the [v1.16.0-rc.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.16.0-rc.0).

## Changelog since v1.16.0-beta.0

## Urgent Update Notes

## Changes by Kind

### API Changes
- `karmada-operator`: Introduced a `PodDisruptionBudget` field to the `CommonSettings` of `Karmada` API for supporting PodDisruptionBudgets (PDBs) for Karmada control plane components. ([#6895](https://github.com/karmada-io/karmada/pull/6895), @baiyutang)

### Features & Enhancements
- `karmada-controller-manager`: Introduced a built-in interpreter for Kubeflow Notebooks. ([#6814](https://github.com/karmada-io/karmada/pull/6814), @dekaihu)

### Deprecation
None.

### Bug Fixes
None.

### Security
None.

## Other
### Dependencies
- Kubernetes dependencies have been updated to v1.34.1. ([#6868](https://github.com/karmada-io/karmada/pull/6868), @RainbowMango)

### Helm Charts
None.

### Instrumentation
None.

### Performance
None.

# v1.16.0-beta.0
## Downloads for v1.16.0-beta.0

Download v1.16.0-beta.0 in the [v1.16.0-beta.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.16.0-beta.0).

## Changelog since v1.16.0-alpha.2

## Urgent Update Notes

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmadactl`: The `init` command now supports customizing Karmada component command line flags via the configuration file. ([#6785](https://github.com/karmada-io/karmada/pull/6785), @luyb177)
- `karmada-controller-manager`: Introduced built-in interpreter for Volcano `Job`. ([#6790](https://github.com/karmada-io/karmada/pull/6790), @dekaihu)
- `karmada-controller-manager`: Added a new Warning event `DependencyPolicyConflict` to surface when dependency policies have conflicts. ([#6795](https://github.com/karmada-io/karmada/pull/6795), @Kexin2000)
- `karmada-controller-manager`: Compute effective field values for attached ResourceBindings when referenced by multiple ResourceBindings: - ConflictResolution: any Overwrite → Overwrite; else Abort. - PreserveResourcesOnDeletion: any true → true; else false. Applies only to dependency-generated ResourceBindings; policy-owned bindings are unaffected. ([#6796](https://github.com/karmada-io/karmada/pull/6796), @Kexin2000)
- `karmada-controller-manager`: Introduced built-in resource interpreter for Kubernetes ReplicaSet workloads. ([#6833](https://github.com/karmada-io/karmada/pull/6833), @ryanwuer)
- `karmada-controller-manager`: Introduced `--resource-eviction-rate` flag to specify the eviction rate during cluster failover. ([#6777](https://github.com/karmada-io/karmada/pull/6777), @whosefriendA)
- `karmada-scheduler`: Enabled the capability for multiple component estimation in the scheduler. The feature is gated behind MultiplePodTemplatesScheduling. ([#6857](https://github.com/karmada-io/karmada/pull/6857), @RainbowMango)
- `karmada-scheduler`: Migrate dynamic weight assignment to use the Webster algorithm. ([#6837](https://github.com/karmada-io/karmada/pull/6837), @zhzhuang-zju)
- `karmada-scheduler-estimator`: Allow adding plugins in estimator for component scheduling. ([#6864](https://github.com/karmada-io/karmada/pull/6864), @seanlaii)
- `karmada-scheduler-estimator`: Add `ResourceQuota` plugin for multi-component scheduling. ([#6875](https://github.com/karmada-io/karmada/pull/6875), @seanlaii)
- `karmada-scheduler-estimator`: implement the noderesource plugin for multi-component scheduling estimation. ([#6896](https://github.com/karmada-io/karmada/pull/6896), @zhzhuang-zju)
- `karmada-scheduler`/`karmada-scheduler-estimator`: Implements maxAvailableComponentSets for the accurate estimator. ([#6876](https://github.com/karmada-io/karmada/pull/6876), @mszacillo)
- `karmada-scheduler-estimator`: Refactors the replica estimation logic by moving the node resource-based calculation into a dedicated, default plugin. ([#6877](https://github.com/karmada-io/karmada/pull/6877), @zhzhuang-zju)

### Deprecation
- `karmada-operator`: Deprecated external etcd fields `CAData`, `CertData`, and `KeyData` have been removed. ([#6860](https://github.com/karmada-io/karmada/pull/6860), @jabellard)

### Bug Fixes
- `karmadactl`: Fixed the issue that the `register` command still uses the cluster-info endpoint when registering a pull-mode cluster, even if the user provides the API server endpoint. ([#6866](https://github.com/karmada-io/karmada/pull/6866), @ssenecal-modular)

### Security
None.

## Other
### Dependencies
- Karmada is now built with Golang v1.24.9. ([#6853](https://github.com/karmada-io/karmada/pull/6853), @rayo1uo)

### Helm Charts
None.

### Instrumentation
None.

### Performance
None.

# v1.16.0-alpha.2
## Downloads for v1.16.0-alpha.2

Download v1.16.0-alpha.2 in the [v1.16.0-alpha.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.16.0-alpha.2).

## Changelog since v1.16.0-alpha.1

## Urgent Update Notes

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmadactl`: The `init` command now supports customizing Karmada component command line flags. ([#6637](https://github.com/karmada-io/karmada/pull/6637), @luyb177)
- `karmada-scheduler-estimator`: Introduce MaxAvailableComponentSetsRequest & MaxAvailableComponentSetsResponse for component scheduling. ([#6787](https://github.com/karmada-io/karmada/pull/6787), @seanlaii)
- `karmada-scheduler`: Implemented `MaxAvailableComponentSets` interface for general estimator based on resource summary. ([#6812](https://github.com/karmada-io/karmada/pull/6812), @mszacillo)

### Deprecation
None.

### Bug Fixes
- `karmada-metrics-adapter`: Fixed a panic when querying node metrics by name caused by using the wrong GroupVersionResource (PodsGVR instead of NodesGVR) when creating a lister. ([#6838](https://github.com/karmada-io/karmada/pull/6838), @vie-serendipity)
- `karmadactl`: Fixed the issue that the `register` command still uses the cluster-info endpoint when registering a pull-mode cluster, even if the user provides the API server endpoint. ([#6866](https://github.com/karmada-io/karmada/pull/6866), @ssenecal-modular)

### Security
None.

## Other
### Dependencies
- The base image `alpine` now has been promoted from 3.22.1 to 3.22.2. ([#6822](https://github.com/karmada-io/karmada/pull/6822), @dependabot)

### Helm Charts
None.

### Instrumentation
None.

# v1.16.0-alpha.1
## Downloads for v1.16.0-alpha.1

Download v1.16.0-alpha.1 in the [v1.16.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.16.0-alpha.1).

## Changelog since v1.15.0

## Urgent Update Notes

## Changes by Kind

### API Changes
- Introduced `Components` field to `ResourceInterpreterContext` in the `ResourceInterpreterResponse` to support interpreting components for webhook interpreter. ([#6740](https://github.com/karmada-io/karmada/pull/6740), @RainbowMango)

### Features & Enhancements
- `ResourceInterpreter`: Enable `GetComponents` interpreter operation through Webhook Interpreter. ([#6745](https://github.com/karmada-io/karmada/pull/6745), @seanlaii)
- `ResourceInterpreter`: Adding maxAvailableComponentSets to estimator interface. ([#6765](https://github.com/karmada-io/karmada/pull/6765), @mszacillo)

### Deprecation
None.

### Bug Fixes
- `karmada-scheduler`: Fixed the issue where increasing the total number of replicas can cause some clusters to receive fewer replicas under the StaticWeight strategy by introducing the Webster algorithm. ([#6793](https://github.com/karmada-io/karmada/pull/6793), @RainbowMango, @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the issue that `rbSpec.Components` is not updated when the template is updated. ([#6723](https://github.com/karmada-io/karmada/pull/6723), @zhzhuang-zju)
- `ResourceInterpreter`: Fixed the issue that when an object API field name contains dots or colons, it would cause the resource interpreter to fail. ([#6749](https://github.com/karmada-io/karmada/pull/6749), @zhzhuang-zju)
- `karmada-webhook`: Fixed the issue that resourcebinding validating webhook may panic when ReplicaRequirements of a Component in rbSpec.Components is nil. ([#6755](https://github.com/karmada-io/karmada/pull/6755), @zhzhuang-zju)
- `karmada-operator`: Fixed the issue that CRDs can not be updated during upgrades of the Karmada instance. ([#6775](https://github.com/karmada-io/karmada/pull/6775), @jabellard)

### Security
- Bump github.com/vektra/mockery to v3.5.5 to address security concerns(GO-2025-3900). ([#6761](https://github.com/karmada-io/karmada/pull/6761), @RainbowMango)

## Other
### Dependencies

### Helm Charts
- `Helm chart`: Added helm index for 1.15 release. ([#6727](https://github.com/karmada-io/karmada/pull/6727), @liaolecheng)

### Instrumentation
None.
