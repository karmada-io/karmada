---
title: karmadactl label
---

Update the labels on a resource

### Synopsis

Update the labels on a resource.

  *  A label key and value must begin with a letter or number, and may contain letters, numbers, hyphens, dots, and underscores, up to 63 characters each.
  *  Optionally, the key can begin with a DNS subdomain prefix and a single '/', like example.com/my-app.
  *  If --overwrite is true, then existing labels can be overwritten, otherwise attempting to overwrite a label will result in an error.
  *  If --resource-version is specified, then updates will use this resource version, otherwise the existing resource-version will be used.

```
karmadactl label [--overwrite] (-f FILENAME | TYPE NAME) KEY_1=VAL_1 ... KEY_N=VAL_N [--resource-version=version]
```

### Examples

```
  # Update deployment 'foo' with the label 'resourcetemplate.karmada.io/deletion-protected' and the value 'Always'
  karmadactl label deployment foo resourcetemplate.karmada.io/deletion-protected=Always
  
  # Update deployment 'foo' with the label 'resourcetemplate.karmada.io/deletion-protected' and the value '', overwriting any existing value
  karmadactl label --overwrite deployment foo resourcetemplate.karmada.io/deletion-protected=
  
  # Update all deployments in the namespace
  karmadactl label deployments --all resourcetemplate.karmada.io/deletion-protected=
  
  # Update a deployment identified by the type and name in "deployment.json"
  karmadactl label -f deployment.json resourcetemplate.karmada.io/deletion-protected=
  
  # Update deployment 'foo' only if the resource is unchanged from version 1
  karmadactl label deployment foo resourcetemplate.karmada.io/deletion-protected=Always --resource-version=1
  
  # Update deployment 'foo' by removing a label named 'bar' if it exists
  # Does not require the --overwrite flag
  karmadactl label deployment foo bar-
```

### Options

```
      --all                            Select all resources, in the namespace of the specified resource types
  -A, --all-namespaces                 If true, check the specified action in all namespaces.
      --allow-missing-template-keys    If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats. (default true)
      --dry-run string[="unchanged"]   Must be "none", "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource. (default "none")
      --field-manager string           Name of the manager used to track field ownership. (default "kubectl-label")
      --field-selector string          Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.
  -f, --filename strings               Filename, directory, or URL to files identifying the resource to update the labels
  -h, --help                           help for label
      --karmada-context string         The name of the kubeconfig context to use
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
  -k, --kustomize string               Process the kustomization directory. This flag can't be used together with -f or -R.
      --list                           If true, display the labels for a given resource.
      --local                          If true, label will NOT contact api-server but run locally.
  -n, --namespace string               If present, the namespace scope for this CLI request.
  -o, --output string                  Output format. One of: (json, yaml, kyaml, name, go-template, go-template-file, template, templatefile, jsonpath, jsonpath-as-json, jsonpath-file).
      --overwrite                      If true, allow labels to be overwritten, otherwise reject label updates that overwrite existing labels.
  -R, --recursive                      Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
      --resource-version string        If non-empty, the labels update will only succeed if this is the current resource-version for the object. Only valid when specifying a single resource.
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