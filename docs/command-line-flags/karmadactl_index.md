---
title: Karmadactl Commands
---


## Basic Commands

* [karmadactl create](karmadactl_create.md)	 - Create a resource from a file or from stdin.

 JSON and YAML formats are accepted.
* [karmadactl delete](karmadactl_delete.md)	 - Delete resources by file names, stdin, resources and names, or by resources and label selector.

 JSON and YAML formats are accepted. Only one type of argument may be specified: file names, resources and names, or resources and label selector.

 Some resources, such as pods, support graceful deletion. These resources define a default period before they are forcibly terminated (the grace period) but you may override that value with the --grace-period flag, or pass --now to set a grace-period of 1. Because these resources often represent entities in the cluster, deletion may not be acknowledged immediately. If the node hosting a pod is down or cannot reach the API server, termination may take significantly longer than the grace period. To force delete a resource, you must specify the --force flag. Note: only a subset of resources support graceful deletion. In absence of the support, the --grace-period flag is ignored.

 IMPORTANT: Force deleting pods does not wait for confirmation that the pod's processes have been terminated, which can leave those processes running until the node detects the deletion and completes graceful deletion. If your processes use shared storage or talk to a remote API and depend on the name of the pod to identify themselves, force deleting those pods may result in multiple processes running on different machines using the same identification which may lead to data corruption or inconsistency. Only force delete pods when you are sure the pod is terminated, or if your application can tolerate multiple copies of the same pod running at once. Also, if you force delete pods, the scheduler may place new pods on those nodes before the node has released those resources and causing those pods to be evicted immediately.

 Note that the delete command does NOT do resource version checks, so if someone submits an update to a resource right when you submit a delete, their update will be lost along with the rest of the resource.

 After a CustomResourceDefinition is deleted, invalidation of discovery cache may take up to 6 hours. If you don't want to wait, you might want to run "karmadactl api-resources" to refresh the discovery cache.
* [karmadactl edit](karmadactl_edit.md)	 - Edit a resource from the default editor.

 The edit command allows you to directly edit any API resource you can retrieve via the command-line tools. It will open the editor defined by your KUBE_EDITOR, or EDITOR environment variables, or fall back to 'vi' for Linux or 'notepad' for Windows. When attempting to open the editor, it will first attempt to use the shell that has been defined in the 'SHELL' environment variable. If this is not defined, the default shell will be used, which is '/bin/bash' for Linux or 'cmd' for Windows.

 You can edit multiple objects, although changes are applied one at a time. The command accepts file names as well as command-line arguments, although the files you point to must be previously saved versions of resources.

 Editing is done with the API version used to fetch the resource. To edit using a specific API version, fully-qualify the resource, version, and group.

 The default format is YAML. To edit in JSON, specify "-o json".

 The flag --windows-line-endings can be used to force Windows line endings, otherwise the default for your operating system will be used.

 In the event an error occurs while updating, a temporary file will be created on disk that contains your unapplied changes. The most common error when updating a resource is another editor changing the resource on the server. When this occurs, you will have to apply your changes to the newer version of the resource, or update your temporary saved copy to include the latest resource version.
* [karmadactl explain](karmadactl_explain.md)	 - Describe fields and structure of various resources in Karmada control plane or a member cluster.

 This command describes the fields associated with each supported API resource. Fields are identified via a simple JSONPath identifier:

        `<type>.<fieldName>[.<fieldName>]`
        
 Information about each field is retrieved from the server in OpenAPI format.
* [karmadactl get](karmadactl_get.md)	 - Display one or many resources in Karmada control plane and member clusters.

 Prints a table of the most important information about the specified resources. You can filter the list using a label selector and the --selector flag. If the desired resource type is namespaced you will only see results in your current namespace unless you pass --all-namespaces.

 By specifying the output as 'template' and providing a Go template as the value of the --template flag, you can filter the attributes of the fetched resources.

## Cluster Registration Commands

* [karmadactl addons](karmadactl_addons.md)	 - Enable or disable a Karmada addon.

 These addons are currently supported:

  1.  karmada-descheduler
  2.  karmada-metrics-adapter
  3.  karmada-scheduler-estimator
  4.  karmada-search
* [karmadactl deinit](karmadactl_deinit.md)	 - Remove the Karmada control plane from the Kubernetes cluster.
* [karmadactl init](karmadactl_init.md)	 - Install the Karmada control plane in a Kubernetes cluster.

 By default, the images and CRD tarball are downloaded remotely. For offline installation, you can set '--private-image-registry' and '--crds'.
* [karmadactl join](karmadactl_join.md)	 - Register a cluster to Karmada control plane with Push mode.
* [karmadactl register](karmadactl_register.md)	 - Register a cluster to Karmada control plane with Pull mode.
* [karmadactl token](karmadactl_token.md)	 - This command manages bootstrap tokens. It is optional and needed only for advanced use cases.

 In short, bootstrap tokens are used for establishing bidirectional trust between a client and a server. A bootstrap token can be used when a client (for example a member cluster that is about to join control plane) needs to trust the server it is talking to. Then a bootstrap token with the "signing" usage can be used. bootstrap tokens can also function as a way to allow short-lived authentication to the API Server (the token serves as a way for the API Server to trust the client), for example for doing the TLS Bootstrap.

 What is a bootstrap token more exactly? - It is a Secret in the kube-system namespace of type "bootstrap.kubernetes.io/token". - A bootstrap token must be of the form "[a-z0-9]{6}.[a-z0-9]{16}". The former part is the public token ID, while the latter is the Token Secret and it must be kept private at all circumstances! - The name of the Secret must be named "bootstrap-token-(token-id)".

 This command is same as 'kubeadm token', but it will create tokens that are used by member clusters.
