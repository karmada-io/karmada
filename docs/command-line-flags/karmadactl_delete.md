---
title: karmadactl delete
---

Delete resources by file names, stdin, resources and names, or by resources and label selector

### Synopsis

Delete resources by file names, stdin, resources and names, or by resources and label selector.

 JSON and YAML formats are accepted. Only one type of argument may be specified: file names, resources and names, or resources and label selector.

 Some resources, such as pods, support graceful deletion. These resources define a default period before they are forcibly terminated (the grace period) but you may override that value with the --grace-period flag, or pass --now to set a grace-period of 1. Because these resources often represent entities in the cluster, deletion may not be acknowledged immediately. If the node hosting a pod is down or cannot reach the API server, termination may take significantly longer than the grace period. To force delete a resource, you must specify the --force flag. Note: only a subset of resources support graceful deletion. In absence of the support, the --grace-period flag is ignored.

 IMPORTANT: Force deleting pods does not wait for confirmation that the pod's processes have been terminated, which can leave those processes running until the node detects the deletion and completes graceful deletion. If your processes use shared storage or talk to a remote API and depend on the name of the pod to identify themselves, force deleting those pods may result in multiple processes running on different machines using the same identification which may lead to data corruption or inconsistency. Only force delete pods when you are sure the pod is terminated, or if your application can tolerate multiple copies of the same pod running at once. Also, if you force delete pods, the scheduler may place new pods on those nodes before the node has released those resources and causing those pods to be evicted immediately.

 Note that the delete command does NOT do resource version checks, so if someone submits an update to a resource right when you submit a delete, their update will be lost along with the rest of the resource.

 After a CustomResourceDefinition is deleted, invalidation of discovery cache may take up to 6 hours. If you don't want to wait, you might want to run "karmadactl api-resources" to refresh the discovery cache.

```
karmadactl delete ([-f FILENAME] | [-k DIRECTORY] | TYPE [(NAME | -l label | --all)])
```

### Examples

```
  # Delete a propagationpolicy using the type and name specified in propagationpolicy.json
  karmadactl delete -f ./propagationpolicy.json
  
  # Delete resources from a directory containing kustomization.yaml - e.g. dir/kustomization.yaml
  karmadactl delete -k dir
  
  # Delete resources from all files that end with '.json'
  karmadactl delete -f '*.json'
  
  # Delete a propagationpolicy based on the type and name in the JSON passed into stdin
  cat propagationpolicy.json | karmadactl delete -f -
  
  # Delete propagationpolicies and services with same names "baz" and "foo"
  karmadactl delete propagationpolicy,service baz foo
  
  # Delete propagationpolicies and services with label name=myLabel
  karmadactl delete propagationpolicies,services -l name=myLabel
  
  # Delete a propagationpolicy with minimal delay
  karmadactl delete propagationpolicy foo --now
  
  # Force delete a propagationpolicy on a dead node
  karmadactl delete propagationpolicy foo --force
  
  # Delete all propagationpolicies
  karmadactl delete propagationpolicies --all
```

### Options

```
      --all                             Delete all resources, in the namespace of the specified resource types.
  -A, --all-namespaces                  If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.
      --cascade string[="background"]   Must be "background", "orphan", or "foreground". Selects the deletion cascading strategy for the dependents (e.g. Pods created by a ReplicationController). Defaults to background. (default "background")
      --dry-run string[="unchanged"]    Must be "none", "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource. (default "none")
      --field-selector string           Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.
  -f, --filename strings                containing the resource to delete.
      --force                           If true, immediately remove resources from API and bypass graceful deletion. Note that immediate deletion of some resources may result in inconsistency or data loss and requires confirmation.
      --grace-period int                Period of time in seconds given to the resource to terminate gracefully. Ignored if negative. Set to 1 for immediate shutdown. Can only be set to 0 when --force is true (force deletion). (default -1)
  -h, --help                            help for delete
      --ignore-not-found                Treat "resource not found" as a successful delete. Defaults to "true" when --all is specified.
  -i, --interactive                     If true, delete resource only when user confirms.
      --karmada-context string          The name of the kubeconfig context to use
      --kubeconfig string               Path to the kubeconfig file to use for CLI requests.
  -k, --kustomize string                Process a kustomization directory. This flag can't be used together with -f or -R.
  -n, --namespace string                If present, the namespace scope for this CLI request.
      --now                             If true, resources are signaled for immediate shutdown (same as --grace-period=1).
  -o, --output string                   Output mode. Use "-o name" for shorter output (resource/name).
      --raw string                      Raw URI to DELETE to the server.  Uses the transport specified by the kubeconfig file.
  -R, --recursive                       Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
  -l, --selector string                 Selector (label query) to filter on, supports '=', '==', '!=', 'in', 'notin'.(e.g. -l key1=value1,key2=value2,key3 in (value3)). Matching objects must satisfy all of the specified label constraints.
      --timeout duration                The length of time to wait before giving up on a delete, zero means determine a timeout from the size of the object
      --wait                            If true, wait for resources to be gone before returning. This waits for finalizers. (default true)
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