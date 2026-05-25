---
title: karmadactl logs
---

Print the logs for a container in a pod in a cluster

### Synopsis

Print the logs for a container in a pod in a member cluster or specified resource. If the pod has only one container, the container name is optional.

```
karmadactl logs [-f] [-p] (POD | TYPE/NAME) [-c CONTAINER] (-C CLUSTER)
```

### Examples

```
  # Return snapshot logs from pod nginx with only one container in cluster(member1)
  karmadactl logs nginx -C=member1
  
  # Return snapshot logs from pod nginx with multi containers in cluster(member1)
  karmadactl logs nginx --all-containers=true -C=member1
  
  # Return snapshot logs from all containers in pods defined by label app=nginx in cluster(member1)
  karmadactl logs -l app=nginx --all-containers=true -C=member1
  
  # Return snapshot of previous terminated ruby container logs from pod web-1 in cluster(member1)
  karmadactl logs -p -c ruby web-1 -C=member1
  
  # Begin streaming the logs of the ruby container in pod web-1 in cluster(member1)
  karmadactl logs -f -c ruby web-1 -C=member1
  
  # Begin streaming the logs from all containers in pods defined by label app=nginx in cluster(member1)
  karmadactl logs -f -l app=nginx --all-containers=true -C=member1
  
  # Display only the most recent 20 lines of output in pod nginx in cluster(member1)
  karmadactl logs --tail=20 nginx -C=member1
  
  # Show all logs from pod nginx written in the last hour in cluster(member1)
  karmadactl logs --since=1h nginx -C=member1
```

### Options

```
      --all-containers                     Get all containers' logs in the pod(s).
      --all-pods                           Get logs from all pod(s). Sets prefix to true.
  -C, --cluster string                     Specify a member cluster
  -c, --container string                   Print the logs of this container
  -f, --follow                             Specify if the logs should be streamed.
  -h, --help                               help for logs
      --ignore-errors                      If watching / following pod logs, allow for any errors that occur to be non-fatal
      --insecure-skip-tls-verify-backend   Skip verifying the identity of the kubelet that logs are requested from.  In theory, an attacker could provide invalid log content back. You might want to use this if your kubelet serving certificates have expired.
      --karmada-context string             The name of the kubeconfig context to use
      --kubeconfig string                  Path to the kubeconfig file to use for CLI requests.
      --limit-bytes int                    Maximum bytes of logs to return. Defaults to no limit.
      --max-log-requests int               Specify maximum number of concurrent logs to follow when using by a selector. Defaults to 5. (default 5)
  -n, --namespace string                   If present, the namespace scope for this CLI request.
      --pod-running-timeout duration       The length of time (like 5s, 2m, or 3h, higher than zero) to wait until at least one pod is running (default 20s)
      --prefix                             Prefix each log line with the log source (pod name and container name)
  -p, --previous                           If true, print the logs for the previous instance of the container in a pod if it exists.
  -l, --selector string                    Selector (label query) to filter on, supports '=', '==', '!=', 'in', 'notin'.(e.g. -l key1=value1,key2=value2,key3 in (value3)). Matching objects must satisfy all of the specified label constraints.
      --since duration                     Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs. Only one of since-time / since may be used.
      --since-time string                  Only return logs after a specific date (RFC3339). Defaults to all logs. Only one of since-time / since may be used.
      --tail int                           Lines of recent log file to display. Defaults to -1 with no selector, showing all log lines otherwise 10, if a selector is provided. (default -1)
      --timestamps                         Include timestamps on each line in the log output
```

### Options inherited from parent commands

```
      --add-dir-header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files (no effect when -logtostderr=true)
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log-file string                  If non-empty, use this log file (no effect when -logtostderr=true)
      --log-file-max-size uint           Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --one-output                       If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --skip-headers                     If true, avoid header prefixes in the log messages
      --skip-log-headers                 If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity         logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true) (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [karmadactl](karmadactl.md)	 - karmadactl controls a Kubernetes Cluster Federation.

#### Go Back to [Karmadactl Commands](karmadactl_index.md) Homepage.


###### Auto generated by [spf13/cobra script in Karmada](https://github.com/karmada-io/karmada/tree/master/hack/tools/genkarmadactldocs).