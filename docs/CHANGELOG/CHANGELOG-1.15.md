<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.15.0-alpha.2](#v1150-alpha2)
  - [Downloads for v1.15.0-alpha.2](#downloads-for-v1150-alpha2)
  - [Changelog since v1.15.0-alpha.1](#changelog-since-v1150-alpha1)
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
- [v1.15.0-alpha.1](#v1150-alpha1)
  - [Downloads for v1.15.0-alpha.1](#downloads-for-v1150-alpha1)
  - [Changelog since v1.14.0](#changelog-since-v1140)
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

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

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
