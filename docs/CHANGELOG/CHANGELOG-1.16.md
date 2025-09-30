<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.16.0-alpha.1](#v1160-alpha1)
  - [Downloads for v1.16.0-alpha.1](#downloads-for-v1160-alpha1)
  - [Changelog since v1.15.0](#changelog-since-v1150)
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

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

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
