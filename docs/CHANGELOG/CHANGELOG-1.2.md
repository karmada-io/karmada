
* [v1.2.0](#v120)
  * [Downloads for v1.2.0](#downloads-for-v120)
  * [Karmada v1.2 Release Notes](#karmada-v12-release-notes)
    * [1.2 What's New](#whats-new)
    * [Other Notable Changes](#other-notable-changes)

# v1.2.0

## Downloads for v1.2.0
Download v1.2.0 in the [v1.2.0 release page](https://github.com/karmada-io/karmada/releases/tag/v1.2.0).

## Karmada v1.2 Release Notes

### What's New
#### Significant improvement on scheduling capability and scalability
##### 1. Karmada Descheduler
A new component `karmada-descheduler` was introduced, for rebalancing the scheduling decisions over time.
   
One example use case is: it helps evict pending replicas (Pods) from resource-starved clusters so that `karmada-scheduler`
can "reschedule" these replicas (Pods) to a cluster with sufficient resources.

For more details please refer to [Descheduler user guide.](https://github.com/karmada-io/website/blob/main/docs/userguide/scheduling/descheduler.md)


##### 2. Multi region HA support
By leveraging the newly added `spread-by-region` constraint, users are now able to deploy workloads across `regions`, 
e.g. people may want their workloads always running on different regions for HA purposes.

We also introduced two plugins to `karmada-scheduler`, which add to accurate scheduling.

- `ClusterLocality` is a scoring plugin that favors clusters already assigned.
- `SpreadConstraint` is a filter plugin that filters clusters as per spread constraints.

We are also in the progress of enhancing the multi-cluster failover mechanism. Part of the work has been included in this release.
For example:

- A new flag(`--cluster-failure-threshold`) has been added to both `karmada-controller-manager` and `karmada-agent`, 
  which specifies the cluster failure threshold (defaults to 30s). A cluster will be considered `not-ready` 
  only when it stays unhealthy longer than supposed.
- A new flag(`--failover-eviction-timeout`) has been added to `karmada-controller-manager`, which specifies the grace period 
  of eviction (defaults to 5 minutes). If a cluster stays `not-ready` longer than supposed, the controller taints the cluster. 
  (Note: The taint is essentially the eviction order and the implementation is planned for the next release.)


#### Fully adopted aggregated API
The `Aggregated API` was initially introduced in Release 1.0, which allows users to access clusters through Karmada by 
a single aggregated API endpoint. By leveraging this feature, we introduced a lot of interesting features to `karmadactl` and `kubectl-karmada`.

1. The get sub-command now supports clusters both in push and pull mode.
```shell
# karmadactl get deployment -n default
NAME      CLUSTER   READY   UP-TO-DATE   AVAILABLE   AGE     ADOPTION
nginx     member1   2/2     2            2           33h     N
nginx     member2   1/1     1            1           4m38s   Y
podinfo   member3   2/2     2            2           27h     N
```
2. The newly added logs command prints the container logs in a specific cluster.
```shell
# ./karmadactl logs nginx-6799fc88d8-9mpxn -c nginx  -C member1
/docker-entrypoint.sh: /docker-entrypoint.d/ is not empty, will attempt to perform configuration
/docker-entrypoint.sh: Looking for shell scripts in /docker-entrypoint.d/
/docker-entrypoint.sh: Launching /docker-entrypoint.d/10-listen-on-ipv6-by-default.sh
10-listen-on-ipv6-by-default.sh: info: Getting the checksum of /etc/nginx/conf.d/default.conf
...
```

3. We also added `watch` and `exec` commands to `karmadactl`, in addition to `get` and `logs`. They all use the aggregated API.


#### Distributed search and analytics engine for Kubernetes resources (`alpha`)
The newly introduced `karmada-search` caches resources in clusters and allows users to search for resources without directly touching real clusters.

```shell
# kubectl get --raw /apis/search.karmada.io/v1alpha1/search/cache/apis/apps/v1/deployments
{
	"apiVersion": "v1",
	"kind": "List",
	"metadata": {},
	"items": [{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"annotations": {
				"cluster.karmada.io/name": "member1",
			},
		}
	},
	]
}
```

The `karmada-search` also supports syncing cached resources to backend stores like [Elasticsearch](https://en.wikipedia.org/wiki/Elasticsearch)
or [OpenSearch](https://github.com/opensearch-project/OpenSearch). By leveraging the search engine, you can perform 
full-text searches with all desired features, by field, and by indice; rank results by score, sort results by field, and aggregate results.


#### Resource Interpreter Webhook enhancement
Introduced `InterpretStatus` for the `Resource Interpreter Webhook` framework, which enables customized resource status collection.

Karmada can thereby learn how to collect status for your resources, especially custom resources. For example, a custom resource 
may have many status fields and only Karmada can collect only those you want.

Refer to [Customizing Resource Interpreter](https://github.com/karmada-io/website/blob/main/docs/userguide/globalview/customizing-resource-interpreter.md) for more details.


#### Integrating verification with the ecosystem
Benefiting from the Kubernetes native APIs, Karmada can easily integrate the Kubernetes ecosystem. The following components are verified by the Karmada community:

- `Kyverno`: policy engine. Refer to [working with kyverno](https://github.com/karmada-io/website/blob/main/docs/userguide/security-governance/working-with-kyverno.md) for more details.
- `Gatekeeper`: another policy engine. Refer to [working with gatekeeper](https://github.com/karmada-io/website/blob/main/docs/userguide/security-governance/working-with-gatekeeper.md) for more details.
- `fluxcd`: GitOps tooling for helm chart. Refer to [working with fluxcd](https://github.com/karmada-io/website/blob/main/docs/userguide/cicd/working-with-flux.md) for more details.


### Other Notable Changes
#### Bug Fixes
- karmadactl: Fixed the cluster joining failures in the case of legacy secrets. ([#1306](https://github.com/karmada-io/karmada/pull/1306))
- karmadactl: Fixed the issue that you cannot use the '-v 6' log level. ([#1426](https://github.com/karmada-io/karmada/pull/1426))
- karmadactl: Fixed the issue that the --namespace flag of init command did not work. ([#1416](https://github.com/karmada-io/karmada/pull/1416))
- karmadactl: Allowed namespaces to be customized. ([#1449](https://github.com/karmada-io/karmada/pull/1449))
- karmadactl: Fixed the init failure due to data path not clean. ([#1455](https://github.com/karmada-io/karmada/pull/1455))
- karmadactl: Fixed the init failure to read the KUBECONFIG environment variable. ([#1437](https://github.com/karmada-io/karmada/pull/1437))
- karmadactl: Fixed the init command failure to select the default release version. ([#1456](https://github.com/karmada-io/karmada/pull/1456))
- karmadactl: Fixed the issue that the karmada-system namespace already exists when deploying karmada-agent. ([#1604](https://github.com/karmada-io/karmada/pull/1604))
- karmadactl: Fixed the issue that the karmada-controller-manager args did not honor customized namespaces.` ([#1683](https://github.com/karmada-io/karmada/pull/1683))
- karmadactl: Fixed a panic due to nil annotation when promoting resources to Karmada.` ([#1759](https://github.com/karmada-io/karmada/pull/1759))
- karmadactl: Fixed the promote command failure to migrate cluster-scoped resources. ([#1766](https://github.com/karmada-io/karmada/pull/1766))
- karmadactl: fixed the karmadactl taint failure while the karmada control plane config is not located in the default path. ([#1825](https://github.com/karmada-io/karmada/pull/1825))
- helm-chart: Fixed the karmada-agent installation failure due to the lack of permission. ([#1457](https://github.com/karmada-io/karmada/pull/1457)))
- helm-chart: Fixed the issue that version constraints skip pre-releases. ([#1444](https://github.com/karmada-io/karmada/pull/1444))
- karmada-controller-manager: Fixed the issue that ResourceBinding may hinder en-queue in the case of schedule failures. ([#1499](https://github.com/karmada-io/karmada/pull/1499))
- karmada-controller-manager: Fixed the panic when the interpreter webhook returns nil patch. ([#1584](https://github.com/karmada-io/karmada/pull/1584))
- karmada-controller-manager: Fixed the RB/CRB controller failure to aggregate status in the case of work condition update. ([#1513](https://github.com/karmada-io/karmada/pull/1513))
- karmada-aggregate-apiserver: Fixed timeout issue when requesting cluster/proxy with options -w or logs -f from karmadactl get. ([#1620](https://github.com/karmada-io/karmada/pull/1620))
- karmada-aggregate-apiserver: Fixed exec failed: error: unable to upgrade connection: you must specify at least 1 of stdin, stdout, stderr. ([#1632](https://github.com/karmada-io/karmada/pull/1632))
#### Features & Enhancements
- karmada-controller-manager: Introduced several flags to specify controller's concurrent capacities(--rate-limiter-base-delay, --rate-limiter-max-delay, --rate-limiter-qps, --rate-limiter-bucket-size). ([#1399](https://github.com/karmada-io/karmada/pull/1399))
- karmada-controller-manager: The klog flags now have been grouped for better readability. ([#1468](https://github.com/karmada-io/karmada/pull/1468))
- karmada-controller-manager: Fixed the FullyApplied condition of ResourceBinding/ClusterResourceBinding mislabeling issue in the case of non-scheduling. ([#1512](https://github.com/karmada-io/karmada/pull/1512))
- karmada-controller-manager: Added default AggregateStatus webhook for DaemonSet and StatefulSet. ([#1586](https://github.com/karmada-io/karmada/pull/1586))
- karmada-controller-manager: OverridePolicy with empty ResourceSelector will be considered to match all resources just like nil. ([#1706](https://github.com/karmada-io/karmada/pull/1706))
- karmada-controller-manager: Introduced --failover-eviction-timeout to specify the grace period of eviction. Tants(cluster.karmada.io/not-ready or cluster.karmada.io/unreachable) will be set on unhealthy clusters after the period. ([#1781](https://github.com/karmada-io/karmada/pull/1781))
- karmada-controller-manager/karmada-agent: Introduced --cluster-failure-threshold flag to specify cluster failure threshold. ([#1829](https://github.com/karmada-io/karmada/pull/1829))
- karmada-scheduler: Workloads can now be rescheduled after the cluster is unregistered. ([#1383](https://github.com/karmada-io/karmada/pull/1383))
- karmada-scheduler: The klog flags now have been grouped for better readability. ([#1491](https://github.com/karmada-io/karmada/pull/1491))
- karmada-scheduler: Added a scoring plugin ClusterLocality to favor clusters already requested. ([#1334](https://github.com/karmada-io/karmada/pull/1334))
- karmada-scheduler: Introduced filter plugin SpreadConstraint to filter clusters that do not meet the spread constraints. ([#1570](https://github.com/karmada-io/karmada/pull/1570))
- karmada-scheduler: Supported spread constraints by region strategy. ([#1646](https://github.com/karmada-io/karmada/pull/1646))
- karmada-webhook: Introduced --tls-cert-file-name and --tls-private-key-file-name flags to specify the server certificate and private key. ([#1464](https://github.com/karmada-io/karmada/pull/1464))
- karmada-agent: The klog flags now have been grouped for better readability. ([#1389](https://github.com/karmada-io/karmada/pull/1389))
- karmada-agent: Introduced several flags to specify controller's concurrent capacities(--rate-limiter-base-delay, --rate-limiter-max-delay, --rate-limiter-qps, --rate-limiter-bucket-size). ([#1505](https://github.com/karmada-io/karmada/pull/1505))
- karmada-scheduler-estimator: The klog flags now have been grouped for better readability. ([#1493](https://github.com/karmada-io/karmada/pull/1493))
- karmadactl: Introduced --context flag to specify the context name to use. ([#1748](https://github.com/karmada-io/karmada/pull/1748))
- karmadactl: Introduced --kube-image-mirror-country and --kube-image-registry flags to init subcommand for Chinese mainland users. ([#1764](https://github.com/karmada-io/karmada/pull/1764))
- karmadactl: Introduced deinit sub-command to uninstall Karmada. ([#1337](https://github.com/karmada-io/karmada/pull/1337))
- Introduced Swagger docs for Karmada API. ([#1401](https://github.com/karmada-io/karmada/pull/1401))
#### Other (Dependencies)
The base image alpine has been promoted to v3.15.1. ([#1519](https://github.com/karmada-io/karmada/pull/1519))
#### Deprecation
- karmada-controller-manager: The hpa controller is disabled by default now. ([#1580](https://github.com/karmada-io/karmada/pull/1580))
- karmada-aggregated-apiserver: The deprecated flags --karmada-config and --master in v1.1 have been removed from the codebase. ([#1834](https://github.com/karmada-io/karmada/pull/1834))

