# What's New
## Multi-Cluster Ingress
The newly introduced [MultiClusterIngress](https://github.com/karmada-io/karmada/blob/d6355ec85296daa46ed344cade6ef10a9bee58dc/pkg/apis/networking/v1alpha1/ingress_types.go#L16) 
API exposes HTTP and HTTPS routes that target multi-cluster services within the Karmada control plane. The specification 
of `MultiClusterIngress` is compatible with [Kubernetes Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/).

Traffic routing is controlled by rules defined on the MultiClusterIngress resource, an MultiClusterIngress controller is
responsible for fulfilling the ingress. The [Multi-Cluster-Nginx Ingress Controller](https://github.com/karmada-io/multi-cluster-ingress-nginx) 
is one of the MultiClusterIngress controller implementations maintained by the community.


## Federated ResourceQuota
The newly introduced [FederatedResourceQuota](https://github.com/karmada-io/karmada/blob/master/pkg/apis/policy/v1alpha1/federatedresourcequota_types.go#L14)
provides constraints that limit total resource consumption per namespace `across all clusters`. 
It can limit the number of objects that can be created in a namespace by type, as well as the total amount of compute 
resources that may be consumed by resources in that namespace.


## Configurability improvement for performance tuning
The default number of reconciling workers has been enlarged and configurable. A larger number of workers means higher 
responsiveness but heavier CPU and network load. The number of concurrent workers could be configured by the flags 
introduced to `karmada-controller-manager` and `karmada-agent`.

Flags introduced to `karmada-controller-manager`:

- `--concurrent-work-syncs`
- `--concurrent-namespace-syncs`
- `--concurrent-resource-template-syncs`
- `--concurrent-cluster-syncs`
- `--concurrent-clusterresourcebinding-syncs`
- `--concurrent-resourcebinding-syncs`

Flags introduced to `karmada-agent`:

- `--concurrent-work-syncs`
- `--concurrent-cluster-syncs`


## Resource Interpreter Webhook Enhancement
Introduced `AggregateStatus` support for the `Resource Interpreter Webhook` framework, which enables customized resource status aggregating.

Introduced `InterpreterOperationInterpretDependency` support for the `Resource Interpreter Webhook` framework, 
which enables propagating workload's dependencies automatically.

Refer to [Customizing Resource Interpreter](https://github.com/karmada-io/website/blob/main/docs/userguide/globalview/customizing-resource-interpreter.md) for more details.


# Other Notable Changes
## Bug Fixes
- `karmadactl` and `kubectl-karmada`: Fixed that init cannot update the `APIService`. [#1207](https://github.com/karmada-io/karmada/pull/1207)
- `karmada-controller-manager`: Fixed ApplyPolicySucceed event type mistake (should be `Normal` but not `Warning`). [#1267](https://github.com/karmada-io/karmada/pull/1267)
- `karmada-controller-manager` and `karmada-agent`: Fixed that resync slows down reconciliation. [1265](https://github.com/karmada-io/karmada/pull/1265)
- `karmada-controller-manager`/`karmada-agent`: Fixed continually updating cluster status due to unordered apiEnablements. [#1304](https://github.com/karmada-io/karmada/pull/1304)
- `karmada-controller-manager`: Fixed that Replicas set by OverridePolicy will be reset by the ReviseReplica interpreterhook. [#1352](https://github.com/karmada-io/karmada/pull/1352)
- `karmada-controller-manager`: Fixed that ResourceBinding couldn't be created in a corner case. [#1368](https://github.com/karmada-io/karmada/pull/1368)
- `karmada-scheduler`: Fixed inaccuracy in requested resources in the case that pod limits are specified but requests are not. [#1225](https://github.com/karmada-io/karmada/pull/1225)
- `karmada-scheduler`: Fixed spreadconstraints[i].MaxGroups is invalidated in some scenarios. [#1324](https://github.com/karmada-io/karmada/pull/1324)
## Features & Enhancements
- `karmadactl`: Introduced --tls-min-version flag to specify the minimum TLS version.  [#1278](https://github.com/karmada-io/karmada/pull/1278)
- `karmadactl`: Improved the get command to show more useful information. [#1270](https://github.com/karmada-io/karmada/pull/1270)
- `karmada-controller-manager`/`karmada-agent`: Introduced --resync-period flag to specify reflector resync period (defaults to 0, meaning no resync). [#1261](https://github.com/karmada-io/karmada/pull/1261)
- `karmada-controller-manager`: Introduced --metrics-bind-address flag to specify the customized address for metrics. [#1341](https://github.com/karmada-io/karmada/pull/1341)
- `karmada-webhook`: Introduced --metrics-bind-address and --health-probe-bind-address flags. [#1346](https://github.com/karmada-io/karmada/pull/1346)
## Instrumentation (Metrics and Events)
- `karmada-controller-manager`: Fixed ApplyPolicySucceed event type mistake (should be Normal but not Warning). [#1267](https://github.com/karmada-io/karmada/pull/1267)
## Deprecation
- `OverridePolicy`/`ClusterOverridePolicy`: The `.spec.targetCluster` and `spec.overriders` have been deprecated in favor of `spec.overrideRules`. [#1238](https://github.com/karmada-io/karmada/pull/1238)
- `karmada-aggregate-apiserver`: Deprecated `--master` and `--karmada-config` flags. Please use `--kubeconfig` instead. [#1336](https://github.com/karmada-io/karmada/pull/1336)
