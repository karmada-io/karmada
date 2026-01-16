<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.17.0-alpha.2](#v1170-alpha2)
  - [Downloads for v1.17.0-alpha.2](#downloads-for-v1170-alpha2)
  - [Changelog since v1.17.0-alpha.1](#changelog-since-v1170-alpha1)
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
- [v1.17.0-alpha.1](#v1170-alpha1)
  - [Downloads for v1.17.0-alpha.1](#downloads-for-v1170-alpha1)
  - [Changelog since v1.17.0-alpha.0](#changelog-since-v1170-alpha0)
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

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.17.0-alpha.2
## Downloads for v1.17.0-alpha.2

Download v1.17.0-alpha.2 in the [v1.17.0-alpha.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.17.0-alpha.2).

## Changelog since v1.17.0-alpha.1

## Urgent Update Notes

## Changes by Kind

### API Changes
- `karmada-operator`: Introduced `Tolerations` and `Affinity` fields to the `CommonSettings` of `Karmada` API for supporting explicit tolerations and affinity for Karmada control plane components. ([#6480](https://github.com/karmada-io/karmada/pull/6480), @abhinav-1305)

### Features & Enhancements
- `karmada-operator`: Now supports explicit tolerations and affinity for all Karmada control plane components via the `Karmada` CR. ([#6480](https://github.com/karmada-io/karmada/pull/6480), @abhinav-1305)

### Deprecation
None.

### Bug Fixes
- `karmada-controller-manager`: Fixed an issue where policy deletion could be blocked if a resource selector targeted a non-existent resource. ([#7038](https://github.com/karmada-io/karmada/pull/7038), @zhzhuang-zju)
- `karmada-webhook`: Fixed an issue where the `condition.reason` was not set to `QuotaExceeded` when FederatedResourceQuota is exceeded. ([#7086](https://github.com/karmada-io/karmada/pull/7086), @kajal-jotwani)

### Security
None.

## Other
### Dependencies
None.

### Helm Charts
None.

### Instrumentation
- `Instrumentation`: The metric `work_sync_workload_duration_seconds` no longer counts retriable Kubernetes 409 conflicts as errors, improving availability accuracy and reducing false alerts caused by conflict retry flapping. ([#7106](https://github.com/karmada-io/karmada/pull/7106), @RainbowMango)

### Performance
None.

# v1.17.0-alpha.1
## Downloads for v1.17.0-alpha.1

Download v1.17.0-alpha.1 in the [v1.17.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.17.0-alpha.1).

## Changelog since v1.17.0-alpha.0

## Urgent Update Notes

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-operator`: Added support for configuring priority class and pod disruption budget config for Karmada operator deployment. ([#7015](https://github.com/karmada-io/karmada/pull/7015), @jabellard)

### Deprecation
- `karmadactl`: `Etcd.Local.InitImage` in `Karmada Init Configuration` has been deprecated and will be removed in a future version. ([#6995](https://github.com/karmada-io/karmada/pull/6995), @zhzhuang-zju)

### Bug Fixes
- `karmada-scheduler`: Fixed the bug in the backoff queue where the sorting function was incorrect, potentially causing high-priority items with long backoffs to block lower-priority items. ([#6987](https://github.com/karmada-io/karmada/pull/6987), @rayo1uo)
- `karmadactl`: Fixed the messy auto-completion suggestions for commands like 'get' and 'apply'. ([#7023](https://github.com/karmada-io/karmada/pull/7023), @zhzhuang-zju)
- `karmadactl`: Removed the deprecated flag `--etcd-init-image` of `init` command. ([#6974](https://github.com/karmada-io/karmada/pull/6974), @AbhinavPInamdar)
- `karmada-controller-manager`: Fixed the issue where PP/CPP cannot be deleted because the resources API selected by the PP/CPP do not exist on the control plane. ([#7024](https://github.com/karmada-io/karmada/pull/7024), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that `HelmRelease` did not define `observedGeneration` variable in the `statusAggregation` operation. ([#7057](https://github.com/karmada-io/karmada/pull/7057), @FAUST-BENCHOU)

### Security
None.

## Other
### Dependencies
- Kubernetes dependencies have been updated to `v1.34.2`. ([#6999](https://github.com/karmada-io/karmada/pull/6999), @RainbowMango)
- The base image `alpine` has been promoted from `alpine:3.22.2` to `alpine:3.23.0`. ([#7001](https://github.com/karmada-io/karmada/pull/7001), @dependabot)
- The base image `alpine` has been promoted from `alpine:3.23.0` to `alpine:3.23.2`. ([#7034](https://github.com/karmada-io/karmada/pull/7034), @dependabot)

### Helm Charts
- `Helm chart`: Added helm index for 1.16 release. ([#6990](https://github.com/karmada-io/karmada/pull/6990), @zhzhuang-zju)
- `helm`: Upgraded `bitnami/common` dependency in karmada operator chart from `1.17.1` to `2.31.4`. ([#6994](https://github.com/karmada-io/karmada/pull/6994), @zhzhuang-zju)

### Instrumentation
None.

### Performance
None.
