<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.19.0-rc.0](#v1190-rc0)
  - [Downloads for v1.19.0-rc.0](#downloads-for-v1190-rc0)
  - [Changelog since v1.19.0-beta.0](#changelog-since-v1190-beta0)
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

- [v1.19.0-beta.0](#v1190-beta0)
  - [Downloads for v1.19.0-beta.0](#downloads-for-v1190-beta0)
  - [Changelog since v1.19.0-alpha.1](#changelog-since-v1190-alpha1)
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

- [v1.19.0-alpha.1](#v1190-alpha1)
  - [Downloads for v1.19.0-alpha.1](#downloads-for-v1190-alpha1)
  - [Changelog since v1.19.0-alpha.0](#changelog-since-v1190-alpha0)
  - [Urgent Update Notes](#urgent-update-notes)
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
    - [Performance](#performance)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.19.0-rc.0

## Downloads for v1.19.0-rc.0

Download v1.19.0-rc.0 from the [v1.19.0-rc.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.19.0-rc.0).

## Changelog since v1.19.0-beta.0

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
- `Karmada API`: Removed the deprecated `PurgeMode` values `Immediately` and `Graciously`. Use `Directly` and `Gracefully` instead. ([#7792](https://github.com/karmada-io/karmada/pull/7792), @FengyuanYin)

### Features & Enhancements
- `karmada-controller-manager`: Added support for automatically refreshing member cluster credentials for push-mode informers, ensuring long-lived resource watches continue working after token rotation. ([#7663](https://github.com/karmada-io/karmada/pull/7663), @zhuyulicfc49)
- `karmada-scheduler`: Moved bindings whose backoff or unschedulable timeout completed into the active queue in priority order within a single flush, so higher-priority bindings were retried first on a best-effort basis. ([#7814](https://github.com/karmada-io/karmada/pull/7814), @CaesarTY)

### Deprecation
None.

### Bug Fixes
- `karmada-scheduler-estimator`: Fixed the issue that installing `schedulerEstimator` as a standalone component (`installMode: component`) failed to mount its certificate Secret because the Secret name was derived from the component release name instead of the host-mode release that created it. Added `schedulerEstimator.certs` (default `karmada-cert`) to configure the Secret name explicitly. ([#7815](https://github.com/karmada-io/karmada/pull/7815), @pujitha24)

### Security
None.

## Other

### Dependencies
None.

### Helm Charts
None.

### Instrumentation
None.

### Performance
None.

# v1.19.0-beta.0
## Downloads for v1.19.0-beta.0

Download v1.19.0-beta.0 in the [v1.19.0-beta.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.19.0-beta.0).

## Changelog since v1.19.0-alpha.1

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
None.

### Features & Enhancements
None.

### Deprecation
None.

### Bug Fixes
- `karmada-controller-manager`: Fixed an issue where the taint-manager eviction queue would enqueue bindings with indefinite taint tolerations. ([#7613](https://github.com/karmada-io/karmada/pull/7613), @mszacillo)
- `karmada-controller-manager`: Fixed an issue where `Cluster.status.remedyActions` could remain stale after an associated `Remedy` resource was removed. ([#7777](https://github.com/karmada-io/karmada/pull/7777), @ranxi2001)
- `karmada-scheduler`: Fixed the issue that WorkloadRebalancer-triggered rescheduling did not reevaluate multiple `clusterAffinities` in policy order starting from the first term. ([#5425](https://github.com/karmada-io/karmada/pull/5425), @bharathguvvala)

### Security
None.

## Other
### Dependencies
- Karmada is now built with Golang v1.26.5. ([#7786](https://github.com/karmada-io/karmada/pull/7786), @SipengShen01)

### Helm Charts
- `Helm chart`: Added helm index for `v1.17.4`. ([#7701](https://github.com/karmada-io/karmada/pull/7701), @github-actions)
- `Helm chart`: Added helm index for `v1.18.1`. ([#7700](https://github.com/karmada-io/karmada/pull/7700), @github-actions)

### Instrumentation
None.

### Performance
None.

# v1.19.0-alpha.1
## Downloads for v1.19.0-alpha.1

Download v1.19.0-alpha.1 in the [v1.19.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.19.0-alpha.1).

## Changelog since v1.19.0-alpha.0

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-chart`: Added `scheduler.enableEmptyWorkloadPropagation` Helm value, which defaults to `false`. When set to `true`, the chart renders `--enable-empty-workload-propagation=true` for `karmada-scheduler`. ([#7570](https://github.com/karmada-io/karmada/pull/7570), @tamarubin)

### Deprecation
- `karmada-scheduler-estimator`: The proto messages `ReplicaRequirements.resourceRequest`, `ComponentReplicaRequirements.resourceRequest`, `NodeClaim.nodeAffinity`, and `NodeClaim.tolerations` have been removed. ([#7590](https://github.com/karmada-io/karmada/pull/7590), @zhzhuang-zju)

### Bug Fixes
- `helm chart`: Fixed TLS certificate SAN mismatch when deploying to a custom namespace by adding systemNamespace SANs to certs.auto.hosts. ([#7624](https://github.com/karmada-io/karmada/pull/7624), @Priyanshu-u07)
- `karmadactl`: Fixed the issue that `init` silently used `127.0.0.1` when `--cert-external-ip` was set to an invalid value. ([#7656](https://github.com/karmada-io/karmada/pull/7656), @Anand-240)

### Security
None.

## Other

### Dependencies
- Karmada is now built with Golang v1.26.4. ([#7600](https://github.com/karmada-io/karmada/pull/7600), @RainbowMango)
- Kubernetes dependencies have been updated to v1.36.2. ([#7634](https://github.com/karmada-io/karmada/pull/7634), @RainbowMango)
- The base image `alpine` has been promoted from `alpine:3.23.4` to `alpine:3.24.0` to address security concerns. ([#7627](https://github.com/karmada-io/karmada/pull/7627), @dependabot)

### Helm Charts
- `Helm chart`: Added helm index for `v1.17.3`. ([#7589](https://github.com/karmada-io/karmada/pull/7589), @github-actions)
- `Helm chart`: Added helm index for `v1.18.0`. ([#7588](https://github.com/karmada-io/karmada/pull/7588), @github-actions)
- `Helm chart`: Updated the default `kube-apiserver` and `kube-controller-manager` images from v1.35.2 to v1.36.2, and updated the default etcd image from 3.6.6-0 to 3.6.8-0. ([#7666](https://github.com/karmada-io/karmada/pull/7666), @ranxi2001)

### Instrumentation
None.

### Performance
None.
