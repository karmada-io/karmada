## karmadactl get

Display one or many resources

```
karmadactl get [NAME | -l label | -n namespace]  [flags]
```

### Examples

```
  # List all pods in ps output format
  karmadactl get pods
  
  # List all pods in ps output format with more information (such as node name)
  karmadactl get pods -o wide
  
  # List all pods of member1 cluster in ps output format
  karmadactl get pods -C member1
  
  # List a single replicasets controller with specified NAME in ps output format
  karmadactl get replicasets nginx
  
  # List deployments in JSON output format, in the "v1" version of the "apps" API group
  karmadactl get deployments.v1.apps -o json
  
  # Return only the phase value of the specified resource
  karmadactl get -o template deployment/nginx -C member1 --template={{.spec.replicas}}
  
  # List all replication controllers and services together in ps output format
  karmadactl get rs,services
  
  # List one or more resources by their type and names
  karmadactl get rs/nginx-cb87b6d88 service/kubernetes
```

### Options

```
  -A, --all-namespaces                If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.
      --allow-missing-template-keys   If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats. (default true)
  -C, --clusters strings              -C=member1,member2
  -h, --help                          help for get
      --ignore-not-found              If the requested object does not exist the command will return exit code 0.
      --karmada-context string        Name of the cluster context in control plane kubeconfig file.
  -L, --label-columns strings         Accepts a comma separated list of labels that are going to be presented as columns. Names are case-sensitive. You can also use multiple flag options like -L label1 -L label2...
  -l, --labels string                 -l=label or -l label
  -n, --namespace string              -n=namespace or -n namespace
      --no-headers                    When using the default or custom-column output format, don't print headers (default print headers).
  -o, --output string                 Output format. One of: (json, yaml, name, go-template, go-template-file, template, templatefile, jsonpath, jsonpath-as-json, jsonpath-file, custom-columns, custom-columns-file, wide). See custom columns [https://kubernetes.io/docs/reference/kubectl/overview/#custom-columns], golang template [http://golang.org/pkg/text/template/#pkg-overview] and jsonpath template [https://kubernetes.io/docs/reference/kubectl/jsonpath/].
      --output-watch-events           Output watch event objects when --watch or --watch-only is used. Existing objects are output as initial ADDED events.
      --show-kind                     If present, list the resource type for the requested object(s).
      --show-labels                   When printing, show all labels as the last column (default hide labels column)
      --show-managed-fields           If true, keep the managedFields when printing objects in JSON or YAML format.
      --sort-by string                If non-empty, sort list types using this field specification.  The field specification is expressed as a JSONPath expression (e.g. '{.metadata.name}'). The field in the API resource specified by this JSONPath expression must be an integer or a string.
      --template string               Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
  -w, --watch                         After listing/getting the requested object, watch for changes. Uninitialized objects are excluded if no object name is provided.
      --watch-only                    Watch for changes to the requested object(s), without listing/getting first.
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
