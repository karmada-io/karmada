---
title: karmadactl interpret
---

Validate, test and edit interpreter customization before applying it to the control plane

### Synopsis

Validate, test and edit interpreter customization before applying it to the control plane.
        
  1.  Validate the ResourceInterpreterCustomization configuration as per API schema and try to load the scripts for syntax check.
  2.  Run the rules locally and test if the result is expected. Similar to the dry run.
  3.  Edit customization. Similar to the kubectl edit.

```
karmadactl interpret (-f FILENAME) (--operation OPERATION) [--ARGS VALUE]... 
```

### Examples

```
  # Check the customizations in file (only YAML and JSON formats are supported)
  karmadactl interpret -f customization.json --check
  
  # Execute the retention rule
  karmadactl interpret -f customization.yml --operation retain --desired-file desired.yml --observed-file observed.yml
  
  # Execute the replicaResource rule
  karmadactl interpret -f customization.yml --operation interpretReplica --observed-file observed.yml
  
  # Execute the componentResource rule
  karmadactl interpret -f customization.yml --operation interpretComponent --observed-file observed.yml
  
  # Execute the replicaRevision rule
  karmadactl interpret -f customization.yml --operation reviseReplica --observed-file observed.yml --desired-replica 2
  
  # Execute the statusReflection rule
  karmadactl interpret -f customization.yml --operation interpretStatus --observed-file observed.yml
  
  # Execute the healthInterpretation rule
  karmadactl interpret -f customization.yml --operation interpretHealth --observed-file observed.yml
  
  # Execute the dependencyInterpretation rule
  karmadactl interpret -f customization.yml --operation interpretDependency --observed-file observed.yml
  
  # Execute the statusAggregation rule
  karmadactl interpret -f customization.yml --operation aggregateStatus --observed-file observed.yml --status-file status.yml
  
  # Fetch observed object from url, and status items from stdin (specified with -)
  karmadactl interpret -f customization.yml --operation aggregateStatus --observed-file https://example.com/observed.yml --status-file -
  
  # Edit customization
  karmadactl interpret -f customization.yml --edit
```

### Options

```
      --allow-missing-template-keys   If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats. (default true)
      --check                         Validates the given ResourceInterpreterCustomization configuration(s)
      --desired-file string           Filename, directory, or URL to files identifying the resource to use as desiredObj argument in rule script.
      --desired-replica int32         The desiredReplica argument in rule script.
      --edit                          Edit customizations
  -f, --filename strings              Filename, directory, or URL to files containing the customizations. Files need to be in either YAML or JSON format.
  -h, --help                          help for interpret
      --karmada-context string        The name of the kubeconfig context to use
      --kubeconfig string             Path to the kubeconfig file to use for CLI requests.
      --observed-file string          Filename, directory, or URL to files identifying the resource to use as observedObj argument in rule script.
      --operation string              The interpret operation to use. One of: (Retain,InterpretReplica,InterpretComponent,ReviseReplica,InterpretStatus,AggregateStatus,InterpretHealth,InterpretDependency)
  -o, --output string                 Output format. One of: (json, yaml, kyaml, name, go-template, go-template-file, template, templatefile, jsonpath, jsonpath-as-json, jsonpath-file).
  -R, --recursive                     Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
      --show-doc                      Show document of rules when editing
      --show-managed-fields           If true, keep the managedFields when printing objects in JSON or YAML format.
      --status-file string            Filename, directory, or URL to files identifying the resource to use as statusItems argument in rule script.
      --template string               Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
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