<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.19.0-alpha.1](#v1190-alpha1)
  - [Downloads for v1.19.0-alpha.1](#downloads-for-v1190-alpha1)
  - [Changelog since v1.19.0-alpha.0](#changelog-since-v1190-alpha0)
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

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

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
