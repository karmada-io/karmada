<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Customizing Resource Interpreter](#customizing-resource-interpreter)
  - [Resource Interpreter Framework](#resource-interpreter-framework)
    - [Interpreter Operations](#interpreter-operations)
  - [Built-in Interpreter](#built-in-interpreter)
    - [InterpretReplica](#interpretreplica)
    - [ReviseReplica](#revisereplica)
    - [Retain](#retain)
    - [AggregateStatus](#aggregatestatus)
    - [InterpretStatus](#interpretstatus)
    - [InterpretDependency](#interpretdependency)
  - [Customized Interpreter](#customized-interpreter)
    - [What are interpreter webhooks?](#what-are-interpreter-webhooks)
    - [Write an interpreter webhook server](#write-an-interpreter-webhook-server)
    - [Deploy the admission webhook service](#deploy-the-admission-webhook-service)
    - [Configure webhook on the fly](#configure-webhook-on-the-fly)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Customizing Resource Interpreter

## Resource Interpreter Framework

In the progress of propagating a resource from `karmada-apiserver` to member clusters, Karmada needs to know the 
resource definition. Take `Propagating Deployment` as an example, at the phase of building `ResourceBinding`, the 
`karmada-controller-manager` will parse the `replicas` from the deployment object.

For Kubernetes native resources, Karmada knows how to parse them, but for custom resources defined by `CRD`(or extended
by something like `aggregated-apiserver`), as lack of the knowledge of the resource structure, they can only be treated 
as normal resources. Therefore, the advanced scheduling algorithms cannot be used for them.

The [Resource Interpreter Framework][1] is designed for interpreting resource structure. It consists of `built-in` and 
`customized` interpreters:
- built-in interpreter: used for common Kubernetes native or well-known extended resources.
- customized interpreter: interprets custom resources or overrides the built-in interpreters.

> Note: The major difference between `built-in` and `customized` interpreters is that the `built-in` interpreter is 
> implemented and maintained by Karmada community and will be built into Karmada components, such as 
> `karmada-controller-manager`. On the contrary, the `customized` interpreter is implemented and maintained by users.
> It should be registered to Karmada as an `Interpreter Webhook` (see below for more details).

### Interpreter Operations

When interpreting resources, we often get multiple pieces of information extracted. The `Interpreter Operations`
defines the interpreter request type, and the `Resource Interpreter Framework` provides services for each operation 
type. 

For all operations designed by `Resource Interpreter Framework`, please refer to [Interpreter Operations][2].

> Note: Not all the designed operations are supported (see below for supported operations).

> Note: At most one interpreter will be consulted to when interpreting a resource with specific `interpreter operation`
> and the `customized` interpreter has higher priority than `built-in` interpreter if they are both interpreting the same 
> resource. 
> For example, the `built-in` interpreter serves `InterpretReplica` for `Deployment` with version `apps/v1`. If there 
> is a customized interpreter registered to Karmada for interpreting the same resource, the `customized` interpreter wins and the 
> `built-in` interpreter will be ignored.

## Built-in Interpreter

For the common Kubernetes native or well-known extended resources, the interpreter operations are built-in, which means
the users usually don't need to implement customized interpreters. If you want more resources to be built-in,
please feel free to [file an issue][3] to let us know your user case.

The built-in interpreter now supports following interpreter operations:

### InterpretReplica

Supported resources:
- Deployment(apps/v1)
- Job(batch/v1)

### ReviseReplica

Supported resources:
- Deployment(apps/v1)
- Job(batch/v1)

### Retain

Supported resources:
- Pod(v1)
- Service(v1)
- ServiceAccount(v1)
- PersistentVolumeClaim(v1)
- Job(batch/v1)

### AggregateStatus

Supported resources:
- Deployment(apps/v1)
- Service(v1)
- Ingress(extensions/v1beta1)
- Job(batch/v1)
- DaemonSet(apps/v1)
- StatefulSet(apps/v1)

### InterpretStatus

Supported resources:
- Deployment(apps/v1)
- Service(v1)
- Ingress(extensions/v1beta1)
- Job(batch/v1)
- DaemonSet(apps/v1)
- StatefulSet(apps/v1)

### InterpretDependency

Supported resources:
- Deployment(apps/v1)
- Job(batch/v1)
- Pod(v1)
- DaemonSet(apps/v1)
- StatefulSet(apps/v1)

## Customized Interpreter

The customized interpreter is implemented and maintained by users, it developed as extensions and 
run as webhooks at runtime.

### What are interpreter webhooks?

Interpreter webhooks are HTTP callbacks that receive interpret requests and do something with them.

### Write an interpreter webhook server

Please refer to the implementation of the [Example of Customize Interpreter][4] that is validated 
in Karmada E2E test. The webhook handles the `ResourceInterpreterRequest` request sent by the 
Karmada components (such as `karmada-controller-manager`), and sends back its decision as an 
`ResourceInterpreterResponse`.

### Deploy the admission webhook service

The [Example of Customize Interpreter][4] is deployed in the host cluster for E2E and exposed by 
a service as the front-end of the webhook server.

You may also deploy your webhooks outside the cluster. You will need to update your webhook 
configurations accordingly.

### Configure webhook on the fly

You can configure what resources and supported operations are subject to what interpreter webhook 
via [ResourceInterpreterWebhookConfiguration][5]. 

The following is an example `ResourceInterpreterWebhookConfiguration`:
```yaml
apiVersion: config.karmada.io/v1alpha1
kind: ResourceInterpreterWebhookConfiguration
metadata:
  name: examples
webhooks:
  - name: workloads.example.com
    rules:
      - operations: [ "InterpretReplica","ReviseReplica","Retain","AggregateStatus" ]
        apiGroups: [ "workload.example.io" ]
        apiVersions: [ "v1alpha1" ]
        kinds: [ "Workload" ]
    clientConfig:
      url: https://karmada-interpreter-webhook-example.karmada-system.svc:443/interpreter-workload
      caBundle: {{caBundle}}
    interpreterContextVersions: [ "v1alpha1" ]
    timeoutSeconds: 3
```

You can config more than one webhook in a `ResourceInterpreterWebhookConfiguration`, each webhook
serves at least one operation.

[1]: https://github.com/karmada-io/karmada/tree/master/docs/proposals/resource-interpreter-webhook
[2]: https://github.com/karmada-io/karmada/blob/master/pkg/apis/config/v1alpha1/resourceinterpreterwebhook_types.go#L71-L108
[3]: https://github.com/karmada-io/karmada/issues/new?assignees=&labels=kind%2Ffeature&template=enhancement.md
[4]: https://github.com/karmada-io/karmada/tree/master/examples/customresourceinterpreter
[5]: https://github.com/karmada-io/karmada/blob/master/pkg/apis/config/v1alpha1/resourceinterpreterwebhook_types.go#L16
[6]: https://github.com/karmada-io/karmada/blob/master/examples/customresourceinterpreter/webhook-configuration.yaml