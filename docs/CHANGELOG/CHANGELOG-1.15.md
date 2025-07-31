<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.15.0-beta.0](#v1150-beta0)
  - [Downloads for v1.15.0-beta.0](#downloads-for-v1150-beta0)
  - [Changelog since v1.15.0-alpha.2](#changelog-since-v1150-alpha2)
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
- [v1.15.0-alpha.2](#v1150-alpha2)
  - [Downloads for v1.15.0-alpha.2](#downloads-for-v1150-alpha2)
  - [Changelog since v1.15.0-alpha.1](#changelog-since-v1150-alpha1)
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
- [v1.15.0-alpha.1](#v1150-alpha1)
  - [Downloads for v1.15.0-alpha.1](#downloads-for-v1150-alpha1)
  - [Changelog since v1.14.0](#changelog-since-v1140)
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

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.15.0-beta.0
## Downloads for v1.15.0-beta.0

Download v1.15.0-beta.0 in the [v1.15.0-beta.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.15.0-beta.0).

## Changelog since v1.15.0-alpha.2

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-controller-manager`: Added resource interpreter support for OpenKruise SidecarSet. Karmada can now interpret and manage OpenKruise SidecarSet resources across clusters, including multi-cluster status aggregation, health checks, dependency resolution for ConfigMaps and Secrets, and comprehensive test coverage. ([#6524](https://github.com/karmada-io/karmada/pull/6524), @abhi0324)
- `karmada-controller-manager`: Added resource interpreter support for OpenKruise UnitedDeployment. Karmada can now interpret and manage OpenKruise UnitedDeployment resources across clusters, including multi-cluster status aggregation, health checks, dependency resolution for ConfigMaps and Secrets, and comprehensive test coverage. ([#6533](https://github.com/karmada-io/karmada/pull/6533), @abhi0324)

### Deprecation
- The deprecated label `propagation.karmada.io/instruction`, which was designed to suspend Work propagation, has now been removed. ([#6512](https://github.com/karmada-io/karmada/pull/6512), @XiShanYongYe-Chang)

### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that EndpointSlice are deleted unexpectedly due to the EndpointSlice informer cache not being synced. ([#6434](https://github.com/karmada-io/karmada/pull/6434), @XiShanYongYe-Chang)

### Security
- Bump go version to 1.24.5 for addressing CVE-2025-4674 concern. ([#6557](https://github.com/karmada-io/karmada/pull/6557), @seanlaii)

## Other
### Dependencies
- Upgraded sigs.k8s.io/metrics-server to v0.8.0. ([#6548](https://github.com/karmada-io/karmada/pull/6548), @seanlaii)
- Upgraded sigs.k8s.io/kind to v0.29.0. ([#6549](https://github.com/karmada-io/karmada/pull/6549), @seanlaii)
- Upgraded vektra/mockery to v3.5.1, switching to a configuration-driven approach via mockery.yaml and removing deprecated v2 flags like --inpackage and --name. ([#6550](https://github.com/karmada-io/karmada/pull/6550), @liaolecheng)
- Upgraded controller-gen to v0.18.0. ([#6558](https://github.com/karmada-io/karmada/pull/6558), @seanlaii)

### Helm Charts
None.

### Instrumentation
None.

### Performance
None.

# v1.15.0-alpha.2
## Downloads for v1.15.0-alpha.2

Download v1.15.0-alpha.2 in the [v1.15.0-alpha.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.15.0-alpha.2).

## Changelog since v1.15.0-alpha.1

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-controller-manager`: Enhanced ServiceAccount retention logic to also preserve `imagePullSecrets`, preventing their continuous regeneration in member clusters. ([#6532](https://github.com/karmada-io/karmada/pull/6532), @whitewindmills)
- `karmada-aggregated-apiserver`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6507](https://github.com/karmada-io/karmada/pull/6507), @ritzdevp)

### Deprecation
None.

### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that resources will be recreated after being deleted on the cluster when resource is suspended for dispatching. ([#6525](https://github.com/karmada-io/karmada/pull/6525), @XiShanYongYe-Chang)

### Security
None.

## Other
### Dependencies
- Kubernetes dependencies have been updated to v1.33.2. ([#6498](https://github.com/karmada-io/karmada/pull/6498), @RainbowMango)

### Helm Charts
- `karmada-search`: karmada-search helm chart template now references the resources from `search.resources`. ([#6517](https://github.com/karmada-io/karmada/pull/6517), @seanlaii)

### Instrumentation
None.

### Performance
None.

# v1.15.0-alpha.1
## Downloads for v1.15.0-alpha.1

Download v1.15.0-alpha.1 in the [v1.15.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.15.0-alpha.1).

## Changelog since v1.14.0

## Urgent Update Notes

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-webhook`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6430](https://github.com/karmada-io/karmada/pull/6430), @seanlaii)
- `karmada-scheduler-estimator`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6457](https://github.com/karmada-io/karmada/pull/6457), @linyao22)
- `karmada-scheduler`: Introduced `--logging-format` flag, which can be set to `json` to enable JSON logging. ([#6473](https://github.com/karmada-io/karmada/pull/6473), @zzklachlan)
- `karmada-search`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6466](https://github.com/karmada-io/karmada/pull/6466), @liwang0513)
- `karmada-metrics-adapter`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6439](https://github.com/karmada-io/karmada/pull/6439), @nihar4276)
- `karmada-descheduler`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6493](https://github.com/karmada-io/karmada/pull/6493), @mszacillo)
- `karmada-operator`: Introduced `--logging-format` flag which can be set to `json` to enable JSON logging. ([#6496](https://github.com/karmada-io/karmada/pull/6496), @LeonZh0u)

### Deprecation
- `karmada-webhook`: The `--default-not-ready-toleration-seconds` and `--default-unreachable-toleration-seconds` flags which were deprecated in release-1.14, now has been removed. ([#6444](https://github.com/karmada-io/karmada/pull/6444), @XiShanYongYe-Chang)
- `karmaa-controller-manager`: The flag `--failover-eviction-timeout` has been deprecated in release-1.14, now has been removed. ([#6450](https://github.com/karmada-io/karmada/pull/6450), @XiShanYongYe-Chang)

### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that a workload propagated with `duplicated mode` can bypass quota checks during scale up. ([#6474](https://github.com/karmada-io/karmada/pull/6474), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the issue where the federated-resource-quota-enforcement-controller miscalculates quota usage. ([#6477](https://github.com/karmada-io/karmada/pull/6477), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the bug that cluster can not be unjoined in case of the `--enable-taint-manager=false` or the feature gate `Failover` is disabled. ([#6446](https://github.com/karmada-io/karmada/pull/6446), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that `taint-maanger` didn't honour`--no-execute-taint-eviction-purge-mode` when evicting `ClusterResourceBinding`. ([#6491](https://github.com/karmada-io/karmada/pull/6491), @XiShanYongYe-Chang)
- `helm`:  Fixed the issue where `helm upgrade` failed to update Karmada's static resources properly. ([#6395](https://github.com/karmada-io/karmada/pull/6395), @deefreak)

### Security
None.

## Other
### Dependencies
- The base image `alpine` now has been promoted from 3.21.3 to 3.22.0. ([#6419](https://github.com/karmada-io/karmada/pull/6419))
- Bump go version to 1.24.4. ([#6490](https://github.com/karmada-io/karmada/pull/6490), @seanlaii)

### Helm Charts
- `Chart`: Introduced parameter `certs.auto.rootCAExpiryDays` for root ca certification expiry customization. ([#6447](https://github.com/karmada-io/karmada/pull/6447), @ryanwuer)

### Instrumentation
None.
