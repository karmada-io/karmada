---
title: karmada-controller-manager
---



### Synopsis

The karmada-controller-manager runs various controllers.
The controllers watch Karmada objects and then talk to the underlying
clusters' API servers to create regular Kubernetes resources.

```
karmada-controller-manager [flags]
```

### Options

```
Available Commands:
  karmada-controller-manager completion                      Generate the autocompletion script for the specified shell
  karmada-controller-manager help                            Help about any command
  karmada-controller-manager version                         Print the version information

Logs flags:

      --add-dir-header                       If true, adds the file directory to the header of the log messages
      --alsologtostderr                      log to standard error as well as files (no effect when -logtostderr=true)
      --log-backtrace-at traceLocation       when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                       If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log-file string                      If non-empty, use this log file (no effect when -logtostderr=true)
      --log-file-max-size uint               Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --log-flush-frequency duration         Maximum number of seconds between log flushes (default 5s)
      --log-text-info-buffer-size quantity   [Alpha] In text format with split output streams, the info messages can be buffered for a while to increase performance. The default value of zero bytes disables buffering. The size can be specified as number of bytes (512), multiples of 1000 (1K), multiples of 1024 (2Ki), or powers of those (3M, 4G, 5Mi, 6Gi). Enable the LoggingAlphaOptions feature gate to use this.
      --log-text-split-stream                [Alpha] In text format, write error messages to stderr and info messages to stdout. The default is to write a single stream to stdout. Enable the LoggingAlphaOptions feature gate to use this.
      --logging-format string                Sets the log format. Permitted formats: "text". (default "text")
      --logtostderr                          log to standard error instead of files (default true)
      --one-output                           If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --skip-headers                         If true, avoid header prefixes in the log messages
      --skip-log-headers                     If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity             logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true) (default 2)
  -v, --v Level                              number for the log level verbosity
      --vmodule pattern=N,...                comma-separated list of pattern=N settings for file-filtered logging (only works for text log format)

Generic flags:

      --cluster-api-burst int                                          Burst to use while talking with cluster kube-apiserver. (default 60)
      --cluster-api-context string                                     Name of the cluster context in cluster-api management cluster kubeconfig file.
      --cluster-api-kubeconfig string                                  Path to the cluster-api management cluster kubeconfig file.
      --cluster-api-qps float32                                        QPS to use while talking with cluster kube-apiserver. (default 40)
      --cluster-cache-sync-timeout duration                            Timeout period waiting for cluster cache to sync. (default 2m0s)
      --cluster-failure-threshold duration                             The duration of failure for the cluster to be considered unhealthy. (default 30s)
      --cluster-monitor-grace-period duration                          Specifies the grace period of allowing a running cluster to be unresponsive before marking it unhealthy. (default 40s)
      --cluster-monitor-period duration                                Specifies how often karmada-controller-manager monitors cluster health status. (default 5s)
      --cluster-startup-grace-period duration                          Specifies the grace period of allowing a cluster to be unresponsive during startup before marking it unhealthy. (default 1m0s)
      --cluster-status-update-frequency duration                       Specifies how often karmada-controller-manager posts cluster status to karmada-apiserver. (default 10s)
      --cluster-success-threshold duration                             The duration of successes for the cluster to be considered healthy after recovery. (default 30s)
      --concurrent-cluster-propagation-policy-syncs int                The number of ClusterPropagationPolicy that are allowed to sync concurrently. (default 1)
      --concurrent-cluster-syncs int                                   The number of Clusters that are allowed to sync concurrently. (default 5)
      --concurrent-clusterresourcebinding-syncs int                    The number of ClusterResourceBindings that are allowed to sync concurrently. (default 5)
      --concurrent-dependent-resource-syncs int                        The number of dependent resource that are allowed to sync concurrently. (default 2)
      --concurrent-namespace-syncs int                                 The number of Namespaces that are allowed to sync concurrently. (default 1)
      --concurrent-propagation-policy-syncs int                        The number of PropagationPolicy that are allowed to sync concurrently. (default 1)
      --concurrent-resource-template-syncs int                         The number of resource templates that are allowed to sync concurrently. (default 5)
      --concurrent-resourcebinding-syncs int                           The number of ResourceBindings that are allowed to sync concurrently. (default 5)
      --concurrent-work-syncs int                                      The number of Works that are allowed to sync concurrently. (default 5)
      --controllers strings                                            A list of controllers to enable. '*' enables all on-by-default controllers, 'foo' enables the controller named 'foo', '-foo' disables the controller named 'foo'. 
                                                                       All controllers: agentcsrapproving, applicationFailover, binding, bindingStatus, cluster, clusterStatus, clustertaintpolicy, cronFederatedHorizontalPodAutoscaler, deploymentReplicasSyncer, endpointSlice, endpointsliceCollect, endpointsliceDispatch, execution, federatedHorizontalPodAutoscaler, federatedResourceQuotaEnforcement, federatedResourceQuotaStatus, federatedResourceQuotaSync, gracefulEviction, hpaScaleTargetMarker, multiclusterservice, namespace, remedy, serviceExport, serviceImport, unifiedAuth, workStatus, workloadRebalancer.
                                                                       Disabled-by-default controllers: deploymentReplicasSyncer, hpaScaleTargetMarker (default [*])
      --enable-cluster-resource-modeling                               Enable means controller would build resource modeling for each cluster by syncing Nodes and Pods resources.
                                                                       The resource modeling might be used by the scheduler to make scheduling decisions in scenario of dynamic replica assignment based on cluster free resources.
                                                                       Disable if it does not fit your cases for better performance. (default true)
      --enable-pprof                                                   Enable profiling via web interface host:port/debug/pprof/.
      --enable-taint-manager                                           If set to true enables NoExecute Taints and will evict all not-tolerating objects propagating on Clusters tainted with this kind of Taints. (default true)
      --feature-gates mapStringBool                                    A set of key=value pairs that describe feature gates for alpha/experimental features. Options are:
                                                                       AllAlpha=true|false (ALPHA - default=false)
                                                                       AllBeta=true|false (BETA - default=false)
                                                                       ContextualLogging=true|false (BETA - default=true)
                                                                       ControllerPriorityQueue=true|false (BETA - default=true)
                                                                       CustomizedClusterResourceModeling=true|false (BETA - default=true)
                                                                       Failover=true|false (BETA - default=false)
                                                                       FederatedQuotaEnforcement=true|false (ALPHA - default=false)
                                                                       GracefulEviction=true|false (BETA - default=true)
                                                                       LoggingAlphaOptions=true|false (ALPHA - default=false)
                                                                       LoggingBetaOptions=true|false (BETA - default=true)
                                                                       MultiClusterService=true|false (ALPHA - default=false)
                                                                       MultiplePodTemplatesScheduling=true|false (ALPHA - default=false)
                                                                       PriorityBasedScheduling=true|false (ALPHA - default=false)
                                                                       PropagateDeps=true|false (BETA - default=true)
                                                                       PropagationPolicyPreemption=true|false (ALPHA - default=false)
                                                                       ResourceQuotaEstimate=true|false (ALPHA - default=false)
                                                                       StatefulFailoverInjection=true|false (ALPHA - default=false)
                                                                       WorkloadAffinity=true|false (ALPHA - default=false)
      --federated-resource-quota-sync-period duration                  The interval for periodic full resynchronization of FederatedResourceQuota resources. This ensures quota recalculations occur at regular intervals to correct potential inaccuracies, particularly when webhook validation side effects. (default 5m0s)
      --graceful-eviction-timeout duration                             Specifies the timeout period waiting for the graceful-eviction-controller performs the final removal since the workload(resource) has been moved to the graceful eviction tasks. (default 10m0s)
      --health-probe-bind-address string                               The TCP address that the controller should bind to for serving health probes(e.g. 127.0.0.1:10357, :10357). It can be set to "0" to disable serving the health probe. Defaults to 0.0.0.0:10357. (default ":10357")
      --horizontal-pod-autoscaler-cpu-initialization-period duration   The period after pod start when CPU samples might be skipped. (default 5m0s)
      --horizontal-pod-autoscaler-downscale-delay duration             The period since last downscale, before another downscale can be performed in horizontal pod autoscaler. (default 5m0s)
      --horizontal-pod-autoscaler-downscale-stabilization duration     The period for which autoscaler will look backwards and not scale down below any recommendation it made during that period. (default 5m0s)
      --horizontal-pod-autoscaler-initial-readiness-delay duration     The period after pod start during which readiness changes will be treated as initial readiness. (default 30s)
      --horizontal-pod-autoscaler-sync-period duration                 The period for syncing the number of pods in horizontal pod autoscaler. (default 15s)
      --horizontal-pod-autoscaler-tolerance float                      The minimum change (from 1.0) in the desired-to-actual metrics ratio for the horizontal pod autoscaler to consider scaling. (default 0.1)
      --horizontal-pod-autoscaler-upscale-delay duration               The period since last upscale, before another upscale can be performed in horizontal pod autoscaler. (default 3m0s)
      --kube-api-burst int                                             Burst to use while talking with karmada-apiserver. (default 60)
      --kube-api-qps float32                                           QPS to use while talking with karmada-apiserver. (default 40)
      --kubeconfig string                                              Path to karmada control plane kubeconfig file.
      --leader-elect                                                   Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability. (default true)
      --leader-elect-lease-duration duration                           The duration that non-leader candidates will wait after observing a leadership renewal until attempting to acquire leadership of a led but unrenewed leader slot. This is effectively the maximum duration that a leader can be stopped before it is replaced by another candidate. This is only applicable if leader election is enabled. (default 15s)
      --leader-elect-renew-deadline duration                           The interval between attempts by the acting master to renew a leadership slot before it stops leading. This must be less than or equal to the lease duration. This is only applicable if leader election is enabled. (default 10s)
      --leader-elect-resource-namespace string                         The namespace of resource object that is used for locking during leader election. (default "karmada-system")
      --leader-elect-retry-period duration                             The duration the clients should wait between attempting acquisition and renewal of a leadership. This is only applicable if leader election is enabled. (default 2s)
      --metrics-bind-address string                                    The TCP address that the controller should bind to for serving prometheus metrics(e.g. 127.0.0.1:8080, :8080). It can be set to "0" to disable the metrics serving. (default ":8080")
      --profiling-bind-address string                                  The TCP address for serving profiling(e.g. 127.0.0.1:6060, :6060). This is only applicable if profiling is enabled. (default ":6060")
      --rate-limiter-base-delay duration                               The base delay for rate limiter. (default 5ms)
      --rate-limiter-bucket-size int                                   The bucket size for rate limier. (default 100)
      --rate-limiter-max-delay duration                                The max delay for rate limiter. (default 16m40s)
      --rate-limiter-qps int                                           The QPS for rate limier. (default 10)
      --resync-period duration                                         Base frequency the informers are resynced.
      --skipped-propagating-apis string                                Semicolon separated resources that should be skipped from propagating in addition to the default skip list(cluster.karmada.io;policy.karmada.io;work.karmada.io). Supported formats are:
                                                                       <group> for skip resources with a specific API group(e.g. networking.k8s.io),
                                                                       <group>/<version> for skip resources with a specific API version(e.g. networking.k8s.io/v1beta1),
                                                                       <group>/<version>/<kind>,<kind> for skip one or more specific resource(e.g. networking.k8s.io/v1beta1/Ingress,IngressClass) where the kinds are case-insensitive.
      --skipped-propagating-namespaces strings                         Comma-separated namespaces that should be skipped from propagating.
                                                                       Note: 'karmada-system', 'karmada-cluster' and 'karmada-es-.*' are Karmada reserved namespaces that will always be skipped. (default [kube-.*])

Cluster failover flags:

      --enable-no-execute-taint-eviction              Enables controller response to NoExecute taints on clusters, which triggers eviction of workloads without explicit tolerations. Given the impact of eviction caused by NoExecute Taint, this parameter is designed to remain disabled by default and requires careful evaluation by administrators before being enabled.
                                                      
      --no-execute-taint-eviction-purge-mode string   Controls resource cleanup behavior for NoExecute-triggered evictions (only active when --enable-no-execute-taint-eviction=true). Supported values are "Directly", and "Gracefully". "Directly" mode directly evicts workloads first (risking temporary service interruption) and then triggers rescheduling to other clusters, while "Gracefully" mode first schedules workloads to new clusters and then cleans up original workloads after successful startup elsewhere to ensure service continuity. (default "Gracefully")
      --resource-eviction-rate float32                This is the number of resources to be evicted per second in a cluster failover scenario. (default 0.5)
```

###### Auto generated by [spf13/cobra script in Karmada](https://github.com/karmada-io/karmada/tree/master/hack/tools/gencomponentdocs)