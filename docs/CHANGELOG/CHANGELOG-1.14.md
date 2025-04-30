<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.14.0-beta.0](#v1140-beta0)
  - [Downloads for v1.14.0-beta.0](#downloads-for-v1140-beta0)
  - [Changelog since v1.14.0-alpha.2](#changelog-since-v1140-alpha2)
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
- [v1.14.0-alpha.2](#v1140-alpha2)
  - [Downloads for v1.14.0-alpha.2](#downloads-for-v1140-alpha2)
  - [Changelog since v1.14.0-alpha.1](#changelog-since-v1140-alpha1)
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
- [v1.14.0-alpha.1](#v1140-alpha1)
  - [Downloads for v1.14.0-alpha.1](#downloads-for-v1140-alpha1)
  - [Changelog since v1.13.0](#changelog-since-v1130)
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

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.14.0-beta.0
## Downloads for v1.14.0-beta.0

Download v1.14.0-beta.0 in the [v1.14.0-beta.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.14.0-beta.0).

## Changelog since v1.14.0-alpha.2

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
- `karmada-search`: Fixed the proxy error with pb content type by explicitly suppressing PB serialization from karmada-search serverside. ([#6326](https://github.com/karmada-io/karmada/pull/6326), @ikaven1024)

### Security
- Change the listening address of karmada components to the POD IP address to avoid all-zero listening. ([#6266](https://github.com/karmada-io/karmada/issues/6266), @seanlaii, @XiShanYongYe-Chang)

## Other
### Dependencies
- Kubernetes dependencies have been updated to v1.32.3. ([#6311](https://github.com/karmada-io/karmada/pull/6311), @RainbowMango)

### Helm Charts
None.

### Instrumentation
- `karmada-controller-manager`: Fixed the issue that the result label of `federatedhpa_pull_metrics_duration_seconds` is always `success`. ([#6303](https://github.com/karmada-io/karmada/pull/6303), @tangzhongren)

### Performance
- `karmada-controller-manager`: Significant performance improvements have been achieved by reducing deepcopy operations during list processes: ([#5813](https://github.com/karmada-io/karmada/pull/5813), @@CharlesQQ)
  - Binding Controller: Response time reduced by 50%
  - Dependencies Controller: Execution time improved 5× faster
  - Detector Controller: Processing time cut by 50%
- `karmada-controller-manager`: Now when karmada-controller-manager tries to update a resource to a member cluster, it will attempt to compare the contents to skip redundant update operations. The optimization significantly reduces the execution-controller by 80% during the controller start-up. ([#6150](https://github.com/karmada-io/karmada/pull/6150), @zach593)

# v1.14.0-alpha.2
## Downloads for v1.14.0-alpha.2

Download v1.14.0-alpha.2 in the [v1.14.0-alpha.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.14.0-alpha.2).

## Changelog since v1.14.0-alpha.1

## Urgent Update Notes
None.

## Changes by Kind

### API Changes

### Features & Enhancements
- `karmada-controller-manager`: The default resource interpreter will deduplicate and sort `status.Loadbalancer.Ingress` field for Service and Ingress resource when aggregating status. ([#6252](https://github.com/karmada-io/karmada/pull/6252), @zach593)
- Updated Flink interpreter to check error when job state is not published. ([#6287](https://github.com/karmada-io/karmada/pull/6287), @liwang0513)

### Deprecation
None.

### Bug Fixes
- `karmada-controller-manager`: The default resource interpreter will no longer populate the `.status.LoadBalancer.Ingress[].Hostname` field with the member cluster name for Service and Ingress resources. ([#6249](https://github.com/karmada-io/karmada/pull/6249), @zach593)
- `karmada-controller-manager`: when cluster is not-ready doesn't clean MultiClusterService and EndpointSlice work. ([#6258](https://github.com/karmada-io/karmada/pull/6258), @XiShanYongYe-Chang)
- `karmada-agent`: Fixed the issue where a new pull-mode cluster may overwrite the existing member clusters. ([#6253](https://github.com/karmada-io/karmada/pull/6253), @zhzhuang-zju)
- `karmadactl`: Fixed the issue where option `discovery-timeout` fails to work properly. ([#6270](https://github.com/karmada-io/karmada/pull/6270), @zhzhuang-zju)

### Security
None.

## Other
### Dependencies
- Bump go version to 1.23.8. ([#6272](https://github.com/karmada-io/karmada/pull/6272), @seanlaii)

### Helm Charts
- Added `scheduler.enableSchedulerEstimator` to helm values for karmada chart to allow the scheduler to connect with the scheduler estimator. ([#6286](https://github.com/karmada-io/karmada/pull/6286), @mojojoji)

### Instrumentation
- Introduced `karmada_build_info` metric, which exposes build metadata, to components `karmada-agent`, `karmada-controller-manager`, `karmada-descheduler`, `karmada-metrics-adapter`, `karmada-scheduler-estimator`, `karmada-scheduler`, and `karmada-webhook`. ([#6215](https://github.com/karmada-io/karmada/pull/6215), @dongjiang1989)

### Performance
- After replacing dynamic informers in the `detector` controller with typed informers, which reduces CPU usage by 25% during restarts and reduces memory consumption by 10%. ([#5802](https://github.com/karmada-io/karmada/pull/5802), @CharlesQQ)

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
