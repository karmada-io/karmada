---
title: karmadactl addons enable
---

Enable Karmada addons from Kubernetes

### Synopsis

Enable Karmada addons from Kubernetes

```
karmadactl addons enable
```

### Examples

```
  # Enable Karmada all addons except karmada-scheduler-estimator to Kubernetes cluster
  karmadactl addons enable all
  
  # Enable Karmada search to the Kubernetes cluster
  karmadactl addons enable karmada-search
  
  # Enable karmada search and descheduler to the kubernetes cluster
  karmadactl addons enable karmada-search karmada-descheduler
  
  # Enable karmada search and scheduler-estimator of cluster member1 to the kubernetes cluster
  karmadactl addons enable karmada-search karmada-scheduler-estimator -C member1 --member-kubeconfig /etc/karmada/member.config --member-context member1
  
  # Specify the host cluster kubeconfig
  karmadactl addons enable karmada-search --kubeconfig /root/.kube/config
  
  # Specify the Karmada control plane kubeconfig
  karmadactl addons enable karmada-search --karmada-kubeconfig /etc/karmada/karmada-apiserver.config
  
  # Specify the karmada-search image
  karmadactl addons enable karmada-search --karmada-search-image docker.io/karmada/karmada-search:latest
  
  # Specify the namespace where Karmada components are installed
  karmadactl addons enable karmada-search --namespace karmada-system
```

### Options

```
      --apiservice-timeout int                     Wait apiservice ready timeout. (default 30)
  -C, --cluster string                             Name of the member cluster that enables or disables the scheduler estimator.
      --context string                             The name of the kubeconfig context to use.
      --descheduler-priority-class string          The priority class name for the component karmada-descheduler. (default "system-node-critical")
      --estimator-priority-class string            The priority class name for the component karmada-scheduler-estimator. (default "system-node-critical")
  -h, --help                                       help for enable
      --host-cluster-domain string                 The cluster domain of karmada host cluster. (e.g. --host-cluster-domain=host.karmada) (default "cluster.local")
      --karmada-context string                     The name of the karmada control plane kubeconfig context to use.
      --karmada-descheduler-image string           karmada-descheduler image (default "docker.io/karmada/karmada-descheduler:v0.0.0-master")
      --karmada-descheduler-replicas int32         karmada descheduler replica set (default 1)
      --karmada-estimator-replicas int32           karmada-scheduler-estimator replica set (default 1)
      --karmada-kubeconfig string                  Path to the karmada control plane kubeconfig file. (default "/etc/karmada/karmada-apiserver.config")
      --karmada-metrics-adapter-image string       karmada-metrics-adapter image (default "docker.io/karmada/karmada-metrics-adapter:v0.0.0-master")
      --karmada-metrics-adapter-replicas int32     karmada-metrics-adapter replica set (default 1)
      --karmada-scheduler-estimator-image string   karmada-scheduler-estimator image (default "docker.io/karmada/karmada-scheduler-estimator:v0.0.0-master")
      --karmada-search-image string                karmada-search image (default "docker.io/karmada/karmada-search:v0.0.0-master")
      --karmada-search-replicas int32              Karmada-search replica set (default 1)
      --kubeconfig string                          Path to the host cluster kubeconfig file.
      --member-context string                      Member cluster's context which to deploy scheduler estimator
      --member-kubeconfig string                   Member cluster's kubeconfig which to deploy scheduler estimator
      --metrics-adapter-priority-class string      The priority class name for the component karmada-metrics-adaptor. (default "system-node-critical")
  -n, --namespace string                           namespace where Karmada components are installed. (default "karmada-system")
      --pod-timeout int                            Wait pod ready timeout. (default 120)
      --private-image-registry string              Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.
      --search-priority-class string               The priority class name for the component karmada-search. (default "system-node-critical")
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

* [karmadactl addons](karmadactl_addons.md)	 - Enable or disable a Karmada addon

#### Go Back to [Karmadactl Commands](karmadactl_index.md) Homepage.


###### Auto generated by [spf13/cobra script in Karmada](https://github.com/karmada-io/karmada/tree/master/hack/tools/genkarmadactldocs).