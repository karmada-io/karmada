---
title: karmadactl describe
---

Show details of a specific resource or group of resources in Karmada control plane or a member cluster

### Synopsis

Show details of a specific resource or group of resources in Karmada control plane or a member cluster.

 Print a detailed description of the selected resources, including related resources such as events or controllers. You may select a single object by name, all objects of that type, provide a name prefix, or label selector. For example:

 $ karmadactl describe TYPE NAME_PREFIX

 will first check for an exact match on TYPE and NAME_PREFIX. If no such resource exists, it will output details for every resource that has a name prefixed with NAME_PREFIX.

```
karmadactl describe (-f FILENAME | TYPE [NAME_PREFIX | -l label] | TYPE/NAME) (--operation-scope=SCOPE --cluster=CLUSTER)
```

### Examples

```
  # Describe a deployment in Karmada control plane
  karmadactl describe deployment/nginx
  
  # Describe a pod in cluster(member1)
  karmadactl describe pods/nginx --operation-scope=members --cluster=member1
  
  # Describe all pods in cluster(member1)
  karmadactl describe pods --operation-scope=members --cluster=member1
  
  # Describe a pod identified by type and name in "pod.json" in cluster(member1)
  karmadactl describe -f pod.json --operation-scope=members --cluster=member1
  
  # Describe pods by label name=myLabel in cluster(member1)
  karmadactl describe po -l name=myLabel --operation-scope=members --cluster=member1
  
  # Describe all pods managed by the 'frontend' replication controller in cluster(member1)
  # (rc-created pods get the name of the rc as a prefix in the pod name)
  karmadactl describe pods frontend --operation-scope=members --cluster=member1
```

### Options

```
  -A, --all-namespaces                   If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.
      --chunk-size int                   Return large lists in chunks rather than all at once. Pass 0 to disable. (default 500)
  -C, --cluster string                   Used to specify a target member cluster and only takes effect when the command's operation scope is members, for example: --operation-scope=members --cluster=member1
  -f, --filename strings                 Filename, directory, or URL to files containing the resource to describe
  -h, --help                             help for describe
      --karmada-context string           The name of the kubeconfig context to use
      --kubeconfig string                Path to the kubeconfig file to use for CLI requests.
  -k, --kustomize string                 Process the kustomization directory. This flag can't be used together with -f or -R.
  -n, --namespace string                 If present, the namespace scope for this CLI request.
  -s, --operation-scope operationScope   Used to control the operation scope of the command. The optional values are karmada and members. Defaults to karmada. (default karmada)
  -R, --recursive                        Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
  -l, --selector string                  Selector (label query) to filter on, supports '=', '==', '!=', 'in', 'notin'.(e.g. -l key1=value1,key2=value2,key3 in (value3)). Matching objects must satisfy all of the specified label constraints.
      --show-events                      If true, display events related to the described object. (default true)
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