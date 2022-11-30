* [v1.3.0](#v130)
  * [Downloads for v1.3.0](#downloads-for-v130)
  * [Karmada v1.3 Release Notes](#karmada-v13-release-notes)
    * [1.3 What's New](#whats-new)
    * [Other Notable Changes](#other-notable-changes)

# v1.3.0

## Downloads for v1.3.0
Download v1.3.0 in the [v1.3.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.3.0).

## Karmada v1.3 Release Notes

### What's New
#### Taint-based eviction in graceful way

We introduced a new controller named `taint manager` which aims to evict workloads from faulty clusters after a grace period. 
Then the scheduler would select new best-fit clusters for the workloads. In addition, if the feature `GracefulEviction` is enabled, 
the eviction will be very smooth, that is, the removal of evicted workloads will be delayed until the workloads are available on 
new clusters or reach the maximum grace period. For more details please refer to [Failover Overview](https://karmada.io/docs/userguide/failover/). 

(Feature contributor: @Garrybest, @XiShanYongYe-Chang)

#### Global proxy for resources across multi-clusters

We introduced a new proxy feature to `karmada-search` that allows users to access resources in multiple clusters in a way just like accessing resources in a single cluster. No matter whether the resources are managed by Karmada, by leveraging the proxy, users 
can manipulate the resources from the Karmada control plane.
For more details please refer to [Global Resource Proxy](https://karmada.io/docs/userguide/globalview/proxy-global-resource).

(Feature contributor: @ikaven1024, @XiShanYongYe-Chang)

#### Cluster resource modeling

To provide a more accurate scheduling basis for the scheduler, we introduced a way to model the cluster's available resources.
The cluster status controller will model the resources as per the customized resource models, which is more accurate than the general resource summary. For more details please refer to [Cluster Resource Modeling](https://karmada.io/docs/userguide/scheduling/cluster-resources).

(Feature contributor: @halfrost, @Poor12)

#### Bootstrap token-based cluster registration

Now for clusters in `Pull` mode, we provide a way for them to register with the Karmada control plane. By leveraging the commands `token` and `register` in `kubectl`, the registration process including deploying the `karmada-agent` can be completed very easily. For more details please refer to [Register cluster with Pull mode](https://karmada.io/docs/userguide/clustermanager/cluster-registration#register-cluster-with-push-mode).

(Feature contributor: @lonelyCZ )

#### Significant improvement in system scalability 

We improved the system scalability, such as:
- Enable pprof([#2008](https://github.com/karmada-io/karmada/pull/2008))
- Introduce `cachedRESTMapper`([#2187](https://github.com/karmada-io/karmada/pull/2187))
- Adopt the transform function to reduce memory usage([#2383](https://github.com/karmada-io/karmada/pull/2383))

With these improvements, Karmada can easily manage hundreds of huge clusters. The detailed test report will be released soon.

### Other Notable Changes

#### API changes
- The `Cluster` API is added optional field `ID` to uniquely identify the cluster. (@RainbowMango, [#2180](https://github.com/karmada-io/karmada/pull/2180))
- The `Cluster` API is added optional field `ProxyHeader` to specify the HTTP header required by the proxy server. (@mrlihanbo, [#1874](https://github.com/karmada-io/karmada/pull/1874))
- The `Cluster` API is added optional field named `ResourceModels` to specify resource modeling. (@halfrost, [#2386](https://github.com/karmada-io/karmada/pull/2386))
- The `Work` and `ResourceBinding`/`ClusterResourceBinding` APIs are added field `health` to represent the state of workload. (@XiShanYongYe-Chang, [#2351](https://github.com/karmada-io/karmada/pull/2351))

#### Bug Fixes
- `karmadactl`: Fixed issue that Kubernetes v1.24 cannot be joined. (@zgfh, [#1972](https://github.com/karmada-io/karmada/pull/1972))
- `karmadactl`: Fixed a panic issue when retrieving resources from an unknown cluster(`karmadactl get xxx --cluster=not-exist`). (@my-git9, [#2171](https://github.com/karmada-io/karmada/pull/2171))
- `karmadactl`: Fixed failed promoting if a resource with another kind using the same name has been promoted before. (@wuyingjun-lucky, [#1824](https://github.com/karmada-io/karmada/pull/1824))
- `karmada-search`: Fixed panic when the resource annotation is nil. (@XiShanYongYe-Chang, [#1921](https://github.com/karmada-io/karmada/pull/1921))
- `karmada-search`: Fixed panic `comparing uncomparable type cache.ResourceEventHandlerFuncs`. (@liys87x, [#1951](https://github.com/karmada-io/karmada/pull/1951))
- `karmada-search`: Fixed failed query on a single namespace (@luoMonkeyKing, [#2227](https://github.com/karmada-io/karmada/pull/2227))
- `karmada-controller-manager`: Fixed that `Job` status might be incorrectly marked as `Completed`. (@Garrybest, [#1987](https://github.com/karmada-io/karmada/pull/1987))
- `karmada-controller-manager`: Fixed returning err when the interpreter webhook returns nil patch and nil patchType. (@CharlesQQ, [#2161](https://github.com/karmada-io/karmada/pull/2161))
- `karmada-controller-manager`: Fixed that Argo CD cannot assess Deployment health status. (@xuqianjins, [#2241](https://github.com/karmada-io/karmada/pull/2241))
- `karmada-controller-manager`: Fixed that Argo CD cannot assess StatefulSet/DaemonSet health status. (@RainbowMango, [#2252](https://github.com/karmada-io/karmada/pull/2252))
- `karmada-controller-manager`/`karmada-agent`: Fixed an resource status can not be collected issue in case of Resource Interpreter returns an error. (@XiShanYongYe-Chang, [#2428](https://github.com/karmada-io/karmada/pull/2428))
- `karmada-sechduler`: Fixed a panic issue when `replicaDivisionPreference` is `Weighted` and `WeightPreference` is nil. (@XiShanYongYe-Chang, [#2451](https://github.com/karmada-io/karmada/pull/2451))

#### Features & Enhancements
- `karmadactl`: Added `--force` flag to `deinit` to skip confirmation. (@zgfh, [#2016](https://github.com/karmada-io/karmada/pull/2016))
- `karmadactl`: The flag `-c` of sub-command `promote` now has been changed to uppercase `-C`. (@Fish-pro, [#2140](https://github.com/karmada-io/karmada/pull/2140))
- ``karmadactl`: Introduced `--cluster-zone` and `--cluster-region` flags to `join` command to specify the zone and region of joining cluster. (@chaunceyjiang, [#2048](https://github.com/karmada-io/karmada/pull/2048))
- `karmadactl`: Introduced `--namespace` flag to `exec` command to specify the workload namespace. (@carlory, [#2092](https://github.com/karmada-io/karmada/pull/2092))
- `karmadactl`:  Allowed reading namespaces from the context field of karmada config for `get` command. (@carlory, [#2148](https://github.com/karmada-io/karmada/pull/2148))
- `karmadactl`: Introduced `apply` subcommand to apply a configuration to a resource by file name or stdin. (@carlory, [#2000](https://github.com/karmada-io/karmada/pull/2000))
- `karmadactl`: Introduced `--namespace` flag to `describe` command to specify the namespace the workload belongs to. (@TheStylite, [#2153](https://github.com/karmada-io/karmada/pull/2153))
- `karmadactl`: Introduced `--cluster` flag for `apply` command to allow users to select one or many member clusters to propagate resources. (@carlory, [#2192](https://github.com/karmada-io/karmada/pull/2192))
- `karmadactl`: Introduced options subcmd to list global command-line options. (@lonelyCZ, [#2283](https://github.com/karmada-io/karmada/pull/2283))
- `karmadactl`: Introduced the` token` command to manage bootstrap tokens. (@lonelyCZ, [#2399](https://github.com/karmada-io/karmada/pull/2399))
- `karmadactl`: Introduced the` register` command for joining PULL mode cluster. (@lonelyCZ, [#2388](https://github.com/karmada-io/karmada/pull/2388))
- `karmada-scheduler`: Introduced `--enable-empty-workload-propagation` flag to enable propagating empty workloads. (@CharlesQQ, [#1720](https://github.com/karmada-io/karmada/pull/1720))
- `karmada-scheduler`: Allowed extended plugins in an out-of-tree mode. (@kerthcet, [#1663](https://github.com/karmada-io/karmada/pull/1663))
- `karmada-scheduler`: Introduced  `--disable-scheduler-estimator-in-pull-mode` flag to disable scheduler-estimator for clusters in pull mode. (@prodanlabs, [#2064](https://github.com/karmada-io/karmada/pull/2064))
- `karmada-scheduler`: Introduced `--plugins` flag to enable or disable scheduler plugins. (@chaunceyjiang, [#2135](https://github.com/karmada-io/karmada/pull/2135))
- `karmada-scheduler`: Now the scheduler starts to re-schedule in case of cluster state changes. (@chaunceyjiang, [#2301](https://github.com/karmada-io/karmada/pull/2301))
- `karmada-search`: The search API supports searching for resources according to labels. (@XiShanYongYe-Chang, [#1917](https://github.com/karmada-io/karmada/pull/1917))
- `karmada-search`: The annotation `cluster.karmada.io/name` which is used to represent the source of cache now has been changed to `resource.karmada.io/cached-from-cluster`. (@calvin0327, [#1960](https://github.com/karmada-io/karmada/pull/1960))
- `karmada-search`: Fixed panic issue when dumping error info. (@AllenZMC, [#2231](https://github.com/karmada-io/karmada/pull/2331))
- `karmada-controller-manager/karmada-agent`: Cluster state controller now able to collect partial API list in the case of discovery failure. (@duanmengkk, [#1968](https://github.com/karmada-io/karmada/pull/1968))
- `karmada-controller-manager/karmada-agent`: Introduced `--cluster-success-threshold` flag to specify cluster success threshold. Default to 30s. (@dddddai, [#1884](https://github.com/karmada-io/karmada/pull/1884))
- `karmada-controller-manager/karmada-agent`: Added CronJob support to the default resource interpreter framework. (@chaunceyjiang, [#2060](https://github.com/karmada-io/karmada/pull/2060))
- `karmada-controller-manager`/`karmada-agent`: Introduced `--leader-elect-lease-duration`, `--leader-elect-renew-deadline` and `--leader-elect-retry-period` flags to specify leader election behaviors. (@CharlesQQ, [#2056](https://github.com/karmada-io/karmada/pull/2056))
- `karmada-controller-manager`/`karmada-agent` : Fixed panic issue when dumping error infos. (@AllenZMC, [#2117](https://github.com/karmada-io/karmada/pull/2117))
- `karmada-controller-manager`/`karmada-agent`: Supported interpreting health state by levering `Resource Interpreter Framework`. (@zhuwint, [#2329](https://github.com/karmada-io/karmada/pull/2329))
- `karmada-controller-manager`/`karmada-agent`: Introduced `--enable-cluster-resource-modeling` flag to enable or disable cluster modeling feature. (@RainbowMango, [#2387](https://github.com/karmada-io/karmada/pull/2387))
- `karmada-controller-manager`/`karmada-agent`: Now able to retain `.spec.claimRef` field of `PersistentVolume`. (@Garrybest, [#2415](https://github.com/karmada-io/karmada/pull/2415))
- `karmada-controller-manager`:  `interpreter framework` starts to support Pod state aggregation.  (@xyz2277, [#1913](https://github.com/karmada-io/karmada/pull/1913))
- `karmada-controller-manager`:  `interpreter framework` starts to support PVC state aggregation. (@chaunceyjiang, [#2070](https://github.com/karmada-io/karmada/pull/2070))
- `karmada-controller-manager`: Stopped reporting and refreshing `lease` for clusters in `Push` mode. (@dddddai, [#2033](https://github.com/karmada-io/karmada/pull/2033))
- `karmada-controller-manager`:  `interpreter framework` starts to support Pod  `Failded` and `Succeeded`  aggregation.  (@chaunceyjiang, [#2146](https://github.com/karmada-io/karmada/pull/2146))
- `karmada-controller-manager`: `namespace` controller starts to apply `ClusterOverridePolicy` during propagation namespaces. (@zirain, [#2263](https://github.com/karmada-io/karmada/pull/2263))
- `karmada-controller-manager`: Propagation dependencies support propagating ServiceAccounts. (@chaunceyjiang, [#2035](https://github.com/karmada-io/karmada/pull/2035))
- `karmada-agent`: Introduced `--metrics-bind-address` flag to specify the address for serving Prometheus metrics. (@1953, [#1953](https://github.com/karmada-io/karmada/pull/1953))
- `karmada-agent`: Introduced `--report-secrets` flag to allow secrets to be reported to the Karmada control plane during registering. (@CharlesQQ, [#1990](https://github.com/karmada-io/karmada/pull/1990))
- `karmada-agent`: Introduced `--cluster-provider` and `--cluster-region` flags to specify cluster-provider and cluster-region during registering. (@CharlesQQ, [#2152](https://github.com/karmada-io/karmada/pull/2152))
- `karmada-webhook`: Added default tolerations, defaultNotReadyTolerationSeconds, and defaultUnreachableTolerationSeconds, for (Cluster)PropagationPolicy. (@Garrybest, [#2284](https://github.com/karmada-io/karmada/pull/2284))
- `karmada-webhook`: The '.spec.ttlSecondsAfterFinished' field of the Job resource will be removed before propagating to member clusters. (@chaunceyjiang, [#2294](https://github.com/karmada-io/karmada/pull/2294))
- `karmada-agent`/`karmadactl`: Now an error will be reported when registering the same cluster to Karmada. (@yy158775, [#2369](https://github.com/karmada-io/karmada/pull/2369))

### Other

#### Helm Chart
- `Helm chart`: Updated default `kube-apiserver` from v1.21 to v1.22. (@AllenZMC, [#1941](https://github.com/karmada-io/karmada/pull/1941))
- `Helm Chart`: Added missing `APIService` configuration for `karmada-aggregated-apiserver`. (@zhixian82, [#2258](https://github.com/karmada-io/karmada/pull/2258))
- `Helm Chart`: Fixed the webhook service mismatch issue in the case of customized release name. (@calvin0327, [#2275](https://github.com/karmada-io/karmada/pull/2275))
- `Helm Chart`: Introduced `--cluster-api-endpoint` for `karmada-agent`. (@my-git9, [#2299](https://github.com/karmada-io/karmada/pull/2299))
- `Helm Chart`: Fixed misconfigured `MutatingWebhookConfiguration`. (@zhixian82, [#2401](https://github.com/karmada-io/karmada/pull/2401))

#### Dependencies
- Karmada is now built with Golang 1.18.3. (@RainbowMango, [#2032](https://github.com/karmada-io/karmada/pull/2032))
- Kubernetes dependencies are now updated to v1.24.2. (@RainbowMango, [#2050](https://github.com/karmada-io/karmada/pull/2050))

#### Deprecation
- `karmadactl`: Removed `--dry-run` flag from `describe`, `exec` and `log` commands. (@carlory, [#2023](https://github.com/karmada-io/karmada/pull/2023))
- `karmadactl`: Removed the `--cluster-namespace` flag for `get` command. (@carlory, [#2190](https://github.com/karmada-io/karmada/pull/2190))
- `karmadactl`: Removed the `--cluster-namespace` flag for `promote` command. (@carlory, [#2193](https://github.com/karmada-io/karmada/pull/2193))