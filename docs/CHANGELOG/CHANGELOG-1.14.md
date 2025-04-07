<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.14.0-alpha.1](#v1140-alpha1)
  - [Downloads for v1.14.0-alpha.1](#downloads-for-v1140-alpha1)
  - [Changelog since v1.13.0](#changelog-since-v1130)
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

# v1.14.0-alpha.1
## Downloads for v1.14.0-alpha.1

Download v1.14.0-alpha.1 in the [v1.14.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.14.0-alpha.1).

## Changelog since v1.13.0

## Urgent Update Notes

## Changes by Kind

### API Changes
- Introduced `spec.customCertificate.leafCertValidityDays` field in `Karmada` API to specify a custom validity period in days for control plane leaf certificates (e.g., API server certificate). When not explicitly set, the default validity period of 1 year is used. ([#6193](https://github.com/karmada-io/karmada/pull/6193), @jabellard)

### Features & Enhancements
- `karmada-controller-manager`: All controller reconcile frequency will honor a unified rate limiter configuration(--rate-limiter-*). ([#6145](https://github.com/karmada-io/karmada/pull/6145), @CharlesQQ)
- `karmada-agent`: All controller reconcile frequency will honor a unified rate limiter configuration(--rate-limiter-*). ([#6145](https://github.com/karmada-io/karmada/pull/6145), @CharlesQQ)

### Deprecation
- `karmadactl`: The flag `--ca-cert-path` of `register`, which has been deprecated in release-1.13, now has been removed. ([#6191](https://github.com/karmada-io/karmada/pull/6191), @husnialhamdani)
- The label `propagation.karmada.io/instruction` now has been deprecated in favor of the field(`.spec.suspendDispatching`) in Work API, the label will be removed in future releases. ([#6043](https://github.com/karmada-io/karmada/pull/6043), @vie-serendipity)

### Bug Fixes
- `karmada-agent`: Fixed a panic issue where the agent does not need to report secret when registering cluster. ([#6214](https://github.com/karmada-io/karmada/pull/6214), @jabellard)
- `karmada-operator`: The `karmada-app` label key previously used for control plane components and for the components of the operator itself has been changed to the more idiomatic `app.kubernetes.io/name` label key. ([#6180](https://github.com/karmada-io/karmada/pull/6180), @jabellard)
- `karmada-controller-manager`: Fixed the issue that the gracefulEvictionTask of ResourceBinding can not be cleared in case of schedule fails. ([#6227](https://github.com/karmada-io/karmada/pull/6227), @XiShanYongYe-Chang)
- `helm`: Fixed the issue where the required ServiceAccount was missing when the certificate mode was set to custom. ([#6188](https://github.com/karmada-io/karmada/pull/6188), @seanlaii)
- Unify the rate limiter for different clients in each component to access karmada-apiserver. Access may be restricted in large-scale environments compared to before the modification. Administrators can avoid this situation by adjusting upward the rate limit parameters 'kube-api-qps' and 'kube-api-burst' for each component to access karmada-apiserver, and adjusting 'cluster-api-qps' and 'cluster-api-burst' for scheduler-estimator to access member cluster apiserver. ([#6095](https://github.com/karmada-io/karmada/pull/6095), @zach593)

### Security
None.

## Other
### Dependencies
- Karmada is now built with Golang v1.23.7. ([#6218](https://github.com/karmada-io/karmada/pull/6218), @seanlaii)
- Bump golang.org/x/net to 0.37.0. ([#6225](https://github.com/karmada-io/karmada/pull/6225), @seanlaii)
- Bump mockery to v2.53.3 to remove the hashicorp packages. ([#6212](https://github.com/karmada-io/karmada/pull/6212), @seanlaii)

### Helm Charts
- `HELM Chart`: OpenID Connect based auth can be configured for the Karmada API server when installing via Helm. ([#6159](https://github.com/karmada-io/karmada/pull/6159), @tw-mnewman)
- `Helm chart`: Added helm index for 1.13 release. ([#6196](https://github.com/karmada-io/karmada/pull/6196), @zhzhuang-zju)

### Instrumentation
- `karmada-controller-manager`: metric `recreate_resource_to_cluster` has been merged into `create_resource_to_cluster`. ([#6148](https://github.com/karmada-io/karmada/pull/6148), @zach593)
- `karmada-operator`: Introduced `karmada_build_info` metrics to emit the build info, as well as a bunch of Go runtime metrics. ([#6044](https://github.com/karmada-io/karmada/pull/6044), @dongjiang1989)
