---
title: karmadactl create
---

Create a resource from a file or from stdin

### Synopsis

Create a resource from a file or from stdin.

 JSON and YAML formats are accepted.

```
karmadactl create -f FILENAME
```

### Examples

```
  # Create a pod using the data in pod.json
  karmadactl create -f ./pod.json
  
  # Create a pod based on the JSON passed into stdin
  cat pod.json | karmadactl create -f -
  
  # Edit the data in registry.yaml in JSON then create the resource using the edited data
  karmadactl create -f registry.yaml --edit -o json
```

### Options

```
      --allow-missing-template-keys    If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats. (default true)
      --dry-run string[="unchanged"]   Must be "none", "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource. (default "none")
      --edit                           Edit the API resource before creating
      --field-manager string           Name of the manager used to track field ownership. (default "kubectl-create")
  -f, --filename strings               Filename, directory, or URL to files to use to create the resource
  -h, --help                           help for create
      --karmada-context string         The name of the kubeconfig context to use
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
  -k, --kustomize string               Process the kustomization directory. This flag can't be used together with -f or -R.
  -n, --namespace string               If present, the namespace scope for this CLI request.
  -o, --output string                  Output format. One of: (json, yaml, kyaml, name, go-template, go-template-file, template, templatefile, jsonpath, jsonpath-as-json, jsonpath-file).
      --raw string                     Raw URI to POST to the server.  Uses the transport specified by the kubeconfig file.
  -R, --recursive                      Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
      --save-config                    If true, the configuration of current object will be saved in its annotation. Otherwise, the annotation will be unchanged. This flag is useful when you want to perform kubectl apply on this object in the future.
  -l, --selector string                Selector (label query) to filter on, supports '=', '==', '!=', 'in', 'notin'.(e.g. -l key1=value1,key2=value2,key3 in (value3)). Matching objects must satisfy all of the specified label constraints.
      --show-managed-fields            If true, keep the managedFields when printing objects in JSON or YAML format.
      --template string                Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
      --validate string[="strict"]     Must be one of: strict (or true), warn, ignore (or false). "true" or "strict" will use a schema to validate the input and fail the request if invalid. It will perform server side validation if ServerSideFieldValidation is enabled on the api-server, but will fall back to less reliable client-side validation if not. "warn" will warn about unknown or duplicate fields without blocking the request if server-side field validation is enabled on the API server, and behave as "ignore" otherwise. "false" or "ignore" will not perform any schema validation, silently dropping any unknown or duplicate fields. (default "strict")
      --windows-line-endings           Only relevant if --edit=true. Defaults to the line ending native to your platform.
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
* [karmadactl create clusterrole](karmadactl_create_clusterrole.md)	 - Create a cluster role
* [karmadactl create clusterrolebinding](karmadactl_create_clusterrolebinding.md)	 - Create a cluster role binding for a particular cluster role
* [karmadactl create configmap](karmadactl_create_configmap.md)	 - Create a config map from a local file, directory or literal value
* [karmadactl create cronjob](karmadactl_create_cronjob.md)	 - Create a cron job with the specified name
* [karmadactl create deployment](karmadactl_create_deployment.md)	 - Create a deployment with the specified name
* [karmadactl create ingress](karmadactl_create_ingress.md)	 - Create an ingress with the specified name
* [karmadactl create job](karmadactl_create_job.md)	 - Create a job with the specified name
* [karmadactl create namespace](karmadactl_create_namespace.md)	 - Create a namespace with the specified name
* [karmadactl create poddisruptionbudget](karmadactl_create_poddisruptionbudget.md)	 - Create a pod disruption budget with the specified name
* [karmadactl create priorityclass](karmadactl_create_priorityclass.md)	 - Create a priority class with the specified name
* [karmadactl create quota](karmadactl_create_quota.md)	 - Create a quota with the specified name
* [karmadactl create role](karmadactl_create_role.md)	 - Create a role with single rule
* [karmadactl create rolebinding](karmadactl_create_rolebinding.md)	 - Create a role binding for a particular role or cluster role
* [karmadactl create secret](karmadactl_create_secret.md)	 - Create a secret using a specified subcommand
* [karmadactl create service](karmadactl_create_service.md)	 - Create a service using a specified subcommand
* [karmadactl create serviceaccount](karmadactl_create_serviceaccount.md)	 - Create a service account with the specified name
* [karmadactl create token](karmadactl_create_token.md)	 - Request a service account token

#### Go Back to [Karmadactl Commands](karmadactl_index.md) Homepage.


###### Auto generated by [spf13/cobra script in Karmada](https://github.com/karmada-io/karmada/tree/master/hack/tools/genkarmadactldocs).