<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.20.0-alpha.1](#v1200-alpha1)
  - [Downloads for v1.20.0-alpha.1](#downloads-for-v1200-alpha1)
  - [Changelog since v1.20.0-alpha.0](#changelog-since-v1200-alpha0)
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
