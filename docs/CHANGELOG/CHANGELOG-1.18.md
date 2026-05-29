<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.18.0](#v1180)
  - [Downloads for v1.18.0](#downloads-for-v1180)
  - [Urgent Update Notes](#urgent-update-notes)
  - [What's New](#whats-new)
    - [Overflow Cluster Affinities for Hybrid Cloud Scheduling](#overflow-cluster-affinities-for-hybrid-cloud-scheduling)
    - [Scheduling Overcommit Protection](#scheduling-overcommit-protection)
  - [Other Notable Changes](#other-notable-changes)
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
  - [Contributors](#contributors)
- [v1.18.0-rc.0](#v1180-rc0)
  - [Downloads for v1.18.0-rc.0](#downloads-for-v1180-rc0)
  - [Changelog since v1.18.0-beta.0](#changelog-since-v1180-beta0)
  - [Urgent Update Notes](#urgent-update-notes-1)
  - [Changes by Kind](#changes-by-kind)
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
- [v1.18.0-beta.0](#v1180-beta0)
  - [Downloads for v1.18.0-beta.0](#downloads-for-v1180-beta0)
  - [Changelog since v1.18.0-alpha.2](#changelog-since-v1180-alpha2)
  - [Urgent Update Notes](#urgent-update-notes-2)
  - [Changes by Kind](#changes-by-kind-1)
    - [API Changes](#api-changes-2)
    - [Features & Enhancements](#features--enhancements-2)
    - [Deprecation](#deprecation-2)
    - [Bug Fixes](#bug-fixes-2)
    - [Security](#security-2)
  - [Other](#other-2)
    - [Dependencies](#dependencies-2)
    - [Helm Charts](#helm-charts-2)
    - [Instrumentation](#instrumentation-2)
    - [Performance](#performance-2)
- [v1.18.0-alpha.2](#v1180-alpha2)
  - [Downloads for v1.18.0-alpha.2](#downloads-for-v1180-alpha2)
  - [Changelog since v1.18.0-alpha.1](#changelog-since-v1180-alpha1)
  - [Urgent Update Notes](#urgent-update-notes-3)
  - [Changes by Kind](#changes-by-kind-2)
    - [API Changes](#api-changes-3)
    - [Features & Enhancements](#features--enhancements-3)
    - [Deprecation](#deprecation-3)
    - [Bug Fixes](#bug-fixes-3)
    - [Security](#security-3)
  - [Other](#other-3)
    - [Dependencies](#dependencies-3)
    - [Helm Charts](#helm-charts-3)
    - [Instrumentation](#instrumentation-3)
    - [Performance](#performance-3)
- [v1.18.0-alpha.1](#v1180-alpha1)
  - [Downloads for v1.18.0-alpha.1](#downloads-for-v1180-alpha1)
  - [Changelog since v1.18.0-alpha.0](#changelog-since-v1180-alpha0)
  - [Urgent Update Notes](#urgent-update-notes-4)
  - [Changes by Kind](#changes-by-kind-3)
    - [API Changes](#api-changes-4)
    - [Features & Enhancements](#features--enhancements-4)
    - [Deprecation](#deprecation-4)
    - [Bug Fixes](#bug-fixes-4)
    - [Security](#security-4)
  - [Other](#other-4)
    - [Dependencies](#dependencies-4)
    - [Helm Charts](#helm-charts-4)
    - [Instrumentation](#instrumentation-4)
    - [Performance](#performance-4)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.18.0
## Downloads for v1.18.0

Download v1.18.0 in the [v1.18.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.18.0).

## Urgent Update Notes
- `karmada-operator-chart`: To ensure a smooth upgrade, users must first upgrade to release `v1.17.3+` before upgrading to a `v1.18.x` release. ([#7457](https://github.com/karmada-io/karmada/pull/7457), @jabellard)

## What's New

### Overflow Cluster Affinities for Hybrid Cloud Scheduling

In hybrid cloud environments, organizations typically rely on local data centers as the primary resource pool and use public clouds as elastic extensions for peak traffic. Previously, Karmada's `ClusterAffinities` supported multiple candidate cluster groups, but each group was mutually exclusive—the scheduler would select only one group per scheduling cycle and could not spread replicas across groups when capacity was insufficient.

This release extends `ClusterAffinities` with the **Overflow Cluster Affinities** capability, enabling replicas to automatically overflow from preferred cluster groups to supplementary ones when capacity is exhausted—**use IDC first, overflow to cloud when capacity is insufficient**:
- **Progressive overflow**: Users declare supplementary cluster groups via the `overflowAffinities` field within a `ClusterAffinityTerm`. The scheduler fills the primary group first, and when its resources are exhausted, progressively expands scheduling to the supplementary groups in declared order.
- **Reverse contraction**: During scale-down, replicas are reclaimed from supplementary groups first, ensuring cost-efficient resource usage as traffic decreases.
- **Cost optimization**: Keeps baseline workloads on cost-effective local clusters while allowing burst traffic to expand to public cloud clusters only when necessary.

This feature is particularly useful when:
- **Elastic GPU scheduling**: Running GPU inference workloads across IDC and cloud clusters, using cloud GPUs only for overflow peak traffic.
- **Cost control**: Ensuring baseline workloads stay on cheaper on-premises infrastructure while leveraging public cloud elasticity for demand spikes.
- **Capacity planning**: Defining clear priority relationships for cluster resource consumption without complex manual intervention.

For a detailed description of this feature, see the [official documentation](https://karmada.io/docs/next/userguide/scheduling/propagation-policy#clusteraffinities-multiple-cluster-groups) and the original [proposal](https://github.com/karmada-io/karmada/blob/master/docs/proposals/scheduling/multi-scheduling-group/overflow-affinities/README.md).

(Feature contributors: @zhzhuang-zju, @RainbowMango, @vie-serendipity)

### Scheduling Overcommit Protection

The Karmada scheduler processes scheduling tasks sequentially, but it relies on the `karmada-scheduler-estimator` to determine available cluster capacity. Because the estimator computes capacity based on Pods already bound to nodes, there is an inherent timing gap: after the scheduler assigns workloads to a cluster, it takes time for those workloads to be distributed, created, and bound to nodes. If new scheduling requests arrive during this window, the estimator operates on a stale snapshot and may over-commit the same cluster's resources across multiple consecutive scheduling decisions.

This release introduces **Scheduling Overcommit Protection** to close this gap with an "assume and deduct" mechanism between the scheduler and the estimator:
- **Assumed workloads tracking**: Once the scheduler makes a placement decision and patches the ResourceBinding, it caches the decision as "assumed" resource consumption in memory.
- **Deduction at estimation time**: When querying the estimator for available replicas, the scheduler passes these assumed workloads along with the gRPC request. The estimator deducts the assumed consumption from its local resource view before computing available capacity.
- **Automatic release**: Assumptions are proactively released when the workload reaches a healthy state in the target cluster, with a TTL-based garbage collection safety net to handle downstream failures.

This eliminates resource over-commitment in high-throughput scheduling scenarios, ensuring workloads are accurately distributed to clusters with genuine available capacity rather than silently Pending due to resource exhaustion.

For a detailed description of this feature, see the original [proposal](https://github.com/karmada-io/karmada/blob/master/docs/proposals/scheduling/estimator-reservation/README.md).

(Feature contributors: @XiShanYongYe-Chang, @RainbowMango, @mszacillo)

## Other Notable Changes

### API Changes
- Introduced `OverflowClusterAffinities` to `PropagationPolicy`/`ClusterPropagationPolicy` API to support overflow cluster resource pools for hybrid cloud scheduling. ([#7386](https://github.com/karmada-io/karmada/pull/7386), @zhzhuang-zju)
- `karmada-webhook`: Now rejects `PropagationPolicy` and `ClusterPropagationPolicy` resources that use the `Lt` or `Gt` operator in `spec.placement.clusterTolerations`. ([#7272](https://github.com/karmada-io/karmada/pull/7272), @XiShanYongYe-Chang)
- `karmada-scheduler-estimator`: Introduced `AssumedWorkloads` field into the gRPC message `MaxAvailableComponentSetsRequest`. ([#7491](https://github.com/karmada-io/karmada/pull/7491), @XiShanYongYe-Chang)
- `karmada-scheduler-estimator`: Introduced `AssumedWorkloads` field into the gRPC message `MaxAvailableReplicasRequest`. ([#7534](https://github.com/karmada-io/karmada/pull/7534), @XiShanYongYe-Chang)
- `karmada-scheduler-estimator`: Migrated to standard `protoc-gen-go` for gRPC API generation to support Kubernetes 1.35+. Introduced peer `bytes` fields for K8s types to ensure compatibility. ([#7298](https://github.com/karmada-io/karmada/pull/7298), @zhzhuang-zju)
  - `ReplicaRequirements.resourceRequest` has been deprecated in favor of `resourceRequestBytes`, which stores proto-serialized `resource.Quantity` objects.
  - `ComponentReplicaRequirements.resourceRequest` has been deprecated in favor of `resourceRequestBytes`, which stores proto-serialized `resource.Quantity` objects.
  - `NodeClaim.nodeAffinity` has been deprecated in favor of `nodeAffinityBytes`, which stores a proto-serialized `corev1.NodeSelector` object.
  - `NodeClaim.tolerations` has been deprecated in favor of `tolerationsBytes`, which stores proto-serialized `corev1.Toleration` objects.

### Features & Enhancements
- `karmada-operator`: Aligned with [recommendations from SIG instrumentation](https://github.com/kubernetes/community/blob/main/contributors/devel/sig-instrumentation/logging.md#how-to-log), when provisioning a Karmada instance, the operator now sets the default verbosity level for the following components to 2: `karmada-webhook`, `kube-controller-manager`, `karmada-controller-manager`, `karmada-scheduler`, `karmada-apiserver`, and `karmada-descheduler`. ([#7352](https://github.com/karmada-io/karmada/pull/7352), @jabellard)
- `karmada-scheduler`: Extended cluster affinities to support overflow cluster resource pools for hybrid cloud scheduling. ([#7423](https://github.com/karmada-io/karmada/pull/7423), @zhzhuang-zju)
- `karmada-scheduler`: Extended the MultiplePodTemplatesScheduling feature to support workloads with only one component. ([#7287](https://github.com/karmada-io/karmada/pull/7287), @seanlaii)
- `karmada-scheduler/karmada-scheduler-estimator`: Introduced `SchedulingOvercommitProtection` feature gate, which defaults to false. ([#7574](https://github.com/karmada-io/karmada/pull/7574), @XiShanYongYe-Chang)
- `karmada-scheduler-estimator`: Replaced input parameters in `RunEstimateComponentsPlugins` with a single structure `ComponentEstimationContext`. ([#7503](https://github.com/karmada-io/karmada/pull/7503), @XiShanYongYe-Chang)
- `karmada-scheduler-estimator`: Replaced input parameters in `EstimateReplicasPlugin` with a single structure `ReplicaEstimationContext`. ([#7560](https://github.com/karmada-io/karmada/pull/7560), @XiShanYongYe-Chang)
- `karmada-webhook`: Added `overflowAffinities` validation for `spec.placement.clusterAffinities` in `PropagationPolicy`/`ClusterPropagationPolicy`. ([#7430](https://github.com/karmada-io/karmada/pull/7430), @zhzhuang-zju)
- `karmada-webhook`: Added validation for `spec.overriders.fieldOverrider` in `OverridePolicy` and `ClusterOverridePolicy` to ensure that a `FieldOverrider` must have either JSON or YAML set. ([#7365](https://github.com/karmada-io/karmada/pull/7365), @Denyme24)
- `karmadactl`: Aligned with [recommendations from SIG instrumentation](https://github.com/kubernetes/community/blob/main/contributors/devel/sig-instrumentation/logging.md#how-to-log), when provisioning a Karmada instance, `karmadactl` now sets the default verbosity level to 2 for the following components: `karmada-apiserver`, `karmada-aggregated-apiserver`, `karmada-controller-manager`, `karmada-scheduler`, `karmada-webhook`, `karmada-agent`, `karmada-descheduler`, and `kube-controller-manager`. Users can override the default via `--<component>-extra-args="--v=<level>"`. ([#7356](https://github.com/karmada-io/karmada/pull/7356), @seanlaii)

### Deprecation
- `karmada-controller-manager`: The flags `--cluster-lease-duration` and `--cluster-lease-renew-interval-fraction` have been removed. ([#7271](https://github.com/karmada-io/karmada/pull/7271), @dahuo98)
- `karmada-scheduler-estimator`: The deprecated `Estimator` label value for `estimating_plugin_extension_point` in the `estimating_plugin_execution_duration_seconds` and `estimating_plugin_extension_point_duration_seconds` metrics has been removed. ([#7571](https://github.com/karmada-io/karmada/pull/7571), @RainbowMango)
- `karmadactl`: The deprecated `Etcd.Local.InitImage` in `Karmada Init Configuration` has been removed. ([#7259](https://github.com/karmada-io/karmada/pull/7259), @dahuo98)

### Bug Fixes
- `etcd`: Changed health probes to use the `/livez` and `/readyz` endpoints, since the previous shell-based probes no longer worked in the 3.6.6-0 etcd container image. ([#7381](https://github.com/karmada-io/karmada/pull/7381), @vgt-rangehrn)
- `karmada-agent`: Fixed the issue that certificate rotation CSRs were never auto-approved due to a SignerName mismatch between `cert_rotation_controller` and `agent_csr_approving`. ([#7275](https://github.com/karmada-io/karmada/pull/7275), @hl8086)
- `karmada-controller-manager`: Avoided blocking dependency propagation on informer cache synchronization for newly watched dependent resources. ([#7276](https://github.com/karmada-io/karmada/pull/7276), @whitewindmills)
- `karmada-controller-manager`: Fixed `ClusterTaintPolicyController` silently dropping concurrent cluster health taints (`not-ready:NoSchedule`, `unreachable:NoSchedule`) during taint patches, which could cause workloads to be scheduled onto degraded clusters. ([#7348](https://github.com/karmada-io/karmada/pull/7348), @Ady0333)
- `karmada-controller-manager`: Fixed a race condition where graceful eviction tasks could be silently dropped when multiple controllers concurrently modify the same ResourceBinding or ClusterResourceBinding, preventing workloads from being evacuated from tainted or failing clusters. ([#7302](https://github.com/karmada-io/karmada/pull/7302), @Ady0333)
- `karmada-controller-manager`: Fixed an issue where the FullyApplied condition of ResourceBinding could be incorrectly reported during RetryOnConflict retries when the cluster set changed. ([#7226](https://github.com/karmada-io/karmada/pull/7226), @Ady0333)
- `karmada-controller-manager`: Fixed the flink-deployment health interpreter to only treat `ErrImagePull` and `ImagePullBackOff` as healthy during ephemeral state. ([#7476](https://github.com/karmada-io/karmada/pull/7476), @dahuo98)
- `karmada-controller-manager`: Fixed the issue that `Job` completions were assigned to the wrong replicas for each cluster. ([#7387](https://github.com/karmada-io/karmada/pull/7387), @Ady0333)
- `karmada-controller-manager`: Fixed the issue that a transient `ClusterClientSetFunc` failure (e.g., missing `SecretRef` during credential rotation) would immediately set the cluster `Ready=False` without respecting `ClusterFailureThreshold`, potentially triggering unnecessary workload failover. ([#7426](https://github.com/karmada-io/karmada/pull/7426), @Tej-Katika)
- `karmada-operator`: Fixed init reconciliation failure by replacing non-idempotent secret creation with an idempotent approach. ([#7358](https://github.com/karmada-io/karmada/pull/7358), @qiuming520)
- `karmada-operator`: Fixed the issue that the operator did not apply `tolerations` and `affinity` settings to `karmada-aggregated-apiserver` and `karmada-search` deployments when configured via the Karmada CR. ([#7449](https://github.com/karmada-io/karmada/pull/7449), @jabellard)
- `karmada-scheduler`: Fixed an issue where the schedule success event was missing cluster information when scheduling with `ClusterAffinities`. ([#7394](https://github.com/karmada-io/karmada/pull/7394), @vie-serendipity)
- `karmada-scheduler`: Fixed incorrect error type propagation that caused bindings with insufficient cluster replicas to be misrouted to `backoffQ` instead of `unschedulableBindings`, changing retry behavior from exponential backoff (1–10s) to timer-based retry (5 minutes). ([#7340](https://github.com/karmada-io/karmada/pull/7340), @seanlaii)
- `karmada-scheduler`: Fixed the issue when cluster resources are insufficient, multiple template resources can still be scheduled. ([#7554](https://github.com/karmada-io/karmada/pull/7554), @XiShanYongYe-Chang)
- `karmada-scheduler`: Relaxed zone affinity matching for multi-zone clusters, so a cluster is eligible when any configured zone overlaps the selector. ([#6431](https://github.com/karmada-io/karmada/pull/6431), @whitewindmills)
- `karmada-search`: Fixed the issue that watch connect cannot reflect resources from recovered clusters immediately. ([#7074](https://github.com/karmada-io/karmada/pull/7074), @XiShanYongYe-Chang)
- `openapi schema`: Fixed the unknown model error by using fully qualified model names as OpenAPI model names instead of Go type names. ([#7301](https://github.com/karmada-io/karmada/pull/7301), @zhzhuang-zju)

### Security
- The base image `alpine` has been promoted from `alpine:3.23.3` to `alpine:3.23.4` to address security concerns. ([#7412](https://github.com/karmada-io/karmada/pull/7412), @dependabot)

## Other

### Dependencies
- `karmada-controller-manager`/`karmada-agent`: Upgraded `gopher-lua` dependency to v1.1.1. ([#7270](https://github.com/karmada-io/karmada/pull/7270), @XiShanYongYe-Chang)
- Kubernetes dependencies have been updated to v1.35.3. ([#7326](https://github.com/karmada-io/karmada/pull/7326), @dahuo98)

### Helm Charts
- `Helm chart`: Added helm index for `v1.17.0`. ([#7282](https://github.com/karmada-io/karmada/pull/7282), @github-actions)
- `Helm chart`: Added helm index for `v1.17.1`. ([#7351](https://github.com/karmada-io/karmada/pull/7351), @github-actions)
- `Helm chart`: Added helm index for `v1.17.2`. ([#7456](https://github.com/karmada-io/karmada/pull/7456), @github-actions)
- `Helm chart`: Aligned with recommendations from SIG instrumentation, the Helm chart now sets the default verbosity level to 2 for the following components: `karmada-apiserver`, `karmada-aggregated-apiserver`, `karmada-controller-manager`, `karmada-scheduler`, `karmada-webhook`, `karmada-agent`, `karmada-descheduler`, `kube-controller-manager`, `karmada-search`, `karmada-metrics-adapter`, and `karmada-scheduler-estimator`. ([#7372](https://github.com/karmada-io/karmada/pull/7372), @SujoyDutta)
- `karmada-chart`: Fixed unrendered `{{ ca_crt }}` during upgrades. ([#7185](https://github.com/karmada-io/karmada/pull/7185), @faucct)
- `karmada-operator-chart`: Fixed the issue where embedding the `Karmada` CRD exceeded the ConfigMap size limit in the chart's installation/upgrade job, which prevented users from upgrading the chart. ([#7457](https://github.com/karmada-io/karmada/pull/7457), @jabellard)

### Instrumentation
- `Instrumentation`: The deprecated `cluster` and `cluster_name` Prometheus metric labels have been removed. The newly introduced `member_cluster` metric label name will now be used for that purpose moving forward. ([#7258](https://github.com/karmada-io/karmada/pull/7258), @dahuo98)

### Performance
- `karmada-controller-manager`: Avoided unnecessary reconciles by skipping no-op updates in ResourceDetector policy handlers. ([#7417](https://github.com/karmada-io/karmada/pull/7417), @LivingCcj)

## Contributors

Thank you to everyone who contributed to this release!

Users whose commits are in this release (alphabetically by username)

- @abhicodes11
- @Ady0333
- @dahuo98
- @Denyme24
- @faucct
- @FAUST-BENCHOU
- @gjbravi
- @hl8086
- @jabellard
- @Krishiv-Mahajan
- @LivingCcj
- @manmathbh
- @mszacillo
- @nXtCyberNet
- @Park-Jiyeonn
- @qiuming520
- @RainbowMango
- @seanlaii
- @SujoyDutta
- @SunsetB612
- @Tej-Katika
- @tessapham
- @vanshiz
- @vgt-rangehrn
- @vie-serendipity
- @Vinayak9769
- @Viscous106
- @warjiang
- @whitewindmills
- @XiShanYongYe-Chang
- @zhzhuang-zju

# v1.18.0-rc.0
## Downloads for v1.18.0-rc.0

Download v1.18.0-rc.0 in the [v1.18.0-rc.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.18.0-rc.0).

## Changelog since v1.18.0-beta.0

## Urgent Update Notes
- `karmada-operator-chart`: To ensure a smooth upgrade, users must first upgrade to release `v1.17.3+` before upgrading to a `v1.18.x` release. ([#7457](https://github.com/karmada-io/karmada/pull/7457), @jabellard)

## Changes by Kind

### API Changes
- `karmada-scheduler-estimator`: Introduced `AssumedWorkloads` field into the gRPC message `MaxAvailableComponentSetsRequest`. ([#7491](https://github.com/karmada-io/karmada/pull/7491), @XiShanYongYe-Chang)

### Features & Enhancements
- `karmada-controller-manager`: Avoided unnecessary reconciles by skipping no-op updates in ResourceDetector policy handlers. ([#7417](https://github.com/karmada-io/karmada/pull/7417), @LivingCcj)

### Deprecation
None.

### Bug Fixes
- `etcd`: Changed health probes to use the `/livez` and `/readyz` endpoints, since the previous shell-based probes no longer worked in the 3.6.6-0 etcd container image. ([#7381](https://github.com/karmada-io/karmada/pull/7381), @vgt-rangehrn)
- `karmada-controller-manager`: Fixed the flink-deployment health interpreter to only treat `ErrImagePull` and `ImagePullBackOff` as healthy during ephemeral state. ([#7476](https://github.com/karmada-io/karmada/pull/7476), @dahuo98)
- `karmada-operator-chart`: Fixed the issue where embedding the `Karmada` CRD exceeded the ConfigMap size limit in the chart's installation/upgrade job, which prevented users from upgrading the chart. ([#7457](https://github.com/karmada-io/karmada/pull/7457), @jabellard)

### Security
None.

## Other

### Dependencies
None.

### Helm Charts
- `Helm chart`: Added helm index for `v1.17.2`. ([#7456](https://github.com/karmada-io/karmada/pull/7456), @github-actions)

### Instrumentation
None.

### Performance
None.

# v1.18.0-beta.0
## Downloads for v1.18.0-beta.0

Download v1.18.0-beta.0 in the [v1.18.0-beta.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.18.0-beta.0).

## Changelog since v1.18.0-alpha.2

## Urgent Update Notes

## Changes by Kind

### API Changes
- `scheduler-estimator`: Migrated to standard `protoc-gen-go` for gRPC API generation to support Kubernetes 1.35+. Introduced peer `bytes` fields for K8s types to ensure compatibility. ([#7298](https://github.com/karmada-io/karmada/pull/7298), @zhzhuang-zju)
  - `ReplicaRequirements.resourceRequest` has been deprecated in favor of `resourceRequestBytes`, which stores proto-serialized `resource.Quantity` objects.
  - `ComponentReplicaRequirements.resourceRequest` has been deprecated in favor of `resourceRequestBytes`, which stores proto-serialized `resource.Quantity` objects.
  - `NodeClaim.nodeAffinity` has been deprecated in favor of `nodeAffinityBytes`, which stores a proto-serialized `corev1.NodeSelector` object.
  - `NodeClaim.tolerations` has been deprecated in favor of `tolerationsBytes`, which stores proto-serialized `corev1.Toleration` objects.
- Introduced `OverflowClusterAffinities` to `PropagationPolicy`/`ClusterPropagationPolicy` API to support overflow cluster resource pools for hybrid cloud scheduling. ([#7386](https://github.com/karmada-io/karmada/pull/7386), @zhzhuang-zju)

### Features & Enhancements
- `karmada-scheduler`: Extended cluster affinities to support overflow cluster resource pools for hybrid cloud scheduling. ([#7423](https://github.com/karmada-io/karmada/pull/7423), @zhzhuang-zju)
- `karmada-webhook`: Added `overflowAffinities` validation for `spec.placement.clusterAffinities` in `PropagationPolicy`/`ClusterPropagationPolicy`. ([#7430](https://github.com/karmada-io/karmada/pull/7430), @zhzhuang-zju)

### Deprecation
None.

### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that a transient `ClusterClientSetFunc` failure (e.g., missing `SecretRef` during credential rotation) would immediately set the cluster `Ready=False` without respecting `ClusterFailureThreshold`, potentially triggering unnecessary workload failover. ([#7426](https://github.com/karmada-io/karmada/pull/7426), @Tej-Katika)
- `karmada-operator`: Fixed the issue that the operator did not apply `tolerations` and `affinity` settings to `karmada-aggregated-apiserver` and `karmada-search` deployments when configured via the Karmada CR. ([#7449](https://github.com/karmada-io/karmada/pull/7449), @jabellard)
- `karmada-scheduler`: Fixed an issue where the schedule success event was missing cluster information when scheduling with `ClusterAffinities`. ([#7394](https://github.com/karmada-io/karmada/pull/7394), @vie-serendipity)

### Security
- The base image `alpine` has been promoted from `alpine:3.23.3` to `alpine:3.23.4` to address security concerns. ([#7412](https://github.com/karmada-io/karmada/pull/7412), @dependabot)

## Other

### Dependencies
None.

### Helm Charts
None.

### Instrumentation
None.

### Performance
None.

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
