---
title: karmadactl
---

karmadactl controls a Kubernetes Cluster Federation.

### Synopsis

karmadactl controls a Kubernetes Cluster Federation.

```
karmadactl [flags]
```

### Options

```
      --add-dir-header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files (no effect when -logtostderr=true)
  -h, --help                             help for karmadactl
      --kubeconfig string                Paths to a kubeconfig. Only required if out-of-cluster.
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
* [karmadactl annotate](karmadactl_annotate.md)	 - Update the annotations on a resource
* [karmadactl api-resources](karmadactl_api-resources.md)	 - Print the supported API resources on the server
* [karmadactl api-versions](karmadactl_api-versions.md)	 - Print the supported API versions on the server, in the form of "group/version"
* [karmadactl apply](karmadactl_apply.md)	 - Apply a configuration to a resource by file name or stdin and propagate them into member clusters
* [karmadactl attach](karmadactl_attach.md)	 - Attach to a running container
* [karmadactl completion](karmadactl_completion.md)	 - Output shell completion code for the specified shell (bash, zsh, fish)
* [karmadactl cordon](karmadactl_cordon.md)	 - Mark cluster as unschedulable
* [karmadactl create](karmadactl_create.md)	 - Create a resource from a file or from stdin
* [karmadactl deinit](karmadactl_deinit.md)	 - Remove the Karmada control plane from the Kubernetes cluster.
* [karmadactl delete](karmadactl_delete.md)	 - Delete resources by file names, stdin, resources and names, or by resources and label selector
* [karmadactl describe](karmadactl_describe.md)	 - Show details of a specific resource or group of resources in Karmada control plane or a member cluster
* [karmadactl edit](karmadactl_edit.md)	 - Edit a resource on the server
* [karmadactl exec](karmadactl_exec.md)	 - Execute a command in a container in a cluster
* [karmadactl explain](karmadactl_explain.md)	 - Get documentation for a resource
* [karmadactl get](karmadactl_get.md)	 - Display one or many resources in Karmada control plane and member clusters.
* [karmadactl init](karmadactl_init.md)	 - Install the Karmada control plane in a Kubernetes cluster
* [karmadactl interpret](karmadactl_interpret.md)	 - Validate, test and edit interpreter customization before applying it to the control plane
* [karmadactl join](karmadactl_join.md)	 - Register a cluster to Karmada control plane with Push mode
* [karmadactl label](karmadactl_label.md)	 - Update the labels on a resource
* [karmadactl logs](karmadactl_logs.md)	 - Print the logs for a container in a pod in a cluster
* [karmadactl options](karmadactl_options.md)	 - Print the list of flags inherited by all commands
* [karmadactl patch](karmadactl_patch.md)	 - Update fields of a resource
* [karmadactl promote](karmadactl_promote.md)	 - Promote resources from legacy clusters to Karmada control plane
* [karmadactl register](karmadactl_register.md)	 - Register a cluster to Karmada control plane with Pull mode
* [karmadactl taint](karmadactl_taint.md)	 - Update the taints on one or more clusters
* [karmadactl token](karmadactl_token.md)	 - Manage bootstrap tokens for joining member clusters to Karmada
* [karmadactl top](karmadactl_top.md)	 - Display resource (CPU/memory) usage of member clusters
* [karmadactl uncordon](karmadactl_uncordon.md)	 - Mark cluster as schedulable
* [karmadactl unjoin](karmadactl_unjoin.md)	 - Remove a cluster from Karmada control plane
* [karmadactl unregister](karmadactl_unregister.md)	 - Remove a pull mode cluster from Karmada control plane
* [karmadactl version](karmadactl_version.md)	 - Print the version information

#### Go Back to [Karmadactl Commands](karmadactl_index.md) Homepage.


###### Auto generated by [spf13/cobra script in Karmada](https://github.com/karmada-io/karmada/tree/master/hack/tools/genkarmadactldocs).