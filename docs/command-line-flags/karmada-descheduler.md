---
title: karmada-descheduler
---



### Synopsis

The karmada-descheduler evicts replicas from member clusters
if they are failed to be scheduled for a period of time. It relies on
karmada-scheduler-estimator to get replica status.

```
karmada-descheduler [flags]
```

### Options

```
Available Commands:
  karmada-descheduler completion                      Generate the autocompletion script for the specified shell
  karmada-descheduler help                            Help about any command
  karmada-descheduler version                         Print the version information

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

      --descheduling-interval duration                 Time interval between two consecutive descheduler executions. Setting this value instructs the descheduler to run in a continuous loop at the interval specified. (default 2m0s)
      --enable-pprof                                   Enable profiling via web interface host:port/debug/pprof/.
      --health-probe-bind-address string               The TCP address that the server should bind to for serving health probes(e.g. 127.0.0.1:10358, :10358). It can be set to "0" to disable serving the health probe. Defaults to 0.0.0.0:10358. (default ":10358")
      --insecure-skip-estimator-verify                 Controls whether verifies the scheduler estimator's certificate chain and host name.
      --kube-api-burst int                             Burst to use while talking with karmada-apiserver. (default 60)
      --kube-api-qps float32                           QPS to use while talking with karmada-apiserver. (default 40)
      --kubeconfig string                              Path to karmada control plane kubeconfig file.
      --leader-elect                                   Enable leader election, which must be true when running multi instances. (default true)
      --leader-elect-lease-duration duration           The duration that non-leader candidates will wait after observing a leadership renewal until attempting to acquire leadership of a led but unrenewed leader slot. This is effectively the maximum duration that a leader can be stopped before it is replaced by another candidate. This is only applicable if leader election is enabled. (default 15s)
      --leader-elect-renew-deadline duration           The interval between attempts by the acting master to renew a leadership slot before it stops leading. This must be less than or equal to the lease duration. This is only applicable if leader election is enabled. (default 10s)
      --leader-elect-resource-namespace string         The namespace of resource object that is used for locking during leader election. (default "karmada-system")
      --leader-elect-retry-period duration             The duration the clients should wait between attempting acquisition and renewal of a leadership. This is only applicable if leader election is enabled. (default 2s)
      --master string                                  The address of the Kubernetes API server. Overrides any value in KubeConfig. Only required if out-of-cluster.
      --metrics-bind-address string                    The TCP address that the server should bind to for serving prometheus metrics(e.g. 127.0.0.1:8080, :8080). It can be set to "0" to disable the metrics serving. Defaults to 0.0.0.0:8080. (default ":8080")
      --profiling-bind-address string                  The TCP address for serving profiling(e.g. 127.0.0.1:6060, :6060). This is only applicable if profiling is enabled. (default ":6060")
      --scheduler-estimator-ca-file string             SSL Certificate Authority file used to secure scheduler estimator communication.
      --scheduler-estimator-cert-file string           SSL certification file used to secure scheduler estimator communication.
      --scheduler-estimator-key-file string            SSL key file used to secure scheduler estimator communication.
      --scheduler-estimator-port int                   The secure port on which to connect the accurate scheduler estimator. (default 10352)
      --scheduler-estimator-service-namespace string   The namespace to be used for discovering scheduler estimator services. (default "karmada-system")
      --scheduler-estimator-service-prefix string      The prefix of scheduler estimator service name (default "karmada-scheduler-estimator")
      --scheduler-estimator-timeout duration           Specifies the timeout period of calling the scheduler estimator service. (default 3s)
      --unschedulable-threshold duration               The period of pod unschedulable condition. This value is considered as a classification standard of unschedulable replicas. (default 5m0s)
```

###### Auto generated by [spf13/cobra script in Karmada](https://github.com/karmada-io/karmada/tree/master/hack/tools/gencomponentdocs)