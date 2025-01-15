<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.13.0-alpha.2](#v1130-alpha2)
  - [Downloads for v1.13.0-alpha.2](#downloads-for-v1130-alpha2)
  - [Changelog since v1.13.0-alpha.1](#changelog-since-v1130-alpha1)
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
- [v1.13.0-alpha.1](#v1130-alpha1)
  - [Downloads for v1.13.0-alpha.1](#downloads-for-v1130-alpha1)
  - [Changelog since v1.12.0](#changelog-since-v1120)
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

# v1.13.0-alpha.2
## Downloads for v1.13.0-alpha.2

Download v1.13.0-alpha.2 in the [v1.13.0-alpha.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.0-alpha.2).

## Changelog since v1.13.0-alpha.1

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-metrics-adapter`: Introduced `--metrics-bind-address` flag which will be used to expose Prometheus metrics. ([#6013](https://github.com/karmada-io/karmada/pull/6013), @chaosi-zju)

### Deprecation
- Replace `grpc.DialContext` and `grpc.WithBlock` with `grpc.NewClient` since DialContext and WithBlock are deprecated while maintaining the original functionality. ([#6026](https://github.com/karmada-io/karmada/pull/6026), @seanlaii)

### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that newly created attached-ResourceBinding be mystically garbage collected. ([#6034](https://github.com/karmada-io/karmada/pull/6034), @whitewindmills)

### Security
None.

## Other
### Dependencies
- The base image `alpine` now has been promoted from 3.21.0 to 3.21.2. ([#6040](https://github.com/karmada-io/karmada/pull/6040))

### Helm Charts
None.

### Instrumentation
None.

# v1.13.0-alpha.1
## Downloads for v1.13.0-alpha.1

Download v1.13.0-alpha.1 in the [v1.13.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.0-alpha.1).

## Changelog since v1.12.0

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
- `API Change`: Introduced `Scheduling` suspension in both `ResourceBinding` and `ClusterResourceBinding` which will be used for third-party systems to suspend application scheduling. ([#5937](https://github.com/karmada-io/karmada/pull/5937), @Monokaix)

### Features & Enhancements
- `karmadactl`: Add Fish shell autocompletion support for improved command-line efficiency. ([#5876](https://github.com/karmada-io/karmada/pull/5876), @tiansuo114)

### Deprecation
- `karmadactl`: The flag `--ca-cert-path` of command `register` has been marked deprecated because it has never been used, and will be removed in the future release. ([#5862](https://github.com/karmada-io/karmada/pull/5862), @zhzhuang-zju)

### Bug Fixes
- `karmada-controller-manager`: Fixed the problem of ResourceBinding remaining after the resource template is deleted in the dependencies distribution scenario. ([#5943](https://github.com/karmada-io/karmada/pull/5943), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the bug of WorkloadRebalancer doesn't get deleted after TTL. ([#5989](https://github.com/karmada-io/karmada/pull/5989), @chaosi-zju)
- `karmada-controller-manager`: Fixed the issue of missing work queue metrics. ([#5972](https://github.com/karmada-io/karmada/pull/5972), @XiShanYongYe-Chang)
- `karmada-webhook`: Fixed panic when validating ResourceInterpreterWebhookConfiguration with unspecified service port. ([#5960](https://github.com/karmada-io/karmada/pull/5960), @XiShanYongYe-Chang)
- `karmada-operator`: Fixed the issue that external ETCD certificate be overwritten by generated in-cluster ETCD certificate. ([#5976](https://github.com/karmada-io/karmada/pull/5976), @jabellard)

### Security
None.

## Other
### Dependencies
- update kubernetes version to v1.31.3 ([#5910](https://github.com/karmada-io/karmada/pull/5910), @dongjiang1989)
- The base image `alpine` now has been promoted from `3.20.3` to `3.21.0`. ([#5920](https://github.com/karmada-io/karmada/pull/5920))

### Helm Charts
- upgrade helm chart index to v1.12.0. ([#5918](https://github.com/karmada-io/karmada/pull/5918), @chaosi-zju)

### Instrumentation
None.