* [karmadactl unjoin](karmadactl_unjoin.md)	 - Remove a cluster from Karmada control plane.
* [karmadactl unregister](karmadactl_unregister.md)	 - Unregister removes a member cluster from Karmada, it will clean up the cluster object in the control plane and Karmada resources in the member cluster.

## Cluster Management Commands

* [karmadactl cordon](karmadactl_cordon.md)	 - Mark cluster as unschedulable.
* [karmadactl taint](karmadactl_taint.md)	 - Update the taints on one or more clusters.

  *  A taint consists of a key, value, and effect. As an argument here, it is expressed as key=value:effect.
  *  The key must begin with a letter or number, and may contain letters, numbers, hyphens, dots, and underscores, up to 253 characters.
  *  Optionally, the key can begin with a DNS subdomain prefix and a single '/', like example.com/my-app.
  *  The value is optional. If given, it must begin with a letter or number, and may contain letters, numbers, hyphens, dots, and underscores, up to  63 characters.
  *  The effect must be NoSchedule, PreferNoSchedule or NoExecute.
  *  Currently taint can only apply to cluster.
* [karmadactl uncordon](karmadactl_uncordon.md)	 - Mark cluster as schedulable.

## Troubleshooting and Debugging Commands

* [karmadactl attach](karmadactl_attach.md)	 - Attach to a process that is already running inside an existing container.
* [karmadactl describe](karmadactl_describe.md)	 - Show details of a specific resource or group of resources in Karmada control plane or a member cluster.

 Print a detailed description of the selected resources, including related resources such as events or controllers. You may select a single object by name, all objects of that type, provide a name prefix, or label selector. For example:

 $ karmadactl describe TYPE NAME_PREFIX

 will first check for an exact match on TYPE and NAME_PREFIX. If no such resource exists, it will output details for every resource that has a name prefixed with NAME_PREFIX.
* [karmadactl exec](karmadactl_exec.md)	 - Execute a command in a container.
* [karmadactl interpret](karmadactl_interpret.md)	 - Validate, test and edit interpreter customization before applying it to the control plane.
        
  1.  Validate the ResourceInterpreterCustomization configuration as per API schema and try to load the scripts for syntax check.
  2.  Run the rules locally and test if the result is expected. Similar to the dry run.
  3.  Edit customization. Similar to the kubectl edit.
* [karmadactl logs](karmadactl_logs.md)	 - Print the logs for a container in a pod in a member cluster or specified resource. If the pod has only one container, the container name is optional.

## Advanced Commands

* [karmadactl apply](karmadactl_apply.md)	 - Apply a configuration to a resource by file name or stdin and propagate them into member clusters. The resource name must be specified. This resource will be created if it doesn't exist yet. To use 'apply', always create the resource initially with either 'apply' or 'create --save-config'.

 JSON and YAML formats are accepted.

 Alpha Disclaimer: the --prune functionality is not yet complete. Do not use unless you are aware of what the current state is. See https://issues.k8s.io/34274.

 Note: It implements the function of 'kubectl apply' by default. If you want to propagate them into member clusters, please use %[1]s apply --all-clusters'.
* [karmadactl patch](karmadactl_patch.md)	 - Update fields of a resource using strategic merge patch, a JSON merge patch, or a JSON patch.

 JSON and YAML formats are accepted.

 Note: Strategic merge patch is not supported for custom resources.
* [karmadactl promote](karmadactl_promote.md)	 - Promote resources from legacy clusters to the Karmada control plane. Requires the cluster to have been joined or registered.

 If the resource already exists in the Karmada control plane, please edit PropagationPolicy and OverridePolicy to propagate it.
* [karmadactl top](karmadactl_top.md)	 - Display Resource (CPU/Memory) usage of member clusters.

 The top command allows you to see the resource consumption for pods of member clusters.

 This command requires karmada-metrics-adapter to be correctly configured and working on the Karmada control plane and Metrics Server to be correctly configured and working on the member clusters.

## Settings Commands

* [karmadactl annotate](karmadactl_annotate.md)	 - Update the annotations on one or more resources.

 All Kubernetes objects support the ability to store additional data with the object as annotations. Annotations are key/value pairs that can be larger than labels and include arbitrary string values such as structured JSON. Tools and system extensions may use annotations to store their own data.

 Attempting to set an annotation that already exists will fail unless --overwrite is set. If --resource-version is specified and does not match the current resource version on the server the command will fail.

Use "karmadactl api-resources" for a complete list of supported resources.
* [karmadactl label](karmadactl_label.md)	 - Update the labels on a resource.

  *  A label key and value must begin with a letter or number, and may contain letters, numbers, hyphens, dots, and underscores, up to 63 characters each.
  *  Optionally, the key can begin with a DNS subdomain prefix and a single '/', like example.com/my-app.
  *  If --overwrite is true, then existing labels can be overwritten, otherwise attempting to overwrite a label will result in an error.
  *  If --resource-version is specified, then updates will use this resource version, otherwise the existing resource-version will be used.

## Other Commands

* [karmadactl api-resources](karmadactl_api-resources.md)	 - Print the supported API resources on the server.
* [karmadactl api-versions](karmadactl_api-versions.md)	 - Print the supported API versions on the server, in the form of "group/version".

###### Auto generated by [script in Karmada](https://github.com/karmada-io/karmada/tree/master/hack/tools/genkarmadactldocs).s).