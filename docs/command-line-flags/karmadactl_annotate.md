---
title: karmadactl annotate
---

Update the annotations on a resource

### Synopsis

Update the annotations on one or more resources.

 All Kubernetes objects support the ability to store additional data with the object as annotations. Annotations are key/value pairs that can be larger than labels and include arbitrary string values such as structured JSON. Tools and system extensions may use annotations to store their own data.

 Attempting to set an annotation that already exists will fail unless --overwrite is set. If --resource-version is specified and does not match the current resource version on the server the command will fail.

Use "karmadactl api-resources" for a complete list of supported resources.

```
karmadactl annotate [--overwrite] (-f FILENAME | TYPE NAME) KEY_1=VAL_1 ... KEY_N=VAL_N [--resource-version=version]
```

### Examples

```
  # Update deployment 'foo' with the annotation 'work.karmada.io/conflict-resolution' and the value 'overwrite'
  # If the same annotation is set multiple times, only the last value will be applied
  karmadactl annotate deployment foo work.karmada.io/conflict-resolution='overwrite'
  
  # Update a deployment identified by type and name in "deployment.json"
  karmadactl annotate -f deployment.json work.karmada.io/conflict-resolution='overwrite'
  
  # Update deployment 'foo' with the annotation 'work.karmada.io/conflict-resolution' and the value 'abort', overwriting any existing value
  karmadactl annotate --overwrite deployment foo work.karmada.io/conflict-resolution='abort'
  
  # Update all deployments in the namespace
  karmadactl annotate deployment --all work.karmada.io/conflict-resolution='abort'
  
  # Update deployment 'foo' only if the resource is unchanged from version 1
  karmadactl annotate deployment foo work.karmada.io/conflict-resolution='abort' --resource-version=1
  
  # Update deployment 'foo' by removing an annotation named 'work.karmada.io/conflict-resolution' if it exists
  # Does not require the --overwrite flag
  karmadactl annotate deployment foo work.karmada.io/conflict-resolution-
```

### Options

```
      --all                            Select all resources, in the namespace of the specified resource types.
  -A, --all-namespaces                 If true, check the specified action in all namespaces.
      --allow-missing-template-keys    If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats. (default true)
      --dry-run string[="unchanged"]   Must be "none", "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource. (default "none")
      --field-manager string           Name of the manager used to track field ownership. (default "kubectl-annotate")
      --field-selector string          Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.
  -f, --filename strings               Filename, directory, or URL to files identifying the resource to update the annotation
  -h, --help                           help for annotate
      --karmada-context string         The name of the kubeconfig context to use
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
  -k, --kustomize string               Process the kustomization directory. This flag can't be used together with -f or -R.
      --list                           If true, display the annotations for a given resource.
      --local                          If true, annotation will NOT contact api-server but run locally.
  -n, --namespace string               If present, the namespace scope for this CLI request.
  -o, --output string                  Output format. One of: (json, yaml, kyaml, name, go-template, go-template-file, template, templatefile, jsonpath, jsonpath-as-json, jsonpath-file).
      --overwrite                      If true, allow annotations to be overwritten, otherwise reject annotation updates that overwrite existing annotations.
  -R, --recursive                      Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
      --resource-version string        If non-empty, the annotation update will only succeed if this is the current resource-version for the object. Only valid when specifying a single resource.
  -l, --selector string                Selector (label query) to filter on, supports '=', '==', '!=', 'in', 'notin'.(e.g. -l key1=value1,key2=value2,key3 in (value3)). Matching objects must satisfy all of the specified label constraints.
      --show-managed-fields            If true, keep the managedFields when printing objects in JSON or YAML format.
      --template string                Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
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