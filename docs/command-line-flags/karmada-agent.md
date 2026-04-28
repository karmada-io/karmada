---
title: karmada-agent
---



### Synopsis

The karmada-agent is the agent of member clusters. It can register a specific cluster to the Karmada control
plane and sync manifests from the Karmada control plane to the member cluster. In addition, it also syncs the status of member
cluster and manifests to the Karmada control plane.

```
karmada-agent [flags]
```

### Options

```
Available Commands:
  karmada-agent completion                      Generate the autocompletion script for the specified shell
  karmada-agent help                            Help about any command
  karmada-agent version                         Print the version information

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

      --cert-rotation-checking-interval duration       The interval of checking if the certificate need to be rotated. This is only applicable if cert rotation is enabled (default 5m0s)
      --cert-rotation-remaining-time-threshold float   The threshold of remaining time of the valid certificate. This is only applicable if cert rotation is enabled. (default 0.2)
      --cluster-api-burst int                          Burst to use while talking with cluster kube-apiserver. (default 60)
      --cluster-api-endpoint string                    APIEndpoint of the cluster.
      --cluster-api-qps float32                        QPS to use while talking with cluster kube-apiserver. (default 40)
      --cluster-cache-sync-timeout duration            Timeout period waiting for cluster cache to sync. (default 2m0s)
      --cluster-failure-threshold duration             The duration of failure for the cluster to be considered unhealthy. (default 30s)
      --cluster-lease-duration duration                Specifies the expiration period of a cluster lease. (default 40s)
      --cluster-lease-renew-interval-fraction float    Specifies the cluster lease renew interval fraction. (default 0.25)
      --cluster-name string                            Name of member cluster that the agent serves for.
      --cluster-namespace string                       Namespace in the control plane where member cluster secrets are stored. (default "karmada-cluster")
      --cluster-provider string                        Provider of the joining cluster. The Karmada scheduler can use this information to spread workloads across providers for higher availability.
      --cluster-region string                          The region of the joining cluster. The Karmada scheduler can use this information to spread workloads across regions for higher availability.
      --cluster-status-update-frequency duration       Specifies how often karmada-agent posts cluster status to karmada-apiserver. Note: be cautious when changing the constant, it must work with ClusterMonitorGracePeriod in karmada-controller-manager. (default 10s)
      --cluster-success-threshold duration             The duration of successes for the cluster to be considered healthy after recovery. (default 30s)
      --cluster-zones strings                          The zones of the joining cluster. The Karmada scheduler can use this information to spread workloads across zones for higher availability.
      --concurrent-cluster-syncs int                   The number of Clusters that are allowed to sync concurrently. (default 5)
      --concurrent-work-syncs int                      The number of Works that are allowed to sync concurrently. (default 5)
      --controllers strings                            A list of controllers to enable. '*' enables all on-by-default controllers, 'foo' enables the controller named 'foo', '-foo' disables the controller named 'foo'. All controllers: certRotation, clusterStatus, endpointsliceCollect, execution, serviceExport, workStatus. (default [*])
      --enable-cluster-resource-modeling               Enable means controller would build resource modeling for each cluster by syncing Nodes and Pods resources.
                                                       The resource modeling might be used by the scheduler to make scheduling decisions in scenario of dynamic replica assignment based on cluster free resources.
                                                       Disable if it does not fit your cases for better performance. (default true)
      --enable-pprof                                   Enable profiling via web interface host:port/debug/pprof/.
      --feature-gates mapStringBool                    A set of key=value pairs that describe feature gates for alpha/experimental features. Options are:
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
      --health-probe-bind-address string               The TCP address that the controller should bind to for serving health probes(e.g. 127.0.0.1:10357, :10357). It can be set to "0" to disable serving the health probe. Defaults to 0.0.0.0:10357. (default ":10357")
      --karmada-context string                         Name of the cluster context in karmada control plane kubeconfig file.
      --karmada-kubeconfig string                      Path to karmada control plane kubeconfig file.
      --karmada-kubeconfig-namespace string            Namespace of the secret containing karmada-agent certificate. This is only applicable if cert rotation is enabled. (default "karmada-system")
      --kube-api-burst int                             Burst to use while talking with karmada-apiserver. (default 60)
      --kube-api-qps float32                           QPS to use while talking with karmada-apiserver. (default 40)
      --kubeconfig string                              Paths to a kubeconfig. Only required if out-of-cluster.
      --leader-elect                                   Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability. (default true)
      --leader-elect-lease-duration duration           The duration that non-leader candidates will wait after observing a leadership renewal until attempting to acquire leadership of a led but unrenewed leader slot. This is effectively the maximum duration that a leader can be stopped before it is replaced by another candidate. This is only applicable if leader election is enabled. (default 15s)
      --leader-elect-renew-deadline duration           The interval between attempts by the acting master to renew a leadership slot before it stops leading. This must be less than or equal to the lease duration. This is only applicable if leader election is enabled. (default 10s)
      --leader-elect-resource-namespace string         The namespace of resource object that is used for locking during leader election. (default "karmada-system")
      --leader-elect-retry-period duration             The duration the clients should wait between attempting acquisition and renewal of a leadership. This is only applicable if leader election is enabled. (default 2s)
      --metrics-bind-address string                    The TCP address that the controller should bind to for serving prometheus metrics(e.g. 127.0.0.1:8080, :8080). It can be set to "0" to disable the metrics serving. (default ":8080")
      --profiling-bind-address string                  The TCP address for serving profiling(e.g. 127.0.0.1:6060, :6060). This is only applicable if profiling is enabled. (default ":6060")
      --proxy-server-address string                    Address of the proxy server that is used to proxy to the cluster.
      --rate-limiter-base-delay duration               The base delay for rate limiter. (default 5ms)
      --rate-limiter-bucket-size int                   The bucket size for rate limier. (default 100)
      --rate-limiter-max-delay duration                The max delay for rate limiter. (default 16m40s)
      --rate-limiter-qps int                           The QPS for rate limier. (default 10)
      --report-secrets strings                         The secrets that are allowed to be reported to the Karmada control plane during registering. Valid values are 'KubeCredentials', 'KubeImpersonator' and 'None'. e.g 'KubeCredentials,KubeImpersonator' or 'None'. (default [KubeCredentials,KubeImpersonator])
      --resync-period duration                         Base frequency the informers are resynced.
```

###### Auto generated by [spf13/cobra script in Karmada](https://github.com/karmada-io/karmada/tree/master/hack/tools/gencomponentdocs)