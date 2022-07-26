## karmadactl apply

Apply a configuration to a resource by file name or stdin and propagate them into member clusters

### Synopsis

Apply a configuration to a resource by file name or stdin and propagate them into member clusters. The resource name must be specified. This resource will be created if it doesn't exist yet. To use 'apply', always create the resource initially with either 'apply' or 'create --save-config'.

 JSON and YAML formats are accepted.

 Alpha Disclaimer: the --prune functionality is not yet complete. Do not use unless you are aware of what the current state is. See https://issues.k8s.io/34274.

 Note: It implements the function of 'kubectl apply' by default. If you want to propagate them into member clusters, please use 'kubectl apply --all-clusters'.

```
karmadactl apply (-f FILENAME | -k DIRECTORY) [flags]
```

### Examples

```
  # Apply the configuration without propagation into member clusters. It acts as 'kubectl apply'.
  karmadactl apply -f manifest.yaml
  
  # Apply the configuration with propagation into specific member clusters.
  karmadactl apply -f manifest.yaml --cluster member1,member2
  
  # Apply resources from a directory and propagate them into all member clusters.
  karmadactl apply -f dir/ --all-clusters
```

### Options

```
      --all                             Select all resources in the namespace of the specified resource types.
      --all-clusters                    If present, propagates a group of resources to all member clusters.
      --allow-missing-template-keys     If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats. (default true)
      --cascade string[="background"]   Must be "background", "orphan", or "foreground". Selects the deletion cascading strategy for the dependents (e.g. Pods created by a ReplicationController). Defaults to background. (default "background")
  -C, --cluster strings                 If present, propagates a group of resources to specified clusters.
      --dry-run string[="unchanged"]    Must be "none", "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource. (default "none")
      --field-manager string            Name of the manager used to track field ownership. (default "kubectl-client-side-apply")
  -f, --filename strings                that contains the configuration to apply
      --force                           If true, immediately remove resources from API and bypass graceful deletion. Note that immediate deletion of some resources may result in inconsistency or data loss and requires confirmation.
      --force-conflicts                 If true, server-side apply will force the changes against conflicts.
      --grace-period int                Period of time in seconds given to the resource to terminate gracefully. Ignored if negative. Set to 1 for immediate shutdown. Can only be set to 0 when --force is true (force deletion). (default -1)
  -h, --help                            help for apply
      --karmada-context string          Name of the cluster context in control plane kubeconfig file.
  -k, --kustomize string                Process a kustomization directory. This flag can't be used together with -f or -R.
  -n, --namespace string                If present, the namespace scope for this CLI request
      --openapi-patch                   If true, use openapi to calculate diff when the openapi presents and the resource can be found in the openapi spec. Otherwise, fall back to use baked-in types. (default true)
  -o, --output string                   Output format. One of: (json, yaml, name, go-template, go-template-file, template, templatefile, jsonpath, jsonpath-as-json, jsonpath-file).
      --overwrite                       Automatically resolve conflicts between the modified and live configuration by using values from the modified configuration (default true)
      --prune                           Automatically delete resource objects, that do not appear in the configs and are created by either apply or create --save-config. Should be used with either -l or --all.
      --prune-whitelist stringArray     Overwrite the default whitelist with <group/version/kind> for --prune
  -R, --recursive                       Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
  -l, --selector string                 Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.
      --server-side                     If true, apply runs in the server instead of the client.
      --show-managed-fields             If true, keep the managedFields when printing objects in JSON or YAML format.
      --template string                 Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
      --timeout duration                The length of time to wait before giving up on a delete, zero means determine a timeout from the size of the object
      --validate string                 Must be one of: strict (or true), warn, ignore (or false).
                                        		"true" or "strict" will use a schema to validate the input and fail the request if invalid. It will perform server side validation if ServerSideFieldValidation is enabled on the api-server, but will fall back to less reliable client-side validation if not.
                                        		"warn" will warn about unknown or duplicate fields without blocking the request if server-side field validation is enabled on the API server, and behave as "ignore" otherwise.
                                        		"false" or "ignore" will not perform any schema validation, silently dropping any unknown or duplicate fields. (default "strict")
      --wait                            If true, wait for resources to be gone before returning. This waits for finalizers.
```

### Options inherited from parent commands

```
      --add-dir-header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files
      --kubeconfig string                Paths to a kubeconfig. Only required if out-of-cluster.
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory
      --log-file string                  If non-empty, use this log file
      --log-file-max-size uint           Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --one-output                       If true, only write logs to their native severity level (vs also writing to each lower severity level)
      --skip-headers                     If true, avoid header prefixes in the log messages
      --skip-log-headers                 If true, avoid headers when opening log files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [karmadactl](karmadactl.md)	 - karmadactl controls a Kubernetes Cluster Federation.

###### Auto generated by spf13/cobra on 26-Jul-2022
