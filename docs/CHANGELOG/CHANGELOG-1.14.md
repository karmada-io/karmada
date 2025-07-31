<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [v1.14.2](#v1142)
  - [Downloads for v1.14.2](#downloads-for-v1142)
  - [Changelog since v1.14.1](#changelog-since-v1141)
    - [Changes by Kind](#changes-by-kind)
      - [Bug Fixes](#bug-fixes)
      - [Others](#others)
- [v1.14.1](#v1141)
  - [Downloads for v1.14.1](#downloads-for-v1141)
  - [Changelog since v1.14.0](#changelog-since-v1140)
    - [Changes by Kind](#changes-by-kind-1)
      - [Bug Fixes](#bug-fixes-1)
      - [Others](#others-1)
- [v1.14.0](#v1140)
  - [Downloads for v1.14.0](#downloads-for-v1140)
  - [Urgent Update Notes](#urgent-update-notes)
  - [What's New](#whats-new)
    - [Federated ResourceQuota Enforcement (Alpha)](#federated-resourcequota-enforcement-alpha)
    - [Elimate Implicit Cluster Failover, Taint Cluster By Conditions](#elimate-implicit-cluster-failover-taint-cluster-by-conditions)
    - [Karmada Operator Continuous Enhancement](#karmada-operator-continuous-enhancement)
    - [Remarkable Performance Optimization of the Karmada Controllers](#remarkable-performance-optimization-of-the-karmada-controllers)
  - [Other Notable Changes](#other-notable-changes)
    - [API Changes](#api-changes)
    - [Deprecation](#deprecation)
    - [Bug Fixes](#bug-fixes-2)
    - [Security](#security)
    - [Features & Enhancements](#features--enhancements)
  - [Other](#other)
    - [Dependencies](#dependencies)
    - [Helm Charts](#helm-charts)
    - [Instrumentation](#instrumentation)
  - [Contributors](#contributors)
- [v1.14.0-rc.0](#v1140-rc0)
  - [Downloads for v1.14.0-rc.0](#downloads-for-v1140-rc0)
  - [Changelog since v1.14.0-beta.0](#changelog-since-v1140-beta0)
  - [Urgent Update Notes](#urgent-update-notes-1)
  - [Changes by Kind](#changes-by-kind-2)
    - [API Changes](#api-changes-1)
    - [Features & Enhancements](#features--enhancements-1)
    - [Deprecation](#deprecation-1)
    - [Bug Fixes](#bug-fixes-3)
    - [Security](#security-1)
  - [Other](#other-1)
    - [Dependencies](#dependencies-1)
    - [Helm Charts](#helm-charts-1)
    - [Instrumentation](#instrumentation-1)
    - [Performance](#performance)
- [v1.14.0-beta.0](#v1140-beta0)
  - [Downloads for v1.14.0-beta.0](#downloads-for-v1140-beta0)
  - [Changelog since v1.14.0-alpha.2](#changelog-since-v1140-alpha2)
  - [Urgent Update Notes](#urgent-update-notes-2)
  - [Changes by Kind](#changes-by-kind-3)
    - [API Changes](#api-changes-2)
    - [Features & Enhancements](#features--enhancements-2)
    - [Deprecation](#deprecation-2)
    - [Bug Fixes](#bug-fixes-4)
    - [Security](#security-2)
  - [Other](#other-2)
    - [Dependencies](#dependencies-2)
    - [Helm Charts](#helm-charts-2)
    - [Instrumentation](#instrumentation-2)
    - [Performance](#performance-1)
- [v1.14.0-alpha.2](#v1140-alpha2)
  - [Downloads for v1.14.0-alpha.2](#downloads-for-v1140-alpha2)
  - [Changelog since v1.14.0-alpha.1](#changelog-since-v1140-alpha1)
  - [Urgent Update Notes](#urgent-update-notes-3)
  - [Changes by Kind](#changes-by-kind-4)
    - [API Changes](#api-changes-3)
    - [Features & Enhancements](#features--enhancements-3)
    - [Deprecation](#deprecation-3)
    - [Bug Fixes](#bug-fixes-5)
    - [Security](#security-3)
  - [Other](#other-3)
    - [Dependencies](#dependencies-3)
    - [Helm Charts](#helm-charts-3)
    - [Instrumentation](#instrumentation-3)
    - [Performance](#performance-2)
- [v1.14.0-alpha.1](#v1140-alpha1)
  - [Downloads for v1.14.0-alpha.1](#downloads-for-v1140-alpha1)
  - [Changelog since v1.13.0](#changelog-since-v1130)
  - [Urgent Update Notes](#urgent-update-notes-4)
  - [Changes by Kind](#changes-by-kind-5)
    - [API Changes](#api-changes-4)
    - [Features & Enhancements](#features--enhancements-4)
    - [Deprecation](#deprecation-4)
    - [Bug Fixes](#bug-fixes-6)
    - [Security](#security-4)
  - [Other](#other-4)
    - [Dependencies](#dependencies-4)
    - [Helm Charts](#helm-charts-4)
    - [Instrumentation](#instrumentation-4)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# v1.14.2
## Downloads for v1.14.2

Download v1.14.2 in the [v1.14.2 release page](https://github.com/karmada-io/karmada/releases/tag/v1.14.2).

## Changelog since v1.14.1
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the issue that resources will be recreated after being deleted on the cluster when resource is suspended for dispatching. ([#6536](https://github.com/karmada-io/karmada/pull/6536), @luyb177)
- `karmada-controller-manager`: Fixed the issue that EndpointSlice are deleted unexpectedly due to the EndpointSlice informer cache not being synced. ([#6583](https://github.com/karmada-io/karmada/pull/6583), @XiShanYongYe-Chang)

#### Others
- The base image `alpine` now has been promoted from 3.22.0 to 3.22.1. ([#6559](https://github.com/karmada-io/karmada/pull/6559))

# v1.14.1
## Downloads for v1.14.1

Download v1.14.1 in the [v1.14.1 release page](https://github.com/karmada-io/karmada/releases/tag/v1.14.1).

## Changelog since v1.14.0
### Changes by Kind
#### Bug Fixes
- `karmada-controller-manager`: Fixed the bug that cluster can not be unjoined in case of the `--enable-taint-manager=false` or the feature gate `Failover` is disabled. ([#6448](https://github.com/karmada-io/karmada/pull/6448), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that `taint-maanger` didn't honour`--no-execute-taint-eviction-purge-mode` when evicting `ClusterResourceBinding`. ([#6505](https://github.com/karmada-io/karmada/pull/6505), @XiShanYongYe-Chang)
- `karmada-controller-manager`: Fixed the issue that a workload propagated with `duplicated mode` can bypass quota checks during scale up. ([#6502](https://github.com/karmada-io/karmada/pull/6502), @zhzhuang-zju)
- `karmada-controller-manager`: Fixed the issue where the federated-resource-quota-enforcement-controller miscalculates quota usage. ([#6503](https://github.com/karmada-io/karmada/pull/6503), @zhzhuang-zju)

#### Others
- The base image `alpine` now has been promoted from 3.21.3 to 3.22.0. ([#6459](https://github.com/karmada-io/karmada/pull/6459))

# v1.14.0
## Downloads for v1.14.0

Download v1.14.0 in the [v1.14.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.14.0).

## Urgent Update Notes
None.

## What's New

### Federated ResourceQuota Enforcement (Alpha)

Quota management is critical for ensuring fair resource allocation and preventing over-commitment in cloud infrastructures. 
In multi-cloud and multi-cluster environments, fragmented quota systems hinder unified resource visibility and control, 
making federated quota management essential for efficient cross-cluster resource governance.

Karmada previously enabled FederatedResourceQuota to split global quotas to member clusters, which enforce quotas locally. 
This release enhances federated quota management by introducing control-plane overall quota enforcement, allowing global resource quota checks directly at the control plane.

This can be especially useful if:
- You need to track resource consumption and limits from a unified place, without worrying about cluster-level distribution.
- You would like to prevent unnecessary scheduling to clusters by verifying quota resource limits.

Note: FederatedQuotaEnforcement is currently in Alpha and requires enabling the FederatedQuotaEnforcement feature gate.

For a detailed usage of this feature, see the [User Guide: Using FederatedResourceQuota enforcement](https://karmada.io/docs/next/userguide/bestpractices/federated-resource-quota/#using-federatedresourcequota-enforcement).

(Feature contributors: @mszacillo, @RainbowMango, @zhzhuang-zju, @seanlaii, @liwang0513)

### Elimate Implicit Cluster Failover, Taint Cluster By Conditions

In previous versions, when users enabled the Failover feature, the system would automatically add a `NoExecute` taint to
clusters upon detecting abnormal health status, triggering migration of all resources on the target cluster.

In this version, we have conducted a comprehensive review of potential migration triggers in the system. All implicit
cluster failover behaviors have been eliminated, and explicit constraints for cluster failure mechanisms have been
introduced. This enables unified governance of resource migration caused by cluster failures, further enhancing system
stability and predictability.

Cluster failure conditions are determined by evaluating the status conditions of faulty cluster objects to apply taints,
which is a process termed "taint cluster by conditions". This version introduces a new API, `ClusterTaintPolicy`, which
allows users to customize rules for adding specific taints to target clusters when predefined cluster status conditions
are met.

Considering that the NoExecute effect taint evicts all resources on the cluster with a drastic impact and high-risk
nature, we have added the following parameters to the `karmada-webhook` and `karmada-controller-manager` components
respectively:

`--allow-no-execute-taint-policy bool`:
Allows configuring taints with NoExecute effect in ClusterTaintPolicy. Given the impact risk of NoExecute, applying such a
taint to a cluster may trigger the eviction of workloads that do not explicitly tolerate it, potentially causing
unexpected service disruptions.
This parameter is designed to remain disabled by default and requires careful evaluation by administrators before being
enabled.

`--enable-no-execute-taint-eviction bool`:
Enables controller response to NoExecute taints on clusters, which triggers eviction of workloads without
explicit tolerations. Given the high impact of eviction, this parameter is designed to remain disabled by default and
requires careful evaluation by administrators before being enabled.

(Feature contributors: @kevin-wangzefeng, @XiShanYongYe-Chang, @RainbowMango, @mszacillo, @tiansuo114)

### Karmada Operator Continuous Enhancement

This release continues to enhance the Karmada Operator, which is responsible for managing the lifecycle of Karmada components. The following features are added:

- Support configuring Leaf Certificate Validity Period in Karmada Operator. For more information, see the [Proposal: Support to Configure Leaf Certificate Validity Period in Karmada Operator](https://github.com/karmada-io/karmada/blob/master/docs/proposals/karmada-operator/custom-leaf-cert-validity/README.md).
- Support Suspension of Reconciliation for Karmada Control Planes. For more information, see the [Proposal: Support Suspension of Reconciliation for Karmada Control Planes](https://github.com/karmada-io/karmada/blob/master/docs/proposals/karmada-operator/reconciliation-suspension/README.md).
- Support configuring feature gates for karmada-webhook. 
- Support specifying a `loadBalancerClass` for component karmada-apiserver to select a specific load balancer implementation.
- Introduced `karmada_build_info` metrics to emit the build info, as well as a bunch of Go runtime metrics.

These enhancements allow karmada-operator to be more flexible and customizable, enhancing the overall karmada system reliability and stability.

(Feature contributors: @jabellard, @rajsinghtech, @seanlaii, @dongjiang1989)

### Remarkable Performance Optimization of the Karmada Controllers

Since release 1.13, Karmada adopters spontaneously organize themselves to optimize the performance of Karmada. Now a stable and continuous performance optimization team forms, dedicated to enhancing the performance and stability of Karmada. Thank all participants for their efforts.

In this release, Karmada delivers significant performance improvements, especially in karmada-controller-manager. To validate these enhancements, the following test setup was implemented: 

The test setup includes 5,000 Deployments, each paired with a corresponding PropagationPolicy that schedules it to two member clusters. Each Deployment also has a unique ConfigMap as a dependency, which is distributed along with the Deployment to the same clusters.
These resources were created while karmada-controller-manager was offline, meaning Karmada is syncing them for the first time during the test. The test results are as follows:

- Cold start time (clear the workqueue) reduced from ~7 minutes to ~4 minutes, a 45% improvement.
- Resource detector: Max value of average processing time reduced from 391ms to 180ms (54% reduction).
- Dependencies distributor: Max value of average processing time reduced from 378ms to 216ms (43% reduction).
- Execution controller: Max value of average processing time reduced from 505ms to 248ms (50% reduction).

In addition to faster processing speed, resource consumption has also been significantly reduced:

- CPU usage reduced from 4–7.5 cores to 1.8–2.4 cores (40%–65% reduction).
- Memory peak usage reduced from 1.9GB to 1.47GB (22% reduction).

These data prove that the performance of karmada controllers has been greatly enhanced in v1.14. In the future, we will continue to carry out systematic performance optimizations on the controllers and the scheduler.

For the detailed progress and test report, please refer to [Issue: [Performance] Overview of performance improvements for v1.14](https://github.com/karmada-io/karmada/issues/6394).

(Performance contributors: @zach593, @CharlesQQ)

## Other Notable Changes
### API Changes
- Introduced `spec.components.karmadaAPIServer.loadBalancerClass` field in `Karmada` API to specify a `loadBalancerClass` to select a specific load balancer implementation, aligning with Kubernetes Service behavior. ([#6348](https://github.com/karmada-io/karmada/pull/6348), @rajsinghtech)
- Introduced `spec.customCertificate.leafCertValidityDays` field in `Karmada` API to specify a custom validity period in days for control plane leaf certificates (e.g., API server certificate). When not explicitly set, the default validity period of 1 year is used. ([#6193](https://github.com/karmada-io/karmada/pull/6193), @jabellard)
- Introduced `spec.suspend` field in `Karmada` API to temporarily pause/suspend the reconciliation of a `Karmada` object. ([#6359](https://github.com/karmada-io/karmada/pull/6359), @jabellard)
- `API Change`: Introduced a new API named ClusterTaintPolicy to handle cluster taint. ([#6319](https://github.com/karmada-io/karmada/pull/6319) & [#6390](https://github.com/karmada-io/karmada/pull/6390), @XiShanYongYe-Chang)
- `FederatedResourceQuota`: Added two additional printer columns, `OVERALL` and `OVERALL_USED`, to represent the enforced hard limits and current total usage; these columns will be displayed in the output of kubectl/karmadactl get. ([#6364](https://github.com/karmada-io/karmada/pull/6364), @zhzhuang-zju)

### Deprecation
- `karmadactl`: The flag `--ca-cert-path` of `register`, which has been deprecated in release-1.13, now has been removed. ([#6191](https://github.com/karmada-io/karmada/pull/6191), @husnialhamdani)
- The label `propagation.karmada.io/instruction` now has been deprecated in favor of the field(`.spec.suspendDispatching`) in Work API, the label will be removed in future releases. ([#6043](https://github.com/karmada-io/karmada/pull/6043), @vie-serendipity)
- `karmada-webhook`: The `--default-not-ready-toleration-seconds` and `--default-unreachable-toleration-seconds` flags now have been deprecated, they will remain functional until v1.15. From release 1.15 the two flags will be removed and no default toleration will be added to the newly created PropagationPolicy/ClusterPropagationPolicy. ([#6373](https://github.com/karmada-io/karmada/pull/6373), @XiShanYongYe-Chang)
- `karmada-controller-manager`: The flag `--failover-eviction-timeout` now has been deprecated, it becomes obsolete now as the `cluster-controller` no longer automatically marks NoExecute taints. ([#6405](https://github.com/karmada-io/karmada/pull/6405), @RainbowMango)

### Bug Fixes
- Unify the rate limiter for different clients in each component to access member cluster apiserver. Access may be restricted in large-scale environments compared to before the modification. Administrators can avoid this situation by adjusting 'cluster-api-qps' and 'cluster-api-burst' for karmada-controller-manager, karmada-agent and karmada-metrics-adapter. ([#6192](https://github.com/karmada-io/karmada/pull/6192), @zach593)
- Unify the rate limiter for different clients in each component to access karmada-apiserver. Access may be restricted in large-scale environments compared to before the modification. Administrators can avoid this situation by adjusting upward the rate limit parameters 'kube-api-qps' and 'kube-api-burst' for each component to access karmada-apiserver, and adjusting 'cluster-api-qps' and 'cluster-api-burst' for scheduler-estimator to access member cluster apiserver. ([#6095](https://github.com/karmada-io/karmada/pull/6095), @zach593)
- `karmada-controller-manager`: Fixed the issue that the gracefulEvictionTask of ResourceBinding can not be cleared in case of schedule fails. ([#6227](https://github.com/karmada-io/karmada/pull/6227), @XiShanYongYe-Chang)
- `karmada-controller-manager`/`karmada-agent`: Fixed the issue that cluster status update interval shorter than configured `--cluster-status-update-frequency`. ([#6284](https://github.com/karmada-io/karmada/pull/6284), @LivingCcj)
- `karmada-controller-manager`: The default resource interpreter will no longer populate the `.status.LoadBalancer.Ingress[].Hostname` field with the member cluster name for Service and Ingress resources. ([#6249](https://github.com/karmada-io/karmada/pull/6249), @zach593)
- `karmada-controller-manager`: when cluster is not-ready doesn't clean MultiClusterService and EndpointSlice work. ([#6258](https://github.com/karmada-io/karmada/pull/6258), @XiShanYongYe-Chang)
- `karmada-controller-manager`: add failover flags support-no-execute and no-execute-purge-mode to control taint-manager. ([#6404](https://github.com/karmada-io/karmada/pull/6404), @XiShanYongYe-Chang)
- `karmada-operator`: The `karmada-app` label key previously used for control plane components and for the components of the operator itself has been changed to the more idiomatic `app.kubernetes.io/name` label key. ([#6180](https://github.com/karmada-io/karmada/pull/6180), @jabellard)
- `karmada-scheduler`: Fixed the issue where resource scheduling suspension may become ineffective during a cluster update. ([#6356](https://github.com/karmada-io/karmada/pull/6356), @zhzhuang-zju)
- `karmada-scheduler`: Fixed the issue where bindings that fail occasionally will be treated as unschedulableBindings when feature gate PriorityBasedScheduling is enabled. ([#6369](https://github.com/karmada-io/karmada/pull/6369), @zhzhuang-zju)
- `karmada-webhook`: Added the ClusterTaintPolicy validation webhook, and added `--allow-no-execute-taints` flag. ([#6370](https://github.com/karmada-io/karmada/pull/6370), @XiShanYongYe-Chang)
- `karmada-search`: Suppress protocol buffer serialization explicitly from the server side.  ([#6326](https://github.com/karmada-io/karmada/pull/6326), @ikaven1024)
- `karmada-agent`: Fixed the issue where a new pull-mode cluster may overwrite the existing member clusters. ([#6253](https://github.com/karmada-io/karmada/pull/6253), @zhzhuang-zju)
- `karmada-agent`: Fixed a panic issue where the agent does not need to report secret when registering cluster. ([#6214](https://github.com/karmada-io/karmada/pull/6214), @jabellard)
- `karmadactl`: Fixed the issue where option `discovery-timeout` fails to work properly. ([#6270](https://github.com/karmada-io/karmada/pull/6270), @zhzhuang-zju)
- `helm`: Fixed the issue where the required ServiceAccount was missing when the certificate mode was set to custom. ([#6188](https://github.com/karmada-io/karmada/pull/6188), @seanlaii)

### Security
- Change the listening address of karmada components to the POD IP address to avoid all-zero listening. ([#6266](https://github.com/karmada-io/karmada/issues/6266), @seanlaii, @XiShanYongYe-Chang)

### Features & Enhancements
- `karmada-controller-manager`: Added `federated-resource-quota-enforcement-controller` as part of ResourceQuotaEnforcement feature to update `status.overall` and `status.overallUsed` of FederatedResourceQuota resource. ([#6367](https://github.com/karmada-io/karmada/pull/6367), @mszacillo)
- `karmada-controller-manager`: Introduced a new feature gate `FederatedQuotaEnforcement` for federated resource quota enforcement. ([#6366](https://github.com/karmada-io/karmada/pull/6366), @liwang0513)
- `karmada-controller-manager`: Introduced flag `--federated-resource-quota-sync-period` to control the period for syncing federated resource quota usage status in the system. ([#6407](https://github.com/karmada-io/karmada/pull/6407), @zhzhuang-zju)
- `karmada-controller-manager`: The `federated-resource-quota-status-controller` will skip reflecting overall and overall used in case the Federated Resource Quota Enforcement feature is on. ([#6382](https://github.com/karmada-io/karmada/pull/6382), @liwang0513)
- `karmada-controller-manager`: Adjust the behavior of the legacy staticAssignments to skip creating redundant ResourceQuotas to member clusters which are not explicitly listed in `StaticAssignments`. ([#6363](https://github.com/karmada-io/karmada/issues/6363), @zhzhuang-zju)
- `karmada-controller-manager`: Clusters not explicitly listed in `StaticAssignments` of `FederatedResourceQuota` will no longer have an empty ResourceQuota created. ([#6351](https://github.com/karmada-io/karmada/pull/6351), @RainbowMango)
- `karmada-controller-manager`: Added the `clustertaintpolicy` controller to manage cluster taint based on ClusterTaintPolicy. ([#6368](https://github.com/karmada-io/karmada/pull/6368), @XiShanYongYe-Chang)
- `karmada-controller-manager`: The default resource interpreter will deduplicate and sort `status.Loadbalancer.Ingress` field for Service and Ingress resource when aggregating status. ([#6252](https://github.com/karmada-io/karmada/pull/6252), @zach593)
- `karmada-controller-manager`: All controller reconcile frequency will honor a unified rate limiter configuration(--rate-limiter-*). ([#6145](https://github.com/karmada-io/karmada/pull/6145), @CharlesQQ)
- `karmada-controller-manager`: The clusters-controller no longer automatically adds NoExecute taints to clusters. ([#6389](https://github.com/karmada-io/karmada/pull/6389), @tiansuo114)
- `karmada-controller-manager`: Adjusts scope of Failover feature-gate to encompass TaintManager. If failover is disabled, then taint-based cluster eviction should not occur. ([#6400](https://github.com/karmada-io/karmada/pull/6400), @mszacillo)
- `karmada-controller-manager`: The `cluster-controller` no longer relies on taints to clean up resources on clusters scheduled for deletion, but instead depends on the scheduler to reschedule workloads to other available clusters. ([#6403](https://github.com/karmada-io/karmada/pull/6403), @XiShanYongYe-Chang)
- `karmada-webhook`: Introduced ResourceBinding webhook for validating quota usage. ([#6377](https://github.com/karmada-io/karmada/pull/6377), @seanlaii)
- `karmada-webhook`: The length of FederatedResourceQuota now has been restricted to no more than 63 chars. ([#2168](https://github.com/karmada-io/karmada/pull/2168), @likakuli)
- `karmada-operator`: Users can now configure feature gates for karmada-webhook. ([#6385](https://github.com/karmada-io/karmada/pull/6385), @seanlaii)
- `karmada-agent`: All controller reconcile frequency will honor a unified rate limiter configuration(--rate-limiter-*). ([#6145](https://github.com/karmada-io/karmada/pull/6145), @CharlesQQ)
- Updated Flink interpreter to check error when job state is not published. ([#6287](https://github.com/karmada-io/karmada/pull/6287), @liwang0513)

## Other
### Dependencies
- Kubernetes dependencies have been updated to v1.32.3. ([#6311](https://github.com/karmada-io/karmada/pull/6311), @RainbowMango)
- Bump go version to 1.23.8. ([#6272](https://github.com/karmada-io/karmada/pull/6272), @seanlaii)
- Bump golang.org/x/net to 0.37.0. ([#6225](https://github.com/karmada-io/karmada/pull/6225), @seanlaii)
- Bump mockery to v2.53.3 to remove the hashicorp packages. ([#6212](https://github.com/karmada-io/karmada/pull/6212), @seanlaii)

### Helm Charts
- Added `scheduler.enableSchedulerEstimator` to helm values for karmada chart to allow the scheduler to connect with the scheduler estimator. ([#6286](https://github.com/karmada-io/karmada/pull/6286), @mojojoji)
- `HELM Chart`: OpenID Connect based auth can be configured for the Karmada API server when installing via Helm. ([#6159](https://github.com/karmada-io/karmada/pull/6159), @tw-mnewman)
- `Helm chart`: Added helm index for 1.13 release. ([#6196](https://github.com/karmada-io/karmada/pull/6196), @zhzhuang-zju)
- `Helm Chart`: Users can now configure feature gates for karmada-webhook via the Helm Chart using the `webhook.featureGates` field in values.yaml. ([#6384](https://github.com/karmada-io/karmada/pull/6384), @seanlaii)

### Instrumentation
- Introduced `karmada_build_info` metric, which exposes build metadata, to components `karmada-agent`, `karmada-controller-manager`, `karmada-descheduler`, `karmada-metrics-adapter`, `karmada-scheduler-estimator`, `karmada-scheduler`, and `karmada-webhook`. ([#6215](https://github.com/karmada-io/karmada/pull/6215), @dongjiang1989)
- `karmada-controller-manager`: Fixed the issue that the result label of `federatedhpa_pull_metrics_duration_seconds` is always `success`. ([#6303](https://github.com/karmada-io/karmada/pull/6303), @tangzhongren)
- `karmada-controller-manager`: metric `recreate_resource_to_cluster` has been merged into `create_resource_to_cluster`. ([#6148](https://github.com/karmada-io/karmada/pull/6148), @zach593)
- `karmada-operator`: Introduced `karmada_build_info` metrics to emit the build info, as well as a bunch of Go runtime metrics. ([#6044](https://github.com/karmada-io/karmada/pull/6044), @dongjiang1989)

## Contributors

Thank you to everyone who contributed to this release!

Users whose commits are in this release (alphabetically by username)

- baiyutang
- CharlesQQ
- dongjiang1989
- everpeace
- husnialhamdani
- ikaven1024
- jabellard
- liangyuanpeng
- likakuli
- LivingCcj
- liwang0513
- mohamedawnallah
- mojojoji
- mszacillo
- my-git9
- Pratham-B-Parlecha
- RainbowMango
- rajsinghtech
- seanlaii
- tangzhongren
- tiansuo114
- vie-serendipity
- warjiang
- XiShanYongYe-Chang
- zach593
- zhzhuang-zju

# v1.14.0-rc.0
## Downloads for v1.14.0-rc.0

Download v1.14.0-rc.0 in the [v1.14.0-rc.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.14.0-rc.0).

## Changelog since v1.14.0-beta.0

## Urgent Update Notes
None.

## Changes by Kind

### API Changes
- `API Change`: Clusters not explicitly listed in `StaticAssignments` of `FederatedResourceQuota` will no longer have an empty ResourceQuota created. ([#6351](https://github.com/karmada-io/karmada/pull/6351), @RainbowMango)
- `FederatedResourceQuota`: Added two additional printer columns, `OVERALL` and `OVERALL_USED`, to represent the enforced hard limits and current total usage; these columns will be displayed in the output of kubectl/karmadactl get. ([#6364](https://github.com/karmada-io/karmada/pull/6364), @zhzhuang-zju)
- Introduced `spec.components.karmadaAPIServer.loadBalancerClass` field in `Karmada` API to specify a `loadBalancerClass` to select a specific load balancer implementation, aligning with Kubernetes Service behavior. ([#6348](https://github.com/karmada-io/karmada/pull/6348), @rajsinghtech)
- Introduced a new API named `ClusterTaintPolicy` to handle cluster taint. ([#6319](https://github.com/karmada-io/karmada/pull/6319), @XiShanYongYe-Chang)

### Features & Enhancements
- `karmada-controller-manager`: Added `federated-resource-quota-enforcement-controller` as part of ResourceQuotaEnforcement feature to update `status.overall` and `status.overallUsed` of FederatedResourceQuota resource. ([#6367](https://github.com/karmada-io/karmada/pull/6367), @mszacillo)
- `karmada-controller-manager`: Introduced a new feature gate `FederatedQuotaEnforcement` for federated resource quota enforcement. ([#6366](https://github.com/karmada-io/karmada/pull/6366), @liwang0513)
- `karmada-controller-manager`: The `federated-resource-quota-status-controller` will skip reflecting overall and overall used in case the Federated Resource Quota Enforcement feature is on. ([#6382](https://github.com/karmada-io/karmada/pull/6382), @liwang0513)
- `karmada-controller-manager`: Adjust the behavior of the legacy staticAssignments to skip creating redundant ResourceQuotas to member clusters which are not explicitly listed in `StaticAssignments`. ([#6363](https://github.com/karmada-io/karmada/issues/6363), @zhzhuang-zju)
- `karmada-controller-manager`: Added the `clustertaintpolicy` controller to manage cluster taint based on ClusterTaintPolicy. ([#6368](https://github.com/karmada-io/karmada/pull/6368), @XiShanYongYe-Chang)
- `karmada-webhook`: Introduced ResourceBinding webhook for validating quota usage. ([#6377](https://github.com/karmada-io/karmada/pull/6377), @seanlaii)
- `karmada-webhook`: The length of FederatedResourceQuota now has been restricted to no more than 63 chars. ([#2168](https://github.com/karmada-io/karmada/pull/2168), @likakuli)

### Deprecation
None.

### Bug Fixes
- Unify the rate limiter for different clients in each component to access member cluster apiserver. Access may be restricted in large-scale environments compared to before the modification. Administrators can avoid this situation by adjusting 'cluster-api-qps' and 'cluster-api-burst' for karmada-controller-manager, karmada-agent and karmada-metrics-adapter. ([#6192](https://github.com/karmada-io/karmada/pull/6192), @zach593)
- `karmada-scheduler`: Fixed the issue where resource scheduling suspension may become ineffective during a cluster update. ([#6356](https://github.com/karmada-io/karmada/pull/6356), @zhzhuang-zju)
- `karmada-scheduler`: Fixed the issue where bindings that fail occasionally will be treated as unschedulableBindings when feature gate PriorityBasedScheduling is enabled. ([#6369](https://github.com/karmada-io/karmada/pull/6369), @zhzhuang-zju)

### Security
None.

## Other
### Dependencies
None.

### Helm Charts
None.

### Instrumentation
None.

### Performance
None.

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
- `karmada-controller-manager`/`karmada-agent`: Fixed the issue that cluster status update interval shorter than configured `--cluster-status-update-frequency`. ([#6284](https://github.com/karmada-io/karmada/pull/6284), @LivingCcj)
- `karmada-search`: Suppress protocol buffer serialization explicitly from the server side.  ([#6326](https://github.com/karmada-io/karmada/pull/6326), @ikaven1024)

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
