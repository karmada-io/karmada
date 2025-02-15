<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.13.0-rc.0](#v1130-rc0)
  - [Downloads for v1.13.0-rc.0](#downloads-for-v1130-rc0)
  - [Changelog since v1.13.0-beta.0](#changelog-since-v1130-beta0)
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
- [v1.13.0-beta.0](#v1130-beta0)
  - [Downloads for v1.13.0-beta.0](#downloads-for-v1130-beta0)
  - [Changelog since v1.13.0-alpha.2](#changelog-since-v1130-alpha2)
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
- [v1.13.0-alpha.2](#v1130-alpha2)
  - [Downloads for v1.13.0-alpha.2](#downloads-for-v1130-alpha2)
  - [Changelog since v1.13.0-alpha.1](#changelog-since-v1130-alpha1)
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
- [v1.13.0-alpha.1](#v1130-alpha1)
  - [Downloads for v1.13.0-alpha.1](#downloads-for-v1130-alpha1)
  - [Changelog since v1.12.0](#changelog-since-v1120)
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

# v1.13.0-rc.0
## Downloads for v1.13.0-rc.0

Download v1.13.0-rc.0 in the [v1.13.0-rc.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.0-rc.0).

## Changelog since v1.13.0-beta.0

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-controller-manager`: FlinkDeployment health interpreter improvements, adding status.error to reflected status. ([#6073](https://github.com/karmada-io/karmada/pull/6073), @mszacillo)
- `karmada-operator`: standardize the naming of karmada config in karmada-operator installation method. ([#6082](https://github.com/karmada-io/karmada/pull/6082), @seanlaii)
- `karmadactl`: command `init` now can specify the priority class name of the karmada components, default to `system-node-critical`. ([#6110](https://github.com/karmada-io/karmada/pull/6110), @zhzhuang-zju)
- `karmadactl`: The `unjoin` command is restricted to only unjoin push mode member clusters. The `unregister` command is restricted to only unregister pull mode member clusters. ([#6081](https://github.com/karmada-io/karmada/pull/6081), @zhzhuang-zju)

### Deprecation
None.

### Bug Fixes
- `karmada-operator`: fix the "no such host" error when accessing the /convert webhook if Karmada instance is deployed in a namespace other than karmada-system. ([#6079](https://github.com/karmada-io/karmada/pull/6079), @zhzhuang-zju)
- `karmadactl`: fix the "no such host" error when accessing the /convert webhook if Karmada instance is deployed in a namespace other than karmada-system via the `init` command. ([#6079](https://github.com/karmada-io/karmada/pull/6079), @zhzhuang-zju)

### Security
None.

## Other
### Dependencies
None.

### Helm Charts
- `helm`: The new `PriorityClassName` field added as part of the Karmada control plane component configurations can be used to specify the priority class name of that component, default to `system-node-critical`. ([#6108](https://github.com/karmada-io/karmada/pull/6108), @zhzhuang-zju)

### Instrumentation
- The cluster status-related metrics, emitting from `karmada-controller-manager`, will be cleaned up after the cluster is removed. ([#5866](https://github.com/karmada-io/karmada/pull/5866), @CharlesQQ)

# v1.13.0-beta.0
## Downloads for v1.13.0-beta.0

Download v1.13.0-beta.0 in the [v1.13.0-beta.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.13.0-beta.0).

## Changelog since v1.13.0-alpha.2

## Urgent Update Notes
- The Karmada Lua interpreter will no longer support the Lua functions `string.rep` and `string.gsub`. Typically, these functions are not 
frequently used in custom Karmada resource interpreters. They are disabled due to potential security risks. Before upgrading, please review 
your Lua scripts to verify whether these functions are being used. If they are, please replace them with alternative implementations.

## Changes by Kind

### API Changes
- `API Change`: Introduced `PriorityClassName` in `Karmada` API which will be used to specify the priority class name of that component. ([#6068](https://github.com/karmada-io/karmada/pull/6068), @jabellard)

### Features & Enhancements
- `karmadactl`: standardize the naming of karmada config in karmadactl installation method. ([#5797](https://github.com/karmada-io/karmada/pull/5797), @chaosi-zju)

### Deprecation
None.

### Bug Fixes
- `karmada-controller-manager`: Fixed an issue that the scheduling suspension on ResourceBinding might be mistakenly overwritten. ([#6062](https://github.com/karmada-io/karmada/pull/6062), @Monokaix)
- `karmada-search`: Fixed the issue that namespaces in different ResourceRegistry might be overwritten. ([#6065](https://github.com/karmada-io/karmada/pull/6065), @JimDevil)

### Security
- `karmada-controller-manager`: For security reasons, we made the following changes to restrict the string library functions used when users customize the Karmada Lua interpreter. ([#6087](https://github.com/karmada-io/karmada/pull/6087), @zhzhuang-zju)
  1. do not allow users to use string.gsub and string.rep when interpreting resources with lua scripts, which may be used to create overly long strings.
  2. limit the length of the string type parameters of the function to 1000,000.
  3. add timeout checks to the internal for loops within the functions.

## Other
### Dependencies
- Karmada now built with Golang v1.22.11. ([#6066](https://github.com/karmada-io/karmada/pull/6066), @y1hao)

### Helm Charts
None.

### Instrumentation
None.

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
