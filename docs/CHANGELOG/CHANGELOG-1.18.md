<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.18.0-alpha.2](#v1180-alpha2)
  - [Downloads for v1.18.0-alpha.2](#downloads-for-v1180-alpha2)
  - [Changelog since v1.18.0-alpha.1](#changelog-since-v1180-alpha1)
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
- [v1.18.0-alpha.1](#v1180-alpha1)
  - [Downloads for v1.18.0-alpha.1](#downloads-for-v1180-alpha1)
  - [Changelog since v1.18.0-alpha.0](#changelog-since-v1180-alpha0)
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

# v1.18.0-alpha.2
## Downloads for v1.18.0-alpha.2

Download v1.18.0-alpha.2 in the [v1.18.0-alpha.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.18.0-alpha.2).

## Changelog since v1.18.0-alpha.1

## Urgent Update Notes

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-operator`: Aligned with [recommendations from SIG instrumentation](https://github.com/kubernetes/community/blob/main/contributors/devel/sig-instrumentation/logging.md#how-to-log), when provisioning a Karmada instance, the operator now sets the default verbosity level for the following components to 2: `karmada-webhook`, `kube-controller-manager`, `karmada-controller-manager`, `karmada-scheduler`, `karmada-apiserver`, and `karmada-descheduler`. ([#7352](https://github.com/karmada-io/karmada/pull/7352), @jabellard)
- `karmada-webhook`: Added validation for `spec.overriders.fieldOverrider` in `OverridePolicy` and `ClusterOverridePolicy` to ensure that a `FieldOverrider` must have either JSON or YAML set. ([#7365](https://github.com/karmada-io/karmada/pull/7365), @Denyme24)
- `karmadactl`: Aligned with [recommendations from SIG instrumentation](https://github.com/kubernetes/community/blob/main/contributors/devel/sig-instrumentation/logging.md#how-to-log), when provisioning a Karmada instance, `karmadactl` now sets the default verbosity level to 2 for the following components: `karmada-apiserver`, `karmada-aggregated-apiserver`, `karmada-controller-manager`, `karmada-scheduler`, `karmada-webhook`, `karmada-agent`, `karmada-descheduler`, and `kube-controller-manager`. Users can override the default via `--<component>-extra-args="--v=<level>"`. ([#7356](https://github.com/karmada-io/karmada/pull/7356), @seanlaii)

### Deprecation
None.

### Bug Fixes
- `karmada-controller-manager`: Fixed `ClusterTaintPolicyController` silently dropping concurrent cluster health taints (`not-ready:NoSchedule`, `unreachable:NoSchedule`) during taint patches, which could cause workloads to be scheduled onto degraded clusters. ([#7348](https://github.com/karmada-io/karmada/pull/7348), @Ady0333)
- `karmada-controller-manager`: Fixed the issue that `Job` completions were assigned to the wrong replicas for each cluster. ([#7387](https://github.com/karmada-io/karmada/pull/7387), @Ady0333)
- `karmada-operator`: Fixed init reconciliation failure by replacing non-idempotent secret creation with an idempotent approach. ([#7358](https://github.com/karmada-io/karmada/pull/7358), @qiuming520)
- `karmada-scheduler`: Fixed incorrect error type propagation that caused bindings with insufficient cluster replicas to be misrouted to `backoffQ` instead of `unschedulableBindings`, changing retry behavior from exponential backoff (1–10s) to timer-based retry (5 minutes). ([#7340](https://github.com/karmada-io/karmada/pull/7340), @seanlaii)

### Security
None.

## Other

### Dependencies
None.

### Helm Charts
- `Helm chart`: Added helm index for `v1.17.1`. ([#7351](https://github.com/karmada-io/karmada/pull/7351), @github-actions)
- `Helm chart`: Aligned with recommendations from SIG instrumentation, the Helm chart now sets the default verbosity level to 2 for the following components: `karmada-apiserver`, `karmada-aggregated-apiserver`, `karmada-controller-manager`, `karmada-scheduler`, `karmada-webhook`, `karmada-agent`, `karmada-descheduler`, `kube-controller-manager`, `karmada-search`, `karmada-metrics-adapter`, and `karmada-scheduler-estimator`. ([#7372](https://github.com/karmada-io/karmada/pull/7372), @SujoyDutta)

### Instrumentation
None.

### Performance
None.

# v1.18.0-alpha.1
## Downloads for v1.18.0-alpha.1

Download v1.18.0-alpha.1 in the [v1.18.0-alpha.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.18.0-alpha.1).

## Changelog since v1.18.0-alpha.0

## Urgent Update Notes

## Changes by Kind

### API Changes
None.

### Features & Enhancements
- `karmada-scheduler`: Extended the MultiplePodTemplatesScheduling feature to support workloads with only one component. ([#7287](https://github.com/karmada-io/karmada/pull/7287), @seanlaii)
- `karmada-search`: Added `--cluster-api-qps` and `--cluster-api-burst` flags, and used them when creating member-cluster dynamic clients in search control. ([#7325](https://github.com/karmada-io/karmada/pull/7325), @manmathbh)
- `karmada-webhook`: Now rejects `PropagationPolicy` and `ClusterPropagationPolicy` resources that use the `Lt` or `Gt` operator in `spec.placement.clusterTolerations`. ([#7272](https://github.com/karmada-io/karmada/pull/7272), @XiShanYongYe-Chang)

### Deprecation
- `Instrumentation`: The deprecated `cluster` and `cluster_name` Prometheus metric labels have been removed. The newly introduced `member_cluster` metric label name will now be used for that purpose moving forward. ([#7258](https://github.com/karmada-io/karmada/pull/7258), @dahuo98)
- `karmada-controller-manager`: The flags `--cluster-lease-duration` and `--cluster-lease-renew-interval-fraction` have been removed. ([#7271](https://github.com/karmada-io/karmada/pull/7271), @dahuo98)
- `karmadactl`: The deprecated `Etcd.Local.InitImage` in `Karmada Init Configuration` has been removed. ([#7259](https://github.com/karmada-io/karmada/pull/7259), @dahuo98)

### Bug Fixes
- `karmada-agent`: Fixed the issue that certificate rotation CSRs were never auto-approved due to a SignerName mismatch between `cert_rotation_controller` and `agent_csr_approving`. ([#7275](https://github.com/karmada-io/karmada/pull/7275), @hl8086)
- `karmada-controller-manager`: Avoided blocking dependency propagation on informer cache synchronization for newly watched dependent resources. ([#7276](https://github.com/karmada-io/karmada/pull/7276), @whitewindmills)
- `karmada-controller-manager`: Fixed a race condition where graceful eviction tasks could be silently dropped when multiple controllers concurrently modify the same ResourceBinding or ClusterResourceBinding, preventing workloads from being evacuated from tainted or failing clusters. ([#7302](https://github.com/karmada-io/karmada/pull/7302), @Ady0333)
- `karmada-controller-manager`: Fixed an issue where the FullyApplied condition of ResourceBinding could be incorrectly reported during RetryOnConflict retries when the cluster set changed. ([#7226](https://github.com/karmada-io/karmada/pull/7226), @Ady0333)
- `karmada-scheduler`: Relaxed zone affinity matching for multi-zone clusters, so a cluster is eligible when any configured zone overlaps the selector. ([#6431](https://github.com/karmada-io/karmada/pull/6431), @whitewindmills)
- `karmada-search`: Fixed the issue that watch connect cannot reflect resources from recovered clusters immediately. ([#7074](https://github.com/karmada-io/karmada/pull/7074), @XiShanYongYe-Chang)
- `openapi schema`: Fixed the unknown model error by using fully qualified model names as OpenAPI model names instead of Go type names. ([#7301](https://github.com/karmada-io/karmada/pull/7301), @zhzhuang-zju)

### Security
None.

## Other

### Dependencies
- `karmada-controller-manager/karmada-agent`: Upgraded `gopher-lua` dependency to v1.1.1. ([#7270](https://github.com/karmada-io/karmada/pull/7270), @XiShanYongYe-Chang)
- Updated Kubernetes dependencies to v1.35.3. ([#7326](https://github.com/karmada-io/karmada/pull/7326), @dahuo98)

### Helm Charts
- `Helm chart`: Added helm index for `v1.17.0`. ([#7282](https://github.com/karmada-io/karmada/pull/7282), @github-actions)
- `karmada-chart`: Fixed unrendered `{{ ca_crt }}` during upgrades. ([#7185](https://github.com/karmada-io/karmada/pull/7185), @faucct)

### Instrumentation
None.

### Performance
None.
