<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.20.0-beta.0](#v1200-beta0)
  - [Downloads for v1.20.0-beta.0](#downloads-for-v1200-beta0)
  - [Changelog since v1.20.0-alpha.1](#changelog-since-v1200-alpha1)
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
- [v1.20.0-alpha.1](#v1200-alpha1)
  - [Downloads for v1.20.0-alpha.1](#downloads-for-v1200-alpha1)
  - [Changelog since v1.20.0-alpha.0](#changelog-since-v1200-alpha0)
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

# v1.20.0-beta.0
## Downloads for v1.20.0-beta.0

Download v1.20.0-beta.0 in the [v1.20.0-beta.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.20.0-beta.0).

## Changelog since v1.20.0-alpha.1

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
- `ResourceBinding`: Introduced `spec.schedulePriority.preemptionPolicy` to both the `ResourceBinding` and `ClusterResourceBinding` APIs to specify whether a binding may preempt lower-priority bindings. ([#7954](https://github.com/karmada-io/karmada/pull/7954), @RainbowMango)

### Features & Enhancements
- `karmada-agent`: Introduced the `--enable-cluster-registration` flag (default `true`). When set to `false`, the agent skips self-registration and validates that the member cluster is already registered in Pull mode before running its controllers. Registration-only flags are rejected in this mode. ([#7821](https://github.com/karmada-io/karmada/pull/7821), @driegel1)
- `karmada-scheduler`: Added support for rescheduling multi-template workloads when component replicas change. ([#7830](https://github.com/karmada-io/karmada/pull/7830), @ranxi2001)
- `karmada-controller-manager`: Added propagation of the preemption policy from Kubernetes `PriorityClass` to `ResourceBinding` when priority-based preemptive scheduling is enabled. ([#7978](https://github.com/karmada-io/karmada/pull/7978), @asarj)

### Deprecation
- `Karmada`: Image publication to the Huawei Cloud SWR registry (`swr.ap-southeast-1.myhuaweicloud.com/karmada`) has been discontinued starting with release 1.20. Users should pull images from DockerHub (`docker.io/karmada`) instead. ([#7950](https://github.com/karmada-io/karmada/pull/7950), @RainbowMango)

### Bug Fixes
- `karmadactl`: Fixed the missing `ClusterTaintPolicy` validating webhook registration in installations created by `karmadactl init`. The webhook denies create and update requests for policies with a `NoExecute` taint unless `karmada-webhook` runs with `--allow-no-execute-taint-policy=true`; users managing these policies must set the flag before upgrading. ([#7963](https://github.com/karmada-io/karmada/pull/7963), @cmontemuino)
- `karmada-operator`: Fixed the missing `ClusterTaintPolicy` validating webhook registration in operator-managed installations. The webhook denies create and update requests for policies with a `NoExecute` taint unless `karmada-webhook` runs with `--allow-no-execute-taint-policy=true`; users managing these policies must set the flag before upgrading. ([#7963](https://github.com/karmada-io/karmada/pull/7963), @cmontemuino)
- `karmada-controller-manager`: Fixed an issue where the built-in `MPIJob` interpreter could return components in a different order for the same object. ([#7970](https://github.com/karmada-io/karmada/pull/7970), @Sarthak-Shreshtha01)
- `karmada-controller-manager`: Fixed a panic in `CronFederatedHPA` when `failedHistoryLimit` is 0 and a rule fails. ([#7971](https://github.com/karmada-io/karmada/pull/7971), @Sarthak-Shreshtha01)

### Security
None.

## Other

### Dependencies
- `Karmada`: Upgraded the Go build toolchain to v1.26.9. ([#7932](https://github.com/karmada-io/karmada/pull/7932), @FengyuanYin)
- `Karmada`: Upgraded the Go build toolchain to v1.26.10. ([#7959](https://github.com/karmada-io/karmada/pull/7959), @FengyuanYin)
- `Karmada`: Upgraded the Alpine base image from `alpine:3.24.1` to `alpine:3.24.2` to address security concerns. ([#7912](https://github.com/karmada-io/karmada/pull/7912), @dependabot)
- `Karmada`: Updated Kubernetes dependencies to v1.37.1. ([#7974](https://github.com/karmada-io/karmada/pull/7974), @RainbowMango)

### Helm Charts
- `Helm chart`: Introduced `extraCommandArgs` for the `scheduler`, `descheduler`, `schedulerEstimator`, `agent`, `aggregatedApiServer`, `apiServer`, `metricsAdapter`, and `search` components, allowing users to pass extra command-line arguments. ([#7639](https://github.com/karmada-io/karmada/pull/7639), @rakshityadav1868)
- `Helm chart`: Added `webhook.extraCommandArgs` to pass extra command-line flags to `karmada-webhook`. ([#7953](https://github.com/karmada-io/karmada/pull/7953), @cmontemuino)
- `Helm chart`: Fixed the missing `ClusterTaintPolicy` validating webhook registration in Helm installations. The webhook denies create and update requests for policies with a `NoExecute` taint unless `karmada-webhook` runs with `--allow-no-execute-taint-policy=true`; users managing these policies must set the flag before upgrading. ([#7963](https://github.com/karmada-io/karmada/pull/7963), @cmontemuino)

### Instrumentation
None.

### Performance
None.

# v1.20.0-alpha.1
## Downloads for v1.20.0-alpha.1

Download v1.20.0-alpha.1 in the [v1.20.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.20.0-alpha.1).

## Changelog since v1.20.0-alpha.0

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
None.

### Features & Enhancements
None.

### Deprecation
- `karmada-controller-manager`: The `recreate` label of the `create_resource_to_cluster` metric, which was deprecated in v1.19 and always reported `false`, has been removed. Users should update any PromQL queries, alerts, and dashboards that filter on or group by this label. ([#7855](https://github.com/karmada-io/karmada/pull/7855), @zach593)

### Bug Fixes
None.

### Security
None.

## Other

### Dependencies
- Karmada is now built with Golang v1.26.8. ([#7893](https://github.com/karmada-io/karmada/pull/7893), @FengyuanYin)

### Helm Charts
- `Helm chart`: Added helm index for `v1.19.0`. ([#7871](https://github.com/karmada-io/karmada/pull/7871), @github-actions)

### Instrumentation
None.

### Performance
None.
